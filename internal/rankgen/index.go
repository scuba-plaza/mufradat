package rankgen

import (
	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/morph"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

type Index struct {
	Lemmas []*lexicon.Lemma
	byKey  map[string]int
	table  *morph.Table
}

func key(l *lexicon.Lemma) string {
	return tashkeel.LexKey(l.Main().Canonical)
}

func NewIndex(lemmas []*lexicon.Lemma) *Index {
	facts := make([]morph.Facts, len(lemmas))
	for i, l := range lemmas {
		facts[i] = morph.FactsOf(l)
	}
	ix := &Index{Lemmas: lemmas, byKey: map[string]int{}, table: morph.NewTable(facts)}
	for i, l := range lemmas {
		ix.byKey[key(l)] = i
		for _, e := range l.Entries {
			for _, f := range e.Forms {
				ix.table.Add(i, f)
			}
		}
	}
	return ix
}

func (ix *Index) Lookup(word string) (int, bool) {
	i, ok := ix.byKey[tashkeel.LexKey(word)]
	return i, ok
}

func (ix *Index) Matches(word string) []morph.Match { return ix.table.Matches(word) }

func (ix *Index) Analyze(word string) []int { return ix.table.Analyze(word) }
