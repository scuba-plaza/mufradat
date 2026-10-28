package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/store"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

type state int

const (
	open state = iota
	kept
	edited
	rewritten
)

func (s state) String() string {
	return [...]string{"open", "kept", "edited", "rewritten"}[s]
}

type snapshot struct {
	state    state
	changed  state
	issues   []verify.Issue
	verified bool
}

type entry struct {
	id string
	snapshot
	asking      bool
	working     bool
	checking    bool
	request     int
	recheck     int
	cancel      context.CancelFunc
	since       time.Time
	proposal    *note.Note
	proposalRev string
	notice      string
	noticeTone  string
}

type step struct {
	id     string
	before snapshot
	prior  *note.Note
	after  string
	keys   []string
}

type userError struct{ error }

type conflictError struct{ error }

func userErrorf(format string, args ...any) error {
	return userError{fmt.Errorf(format, args...)}
}

type result struct {
	Message  string `json:"message,omitempty"`
	Tone     string `json:"tone,omitempty"`
	Show     string `json:"show,omitempty"`
	Conflict bool   `json:"conflict,omitempty"`
}

type edit struct {
	Lemma     string      `json:"lemma"`
	POS       note.POS    `json:"pos"`
	Gender    string      `json:"gender"`
	VerbForm  string      `json:"verb_form"`
	Root      string      `json:"root"`
	Forms     []note.Form `json:"forms"`
	English   string      `json:"english"`
	Hint      string      `json:"hint"`
	Example   string      `json:"example"`
	ExampleEN string      `json:"example_en"`
	Comment   string      `json:"comment"`
}

func editOf(n note.Note) edit {
	return edit{
		Lemma: n.Lemma, POS: n.POS, Gender: n.Gender, VerbForm: n.VerbForm, Root: n.Root, Forms: slices.Clone(n.Forms),
		English: n.English, Hint: n.Hint, Example: n.Example, ExampleEN: n.ExampleEN, Comment: n.Comment,
	}
}

func apply(n note.Note, e edit) note.Note {
	trim := strings.TrimSpace
	n.Lemma, n.POS, n.Gender, n.VerbForm, n.Root = trim(e.Lemma), e.POS, trim(e.Gender), trim(e.VerbForm), trim(e.Root)
	n.Forms = nil
	for _, f := range e.Forms {
		if trim(f.Label) == "" && trim(f.Arabic) == "" {
			continue
		}
		n.Forms = append(n.Forms, note.Form{Label: trim(f.Label), Arabic: trim(f.Arabic)})
	}
	n.English, n.Hint, n.Example, n.ExampleEN, n.Comment = trim(e.English), trim(e.Hint), trim(e.Example), trim(e.ExampleEN), trim(e.Comment)
	return n
}

func sameContent(a, b note.Note) bool {
	x, errA := json.Marshal(editOf(a))
	y, errB := json.Marshal(editOf(b))
	return errA == nil && errB == nil && bytes.Equal(x, y)
}

type session struct {
	mu      sync.Mutex
	ctx     context.Context
	deps    Deps
	filter  Filter
	now     func() time.Time
	entries []*entry
	byID    map[string]*entry
	history []step
}

func newSession(ctx context.Context, deps Deps, items []Item, filter Filter) *session {
	s := &session{ctx: ctx, deps: deps, filter: filter, now: deps.Now, byID: map[string]*entry{}}
	if s.now == nil {
		s.now = time.Now
	}
	for _, it := range items {
		e := &entry{id: it.ID, snapshot: snapshot{issues: slices.Clone(it.Issues), verified: it.Verified}}
		s.entries = append(s.entries, e)
		s.byID[it.ID] = e
	}
	return s
}

func (s *session) lookup(id, ifRev string, required bool) (*entry, note.Note, error) {
	e, ok := s.byID[id]
	if !ok {
		return nil, note.Note{}, userErrorf("that note is not part of this review")
	}
	set, err := s.deps.Notes.Load()
	if err != nil {
		return nil, note.Note{}, err
	}
	n, ok := set.Get(id)
	if !ok {
		return nil, note.Note{}, userErrorf("that note no longer exists in the workspace")
	}
	switch {
	case ifRev == "" && required:
		return nil, note.Note{}, userErrorf("the request did not say which version of the note it was made on")
	case ifRev != "" && n.Rev() != ifRev:
		return nil, note.Note{}, conflictError{errors.New("the note changed since you loaded it, so nothing was done; it is shown again as it is now")}
	}
	return e, n, nil
}

