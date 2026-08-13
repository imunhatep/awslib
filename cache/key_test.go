package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type keyInner struct {
	Name   *string
	Values []string
}

type keyInput struct {
	Period *keyInner
	Limit  *int32
	Nested map[string]*keyInner
}

type keyNode struct {
	Label string
	Next  *keyNode
}

type keyHashable struct {
	hidden string
}

func (k keyHashable) Hash() string { return "hashable:" + k.hidden }

func ptr[T any](v T) *T { return &v }

func TestKey_NoParams(t *testing.T) {
	assert.Equal(t, "ListInstancesAll", Key("ListInstancesAll"))
}

// The regression this whole file exists for: %+v renders nested pointer fields as
// addresses, so two structurally identical inputs used to hash differently.
func TestKey_EqualInputsWithNestedPointersMatch(t *testing.T) {
	build := func() *keyInput {
		return &keyInput{
			Period: &keyInner{Name: ptr("period"), Values: []string{"a", "b"}},
			Limit:  ptr(int32(50)),
			Nested: map[string]*keyInner{"x": {Name: ptr("x")}},
		}
	}

	assert.Equal(t, Key("Get", build()), Key("Get", build()))
}

func TestKey_DifferentInputsDiffer(t *testing.T) {
	a := &keyInput{Period: &keyInner{Name: ptr("one")}}
	b := &keyInput{Period: &keyInner{Name: ptr("two")}}

	assert.NotEqual(t, Key("Get", a), Key("Get", b))

	// a differing value deep inside a slice must still change the key
	c := &keyInput{Period: &keyInner{Values: []string{"a", "b"}}}
	d := &keyInput{Period: &keyInner{Values: []string{"a", "c"}}}
	assert.NotEqual(t, Key("Get", c), Key("Get", d))
}

func TestKey_NilVersusEmptyDiffer(t *testing.T) {
	assert.NotEqual(t,
		Key("Get", &keyInner{Values: nil}),
		Key("Get", &keyInner{Values: []string{}}),
	)
}

func TestKey_DistinguishesMethods(t *testing.T) {
	assert.NotEqual(t, Key("GetA", 1), Key("GetB", 1))
}

func TestKey_PrefixedWithMethodName(t *testing.T) {
	assert.Regexp(t, `^GetCostAndUsage-[0-9a-f]{16}$`, Key("GetCostAndUsage", 1))
}

func TestKey_MapOrderIndependent(t *testing.T) {
	// Go randomises map iteration; build the same map with different insertion
	// orders and confirm the rendered key is stable across many attempts.
	first := map[string][]string{"a": {"1"}, "b": {"2"}, "c": {"3"}, "d": {"4"}}

	expected := Key("Get", first)
	for range 50 {
		other := map[string][]string{"d": {"4"}, "c": {"3"}, "b": {"2"}, "a": {"1"}}
		assert.Equal(t, expected, Key("Get", other))
	}
}

func TestKey_MultipleParams(t *testing.T) {
	assert.NotEqual(t, Key("Get", "a", "b"), Key("Get", "b", "a"))
	assert.Equal(t, Key("Get", "a", 1, true), Key("Get", "a", 1, true))
}

func TestKey_TimeUsesCanonicalForm(t *testing.T) {
	base := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)

	// same instant, one carrying a monotonic reading stripped by Round(0)
	assert.Equal(t, Key("Get", base), Key("Get", base.Round(0)))
	assert.NotEqual(t, Key("Get", base), Key("Get", base.Add(time.Hour)))
}

func TestKey_HashableEscapeHatch(t *testing.T) {
	// state lives in unexported fields — only Hash() can distinguish these
	assert.NotEqual(t, Key("Get", keyHashable{"a"}), Key("Get", keyHashable{"b"}))
	assert.Equal(t, Key("Get", keyHashable{"a"}), Key("Get", keyHashable{"a"}))
}

func TestKey_UnexportedFieldsAreIncluded(t *testing.T) {
	// a type with no exported fields and no Hash() must not collapse to one key
	type opaque struct{ v string }

	assert.NotEqual(t, Key("Get", opaque{"a"}), Key("Get", opaque{"b"}))
}

func TestKey_CyclicStructureTerminates(t *testing.T) {
	a := &keyNode{Label: "a"}
	b := &keyNode{Label: "b", Next: a}
	a.Next = b

	assert.NotPanics(t, func() { Key("Get", a) })
	assert.NotEqual(t, Key("Get", a), Key("Get", b))
}

func TestKey_NilPointerParam(t *testing.T) {
	var nilInput *keyInput

	assert.NotPanics(t, func() { Key("Get", nilInput) })
	assert.NotEqual(t, Key("Get", nilInput), Key("Get", &keyInput{}))
}

func TestKey_NilInterfaceParam(t *testing.T) {
	assert.NotPanics(t, func() { Key("Get", nil) })
}

// The same pointer appearing twice is not a cycle and must not be reported as one.
func TestKey_SharedPointerIsNotACycle(t *testing.T) {
	shared := &keyInner{Name: ptr("shared")}
	both := []*keyInner{shared, shared}
	distinct := []*keyInner{{Name: ptr("shared")}, {Name: ptr("shared")}}

	assert.Equal(t, Key("Get", both), Key("Get", distinct))
}
