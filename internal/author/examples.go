package author

import (
	"slices"

	"github.com/scuba-plaza/mufradat/internal/note"
)

func PickExamples(notes []note.Note, count int) []note.Note {
	ordered := slices.Clone(notes)
	slices.SortFunc(ordered, note.CompareStudy)
	var out []note.Note
	used := map[string]bool{}
	havePOS := map[note.POS]bool{}
	for _, distinct := range []bool{true, false} {
		for _, n := range ordered {
			if len(out) == count {
				break
			}
			if used[n.ID] || (distinct && havePOS[n.POS]) {
				continue
			}
			used[n.ID] = true
			havePOS[n.POS] = true
			out = append(out, n)
		}
	}
	slices.SortFunc(out, note.CompareStudy)
	return out
}
