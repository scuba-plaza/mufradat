package author

import (
	"fmt"
	"slices"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/ranking"
)

const (
	maxEntries = 3
	maxPlurals = 2
)

type Target struct {
	Draft   note.Note
	Entries []*lexicon.Entry
	Notice  string
	Typed   string
	Pinned  bool
}

func DraftNote(e *lexicon.Entry, rank int, origin note.Origin) (note.Note, bool) {
	pos, ok := note.POSFromWiktionary(e.Pos)
	if !ok {
		return note.Note{}, false
	}
	n := note.Note{
		Rank: rank, Origin: origin, Lemma: e.Canonical, POS: pos,
		Gender: e.Gender, VerbForm: e.VerbForm, Root: e.Root,
	}
	add := func(label, arabic string) {
		if arabic != "" {
			n.Forms = append(n.Forms, note.Form{Label: label, Arabic: arabic})
		}
	}
	switch pos {
	case note.Noun:
		for i, p := range e.Plurals {
			if i == maxPlurals {
				break
			}
			add("pl.", p)
		}
	case note.Verb:
		add("pres.", e.NonPast)
		if len(e.VerbalNouns) > 0 {
			add("masdar", e.VerbalNouns[0])
		}
	case note.Adj:
		add("f.", e.Feminine)
		if len(e.Plurals) > 0 {
			add("pl.", e.Plurals[0])
		}
	case note.Pron, note.Det:
		add("f.", e.Feminine)
	}
	return n, true
}

func NextRanked(set *note.Set, list *ranking.List, ix *lexicon.Index, count int) []Target {
	taken := map[int]bool{}
	lexemes := map[string]bool{}
	for _, n := range set.All() {
		if n.Rank > 0 {
			taken[n.Rank] = true
		}
		lexemes[n.LexemeKey()] = true
	}
	var out []Target
	for rank := 1; rank <= list.Len() && len(out) < count; rank++ {
		if taken[rank] {
			continue
		}
		id, _ := list.ID(rank)
		lemma, ok := ix.Lemma(id)
		if !ok || !lemma.MSA() || lemma.Name() {
			continue
		}
		main := lemma.Main()
		draft, ok := DraftNote(main, rank, note.OriginRanked)
		if !ok || lexemes[draft.LexemeKey()] {
			continue
		}
		lexemes[draft.LexemeKey()] = true
		out = append(out, Target{Draft: draft, Entries: limitEntries(lemma.Entries)})
	}
	return out
}

func limitEntries(entries []*lexicon.Entry) []*lexicon.Entry {
	var out []*lexicon.Entry
	for _, e := range entries {
		if e.Pos != "name" && e.MSA() {
			out = append(out, e)
		}
		if len(out) == maxEntries {
			break
		}
	}
	return out
}

type Placement struct {
	Target   Target
	Existing *note.Note
}

func Place(c lexicon.Candidate, typed string, set *note.Set, list *ranking.List) (Placement, bool) {
	e := c.Main()
	rank := 0
	if c.Primary {
		if r := list.Rank(c.Lemma.ID); r > 0 {
			if _, taken := set.ByRank(r); !taken {
				rank = r
			}
		}
	}
	draft, ok := DraftNote(e, rank, note.OriginNamed)
	if !ok {
		return Placement{}, false
	}
	if existing, found := set.ByLexeme(draft.LexemeKey()); found {
		return Placement{Existing: &existing}, true
	}
	entries := slices.Clone(c.Entries)
	return Placement{Target: Target{
		Draft: draft, Entries: entries, Typed: typed, Pinned: true,
		Notice: fmt.Sprintf("The learner asked for this entry in particular (%s, %s), the only Wiktionary entry listed for this card. Write the card for it: keep its headword, part of speech and forms, and do not switch to another entry of the same spelling.", e.Pos, e.Canonical),
	}}, true
}

func Dedupe(targets []Target) []Target {
	seen := map[string]bool{}
	var out []Target
	for _, t := range targets {
		key := t.Draft.LexemeKey()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}
