package verify

import (
	"context"
	"fmt"
	"sync"
)

const flushEvery = 16

type FillResult struct {
	Added    int
	Failures map[string]string
}

type FillOptions struct {
	Fingerprint string
	Progress    func(done, total int)
	Status      func(message string)
}

func Fill(ctx context.Context, engine Engine, cache *Cache, items []Item, opts FillOptions) (FillResult, error) {
	result := FillResult{Failures: map[string]string{}}
	if len(items) == 0 {
		return result, nil
	}
	var (
		mu       sync.Mutex
		buffered []Reading
		flushErr error
	)
	flush := func() {
		if len(buffered) == 0 {
			return
		}
		if err := cache.Append(opts.Fingerprint, buffered); err != nil && flushErr == nil {
			flushErr = err
		}
		result.Added += len(buffered)
		buffered = nil
	}
	ev := Events{
		Reading: func(r Reading) {
			mu.Lock()
			defer mu.Unlock()
			buffered = append(buffered, r)
			if len(buffered) >= flushEvery {
				flush()
			}
		},
		Failure: func(key, message string) {
			mu.Lock()
			defer mu.Unlock()
			result.Failures[key] = message
		},
		Progress: opts.Progress,
		Status:   opts.Status,
	}
	err := engine.Check(ctx, items, ev)
	mu.Lock()
	flush()
	mu.Unlock()
	if flushErr != nil {
		return result, fmt.Errorf("saving readings: %w", flushErr)
	}
	return result, err
}
