package decisions

import (
	"fmt"
	"time"

	"github.com/scuba-plaza/mufradat/internal/jsonl"
)

type Kind string

const (
	DismissIssue Kind = "dismiss_issue"
	RestoreIssue Kind = "restore_issue"
)

type Decision struct {
	Kind Kind      `json:"kind"`
	Note string    `json:"note"`
	Key  string    `json:"key"`
	At   time.Time `json:"at"`
}

type Log struct {
	path string
	now  func() time.Time
}

func NewLog(path string, now func() time.Time) *Log { return &Log{path: path, now: now} }

type Set struct {
	dismissed map[string]bool
}

func setKey(noteID, key string) string { return noteID + "\x00" + key }

func (s *Set) Dismissed(noteID, key string) bool {
	if s == nil {
		return false
	}
	return s.dismissed[setKey(noteID, key)]
}

func (s *Set) Count() int {
	if s == nil {
		return 0
	}
	return len(s.dismissed)
}

func (l *Log) Load() (*Set, error) {
	entries, err := jsonl.ReadFile[Decision](l.path)
	if err != nil {
		return nil, err
	}
	set := &Set{dismissed: map[string]bool{}}
	for _, d := range entries {
		switch d.Kind {
		case DismissIssue:
			set.dismissed[setKey(d.Note, d.Key)] = true
		case RestoreIssue:
			delete(set.dismissed, setKey(d.Note, d.Key))
		default:
			return nil, fmt.Errorf("%s: unknown decision kind %q", l.path, d.Kind)
		}
	}
	return set, nil
}

func (l *Log) record(kind Kind, noteID, key string) error {
	if noteID == "" || key == "" {
		return fmt.Errorf("a decision needs a note and an issue key")
	}
	return jsonl.Append(l.path, Decision{Kind: kind, Note: noteID, Key: key, At: l.now().UTC().Truncate(time.Second)})
}

func (l *Log) Dismiss(noteID, key string) error { return l.record(DismissIssue, noteID, key) }

func (l *Log) Restore(noteID, key string) error { return l.record(RestoreIssue, noteID, key) }
