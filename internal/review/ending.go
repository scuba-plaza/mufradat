package review

import (
	"slices"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

const (
	sourceCATT  = "catt"
	sourceCAMeL = "camel"
	sourceBoth  = "both"
)

type endingFix struct {
	Source  string   `json:"source"`
	Words   []string `json:"words"`
	Example string   `json:"-"`
}

func bareEnding(is verify.Issue) bool {
	return is.Kind == verify.KindUnmarked && is.Field == verify.FieldExample && slices.Equal(is.Missing, []int{tashkeel.EndingIndex(is.Word)})
}

func withEndings(example string, issues []verify.Issue, reading func(verify.Issue) string) (string, []string, bool) {
	ps := pieces(example)
	if markupOf(ps) != example {
		return "", nil, false
	}
	var found []string
	next := 0
	for _, is := range issues {
		if !bareEnding(is) {
			continue
		}
		at, marks, ok := tashkeel.EndingMarks(is.Word, reading(is))
		if !ok {
			return "", nil, false
		}
		k := next
		for k < len(ps) && !(ps[k].word && ps[k].String() == is.Word) {
			k++
		}
		if k == len(ps) {
			return "", nil, false
		}
		p := &ps[k]
		flag := p.target[max(at-1, 0)]
		p.runes = slices.Insert(p.runes, at, marks...)
		p.target = slices.Insert(p.target, at, slices.Repeat([]bool{flag}, len(marks))...)
		found = append(found, p.String())
		next = k + 1
	}
	if len(found) == 0 {
		return "", nil, false
	}
	return markupOf(ps), found, true
}

func endingFixes(n note.Note, issues []verify.Issue) []endingFix {
	var out []endingFix
	for _, source := range []string{sourceCATT, sourceCAMeL} {
		example, found, ok := withEndings(n.Example, issues, func(is verify.Issue) string {
			if source == sourceCATT {
				return is.CATT
			}
			return is.CAMeL
		})
		if !ok {
			continue
		}
		if len(out) == 1 && out[0].Example == example {
			out[0].Source = sourceBoth
			continue
		}
		out = append(out, endingFix{Source: source, Words: found, Example: example})
	}
	return out
}
