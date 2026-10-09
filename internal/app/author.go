package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/scuba-plaza/mufradat/internal/author"
	"github.com/scuba-plaza/mufradat/internal/claude"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/jsonl"
	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/prompt"
)

const (
	llmAttempts      = 3
	llmBackoff       = 2 * time.Second
	llmMaxWait       = 10 * time.Minute
	exampleCount     = 8
	promptVocabulary = 500
)

func (a *App) Claude() (llm.Backend, error) {
	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}
	ws, err := a.Workspace()
	if err != nil {
		return nil, err
	}
	path, err := a.Env.Runner.LookPath(claude.DefaultCommand)
	if err != nil {
		return nil, failure.NotSetUp("install Claude Code (https://claude.com/claude-code) and sign in with `claude`; it writes the notes", err)
	}
	base := claude.Backend{
		Runner: a.Env.Runner, Command: path,
		Model: cfg.Author.Model, Effort: cfg.Author.Effort, Now: a.Env.Now,
	}
	retrying := llm.Retrying{
		Next: base, Attempts: llmAttempts, Backoff: llmBackoff, MaxWait: llmMaxWait, Sleep: llm.Sleep,
		OnWait: func(d time.Duration, err error) {
			a.Reporter().Warn("%v; trying again in %s", err, d.Round(time.Second))
		},
	}
	return llm.Recording{
		Next: retrying, Now: a.Env.Now,
		Sink: func(r llm.Record) error { return jsonl.Append(ws.UsageFile(), r) },
	}, nil
}

func (a *App) Author(backend llm.Backend) (author.Author, error) {
	st, err := a.Store()
	if err != nil {
		return author.Author{}, err
	}
	cfg, err := a.Config()
	if err != nil {
		return author.Author{}, err
	}
	out := author.Author{LLM: backend, Notes: st, Schema: prompt.Schema(), AllowRank: cfg.Author.AllowRank}
	if cfg.Author.AllowRank > 0 {
		checker, err := a.Vocabulary()
		if err != nil {
			return author.Author{}, err
		}
		out.Vocabulary = checker
	}
	return out, nil
}

func (a *App) AuthorSystem() (string, error) {
	st, err := a.Store()
	if err != nil {
		return "", err
	}
	set, err := st.Load()
	if err != nil {
		return "", err
	}
	list, err := a.Ranking()
	if err != nil {
		return "", err
	}
	cfg, err := a.Config()
	if err != nil {
		return "", err
	}
	ids := list.IDs()
	vocabulary := ids[:min(cfg.Author.AllowRank, promptVocabulary, len(ids))]
	return prompt.System(author.PickExamples(set.All(), exampleCount), vocabulary), nil
}

func (a *App) AuthorFailure(err error) error {
	if limit, ok := llm.AsUsageLimit(err); ok {
		hint := "your Claude plan's usage limit was reached; the finished notes are saved, so run the command again later"
		if !limit.ResetAt.IsZero() {
			hint = fmt.Sprintf("your Claude plan's usage limit was reached and resets at %s; the finished notes are saved, so run the command again then", limit.ResetAt.Local().Format("15:04 on 2 Jan"))
		}
		return failure.Temporary(hint, err)
	}
	if llm.AsAuth(err) {
		return failure.NotSetUp("sign in with `claude`, then run the command again", err)
	}
	if llm.AsUnavailable(err) {
		return failure.NotSetUp("install Claude Code (https://claude.com/claude-code); it writes the notes", err)
	}
	var transient *llm.TransientError
	var limited *llm.RateLimitedError
	if errors.As(err, &transient) || errors.As(err, &limited) {
		return failure.Temporary("the service is busy; the finished notes are saved, so run the command again in a few minutes", err)
	}
	return err
}
