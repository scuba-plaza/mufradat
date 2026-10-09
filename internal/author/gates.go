package author

import (
	"fmt"
	"slices"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

const (
	minExampleWords = 3
	maxExampleWords = 12
)

var functionWords = []note.POS{note.Adv, note.Conj, note.Det, note.Intj, note.Num, note.Particle, note.Phrase, note.Prep, note.Pron}

func posClass(p note.POS) string {
	if slices.Contains(functionWords, p) {
		return "function"
	}
	return string(p)
}

func Gate(t Target, n note.Note) error {
	words := len(tashkeel.Words(note.PlainExample(n.Example)))
	if words < minExampleWords || words > maxExampleWords {
		return fmt.Errorf("the example has %d words; write %d to %d", words, minExampleWords, maxExampleWords-3)
	}
	if !t.Pinned {
		return nil
	}
	if posClass(n.POS) != posClass(t.Draft.POS) {
		return fmt.Errorf("the learner asked for the %s %s, but the card is for a %s; write the card for the %s", t.Draft.POS, t.Draft.Lemma, n.POS, t.Draft.POS)
	}
	if !tashkeel.CompatibleCitation(n.Lemma, t.Draft.Lemma) {
		return fmt.Errorf("the learner asked for the headword %s, but the card's headword is %s; keep %s", t.Draft.Lemma, n.Lemma, t.Draft.Lemma)
	}
	return nil
}
