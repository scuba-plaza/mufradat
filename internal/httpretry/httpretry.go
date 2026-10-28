package httpretry

import (
	"bytes"
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Sleeper func(ctx context.Context, d time.Duration) error

type Transport struct {
	Base     http.RoundTripper
	Attempts int
	Backoff  time.Duration
	MaxWait  time.Duration
	Sleep    Sleeper
	Now      func() time.Time
}

func New(base http.RoundTripper) *Transport {
	return &Transport{Base: base, Attempts: 4, Backoff: 500 * time.Millisecond, MaxWait: time.Minute, Sleep: Sleep, Now: time.Now}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempts := max(t.Attempts, 1)
	for attempt := 1; ; attempt++ {
		r, err := cloneRequest(req)
		if err != nil {
			return nil, err
		}
		resp, err := t.Base.RoundTrip(r)
		if err != nil && req.Context().Err() != nil {
			return nil, err
		}
		if attempt >= attempts || !t.shouldRetry(resp, err) {
			return resp, err
		}
		wait := t.delay(attempt, resp)
		if resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
			resp.Body.Close()
		}
		if err := t.Sleep(req.Context(), wait); err != nil {
			return nil, err
		}
	}
}

func (t *Transport) shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
}

func (t *Transport) delay(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if d := RetryAfter(resp.Header, t.Now()); d > 0 {
			return min(d, t.MaxWait)
		}
	}
	base := t.Backoff << (attempt - 1)
	jitter := time.Duration(rand.Int64N(int64(base)/2 + 1))
	return min(base+jitter, t.MaxWait)
}

func cloneRequest(req *http.Request) (*http.Request, error) {
	r := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return r, nil
	}
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		r.Body = body
		return r, nil
	}
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(raw))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(raw)), nil }
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return r, nil
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

func RetryAfter(h http.Header, now time.Time) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}
