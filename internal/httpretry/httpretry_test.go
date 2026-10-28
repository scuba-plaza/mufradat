package httpretry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
