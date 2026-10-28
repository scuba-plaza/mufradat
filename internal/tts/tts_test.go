package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type route struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
	Status int
	Reply  string
	Header map[string]string
}

type fake struct {
	*httptest.Server
	mu    sync.Mutex
	calls map[string]int
}

func newFake(t *testing.T, routes ...route) *fake {
	t.Helper()
	f := &fake{calls: map[string]int{}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		f.mu.Lock()
		f.calls[key]++
		f.mu.Unlock()
		for _, rt := range routes {
			if rt.Method+" "+rt.Path != key {
				continue
			}
			if !reflect.DeepEqual(nonNil(r.URL.Query()), nonNil(rt.Query)) {
				t.Errorf("%s: query %v, want %v", key, r.URL.Query(), rt.Query)
			}
			raw, _ := io.ReadAll(r.Body)
			if rt.Body == "" {
				if len(raw) != 0 {
					t.Errorf("%s: unexpected body %s", key, raw)
				}
			} else {
				if got := r.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
					t.Errorf("%s: Content-Type %q", key, got)
				}
				var got, want any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Errorf("%s: body is not JSON: %s", key, raw)
				}
				_ = json.Unmarshal([]byte(rt.Body), &want)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s: body\n got  %s\n want %s", key, raw, rt.Body)
				}
			}
			for k, v := range rt.Header {
				w.Header().Set(k, v)
			}
			if rt.Status != 0 {
				w.WriteHeader(rt.Status)
			}
			io.WriteString(w, rt.Reply)
			return
		}
		t.Errorf("unexpected request %s", key)
		http.Error(w, "{}", http.StatusNotFound)
	}))
	t.Cleanup(f.Close)
	return f
}

func nonNil(v url.Values) url.Values {
	if v == nil {
		return url.Values{}
	}
	return v
}

func (f *fake) count(method, path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[method+" "+path]
}

func synthBody(text, voice string, rate float64) string {
	raw, _ := json.Marshal(map[string]any{
		"input":       map[string]any{"text": text},
		"voice":       map[string]any{"languageCode": "ar-XA", "name": voice},
		"audioConfig": map[string]any{"audioEncoding": "MP3", "speakingRate": rate},
	})
	return string(raw)
}

func audioReply(data string) string {
	return `{"audioContent":"` + base64.StdEncoding.EncodeToString([]byte(data)) + `"}`
}

