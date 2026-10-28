package review

import (
	"fmt"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

const (
	classMajor  = "flag-major"
	classMinor  = "flag-minor"
	classTarget = "target"
)

type span struct {
	T string `json:"t"`
	C string `json:"c,omitempty"`
}

type row struct {
	Label string `json:"label"`
	Spans []span `json:"spans"`
	RTL   bool   `json:"rtl"`
	Note  string `json:"note,omitempty"`
}

type flagView struct {
	Tone    string `json:"tone"`
	Symbol  string `json:"symbol"`
	Title   string `json:"title"`
	Where   string `json:"where"`
	Rows    []row  `json:"rows"`
	Explain string `json:"explain,omitempty"`
	Faded   bool   `json:"faded"`
}

type formView struct {
	Label string `json:"label"`
	Spans []span `json:"spans"`
}

type cardView struct {
	Head      []span     `json:"head"`
	Meta      string     `json:"meta"`
	English   string     `json:"english"`
	Hint      string     `json:"hint,omitempty"`
	Forms     []formView `json:"forms"`
	Example   []span     `json:"example"`
	ExampleEN string     `json:"example_en"`
	Comment   string     `json:"comment,omitempty"`
}

type changeView struct {
	Label string `json:"label"`
	RTL   bool   `json:"rtl"`
	Old   []span `json:"old"`
	New   []span `json:"new"`
}

func fieldName(field string) string {
	switch field {
	case verify.FieldLemma:
		return "word"
	case verify.FieldExample:
		return "sentence"
	}
	return field
}

func appendSpan(out []span, text, class string) []span {
	if n := len(out); n > 0 && out[n-1].C == class {
		out[n-1].T += text
		return out
	}
	return append(out, span{T: text, C: class})
}

func withClass(a, b string) string {
	if a == "" {
		return b
	}
	return a + " " + b
}

func flaggedWords(open []verify.Issue) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, is := range open {
		if is.Word == "" {
			continue
		}
		if out[is.Field] == nil {
			out[is.Field] = map[string]string{}
		}
		if out[is.Field][is.Word] == classMajor {
			continue
		}
		out[is.Field][is.Word] = classMinor
		if is.Severity == verify.Major {
			out[is.Field][is.Word] = classMajor
		}
	}
	return out
}

func pieceSpans(out []span, p piece, base string) []span {
	start := 0
	for i := 1; i <= len(p.runes); i++ {
		if i < len(p.runes) && p.target[i] == p.target[start] {
			continue
		}
		class := base
		if p.target[start] {
			class = withClass(class, classTarget)
		}
		out = appendSpan(out, string(p.runes[start:i]), class)
		start = i
	}
	return out
}

func markup(s string, flagged map[string]string) []span {
	out := []span{}
	for _, p := range pieces(s) {
		base := ""
		if p.word {
			base = flagged[p.String()]
		}
		out = pieceSpans(out, p, base)
	}
	return out
}

func meta(n note.Note) string {
	parts := []string{n.POS.Label()}
	switch n.Gender {
	case "m":
		parts = append(parts, "masc.")
	case "f":
		parts = append(parts, "fem.")
	case "m+f":
		parts = append(parts, "masc./fem.")
	}
	if n.VerbForm != "" {
		parts = append(parts, "form "+n.VerbForm)
	}
	if n.Rank > 0 {
		parts = append(parts, fmt.Sprintf("#%d", n.Rank))
	} else {
		parts = append(parts, string(n.Origin))
	}
	return strings.Join(parts, " · ")
}

func cardOf(n note.Note, open []verify.Issue) cardView {
	flags := flaggedWords(open)
	c := cardView{
		Head: markup(n.Lemma, flags[verify.FieldLemma]), Meta: meta(n),
		English: n.English, Hint: n.Hint, Forms: []formView{},
		Example: markup(n.Example, flags[verify.FieldExample]), ExampleEN: n.ExampleEN, Comment: n.Comment,
	}
	for _, f := range n.Forms {
		c.Forms = append(c.Forms, formView{Label: f.Label, Spans: markup(f.Arabic, flags[verify.FieldForms])})
	}
	return c
}

func issueView(is verify.Issue, faded bool) flagView {
	tone, symbol := "bad", "✗"
	if is.Severity == verify.Minor {
		tone, symbol = "warn", "!"
	}
	hl := "mark-" + tone
	if faded {
		hl = ""
	}
	title, rows, explain := describe(is, hl)
	return flagView{Tone: tone, Symbol: symbol, Title: title, Where: fieldName(is.Field) + " · " + string(is.Severity), Rows: rows, Explain: explain, Faded: faded}
}

