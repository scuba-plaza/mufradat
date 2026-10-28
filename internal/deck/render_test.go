package deck

import (
	"testing"

	"github.com/scuba-plaza/mufradat/internal/note"
)

func TestRankBucketsGroupFiveHundredRanksAndNameThemWithFourDigits(t *testing.T) {
	for rank, want := range map[int]string{
		-5: "rank::unranked", 0: "rank::unranked",
		1: "rank::0001-0500", 2: "rank::0001-0500", 499: "rank::0001-0500", 500: "rank::0001-0500",
		501: "rank::0501-1000", 1000: "rank::0501-1000",
		1001: "rank::1001-1500", 4999: "rank::4501-5000", 5000: "rank::4501-5000", 5001: "rank::5001-5500",
	} {
		if got := rankBucket(rank); got != want {
			t.Errorf("rankBucket(%d) = %q, want %q", rank, got, want)
		}
	}
}

func TestDetailsListGenderVerbFormAndRootInThatOrder(t *testing.T) {
	root := `<span class="ar" lang="ar" dir="rtl">ك ت ب</span>`
	for _, c := range []struct {
		name string
		n    note.Note
		want string
	}{
		{"nothing to say", note.Note{}, ""},
		{"masculine", note.Note{Gender: "m"}, "masc."},
		{"feminine", note.Note{Gender: "f"}, "fem."},
		{"either", note.Note{Gender: "m+f"}, "masc./fem."},
		{"an unknown gender is not shown", note.Note{Gender: "x"}, ""},
		{"a verb form", note.Note{VerbForm: "II"}, "form II"},
		{"a verb form is escaped", note.Note{VerbForm: "<b>"}, "form &lt;b&gt;"},
		{"a root", note.Note{Root: "ك ت ب"}, "root " + root},
		{"everything", note.Note{Gender: "f", VerbForm: "X", Root: "ك ت ب"}, "fem. · form X · root " + root},
		{"a verb form and a root", note.Note{VerbForm: "I", Root: "ك ت ب"}, "form I · root " + root},
	} {
		if got := details(c.n); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