func (s *session) nextOpen(id string) string {
	from := slices.IndexFunc(s.entries, func(e *entry) bool { return e.id == id })
	for k := 1; k <= len(s.entries); k++ {
		e := s.entries[(from+k+len(s.entries))%len(s.entries)]
		if e.state == open {
			return e.id
		}
	}
	return ""
}

func (s *session) settle(e *entry) {
	e.proposal, e.proposalRev, e.notice, e.noticeTone = nil, "", "", ""
	e.recheck++
	e.checking = false
	if e.asking {
		e.cancel()
		e.asking = false
	}
}

func (s *session) modify(id, ifRev string, revert bool, fn func(*note.Note)) (note.Note, note.Note, error) {
	var before, after note.Note
	err := s.deps.Notes.Update(s.ctx, func(tx *store.Tx) error {
		cur, ok := tx.Notes().Get(id)
		if !ok {
			return store.ErrNotFound
		}
		before = cur
		var err error
		if revert {
			after, err = tx.Revert(id, ifRev, fn)
			return err
		}
		after, err = tx.Modify(id, ifRev, func(n *note.Note) error {
			fn(n)
			return nil
		})
		return err
	})
	switch {
	case errors.Is(err, store.ErrConflict):
		return before, after, conflictError{errors.New("the note changed since you loaded it, so nothing was done; it is shown again as it is now")}
	case errors.Is(err, store.ErrNotFound):
		return before, after, userErrorf("that note no longer exists in the workspace")
	}
	return before, after, err
}

func (s *session) change(e *entry, ifRev string, to state, fn func(*note.Note)) (note.Note, error) {
	before, after, err := s.modify(e.id, ifRev, false, fn)
	if err != nil {
		return note.Note{}, err
	}
	prior := before
	s.history = append(s.history, step{id: e.id, before: e.snapshot, prior: &prior, after: after.Rev()})
	s.settle(e)
	e.state, e.changed = to, to
	s.startRecheck(e, after)
	return after, nil
}

