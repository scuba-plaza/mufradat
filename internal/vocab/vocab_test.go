package vocab

import (
	"bytes"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/morph"
	"github.com/scuba-plaza/mufradat/internal/ranking"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

func lemma(id, pos string, forms ...string) *lexicon.Lemma {
	e := &lexicon.Entry{Title: tashkeel.Skeleton(id), Pos: pos, Canonical: id, Senses: []lexicon.Sense{{Gloss: "x", MSA: true}}}
	for _, f := range forms {
		e.Forms = append(e.Forms, lexicon.Form{Skeleton: tashkeel.Skeleton(f), Class: lexicon.Nominal})
	}
	e.Forms = append(e.Forms, lexicon.Form{Skeleton: tashkeel.Skeleton(id), Class: lexicon.Nominal}, lexicon.Form{Skeleton: "ال" + tashkeel.Skeleton(id), Class: lexicon.Definite})
	return &lexicon.Lemma{ID: id, Entries: []*lexicon.Entry{e}}
}

func fixture(t *testing.T) *Checker {
	t.Helper()
	lemmas := []*lexicon.Lemma{
		lemma("كِتَاب", "noun", "كُتُب"),
		lemma("بَيْت", "noun"),
		lemma("قَلَم", "noun"),
		lemma("طَاوِلَة", "noun"),
		lemma("مِجْهَر", "noun"),
		lemma("قَلْب", "noun"),
	}
	table := morph.NewTable(make([]morph.Facts, len(lemmas)))
	for i, l := range lemmas {
		for _, f := range l.Entries[0].Forms {
			table.Add(i, f)
		}
	}
	var forms bytes.Buffer
	if err := table.Encode(&forms); err != nil {
		t.Fatal(err)
	}
	list, err := ranking.Parse(strings.NewReader("1\tكِتَاب\n2\tبَيْت\n3\tقَلَم\n4\tطَاوِلَة\n"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(lexicon.NewIndex(lemmas), list, &forms)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func words(off []Offender) []string {
	var out []string
	for _, o := range off {
		out = append(out, o.Word)
	}
	return out
}

func TestBeyondFlagsKnownWordsRarerThanTheCeiling(t *testing.T) {
	c := fixture(t)
	off := c.Beyond("[الْكِتَابُ] عَلَى الطَّاوِلَةِ", 3)
	if len(off) != 1 || off[0].Word != "الطَّاوِلَةِ" || off[0].Lemma != "طَاوِلَة" || off[0].Rank != 4 {
		t.Fatalf("offenders = %+v", off)
	}
	if off := c.Beyond("[الْكِتَابُ] عَلَى الطَّاوِلَةِ", 4); len(off) != 0 {
		t.Errorf("a word at the ceiling is allowed: %+v", off)
	}
}

func TestBeyondFlagsWordsOutsideTheRanking(t *testing.T) {
	c := fixture(t)
	off := c.Beyond("[الْكِتَابُ] تَحْتَ الْمِجْهَرِ", 100)
	if len(off) != 1 || off[0].Word != "الْمِجْهَرِ" || off[0].Rank != 0 {
		t.Fatalf("offenders = %+v", off)
	}
	if got := off[0].String(); !strings.Contains(got, "not among the 5000 most frequent words") {
		t.Errorf("message = %q", got)
	}
}

func TestBeyondLeavesTheTargetAndUnknownWordsAlone(t *testing.T) {
	c := fixture(t)
	if off := c.Beyond("قَرَأَ مُحَمَّدٌ [الْمِجْهَرَ] فِي الْبَيْتِ", 100); len(off) != 0 {
		t.Errorf("the marked word is the card's own, and a word the lexicon cannot place is not judged: %+v", off)
	}
	if off := c.Beyond("بَيْتٌ مِجْهَرٌ", 1); len(off) != 2 {
		t.Errorf("without markup every word is judged: %v", words(off))
	}
}

func TestBeyondResolvesClitics(t *testing.T) {
	c := fixture(t)
	for _, sentence := range []string{"[قَرَأْتُ] وَالطَّاوِلَةَ", "[قَرَأْتُ] بِالطَّاوِلَةِ", "[قَرَأْتُ] طَاوِلَتَهُ", "[قَرَأْتُ] لِلطَّاوِلَةِ"} {
		off := c.Beyond(sentence, 3)
		if len(off) != 1 || off[0].Lemma != "طَاوِلَة" {
			t.Errorf("%s: the clitics should come off: %+v", sentence, off)
		}
		if off := c.Beyond(sentence, 4); len(off) != 0 {
			t.Errorf("%s: inside the ceiling: %+v", sentence, off)
		}
	}
}

func TestNewRejectsARankingThatDisagreesWithTheLexicon(t *testing.T) {
	lemmas := []*lexicon.Lemma{lemma("كِتَاب", "noun")}
	list, _ := ranking.Parse(strings.NewReader("1\tبَيْت\n"))
	if _, err := New(lexicon.NewIndex(lemmas), list, strings.NewReader("")); err == nil {
		t.Error("a ranking that disagrees with the lexicon must be refused")
	}
}

func TestEmbeddedDataAgrees(t *testing.T) {
	lex, err := lexicon.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	list, err := ranking.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	c, err := Embedded(lex, list)
	if err != nil {
		t.Fatal(err)
	}
	if off := c.Beyond("[الْكِتَابُ] فِي الْحَقِيبَةِ", 1500); len(off) != 0 {
		t.Errorf("everyday words should be inside the first 1500: %+v", off)
	}
	if off := c.Beyond("[الْكِتَابُ] عَلَى الطَّاوِلَةِ", 1500); len(off) != 1 || off[0].Lemma != "طَاوِلَة" {
		t.Errorf("طَاوِلَة ranks beyond 1500: %+v", off)
	}
}

func TestBeyondReportsARepeatedWordOnce(t *testing.T) {
	c := fixture(t)
	off := c.Beyond("[الْكِتَابُ] الطَّاوِلَةِ ثُمَّ الطَّاوِلَةِ", 3)
	if len(off) != 1 || off[0].Word != "الطَّاوِلَةِ" {
		t.Errorf("offenders = %v", words(off))
	}
}
