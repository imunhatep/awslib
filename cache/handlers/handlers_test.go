package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/allegro/bigcache/v3"
)

type product struct {
	Price string
}

func newMemory(t *testing.T) *InMemory {
	t.Helper()

	bc, err := bigcache.New(context.Background(), bigcache.DefaultConfig(time.Minute))
	if err != nil {
		t.Fatalf("bigcache.New: %v", err)
	}

	return NewInMemory(bc)
}

// gob panics on a top-level nil pointer, so a (nil, nil) repository result must
// be skipped rather than encoded.
func TestWriteNilPointerIsNoop(t *testing.T) {
	var missing *product

	inMemory := newMemory(t)
	if err := inMemory.Write("pricing:missing", missing); err != nil {
		t.Fatalf("InMemory.Write returned %v, want nil", err)
	}

	var read *product
	if inMemory.Read("pricing:missing", &read) {
		t.Fatal("InMemory.Read reported a hit for a value that was never stored")
	}

	inFile, err := NewInFile(t.TempDir(), time.Minute)
	if err != nil {
		t.Fatalf("NewInFile: %v", err)
	}

	if err := inFile.Write("pricing:missing", missing); err != nil {
		t.Fatalf("InFile.Write returned %v, want nil", err)
	}

	if inFile.Read("pricing:missing", &read) {
		t.Fatal("InFile.Read reported a hit for a value that was never stored")
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	inMemory := newMemory(t)
	if err := inMemory.Write("pricing:m5.large", &product{Price: "0.107"}); err != nil {
		t.Fatalf("InMemory.Write: %v", err)
	}

	var read *product
	if !inMemory.Read("pricing:m5.large", &read) {
		t.Fatal("InMemory.Read reported a miss for a stored value")
	}

	if read == nil || read.Price != "0.107" {
		t.Fatalf("InMemory.Read gave %#v, want price 0.107", read)
	}
}
