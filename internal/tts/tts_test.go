package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type recordingSleep struct{ waits []time.Duration }

func (r *recordingSleep) sleep(_ context.Context, d time.Duration) error {
	r.waits = append(r.waits, d)
	return nil
}

func retrying(sl *recordingSleep) *http.Client {
	t := NewRetryTransport(http.DefaultTransport)
	t.Sleep = sl.sleep
	return &http.Client{Transport: t}
}

func TestSynthesizeSendsExpectedRequest(t *testing.T) {
	var got synthesizeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/text:synthesize" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		json.NewEncoder(w).Encode(synthesizeResponse{AudioContent: base64.StdEncoding.EncodeToString([]byte("MP3DATA"))})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	audio, err := c.Synthesize(context.Background(), Request{Text: "كِتَاب", Voice: "ar-XA-Test", Rate: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if string(audio) != "MP3DATA" {
		t.Fatalf("audio %q", audio)
	}
	if got.Input.Text != "كِتَاب" || got.Voice.Name != "ar-XA-Test" || got.Voice.LanguageCode != "ar-XA" {
		t.Fatalf("request %+v", got)
	}
	if got.AudioConfig.AudioEncoding != "MP3" || got.AudioConfig.SpeakingRate != 0.9 {
		t.Fatalf("audio config %+v", got.AudioConfig)
	}
}

func TestSynthesizeRejectsBadInput(t *testing.T) {
	c := &Client{BaseURL: "http://unused.invalid", HTTP: http.DefaultClient}
	if _, err := c.Synthesize(context.Background(), Request{Text: "  "}); err == nil {
		t.Fatal("empty text accepted")
	}
	if _, err := c.Synthesize(context.Background(), Request{Text: strings.Repeat("ا", 3000)}); err == nil {
		t.Fatal("oversized text accepted")
	}
}

func TestRetryHonoursRetryAfterThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "7")
			http.Error(w, `{"error":{"message":"slow down","status":"RESOURCE_EXHAUSTED"}}`, http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(synthesizeResponse{AudioContent: base64.StdEncoding.EncodeToString([]byte("ok"))})
	}))
	defer srv.Close()

	sl := &recordingSleep{}
	c := &Client{BaseURL: srv.URL, HTTP: retrying(sl)}
	if _, err := c.Synthesize(context.Background(), Request{Text: "x", Voice: "v", Rate: 1}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("calls %d", calls.Load())
	}
	if len(sl.waits) != 2 || sl.waits[0] != 7*time.Second || sl.waits[1] != 7*time.Second {
		t.Fatalf("waits %v", sl.waits)
	}
	if bodies[0] == "" || bodies[0] != bodies[2] {
		t.Fatalf("body was not replayed: %q vs %q", bodies[0], bodies[2])
	}
}

func TestRetryGivesUpAndReportsKind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"busy","status":"UNAVAILABLE"}}`, http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	sl := &recordingSleep{}
	c := &Client{BaseURL: srv.URL, HTTP: retrying(sl)}
	_, err := c.Synthesize(context.Background(), Request{Text: "x", Voice: "v", Rate: 1})
	var api *APIError
	if !errors.As(err, &api) {
		t.Fatalf("error %v", err)
	}
	if api.Kind() != KindTransient || api.Fatal() || !api.Retryable() {
		t.Fatalf("kind %v", api.Kind())
	}
	if len(sl.waits) != 3 {
		t.Fatalf("waits %v", sl.waits)
	}
}

func TestErrorClassification(t *testing.T) {
	cases := []struct {
		status int
		body   string
		kind   Kind
		fatal  bool
	}{
		{401, `{"error":{"message":"bad token","status":"UNAUTHENTICATED"}}`, KindAuth, true},
		{403, `{"error":{"message":"denied","status":"PERMISSION_DENIED"}}`, KindAuth, true},
		{403, `{"error":{"message":"api off","status":"PERMISSION_DENIED","details":[{"reason":"SERVICE_DISABLED"}]}}`, KindQuota, true},
		{429, `{"error":{"message":"quota","status":"RESOURCE_EXHAUSTED"}}`, KindRateLimited, false},
		{400, `{"error":{"message":"bad voice","status":"INVALID_ARGUMENT"}}`, KindBadRequest, false},
		{500, `oops`, KindTransient, false},
		{404, `nope`, KindOther, false},
	}
	for _, tc := range cases {
		e := newAPIError(tc.status, []byte(tc.body), 0)
		if e.Kind() != tc.kind || e.Fatal() != tc.fatal {
			t.Errorf("%d %s: kind %v fatal %v", tc.status, tc.body, e.Kind(), e.Fatal())
		}
	}
}

func TestRetryDoesNotRetryClientErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, `{"error":{"message":"no","status":"UNAUTHENTICATED"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTP: retrying(&recordingSleep{})}
	if _, err := c.Synthesize(context.Background(), Request{Text: "x", Voice: "v", Rate: 1}); err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("calls %d", calls.Load())
	}
}

func TestRetryStopsWhenContextIsCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "x", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	tr := NewRetryTransport(http.DefaultTransport)
	tr.Sleep = func(ctx context.Context, d time.Duration) error {
		cancel()
		return ctx.Err()
	}
	c := &Client{BaseURL: srv.URL, HTTP: &http.Client{Transport: tr}}
	_, err := c.Synthesize(ctx, Request{Text: "x", Voice: "v", Rate: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
}

func TestVoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("languageCode") != "ar-XA" {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		io.WriteString(w, `{"voices":[{"name":"ar-XA-Chirp3-HD-Achernar","ssmlGender":"FEMALE","naturalSampleRateHertz":24000,"languageCodes":["ar-XA"]},{"name":"ar-XA-Wavenet-A","ssmlGender":"FEMALE","naturalSampleRateHertz":24000,"languageCodes":["ar-XA"]}]}`)
	}))
	defer srv.Close()

	voices, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).Voices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(voices) != 2 || voices[0].Name != "ar-XA-Chirp3-HD-Achernar" || voices[0].Gender != "female" || voices[0].SampleRate != 24000 {
		t.Fatalf("voices %+v", voices)
	}
}

type countingWaiter struct{ n int }

func (c *countingWaiter) Wait(context.Context) error { c.n++; return nil }

func TestLimitIsAppliedPerRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"audioContent":"b2s="}`)
	}))
	defer srv.Close()

	w := &countingWaiter{}
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Limit: w}
	for range 3 {
		if _, err := c.Synthesize(context.Background(), Request{Text: "x", Voice: "v", Rate: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if w.n != 3 {
		t.Fatalf("waits %d", w.n)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	h := http.Header{}
	if retryAfter(h, now) != 0 {
		t.Fatal("empty header")
	}
	h.Set("Retry-After", "12")
	if retryAfter(h, now) != 12*time.Second {
		t.Fatal("seconds")
	}
	h.Set("Retry-After", now.Add(90*time.Second).Format(http.TimeFormat))
	if d := retryAfter(h, now); d != 90*time.Second {
		t.Fatalf("date %v", d)
	}
	h.Set("Retry-After", "garbage")
	if retryAfter(h, now) != 0 {
		t.Fatal("garbage")
	}
}
