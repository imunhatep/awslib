package service

import (
	"errors"
	"testing"
)

func TestWriteToChanReportsFullChannel(t *testing.T) {
	ch := make(chan int, 1)

	if err := WriteToChan(ch, 1); err != nil {
		t.Fatalf("write into an empty buffer: %v", err)
	}

	if err := WriteToChan(ch, 2); !errors.Is(err, ErrChannelFull) {
		t.Fatalf("write into a full buffer returned %v, want ErrChannelFull", err)
	}

	if got := <-ch; got != 1 {
		t.Fatalf("channel holds %d, want the value written before it filled", got)
	}
}
