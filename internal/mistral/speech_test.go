package mistral

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/httpretry"
)

const voicesPage1 = `{"data":[
 {"id":"11111111-1111-4111-8111-111111111111","name":"Maryam","slug":"voice_479","languages":["ar"],"gender":"female","kind":"preset","type":"tts"},
 {"id":"22222222-2222-4222-8222-222222222222","name":"Oliver","slug":"voice_001","languages":["en_GB","en_us"],"gender":"male","kind":"preset","type":"tts"},
 {"id":"33333333-3333-4333-8333-333333333333","name":"Layla","slug":"voice_480","languages":["AR_SA","en"],"gender":"female","kind":"preset","type":"tts"}
],"next_page_token":"tok2"}`

const voicesPage2 = `{"data":[
 {"id":"44444444-4444-4444-8444-444444444444","name":"Omar","slug":null,"languages":["ar-EG"],"gender":"male","kind":"custom","type":"tts"}
],"next_page_token":null}`

func speechFake(t *testing.T, speech route) *fake {
	t.Helper()
	list := route{Method: http.MethodGet, Path: "/v2/audio/voices", AnyQuery: true}
	list.Reply = func(call int, q url.Values) reply {
		want := url.Values{"page_size": {"100"}, "language": {"ar"}}
		if call == 2 {
			want.Set("page_token", "tok2")
		}
		if q.Encode() != want.Encode() {
			t.Errorf("voices call %d: query %v, want %v", call, q, want)
		}
		return reply{Body: [...]string{voicesPage1, voicesPage2}[min(call, 2)-1]}
	}
	return newFake(t, list, speech)
}

func speechBody(text, voiceID string) string {
	return `{"model":"voxtral-mini-tts-3","input":` + quote(text) + `,"voice_id":"` + voiceID + `","response_format":"mp3","stream":false}`
}

func clientFor(f *fake) *Client {
	return &Client{BaseURL: f.URL, Key: testKey, HTTP: f.Client()}
}

func TestVoicesFollowsPagesAndKeepsOnlyArabic(t *testing.T) {
	f := speechFake(t, route{Method: http.MethodPost, Path: PathSpeech, Body: "{}", Reply: ok("{}")})
	voices, err := clientFor(f).Voices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, v := range voices {
		names = append(names, v.Name)
	}
	if !slices.Equal(names, []string{"Maryam", "Layla", "Omar"}) {
		t.Fatalf("voices %v: Oliver (English) must be filtered out, the second page must be read", names)
	}
	if voices[0].ID != "11111111-1111-4111-8111-111111111111" || voices[0].Slug != "voice_479" || voices[0].Gender != "female" {
		t.Errorf("first voice %+v", voices[0])
	}
	if voices[2].Slug != "" {
		t.Errorf("a null slug should be empty, got %q", voices[2].Slug)
	}
	if f.count(http.MethodGet, "/v2/audio/voices") != 2 {
		t.Errorf("expected exactly two pages, got %d requests", f.count(http.MethodGet, "/v2/audio/voices"))
	}
}

