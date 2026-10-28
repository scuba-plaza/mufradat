package author

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/prompt"
)

func existing() note.Note {
	return note.Note{
		ID: "n_aaaaaaaaaaaa", Seq: 4, Rank: 7, Origin: note.OriginRanked, Lemma: "كُتَاب", POS: note.Noun, Gender: "m",
		English: "book", Example: "قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ.", ExampleEN: "I read the book at home.",
	}
}

func rewriterWith(answer func(call int, cards []prompt.Card) (json.RawMessage, error)) (Rewriter, *fakeLLM) {
	f := &fakeLLM{answer: answer}
	return Rewriter{LLM: f, Schema: prompt.Schema(), System: "system"}, f
}

func TestRewriteKeepsIdentityAndAppliesTheCard(t *testing.T) {
	r, f := rewriterWith(func(call int, cards []prompt.Card) (json.RawMessage, error) {
		return finish(cards, func(c *prompt.Card) {
			c.Arabic = "كِتَاب"
			c.English = "book"
		}), nil
	})
	got, err := r.Rewrite(context.Background(), existing(), "- كُتَاب in the lemma: CAMeL does not allow these vowels")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "n_aaaaaaaaaaaa" || got.Rank != 7 || got.Seq != 4 || got.Gender != "m" || got.Origin != note.OriginRanked {
		t.Fatalf("identity lost: %+v", got)
	}
	if got.Lemma != "كِتَاب" || got.Example != goodExample {
		t.Fatalf("card not applied: %+v", got)
	}
	if len(f.prompts) != 1 || !strings.Contains(f.prompts[0], "CAMeL does not allow these vowels") || !strings.Contains(f.prompts[0], `"ref":1`) {
		t.Fatalf("prompt = %q", f.prompts)
	}
}

func TestRewriteRejectsAChangedHeadwordAndRetries(t *testing.T) {
	r, f := rewriterWith(func(call int, cards []prompt.Card) (json.RawMessage, error) {
		return finish(cards, func(c *prompt.Card) {
			if call == 0 {
				c.Arabic = "قَلَم"
			} else {
				c.Arabic = "كِتَاب"
			}
		}), nil
	})
	got, err := r.Rewrite(context.Background(), existing(), "check it")
	if err != nil || got.Lemma != "كِتَاب" {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if len(f.prompts) != 2 || !strings.Contains(f.prompts[1], "A previous answer was rejected: the headword must stay") {
		t.Fatalf("second prompt lacks the rejection: %q", f.prompts)
	}
}

func TestRewriteGivesUpAfterTheAttempts(t *testing.T) {
	r, f := rewriterWith(func(call int, cards []prompt.Card) (json.RawMessage, error) {
		return finish(cards, func(c *prompt.Card) { c.Example = "قَرَأَ [الْوَلَدُ]." }), nil
	})
	_, err := r.Rewrite(context.Background(), existing(), "check it")
	if err == nil || !strings.Contains(err.Error(), "no acceptable version after 2 attempts") || !strings.Contains(err.Error(), "words") {
		t.Fatalf("err = %v", err)
	}
	if len(f.prompts) != 2 {
		t.Fatalf("asked %d times", len(f.prompts))
	}
}

func TestRewriteDoesNotRetryAServiceFailure(t *testing.T) {
	r, f := rewriterWith(func(call int, cards []prompt.Card) (json.RawMessage, error) {
		return nil, &llm.CallError{Message: "the service answered 400"}
	})
	_, err := r.Rewrite(context.Background(), existing(), "check it")
	if !isCallError(err) || len(f.prompts) != 1 {
		t.Fatalf("err = %v, asked %d times", err, len(f.prompts))
	}
}

func TestRewriteRetriesAnUnreadableAnswer(t *testing.T) {
	r, f := rewriterWith(func(call int, cards []prompt.Card) (json.RawMessage, error) {
		if call == 0 {
			return json.RawMessage(`{"cards":[]}`), nil
		}
		return finish(cards, func(c *prompt.Card) { c.Arabic = "كِتَاب" }), nil
	})
	got, err := r.Rewrite(context.Background(), existing(), "check it")
	if err != nil || got.Lemma != "كِتَاب" || len(f.prompts) != 2 {
		t.Fatalf("got %+v, err %v, asked %d", got, err, len(f.prompts))
	}
}

func TestRewriteRetriesAnUnusableAnswerAndTellsTheModelWhy(t *testing.T) {
	r, f := rewriterWith(func(call int, cards []prompt.Card) (json.RawMessage, error) {
		if call == 0 {
			return nil, &llm.BadOutputError{Message: "the reply was cut off"}
		}
		return finish(cards, func(c *prompt.Card) { c.Arabic = "كِتَاب" }), nil
	})
	got, err := r.Rewrite(context.Background(), existing(), "check it")
	if err != nil || got.Lemma != "كِتَاب" {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if len(f.prompts) != 2 || !strings.Contains(f.prompts[1], "A previous answer was rejected: unusable answer: the reply was cut off") {
		t.Fatalf("second prompt lacks the reason: %q", f.prompts)
	}
}

func TestRewriteStopsAtOnceOnACredentialOrQuotaFailureAndKeepsItsType(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		is   func(error) bool
		text string
	}{
		{"credentials", &llm.AuthError{Message: "bad key"}, llm.AsAuth, "the service rejected the credentials: bad key"},
		{"quota", &llm.RateLimitedError{Message: "slow down"}, func(err error) bool { var e *llm.RateLimitedError; return errors.As(err, &e) }, "rate limited: slow down"},
	} {
		r, f := rewriterWith(func(call int, cards []prompt.Card) (json.RawMessage, error) { return nil, c.err })
		_, err := r.Rewrite(context.Background(), existing(), "check it")
		if err == nil || !c.is(err) || err.Error() != c.text || len(f.prompts) != 1 {
			t.Errorf("%s: err = %v, asked %d times", c.name, err, len(f.prompts))
		}
	}
}

func TestRewriteStopsWhenTheCallerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	boom := errors.New("connection reset")
	r, f := rewriterWith(func(call int, cards []prompt.Card) (json.RawMessage, error) {
		cancel()
		return nil, boom
	})
	_, err := r.Rewrite(ctx, existing(), "check it")
	if !errors.Is(err, boom) || len(f.prompts) != 1 {
		t.Fatalf("err = %v, asked %d times: a cancelled run must not ask again", err, len(f.prompts))
	}
}
