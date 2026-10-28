package app

import (
	"context"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/store"
)

func addNotes(t *testing.T, a *App, lemmas ...string) {
	t.Helper()
	st, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	err = st.Update(context.Background(), func(tx *store.Tx) error {
		for i, lemma := range lemmas {
			pos := note.POS("noun")
			if i%3 == 1 {
				pos = "verb"
			} else if i%3 == 2 {
				pos = "adj"
			}
			if _, err := tx.Add(note.Note{Origin: note.OriginManual, Lemma: lemma, POS: pos, English: "word " + lemma, Example: "قَرَأَ الرَّجُلُ [كِتَابًا] جَدِيدًا.", ExampleEN: "The man read a new book."}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func promptSections(system string) (examples, vocabulary int) {
	lines := strings.Split(system, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "Finished cards, in the format you return:"):
			for _, l := range lines[i+1:] {
				if !strings.HasPrefix(l, "{") {
					break
				}
				examples++
			}
		case strings.HasPrefix(line, "The learner's core vocabulary"):
			for _, l := range lines[i+1:] {
				if l == "" {
					break
				}
				vocabulary++
			}
		}
	}
	return examples, vocabulary
}

func TestTheNotePromptCarriesEightExamplesAndNeverMoreThanFiveHundredWords(t *testing.T) {
	a, _ := appWith(t, nil)
	withWorkspace(t, a)
	addNotes(t, a, "كِتَاب", "قَلَم", "بَيْت", "مَدْرَسَة", "سَرِيع", "يَوْم", "عَيْن", "ذَهَبَ", "قَالَ", "جَمِيل", "كَبِير", "مَاء")
	cases := []struct {
		allowRank           string
		wantVocabulary      int
		wantVocabularyShown bool
	}{{"1500", 500, true}, {"20", 20, true}, {"0", 0, false}}
	for _, tc := range cases {
		setUser(t, a, map[string]string{"author.allow_rank": tc.allowRank})
		system, err := a.AuthorSystem()
		if err != nil {
			t.Fatal(err)
		}
		examples, vocabulary := promptSections(system)
		if examples != 8 || vocabulary != tc.wantVocabulary || strings.Contains(system, "core vocabulary") != tc.wantVocabularyShown {
			t.Errorf("allow_rank %s: %d examples and %d vocabulary words, want 8 and %d", tc.allowRank, examples, vocabulary, tc.wantVocabulary)
		}
	}
}

func TestTheVocabularyCheckerExistsOnlyWhileTheLimitIsOn(t *testing.T) {
	a, _ := appWith(t, nil)
	withWorkspace(t, a)
	for allowRank, wantChecker := range map[string]bool{"1500": true, "0": false} {
		setUser(t, a, map[string]string{"author.allow_rank": allowRank})
		author, err := a.Author(nil)
		if err != nil {
			t.Fatal(err)
		}
		if (author.Vocabulary != nil) != wantChecker || author.AllowRank != map[string]int{"1500": 1500, "0": 0}[allowRank] {
			t.Errorf("allow_rank %s: checker %v, allow rank %d", allowRank, author.Vocabulary, author.AllowRank)
		}
	}
}