func TestVoicesAreListedOncePerClient(t *testing.T) {
	f := speechFake(t, route{Method: http.MethodPost, Path: PathSpeech, Body: "{}", Reply: ok("{}")})
	c := clientFor(f)
	for range 3 {
		if _, err := c.Voices(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.count(http.MethodGet, "/v2/audio/voices"); got != 2 {
		t.Fatalf("%d list requests, want 2 (two pages, listed once)", got)
	}
}

func TestVoicesFailureIsNotCachedAndInventsNothing(t *testing.T) {
	f := newFake(t, route{
		Method: http.MethodGet, Path: "/v2/audio/voices",
		Query: url.Values{"page_size": {"100"}, "language": {"ar"}},
		Reply: func(call int, _ url.Values) reply {
			if call == 1 {
				return reply{Status: 503, Body: `{"object":"error","message":"down"}`}
			}
			return reply{Body: voicesPage2}
		},
	})
	c := clientFor(f)
	voices, err := c.Voices(context.Background())
	var api *APIError
	if !errors.As(err, &api) || api.Kind() != KindTransient || voices != nil {
		t.Fatalf("got %v, %v", voices, err)
	}
	voices, err = c.Voices(context.Background())
	if err != nil || len(voices) != 1 || voices[0].Name != "Omar" {
		t.Fatalf("second attempt should retry the listing: %v, %v", voices, err)
	}
}

func TestSynthesizeSendsTheDocumentedRequest(t *testing.T) {
	audio := []byte("ID3\x04fake-mp3")
	f := speechFake(t, route{
		Method: http.MethodPost, Path: PathSpeech,
		Body:  speechBody("قَلَم", "11111111-1111-4111-8111-111111111111"),
		Reply: ok(`{"audio_data":"` + base64.StdEncoding.EncodeToString(audio) + `"}`),
	})
	got, err := clientFor(f).Synthesize(context.Background(), SpeechRequest{Text: "قَلَم", Voice: "voice_479"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(audio) {
		t.Fatalf("audio %q", got)
	}
}

func TestSynthesizeResolvesTheVoiceByIDSlugOrName(t *testing.T) {
	for _, ref := range []string{"11111111-1111-4111-8111-111111111111", "voice_479", "Maryam", "maryam"} {
		f := speechFake(t, route{
			Method: http.MethodPost, Path: PathSpeech,
			Body:  speechBody("كِتَاب", "11111111-1111-4111-8111-111111111111"),
			Reply: ok(`{"audio_data":"` + base64.StdEncoding.EncodeToString([]byte("a")) + `"}`),
		})
		if _, err := clientFor(f).Synthesize(context.Background(), SpeechRequest{Text: "كِتَاب", Voice: ref}); err != nil {
			t.Errorf("%q: %v", ref, err)
		}
		if f.count(http.MethodPost, PathSpeech) != 1 {
			t.Errorf("%q: %d speech requests", ref, f.count(http.MethodPost, PathSpeech))
		}
	}
	f := speechFake(t, route{
		Method: http.MethodPost, Path: PathSpeech,
		Body:  speechBody("x", "44444444-4444-4444-8444-444444444444"),
		Reply: ok(`{"audio_data":"YQ=="}`),
	})
	if _, err := clientFor(f).Synthesize(context.Background(), SpeechRequest{Text: "x", Voice: "Omar"}); err != nil {
		t.Errorf("a voice without a slug must still resolve by name: %v", err)
	}
}

func TestSynthesizeWithAnUnknownVoiceMakesNoSpeechRequest(t *testing.T) {
	f := speechFake(t, route{Method: http.MethodPost, Path: PathSpeech, Body: "{}", Reply: ok("{}")})
	_, err := clientFor(f).Synthesize(context.Background(), SpeechRequest{Text: "x", Voice: "voice_999"})
	var unknown *VoiceError
	if !errors.As(err, &unknown) {
		t.Fatalf("got %T %v", err, err)
	}
	if !unknown.Fatal() {
		t.Error("an unknown voice would fail every clip, so it must stop the batch")
	}
	if !strings.Contains(err.Error(), `"voice_999"`) {
		t.Errorf("message %q should name the voice", err)
	}
	if f.count(http.MethodPost, PathSpeech) != 0 {
		t.Fatalf("the speech endpoint was called %d times", f.count(http.MethodPost, PathSpeech))
	}
}

func TestSynthesizeRefusesAnAmbiguousVoiceName(t *testing.T) {
	page := `{"data":[
	 {"id":"a","name":"Sara","slug":"s1","languages":["ar"],"gender":"female"},
	 {"id":"b","name":"Sara","slug":"s2","languages":["ar"],"gender":"female"}],"next_page_token":null}`
	f := newFake(t,
		route{Method: http.MethodGet, Path: "/v2/audio/voices", Query: url.Values{"page_size": {"100"}, "language": {"ar"}}, Reply: ok(page)},
		route{Method: http.MethodPost, Path: PathSpeech, Body: "{}", Reply: ok("{}")})
	_, err := clientFor(f).Synthesize(context.Background(), SpeechRequest{Text: "x", Voice: "sara"})
	var ve *VoiceError
	if !errors.As(err, &ve) || !strings.Contains(err.Error(), "2 voices") {
		t.Fatalf("got %T %v", err, err)
	}
	if f.count(http.MethodPost, PathSpeech) != 0 {
		t.Error("no speech request expected")
	}
}

func TestSynthesizeRejectsEmptyTextWithoutCallingTheService(t *testing.T) {
	f := newFake(t)
	if _, err := clientFor(f).Synthesize(context.Background(), SpeechRequest{Text: "  ", Voice: "voice_479"}); err == nil {
		t.Fatal("empty text accepted")
	}
	if len(f.requests()) != 0 {
		t.Errorf("requests %v", f.requests())
	}
}

func TestSynthesizeRejectsRepliesWithoutAudio(t *testing.T) {
	for name, body := range map[string]string{
		"missing":       `{}`,
		"empty":         `{"audio_data":""}`,
		"not base64":    `{"audio_data":"***"}`,
		"not an object": `[]`,
	} {
		f := speechFake(t, route{
			Method: http.MethodPost, Path: PathSpeech,
			Body:  speechBody("x", "11111111-1111-4111-8111-111111111111"),
			Reply: ok(body),
		})
		if _, err := clientFor(f).Synthesize(context.Background(), SpeechRequest{Text: "x", Voice: "voice_479"}); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestSynthesizeErrorKinds(t *testing.T) {
	flat := func(msg string) string {
		return `{"object":"error","message":` + quote(msg) + `,"type":"invalid_request_error","param":null,"code":null}`
	}
	cases := []struct {
		name      string
		status    int
		body      string
		kind      Kind
		fatal     bool
		retryable bool
		blocked   bool
	}{
		{"bad key", 401, flat("Unauthorized"), KindAuth, true, false, false},
		{"forbidden", 403, flat("Forbidden"), KindAuth, true, false, false},
		{"rate limited", 429, flat("Rate limit exceeded"), KindRateLimited, false, true, false},
		{"server error", 500, flat("oops"), KindTransient, false, true, false},
		{"validation", 422, `{"object":"error","message":{"detail":[{"type":"extra_forbidden","loc":["body","rate"],"msg":"Extra inputs are not permitted"}]},"type":"invalid_request_error","param":null,"code":null}`, KindBadRequest, false, false, false},
		{"moderation", 403, `{"object":"error","message":"Content flagged by guardrails","type":"guardrail_violation","param":null,"code":"1920"}`, KindBlocked, false, false, true},
	}
	for _, tc := range cases {
		f := speechFake(t, route{
			Method: http.MethodPost, Path: PathSpeech,
			Body:  speechBody("x", "11111111-1111-4111-8111-111111111111"),
			Reply: failing(tc.status, tc.body, nil),
		})
		_, err := clientFor(f).Synthesize(context.Background(), SpeechRequest{Text: "x", Voice: "voice_479"})
		var api *APIError
		if !errors.As(err, &api) {
			t.Errorf("%s: got %T %v", tc.name, err, err)
			continue
		}
		if api.Kind() != tc.kind || api.Fatal() != tc.fatal || api.Retryable() != tc.retryable || api.Blocked() != tc.blocked || api.Status != tc.status {
			t.Errorf("%s: kind %v fatal %v retryable %v blocked %v status %d", tc.name, api.Kind(), api.Fatal(), api.Retryable(), api.Blocked(), api.Status)
		}
	}
}

func TestErrorMessagesCarryTheServiceText(t *testing.T) {
	e := newAPIError(403, []byte(`{"object":"error","message":"Content flagged by guardrails","type":"guardrail_violation","code":"1920"}`), 0)
	if e.Message != "Content flagged by guardrails" || e.Type != "guardrail_violation" || e.Code != "1920" {
		t.Errorf("%+v", e)
	}
	if got := e.Error(); got != "mistral: Content flagged by guardrails (HTTP 403)" {
		t.Errorf("Error() = %q", got)
	}
	v := newAPIError(422, []byte(`{"object":"error","message":{"detail":[{"msg":"Extra inputs are not permitted"}]},"type":"invalid_request_error","code":3000}`), 0)
	if !strings.Contains(v.Message, "Extra inputs are not permitted") || v.Code != "3000" {
		t.Errorf("structured message not kept: %+v", v)
	}
	plain := newAPIError(502, []byte("<html>Bad Gateway</html>"), 0)
	if plain.Message != "<html>Bad Gateway</html>" {
		t.Errorf("%+v", plain)
	}
	empty := newAPIError(503, nil, 0)
	if got := empty.Error(); got != "mistral: Service Unavailable (HTTP 503)" {
		t.Errorf("Error() = %q", got)
	}
}

func TestAModerationBlockIsNeverRetried(t *testing.T) {
	f := speechFake(t, route{
		Method: http.MethodPost, Path: PathSpeech,
		Body:  speechBody("x", "11111111-1111-4111-8111-111111111111"),
		Reply: failing(403, `{"object":"error","message":"blocked","type":"guardrail_violation","code":"1920"}`, nil),
	})
	tr := httpretry.New(http.DefaultTransport)
	tr.Sleep = func(context.Context, time.Duration) error { return nil }
	c := &Client{BaseURL: f.URL, Key: testKey, HTTP: &http.Client{Transport: tr}}
	_, err := c.Synthesize(context.Background(), SpeechRequest{Text: "x", Voice: "voice_479"})
	var api *APIError
	if !errors.As(err, &api) || !api.Blocked() {
		t.Fatalf("got %v", err)
	}
	if got := f.count(http.MethodPost, PathSpeech); got != 1 {
		t.Fatalf("speech endpoint called %d times", got)
	}
}

func TestTemporaryFailuresAreRetriedUntilTheyClear(t *testing.T) {
	f := speechFake(t, route{
		Method: http.MethodPost, Path: PathSpeech,
		Body: speechBody("x", "11111111-1111-4111-8111-111111111111"),
		Reply: func(call int, _ url.Values) reply {
			if call < 3 {
				return reply{Status: 429, Body: `{"object":"error","message":"slow"}`, Header: map[string]string{"Retry-After": "2"}}
			}
			return reply{Body: `{"audio_data":"YQ=="}`}
		},
	})
	var waits []time.Duration
	tr := httpretry.New(http.DefaultTransport)
	tr.Sleep = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	c := &Client{BaseURL: f.URL, Key: testKey, HTTP: &http.Client{Transport: tr}}
	if _, err := c.Synthesize(context.Background(), SpeechRequest{Text: "x", Voice: "voice_479"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(waits, []time.Duration{2 * time.Second, 2 * time.Second}) {
		t.Fatalf("waits %v", waits)
	}
}

type countingWaiter struct{ n int }

func (c *countingWaiter) Wait(context.Context) error { c.n++; return nil }

func TestEveryRequestPassesTheSharedLimiter(t *testing.T) {
	f := speechFake(t, route{
		Method: http.MethodPost, Path: PathSpeech,
		Body:  speechBody("x", "11111111-1111-4111-8111-111111111111"),
		Reply: ok(`{"audio_data":"YQ=="}`),
	})
	w := &countingWaiter{}
	c := clientFor(f)
	c.Limit = w
	for range 3 {
		if _, err := c.Synthesize(context.Background(), SpeechRequest{Text: "x", Voice: "voice_479"}); err != nil {
			t.Fatal(err)
		}
	}
	if want := 3 + 2; w.n != want {
		t.Fatalf("limiter waited %d times, want %d (3 speech requests and 2 voice pages)", w.n, want)
	}
}

func TestCheckKeyUsesTheFreeModelsEndpoint(t *testing.T) {
	f := newFake(t, route{Method: http.MethodGet, Path: PathModels, Reply: ok(`{"object":"list","data":[]}`)})
	if err := clientFor(f).CheckKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	bad := newFake(t, route{Method: http.MethodGet, Path: PathModels, Reply: failing(401, `{"object":"error","message":"Unauthorized"}`, nil)})
	err := clientFor(bad).CheckKey(context.Background())
	var api *APIError
	if !errors.As(err, &api) || api.Kind() != KindAuth {
		t.Fatalf("got %v", err)
	}
}
