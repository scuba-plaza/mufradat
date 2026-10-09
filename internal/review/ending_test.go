package review

import (
	"slices"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

func ending(word string, missing int, catt, camel string) verify.Issue {
	return verify.Issue{Field: verify.FieldExample, Kind: verify.KindUnmarked, Severity: verify.Major, Word: word, Missing: []int{missing}, CATT: catt, CAMeL: camel}
}

func TestEndingFixesAddOnlyTheEnding(t *testing.T) {
	n := note.Note{Example: "[صَبَاح الخَيْر]، كَيْفَ حَالُكَ؟"}
	issues := []verify.Issue{ending("صَبَاح", 3, "صَبَاحُ", "صَباحُ"), ending("الخَيْر", 4, "الْخَيْرِ", "الخَيْرِ")}
	fixes := endingFixes(n, issues)
	if len(fixes) != 1 || fixes[0].Source != sourceBoth || fixes[0].Example != "[صَبَاحُ الخَيْرِ]، كَيْفَ حَالُكَ؟" {
		t.Fatalf("when CATT and CAMeL agree there is one fix that keeps the card's other marks: %+v", fixes)
	}
	if !slices.Equal(fixes[0].Words, []string{"صَبَاحُ", "الخَيْرِ"}) {
		t.Errorf("words = %q", fixes[0].Words)
	}

	n.Example = "بَدَا أَنَّهُ [تَعِب]."
	fixes = endingFixes(n, []verify.Issue{ending("تَعِب", 2, "تَعَبٌ", "تَعِبَ")})
	if len(fixes) != 2 || fixes[0].Source != sourceCATT || fixes[0].Example != "بَدَا أَنَّهُ [تَعِبٌ]." || fixes[1].Example != "بَدَا أَنَّهُ [تَعِبَ]." {
		t.Fatalf("fixes = %+v", fixes)
	}

	for _, example := range []string{"بَدَا أَنَّهُ [تَعِبٌ].", "بَدَا أَنَّهُ [[تَعِب]].", "بَدَا أَنَّهُ [تَعِب]. ]"} {
		n.Example = example
		if fixes := endingFixes(n, []verify.Issue{ending("تَعِب", 2, "تَعَبٌ", "")}); len(fixes) != 0 {
			t.Errorf("%s: no fix when the sentence no longer has the bare word or the markup is broken: %+v", example, fixes)
		}
	}
	n.Example = "بَدَا أَنَّهُ [تَعِب]."
	inside := verify.Issue{Field: verify.FieldExample, Kind: verify.KindUnmarked, Severity: verify.Major, Word: "تَعِب", Missing: []int{0, 2}, CATT: "تَعِبٌ"}
	if fixes := endingFixes(n, []verify.Issue{inside}); len(fixes) != 0 {
		t.Errorf("a flag with more than the ending missing needs a real edit: %+v", fixes)
	}
}
