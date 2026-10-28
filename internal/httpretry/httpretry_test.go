package httpretry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type recordingSleep struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (r *recordingSleep) sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waits = append(r.waits, d)
	return nil
}

func client(sl *recordingSleep) *http.Client {
	t := New(http.DefaultTransport)
	t.Sleep = sl.sleep
	return &http.Client{Transport: t}
}

func post(t *testing.T, c *http.Client, url, body string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return c.Do(req)
}

func TestHonoursRetryAfterThenSucceedsReplayingTheBody(t *testing.T) {
	var calls atomic.Int32
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "7")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	sl := &recordingSleep{}
	resp, err := post(t, client(sl), srv.URL, "payload")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || calls.Load() != 3 {
		t.Fatalf("status %d after %d calls", resp.StatusCode, calls.Load())
	}
	if len(sl.waits) != 2 || sl.waits[0] != 7*time.Second || sl.waits[1] != 7*time.Second {
		t.Fatalf("waits %v, want two waits of 7s", sl.waits)
	}
	for i, b := range bodies {
		if b != "payload" {
			t.Fatalf("attempt %d sent body %q", i+1, b)
		}
	}
}

func TestGivesUpAfterTheAttemptsAndReturnsTheLastResponse(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	sl := &recordingSleep{}
	resp, err := post(t, client(sl), srv.URL, "x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if calls.Load() != 4 || len(sl.waits) != 3 {
		t.Fatalf("%d calls and %d waits, want 4 and 3", calls.Load(), len(sl.waits))
	}
}

func TestNeverRetriesClientErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity} {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			http.Error(w, "no", status)
		}))
		resp, err := post(t, client(&recordingSleep{}), srv.URL, "x")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		srv.Close()
		if calls.Load() != 1 {
			t.Errorf("status %d was tried %d times", status, calls.Load())
		}
	}
}

func TestStopsWhenTheContextIsCancelledWhileWaiting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "x", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	tr := New(http.DefaultTransport)
	tr.Sleep = func(ctx context.Context, d time.Duration) error {
		cancel()
		return ctx.Err()
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, strings.NewReader("x"))
	_, err := (&http.Client{Transport: tr}).Do(req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want context.Canceled", err)
	}
}

func TestBackoffGrowsAndIsCappedByMaxWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "x", http.StatusBadGateway)
	}))
	defer srv.Close()

	sl := &recordingSleep{}
	tr := New(http.DefaultTransport)
	tr.Sleep = sl.sleep
	tr.Backoff = 100 * time.Millisecond
	tr.MaxWait = 250 * time.Millisecond
	resp, err := post(t, &http.Client{Transport: tr}, srv.URL, "x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(sl.waits) != 3 {
		t.Fatalf("waits %v", sl.waits)
	}
	if sl.waits[0] < 100*time.Millisecond || sl.waits[0] > 150*time.Millisecond {
		t.Errorf("first wait %v, want 100ms plus up to 50ms jitter", sl.waits[0])
	}
	if sl.waits[1] < 200*time.Millisecond || sl.waits[1] > 250*time.Millisecond {
		t.Errorf("second wait %v, want 200ms plus jitter capped at 250ms", sl.waits[1])
	}
	if sl.waits[2] != 250*time.Millisecond {
		t.Errorf("third wait %v, want the 250ms cap", sl.waits[2])
	}
}

func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name, header string
		want         time.Duration
	}{
		{"absent", "", 0},
		{"seconds", "12", 12 * time.Second},
		{"padded", " 12 ", 12 * time.Second},
		{"zero seconds", "0", 0},
		{"http date", now.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second},
		{"date in the past", now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"garbage", "garbage", 0},
		{"negative", "-5", 0},
	}
	for _, tc := range cases {
		h := http.Header{}
		if tc.header != "" {
			h.Set("Retry-After", tc.header)
		}
		if got := RetryAfter(h, now); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRetriesRateLimitsAndEveryServerError(t *testing.T) {
	for _, status := range []int{429, 500, 502, 503, 504, 529} {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			http.Error(w, "no", status)
		}))
		resp, err := post(t, client(&recordingSleep{}), srv.URL, "x")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		srv.Close()
		if calls.Load() != 4 {
			t.Errorf("status %d was tried %d times, want 4", status, calls.Load())
		}
	}
}

