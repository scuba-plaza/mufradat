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

type flagged struct {
	Kind     Kind
	Severity Severity
	Word     string
	Detail   string
	Missing  []int
}

func flags(issues []Issue) []flagged {
	var out []flagged
	for _, is := range issues {
		out = append(out, flagged{is.Kind, is.Severity, is.Word, is.Detail, is.Missing})
	}
	return out
}

func TestEvaluateSaysExactlyWhatIsWrongAndWhoSaidSo(t *testing.T) {
	const camelRejects = "CAMeL does not allow these vowels; it knows "
	for _, c := range []struct {
		name string
		item Item
		read Reading
		want []flagged
	}{
		{
			"a sentence word CAMeL rejects is flagged even when CATT agrees with the card",
			sentence("كُتَاب"),
			Reading{Analyses: [][]string{{"كِتَاب"}}, CATT: []string{"كُتَاب"}, BERT: []string{"كِتَاب"}},
			[]flagged{{KindInvalid, Major, "كُتَاب", camelRejects + "كِتَاب", nil}},
		},
		{
			"a citation form lacking letter marks names them",
			citation("كتاب"),
			Reading{Analyses: [][]string{{"كِتَاب"}}},
			[]flagged{
				{KindUnmarked, Major, "كتاب", "no vowel mark on ك، ت", []int{0, 1}},
				{KindInvalid, Major, "كتاب", camelRejects + "كِتَاب", nil},
			},
		},
		{
			"a sentence word lacking marks and an ending names both and quotes both readers",
			sentence("ولد"),
			Reading{Analyses: [][]string{{"وَلَد"}}, CATT: []string{"وَلَدٌ"}, BERT: []string{"وَلَدٌ"}},
			[]flagged{
				{KindUnmarked, Major, "ولد", "no vowel mark on و، ل and the ending; CATT reads وَلَدٌ; CAMeL reads وَلَدٌ", []int{0, 1, 2}},
				{KindInvalid, Major, "ولد", camelRejects + "وَلَد", nil},
			},
		},
		{
			"without CAMeL's reading only CATT is quoted",
			sentence("ولد"),
			Reading{Analyses: [][]string{{"وَلَد"}}, CATT: []string{"وَلَدٌ"}},
			[]flagged{
				{KindUnmarked, Major, "ولد", "no vowel mark on و، ل and the ending; CATT reads وَلَدٌ", []int{0, 1, 2}},
				{KindInvalid, Major, "ولد", camelRejects + "وَلَد", nil},
			},
		},
		{
			"a sentence word lacking only its ending says so, and that is the whole problem when CATT only adds it",
			sentence("وَلَد"),
			Reading{Analyses: [][]string{{"وَلَد"}}, CATT: []string{"وَلَدُ"}, BERT: []string{"وَلَدُ"}},
			[]flagged{{KindUnmarked, Major, "وَلَد", "the ending has no vowel mark; CATT reads وَلَدُ; CAMeL reads وَلَدُ", []int{2}}},
		},
		{
			"a missing ending is not the whole problem when CATT also changes a vowel the card has",
			sentence("وَلَد"),
			Reading{Analyses: [][]string{{"وَلَد"}}, CATT: []string{"وِلَدٌ"}, BERT: []string{"وِلَدٌ"}},
			[]flagged{
				{KindUnmarked, Major, "وَلَد", "the ending has no vowel mark; CATT reads وِلَدٌ; CAMeL reads وِلَدٌ", []int{2}},
				{KindDiacritics, Major, "وَلَد", "CATT reads وِلَدٌ; CAMeL reads وِلَدٌ", nil},
			},
		},
		{
			"CATT alone vouching for the stem is enough when CAMeL does not know the word",
			sentence("وَلَد"),
			Reading{Analyses: [][]string{nil}, CATT: []string{"وَلَدٌ"}, BERT: []string{"وَلَدٌ"}},
			[]flagged{{KindUnmarked, Major, "وَلَد", "the ending has no vowel mark; CATT reads وَلَدٌ; CAMeL reads وَلَدٌ", []int{2}}},
		},
		{
			"an unmarked letter is excused when CAMeL alone vouches for the stem",
			sentence("الشَّمسِ"),
			Reading{Analyses: [][]string{{"الشَّمْس"}}},
			[]flagged{{KindUnchecked, Major, "", "CATT's reading could not be aligned with the sentence", nil}},
		},
		{
			"an unmarked letter is excused when CATT alone vouches for the stem",
			sentence("الشَّمسِ"),
			Reading{Analyses: [][]string{nil}, CATT: []string{"الشَّمْسُ"}},
			[]flagged{{KindDiacritics, Major, "الشَّمسِ", "CATT reads الشَّمْسُ", nil}},
		},
		{
			"an unmarked letter is flagged when neither reader vouches for the stem",
			sentence("الشَّمسِ"),
			Reading{Analyses: [][]string{{"الشِّمْس"}}, CATT: []string{"الشِّمْسُ"}},
			[]flagged{
				{KindUnmarked, Major, "الشَّمسِ", "no vowel mark on م; CATT reads الشِّمْسُ", []int{3}},
				{KindInvalid, Major, "الشَّمسِ", camelRejects + "الشِّمْس", nil},
				{KindDiacritics, Major, "الشَّمسِ", "CATT reads الشِّمْسُ", nil},
			},
		},
		{
			"a sentence word CAMeL does not know and CATT agrees with is fine",
			sentence("ذَهَبَ"),
			Reading{Analyses: [][]string{nil}, CATT: []string{"ذَهَبَ"}, BERT: []string{"ذَهَبَ"}},
			nil,
		},
		{
			"only the first six readings are kept and four are quoted",
			citation("كُتَاب"),
			Reading{Analyses: [][]string{{"كِتَاب", "كِتَابٌ", "كَتَّاب", "كُتَّاب", "كَاتِب", "كُتُب", "كَتَب", "كَتَبَ"}}},
			[]flagged{{KindInvalid, Major, "كُتَاب", camelRejects + "كِتَاب، كِتَابٌ، كَتَّاب، كُتَّاب", nil}},
		},
	} {
		got := flags(EvaluateItem(c.item, c.read))
		if len(got) != len(c.want) {
			t.Errorf("%s: %d issues, want %d: %+v", c.name, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i].Kind != c.want[i].Kind || got[i].Severity != c.want[i].Severity || got[i].Word != c.want[i].Word || got[i].Detail != c.want[i].Detail || !slices.Equal(got[i].Missing, c.want[i].Missing) {
				t.Errorf("%s: issue %d is %+v, want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
	long := EvaluateItem(citation("كُتَاب"), Reading{Analyses: [][]string{{"كِتَاب", "كِتَابٌ", "كَتَّاب", "كُتَّاب", "كَاتِب", "كُتُب", "كَتَب", "كَتَبَ"}}})
	if len(long) != 1 || len(long[0].Known) != 6 {
		t.Errorf("known readings kept: %+v", long)
	}
}
