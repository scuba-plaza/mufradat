package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

const (
	fakeMistralKey = "fake-mistral-key"
	fakeExample    = "قَرَأَ [الْوَلَدُ] الْكِتَابَ فِي الْبَيْتِ."
	fakeRare       = "قَرَأَ [الْوَلَدُ] الْكِتَابَ عَلَى الطَّاوِلَةِ."
)

var draftRef = regexp.MustCompile(`(?m)^\{"ref":\d+.*$`)

type fakeCard struct {
	Ref       int               `json:"ref"`
	Arabic    string            `json:"arabic"`
	POS       string            `json:"pos"`
	English   string            `json:"english"`
	Hint      string            `json:"hint"`
	Forms     []json.RawMessage `json:"forms"`
	Example   string            `json:"example"`
	ExampleEN string            `json:"example_en"`
	Comment   string            `json:"comment"`
}

type fakeMistral struct {
	*httptest.Server
	requests string
	mu       sync.Mutex
	mode     string
	calls    int
	problems []string
}

func newFakeMistral(t *testing.T, requests string) *fakeMistral {
	t.Helper()
	f := &fakeMistral{requests: requests, mode: "ok"}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/models", f.models)
	mux.HandleFunc("POST /v1/chat/completions", f.chat)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.problem(w, http.StatusNotFound, "unexpected request %s %s", r.Method, r.URL.RequestURI())
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeMistral) setMode(mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = mode
}

func (f *fakeMistral) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = 0
}

func (f *fakeMistral) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeMistral) violations() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.problems, "\n")
}

func (f *fakeMistral) problem(w http.ResponseWriter, status int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	f.mu.Lock()
	f.problems = append(f.problems, msg)
	f.mu.Unlock()
	writeMistralError(w, status, "fake server: "+msg, "invalid_request_error", "")
}

func writeMistralError(w http.ResponseWriter, status int, message, kind, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	var codeJSON any
	if code != "" {
		codeJSON = code
	}
	json.NewEncoder(w).Encode(map[string]any{"object": "error", "message": message, "type": kind, "param": nil, "code": codeJSON})
}

func (f *fakeMistral) authorized(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != "Bearer "+fakeMistralKey {
		writeMistralError(w, http.StatusUnauthorized, "Unauthorized", "invalid_request_error", "")
		return false
	}
	return true
}

func (f *fakeMistral) decode(w http.ResponseWriter, r *http.Request, into any) bool {
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		f.problem(w, http.StatusBadRequest, "Content-Type %q", got)
		return false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		f.problem(w, http.StatusUnprocessableEntity, "bad body: %v", err)
		return false
	}
	return true
}