func TestRetriesNetworkErrorsAndReturnsTheLastOne(t *testing.T) {
	boom := errors.New("connection reset")
	calls := 0
	tr := New(roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, fmt.Errorf("attempt %d: %w", calls, boom)
	}))
	sl := &recordingSleep{}
	tr.Sleep = sl.sleep
	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/", nil)
	_, err := tr.RoundTrip(req)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "attempt 4") || calls != 4 || len(sl.waits) != 3 {
		t.Fatalf("error %v after %d calls and %d waits", err, calls, len(sl.waits))
	}
}

func TestRecoversWhenTheNetworkComesBack(t *testing.T) {
	calls := 0
	tr := New(roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("connection reset")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}))
	tr.Sleep = (&recordingSleep{}).sleep
	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusOK || calls != 3 {
		t.Fatalf("status %v, error %v, %d calls", resp, err, calls)
	}
}

func TestDoesNotRetryOnceTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	tr := New(roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		cancel()
		return nil, errors.New("closed")
	}))
	sl := &recordingSleep{}
	tr.Sleep = sl.sleep
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.invalid/", nil)
	if _, err := tr.RoundTrip(req); err == nil || calls != 1 || len(sl.waits) != 0 {
		t.Fatalf("error %v after %d calls and %d waits", err, calls, len(sl.waits))
	}
}

func TestReplaysABodyThatCannotBeRewound(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		n := len(bodies)
		mu.Unlock()
		if n < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL, io.NopCloser(strings.NewReader("payload")))
	if req.GetBody != nil {
		t.Fatal("the request must not know how to rewind its body")
	}
	resp, err := client(&recordingSleep{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !slices.Equal(bodies, []string{"payload", "payload", "payload"}) {
		t.Fatalf("bodies %q", bodies)
	}
}

func TestTheDefaultsWaitHalfASecondDoubleAndNeverExceedAMinute(t *testing.T) {
	var retryAfter atomic.Value
	retryAfter.Store("")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := retryAfter.Load().(string); v != "" {
			w.Header().Set("Retry-After", v)
		}
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	sl := &recordingSleep{}
	resp, err := post(t, client(sl), srv.URL, "x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	bounds := [][2]time.Duration{{500 * time.Millisecond, 750 * time.Millisecond}, {time.Second, 1500 * time.Millisecond}, {2 * time.Second, 3 * time.Second}}
	if len(sl.waits) != len(bounds) {
		t.Fatalf("waits %v", sl.waits)
	}
	for i, w := range sl.waits {
		if w < bounds[i][0] || w > bounds[i][1] {
			t.Errorf("wait %d is %v, want between %v and %v", i+1, w, bounds[i][0], bounds[i][1])
		}
	}

	retryAfter.Store("3600")
	sl = &recordingSleep{}
	resp, err = post(t, client(sl), srv.URL, "x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for _, w := range sl.waits {
		if w != time.Minute {
			t.Errorf("a Retry-After of an hour waited %v, want the one-minute cap", w)
		}
	}
}

func TestBackoffIsSpreadWithJitter(t *testing.T) {
	tr := New(http.DefaultTransport)
	seen := map[time.Duration]bool{}
	for range 20 {
		d := tr.delay(1, nil)
		if d < 500*time.Millisecond || d > 750*time.Millisecond {
			t.Fatalf("delay %v outside 500ms..750ms", d)
		}
		seen[d] = true
	}
	if len(seen) < 2 {
		t.Error("twenty retries waited exactly the same time: clients would retry in lockstep")
	}
}