func describe(is verify.Issue, hl string) (string, []row, string) {
	word := func(label, w string, marked []bool) row {
		if w == "" {
			return row{Label: label, Spans: []span{}, Note: "no reading"}
		}
		return row{Label: label, Spans: letterSpans(w, marked, hl), RTL: true}
	}
	switch is.Kind {
	case verify.KindDiacritics:
		catt := tashkeel.Differences(is.Word, is.CATT)
		camel := tashkeel.Differences(is.Word, is.CAMeL)
		either := make([]bool, len(tashkeel.Letters(is.Word)))
		for i := range either {
			either[i] = i < len(catt) && catt[i] || is.Severity == verify.Major && i < len(camel) && camel[i]
		}
		rows := []row{word("card", is.Word, either), word("CATT", is.CATT, catt)}
		switch {
		case is.CAMeL == "":
			rows = append(rows, word("CAMeL", "", nil))
			return "CATT reads this word with other vowels", rows, "CAMeL had no reading for it, so check the vowels yourself."
		case is.Severity == verify.Minor:
			agrees := word("CAMeL", is.CAMeL, nil)
			agrees.Note = "agrees with the card"
			rows = append(rows, agrees)
			return "Only CATT reads this word with other vowels", rows, "CAMeL agrees with the card, so the card is most likely right."
		}
		rows = append(rows, word("CAMeL", is.CAMeL, camel))
		return "CATT and CAMeL both read this word with other vowels", rows, "Check the vowels. If the card is right, the sentence can probably be read another way without vowel marks; Mistral can suggest a clearer one."
	case verify.KindInvalid:
		rows := []row{word("card", is.Word, nil), word("known", strings.Join(is.Known, "، "), nil)}
		return "CAMeL does not know these vowels for this word", rows, "CAMeL's dictionary has the word, but not with these vowels. Compare with Wiktionary."
	case verify.KindUnmarked:
		rows := []row{word("card", is.Word, bareLetters(is))}
		if is.CATT != "" {
			rows = append(rows, word("CATT", is.CATT, tashkeel.Differences(is.Word, is.CATT)))
		}
		if is.CAMeL != "" {
			rows = append(rows, word("CAMeL", is.CAMeL, tashkeel.Differences(is.Word, is.CAMeL)))
		}
		if bareEnding(is) {
			return "The ending has no vowel mark", rows, "Every word in a sentence carries its ending, the last one too. The audio drops the last word's ending by itself."
		}
		return "Some letters have no vowel mark", rows, "No reading from CAMeL or CATT has these letters without a vowel, so one is probably missing."
	case verify.KindUnknown:
		return "CAMeL does not know this word", []row{word("card", is.Word, nil)}, "Only Wiktionary vouches for its vowels, which is usually fine for less common words."
	case verify.KindUnchecked:
		return "CATT could not be compared with this sentence", []row{}, "CATT split the sentence into different words, so its vowels were not compared. Read the sentence yourself."
	}
	return string(is.Kind), []row{word("card", is.Word, nil)}, is.Detail
}

func letterSpans(word string, marked []bool, hl string) []span {
	ls := tashkeel.Letters(word)
	if hl == "" || len(marked) != len(ls) {
		return []span{{T: word}}
	}
	var out []span
	for i, l := range ls {
		class := ""
		if marked[i] {
			class = hl
		}
		out = appendSpan(out, l, class)
	}
	return out
}

func bareLetters(is verify.Issue) []bool {
	out := make([]bool, len(tashkeel.Letters(is.Word)))
	for _, i := range is.Missing {
		if i >= 0 && i < len(out) {
			out[i] = true
		}
	}
	return out
}

func wordSpans(ws []string, marked []bool, class string) []span {
	out := []span{}
	for i, w := range ws {
		if i > 0 {
			out = appendSpan(out, " ", "")
		}
		c := ""
		if marked[i] {
			c = class
		}
		out = appendSpan(out, w, c)
	}
	return out
}

func formsText(n note.Note) string {
	parts := make([]string, len(n.Forms))
	for i, f := range n.Forms {
		parts[i] = f.Label + " " + f.Arabic
	}
	return strings.Join(parts, "   ")
}

func changes(cur, fresh note.Note) []changeView {
	fields := []struct {
		label, old, new string
		rtl, markup     bool
	}{
		{"word", cur.Lemma, fresh.Lemma, true, false},
		{"type", cur.POS.Label(), fresh.POS.Label(), false, false},
		{"meaning", cur.English, fresh.English, false, false},
		{"hint", cur.Hint, fresh.Hint, false, false},
		{"forms", formsText(cur), formsText(fresh), false, false},
		{"sentence", cur.Example, fresh.Example, true, true},
		{"translation", cur.ExampleEN, fresh.ExampleEN, false, false},
		{"comment", cur.Comment, fresh.Comment, false, false},
	}
	out := []changeView{}
	for _, f := range fields {
		if f.old == f.new {
			continue
		}
		c := changeView{Label: f.label, RTL: f.rtl}
		c.Old, c.New = diffSpans(f.old, f.new, f.markup)
		out = append(out, c)
	}
	return out
}

func plainSpans(s string, withMarkup bool) []span {
	switch {
	case withMarkup:
		return markup(s, nil)
	case s == "":
		return []span{}
	}
	return []span{{T: s}}
}

func diffSpans(old, fresh string, withMarkup bool) ([]span, []span) {
	if old == "" || fresh == "" {
		return plainSpans(old, withMarkup), plainSpans(fresh, withMarkup)
	}
	if withMarkup {
		po, pf := pieces(old), pieces(fresh)
		keepOld, keepFresh := common(words(po), words(pf))
		return changedPieces(po, keepOld, "del"), changedPieces(pf, keepFresh, "ins")
	}
	wo, wf := strings.Fields(old), strings.Fields(fresh)
	keepOld, keepFresh := common(wo, wf)
	return wordSpans(wo, invert(keepOld), "del"), wordSpans(wf, invert(keepFresh), "ins")
}

func changedPieces(ps []piece, keep []bool, class string) []span {
	out := []span{}
	k := 0
	for _, p := range ps {
		base := ""
		if p.word {
			if !keep[k] {
				base = class
			}
			k++
		}
		out = pieceSpans(out, p, base)
	}
	return out
}

func invert(bs []bool) []bool {
	out := make([]bool, len(bs))
	for i, b := range bs {
		out[i] = !b
	}
	return out
}
