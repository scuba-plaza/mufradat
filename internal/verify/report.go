package verify

import (
	"github.com/scuba-plaza/mufradat/internal/note"
)

type NoteResult struct {
	Verified bool
	Issues   []Issue
}

type Report struct {
	Results map[string]NoteResult
	Pending []Item
}

type Dismissals interface {
	Dismissed(noteID, key string) bool
}

func Derive(notes []note.Note, readings map[string]Reading) Report {
	report := Report{Results: make(map[string]NoteResult, len(notes))}
	var missing []Item
	for _, n := range notes {
		result := NoteResult{Verified: true}
		for _, it := range Items(n) {
			r, ok := readings[it.Key]
			if !ok {
				result.Verified = false
				missing = append(missing, it)
				continue
			}
			result.Issues = append(result.Issues, EvaluateItem(it, r)...)
		}
		report.Results[n.ID] = result
	}
	report.Pending = Unique(missing)
	return report
}

func (r NoteResult) Open(noteID string, d Dismissals) []Issue {
	if d == nil {
		return r.Issues
	}
	var open []Issue
	for _, is := range r.Issues {
		if !d.Dismissed(noteID, is.Key()) {
			open = append(open, is)
		}
	}
	return open
}

type Counts struct {
	Unchecked int
	Major     int
	Minor     int
}

func (r Report) Count(d Dismissals) Counts {
	var c Counts
	for id, res := range r.Results {
		if !res.Verified {
			c.Unchecked++
			continue
		}
		major, minor := false, false
		for _, is := range res.Open(id, d) {
			if is.Severity == Major {
				major = true
			} else {
				minor = true
			}
		}
		switch {
		case major:
			c.Major++
		case minor:
			c.Minor++
		}
	}
	return c
}
