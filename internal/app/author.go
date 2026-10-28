package app

import (
	"errors"
	"fmt"

	"github.com/scuba-plaza/mufradat/internal/author"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/prompt"
)

const (
	exampleCount     = 8
	promptVocabulary = 500
)

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
	if llm.AsAuth(err) {
		return failure.NotSetUp(fmt.Sprintf("check the key with `%s doctor`, or import another with `%s auth mistral --import FILE`; the finished notes are saved", a.CommandPrefix(), a.CommandPrefix()), err)
	}
	var limited *llm.RateLimitedError
	var transient *llm.TransientError
	if errors.As(err, &limited) || errors.As(err, &transient) {
		return failure.Temporary("the service is busy; the finished notes are saved, so run the command again in a few minutes", err)
	}
	return err
}
