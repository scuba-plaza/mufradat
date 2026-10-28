package mistral

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/llm"
)

const (
	chatPath   = "/v1/chat/completions"
	tinySchema = `{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`
)

func chatBody(t *testing.T, model, system, user, schema string) string {
	t.Helper()
	body := `{"model":` + quote(model) + `,"temperature":0.3,"max_tokens":16384,"messages":[` +
		`{"role":"system","content":` + quote(system) + `},` +
		`{"role":"user","content":` + quote(user) + `}]`
	if schema != "" {
		body += `,"response_format":{"type":"json_schema","json_schema":{"name":"response","strict":true,"schema":` + schema + `}}`
	}
	return body + `}`
}

func quote(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func chatReply(content, finish string) string {
	return `{"id":"x","object":"chat.completion","created":1,"model":"mistral-small-latest",` +
		`"choices":[{"index":0,"finish_reason":"` + finish + `","message":{"role":"assistant","tool_calls":null,"content":` + content + `}}],` +
		`"usage":{"prompt_tokens":21,"total_tokens":28,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens_details":{"reasoning_tokens":0},"service_tier":"standard"}}`
}

func backendFor(f *fake) Backend {
	return Backend{Client: &Client{BaseURL: f.URL, Key: testKey, HTTP: f.Client()}, Model: "mistral-small-latest"}
}

func ask(b Backend) (llm.Response, error) {
	return b.Complete(context.Background(), llm.Request{System: "Answer in JSON.", Prompt: "Say ok.", Schema: []byte(tinySchema)})
}

func TestCompleteSendsTheDocumentedRequestAndDecodesTheReply(t *testing.T) {
	f := newFake(t, route{
		Method: http.MethodPost, Path: chatPath,
		Body:  chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema),
		Reply: ok(chatReply(quote(`{"answer":"ok"}`), "stop")),
	})
	resp, err := ask(backendFor(f))
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Output) != `{"answer":"ok"}` {
		t.Errorf("output %s", resp.Output)
	}
	want := llm.Usage{Calls: 1, Input: 21, Output: 7, CacheRead: 4, Models: []string{"mistral-small-latest"}}
	if resp.Usage.Calls != want.Calls || resp.Usage.Input != want.Input || resp.Usage.Output != want.Output ||
		resp.Usage.CacheRead != want.CacheRead || !slices.Equal(resp.Usage.Models, want.Models) {
		t.Errorf("usage %+v, want %+v", resp.Usage, want)
	}
}

func TestCompleteJoinsAnArrayOfTextChunks(t *testing.T) {
	chunks := `[{"type":"text","text":"{\"answer\":"},{"type":"text","text":"\"ok\"}"}]`
	f := newFake(t, route{
		Method: http.MethodPost, Path: chatPath,
		Body:  chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema),
		Reply: ok(chatReply(chunks, "stop")),
	})
	resp, err := ask(backendFor(f))
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Output) != `{"answer":"ok"}` {
		t.Errorf("output %s", resp.Output)
	}
}

func TestCompleteOmitsResponseFormatWithoutASchema(t *testing.T) {
	f := newFake(t, route{
		Method: http.MethodPost, Path: chatPath,
		Body:  chatBody(t, "mistral-small-latest", "S", "P", ""),
		Reply: ok(chatReply(quote(`{"a":1}`), "stop")),
	})
	resp, err := backendFor(f).Complete(context.Background(), llm.Request{System: "S", Prompt: "P"})
	if err != nil || string(resp.Output) != `{"a":1}` {
		t.Fatalf("%s, %v", resp.Output, err)
	}
}

func TestCompleteRejectsAnswersThatCannotBeUsed(t *testing.T) {
	cases := []struct {
		name  string
		reply string
		want  string
	}{
		{"cut off", chatReply(quote(`{"answer":"o`), "length"), "cut off"},
		{"empty content", chatReply(quote(""), "stop"), "empty"},
		{"null content", chatReply("null", "stop"), "empty"},
		{"no choices", `{"id":"x","object":"chat.completion","model":"m","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":0}}`, "no choices"},
		{"plain text", chatReply(quote("Sure, here you go!"), "stop"), "not JSON"},
		{"chunks without text", chatReply(`[{"type":"image_url","image_url":"x"}]`, "stop"), "empty"},
	}
	for _, tc := range cases {
		f := newFake(t, route{
			Method: http.MethodPost, Path: chatPath,
			Body:  chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema),
			Reply: ok(tc.reply),
		})
		_, err := ask(backendFor(f))
		var bad *llm.BadOutputError
		if !errors.As(err, &bad) {
			t.Errorf("%s: got %T %v, want *llm.BadOutputError", tc.name, err, err)
			continue
		}
		if !strings.Contains(bad.Message, tc.want) {
			t.Errorf("%s: message %q should mention %q", tc.name, bad.Message, tc.want)
		}
	}
}

