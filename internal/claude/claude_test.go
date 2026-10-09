package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/proc"
)

type fakeRunner struct {
	stdout string
	stderr string
	err    error
	got    proc.Cmd
	stdin  string
	delay  time.Duration
}

func (f *fakeRunner) Run(ctx context.Context, c proc.Cmd) error {
	f.got = c
	if c.Stdin != nil {
		raw := new(strings.Builder)
		buf := make([]byte, 4096)
		for {
			n, err := c.Stdin.Read(buf)
			raw.Write(buf[:n])
			if err != nil {
				break
			}
		}
		f.stdin = raw.String()
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
		}
	}
	c.Stdout.Write([]byte(f.stdout))
	c.Stderr.Write([]byte(f.stderr))
	return f.err
}

func (f *fakeRunner) Start(context.Context, proc.Cmd) (proc.Process, error) {
	return nil, errors.New("unused")
}
func (f *fakeRunner) LookPath(string) (string, error) { return "claude", nil }

func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func backend(r proc.Runner) Backend {
	return Backend{Runner: r, Model: "sonnet", Effort: "medium", Now: func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }}
}

func TestCompleteBuildsCommandAndParsesSuccess(t *testing.T) {
	r := &fakeRunner{stdout: fixture(t, "success.json")}
	resp, err := backend(r).Complete(context.Background(), llm.Request{System: "SYS", Prompt: "PROMPT", Schema: []byte(`{"type":"object"}`)})
	if err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, resp.Output); err != nil || compact.String() != `{"answer":"ok"}` {
		t.Fatalf("output %s", resp.Output)
	}
	if r.stdin != "PROMPT" {
		t.Fatalf("stdin %q", r.stdin)
	}
	want := []string{"-p", "--output-format", "json", "--json-schema", `{"type":"object"}`, "--system-prompt", "SYS", "--tools", "", "--no-session-persistence", "--model", "sonnet", "--effort", "medium"}
	if !slices.Equal(r.got.Args, want) {
		t.Fatalf("args %q", r.got.Args)
	}
	u := resp.Usage
	if u.Calls != 1 || u.Input != 1342 || u.Output != 115 || u.CostUSD <= 0 || !slices.Equal(u.Models, []string{"claude-haiku-4-5-20251001"}) {
		t.Fatalf("usage %+v", u)
	}
}

func TestCompleteClassifiesFailures(t *testing.T) {
	cases := []struct {
		fixture string
		check   func(error) bool
	}{
		{"usage_limit.json", func(err error) bool { _, ok := llm.AsUsageLimit(err); return ok }},
		{"rate_limited.json", func(err error) bool { var e *llm.RateLimitedError; return errors.As(err, &e) }},
		{"overloaded.json", func(err error) bool { var e *llm.TransientError; return errors.As(err, &e) }},
		{"not_logged_in.json", llm.AsAuth},
		{"bad_model.json", func(err error) bool {
			var e *llm.CallError
			return errors.As(err, &e) && strings.Contains(e.Message, "not-a-model")
		}},
		{"text_only.json", func(err error) bool { var e *llm.BadOutputError; return errors.As(err, &e) }},
	}
	for _, tc := range cases {
		r := &fakeRunner{stdout: fixture(t, tc.fixture), err: &proc.ExitError{Name: "claude", Code: 1}}
		if tc.fixture == "text_only.json" {
			r.err = nil
		}
		_, err := backend(r).Complete(context.Background(), llm.Request{System: "s", Prompt: "p", Schema: []byte("{}")})
		if err == nil || !tc.check(err) {
			t.Errorf("%s: classified as %T: %v", tc.fixture, err, err)
		}
	}
}

func TestUsageLimitCarriesResetTime(t *testing.T) {
	r := &fakeRunner{stdout: fixture(t, "usage_limit.json"), err: &proc.ExitError{Name: "claude", Code: 1}}
	_, err := backend(r).Complete(context.Background(), llm.Request{System: "s", Prompt: "p", Schema: []byte("{}")})
	limit, ok := llm.AsUsageLimit(err)
	if !ok {
		t.Fatalf("err %v", err)
	}
	want := time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)
	if !limit.ResetAt.Equal(want) {
		t.Fatalf("reset %v, want %v", limit.ResetAt, want)
	}
}

func TestParseReset(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata")
	}
	cases := []struct {
		msg  string
		want time.Time
	}{
		{"Claude AI usage limit reached|1791460800", time.Unix(1791460800, 0).UTC()},
		{"You've hit your limit · resets 3pm (UTC)", time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)},
		{"You've hit your limit · resets 11am (UTC)", time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)},
		{"5-hour limit reached ∙ resets 12:30 am (UTC)", time.Date(2026, 10, 9, 0, 30, 0, 0, time.UTC)},
		{"limit resets 3pm (Europe/Berlin)", time.Date(2026, 10, 8, 15, 0, 0, 0, berlin).UTC()},
		{"limit reached", time.Time{}},
		{"resets 25:99", time.Time{}},
	}
	for _, tc := range cases {
		if got := parseReset(tc.msg, now); !got.Equal(tc.want) {
			t.Errorf("%q: got %v want %v", tc.msg, got, tc.want)
		}
	}
}

func TestCompleteReportsMissingBinaryAndGarbage(t *testing.T) {
	r := &fakeRunner{err: proc.ErrNotFound}
	_, err := backend(r).Complete(context.Background(), llm.Request{System: "s", Prompt: "p", Schema: []byte("{}")})
	if !llm.AsUnavailable(err) {
		t.Fatalf("missing binary: %T %v", err, err)
	}

	r = &fakeRunner{stdout: "not json", stderr: "boom"}
	_, err = backend(r).Complete(context.Background(), llm.Request{System: "s", Prompt: "p", Schema: []byte("{}")})
	var bad *llm.BadOutputError
	if !errors.As(err, &bad) || !strings.Contains(bad.Message, "boom") {
		t.Fatalf("garbage: %T %v", err, err)
	}

	r = &fakeRunner{stderr: "error: unknown option '--bogus'", err: &proc.ExitError{Name: "claude", Code: 1}}
	_, err = backend(r).Complete(context.Background(), llm.Request{System: "s", Prompt: "p", Schema: []byte("{}")})
	var call *llm.CallError
	if !errors.As(err, &call) || !strings.Contains(call.Message, "unknown option") {
		t.Fatalf("exit failure: %T %v", err, err)
	}
}

func TestCompleteTimeoutIsTransientButCancelIsNot(t *testing.T) {
	b := backend(&fakeRunner{delay: time.Second, err: errors.New("killed")})
	b.Timeout = 20 * time.Millisecond
	_, err := b.Complete(context.Background(), llm.Request{System: "s", Prompt: "p", Schema: []byte("{}")})
	var te *llm.TransientError
	if !errors.As(err, &te) {
		t.Fatalf("timeout: %T %v", err, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = backend(&fakeRunner{err: errors.New("killed")}).Complete(ctx, llm.Request{System: "s", Prompt: "p", Schema: []byte("{}")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %T %v", err, err)
	}
}

func TestCompleteRejectsOversizedSystemPrompt(t *testing.T) {
	_, err := backend(&fakeRunner{}).Complete(context.Background(), llm.Request{System: strings.Repeat("x", maxSystemBytes+1), Prompt: "p", Schema: []byte("{}")})
	var call *llm.CallError
	if !errors.As(err, &call) {
		t.Fatalf("err %v", err)
	}
}
