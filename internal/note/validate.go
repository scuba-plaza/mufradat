package note

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/scuba-plaza/mufradat/internal/arabic"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

func Validate(n Note) error {
	if n.ID == "" {
		return errors.Join(errors.New("id is empty"), ValidateContent(n))
	}
	return ValidateContent(n)
}

func ValidateContent(n Note) error {
	var problems []error
	add := func(format string, args ...any) { problems = append(problems, fmt.Errorf(format, args...)) }

	if !n.POS.Valid() {
		add("pos %q is not one of %s", n.POS, strings.Join(POSStrings(), ", "))
	}
	if strings.TrimSpace(n.English) == "" {
		add("english is empty")
	}
	if err := arabicOnly("the headword", n.Lemma); err != nil {
		add("%v", err)
	}
	for _, f := range n.Forms {
		switch {
		case strings.TrimSpace(f.Label) == "":
			add("the form %s has no label", f.Arabic)
		default:
			if err := arabicOnly("the form "+f.Label, f.Arabic); err != nil {
				add("%v", err)
			}
		}
	}
	validateExample(n, add)
	if strings.TrimSpace(n.ExampleEN) == "" {
		add("example_en is empty")
	}
	return errors.Join(problems...)
}

func validateExample(n Note, add func(string, ...any)) {
	parts, err := SplitExample(n.Example)
	if err != nil {
		add("%v", err)
		return
	}
	plain := parts.Plain()
	if err := arabicOnly("the example", plain); err != nil {
		add("%v", err)
		return
	}
	if bare := BareEndings(plain); len(bare) > 0 {
		add("the example leaves the ending of %s without a vowel mark, but every word needs its case or mood ending, the last one too", strings.Join(bare, "، "))
	}
}

func BareEndings(sentence string) []string {
	var out []string
	for _, w := range tashkeel.Words(sentence) {
		if slices.ContainsFunc(tashkeel.UnmarkedLetters(w, false), func(m tashkeel.MissingMark) bool { return m.Ending }) {
			out = append(out, w)
		}
	}
	return out
}

func arabicOnly(field, s string) error {
	switch {
	case !arabic.HasArabic(s):
		return fmt.Errorf("%s has no Arabic text", field)
	case strings.ContainsAny(s, "<>"):
		return fmt.Errorf("%s contains markup", field)
	case strings.ContainsFunc(s, func(r rune) bool { return unicode.In(r, unicode.Latin) }):
		return fmt.Errorf("%s contains Latin letters", field)
	}
	return nil
}
