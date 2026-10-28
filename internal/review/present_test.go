package review

import (
	"slices"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

func classes(spans []span) []string {
	var out []string
	for _, s := range spans {
		if s.C != "" {
			out = append(out, s.C)
		}
	}
	return out
}

func TestUnmarkedFlagsShowTheBareLettersAndTheReadings(t *testing.T) {
	end := issueView(verify.Issue{Field: verify.FieldExample, Kind: verify.KindUnmarked, Severity: verify.Major, Word: "جَمِيل", Missing: []int{3}, CATT: "جَمِيلٌ", CAMeL: "جَمِيلَ"}, false)
	if end.Title != "The ending has no vowel mark" || len(end.Rows) != 3 {
		t.Fatalf("ending flag %+v", end)
	}
	for _, r := range end.Rows {
		if len(r.Spans) != 2 || r.Spans[0].C != "" || r.Spans[1].C != "mark-bad" {
			t.Errorf("the %s row should mark only the last letter: %+v", r.Label, r.Spans)
		}
	}
	inside := issueView(verify.Issue{Field: verify.FieldLemma, Kind: verify.KindUnmarked, Severity: verify.Major, Word: "مَكتَبَة", Missing: []int{1}}, false)
	if inside.Title != "Some letters have no vowel mark" || len(inside.Rows) != 1 {
		t.Fatalf("inside flag %+v", inside)
	}
	if spans := inside.Rows[0].Spans; !slices.Equal(classes(spans), []string{"mark-bad"}) || spans[1].T != "ك" {
		t.Errorf("only ك should be marked: %+v", spans)
	}
}

func TestFadedFlagsCarryNoHighlights(t *testing.T) {
	v := issueView(verify.Issue{Field: verify.FieldExample, Kind: verify.KindDiacritics, Severity: verify.Major, Word: "الْبَيْتِ", CATT: "الْبَيْتُ", CAMeL: "الْبَيْتُ"}, true)
	if !v.Faded || v.Tone != "bad" {
		t.Fatalf("flag = %+v", v)
	}
	for _, r := range v.Rows {
		if len(classes(r.Spans)) != 0 {
			t.Errorf("row %s still highlights: %+v", r.Label, r.Spans)
		}
	}
}

func TestMinorDiacriticsExplainThatCAMeLAgrees(t *testing.T) {
	v := issueView(verify.Issue{Field: verify.FieldExample, Kind: verify.KindDiacritics, Severity: verify.Minor, Word: "الْبَيْتِ", CATT: "الْبَيْتُ", CAMeL: "الْبَيْتِ"}, false)
	if v.Title != "Only CATT reads this word with other vowels" || v.Tone != "warn" || v.Rows[2].Note != "agrees with the card" {
		t.Fatalf("flag = %+v", v)
	}
}

func TestCardMarksFlaggedWordsAndTheTarget(t *testing.T) {
	n := note.Note{Lemma: "كِتَاب", POS: note.Noun, Gender: "m", Rank: 3, English: "book", Example: "قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ.", ExampleEN: "I read the book at home."}
	open := []verify.Issue{{Field: verify.FieldExample, Kind: verify.KindDiacritics, Severity: verify.Major, Word: "الْكِتَابَ"}, {Field: verify.FieldExample, Kind: verify.KindUnknown, Severity: verify.Minor, Word: "فِي"}}
	c := cardOf(n, open)
	if c.Meta != "noun · masc. · #3" {
		t.Errorf("meta = %q", c.Meta)
	}
	var target, major, minor string
	for _, s := range c.Example {
		switch s.C {
		case "flag-major target":
			target = s.T
			major = s.T
		case "flag-minor":
			minor = s.T
		}
	}
	if target != "الْكِتَابَ" || major != "الْكِتَابَ" || minor != "فِي" {
		t.Errorf("example spans = %+v", c.Example)
	}
}

func TestMetaForANamedWordHasNoRank(t *testing.T) {
	if got := meta(note.Note{POS: note.Verb, VerbForm: "II", Origin: note.OriginNamed}); got != "verb · form II · named" {
		t.Errorf("meta = %q", got)
	}
}

func TestChangesOnlyMarkWordsWhenBothSidesHaveText(t *testing.T) {
	cur := note.Note{Lemma: "كِتَاب", POS: note.Noun, English: "book", Example: "[كِتَابٌ] جَدِيدٌ.", ExampleEN: "A new book."}
	fresh := cur
	fresh.Hint = "a written work"
	fresh.ExampleEN = "A thick new book."
	cs := changes(cur, fresh)
	if len(cs) != 2 || cs[0].Label != "hint" || cs[1].Label != "translation" {
		t.Fatalf("changes = %+v", cs)
	}
	if len(cs[0].Old) != 0 || len(classes(cs[0].New)) != 0 || cs[0].New[0].T != "a written work" {
		t.Errorf("a new hint should not be marked word by word: %+v", cs[0])
	}
	if !slices.Equal(classes(cs[1].New), []string{"ins"}) || len(classes(cs[1].Old)) != 0 {
		t.Errorf("translation diff: old %+v new %+v", cs[1].Old, cs[1].New)
	}
}

func TestSentenceDiffKeepsTheTargetHighlight(t *testing.T) {
	cur := note.Note{Example: "قَرَأَ [الْوَلَدُ] الْكِتَابَ."}
	fresh := note.Note{Example: "قَرَأَ [الْوَلَدُ] الصَّحِيفَةَ."}
	cs := changes(cur, fresh)
	if len(cs) != 1 || cs[0].Label != "sentence" || !cs[0].RTL {
		t.Fatalf("changes = %+v", cs)
	}
	if !slices.Contains(classes(cs[0].New), "ins") || !slices.Contains(classes(cs[0].New), "target") || !slices.Contains(classes(cs[0].Old), "del") {
		t.Errorf("old %+v new %+v", cs[0].Old, cs[0].New)
	}
}

func TestEveryKindOfChangeIsLabelledAndKnowsWhichWayItReads(t *testing.T) {
	cur := note.Note{Lemma: "كِتَاب", POS: note.Noun, English: "book", Hint: "read", Forms: []note.Form{{Label: "pl.", Arabic: "كُتُب"}}, Example: "[كِتَابٌ] جَدِيدٌ.", ExampleEN: "A new book.", Comment: "common"}
	fresh := note.Note{Lemma: "قَلَم", POS: note.Verb, English: "pen", Hint: "write", Forms: []note.Form{{Label: "pl.", Arabic: "أَقْلَام"}}, Example: "[قَلَمٌ] جَدِيدٌ.", ExampleEN: "A new pen.", Comment: "rare"}
	want := []struct {
		label string
		rtl   bool
	}{{"word", true}, {"type", false}, {"meaning", false}, {"hint", false}, {"forms", false}, {"sentence", true}, {"translation", false}, {"comment", false}}
	got := changes(cur, fresh)
	if len(got) != len(want) {
		t.Fatalf("changes = %+v", got)
	}
	for i, w := range want {
		if got[i].Label != w.label || got[i].RTL != w.rtl {
			t.Errorf("change %d is %q rtl=%v, want %q rtl=%v", i, got[i].Label, got[i].RTL, w.label, w.rtl)
		}
	}
}

func TestMissingLettersOutsideTheWordAreIgnored(t *testing.T) {
	is := verify.Issue{Word: "تَعِب", Missing: []int{-1, 2, 3, 99}}
	if got := bareLetters(is); !slices.Equal(got, []bool{false, false, true}) {
		t.Errorf("bareLetters = %v", got)
	}
}

func TestTheCardRowHighlightsTheLettersTheReadingsDisagreeOn(t *testing.T) {
	word, catt, camel := "كَتَبَ", "كَتَبُ", "كُتَبَ"
	rowOf := func(severity verify.Severity) row {
		t.Helper()
		v := issueView(verify.Issue{Field: verify.FieldExample, Kind: verify.KindDiacritics, Severity: severity, Word: word, CATT: catt, CAMeL: camel}, false)
		if len(v.Rows) < 1 || v.Rows[0].Label != "card" {
			t.Fatalf("rows = %+v", v.Rows)
		}
		return v.Rows[0]
	}
	major := rowOf(verify.Major)
	if got := classes(major.Spans); len(got) != 2 {
		t.Errorf("a major flag marks the letters either reading changes (the first and the last): %+v", major.Spans)
	}
	minor := rowOf(verify.Minor)
	if got := classes(minor.Spans); len(got) != 1 {
		t.Errorf("a minor flag marks only what CATT changes (the last letter): %+v", minor.Spans)
	}
}
