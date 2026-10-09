package morph

import (
	"bytes"
	"compress/gzip"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
)

func gz(t *testing.T, text string) io.Reader {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	io.WriteString(w, text)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &b
}

func sample() *Table {
	t := NewTable([]Facts{{}, {Verb: true}, {Prep: true}})
	t.Add(0, lexicon.Form{Skeleton: "كتاب", Class: lexicon.Nominal})
	t.Add(0, lexicon.Form{Skeleton: "الكتاب", Class: lexicon.Definite})
	t.Add(0, lexicon.Form{Skeleton: "الكتاب", Class: lexicon.Definite})
	t.Add(1, lexicon.Form{Skeleton: "كتب", Class: lexicon.Perfect})
	t.Add(1, lexicon.Form{Skeleton: "يكتب", Class: lexicon.Imperfect})
	t.Add(2, lexicon.Form{Skeleton: "في", Class: lexicon.Particle})
	return t
}

func TestMatchesStripsClitsAndHonoursWhatEachClassAllows(t *testing.T) {
	table := sample()
	cases := map[string][]int{
		"الكتاب":   {0},
		"بالكتاب":  {0},
		"وكتب":     {1},
		"سيكتب":    {1},
		"بيكتب":    nil,
		"بكتب":     nil,
		"فيه":      {2},
		"كتابه":    {0},
		"الكتابه":  nil,
		"غيرموجود": nil,
	}
	for word, want := range cases {
		if got := table.Analyze(word); !slices.Equal(got, want) {
			t.Errorf("Analyze(%s) = %v, want %v", word, got, want)
		}
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	table := sample()
	var b bytes.Buffer
	if err := table.Encode(&b); err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	if err := table.Encode(&again); err != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("encoding is not stable")
	}
	back, err := Decode(&b, []Facts{{}, {Verb: true}, {Prep: true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"الكتاب", "بالكتاب", "سيكتب", "فيه", "x"} {
		if got, want := back.Analyze(w), table.Analyze(w); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", w, got, want)
		}
	}
}

func TestDecodeRejectsDataThatDoesNotFitTheLexicon(t *testing.T) {
	facts := []Facts{{}, {}}
	for name, text := range map[string]string{
		"no tab":         "كتاب\n",
		"no class":       "كتاب\t0\n",
		"lemma too big":  "كتاب\t2:0\n",
		"negative lemma": "كتاب\t-1:0\n",
		"bad class":      "كتاب\t0:9\n",
		"not numbers":    "كتاب\ta:b\n",
	} {
		if _, err := Decode(gz(t, text), facts); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := Decode(strings.NewReader("not gzip"), facts); err == nil {
		t.Error("plain text accepted")
	}
}
