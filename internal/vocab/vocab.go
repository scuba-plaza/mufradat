package vocab

import (
	"fmt"
	"io"

	"github.com/scuba-plaza/mufradat/data"
	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/morph"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/ranking"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

type Checker struct {
	lex    *lexicon.Index
	table  *morph.Table
	ranked int
}

type Offender struct {
	Word  string
	Lemma string
	Rank  int
}

func (o Offender) String() string {
	if o.Rank == 0 {
		return fmt.Sprintf("%s (from %s, not among the 5000 most frequent words)", o.Word, o.Lemma)
	}
	return fmt.Sprintf("%s (from %s, frequency rank %d)", o.Word, o.Lemma, o.Rank)
}

func Embedded(lex *lexicon.Index, list *ranking.List) (*Checker, error) {
	return New(lex, list, data.Forms())
}

func New(lex *lexicon.Index, list *ranking.List, forms io.Reader) (*Checker, error) {
	lemmas := lex.Lemmas()
	if list.Len() > len(lemmas) {
		return nil, fmt.Errorf("vocab: the ranking has %d words but the lexicon only %d", list.Len(), len(lemmas))
	}
	for i := range list.Len() {
		if id, _ := list.ID(i + 1); lemmas[i].ID != id {
			return nil, fmt.Errorf("vocab: lexicon entry %d is %s but rank %d is %s", i, lemmas[i].ID, i+1, id)
		}
	}
	facts := make([]morph.Facts, len(lemmas))
	for i, l := range lemmas {
		facts[i] = morph.FactsOf(l)
	}
	table, err := morph.Decode(forms, facts)
	if err != nil {
		return nil, err
	}
	return &Checker{lex: lex, table: table, ranked: list.Len()}, nil
}

func (c *Checker) rankOf(lemma int) int {
	if lemma < c.ranked {
		return lemma + 1
	}
	return 0
}

func (c *Checker) Beyond(example string, ceiling int) []Offender {
	text := example
	if parts, err := note.SplitExample(example); err == nil {
		text = parts.Before + " " + parts.After
	}
	var out []Offender
	seen := map[string]bool{}
	for _, word := range tashkeel.Words(text) {
		skeleton := tashkeel.Skeleton(word)
		if skeleton == "" || seen[word] {
			continue
		}
		seen[word] = true
		matches := c.table.Matches(skeleton)
		if len(matches) == 0 {
			continue
		}
		best, within := matches[0], false
		for _, m := range matches {
			if r := c.rankOf(m.Lemma); r > 0 && r <= ceiling {
				within = true
				break
			}
			if m.Quality > best.Quality {
				best = m
			}
		}
		if within {
			continue
		}
		out = append(out, Offender{Word: word, Lemma: c.lex.Lemmas()[best.Lemma].ID, Rank: c.rankOf(best.Lemma)})
	}
	return out
}
