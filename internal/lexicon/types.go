package lexicon

import (
	"math"
	"net/url"
	"slices"
)

type Sense struct {
	Gloss string   `json:"gloss"`
	Tags  []string `json:"tags,omitempty"`
	MSA   bool     `json:"msa"`
}

type Class uint8

const (
	Nominal Class = iota
	Definite
	Construct
	Perfect
	Imperfect
	Imperative
	Particle
)

type Form struct {
	Skeleton string
	Class    Class
}

type Entry struct {
	Title           string   `json:"title"`
	Pos             string   `json:"pos"`
	Canonical       string   `json:"canonical"`
	Etymology       string   `json:"etymology,omitempty"`
	Gender          string   `json:"gender,omitempty"`
	Plurals         []string `json:"plurals,omitempty"`
	Feminine        string   `json:"feminine,omitempty"`
	FemininePlurals []string `json:"feminine_plurals,omitempty"`
	Elative         string   `json:"elative,omitempty"`
	NonPast         string   `json:"non_past,omitempty"`
	VerbalNouns     []string `json:"verbal_nouns,omitempty"`
	VerbForm        string   `json:"verb_form,omitempty"`
	Root            string   `json:"root,omitempty"`
	Senses          []Sense  `json:"senses"`
	Forms           []Form   `json:"-"`
}

func (e *Entry) MSA() bool {
	return slices.ContainsFunc(e.Senses, func(s Sense) bool { return s.MSA })
}

func (e *Entry) MSASenses() []Sense {
	return slices.DeleteFunc(slices.Clone(e.Senses), func(s Sense) bool { return !s.MSA })
}

func (e *Entry) URL() string {
	return "https://en.wiktionary.org/wiki/" + url.PathEscape(e.Title) + "#Arabic"
}

func (e *Entry) Weight() float64 {
	msa := 0
	for _, s := range e.Senses {
		if s.MSA {
			msa++
		}
	}
	w := math.Log1p(float64(msa))
	if msa == 0 {
		w -= 10
	}
	if e.Pos == "name" {
		w -= 5
	}
	return w
}

type Lemma struct {
	ID      string   `json:"id"`
	Entries []*Entry `json:"entries"`
}

func (l *Lemma) Main() *Entry { return l.Entries[0] }

func (l *Lemma) Pos() []string {
	var out []string
	for _, e := range l.Entries {
		if !slices.Contains(out, e.Pos) {
			out = append(out, e.Pos)
		}
	}
	return out
}

func (l *Lemma) Has(pos string) bool {
	return slices.ContainsFunc(l.Entries, func(e *Entry) bool { return e.Pos == pos })
}

func (l *Lemma) MSA() bool {
	return slices.ContainsFunc(l.Entries, (*Entry).MSA)
}

func (l *Lemma) Name() bool {
	return !slices.ContainsFunc(l.Entries, func(e *Entry) bool { return e.Pos != "name" })
}

func (l *Lemma) Gloss() string {
	for _, e := range l.Entries {
		for _, s := range e.Senses {
			if s.MSA {
				return s.Gloss
			}
		}
	}
	return l.Main().Senses[0].Gloss
}
