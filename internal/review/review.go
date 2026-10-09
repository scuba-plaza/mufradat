package review

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/scuba-plaza/mufradat/internal/decisions"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/store"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

type Notes interface {
	Load() (*note.Set, error)
	Update(ctx context.Context, fn func(*store.Tx) error) error
}

type Decisions interface {
	Load() (*decisions.Set, error)
	Dismiss(noteID, key string) error
	Restore(noteID, key string) error
}

type Verifier interface {
	Check(ctx context.Context, notes []note.Note) (verify.Report, error)
}

type Rewriter interface {
	Rewrite(ctx context.Context, n note.Note, feedback string) (note.Note, error)
}

type ClipInfo struct {
	Kind  string
	Ready bool
}

type Audio interface {
	Clips(n note.Note) []ClipInfo
	Path(n note.Note, kind string) (string, bool)
	Remake(ctx context.Context, n note.Note, kind string) error
	Remove(n note.Note, kind string) error
}

type VoiceOption struct {
	Name   string `json:"name"`
	Tier   string `json:"tier,omitempty"`
	Gender string `json:"gender,omitempty"`
}

type Voices interface {
	Current() string
	List(ctx context.Context) ([]VoiceOption, error)
	Set(name string) error
}

type Deps struct {
	Notes     Notes
	Decisions Decisions
	Verifier  Verifier
	Rewriter  Rewriter
	Audio     Audio
	Voices    Voices
	Font      []byte
	Prefix    string
	Now       func() time.Time
}

type Filter struct {
	Minor bool
	All   bool
}

type Item struct {
	ID       string
	Issues   []verify.Issue
	Verified bool
}

func Items(notes []note.Note, report verify.Report, d verify.Dismissals, filter Filter) []Item {
	var items []Item
	for _, n := range notes {
		res := report.Results[n.ID]
		it := Item{ID: n.ID, Verified: res.Verified}
		if res.Verified {
			it.Issues = filterIssues(res.Open(n.ID, d), filter)
		}
		if len(it.Issues) > 0 || filter.All {
			items = append(items, it)
		}
	}
	return items
}

func filterIssues(issues []verify.Issue, filter Filter) []verify.Issue {
	var out []verify.Issue
	for _, is := range issues {
		if filter.Minor || is.Severity == verify.Major {
			out = append(out, is)
		}
	}
	return out
}

func Feedback(issues []verify.Issue) string {
	var b strings.Builder
	if len(issues) == 0 {
		b.WriteString("Nothing was flagged on this card; it is being gone over anyway.\n")
	} else {
		b.WriteString("An automatic check disagreed with this card:\n")
	}
	for _, is := range issues {
		if is.Word != "" {
			fmt.Fprintf(&b, "- %s in the %s: %s\n", is.Word, fieldName(is.Field), is.Detail)
		} else {
			fmt.Fprintf(&b, "- the %s: %s\n", fieldName(is.Field), is.Detail)
		}
	}
	b.WriteString("If the card is wrong, correct it. If it is right but the example can be read another way without the vowel marks, rewrite the example so it reads only one way.")
	return b.String()
}

type Summary struct {
	Total     int `json:"total"`
	Kept      int `json:"kept"`
	Edited    int `json:"edited"`
	Rewritten int `json:"rewritten"`
	Open      int `json:"open"`
}

func (s Summary) Changed() int {
	return s.Edited + s.Rewritten
}

func NextSteps(s Summary, prefix string) string {
	switch {
	case s.Changed() > 0:
		return fmt.Sprintf("Next: `%s audio` makes clips for any changed text, then `%s build`.", prefix, prefix)
	case s.Kept > 0:
		return fmt.Sprintf("Next: `%s build` leaves the flags you cleared out of the deck.", prefix)
	}
	return ""
}
