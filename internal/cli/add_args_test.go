package cli

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/author"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/note"
)

func TestAddArgumentsAreACountOrWordsSplitOnCommasAndLines(t *testing.T) {
	a, _ := reportingApp(t)
	file := filepath.Join(t.TempDir(), "words.txt")
	if err := os.WriteFile(file, []byte("قَلَم  # a pen\n\n# only a comment\nبَاب, كِتَاب\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name      string
		args      []string
		file      string
		wantCount int
		wantWords []string
		wantErr   string
	}{
		{"nothing asks for the default count", nil, "", 10, nil, ""},
		{"a number is a count", []string{"25"}, "", 25, nil, ""},
		{"one is the smallest count", []string{"1"}, "", 1, nil, ""},
		{"zero is not a count", []string{"0"}, "", 0, nil, "the number of words must be at least 1"},
		{"a negative number is not a count", []string{"-3"}, "", 0, nil, "the number of words must be at least 1"},
		{"words keep their vowels and lose tatweel and stray spaces", []string{"كِتَاب", " قـَلَم ", "ذهب   إلى"}, "", 0, []string{"كِتَاب", "قَلَم", "ذهب إلى"}, ""},
		{"a word given twice is added once", []string{"كتاب", "قلم", "كتاب"}, "", 0, []string{"كتاب", "قلم"}, ""},
		{"commas of both kinds and newlines separate words", []string{"كتاب, قلم،باب\nبيت"}, "", 0, []string{"كتاب", "قلم", "باب", "بيت"}, ""},
		{"only separators leave no words", []string{" , ،"}, "", 0, nil, "there are no words to add"},
		{"a blank word leaves no words", []string{""}, "", 0, nil, "there are no words to add"},
		{"a number next to a file is a word, not a count", []string{"5"}, file, 0, []string{"5", "قَلَم", "بَاب", "كِتَاب"}, ""},
		{"a file alone supplies the words, comments dropped", nil, file, 0, []string{"قَلَم", "بَاب", "كِتَاب"}, ""},
		{"a file that cannot be opened is a usage error", nil, filepath.Join(t.TempDir(), "missing.txt"), 0, nil, "no such file"},
	} {
		count, words, err := parseAddArgs(a, c.args, c.file)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) || failure.Code(err) != failure.ExitUsage {
				t.Errorf("%s: err = %v (code %d), want a usage error containing %q", c.name, err, failure.Code(err), c.wantErr)
			}
			continue
		}
		if err != nil || count != c.wantCount && c.wantCount != 0 || !slices.Equal(words, c.wantWords) {
			t.Errorf("%s: count %d, words %q, err %v; want %d, %q", c.name, count, words, err, c.wantCount, c.wantWords)
		}
	}
}

func TestAnEmptyWordFileIsAnErrorNotAnAddOfTheDefaultCount(t *testing.T) {
	a, _ := reportingApp(t)
	file := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(file, []byte("# nothing here\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := parseAddArgs(a, nil, file); err == nil || err.Error() != "there are no words to add" || failure.Code(err) != failure.ExitUsage {
		t.Errorf("err = %v", err)
	}
}

func TestWordsCanComeFromStandardInput(t *testing.T) {
	a, _ := reportingApp(t)
	a.Env.Stdin = strings.NewReader("كتاب\nقلم # pen\n")
	_, words, err := parseAddArgs(a, nil, "-")
	if err != nil || !slices.Equal(words, []string{"كتاب", "قلم"}) {
		t.Errorf("words %q, err %v", words, err)
	}
}

func TestTheResumeCommandRepeatsTheWordsTheUserTypedOnce(t *testing.T) {
	a, _ := reportingApp(t)
	typed := func(words ...string) []author.Target {
		var out []author.Target
		for _, w := range words {
			out = append(out, author.Target{Typed: w, Draft: note.Note{Lemma: "ignored"}})
		}
		return out
	}
	for _, c := range []struct {
		name    string
		targets []author.Target
		named   bool
		want    string
	}{
		{"a ranked run is resumed by count", typed("", "", ""), false, "mufradat add 3"},
		{"a named run is resumed by word", typed("كتاب", "قلم"), true, "mufradat add كتاب قلم"},
		{"a word typed twice appears once", typed("كتاب", "قلم", "كتاب"), true, "mufradat add كتاب قلم"},
		{"targets without a typed word are left out", typed("", "كتاب", ""), true, "mufradat add كتاب"},
		{"a phrase is quoted", typed("ذهب إلى", "كتاب"), true, `mufradat add "ذهب إلى" كتاب`},
	} {
		if got := resumeCommand(a, c.targets, c.named); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestThePromptChooserAsksUntilItGetsAnEntryThatExists(t *testing.T) {
	cands := lexicon.NewIndex([]*lexicon.Lemma{{ID: "مَاء", Entries: []*lexicon.Entry{
		{Title: "ماء", Pos: "noun", Canonical: "مَاء", Senses: []lexicon.Sense{{Gloss: "water", MSA: true}}},
		{Title: "ماء", Pos: "verb", Canonical: "مَاءَ", Senses: []lexicon.Sense{{Gloss: "to meow", MSA: true}}},
	}}}).Find("ماء")
	if len(cands) != 2 {
		t.Fatalf("fixture has %d candidates", len(cands))
	}
	for _, c := range []struct {
		name    string
		input   string
		want    int
		stopped bool
		retries int
	}{
		{"a number picks that entry", "2\n", 2, false, 0},
		{"the first entry", "1\n", 1, false, 0},
		{"enter alone takes the first", "\n", 1, false, 0},
		{"q stops", "q\n", 0, true, 0},
		{"the end of input stops", "", 0, true, 0},
		{"zero is asked again", "0\n2\n", 2, false, 1},
		{"a number past the last is asked again", "3\n1\n", 1, false, 1},
		{"a negative number is asked again", "-1\n2\n", 2, false, 1},
		{"words are asked again", "two\n2\n", 2, false, 1},
		{"a bad answer at the end of input stops", "7", 0, true, 1},
		{"a good answer at the end of input without a newline counts", "2", 2, false, 0},
	} {
		var out bytes.Buffer
		ch := &promptChooser{in: bufio.NewReader(strings.NewReader(c.input)), out: &out}
		got, err := ch.Choose("ماء", cands)
		if c.stopped != errors.Is(err, errStopped) || err != nil && !c.stopped || got != c.want {
			t.Errorf("%s: chose %d, err %v; want %d, stopped=%v", c.name, got, err, c.want, c.stopped)
		}
		text := out.String()
		if !strings.HasPrefix(text, "ماء matches 2 entries:\n  1. مَاء  noun  water\n  2. مَاءَ  verb  to meow\nwhich one? [1, q to stop] ") {
			t.Errorf("%s: the question reads %q", c.name, text)
		}
		if n := strings.Count(text, "enter a number from 1 to 2\n"); n != c.retries {
			t.Errorf("%s: asked again %d times, want %d: %q", c.name, n, c.retries, text)
		}
	}
}