func (s *session) keep(id, ifRev string) (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, n, err := s.lookup(id, ifRev, true)
	if err != nil {
		return result{}, err
	}
	if e.state != open {
		return result{Show: s.nextOpen(id)}, nil
	}
	var keys []string
	for _, is := range e.issues {
		if key := is.Key(); !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	for i, key := range keys {
		if err := s.deps.Decisions.Dismiss(id, key); err != nil {
			for _, done := range keys[:i] {
				_ = s.deps.Decisions.Restore(id, done)
			}
			return result{}, fmt.Errorf("recording the decision failed, so nothing changed: %w", err)
		}
	}
	s.history = append(s.history, step{id: id, before: e.snapshot, keys: keys})
	s.settle(e)
	e.state = kept
	message := "✓ " + n.Lemma + " is right as it is"
	if len(keys) > 0 {
		message += "; its flags stay out of the next build"
	}
	return result{Message: message, Tone: "good", Show: s.nextOpen(id)}, nil
}

func (s *session) save(id, ifRev string, ed edit) (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, n, err := s.lookup(id, ifRev, true)
	if err != nil {
		return result{}, err
	}
	candidate := apply(n, ed)
	if sameContent(candidate, n) {
		return result{Message: "No changes", Show: id}, nil
	}
	if err := note.ValidateContent(candidate); err != nil {
		return result{}, userError{err}
	}
	after, err := s.change(e, ifRev, edited, func(c *note.Note) { *c = apply(*c, ed) })
	if err != nil {
		return result{}, err
	}
	return result{Message: "✎ Saved your changes to " + after.Lemma + s.rechecking(), Tone: "good", Show: s.nextOpen(id)}, nil
}

func (s *session) rechecking() string {
	if s.deps.Verifier == nil {
		return "; run check again before building"
	}
	return "; checking it again"
}

func (s *session) use(id, ifRev string) (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, n, err := s.lookup(id, ifRev, true)
	if err != nil {
		return result{}, err
	}
	switch {
	case e.proposal == nil:
		return result{}, userErrorf("there is no suggestion from Mistral for this note")
	case e.proposalRev != n.Rev():
		e.proposal, e.proposalRev = nil, ""
		return result{}, userErrorf("the note changed after Mistral wrote its suggestion; ask again")
	}
	ed := editOf(*e.proposal)
	candidate := apply(n, ed)
	if err := note.ValidateContent(candidate); err != nil {
		return result{}, userError{err}
	}
	after, err := s.change(e, ifRev, rewritten, func(c *note.Note) { *c = apply(*c, ed) })
	if err != nil {
		return result{}, err
	}
	return result{Message: "✦ Saved Mistral's version of " + after.Lemma + s.rechecking(), Tone: "mistral", Show: s.nextOpen(id)}, nil
}

func (s *session) drop(id string) (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, n, err := s.lookup(id, "", false)
	if err != nil {
		return result{}, err
	}
	e.proposal, e.proposalRev = nil, ""
	return result{Message: "Kept your version of " + n.Lemma, Show: id}, nil
}

func (s *session) ending(id, ifRev, source string) (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, n, err := s.lookup(id, ifRev, true)
	if err != nil {
		return result{}, err
	}
	var fix *endingFix
	if e.state == open {
		for _, f := range endingFixes(n, e.issues) {
			if f.Source == source {
				fix = &f
			}
		}
	}
	if fix == nil {
		return result{}, userErrorf("there is no ending to add to this note from %s", source)
	}
	example := fix.Example
	candidate := n
	candidate.Example = example
	if err := note.ValidateContent(candidate); err != nil {
		return result{}, userError{err}
	}
	after, err := s.change(e, ifRev, edited, func(c *note.Note) { c.Example = example })
	if err != nil {
		return result{}, err
	}
	whose := map[string]string{sourceCATT: "CATT's", sourceCAMeL: "CAMeL's", sourceBoth: "the"}[source]
	what := " ending: "
	if len(fix.Words) > 1 {
		what = " endings: "
	}
	return result{Message: "✎ Added " + whose + what + strings.Join(fix.Words, " ") + s.rechecking(), Tone: "good", Show: s.nextOpen(after.ID)}, nil
}

func (s *session) startRecheck(e *entry, n note.Note) {
	if s.deps.Verifier == nil {
		e.notice, e.noticeTone = "Run 'check' again before building: this note changed after the last check", "info"
		return
	}
	e.recheck++
	request := e.recheck
	e.checking = true
	go func() {
		report, err := s.deps.Verifier.Check(s.ctx, []note.Note{n})
		dismissed, derr := s.deps.Decisions.Load()
		s.mu.Lock()
		defer s.mu.Unlock()
		if e.recheck != request {
			return
		}
		e.checking = false
		if err == nil {
			err = derr
		}
		res, ok := report.Results[n.ID]
		switch {
		case err != nil:
			e.notice, e.noticeTone = "Could not check the change: "+err.Error()+"; run 'check' before building", "warn"
		case !ok || !res.Verified:
			e.verified = false
			e.notice, e.noticeTone = "The change could not be cross-checked; run 'check' before building", "warn"
		default:
			e.verified = true
			flags := filterIssues(res.Open(n.ID, dismissed), s.filter)
			if len(flags) == 0 {
				e.issues = nil
				e.notice, e.noticeTone = "✓ Checked again: nothing is flagged any more", "good"
				return
			}
			e.issues, e.state = flags, open
			e.notice, e.noticeTone = fmt.Sprintf("Still flagged after the change: %d to look at", len(flags)), "warn"
		}
	}()
}

func (s *session) ask(id string) (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, n, err := s.lookup(id, "", false)
	if err != nil {
		return result{}, err
	}
	switch {
	case s.deps.Rewriter == nil:
		return result{}, userErrorf("Mistral is not set up; set MISTRAL_API_KEY or run '%s auth mistral --import FILE'", s.deps.Prefix)
	case e.asking:
		return result{Show: id}, nil
	case e.state == edited || e.state == rewritten:
		return result{}, userErrorf("this note was changed in this review; undo the change to ask Mistral about the earlier version")
	}
	ctx, cancel := context.WithCancel(s.ctx)
	e.request++
	e.asking, e.cancel, e.since = true, cancel, s.now()
	e.proposal, e.proposalRev, e.notice, e.noticeTone = nil, "", "", ""
	request, feedback, rewriter := e.request, Feedback(e.issues), s.deps.Rewriter
	go func() {
		fresh, err := rewriter.Rewrite(ctx, n, feedback)
		s.mu.Lock()
		defer s.mu.Unlock()
		if !e.asking || e.request != request {
			return
		}
		e.asking = false
		cancel()
		switch {
		case err != nil:
			e.notice, e.noticeTone = "Mistral could not write a new version: "+err.Error(), "bad"
		case sameContent(fresh, n):
			e.notice, e.noticeTone = "Mistral would keep this note as it is", "info"
		default:
			e.proposal, e.proposalRev = &fresh, n.Rev()
		}
	}()
	return result{Show: id}, nil
}

func (s *session) stop(id string) (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, n, err := s.lookup(id, "", false)
	if err != nil {
		return result{}, err
	}
	if e.asking {
		e.cancel()
		e.asking = false
	}
	return result{Message: "Stopped asking Mistral about " + n.Lemma, Show: id}, nil
}

func (s *session) remake(id, ifRev, kind string) (result, error) {
	s.mu.Lock()
	if s.deps.Audio == nil {
		s.mu.Unlock()
		return result{}, userErrorf("this review cannot make audio; run 'mufradat audio' instead")
	}
	e, n, err := s.lookup(id, ifRev, false)
	if err != nil {
		s.mu.Unlock()
		return result{}, err
	}
	if e.working {
		s.mu.Unlock()
		return result{Show: id}, nil
	}
	e.working = true
	audio := s.deps.Audio
	s.mu.Unlock()
	err = audio.Remake(s.ctx, n, kind)
	s.mu.Lock()
	defer s.mu.Unlock()
	e.working = false
	if err != nil {
		return result{}, err
	}
	return result{Message: "♪ Made the " + kind + " audio of " + n.Lemma + " again; listen to it", Tone: "audio", Show: id}, nil
}

func (s *session) removeClip(id, ifRev, kind string) (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deps.Audio == nil {
		return result{}, userErrorf("this review cannot remove audio files")
	}
	_, n, err := s.lookup(id, ifRev, false)
	if err != nil {
		return result{}, err
	}
	if err := s.deps.Audio.Remove(n, kind); err != nil {
		return result{}, err
	}
	return result{Message: "♪ Removed the " + kind + " audio of " + n.Lemma + "; 'Remake' or 'mufradat audio' makes it again", Show: id}, nil
}

func (s *session) setVoice(id, name string) (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deps.Voices == nil {
		return result{}, userErrorf("this review cannot change the voice; use 'mufradat config set audio.voice'")
	}
	if name == "" {
		return result{}, userErrorf("no voice was given")
	}
	if name == s.deps.Voices.Current() {
		return result{Show: id}, nil
	}
	if err := s.deps.Voices.Set(name); err != nil {
		return result{}, err
	}
	return result{
		Message: "♪ " + name + " speaks every clip made from now on, here and in 'mufradat audio'; clips already made keep their voice until you remake them",
		Tone:    "audio", Show: id,
	}, nil
}

func (s *session) voices(ctx context.Context) (voicesView, error) {
	s.mu.Lock()
	voices := s.deps.Voices
	s.mu.Unlock()
	v := voicesView{Voices: []VoiceOption{}}
	if voices == nil {
		return v, nil
	}
	v.Voice = voices.Current()
	list, err := voices.List(ctx)
	if err != nil {
		return v, err
	}
	v.Voices = list
	return v, nil
}

func (s *session) undo() (result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.history) == 0 {
		return result{}, userErrorf("there is nothing to undo")
	}
	last := s.history[len(s.history)-1]
	e := s.byID[last.id]
	lemma := ""
	if last.prior != nil {
		prior := editOf(*last.prior)
		before, _, err := s.modify(last.id, last.after, true, func(c *note.Note) { *c = apply(*c, prior) })
		if err != nil {
			if errors.As(err, new(conflictError)) {
				return result{}, conflictError{errors.New("the note was changed again elsewhere, so the undo was not applied; it is shown as it is now")}
			}
			return result{}, err
		}
		lemma = before.Lemma
	} else {
		set, err := s.deps.Notes.Load()
		if err != nil {
			return result{}, err
		}
		if n, ok := set.Get(last.id); ok {
			lemma = n.Lemma
		}
		for i, key := range last.keys {
			if err := s.deps.Decisions.Restore(last.id, key); err != nil {
				for _, done := range last.keys[:i] {
					_ = s.deps.Decisions.Dismiss(last.id, done)
				}
				return result{}, fmt.Errorf("recording the decision failed, so nothing changed: %w", err)
			}
		}
	}
	s.history = s.history[:len(s.history)-1]
	s.settle(e)
	e.snapshot = last.before
	return result{Message: "↶ Undid your last change to " + lemma, Show: last.id}, nil
}

func (s *session) stopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		e.recheck++
		if e.asking {
			e.cancel()
			e.asking = false
		}
	}
}

func (s *session) summary() Summary {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count()
}

func (s *session) count() Summary {
	sum := Summary{Total: len(s.entries)}
	for _, e := range s.entries {
		switch e.state {
		case open:
			sum.Open++
		case kept:
			if e.changed == open {
				sum.Kept++
			}
		}
		switch e.changed {
		case edited:
			sum.Edited++
		case rewritten:
			sum.Rewritten++
		}
	}
	return sum
}

func isUserError(err error) bool {
	var u userError
	return errors.As(err, &u)
}

func isConflict(err error) bool {
	var c conflictError
	return errors.As(err, &c)
}
