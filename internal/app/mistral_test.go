package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/mistral"
	"github.com/scuba-plaza/mufradat/internal/workspace"
)

func appWith(t *testing.T, vars map[string]string) (*App, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	stderr := &bytes.Buffer{}
	a := New(env.Env{
		Getenv: func(k string) string { return vars[k] },
		Now:    time.Now,
		Home:   home,
		Stdout: io.Discard,
		Stderr: stderr,
		Cwd:    func() (string, error) { return home, nil },
	})
	return a, stderr
}

func withWorkspace(t *testing.T, a *App) workspace.Workspace {
	t.Helper()
	root := filepath.Join(t.TempDir(), "ws")
	ws, err := workspace.Init(workspace.InitOptions{Root: root, DeckID: 1000})
	if err != nil {
		t.Fatal(err)
	}
	a.Opts.Workspace = root
	return ws
}

func setUser(t *testing.T, a *App, settings map[string]string) {
	t.Helper()
	for key, value := range settings {
		if err := config.SetFile(a.Dirs().UserConfigFile(), key, value); err != nil {
			t.Fatal(err)
		}
	}
	a.InvalidateConfig()
}

func writeKeyFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheMistralKeyComesFromTheEnvironmentThenTheConfigThenTheImportedFile(t *testing.T) {
	cases := []struct {
		name       string
		env        string
		configured string
		imported   string
		wantValue  string
		wantSource string
	}{
		{"environment wins", "env-key", "cfg-key", "file-key", "env-key", "MISTRAL_API_KEY"},
		{"blank environment is skipped", "  ", "cfg-key", "file-key", "cfg-key", "mistral.api_key"},
		{"configuration beats the imported file", "", "cfg-key", "file-key", "cfg-key", "mistral.api_key"},
		{"imported file is the last resort", "", "", "file-key", "file-key", "imported key"},
	}
	for _, tc := range cases {
		a, _ := appWith(t, map[string]string{"MISTRAL_API_KEY": tc.env})
		if tc.configured != "" {
			setUser(t, a, map[string]string{"mistral.api_key": tc.configured})
		}
		if tc.imported != "" {
			if err := a.ImportMistralKey(writeKeyFile(t, tc.imported+"\n")); err != nil {
				t.Fatal(err)
			}
		}
		key, err := a.MistralKey()
		if err != nil || key.Value != tc.wantValue || key.Source != tc.wantSource {
			t.Errorf("%s: got %+v, %v; want %s from %s", tc.name, key, err, tc.wantValue, tc.wantSource)
		}
	}
}

