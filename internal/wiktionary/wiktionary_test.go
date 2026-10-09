package wiktionary

import (
	"slices"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
)

const dump = `{"word":"ماء","pos":"noun","etymology_number":1,"forms":[{"form":"مَاء","tags":["canonical","masculine"]},{"form":"مِيَاه","tags":["plural"]}],"senses":[{"glosses":["water"]}]}
{"word":"ماء","pos":"verb","etymology_number":2,"forms":[{"form":"مَاءَ","tags":["canonical","form-i"]},{"form":"يَمِيءُ","tags":["non-past"]},{"form":"مَيْء","tags":["noun-from-verb"]}],"senses":[{"glosses":["to meow"]}]}
{"word":"كتاب","pos":"noun","forms":[{"form":"كِتَاب","tags":["canonical"]},{"form":"الْكِتَاب","tags":["definite","singular"],"source":"declension"}],"senses":[{"glosses":["verbal noun of كَتَبَ"],"tags":["form-of"]},{"glosses":["book"]},{"glosses":["a pen","of a tribe"],"tags":["Egyptian"]}]}
{"word":"x","pos":"character","senses":[{"glosses":["a letter"]}]}
{"word":"y","pos":"noun","senses":[{"glosses":["x"],"alt_of":[{"word":"z"}]}]}
{"word":"عمر","pos":"name","forms":[{"form":"عُمَر","tags":["canonical"]}],"senses":[{"glosses":["Omar"]}]}
`

func titles(entries []*lexicon.Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Title+"/"+e.Pos)
	}
	return out
}

func TestReadKeepsMSAEntriesAndDropsFormsOfCharactersAndEmptyOnes(t *testing.T) {
	entries, err := Read(strings.NewReader(dump))
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(entries); !slices.Equal(got, []string{"ماء/noun", "ماء/verb", "كتاب/noun", "عمر/name"}) {
		t.Fatalf("entries = %v", got)
	}
	water, meow, book := entries[0], entries[1], entries[2]
	if water.Canonical != "مَاء" || water.Gender != "m" || !slices.Equal(water.Plurals, []string{"مِيَاه"}) || water.Etymology != "1" {
		t.Errorf("water = %+v", water)
	}
	if meow.VerbForm != "I" || meow.NonPast != "يَمِيءُ" || !slices.Equal(meow.VerbalNouns, []string{"مَيْء"}) {
		t.Errorf("meow = %+v", meow)
	}
	if len(book.Senses) != 2 || book.Senses[0].Gloss != "book" || !book.Senses[0].MSA || book.Senses[1].MSA {
		t.Errorf("a form-of sense is dropped and a regional one is not MSA: %+v", book.Senses)
	}
	if !slices.Contains(book.Forms, lexicon.Form{Skeleton: "الكتاب", Class: lexicon.Definite}) || !slices.Contains(book.Forms, lexicon.Form{Skeleton: "كتاب", Class: lexicon.Nominal}) {
		t.Errorf("forms = %+v", book.Forms)
	}
}

func TestReadReportsBrokenLines(t *testing.T) {
	_, err := Read(strings.NewReader("{\"word\":\"ماء\"}\nnot json\n"))
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err = %v", err)
	}
}

func TestGroupJoinsSpellingsThatDifferOnlyInTheirEndingAndOrdersByWeightThenPartOfSpeech(t *testing.T) {
	entries, err := Read(strings.NewReader(dump))
	if err != nil {
		t.Fatal(err)
	}
	lemmas := Group(entries)
	if len(lemmas) != 3 {
		t.Fatalf("lemmas = %d", len(lemmas))
	}
	l := lemmas[0]
	if l.ID != "مَاءَ" || !slices.Equal(l.Pos(), []string{"verb", "noun"}) {
		t.Errorf("the verb sorts before the noun when both have one sense: %s %v", l.ID, l.Pos())
	}
	if lemmas[1].ID != "كِتَاب" || !lemmas[2].Name() {
		t.Errorf("lemmas = %s, %s", lemmas[1].ID, lemmas[2].ID)
	}
}
