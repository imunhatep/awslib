package cache

import (
	"encoding"
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Hashable lets a type supply its own cache-key fragment instead of being walked
// structurally. Types whose state lives behind a builder API (unexported fields
// wrapping an SDK input) should implement it.
type Hashable interface {
	Hash() string
}

// keyMaxDepth bounds the structural walk so that a value which is not plain data
// (a cache handler, an SDK client accidentally passed as a parameter) cannot drag
// an unbounded amount of the heap into a cache key.
const keyMaxDepth = 12

var (
	hashableType      = reflect.TypeFor[Hashable]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
	timeLocationType  = reflect.TypeFor[time.Location]()
)

// Key builds a deterministic cache key from a method name and its parameters.
//
// Parameters are rendered by value: pointers are dereferenced, map entries sorted
// and unexported fields included, so equal arguments always produce the same key
// and different arguments never share one. Formatting parameters with %v instead
// would embed pointer addresses for every nested pointer field — which is how AWS
// SDK input structs are built — yielding a key that changes on every call and can
// collide once the allocator reuses an address.
//
// The method name is kept as a readable prefix; the parameters are folded into an
// FNV-64a suffix. The result is safe to use as a file name.
func Key(method string, params ...any) string {
	if len(params) == 0 {
		return method
	}

	var sb strings.Builder
	for i, p := range params {
		if i > 0 {
			sb.WriteByte(',')
		}
		writeCanonical(&sb, reflect.ValueOf(p), 0, map[uintptr]struct{}{})
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(sb.String()))

	return fmt.Sprintf("%s-%016x", method, h.Sum64())
}

// writeCanonical appends a deterministic, value-based rendering of v to sb.
// seen holds the pointers on the current path so self-referential structures
// terminate.
func writeCanonical(sb *strings.Builder, v reflect.Value, depth int, seen map[uintptr]struct{}) {
	if !v.IsValid() {
		sb.WriteString("nil")
		return
	}

	if depth > keyMaxDepth {
		sb.WriteString("...")
		return
	}

	if writeSelfDescribed(sb, v) {
		return
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			sb.WriteString("nil")
			return
		}

		addr := v.Pointer()
		if _, ok := seen[addr]; ok {
			sb.WriteString("<cycle>")
			return
		}

		seen[addr] = struct{}{}
		defer delete(seen, addr)

		writeCanonical(sb, v.Elem(), depth, seen)

	case reflect.Interface:
		if v.IsNil() {
			sb.WriteString("nil")
			return
		}

		writeCanonical(sb, v.Elem(), depth, seen)

	case reflect.Struct:
		writeStruct(sb, v, depth, seen)

	case reflect.Slice:
		if v.IsNil() {
			sb.WriteString("nil")
			return
		}

		writeList(sb, v, depth, seen)

	case reflect.Array:
		writeList(sb, v, depth, seen)

	case reflect.Map:
		if v.IsNil() {
			sb.WriteString("nil")
			return
		}

		writeMap(sb, v, depth, seen)

	case reflect.String:
		sb.WriteString(strconv.Quote(v.String()))

	case reflect.Bool:
		sb.WriteString(strconv.FormatBool(v.Bool()))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		sb.WriteString(strconv.FormatInt(v.Int(), 10))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		sb.WriteString(strconv.FormatUint(v.Uint(), 10))

	case reflect.Float32, reflect.Float64:
		sb.WriteString(strconv.FormatFloat(v.Float(), 'g', -1, 64))

	case reflect.Complex64, reflect.Complex128:
		sb.WriteString(strconv.FormatComplex(v.Complex(), 'g', -1, 128))

	default:
		// Func, Chan, UnsafePointer: no value-based rendering exists. Emitting the
		// type keeps the key stable rather than address-dependent — such values are
		// never the part of a call that distinguishes one result from another.
		sb.WriteString(v.Type().String())
	}
}

// writeSelfDescribed handles the types that must not be walked field by field,
// returning true when it rendered v.
func writeSelfDescribed(sb *strings.Builder, v reflect.Value) bool {
	t := v.Type()

	// time.Location caches the most recently looked-up zone in unexported fields,
	// so walking it would make the key depend on lookup history. The zone name is
	// the only part that identifies it.
	if t == timeLocationType {
		sb.WriteString("Location(")
		sb.WriteString(v.Field(0).String())
		sb.WriteString(")")
		return true
	}

	// Values reached through an unexported field cannot be turned back into an
	// interface, so the escape hatches below are unavailable for them.
	if !v.CanInterface() {
		return false
	}

	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil() {
		return false
	}

	if t.Implements(hashableType) {
		sb.WriteString(v.Interface().(Hashable).Hash())
		return true
	}

	// Covers time.Time and anything else with a canonical textual form.
	if t.Implements(textMarshalerType) {
		text, err := v.Interface().(encoding.TextMarshaler).MarshalText()
		if err == nil {
			sb.Write(text)
			return true
		}
	}

	return false
}

func writeStruct(sb *strings.Builder, v reflect.Value, depth int, seen map[uintptr]struct{}) {
	t := v.Type()

	sb.WriteString(t.Name())
	sb.WriteByte('{')

	for i := 0; i < v.NumField(); i++ {
		if i > 0 {
			sb.WriteByte(',')
		}

		sb.WriteString(t.Field(i).Name)
		sb.WriteByte(':')
		writeCanonical(sb, v.Field(i), depth+1, seen)
	}

	sb.WriteByte('}')
}

func writeList(sb *strings.Builder, v reflect.Value, depth int, seen map[uintptr]struct{}) {
	sb.WriteByte('[')

	for i := 0; i < v.Len(); i++ {
		if i > 0 {
			sb.WriteByte(',')
		}

		writeCanonical(sb, v.Index(i), depth+1, seen)
	}

	sb.WriteByte(']')
}

func writeMap(sb *strings.Builder, v reflect.Value, depth int, seen map[uintptr]struct{}) {
	// Map iteration order is randomised, so entries are rendered then sorted.
	entries := make([]string, 0, v.Len())

	iter := v.MapRange()
	for iter.Next() {
		var entry strings.Builder

		writeCanonical(&entry, iter.Key(), depth+1, seen)
		entry.WriteByte(':')
		writeCanonical(&entry, iter.Value(), depth+1, seen)

		entries = append(entries, entry.String())
	}

	sort.Strings(entries)

	sb.WriteByte('{')
	sb.WriteString(strings.Join(entries, ","))
	sb.WriteByte('}')
}