func TestCompleteMapsStatusesToTheLLMErrorTypes(t *testing.T) {
	flat := func(msg string) string {
		return `{"object":"error","message":` + quote(msg) + `,"type":"invalid_request_error","param":null,"code":"x"}`
	}
	t.Run("401 and 403 are authentication failures", func(t *testing.T) {
		for _, status := range []int{401, 403} {
			f := newFake(t, route{Method: http.MethodPost, Path: chatPath, Body: chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema), Reply: failing(status, flat("Unauthorized"), nil)})
			_, err := ask(backendFor(f))
			var auth *llm.AuthError
			if !errors.As(err, &auth) || !strings.Contains(auth.Message, "Unauthorized") {
				t.Errorf("%d: got %T %v", status, err, err)
			}
		}
	})
	t.Run("429 carries Retry-After", func(t *testing.T) {
		f := newFake(t, route{Method: http.MethodPost, Path: chatPath, Body: chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema), Reply: failing(429, flat("Rate limit exceeded"), map[string]string{"Retry-After": "3"})})
		_, err := ask(backendFor(f))
		var limited *llm.RateLimitedError
		if !errors.As(err, &limited) || limited.RetryAfter != 3*time.Second || !strings.Contains(limited.Message, "Rate limit exceeded") {
			t.Errorf("got %T %v", err, err)
		}
	})
	t.Run("429 without Retry-After", func(t *testing.T) {
		f := newFake(t, route{Method: http.MethodPost, Path: chatPath, Body: chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema), Reply: failing(429, flat("slow"), nil)})
		_, err := ask(backendFor(f))
		var limited *llm.RateLimitedError
		if !errors.As(err, &limited) || limited.RetryAfter != 0 {
			t.Errorf("got %T %v", err, err)
		}
	})
	t.Run("5xx are temporary", func(t *testing.T) {
		for _, status := range []int{500, 502, 503, 529} {
			f := newFake(t, route{Method: http.MethodPost, Path: chatPath, Body: chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema), Reply: failing(status, flat("upstream"), nil)})
			_, err := ask(backendFor(f))
			var transient *llm.TransientError
			if !errors.As(err, &transient) || !strings.Contains(transient.Message, "upstream") {
				t.Errorf("%d: got %T %v", status, err, err)
			}
		}
	})
	t.Run("a gateway page that is not JSON is still temporary", func(t *testing.T) {
		f := newFake(t, route{Method: http.MethodPost, Path: chatPath, Body: chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema), Reply: failing(502, "<html>Bad Gateway</html>", nil)})
		_, err := ask(backendFor(f))
		var transient *llm.TransientError
		if !errors.As(err, &transient) || !strings.Contains(transient.Message, "Bad Gateway") {
			t.Errorf("got %T %v", err, err)
		}
	})
	t.Run("other 4xx are call errors carrying the API message", func(t *testing.T) {
		for _, status := range []int{400, 404, 422} {
			f := newFake(t, route{Method: http.MethodPost, Path: chatPath, Body: chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema), Reply: failing(status, flat("Invalid model: nope"), nil)})
			_, err := ask(backendFor(f))
			var call *llm.CallError
			if !errors.As(err, &call) || !strings.Contains(call.Message, "Invalid model: nope") {
				t.Errorf("%d: got %T %v", status, err, err)
			}
		}
	})
	t.Run("a moderation block is not an authentication failure", func(t *testing.T) {
		body := `{"object":"error","message":"blocked","type":"guardrail_violation","param":null,"code":"1920"}`
		f := newFake(t, route{Method: http.MethodPost, Path: chatPath, Body: chatBody(t, "mistral-small-latest", "Answer in JSON.", "Say ok.", tinySchema), Reply: failing(403, body, nil)})
		_, err := ask(backendFor(f))
		var call *llm.CallError
		if !errors.As(err, &call) || !strings.Contains(call.Message, "blocked") || llm.AsAuth(err) {
			t.Errorf("got %T %v", err, err)
		}
	})
}

func TestCompleteReportsAnUnreachableServerAsTemporary(t *testing.T) {
	f := newFake(t)
	b := backendFor(f)
	f.Close()
	_, err := ask(b)
	var transient *llm.TransientError
	if !errors.As(err, &transient) {
		t.Fatalf("got %T %v", err, err)
	}
}

func TestCompleteReturnsContextErrorsUnchanged(t *testing.T) {
	f := newFake(t)
	b := backendFor(f)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := b.Complete(ctx, llm.Request{System: "S", Prompt: "P", Schema: []byte(tinySchema)})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %T %v", err, err)
	}
	if len(f.requests()) != 0 {
		t.Errorf("a cancelled call must not reach the server: %v", f.requests())
	}
}

func TestCompleteRejectsASchemaThatIsNotJSON(t *testing.T) {
	f := newFake(t)
	_, err := backendFor(f).Complete(context.Background(), llm.Request{System: "S", Prompt: "P", Schema: []byte("{nope")})
	var call *llm.CallError
	if !errors.As(err, &call) {
		t.Fatalf("got %T %v", err, err)
	}
	if len(f.requests()) != 0 {
		t.Errorf("no request should be sent: %v", f.requests())
	}
}
