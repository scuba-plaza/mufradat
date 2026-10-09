package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/llm"
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

func TestAuthorFailureExitCodes(t *testing.T) {
	a := testApp(t)
	reset := time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		err  error
		code int
		hint string
	}{
		{"usage limit", &llm.UsageLimitError{ResetAt: reset, Message: "x"}, failure.ExitTemporary, "resets at"},
		{"usage limit without time", &llm.UsageLimitError{Message: "x"}, failure.ExitTemporary, "run the command again later"},
		{"busy", &llm.TransientError{Message: "overloaded"}, failure.ExitTemporary, "in a few minutes"},
		{"rate limited", &llm.RateLimitedError{Message: "slow"}, failure.ExitTemporary, "in a few minutes"},
		{"signed out", &llm.AuthError{Message: "login"}, failure.ExitNotSetUp, "sign in with `claude`"},
		{"missing binary", &llm.UnavailableError{Err: errors.New("not found")}, failure.ExitNotSetUp, "install Claude Code"},
		{"other", errors.New("boom"), failure.ExitFailure, ""},
	}
	for _, tc := range cases {
		got := a.AuthorFailure(tc.err)
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

func TestTTSFailureExitCodes(t *testing.T) {
	a := testApp(t)
	cases := []struct {
		status int
		body   string
		code   int
	}{
		{401, `{"error":{"status":"UNAUTHENTICATED"}}`, failure.ExitNotSetUp},
		{403, `{"error":{"status":"PERMISSION_DENIED","details":[{"reason":"SERVICE_DISABLED"}]}}`, failure.ExitTemporary},
		{429, `{"error":{"status":"RESOURCE_EXHAUSTED"}}`, failure.ExitTemporary},
		{503, `x`, failure.ExitTemporary},
		{400, `{"error":{"status":"INVALID_ARGUMENT"}}`, failure.ExitFailure},
	}
	for _, tc := range cases {
		err := &tts.APIError{Status: tc.status, Reason: tc.body}
		if tc.status == 403 {
			err.Reason = "PERMISSION_DENIED SERVICE_DISABLED"
		}
		if got := failure.Code(a.TTSFailure(err)); got != tc.code {
			t.Errorf("HTTP %d: exit code %d, want %d", tc.status, got, tc.code)
		}
	}
	plain := errors.New("disk full")
	if a.TTSFailure(plain) != plain {
		t.Error("unrelated errors must pass through unchanged")
	}
}
