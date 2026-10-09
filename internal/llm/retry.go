package llm

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

type Sleeper func(ctx context.Context, d time.Duration) error

type Retrying struct {
	Next     Backend
	Attempts int
	Backoff  time.Duration
	MaxWait  time.Duration
	Sleep    Sleeper
	OnWait   func(d time.Duration, err error)
}

func (r Retrying) Complete(ctx context.Context, req Request) (Response, error) {
	attempts := max(r.Attempts, 1)
	for attempt := 1; ; attempt++ {
		resp, err := r.Next.Complete(ctx, req)
		if err == nil || attempt >= attempts || !Retryable(err) || ctx.Err() != nil {
			return resp, err
		}
		wait := r.delay(attempt, err)
		if wait > r.MaxWait {
			return resp, err
		}
		if r.OnWait != nil {
			r.OnWait(wait, err)
		}
		if serr := r.Sleep(ctx, wait); serr != nil {
			return Response{}, serr
		}
	}
}

func (r Retrying) delay(attempt int, err error) time.Duration {
	var limited *RateLimitedError
	if errors.As(err, &limited) && limited.RetryAfter > 0 {
		return limited.RetryAfter
	}
	base := r.Backoff << (attempt - 1)
	return base + time.Duration(rand.Int64N(int64(base)/2+1))
}

func Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
