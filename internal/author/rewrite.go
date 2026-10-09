package author

import (
	"context"
	"errors"
	"fmt"

	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/prompt"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

type Rewriter struct {
	LLM      llm.Backend
	Schema   []byte
	System   string
	Attempts int
}

func (r Rewriter) Rewrite(ctx context.Context, n note.Note, feedback string) (note.Note, error) {
	attempts := r.Attempts
	if attempts <= 0 {
		attempts = DefaultAttempts
	}
	var rejection error
	for attempt := 0; attempt < attempts; attempt++ {
		reason := ""
		if rejection != nil {
			reason = rejection.Error()
		}
		resp, err := r.LLM.Complete(ctx, llm.Request{
			System: r.System,
			Prompt: prompt.Rewrite(prompt.CardOf(n, 1), feedback, reason),
			Schema: r.Schema,
		})
		if err != nil {
			if fatal(err) || ctx.Err() != nil || isCallError(err) {
				return note.Note{}, err
			}
			rejection = err
			continue
		}
		cards, err := prompt.ParseCards(resp.Output)
		if err != nil {
			rejection = err
			continue
		}
		card, ok := cards[1]
		if !ok {
			rejection = errors.New("the answer had no card with ref 1")
			continue
		}
		fresh := Apply(n, card)
		if err := checkRewrite(n, fresh); err != nil {
			rejection = err
			continue
		}
		return fresh, nil
	}
	return note.Note{}, fmt.Errorf("no acceptable version after %d attempts: %w", attempts, rejection)
}

func checkRewrite(old, fresh note.Note) error {
	if err := note.ValidateContent(fresh); err != nil {
		return err
	}
	if tashkeel.Skeleton(fresh.Lemma) != tashkeel.Skeleton(old.Lemma) {
		return fmt.Errorf("the headword must stay %s, not %s", old.Lemma, fresh.Lemma)
	}
	return Gate(Target{}, fresh)
}
