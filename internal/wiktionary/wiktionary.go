package wiktionary

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/arabic"
	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

var skippedPos = map[string]bool{
	"character": true, "symbol": true, "punct": true, "suffix": true, "prefix": true,
	"interfix": true, "root": true, "proverb": true, "infix": true, "circumfix": true,
}

var nonMSATags = map[string]bool{
	"obsolete": true, "archaic": true, "rare": true, "Classical": true, "dated": true,
	"colloquial": true, "dialectal": true, "regional": true, "nonstandard": true, "slang": true,
	"uncommon": true, "proscribed": true, "post-Classical": true, "poetic": true,
	"slur": true, "offensive": true, "vulgar": true,
	"Egypt": true, "Egyptian": true, "al-Andalus": true, "Al-Andalus": true, "Yemen": true,
	"Morocco": true, "Iraq": true, "Levantine": true, "South-Levantine": true, "Kuwait": true,
	"Palestine": true, "Palestinian": true, "Mauritania": true, "Israel": true, "Jazan": true,
	"Cyprus": true, "Nigeria": true, "Iran": true, "Iranian": true, "Gulf": true, "Tunisia": true,
	"Algeria": true, "Sudan": true, "Syria": true, "Lebanon": true, "Hejaz": true, "Saudi-Arabia": true,
}

var skippedSenseTags = map[string]bool{"form-of": true, "alt-of": true, "no-gloss": true}

var romans = map[string]string{
	"form-i": "I", "form-ii": "II", "form-iii": "III", "form-iv": "IV", "form-v": "V",
	"form-vi": "VI", "form-vii": "VII", "form-viii": "VIII", "form-ix": "IX", "form-x": "X",
	"form-xi": "XI", "form-xii": "XII", "form-xiii": "XIII", "form-iq": "Iq", "form-iiq": "IIq",
	"form-iiiq": "IIIq", "form-ivq": "IVq",
}

var particlePos = map[string]bool{
	"prep": true, "conj": true, "particle": true, "intj": true, "pron": true, "det": true,
	"adv": true, "article": true, "prep_phrase": true, "phrase": true, "num": true,
}

var posPriority = map[string]int{
	"verb": 0, "noun": 1, "adj": 2, "adv": 3, "prep": 4, "conj": 5, "pron": 6, "particle": 7,
	"det": 8, "num": 9, "intj": 10, "phrase": 11, "prep_phrase": 12, "article": 13, "name": 20,
}

type rawForm struct {
	Form   string   `json:"form"`
	Tags   []string `json:"tags"`
	Source string   `json:"source"`
}

type rawSense struct {
	Glosses []string        `json:"glosses"`
	Tags    []string        `json:"tags"`
	FormOf  json.RawMessage `json:"form_of"`
	AltOf   json.RawMessage `json:"alt_of"`
}

type rawTemplate struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args"`
}

type rawEntry struct {
	Word               string          `json:"word"`
	Pos                string          `json:"pos"`
	EtymologyNumber    json.RawMessage `json:"etymology_number"`
	Forms              []rawForm       `json:"forms"`
	Senses             []rawSense      `json:"senses"`
	EtymologyTemplates []rawTemplate   `json:"etymology_templates"`
}

func Read(r io.Reader) ([]*lexicon.Entry, error) {
	var entries []*lexicon.Entry
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		var raw rawEntry
		if err := json.Unmarshal(sc.Bytes(), &raw); err != nil {
			return nil, fmt.Errorf("wiktionary dump line %d: %w", line, err)
		}
		if e := convert(&raw); e != nil {
			entries = append(entries, e)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading wiktionary dump: %w", err)
	}
	return entries, nil
}

func convert(raw *rawEntry) *lexicon.Entry {
	if skippedPos[raw.Pos] {
		return nil
	}
	e := &lexicon.Entry{Title: raw.Word, Pos: raw.Pos, Etymology: strings.Trim(string(raw.EtymologyNumber), `"`)}
	if e.Etymology == "null" {
		e.Etymology = ""
	}
	for _, s := range raw.Senses {
		if len(s.Glosses) == 0 || len(s.FormOf) > 0 || len(s.AltOf) > 0 {
			continue
		}
		if slices.ContainsFunc(s.Tags, func(t string) bool { return skippedSenseTags[t] }) {
			continue
		}
		e.Senses = append(e.Senses, lexicon.Sense{
			Gloss: strings.Join(s.Glosses, " › "),
			Tags:  s.Tags,
			MSA:   !slices.ContainsFunc(s.Tags, func(t string) bool { return nonMSATags[t] }),
		})
	}
	if len(e.Senses) == 0 {
		return nil
	}
	if f, w, ok := canonicalForm(raw.Forms); ok {
		e.Canonical = w
		e.Gender = gender(f.Tags)
		e.VerbForm = verbForm(f.Tags)
	}
	if e.Canonical == "" {
		e.Canonical = raw.Word
	}
	headline(e, raw.Forms)
	for _, t := range raw.EtymologyTemplates {
		if t.Name == "ar-root" || t.Name == "ar-rootbox" {
			if r := strings.TrimSpace(t.Args["1"]); r != "" && arabic.HasArabic(r) {
				e.Root = r
				break
			}
		}
	}
	e.Forms = indexForms(e, raw.Forms)
	return e
}

