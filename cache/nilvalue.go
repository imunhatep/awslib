package cache

import "reflect"

// IsNilValue reports whether data is a nil pointer or a nil interface, i.e. a
// value that must not be cached.
//
// gob.Encoder.Encode *panics* on a top-level nil pointer ("gob: cannot encode
// nil pointer of type ...") instead of returning an error, so a repository
// method that legitimately answers "no such thing" with (nil, nil) — an AWS
// lookup that matched nothing — would take the whole process down as soon as
// the generated cached wrapper stored its result. Writes of such values are
// skipped: nothing is stored and the next call refetches.
//
// Handler implementations serializing with gob must apply the same check.
func IsNilValue(data interface{}) bool {
	if data == nil {
		return true
	}

	switch v := reflect.ValueOf(data); v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}
