package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

type scripted struct {
	errs  []error
	calls int
}

func (s *scripted) Complete(ctx context.Context, req Request) (Response, error) {
	i := s.calls
	s.calls++
	if i < len(s.errs) && s.errs[i] != nil {
		return Response{}, s.errs[i]
	}
	return Response{Usage: Usage{Calls: 1, Input: 10}}, nil
}

func noSleep(waits *[]time.Duration) Sleeper {
	return func(_ context.Context, d time.Duration) error {
		*waits = append(*waits, d)
		return nil
	}
}

func TestRetryingRetriesTransientThenSucceeds(t *testing.T) {
	next := &scripted{errs: []error{&TransientError{"overloaded"}, &TransientError{"overloaded"}}}
	var waits []time.Duration
	r := Retrying{Next: next, Attempts: 3, Backoff: time.Second, MaxWait: time.Minute, Sleep: noSleep(&waits)}
	resp, err := r.Complete(context.Background(), Request{})
	if err != nil || resp.Usage.Calls != 1 || next.calls != 3 {
		t.Fatalf("resp %+v err %v calls %d", resp, err, next.calls)
	}
	if len(waits) != 2 || waits[0] < time.Second || waits[1] < 2*time.Second {
		t.Fatalf("waits %v", waits)
	}
}

func TestRetryingGivesUpAfterAttempts(t *testing.T) {
	next := &scripted{errs: []error{&TransientError{"a"}, &TransientError{"b"}, &TransientError{"c"}}}
	var waits []time.Duration
	r := Retrying{Next: next, Attempts: 3, Backoff: time.Second, MaxWait: time.Minute, Sleep: noSleep(&waits)}
	_, err := r.Complete(context.Background(), Request{})
	var te *TransientError
	if !errors.As(err, &te) || te.Message != "c" || next.calls != 3 {
		t.Fatalf("err %v calls %d", err, next.calls)
	}
}

func TestRetryingNeverRetriesAuthBadOutputOrCallErrors(t *testing.T) {
	for _, e := range []error{&AuthError{"x"}, &BadOutputError{"x"}, &CallError{"x"}} {
		next := &scripted{errs: []error{e, e}}
		r := Retrying{Next: next, Attempts: 3, Backoff: time.Second, MaxWait: time.Minute, Sleep: noSleep(new([]time.Duration))}
		if _, err := r.Complete(context.Background(), Request{}); !errors.Is(err, e) || next.calls != 1 {
			t.Fatalf("%T: err %v calls %d", e, err, next.calls)
		}
	}
}

func TestRetryingHonoursRetryAfterWithinMaxWait(t *testing.T) {
	next := &scripted{errs: []error{&RateLimitedError{RetryAfter: 30 * time.Second}}}
	var waits []time.Duration
	r := Retrying{Next: next, Attempts: 2, Backoff: time.Second, MaxWait: time.Minute, Sleep: noSleep(&waits)}
	if _, err := r.Complete(context.Background(), Request{}); err != nil {
		t.Fatal(err)
	}
	if len(waits) != 1 || waits[0] != 30*time.Second {
		t.Fatalf("waits %v", waits)
	}

	next = &scripted{errs: []error{&RateLimitedError{RetryAfter: 10 * time.Minute}}}
	waits = nil
	r = Retrying{Next: next, Attempts: 3, Backoff: time.Second, MaxWait: time.Minute, Sleep: noSleep(&waits)}
	_, err := r.Complete(context.Background(), Request{})
	var rl *RateLimitedError
	if !errors.As(err, &rl) || len(waits) != 0 || next.calls != 1 {
		t.Fatalf("a wait beyond MaxWait must be returned: err %v waits %v calls %d", err, waits, next.calls)
	}
}

func TestRetryingStopsWhenSleepIsCancelled(t *testing.T) {
	next := &scripted{errs: []error{&TransientError{"x"}, &TransientError{"x"}}}
	r := Retrying{Next: next, Attempts: 3, Backoff: time.Second, MaxWait: time.Minute,
		Sleep: func(context.Context, time.Duration) error { return context.Canceled }}
	if _, err := r.Complete(context.Background(), Request{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
}

func TestRecordingSinksUsageOnSuccessOnly(t *testing.T) {
	var got []Record
	now := time.Date(2026, 3, 4, 5, 6, 7, 800, time.UTC)
	rec := Recording{Next: &scripted{errs: []error{nil, &CallError{"x"}}}, Now: func() time.Time { return now },
		Sink: func(r Record) error { got = append(got, r); return nil }}
	if _, err := rec.Complete(context.Background(), Request{}); err != nil {
		t.Fatal(err)
	}
	if _, err := rec.Complete(context.Background(), Request{}); err == nil {
		t.Fatal("expected error")
	}
	if len(got) != 1 || got[0].Usage.Input != 10 || !got[0].Time.Equal(now.Truncate(time.Second)) {
		t.Fatalf("records %+v", got)
	}
}

func TestUsageAddSumsCountersAndMergesModelsSorted(t *testing.T) {
	var u Usage
	u.Add(Usage{Calls: 1, Input: 5, CacheRead: 2, Models: []string{"b"}})
	u.Add(Usage{Calls: 2, Output: 7, CacheRead: 3, Models: []string{"a", "b"}})
	want := Usage{Calls: 3, Input: 5, Output: 7, CacheRead: 5, Models: []string{"a", "b"}}
	if !reflect.DeepEqual(u, want) {
		t.Fatalf("got %+v, want %+v", u, want)
	}
}

func TestUsageHasNoVendorSpecificFields(t *testing.T) {
	raw, err := json.Marshal(Usage{Calls: 1, Input: 2, Output: 3, CacheRead: 4, Models: []string{"m"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"calls":1,"input":2,"output":3,"cache_read":4,"models":["m"]}` {
		t.Fatalf("usage record changed shape: %s", raw)
	}
}

func TestErrorTextsDescribeTheProblem(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&AuthError{Message: "Unauthorized"}, "the service rejected the credentials: Unauthorized"},
		{&RateLimitedError{Message: "slow down"}, "rate limited: slow down"},
		{&TransientError{Message: "502"}, "temporary failure: 502"},
		{&BadOutputError{Message: "cut off"}, "unusable answer: cut off"},
		{&CallError{Message: "Invalid model: x"}, "Invalid model: x"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("%T: %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestRetryableMeansTemporaryOnly(t *testing.T) {
	retry := []error{&TransientError{}, &RateLimitedError{}, fmt.Errorf("wrapped: %w", &TransientError{})}
	stop := []error{&AuthError{}, &BadOutputError{}, &CallError{}, errors.New("plain"), nil}
	for _, e := range retry {
		if !Retryable(e) {
			t.Errorf("%v should be retryable", e)
		}
	}
	for _, e := range stop {
		if Retryable(e) {
			t.Errorf("%v should not be retryable", e)
		}
	}
	if !AsAuth(fmt.Errorf("wrapped: %w", &AuthError{})) || AsAuth(&CallError{}) {
		t.Error("AsAuth must see through wrapping and only match auth errors")
	}
}
