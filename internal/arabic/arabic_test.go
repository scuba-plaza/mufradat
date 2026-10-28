package arabic

import "testing"

func TestIsLetterCoversTheBaseLettersAndExcludesMarksTatweelAndDigits(t *testing.T) {
	for _, c := range []struct {
		r    rune
		want bool
		what string
	}{
		{0x0620, false, "just before hamza"}, {0x0621, true, "hamza"}, {0x0627, true, "alif"}, {0x063A, true, "ghayn"}, {0x063B, false, "just after ghayn"},
		{0x0640, false, "tatweel"},
		{0x0641, true, "fa"}, {0x064A, true, "ya"}, {0x064B, false, "fathatan"}, {0x064E, false, "fatha"}, {0x0651, false, "shadda"},
		{0x0670, false, "superscript alef"}, {0x0671, true, "alef wasla"}, {0x06A9, true, "keheh"}, {0x06D3, true, "last extended letter"}, {0x06D4, false, "arabic full stop"},
		{'a', false, "latin"}, {'1', false, "digit"}, {0x0661, false, "arabic-indic digit"}, {' ', false, "space"},
	} {
		if got := IsLetter(c.r); got != c.want {
			t.Errorf("IsLetter(%U %s) = %v, want %v", c.r, c.what, got, c.want)
		}
	}
}

func TestHasArabicNeedsAtLeastOneLetter(t *testing.T) {
	for in, want := range map[string]bool{
		"": false, "abc": false, "123 !": false, "ً": false, "ـ": false,
		"abc ك": true, "كتاب": true, "x\u0621": true, "ٱ": true,
	} {
		if got := HasArabic(in); got != want {
			t.Errorf("HasArabic(%q) = %v, want %v", in, got, want)
		}
	}
}
