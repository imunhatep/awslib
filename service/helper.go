package service

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
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

// Generic function to read values and errors from returned channels
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

func WriteToChan[T any](source chan<- T, value T) {
	select {
	case source <- value:
	default:
		log.Warn().Msg("Channel is full, skipping event")
	}
}

// Function to read from the error channel and cancel the context on error
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
