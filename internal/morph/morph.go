package morph

import (
	"sort"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

type Facts struct {
	Verb bool
	Prep bool
}

func FactsOf(l *lexicon.Lemma) Facts {
	return Facts{Verb: l.Main().Pos == "verb", Prep: l.Has("prep")}
}

type candidate struct {
	lemma int
	class lexicon.Class
}

type Table struct {
	facts []Facts
	forms map[string][]candidate
	loose map[string][]candidate
}

func NewTable(facts []Facts) *Table {
	return &Table{facts: facts, forms: map[string][]candidate{}, loose: map[string][]candidate{}}
}

func (t *Table) Add(lemma int, f lexicon.Form) {
	c := candidate{lemma: lemma, class: f.Class}
	if contains(t.forms[f.Skeleton], c) {
		return
	}
	t.forms[f.Skeleton] = append(t.forms[f.Skeleton], c)
	if k := tashkeel.Fold(f.Skeleton); k != f.Skeleton {
		t.loose[k] = append(t.loose[k], c)
	}
}

func contains(list []candidate, c candidate) bool {
	for _, x := range list {
		if x == c {
			return true
		}
	}
	return false
}

func (t *Table) lookup(stem string) []candidate {
	if c := t.forms[stem]; len(c) > 0 {
		return c
	}
	folded := tashkeel.Fold(stem)
	var out []candidate
	out = append(out, t.loose[folded]...)
	out = append(out, t.forms[folded]...)
	return out
}

var (
	conjunctions = []string{"", "و", "ف"}
	particles    = []string{"", "ب", "ك", "ل", "س"}
	enclitics    = []string{"", "ه", "ها", "هم", "هما", "هن", "ك", "كم", "كما", "كن", "ي", "ني", "نا"}
)

func stemVariants(stem string, enclitic bool) []string {
	out := []string{stem}
	if !enclitic {
		return out
	}
	trimmed, last := trimLast(stem)
	switch last {
	case 'ت':
		out = append(out, trimmed+"ة")
	case 'ي':
		out = append(out, trimmed+"ى")
	case 'و':
		out = append(out, stem+"ا")
	case 'ا':
		out = append(out, trimmed+"ى")
	case 'ئ', 'ؤ':
		out = append(out, trimmed+"ء")
	}
	return out
}

func trimLast(s string) (string, rune) {
	r := []rune(s)
	if len(r) == 0 {
		return s, 0
	}
	return string(r[:len(r)-1]), r[len(r)-1]
}

type Match struct {
	Lemma   int
	Quality float64
}

const (
	cliticPenalty  = 0.25
	variantPenalty = 0.8
	loosePenalty   = 0.3
)

func (t *Table) Analyze(word string) []int {
	var out []int
	for _, m := range t.Matches(word) {
		out = append(out, m.Lemma)
	}
	return out
}

func (t *Table) Matches(word string) []Match {
	best := map[int]float64{}
	for _, c := range conjunctions {
		w1, ok := strings.CutPrefix(word, c)
		if !ok {
			continue
		}
		for _, p := range particles {
			w2, ok := strings.CutPrefix(w1, p)
			if !ok || w2 == "" {
				continue
			}
			type base struct {
				text    string
				quality float64
			}
			bases := []base{{w2, 1}}
			if p == "ل" && strings.HasPrefix(w2, "ل") {
				bases = append(bases, base{"ا" + w2, 1})
			}
			for _, b := range bases {
				for _, enc := range enclitics {
					stem, ok := strings.CutSuffix(b.text, enc)
					if !ok || stem == "" {
						continue
					}
					q := b.quality
					for _, affix := range []string{c, p, enc} {
						if affix != "" {
							q *= cliticPenalty
						}
					}
					for vi, sv := range stemVariants(stem, enc != "") {
						vq := q
						if vi > 0 {
							vq *= variantPenalty
						}
						strict := t.forms[sv]
						for _, cand := range t.lookup(sv) {
							if !t.allowed(cand, c != "" || p != "" || enc != "", p, enc != "", sv) {
								continue
							}
							mq := vq
							if len(strict) == 0 {
								mq *= loosePenalty
							}
							if mq > best[cand.lemma] {
								best[cand.lemma] = mq
							}
						}
					}
				}
			}
		}
	}
	out := make([]Match, 0, len(best))
	for l, q := range best {
		out = append(out, Match{Lemma: l, Quality: q})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Lemma < out[b].Lemma })
	return out
}

func (t *Table) allowed(c candidate, affixed bool, particle string, enclitic bool, stem string) bool {
	f := t.facts[c.lemma]
	verb := f.Verb || c.class == lexicon.Perfect || c.class == lexicon.Imperfect || c.class == lexicon.Imperative
	if enclitic && c.class == lexicon.Definite {
		return false
	}
	switch particle {
	case "ب", "ك":
		if verb {
			return false
		}
	case "س":
		if c.class != lexicon.Imperfect {
			return false
		}
	case "ل":
		if verb && c.class != lexicon.Imperfect {
			return false
		}
	}
	if affixed && len([]rune(stem)) < 2 && !f.Prep {
		return false
	}
	return true
}
