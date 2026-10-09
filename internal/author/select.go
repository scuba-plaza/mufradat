package author

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/note"
)

type AmbiguousError struct {
	Word       string
	Candidates []lexicon.Candidate
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%s matches %d entries", e.Word, len(e.Candidates))
}

type NotFoundError struct{ Word string }

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no Modern Standard Arabic entry matches %s", e.Word)
}

type Chooser interface {
	Choose(word string, candidates []lexicon.Candidate) (int, error)
}

type Selector struct {
	Pick    int
	Entries map[string]int
	POS     note.POS
	Yes     bool
	Chooser Chooser
}

func ParseEntrySelector(s string) (word string, n int, err error) {
	i := strings.LastIndex(s, "#")
	if i <= 0 || i == len(s)-1 {
		return "", 0, fmt.Errorf("%q is not WORD#N, for example كتاب#2", s)
	}
	n, err = strconv.Atoi(s[i+1:])
	if err != nil || n < 1 {
		return "", 0, fmt.Errorf("%q is not WORD#N: N must be a positive number", s)
	}
	return lexicon.NormalizeWord(s[:i]), n, nil
}

func (s Selector) Select(word string, found []lexicon.Candidate) (lexicon.Candidate, error) {
	cands := found
	if s.POS != "" {
		cands = nil
		for _, c := range found {
			if p, ok := note.POSFromWiktionary(c.Main().Pos); ok && p == s.POS {
				cands = append(cands, c)
			}
		}
	}
	if len(cands) == 0 {
		return lexicon.Candidate{}, &NotFoundError{Word: word}
	}
	choice := func(n int) (lexicon.Candidate, error) {
		if n < 1 || n > len(cands) {
			return lexicon.Candidate{}, fmt.Errorf("%s has %d entries; there is no entry %d", word, len(cands), n)
		}
		return cands[n-1], nil
	}
	if n, ok := s.Entries[lexicon.NormalizeWord(word)]; ok {
		return choice(n)
	}
	if s.Pick > 0 {
		return choice(s.Pick)
	}
	if len(cands) == 1 {
		return cands[0], nil
	}
	if s.Yes {
		return cands[0], nil
	}
	if s.Chooser != nil {
		n, err := s.Chooser.Choose(word, cands)
		if err != nil {
			return lexicon.Candidate{}, err
		}
		return choice(n)
	}
	return lexicon.Candidate{}, &AmbiguousError{Word: word, Candidates: cands}
}

func Describe(c lexicon.Candidate, glosses int) string {
	var parts []string
	for _, s := range c.Senses() {
		if len(parts) == glosses {
			break
		}
		parts = append(parts, s.Gloss)
	}
	return fmt.Sprintf("%s  %s  %s", c.Main().Canonical, c.Main().Pos, strings.Join(parts, "; "))
}
