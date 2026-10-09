package review

import (
	"slices"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

type clipView struct {
	Kind  string `json:"kind"`
	Ready bool   `json:"ready"`
}

type entryView struct {
	ID         string       `json:"id"`
	Rev        string       `json:"rev"`
	State      string       `json:"state"`
	Changed    string       `json:"changed,omitempty"`
	Asking     bool         `json:"asking"`
	Checking   bool         `json:"checking"`
	Seconds    int          `json:"seconds"`
	Verified   bool         `json:"verified"`
	Notice     string       `json:"notice,omitempty"`
	NoticeTone string       `json:"notice_tone,omitempty"`
	Note       note.Note    `json:"note"`
	Card       cardView     `json:"card"`
	Flags      []flagView   `json:"flags"`
	Proposal   *note.Note   `json:"proposal,omitempty"`
	Changes    []changeView `json:"changes,omitempty"`
	Fixes      []endingFix  `json:"fixes,omitempty"`
	Clips      []clipView   `json:"clips"`
}

type listView struct {
	ID         string `json:"id"`
	Rank       int    `json:"rank,omitempty"`
	Lemma      string `json:"lemma"`
	English    string `json:"english"`
	State      string `json:"state"`
	Tone       string `json:"tone"`
	Flags      int    `json:"flags"`
	Asking     bool   `json:"asking"`
	Checking   bool   `json:"checking"`
	Ready      bool   `json:"ready"`
	Gone       bool   `json:"gone,omitempty"`
	Notice     string `json:"notice,omitempty"`
	NoticeTone string `json:"notice_tone,omitempty"`
}

type stateView struct {
	Summary Summary    `json:"summary"`
	Next    string     `json:"next,omitempty"`
	History int        `json:"history"`
	Claude  bool       `json:"claude"`
	Audio   bool       `json:"audio"`
	Verify  bool       `json:"verify"`
	Voice   string     `json:"voice,omitempty"`
	All     bool       `json:"all"`
	List    []listView `json:"list"`
	Entry   *entryView `json:"entry,omitempty"`
}

type voicesView struct {
	Voice  string        `json:"voice"`
	Voices []VoiceOption `json:"voices"`
}

func toneOf(issues []verify.Issue) string {
	if len(issues) == 0 {
		return "plain"
	}
	for _, is := range issues {
		if is.Severity == verify.Major {
			return "bad"
		}
	}
	return "warn"
}

func (s *session) view(id string) (stateView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, err := s.deps.Notes.Load()
	if err != nil {
		return stateView{}, err
	}
	sum := s.count()
	v := stateView{
		Summary: sum, Next: NextSteps(sum, s.deps.Prefix), History: len(s.history),
		Claude: s.deps.Rewriter != nil, Audio: s.deps.Audio != nil, Verify: s.deps.Verifier != nil,
		All: s.filter.All, List: []listView{},
	}
	if s.deps.Voices != nil {
		v.Voice = s.deps.Voices.Current()
	}
	for _, e := range s.entries {
		n, ok := set.Get(e.id)
		item := listView{
			ID: e.id, Rank: n.Rank, Lemma: n.Lemma, English: n.English, State: e.state.String(), Tone: toneOf(e.issues),
			Flags: len(e.issues), Asking: e.asking, Checking: e.checking, Ready: e.proposal != nil,
			Gone: !ok, Notice: e.notice, NoticeTone: e.noticeTone,
		}
		v.List = append(v.List, item)
		if e.id == id && ok {
			ev := s.entryView(e, n)
			v.Entry = &ev
		}
	}
	return v, nil
}

func (s *session) entryView(e *entry, n note.Note) entryView {
	v := entryView{
		ID: e.id, Rev: n.Rev(), State: e.state.String(), Asking: e.asking, Checking: e.checking,
		Verified: e.verified, Notice: e.notice, NoticeTone: e.noticeTone,
		Note: n, Flags: []flagView{}, Proposal: e.proposal, Clips: []clipView{},
	}
	if e.changed != open {
		v.Changed = e.changed.String()
	}
	if e.asking {
		v.Seconds = int(s.now().Sub(e.since).Seconds())
	}
	var flagged []verify.Issue
	if e.state == open {
		flagged = e.issues
	}
	v.Card = cardOf(n, flagged)
	for _, is := range e.issues {
		v.Flags = append(v.Flags, issueView(is, e.state != open))
	}
	if e.proposal != nil {
		v.Changes = changes(n, *e.proposal)
	}
	if e.state == open {
		v.Fixes = endingFixes(n, e.issues)
	}
	if s.deps.Audio != nil {
		for _, c := range s.deps.Audio.Clips(n) {
			v.Clips = append(v.Clips, clipView(c))
		}
	}
	return v
}

func clipKind(kind string) bool {
	return slices.Contains([]string{"word", "forms", "sentence"}, kind)
}
