package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/mistral"
	"github.com/scuba-plaza/mufradat/internal/tts"
)

func testApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	return New(env.Env{
		Getenv: func(string) string { return "" },
		Now:    time.Now,
		Home:   home,
		Cwd:    func() (string, error) { return home, nil },
	})
}

func TestAuthorFailureExitCodesAndHints(t *testing.T) {
	a := testApp(t)
	cases := []struct {
		name string
		err  error
		code int
		hint string
	}{
		{"busy", &llm.TransientError{Message: "overloaded"}, failure.ExitTemporary, "the service is busy; the finished notes are saved"},
		{"rate limited", &llm.RateLimitedError{Message: "slow"}, failure.ExitTemporary, "run the command again in a few minutes"},
		{"bad key", &llm.AuthError{Message: "Unauthorized"}, failure.ExitNotSetUp, "auth mistral --import FILE"},
		{"other", errors.New("boom"), failure.ExitFailure, ""},
		{"call error", &llm.CallError{Message: "Invalid model: x"}, failure.ExitFailure, ""},
	}
	for _, tc := range cases {
		got := a.AuthorFailure(fmt.Errorf("run: %w", tc.err))
		if failure.Code(got) != tc.code {
			t.Errorf("%s: exit code %d, want %d", tc.name, failure.Code(got), tc.code)
		}
		if !strings.Contains(failure.Hint(got), tc.hint) {
			t.Errorf("%s: hint %q lacks %q", tc.name, failure.Hint(got), tc.hint)
		}
		if !errors.Is(got, tc.err) {
			t.Errorf("%s: the original error is lost", tc.name)
		}
	}
}

func TestSpeechFailureForGoogle(t *testing.T) {
	a := testApp(t)
	cases := []struct {
		name string
		err  *tts.APIError
		code int
		hint string
	}{
		{"bad credentials", &tts.APIError{Status: 401, Reason: "UNAUTHENTICATED"}, failure.ExitNotSetUp, "auth google --import KEY.json"},
		{"api disabled", &tts.APIError{Status: 403, Reason: "PERMISSION_DENIED SERVICE_DISABLED"}, failure.ExitTemporary, "google.quota_project"},
		{"rate limited", &tts.APIError{Status: 429, Reason: "RESOURCE_EXHAUSTED"}, failure.ExitTemporary, "run the command again"},
		{"outage", &tts.APIError{Status: 503}, failure.ExitTemporary, "run the command again"},
		{"bad request", &tts.APIError{Status: 400, Reason: "INVALID_ARGUMENT"}, failure.ExitFailure, ""},
	}
	for _, tc := range cases {
		got := a.SpeechFailure(fmt.Errorf("clip: %w", tc.err))
		if failure.Code(got) != tc.code || !strings.Contains(failure.Hint(got), tc.hint) {
			t.Errorf("%s: exit %d hint %q, want exit %d hint containing %q", tc.name, failure.Code(got), failure.Hint(got), tc.code, tc.hint)
		}
		if !errors.Is(got, tc.err) {
			t.Errorf("%s: the original error is lost", tc.name)
		}
	}
}

func TestSpeechFailureForMistral(t *testing.T) {
	a := testApp(t)
	cases := []struct {
		name string
		err  error
		code int
		hint string
	}{
		{"bad key", &mistral.APIError{Status: 401, Message: "Unauthorized"}, failure.ExitNotSetUp, "auth mistral --import FILE"},
		{"forbidden", &mistral.APIError{Status: 403, Message: "Forbidden"}, failure.ExitNotSetUp, "auth mistral --import FILE"},
		{"rate limited", &mistral.APIError{Status: 429}, failure.ExitTemporary, "run the command again"},
		{"outage", &mistral.APIError{Status: 502}, failure.ExitTemporary, "run the command again"},
		{"unreachable", &mistral.APIError{Message: "could not reach the service"}, failure.ExitTemporary, "run the command again"},
		{"moderation", &mistral.APIError{Status: 403, Type: "guardrail_violation", Code: "1920", Message: "flagged"}, failure.ExitFailure, "config set audio.provider google"},
		{"unknown voice", &mistral.VoiceError{Voice: "voice_999"}, failure.ExitFailure, "config set mistral.voice NAME"},
		{"validation", &mistral.APIError{Status: 422, Message: "bad"}, failure.ExitFailure, ""},
	}
	for _, tc := range cases {
		got := a.SpeechFailure(fmt.Errorf("clip: %w", tc.err))
		if failure.Code(got) != tc.code || !strings.Contains(failure.Hint(got), tc.hint) {
			t.Errorf("%s: exit %d hint %q, want exit %d hint containing %q", tc.name, failure.Code(got), failure.Hint(got), tc.code, tc.hint)
		}
		if !errors.Is(got, tc.err) {
			t.Errorf("%s: the original error is lost", tc.name)
		}
	}
	if strings.Contains(failure.Hint(a.SpeechFailure(&mistral.APIError{Status: 403, Type: "guardrail_violation"})), "key") {
		t.Error("a moderation block must never tell the user to check their key")
	}
}

func TestSpeechFailurePassesUnrelatedErrorsThrough(t *testing.T) {
	a := testApp(t)
	plain := errors.New("disk full")
	if a.SpeechFailure(plain) != plain {
		t.Error("unrelated errors must pass through unchanged")
	}
}
