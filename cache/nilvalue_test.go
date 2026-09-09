package cache

import "testing"

type nilValueProduct struct {
	Price string
}

func TestIsNilValue(t *testing.T) {
	var nilPtr *nilValueProduct
	var nilIface interface{ Hash() string }

	tests := []struct {
		name string
		data interface{}
		want bool
	}{
		{name: "untyped nil", data: nil, want: true},
		{name: "nil pointer", data: nilPtr, want: true},
		{name: "nil interface", data: nilIface, want: true},
		{name: "pointer to value", data: &nilValueProduct{Price: "0.1"}, want: false},
		{name: "value", data: nilValueProduct{Price: "0.1"}, want: false},
		{name: "nil slice", data: []string(nil), want: false},
		{name: "empty string", data: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNilValue(tt.data); got != tt.want {
				t.Fatalf("IsNilValue(%#v) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

// A repository method that answers "no such thing" with (nil, nil) reaches the
// cache with a nil pointer; gob panics on those, so DataCache must not pass it on.
func TestDataCacheWriteNilPointerIsNoop(t *testing.T) {
	handler := &countingHandler{}
	dc := NewDataCache().WithHandlers(handler).WithNamespace("test")

	var product *nilValueProduct
	if err := dc.Write("GetInstancePricing:eu-central-1:m5.large", product); err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}

	if handler.writes != 0 {
		t.Fatalf("handler saw %d writes, want 0", handler.writes)
	}
}

type countingHandler struct {
	writes int
}

func (h *countingHandler) Type() string                    { return "counting" }
func (h *countingHandler) Read(string, interface{}) bool   { return false }
func (h *countingHandler) Write(string, interface{}) error { h.writes++; return nil }
