package review

import (
	"slices"
	"testing"
)

func TestPiecesFollowTheBrackets(t *testing.T) {
	ps := pieces("وَ[الْبَيْتُ] كَبِيرٌ.")
	if got := words(ps); !slices.Equal(got, []string{"وَالْبَيْتُ", "كَبِيرٌ"}) {
		t.Fatalf("words = %q", got)
	}
	if ps[0].target[0] || !ps[0].target[len(ps[0].target)-1] {
		t.Errorf("target = %v", ps[0].target)
	}
	if last := ps[len(ps)-1]; last.word || last.String() != "." {
		t.Errorf("last piece = %+v", last)
	}
	if got := markupOf(ps); got != "وَ[الْبَيْتُ] كَبِيرٌ." {
		t.Errorf("markup does not round-trip: %q", got)
	}
}

func TestMarkupOfClosesAnOpenTarget(t *testing.T) {
	if got := markupOf(pieces("قَرَأَ [الْكِتَابَ")); got != "قَرَأَ [الْكِتَابَ]" {
		t.Errorf("got %q", got)
	}
}

func TestCommonFindsTheSharedWords(t *testing.T) {
	a := []string{"وَجَدْتُ", "الْمَعْلُومَاتِ", "فِي", "الْمَوْقِعِ"}
	b := []string{"وَجَدَ", "الطَّالِبُ", "الْمَعْلُومَاتِ", "فِي", "الْمَوْقِعِ"}
	inA, inB := common(a, b)
	if !slices.Equal(inA, []bool{false, true, true, true}) || !slices.Equal(inB, []bool{false, false, true, true, true}) {
		t.Errorf("common = %v %v", inA, inB)
	}
}