func TestSynthesizeSendsTheDocumentedRequestAndDecodesTheAudio(t *testing.T) {
	f := newFake(t, route{Method: "POST", Path: "/v1/text:synthesize", Body: synthBody("كِتَاب", "ar-XA-Test", 0.9), Reply: audioReply("\xfb\xff\xfeMP3DATA")})
	c := &Client{BaseURL: f.URL, HTTP: f.Client()}
	audio, err := c.Synthesize(context.Background(), Request{Text: "كِتَاب", Voice: "ar-XA-Test", Rate: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if string(audio) != "\xfb\xff\xfeMP3DATA" {
		t.Fatalf("audio %q", audio)
	}
}

func TestSynthesizeKeepsATrailingSlashOutOfThePath(t *testing.T) {
	f := newFake(t, route{Method: "POST", Path: "/v1/text:synthesize", Body: synthBody("x", "v", 1), Reply: audioReply("a")})
	c := &Client{BaseURL: f.URL + "/", HTTP: f.Client()}
	if _, err := c.Synthesize(context.Background(), Request{Text: "x", Voice: "v", Rate: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestSynthesizeRejectsBadInputWithoutCallingTheService(t *testing.T) {
	f := newFake(t)
	c := &Client{BaseURL: f.URL, HTTP: f.Client()}
	if _, err := c.Synthesize(context.Background(), Request{Text: "  "}); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty text: %v", err)
	}
	if _, err := c.Synthesize(context.Background(), Request{Text: strings.Repeat("ا", 3000)}); err == nil || !strings.Contains(err.Error(), "at most 4500") {
		t.Fatalf("oversized text: %v", err)
	}
	if n := f.count("POST", "/v1/text:synthesize"); n != 0 {
		t.Fatalf("service called %d times", n)
	}
}

func TestSynthesizeRejectsRepliesWithoutUsableAudio(t *testing.T) {
	for name, tc := range map[string]struct{ reply, want string }{
		"empty":      {`{"audioContent":""}`, "no audio"},
		"missing":    {`{}`, "no audio"},
		"not base64": {`{"audioContent":"%%%"}`, "invalid audio"},
		"not json":   {`<html>`, "unreadable synthesize response"},
	} {
		f := newFake(t, route{Method: "POST", Path: "/v1/text:synthesize", Body: synthBody("x", "v", 1), Reply: tc.reply})
		c := &Client{BaseURL: f.URL, HTTP: f.Client()}
		_, err := c.Synthesize(context.Background(), Request{Text: "x", Voice: "v", Rate: 1})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want an error containing %q", name, err, tc.want)
		}
	}
}

func TestErrorsAreClassifiedFromTheServiceReply(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		kind    Kind
		fatal   bool
		retry   bool
		message string
	}{
		{"bad token", 401, `{"error":{"message":"bad token","status":"UNAUTHENTICATED"}}`, KindAuth, true, false, "bad token"},
		{"permission denied", 403, `{"error":{"message":"denied","status":"PERMISSION_DENIED"}}`, KindAuth, true, false, "denied"},
		{"api disabled", 403, `{"error":{"message":"api off","status":"PERMISSION_DENIED","details":[{"reason":"SERVICE_DISABLED"}]}}`, KindQuota, true, false, "api off"},
		{"billing", 403, `{"error":{"message":"no billing","status":"PERMISSION_DENIED","details":[{"reason":"BILLING_DISABLED"}]}}`, KindQuota, true, false, "no billing"},
		{"quota named in lower case", 429, `{"error":{"message":"daily limit","status":"RESOURCE_EXHAUSTED","details":[{"reason":"quota_exceeded"}]}}`, KindQuota, true, false, "daily limit"},
		{"rate limited", 429, `{"error":{"message":"quota","status":"RESOURCE_EXHAUSTED"}}`, KindRateLimited, false, true, "quota"},
		{"bad voice", 400, `{"error":{"message":"bad voice","status":"INVALID_ARGUMENT"}}`, KindBadRequest, false, false, "bad voice"},
		{"server error", 500, `oops`, KindTransient, false, true, "oops"},
		{"unavailable", 503, `{"error":{"message":"busy","status":"UNAVAILABLE"}}`, KindTransient, false, true, "busy"},
		{"not found", 404, `nope`, KindOther, false, false, "nope"},
	}
	for _, tc := range cases {
		f := newFake(t, route{Method: "POST", Path: "/v1/text:synthesize", Body: synthBody("x", "v", 1), Status: tc.status, Reply: tc.body})
		c := &Client{BaseURL: f.URL, HTTP: f.Client()}
		_, err := c.Synthesize(context.Background(), Request{Text: "x", Voice: "v", Rate: 1})
		var api *APIError
		if !errors.As(err, &api) {
			t.Errorf("%s: got %T %v", tc.name, err, err)
			continue
		}
		if api.Kind() != tc.kind || api.Fatal() != tc.fatal || api.Retryable() != tc.retry || api.Status != tc.status {
			t.Errorf("%s: kind %v fatal %v retryable %v status %d", tc.name, api.Kind(), api.Fatal(), api.Retryable(), api.Status)
		}
		if api.Message != tc.message {
			t.Errorf("%s: message %q, want %q", tc.name, api.Message, tc.message)
		}
	}
}

func TestErrorTextNamesTheServiceMessageAndStatus(t *testing.T) {
	e := newAPIError(429, []byte(`{"error":{"message":"too fast","status":"RESOURCE_EXHAUSTED"}}`), 0)
	if got := e.Error(); got != "text-to-speech: too fast (HTTP 429)" {
		t.Errorf("Error() = %q", got)
	}
	if got := newAPIError(503, nil, 0).Error(); got != "text-to-speech: Service Unavailable (HTTP 503)" {
		t.Errorf("empty body: %q", got)
	}
	long := newAPIError(500, []byte(strings.Repeat("x", 500)), 0)
	if len(long.Message) != 200 {
		t.Errorf("long body kept %d bytes", len(long.Message))
	}
}

func TestRetryAfterHeaderIsCarriedOnTheError(t *testing.T) {
	f := newFake(t, route{Method: "POST", Path: "/v1/text:synthesize", Body: synthBody("x", "v", 1), Status: 429, Reply: `{}`, Header: map[string]string{"Retry-After": "9"}})
	c := &Client{BaseURL: f.URL, HTTP: f.Client()}
	_, err := c.Synthesize(context.Background(), Request{Text: "x", Voice: "v", Rate: 1})
	var api *APIError
	if !errors.As(err, &api) || api.RetryAfter != 9*time.Second {
		t.Fatalf("got %v", err)
	}
}

func TestVoicesAskForArabicAndDecodeTheCatalogue(t *testing.T) {
	f := newFake(t, route{Method: "GET", Path: "/v1/voices", Query: url.Values{"languageCode": {"ar-XA"}}, Reply: `{"voices":[
		{"languageCodes":["ar-XA"],"name":"ar-XA-Chirp3-HD-Achernar","ssmlGender":"FEMALE","naturalSampleRateHertz":24000},
		{"languageCodes":["ar-XA"],"name":"ar-XA-Wavenet-B","ssmlGender":"MALE","naturalSampleRateHertz":24000}]}`})
	voices, err := (&Client{BaseURL: f.URL, HTTP: f.Client()}).Voices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []VoiceInfo{
		{Name: "ar-XA-Chirp3-HD-Achernar", Gender: "female", SampleRate: 24000},
		{Name: "ar-XA-Wavenet-B", Gender: "male", SampleRate: 24000},
	}
	if !reflect.DeepEqual(voices, want) {
		t.Fatalf("voices %+v", voices)
	}
}

func TestVoicesPropagateServiceErrors(t *testing.T) {
	f := newFake(t, route{Method: "GET", Path: "/v1/voices", Query: url.Values{"languageCode": {"ar-XA"}}, Status: 401, Reply: `{"error":{"message":"expired","status":"UNAUTHENTICATED"}}`})
	_, err := (&Client{BaseURL: f.URL, HTTP: f.Client()}).Voices(context.Background())
	var api *APIError
	if !errors.As(err, &api) || api.Kind() != KindAuth {
		t.Fatalf("got %v", err)
	}
}

type countingWaiter struct{ n int }

func (c *countingWaiter) Wait(context.Context) error { c.n++; return nil }

func TestEveryRequestPassesTheLimiter(t *testing.T) {
	f := newFake(t,
		route{Method: "POST", Path: "/v1/text:synthesize", Body: synthBody("x", "v", 1), Reply: audioReply("a")},
		route{Method: "GET", Path: "/v1/voices", Query: url.Values{"languageCode": {"ar-XA"}}, Reply: `{"voices":[]}`})
	w := &countingWaiter{}
	c := &Client{BaseURL: f.URL, HTTP: f.Client(), Limit: w}
	for range 3 {
		if _, err := c.Synthesize(context.Background(), Request{Text: "x", Voice: "v", Rate: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Voices(context.Background()); err != nil {
		t.Fatal(err)
	}
	if w.n != 4 {
		t.Fatalf("limiter waited %d times, want 4", w.n)
	}
}

func TestSynthesizeAcceptsTextAtTheLimitAndRejectsOneByteMore(t *testing.T) {
	atLimit := strings.Repeat("x", 4500)
	f := newFake(t, route{Method: "POST", Path: "/v1/text:synthesize", Body: synthBody(atLimit, "v", 1), Reply: audioReply("a")})
	c := &Client{BaseURL: f.URL, HTTP: f.Client()}
	if _, err := c.Synthesize(context.Background(), Request{Text: atLimit, Voice: "v", Rate: 1}); err != nil {
		t.Fatalf("4500 bytes should be accepted: %v", err)
	}
	_, err := c.Synthesize(context.Background(), Request{Text: atLimit + "x", Voice: "v", Rate: 1})
	if err == nil || !strings.Contains(err.Error(), "4501 bytes") || !strings.Contains(err.Error(), "at most 4500") {
		t.Fatalf("4501 bytes: %v", err)
	}
	if n := f.count("POST", "/v1/text:synthesize"); n != 1 {
		t.Fatalf("service called %d times, want 1", n)
	}
}

func TestVoicesRejectAReplyThatIsNotJSON(t *testing.T) {
	f := newFake(t, route{Method: "GET", Path: "/v1/voices", Query: url.Values{"languageCode": {"ar-XA"}}, Reply: `<html>`})
	c := &Client{BaseURL: f.URL, HTTP: f.Client()}
	voices, err := c.Voices(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unreadable voices response") || voices != nil {
		t.Fatalf("got %v, %v", voices, err)
	}
}

func TestAnErrorKeepsTheStatusAndEveryDetailReason(t *testing.T) {
	body := `{"error":{"code":403,"message":"Cloud Text-to-Speech API has not been used in project 1 before or it is disabled.","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SERVICE_DISABLED","domain":"googleapis.com"},{"@type":"type.googleapis.com/google.rpc.Help"}]}}`
	e := newAPIError(403, []byte(body), 0)
	if e.Reason != "PERMISSION_DENIED SERVICE_DISABLED" {
		t.Errorf("Reason = %q", e.Reason)
	}
	if e.Message != "Cloud Text-to-Speech API has not been used in project 1 before or it is disabled." {
		t.Errorf("Message = %q", e.Message)
	}
	if got := len(newAPIError(500, []byte(strings.Repeat("y", 500)), 0).Message); got != 200 {
		t.Errorf("a long body is cut to 200 bytes, got %d", got)
	}
}
