package verify

import (
	"slices"
	"testing"
)

func citation(tokens ...string) Item {
	return Item{Key: ItemKey(tokens, false), Field: FieldLemma, Tokens: tokens, Citation: true}
}

func sentence(tokens ...string) Item {
	return Item{Key: ItemKey(tokens, true), Field: FieldExample, Tokens: tokens, Context: true}
}

func kinds(issues []Issue) []Kind {
	out := make([]Kind, len(issues))
	for i, is := range issues {
		out[i] = is.Kind
	}
	return out
}

func TestEvaluateCitation(t *testing.T) {
	tests := []struct {
		name     string
		item     Item
		analyses [][]string
		want     []Kind
		severity Severity
	}{
		{"valid", citation("كِتَاب"), [][]string{{"كِتَاب", "كِتَابٌ"}}, nil, ""},
		{"no vowels at all", citation("كتاب"), [][]string{{"كِتَاب"}}, []Kind{KindUnmarked, KindInvalid}, Major},
		{"vowels CAMeL rejects", citation("كُتَاب"), [][]string{{"كِتَاب"}}, []Kind{KindInvalid}, Major},
		{"word CAMeL does not know", citation("كِتَاب"), [][]string{nil}, []Kind{KindUnknown}, Minor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateItem(tt.item, Reading{Key: tt.item.Key, Analyses: tt.analyses})
			if !slices.Equal(kinds(got), tt.want) {
				t.Fatalf("kinds = %v, want %v (%+v)", kinds(got), tt.want, got)
			}
			if len(got) > 0 && got[0].Severity != tt.severity {
				t.Fatalf("severity = %s, want %s", got[0].Severity, tt.severity)
			}
		})
	}
}

func TestEvaluateSentence(t *testing.T) {
	item := sentence("ذَهَبَ", "الْوَلَدُ")
	analyses := [][]string{{"ذَهَبَ"}, {"الْوَلَدُ"}}

	t.Run("CATT agrees", func(t *testing.T) {
		got := EvaluateItem(item, Reading{Analyses: analyses, CATT: []string{"ذَهَبَ", "الْوَلَدُ"}, BERT: []string{"ذَهَبَ", "الْوَلَدُ"}})
		if len(got) != 0 {
			t.Fatalf("unexpected issues: %+v", got)
		}
	})
	t.Run("CATT and CAMeL both disagree", func(t *testing.T) {
		got := EvaluateItem(item, Reading{Analyses: analyses, CATT: []string{"ذَهَبَ", "الْوَلَدِ"}, BERT: []string{"ذَهَبَ", "الْوَلَدِ"}})
		if len(got) != 1 || got[0].Kind != KindDiacritics || got[0].Severity != Major || got[0].Word != "الْوَلَدُ" {
			t.Fatalf("got %+v", got)
		}
		if got[0].CATT != "الْوَلَدِ" || got[0].CAMeL != "الْوَلَدِ" {
			t.Fatalf("readings not recorded: %+v", got[0])
		}
	})
	t.Run("CAMeL sides with the card", func(t *testing.T) {
		got := EvaluateItem(item, Reading{Analyses: analyses, CATT: []string{"ذَهَبَ", "الْوَلَدِ"}, BERT: []string{"ذَهَبَ", "الْوَلَدُ"}})
		if len(got) != 1 || got[0].Severity != Minor {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("CATT is missing", func(t *testing.T) {
		got := EvaluateItem(item, Reading{Analyses: analyses})
		if len(got) != 1 || got[0].Kind != KindUnchecked || got[0].Severity != Major || got[0].Word != "" {
			t.Fatalf("got %+v", got)
		}
		if got[0].Key() != "example:unchecked" {
			t.Fatalf("key = %q", got[0].Key())
		}
	})
	t.Run("CATT out of step with the sentence", func(t *testing.T) {
		got := EvaluateItem(item, Reading{Analyses: analyses, CATT: []string{"ذَهَبَ"}})
		if len(got) != 1 || got[0].Kind != KindUnchecked {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestIssueKey(t *testing.T) {
	if k := (Issue{Word: "كِتَاب", Field: FieldLemma, Kind: KindInvalid}).Key(); k != "كِتَاب" {
		t.Fatalf("key = %q", k)
	}
	if k := (Issue{Field: FieldExample, Kind: KindUnchecked}).Key(); k != "example:unchecked" {
		t.Fatalf("key = %q", k)
	}
}
