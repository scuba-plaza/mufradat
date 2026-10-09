package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/proc"
)

const (
	DefaultCommand = "claude"
	DefaultTimeout = 15 * time.Minute
	maxSystemBytes = 100_000
)

type Backend struct {
	Runner  proc.Runner
	Command string
	Model   string
	Effort  string
	Timeout time.Duration
	Now     func() time.Time
}

type result struct {
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype"`
	IsError          bool            `json:"is_error"`
	APIErrorStatus   *int            `json:"api_error_status"`
	TerminalReason   string          `json:"terminal_reason"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	TotalCostUSD     float64         `json:"total_cost_usd"`
	Usage            struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
	ModelUsage map[string]json.RawMessage `json:"modelUsage"`
}

func (b Backend) command() string {
	if b.Command != "" {
		return b.Command
	}
	return DefaultCommand
}

func (b Backend) Args(req llm.Request) []string {
	args := []string{
		"-p",
		"--output-format", "json",
		"--json-schema", string(req.Schema),
		"--system-prompt", req.System,
		"--tools", "",
		"--no-session-persistence",
	}
	if b.Model != "" {
		args = append(args, "--model", b.Model)
	}
	if b.Effort != "" {
		args = append(args, "--effort", b.Effort)
	}
	return args
}

func (b Backend) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	if len(req.System) > maxSystemBytes {
		return llm.Response{}, &llm.CallError{Message: fmt.Sprintf("the system prompt is %d bytes, more than the %d a command line can carry", len(req.System), maxSystemBytes)}
	}
	timeout := b.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	runErr := b.Runner.Run(callCtx, proc.Cmd{
		Name:   b.command(),
		Args:   b.Args(req),
		Stdin:  strings.NewReader(req.Prompt),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err := ctx.Err(); err != nil {
		return llm.Response{}, err
	}
	if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		return llm.Response{}, &llm.TransientError{Message: fmt.Sprintf("claude did not answer within %s", timeout)}
	}

	var out result
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &out); err != nil || out.Type != "result" {
		return llm.Response{}, b.unreadable(runErr, stdout.String(), stderr.String())
	}
	if out.IsError {
		return llm.Response{}, b.classify(out)
	}
	if runErr != nil {
		return llm.Response{}, &llm.CallError{Message: fmt.Sprintf("claude: %v", runErr)}
	}
	if len(out.StructuredOutput) == 0 || string(out.StructuredOutput) == "null" {
		return llm.Response{}, &llm.BadOutputError{Message: "claude answered in plain text instead of the requested structure"}
	}
	return llm.Response{Output: out.StructuredOutput, Usage: out.usage()}, nil
}

func (r result) usage() llm.Usage {
	u := llm.Usage{
		Calls:      1,
		Input:      r.Usage.InputTokens,
		Output:     r.Usage.OutputTokens,
		CacheRead:  r.Usage.CacheReadInputTokens,
		CacheWrite: r.Usage.CacheCreationInputTokens,
		CostUSD:    r.TotalCostUSD,
	}
	for m := range r.ModelUsage {
		u.Models = append(u.Models, m)
	}
	u.Add(llm.Usage{})
	return u
}

func (b Backend) unreadable(runErr error, stdout, stderr string) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = strings.TrimSpace(stdout)
	}
	if len(detail) > 400 {
		detail = detail[:400]
	}
	switch {
	case errors.Is(runErr, proc.ErrNotFound):
		return &llm.UnavailableError{Err: fmt.Errorf("%q is not on PATH", b.command())}
	case runErr != nil:
		return &llm.CallError{Message: fmt.Sprintf("claude failed: %v: %s", runErr, detail)}
	}
	return &llm.BadOutputError{Message: "claude did not print a JSON result: " + detail}
}

func (b Backend) classify(r result) error {
	msg := strings.TrimSpace(r.Result)
	if msg == "" {
		msg = r.Subtype
	}
	status := 0
	if r.APIErrorStatus != nil {
		status = *r.APIErrorStatus
	}
	lower := strings.ToLower(msg)
	switch {
	case looksLikeUsageLimit(lower):
		return &llm.UsageLimitError{ResetAt: parseReset(msg, b.now()), Message: msg}
	case status == 401, status == 403, containsAny(lower, "invalid api key", "please run /login", "not logged in", "authentication"):
		return &llm.AuthError{Message: msg}
	case status == 429:
		return &llm.RateLimitedError{Message: msg}
	case status == 408, status >= 500, containsAny(lower, "overloaded", "timed out", "timeout", "connection"):
		return &llm.TransientError{Message: msg}
	}
	return &llm.CallError{Message: "claude: " + msg}
}

func (b Backend) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func looksLikeUsageLimit(lower string) bool {
	return containsAny(lower, "hit your limit", "usage limit", "limit reached", "limit will reset") ||
		(strings.Contains(lower, "limit") && strings.Contains(lower, "resets"))
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
