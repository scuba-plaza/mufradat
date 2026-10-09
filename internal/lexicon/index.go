package lexicon

import (
	"compress/gzip"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/scuba-plaza/mufradat/internal/jsonl"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

type Index struct {
	lemmas []*Lemma
	byID   map[string]*Lemma
}

func NewIndex(lemmas []*Lemma) *Index {
	ix := &Index{lemmas: lemmas, byID: make(map[string]*Lemma, len(lemmas))}
	for _, l := range lemmas {
		ix.byID[l.ID] = l
	}
	return ix
}

func Load(r io.Reader) (*Index, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("lexicon: %w", err)
	}
	defer gz.Close()
	lemmas, err := jsonl.Decode[*Lemma](gz)
	if err != nil {
		return nil, fmt.Errorf("lexicon: %w", err)
	}
	return NewIndex(lemmas), nil
}

func (ix *Index) Len() int { return len(ix.lemmas) }

func (ix *Index) Lemmas() []*Lemma { return ix.lemmas }

func (ix *Index) Lemma(id string) (*Lemma, bool) {
	l, ok := ix.byID[id]
	return l, ok
}

type Candidate struct {
	Lemma   *Lemma
	Entries []*Entry
	Primary bool
}

func (c Candidate) Main() *Entry { return c.Entries[0] }

func (c Candidate) Senses() []Sense {
	var out []Sense
	for _, e := range c.Entries {
		out = append(out, e.MSASenses()...)
	}
	return out
}

func NormalizeWord(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == tashkeel.Tatweel || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

type query struct {
	word     string
	skeleton string
	loose    string
}

func newQuery(word string) query {
	word = NormalizeWord(word)
	skeleton := tashkeel.Skeleton(word)
	return query{word: word, skeleton: skeleton, loose: tashkeel.Fold(skeleton)}
}

func (q query) fits(canonical string, loose bool) bool {
	skeleton := tashkeel.Skeleton(canonical)
	if loose {
		return tashkeel.Fold(skeleton) == q.loose && tashkeel.CompatibleCitation(tashkeel.Fold(canonical), tashkeel.Fold(q.word))
	}
	return skeleton == q.skeleton && tashkeel.CompatibleCitation(canonical, q.word)
}

func (q query) fitsTitle(e *Entry, loose bool) bool {
	skeleton := tashkeel.Skeleton(e.Title)
	want := q.skeleton
	if loose {
		skeleton = tashkeel.Fold(skeleton)
		want = q.loose
	}
	if e.Title == "" || skeleton != want {
		return false
	}
	if !strings.ContainsFunc(q.word, tashkeel.IsMark) {
		return true
	}
	for _, w := range strings.Fields(e.Canonical) {
		if q.fits(w, loose) || q.fits(withoutArticle(w), loose) {
			return true
		}
	}
	return false
}

func withoutArticle(word string) string {
	letters := tashkeel.Letters(word)
	if len(letters) > 3 && tashkeel.Skeleton(letters[0]+letters[1]) == "ال" {
		return strings.Join(letters[2:], "")
	}
	return word
}

type pass struct {
	loose bool
	title bool
}

var passes = [...]pass{{false, false}, {true, false}, {false, true}, {true, true}}

func (q query) matches(e *Entry, p pass) bool {
	if p.title {
		return q.fitsTitle(e, p.loose)
	}
	return q.fits(e.Canonical, p.loose)
}

func (ix *Index) Find(word string) []Candidate {
	q := newQuery(word)
	if q.skeleton == "" {
		return nil
	}
	for _, p := range passes {
		var out []Candidate
		for _, l := range ix.lemmas {
			out = append(out, candidatesOf(q, l, p)...)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func candidatesOf(q query, l *Lemma, p pass) []Candidate {
	var out []Candidate
	for _, e := range l.Entries {
		if e.Pos == "name" || !e.MSA() || !q.matches(e, p) {
			continue
		}
		if i := slices.IndexFunc(out, func(c Candidate) bool { return c.Main().Pos == e.Pos }); i >= 0 {
			out[i].Entries = append(out[i].Entries, e)
			continue
		}
		out = append(out, Candidate{Lemma: l, Entries: []*Entry{e}, Primary: e == l.Entries[0]})
	}
	return out
}
