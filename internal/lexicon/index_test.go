package lexicon

import (
	"slices"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

func entry(canonical, pos string, glosses ...string) *Entry {
	e := &Entry{Title: tashkeel.Skeleton(canonical), Pos: pos, Canonical: canonical}
	for _, g := range glosses {
		e.Senses = append(e.Senses, Sense{Gloss: g, MSA: true})
	}
	return e
}

func lemma(entries ...*Entry) *Lemma {
	return &Lemma{ID: entries[0].Canonical, Entries: entries}
}

func names(cands []Candidate) []string {
	var out []string
	for _, c := range cands {
		out = append(out, c.Main().Canonical+"/"+c.Main().Pos)
	}
	return out
}

func fixture() *Index {
	return NewIndex([]*Lemma{
		lemma(entry("عِلْم", "noun", "knowledge")),
		lemma(entry("عَلِمَ", "verb", "to know")),
		lemma(entry("مَاء", "noun", "water"), entry("مَاءَ", "verb", "to meow")),
		lemma(entry("أَكَلَ", "verb", "to eat")),
	})
}

func TestFindListsEverySenseOfASpelling(t *testing.T) {
	got := fixture().Find("ماء")
	if want := []string{"مَاء/noun", "مَاءَ/verb"}; !slices.Equal(names(got), want) {
		t.Fatalf("candidates = %v, want %v", names(got), want)
	}
	if !got[0].Primary || got[1].Primary {
		t.Errorf("only the first entry of a lemma is its main sense: %+v", got)
	}
}

func TestFindFollowsIndexOrderAndNarrowsByVowels(t *testing.T) {
	ix := fixture()
	if got := ix.Find("علم"); !slices.Equal(names(got), []string{"عِلْم/noun", "عَلِمَ/verb"}) {
		t.Fatalf("bare spelling: %v", names(got))
	}
	if got := ix.Find("عِلْم"); !slices.Equal(names(got), []string{"عِلْم/noun"}) {
		t.Errorf("vowelled noun: %v", names(got))
	}
	if got := ix.Find("عَلِمَ"); !slices.Equal(names(got), []string{"عَلِمَ/verb"}) {
		t.Errorf("vowelled verb: %v", names(got))
	}
	if got := ix.Find("عُلِمَ"); len(got) != 0 {
		t.Errorf("vowels that fit no entry should find nothing: %v", names(got))
	}
}

func TestFindFoldsHamzaOnlyWhenNothingMatchesExactly(t *testing.T) {
	ix := fixture()
	if got := ix.Find("اكل"); !slices.Equal(names(got), []string{"أَكَلَ/verb"}) {
		t.Fatalf("folded: %v", names(got))
	}
	with := NewIndex(append(ix.Lemmas(), lemma(entry("اكل", "noun", "x"))))
	if got := with.Find("اكل"); !slices.Equal(names(got), []string{"اكل/noun"}) {
		t.Errorf("an exact spelling should win over a folded one: %v", names(got))
	}
}

func TestFindLeavesOutNamesAndNonMSAEntries(t *testing.T) {
	archaic := entry("مَاء", "adj", "old")
	archaic.Senses[0].MSA = false
	ix := NewIndex([]*Lemma{lemma(entry("مَاء", "name", "a person"), archaic, entry("مَاء", "noun", "water"))})
	got := ix.Find("ماء")
	if len(got) != 1 || got[0].Main().Pos != "noun" {
		t.Fatalf("candidates = %v", names(got))
	}
	if got[0].Primary {
		t.Error("the noun is not the lemma's first entry, so it must not be primary")
	}
}

func TestFindMergesEntriesWithTheSamePartOfSpeech(t *testing.T) {
	ix := NewIndex([]*Lemma{lemma(entry("عَيْن", "noun", "eye", "spring"), entry("عَيْن", "noun", "the letter ع"))})
	got := ix.Find("عين")
	if len(got) != 1 || len(got[0].Entries) != 2 || len(got[0].Senses()) != 3 || !got[0].Primary {
		t.Fatalf("candidates = %+v", got)
	}
}

func TestFindMatchesByPageTitleWhenHeadwordHasAnArticle(t *testing.T) {
	e := entry("الْكِتَاب", "noun", "the book")
	e.Title = "كتاب"
	ix := NewIndex([]*Lemma{lemma(e)})
	if got := ix.Find("كِتَاب"); !slices.Equal(names(got), []string{"الْكِتَاب/noun"}) {
		t.Errorf("title match through the article: %v", names(got))
	}
}

func TestFindIgnoresBlankAndTatweel(t *testing.T) {
	ix := fixture()
	if got := ix.Find("  "); got != nil {
		t.Errorf("blank input found %v", names(got))
	}
	if got := ix.Find("مــاء"); len(got) != 2 {
		t.Errorf("tatweel should be ignored: %v", names(got))
	}
}

func TestEmbeddedIndexLoads(t *testing.T) {
	ix, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if ix.Len() < 4000 {
		t.Fatalf("only %d lemmas", ix.Len())
	}
	l, ok := ix.Lemma("فِي")
	if !ok || l.Main().Pos != "prep" {
		t.Errorf("lemma فِي = %+v %v", l, ok)
	}
	if got := ix.Find("عين"); len(got) < 1 {
		t.Errorf("عين should be found, got %v", names(got))
	}
}
