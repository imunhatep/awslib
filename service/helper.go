package service

import (
	"context"
	stderrors "errors"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/rs/zerolog/log"
)

func LastDays(days time.Duration) *time.Time {
	last := time.Now().Add(-time.Hour * 24 * days)
	return &last
}

func ChunkSliceString(slice []string, chunkSize int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize

		// If end is more than the length of the slice, reassign it to the length of the slice
		if end > len(slice) {
			end = len(slice)
		}

		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

func ChunkSlice[T any](slice []T, chunkSize int) [][]T {
	var chunks [][]T
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize

		// If end is more than the length of the slice, reassign it to the length of the slice
		if end > len(slice) {
			end = len(slice)
		}

		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// ReadChannels generic function to read values and errors from returned channels
func ReadChannels[T any](ctx context.Context, resultChan <-chan T, errorChan <-chan *errors.Error) ([]T, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]T, 0)
	var firstError error

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case result, ok := <-resultChan:
				if !ok {
					return
				}
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			case err, ok := <-errorChan:
				if !ok {
					return
				}
				mu.Lock()
				if firstError == nil {
					firstError = err
				}
				mu.Unlock()
			case <-ctx.Done():
				mu.Lock()
				if firstError == nil {
					firstError = ctx.Err()
				}
				mu.Unlock()
				return
			}
		}
	}()

	wg.Wait()
	return results, firstError
}

// ErrChannelFull is returned by WriteToChan when value could not be written without blocking.
var ErrChannelFull = stderrors.New("channel is full")

// WriteToChan writes value to source without blocking, returning ErrChannelFull if the channel
// has no room. The value is dropped then, so the caller decides whether that ends the stream.
func WriteToChan[T any](source chan<- T, value T) error {
	select {
	case source <- value:
		return nil
	default:
		return ErrChannelFull
	}
}

// CancelContextOnError function to read from the error channel and cancel the context on error
func CancelContextOnError(ctx context.Context, cancel context.CancelFunc, errorChan <-chan *errors.Error) {
	go func() {
		log.Debug().Msg("Starting error channel listener")

		select {
		case err, ok := <-errorChan:
			if ok {
				log.Error().Err(err).Msgf("Trace: %s\n", err.ErrorStack())
				cancel() // Cancel the context
			}

		case <-ctx.Done():
		}
	}()
}