func (f *fakeMistral) models(w http.ResponseWriter, r *http.Request) {
	appendLine(f.requests, "models")
	if !f.authorized(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"object":"list","data":[{"id":"mistral-medium-latest","object":"model"}]}`)
}

type chatBody struct {
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	Messages    []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	ResponseFormat struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Name   string         `json:"name"`
			Schema map[string]any `json:"schema"`
			Strict bool           `json:"strict"`
		} `json:"json_schema"`
	} `json:"response_format"`
}

func (f *fakeMistral) chat(w http.ResponseWriter, r *http.Request) {
	if !f.authorized(w, r) {
		return
	}
	var req chatBody
	if !f.decode(w, r, &req) {
		return
	}
	rf := req.ResponseFormat
	switch {
	case req.Model == "":
		f.problem(w, http.StatusBadRequest, "no model")
		return
	case req.MaxTokens < 1000:
		f.problem(w, http.StatusBadRequest, "max_tokens %d would truncate a batch", req.MaxTokens)
		return
	case len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user":
		f.problem(w, http.StatusBadRequest, "messages must be one system then one user message")
		return
	case rf.Type != "json_schema" || !rf.JSONSchema.Strict || rf.JSONSchema.Name == "":
		f.problem(w, http.StatusBadRequest, "response_format %+v", rf)
		return
	case rf.JSONSchema.Schema["type"] != "object" || rf.JSONSchema.Schema["additionalProperties"] != false:
		f.problem(w, http.StatusBadRequest, "schema is not a closed object")
		return
	}
	appendLine(f.requests, fmt.Sprintf("chat model=%s", req.Model))

	f.mu.Lock()
	f.calls++
	call, mode := f.calls, f.mode
	f.mu.Unlock()

	user := req.Messages[1].Content
	switch {
	case mode == "plain-text":
		f.chatReply(w, req.Model, "this is not json", "stop")
		return
	case mode == "invalid-model":
		writeMistralError(w, http.StatusBadRequest, "Invalid model: "+req.Model, "invalid_model", "1500")
		return
	case mode == "unauthorized":
		writeMistralError(w, http.StatusUnauthorized, "Unauthorized", "invalid_request_error", "")
		return
	case mode == "overloaded":
		writeMistralError(w, http.StatusServiceUnavailable, "upstream overloaded", "service_unavailable", "")
		return
	case strings.HasPrefix(mode, "limit-after-"):
		n, _ := strconv.Atoi(strings.TrimPrefix(mode, "limit-after-"))
		if call > n {
			w.Header().Set("Retry-After", "3600")
			writeMistralError(w, http.StatusTooManyRequests, "Rate limit exceeded", "rate_limited", "1300")
			return
		}
	}

	var cards []fakeCard
	for _, line := range draftRef.FindAllString(user, -1) {
		var c fakeCard
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			f.problem(w, http.StatusBadRequest, "bad draft line: %v", err)
			return
		}
		c.English = "meaning of " + c.Arabic
		c.Example = fakeExample
		c.ExampleEN = "The boy read the book at home."
		if c.Forms == nil {
			c.Forms = []json.RawMessage{}
		}
		if mode == "rare-always" || mode == "rare-first" && call == 1 {
			c.Example = fakeRare
		}
		if mode == "bad-first" && call == 1 {
			c.Example = "قَرَأَ الْوَلَدُ الْكِتَابَ"
		}
		if mode == "switch-pos" && strings.Contains(user, "asked for this entry") && call == 1 {
			c.POS = "verb"
		}
		cards = append(cards, c)
	}
	content, _ := json.Marshal(map[string]any{"cards": cards})
	f.chatReply(w, req.Model, string(content), "stop")
}

func (f *fakeMistral) chatReply(w http.ResponseWriter, model, content, finish string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id": "cmpl-fake", "object": "chat.completion", "created": 1, "model": model,
		"choices": []any{map[string]any{
			"index": 0, "finish_reason": finish,
			"message": map[string]any{"role": "assistant", "tool_calls": nil, "content": content},
		}},
		"usage": map[string]any{
			"prompt_tokens": 100, "completion_tokens": 10, "total_tokens": 110,
			"prompt_tokens_details": map[string]any{"cached_tokens": 5},
		},
	})
}

func mistralOf(ts *testscript.TestScript) *fakeMistral {
	f, _ := ts.Value(mistralKey{}).(*fakeMistral)
	if f == nil {
		ts.Fatalf("no fake Mistral server in this script")
	}
	return f
}

type mistralKey struct{}

func mistralCommands() map[string]func(ts *testscript.TestScript, neg bool, args []string) {
	return map[string]func(ts *testscript.TestScript, neg bool, args []string){
		"mistral-mode": func(ts *testscript.TestScript, neg bool, args []string) {
			if neg || len(args) != 1 {
				ts.Fatalf("usage: mistral-mode NAME")
			}
			mistralOf(ts).setMode(args[0])
		},
		"mistral-reset": func(ts *testscript.TestScript, neg bool, args []string) {
			mistralOf(ts).reset()
		},
		"mistral-calls": func(ts *testscript.TestScript, neg bool, args []string) {
			if neg || len(args) != 1 {
				ts.Fatalf("usage: mistral-calls N")
			}
			want, err := strconv.Atoi(args[0])
			if err != nil {
				ts.Fatalf("mistral-calls: %v", err)
			}
			if got := mistralOf(ts).callCount(); got != want {
				ts.Fatalf("the fake Mistral chat endpoint was called %d times, want %d", got, want)
			}
		},
	}
}

func appendLine(path, line string) {
	if path == "" {
		return
	}
	var buf bytes.Buffer
	buf.WriteString(line)
	buf.WriteByte('\n')
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer fh.Close()
	fh.Write(buf.Bytes())
}
