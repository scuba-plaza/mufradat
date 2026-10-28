package clips

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"unicode/utf8"

	"github.com/scuba-plaza/mufradat/internal/note"
)

type Synthesizer interface {
	Synthesize(ctx context.Context, item Item, v Voice) ([]byte, error)
}

type Plan struct {
	Voice   Voice
	Wanted  int
	Present int
	Missing []Item
	Chars   int
}

func NewPlan(notes []note.Note, v Voice, has func(key string) bool) Plan {
	plan := Plan{Voice: v}
	seen := map[string]bool{}
	for _, n := range notes {
		for _, item := range Wanted(n, v) {
			if seen[item.Key] {
				continue
			}
			seen[item.Key] = true
			plan.Wanted++
			if has(item.Key) {
				plan.Present++
				continue
			}
			plan.Missing = append(plan.Missing, item)
			plan.Chars += utf8.RuneCountInString(item.Text)
		}
	}
	return plan
}

type Failure struct {
	Item Item
	Err  error
}

type Result struct {
	Made    int
	Failed  []Failure
	Blocked []Failure
}

type SynthOptions struct {
	Concurrency int
	Progress    func(done, total int)
}

type fatalError struct{ err error }

func (e fatalError) Error() string { return e.err.Error() }
func (e fatalError) Unwrap() error { return e.err }
func (e fatalError) Fatal() bool   { return true }

func IsBlocked(err error) bool {
	var b interface{ Blocked() bool }
	return errors.As(err, &b) && b.Blocked()
}

func isFatal(err error) bool {
	var f interface{ Fatal() bool }
	return errors.As(err, &f) && f.Fatal()
}

func Synthesize(ctx context.Context, plan Plan, store *Store, synth Synthesizer, opts SynthOptions) (Result, error) {
	var res Result
	if len(plan.Missing) == 0 {
		return res, nil
	}
	workers := max(opts.Concurrency, 1)
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	jobs := make(chan Item)
	go func() {
		defer close(jobs)
		for _, item := range plan.Missing {
			select {
			case jobs <- item:
			case <-ctx.Done():
				return
			}
		}
	}()

	var mu sync.Mutex
	var wg sync.WaitGroup
	done := 0
	finish := func(item Item, err error) {
		mu.Lock()
		defer mu.Unlock()
		done++
		if err == nil {
			res.Made++
		} else if !errors.Is(err, context.Canceled) || ctx.Err() == nil {
			if IsBlocked(err) {
				res.Blocked = append(res.Blocked, Failure{Item: item, Err: err})
			} else {
				res.Failed = append(res.Failed, Failure{Item: item, Err: err})
			}
		}
		if opts.Progress != nil {
			opts.Progress(done, len(plan.Missing))
		}
		if err != nil && isFatal(err) {
			cancel(err)
		}
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				if ctx.Err() != nil {
					return
				}
				audio, err := synth.Synthesize(ctx, item, plan.Voice)
				if err == nil {
					if perr := store.Put(item.Key, audio); perr != nil {
						err = fatalError{fmt.Errorf("saving %s: %w", FileName(item.Key), perr)}
					}
				}
				finish(item, err)
			}
		}()
	}
	wg.Wait()

	if cause := context.Cause(ctx); cause != nil && isFatal(cause) {
		res.Failed = withoutFatal(res.Failed, cause)
		return res, cause
	}
	if err := ctx.Err(); err != nil {
		return res, err
	}
	return res, nil
}

func withoutFatal(failed []Failure, cause error) []Failure {
	out := failed[:0]
	for _, f := range failed {
		if f.Err != cause {
			out = append(out, f)
		}
	}
	return out
}

type Orphan struct {
	Key  string
	Size int64
}

func Orphans(store *Store, notes []note.Note, v Voice) ([]Orphan, error) {
	wanted := map[string]bool{}
	for _, n := range notes {
		for _, item := range Wanted(n, v) {
			wanted[item.Key] = true
		}
	}
	keys, err := store.Keys()
	if err != nil {
		return nil, err
	}
	var out []Orphan
	for _, k := range keys {
		if wanted[k] {
			continue
		}
		info, err := os.Stat(store.Path(k))
		if err != nil {
			return nil, err
		}
		out = append(out, Orphan{Key: k, Size: info.Size()})
	}
	return out, nil
}