func TestMistralKeyProblemsSayWhatToDoNext(t *testing.T) {
	a, _ := appWith(t, nil)
	_, err := a.MistralKey()
	if !errors.Is(err, mistral.ErrNoKey) || failure.Code(err) != failure.ExitNotSetUp ||
		!strings.Contains(failure.Hint(err), "set MISTRAL_API_KEY, or save the key in a file and run `mufradat auth mistral --import FILE`") {
		t.Errorf("no key: %v, exit %d, hint %q", err, failure.Code(err), failure.Hint(err))
	}

	a, _ = appWith(t, map[string]string{"MISTRAL_API_KEY": "two words"})
	_, err = a.MistralKey()
	if failure.Code(err) != failure.ExitNotSetUp || !strings.Contains(err.Error(), "MISTRAL_API_KEY contains whitespace") ||
		!strings.Contains(failure.Hint(err), "fix the key, or import another with `mufradat auth mistral --import FILE`") {
		t.Errorf("bad key: %v, exit %d, hint %q", err, failure.Code(err), failure.Hint(err))
	}

	a, _ = appWith(t, nil)
	if err := os.MkdirAll(a.Dirs().ImportedMistralKey(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.MistralKey(); err == nil || !strings.HasPrefix(err.Error(), "imported key: ") {
		t.Errorf("an unreadable imported key should name where it came from, got %v", err)
	}
}

func TestImportingAMistralKeyKeepsItTrimmedAndPrivate(t *testing.T) {
	a, _ := appWith(t, nil)
	if err := a.ImportMistralKey(writeKeyFile(t, "  sk-abc123 \n\n")); err != nil {
		t.Fatal(err)
	}
	path := a.Dirs().ImportedMistralKey()
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "sk-abc123\n" {
		t.Errorf("stored %q, %v", raw, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, %v: an API key must be readable by its owner only", info.Mode(), err)
	}

	missing := filepath.Join(t.TempDir(), "absent")
	if err := a.ImportMistralKey(missing); err == nil || err.Error() != missing+" does not exist" {
		t.Errorf("missing file: %v", err)
	}
	empty := writeKeyFile(t, " \n")
	if err := a.ImportMistralKey(empty); err == nil || err.Error() != empty+" is empty" {
		t.Errorf("blank file: %v", err)
	}
	spaced := writeKeyFile(t, "sk abc")
	if err := a.ImportMistralKey(spaced); err == nil || !strings.HasPrefix(err.Error(), spaced+" contains whitespace") {
		t.Errorf("key with a space: %v", err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "sk-abc123\n" {
		t.Errorf("a rejected import must leave the stored key alone, found %q", raw)
	}
}

func TestMistralClientUsesTheConfiguredEndpointAndTimeout(t *testing.T) {
	a, _ := appWith(t, map[string]string{"MISTRAL_API_KEY": "k"})
	a.Env.HTTP = &http.Client{Timeout: 7 * time.Second}
	withWorkspace(t, a)
	setUser(t, a, map[string]string{"mistral.endpoint": "http://127.0.0.1:9"})
	client, err := a.mistralClient()
	if err != nil {
		t.Fatal(err)
	}
	if client.BaseURL != "http://127.0.0.1:9" || client.Key != "k" || client.HTTP.Timeout != 7*time.Second {
		t.Errorf("client %+v", client)
	}
}

const chatOK = `{"model":"mistral-medium-2508","choices":[{"finish_reason":"stop","message":{"content":"{\"a\":1}"}}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`

func chatServer(t *testing.T, handler func(call int, w http.ResponseWriter)) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		io.Copy(io.Discard, r.Body)
		handler(int(calls.Add(1)), w)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func llmFor(t *testing.T, url string) (*App, workspace.Workspace, *bytes.Buffer, llm.Backend) {
	t.Helper()
	a, stderr := appWith(t, map[string]string{"MISTRAL_API_KEY": "k"})
	ws := withWorkspace(t, a)
	setUser(t, a, map[string]string{"mistral.endpoint": url})
	a.llmBackoff = time.Millisecond
	backend, err := a.LLM()
	if err != nil {
		t.Fatal(err)
	}
	return a, ws, stderr, backend
}

func ask(backend llm.Backend) (llm.Response, error) {
	return backend.Complete(context.Background(), llm.Request{System: "S", Prompt: "P", Schema: []byte(`{"type":"object"}`)})
}

func TestNotesAreRetriedAfterATemporaryFailureAndTheUserIsTold(t *testing.T) {
	srv, calls := chatServer(t, func(call int, w http.ResponseWriter) {
		if call == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"object":"error","message":"overloaded"}`)
			return
		}
		fmt.Fprint(w, chatOK)
	})
	_, ws, stderr, backend := llmFor(t, srv.URL)
	resp, err := ask(backend)
	if err != nil || string(resp.Output) != `{"a":1}` || calls.Load() != 2 {
		t.Fatalf("%s, %v after %d calls", resp.Output, err, calls.Load())
	}
	if !strings.Contains(stderr.String(), "temporary failure: overloaded; trying again in ") {
		t.Errorf("the user should be told about the retry, stderr: %q", stderr.String())
	}
	usage, err := os.ReadFile(ws.UsageFile())
	if err != nil || strings.Count(string(usage), "\n") != 1 || !strings.Contains(string(usage), `"models":["mistral-medium-2508"]`) {
		t.Errorf("usage file %q, %v", usage, err)
	}
}

func TestNotesGiveUpAfterThreeAttemptsAtATemporaryFailure(t *testing.T) {
	srv, calls := chatServer(t, func(_ int, w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, `{"object":"error","message":"bad gateway"}`)
	})
	_, ws, _, backend := llmFor(t, srv.URL)
	_, err := ask(backend)
	var transient *llm.TransientError
	if !errors.As(err, &transient) || calls.Load() != 3 {
		t.Fatalf("%v after %d calls, want a temporary failure after exactly 3", err, calls.Load())
	}
	if raw, _ := os.ReadFile(ws.UsageFile()); len(raw) != 0 {
		t.Errorf("a failed request must not be recorded as usage: %q", raw)
	}
}

func TestARejectedKeyIsNotRetried(t *testing.T) {
	srv, calls := chatServer(t, func(_ int, w http.ResponseWriter) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"object":"error","message":"Unauthorized"}`)
	})
	_, _, _, backend := llmFor(t, srv.URL)
	if _, err := ask(backend); !llm.AsAuth(err) || calls.Load() != 1 {
		t.Fatalf("%v after %d calls", err, calls.Load())
	}
}

func TestNotesAreWrittenWithTheConfiguredModel(t *testing.T) {
	var model string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		model = string(raw)
		fmt.Fprint(w, chatOK)
	}))
	t.Cleanup(srv.Close)
	a, _ := appWith(t, map[string]string{"MISTRAL_API_KEY": "k"})
	withWorkspace(t, a)
	setUser(t, a, map[string]string{"mistral.endpoint": srv.URL, "author.model": "mistral-small-latest"})
	backend, err := a.LLM()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ask(backend); err != nil || !strings.Contains(model, `"model":"mistral-small-latest"`) {
		t.Fatalf("%v, request %s", err, model)
	}
}

func TestImportingAKeyFileWithoutATrailingNewlineKeepsEveryCharacter(t *testing.T) {
	a, _ := appWith(t, nil)
	if err := a.ImportMistralKey(writeKeyFile(t, "sk-no-newline")); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(a.Dirs().ImportedMistralKey()); err != nil || string(raw) != "sk-no-newline\n" {
		t.Errorf("stored %q, %v", raw, err)
	}
}
