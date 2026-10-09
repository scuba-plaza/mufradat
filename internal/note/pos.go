package note

import "slices"

type POS string

const (
	Noun     POS = "noun"
	Verb     POS = "verb"
	Adj      POS = "adj"
	Adv      POS = "adv"
	Prep     POS = "prep"
	Conj     POS = "conj"
	Particle POS = "particle"
	Pron     POS = "pron"
	Num      POS = "num"
	Intj     POS = "intj"
	Phrase   POS = "phrase"
	Det      POS = "det"
)

var AllPOS = []POS{Noun, Verb, Adj, Adv, Prep, Conj, Particle, Pron, Num, Intj, Phrase, Det}

var posLabels = map[POS]string{
	Noun: "noun", Verb: "verb", Adj: "adjective", Adv: "adverb", Prep: "preposition",
	Conj: "conjunction", Particle: "particle", Pron: "pronoun", Num: "number",
	Intj: "interjection", Phrase: "phrase", Det: "determiner",
}

var wiktionaryPOS = map[string]POS{
	"noun": Noun, "verb": Verb, "adj": Adj, "adv": Adv, "prep": Prep, "conj": Conj,
	"particle": Particle, "pron": Pron, "num": Num, "intj": Intj, "phrase": Phrase,
	"det": Det, "article": Det, "prep_phrase": Prep,
}

func (p POS) Valid() bool { return slices.Contains(AllPOS, p) }

func (p POS) Label() string {
	if l, ok := posLabels[p]; ok {
		return l
	}
	return string(p)
}

func POSFromWiktionary(s string) (POS, bool) {
	p, ok := wiktionaryPOS[s]
	return p, ok
}

func POSStrings() []string {
	out := make([]string, len(AllPOS))
	for i, p := range AllPOS {
		out[i] = string(p)
	}
	return out
}
