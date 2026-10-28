package lexicon

import (
	"math"
	"testing"
)

func TestWeightGrowsWithTheMSASensesAndSinksForEntriesThatAreNotMSAOrAreNames(t *testing.T) {
	notMSA := func(e *Entry) *Entry {
		e.Senses = append(e.Senses, Sense{Gloss: "dialect only", MSA: false}, Sense{Gloss: "also dialect", MSA: false})
		return e
	}
	name := func(e *Entry) *Entry { e.Pos = "name"; return e }
	for _, c := range []struct {
		label string
		entry *Entry
		want  float64
	}{
		{"one sense", entry("كِتَاب", "noun", "book"), math.Ln2},
		{"three senses", entry("كِتَاب", "noun", "book", "writing", "letter"), math.Log(4)},
		{"senses outside MSA add nothing", notMSA(entry("كِتَاب", "noun", "book")), math.Ln2},
		{"no MSA sense at all", notMSA(entry("كِتَاب", "noun")), -10},
		{"a name", name(entry("مُحَمَّد", "noun", "Muhammad", "a given name")), math.Log(3) - 5},
		{"a name with no MSA sense", name(notMSA(entry("مُحَمَّد", "noun"))), -15},
	} {
		if got := c.entry.Weight(); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: weight %v, want %v", c.label, got, c.want)
		}
	}
}

func TestALemmaAnswersForEveryPartOfSpeechOfItsEntries(t *testing.T) {
	l := lemma(entry("مَاء", "noun", "water"), entry("مَاءَ", "verb", "to meow"), entry("مَاء", "noun", "a drink"))
	for pos, want := range map[string]bool{"noun": true, "verb": true, "prep": false, "": false} {
		if got := l.Has(pos); got != want {
			t.Errorf("Has(%q) = %v, want %v", pos, got, want)
		}
	}
}