func canonicalForm(forms []rawForm) (rawForm, string, bool) {
	for _, f := range forms {
		if slices.Contains(f.Tags, "canonical") {
			if w, ok := arabicPhrase(f.Form); ok {
				return f, w, true
			}
		}
	}
	return rawForm{}, "", false
}

func arabicPhrase(s string) (string, bool) {
	var words []string
	for _, w := range strings.Fields(s) {
		if arabic.HasArabic(w) && !strings.ContainsFunc(w, isLatinOrDigit) {
			words = append(words, w)
		}
	}
	return strings.Join(words, " "), len(words) > 0
}

func isLatinOrDigit(r rune) bool {
	return r < 0x80 && r != '-'
}

func arabicWord(s string) (string, bool) {
	var found string
	for _, w := range strings.Fields(s) {
		if !arabic.HasArabic(w) {
			continue
		}
		if found != "" {
			return "", false
		}
		found = w
	}
	return found, found != ""
}

func gender(tags []string) string {
	m, f := slices.Contains(tags, "masculine"), slices.Contains(tags, "feminine")
	switch {
	case m && f:
		return "m+f"
	case m:
		return "m"
	case f:
		return "f"
	}
	return ""
}

func verbForm(tags []string) string {
	for _, t := range tags {
		if r, ok := romans[t]; ok {
			return r
		}
	}
	return ""
}

func headline(e *lexicon.Entry, forms []rawForm) {
	for _, f := range forms {
		if f.Source != "" {
			continue
		}
		w, ok := arabicWord(f.Form)
		if !ok || slices.Contains(f.Tags, "canonical") {
			continue
		}
		has := func(t string) bool { return slices.Contains(f.Tags, t) }
		switch {
		case has("non-past"):
			if e.NonPast == "" {
				e.NonPast = w
			}
		case has("noun-from-verb"):
			e.VerbalNouns = appendUnique(e.VerbalNouns, w)
		case has("elative"):
			if e.Elative == "" {
				e.Elative = w
			}
		case has("feminine") && has("plural"):
			e.FemininePlurals = appendUnique(e.FemininePlurals, w)
		case has("plural") && !has("dual"):
			e.Plurals = appendUnique(e.Plurals, w)
		case has("feminine") && !has("dual") && !has("construct"):
			if e.Feminine == "" {
				e.Feminine = w
			}
		}
	}
}

func appendUnique(list []string, w string) []string {
	if slices.Contains(list, w) {
		return list
	}
	return append(list, w)
}

func formClass(pos string, tags []string) lexicon.Class {
	has := func(t string) bool { return slices.Contains(tags, t) }
	if pos == "verb" {
		switch {
		case has("imperative"):
			return lexicon.Imperative
		case has("non-past"), has("indicative"), has("subjunctive"), has("jussive"):
			return lexicon.Imperfect
		}
		return lexicon.Perfect
	}
	switch {
	case has("definite"):
		return lexicon.Definite
	case has("construct"):
		return lexicon.Construct
	}
	return lexicon.Nominal
}

func indexForms(e *lexicon.Entry, forms []rawForm) []lexicon.Form {
	seen := map[lexicon.Form]bool{}
	var out []lexicon.Form
	add := func(f lexicon.Form) {
		if f.Skeleton == "" || seen[f] {
			return
		}
		seen[f] = true
		out = append(out, f)
	}
	for _, f := range forms {
		if slices.ContainsFunc(f.Tags, func(t string) bool {
			return t == "romanization" || t == "table-tags" || t == "inflection-template" || t == "class"
		}) {
			continue
		}
		if e.Pos == "verb" && slices.ContainsFunc(f.Tags, func(t string) bool { return t == "participle" || t == "noun-from-verb" }) {
			continue
		}
		w, ok := arabicWord(f.Form)
		if !ok {
			continue
		}
		add(lexicon.Form{Skeleton: tashkeel.Skeleton(w), Class: formClass(e.Pos, f.Tags)})
	}
	class := lexicon.Nominal
	switch {
	case e.Pos == "verb":
		class = lexicon.Perfect
	case particlePos[e.Pos]:
		class = lexicon.Particle
	}
	if w, ok := arabicWord(e.Canonical); ok {
		add(lexicon.Form{Skeleton: tashkeel.Skeleton(w), Class: class})
	}
	add(lexicon.Form{Skeleton: tashkeel.Skeleton(e.Title), Class: class})
	return out
}

func Group(entries []*lexicon.Entry) []*lexicon.Lemma {
	byKey := map[string]*lexicon.Lemma{}
	var order []string
	for _, e := range entries {
		key := tashkeel.LexKey(e.Canonical)
		if key == "" {
			continue
		}
		l, ok := byKey[key]
		if !ok {
			l = &lexicon.Lemma{}
			byKey[key] = l
			order = append(order, key)
		}
		l.Entries = append(l.Entries, e)
	}
	out := make([]*lexicon.Lemma, 0, len(order))
	for _, key := range order {
		l := byKey[key]
		slices.SortStableFunc(l.Entries, func(a, b *lexicon.Entry) int {
			if wa, wb := a.Weight(), b.Weight(); wa != wb {
				if wa > wb {
					return -1
				}
				return 1
			}
			return posPriority[a.Pos] - posPriority[b.Pos]
		})
		l.ID = l.Main().Canonical
		out = append(out, l)
	}
	return out
}
