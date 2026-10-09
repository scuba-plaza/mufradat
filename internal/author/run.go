package author

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/prompt"
	"github.com/scuba-plaza/mufradat/internal/store"
)

const (
	DefaultBatch       = 10
	DefaultConcurrency = 2
	DefaultAttempts    = 2
)

type Notes interface {
	Update(ctx context.Context, fn func(*store.Tx) error) error
}

type Event struct {
	Done    int
	Total   int
	Target  Target
	Note    note.Note
	Err     error
	Skipped bool
}

type Options struct {
	Batch       int
	Concurrency int
	Attempts    int
	System      string
	Progress    func(Event)
}

type Failure struct {
	Target Target
	Err    error
}

type Result struct {
	Written   []note.Note
	Skipped   int
	Failed    []Failure
	Unwritten []Target
	Usage     llm.Usage
}

type Author struct {
	LLM        llm.Backend
	Notes      Notes
	Schema     []byte
	Vocabulary Vocabulary
	AllowRank  int
}

type runState struct {
	mu      sync.Mutex
	res     Result
	done    int
	total   int
	targets []Target
	opts    Options
}

func (s *runState) finish(t Target, saved note.Note, skipped bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done++
	switch {
	case err != nil:
		s.res.Failed = append(s.res.Failed, Failure{Target: t, Err: err})
	case skipped:
		s.res.Skipped++
	default:
		s.res.Written = append(s.res.Written, saved)
	}
	if s.opts.Progress != nil {
		s.opts.Progress(Event{Done: s.done, Total: s.total, Target: t, Note: saved, Err: err, Skipped: skipped})
	}
}

func (s *runState) addUsage(u llm.Usage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.res.Usage.Add(u)
}

func (a Author) Run(ctx context.Context, targets []Target, opts Options) (Result, error) {
	if opts.Batch <= 0 {
		opts.Batch = DefaultBatch
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = DefaultConcurrency
	}
	if opts.Attempts <= 0 {
		opts.Attempts = DefaultAttempts
	}
	state := &runState{total: len(targets), targets: targets, opts: opts}
	handled := make([]bool, len(targets))
	var handledMu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.Concurrency)
	indices := make([]int, len(targets))
	for i := range indices {
		indices[i] = i
	}
	for batch := range slices.Chunk(indices, opts.Batch) {
		g.Go(func() error {
			if gctx.Err() != nil {
				return nil
			}
			return a.runBatch(gctx, state, batch, func(i int) {
				handledMu.Lock()
				handled[i] = true
				handledMu.Unlock()
			})
		})
	}
	err := g.Wait()
	for i, t := range targets {
		if !handled[i] {
			state.res.Unwritten = append(state.res.Unwritten, t)
		}
	}
	slices.SortFunc(state.res.Failed, func(x, y Failure) int { return strings.Compare(x.Target.Draft.Lemma, y.Target.Draft.Lemma) })
	return state.res, err
}

func fatal(err error) bool {
	_, limit := llm.AsUsageLimit(err)
	return limit || llm.AsAuth(err) || llm.AsUnavailable(err)
}

func (a Author) runBatch(ctx context.Context, s *runState, batch []int, markHandled func(int)) error {
	pending := slices.Clone(batch)
	problems := map[int]error{}
	for attempt := 0; attempt < s.opts.Attempts && len(pending) > 0; attempt++ {
		drafts := make([]prompt.Draft, len(pending))
		for k, i := range pending {
			t := s.targets[i]
			notice := t.Notice
			if p := problems[i]; p != nil {
				if notice != "" {
					notice += "\n"
				}
				notice += "A previous answer for this card was rejected: " + p.Error() + ". Correct it."
			}
			drafts[k] = prompt.Draft{Ref: k + 1, Card: prompt.CardOf(t.Draft, k+1), Rank: t.Draft.Rank, Entries: t.Entries, Notice: notice}
		}
		resp, err := a.LLM.Complete(ctx, llm.Request{System: s.opts.System, Prompt: prompt.User(drafts), Schema: a.Schema})
		var cards map[int]prompt.Card
		var parseErr error
		switch {
		case err == nil:
			s.addUsage(resp.Usage)
			cards, parseErr = prompt.ParseCards(resp.Output)
		case fatal(err) || ctx.Err() != nil:
			return err
		case isCallError(err):
			for _, i := range pending {
				markHandled(i)
				s.finish(s.targets[i], note.Note{}, false, err)
			}
			return nil
		default:
			parseErr = err
		}
		var retry []int
		for k, i := range pending {
			t := s.targets[i]
			card, ok := cards[k+1]
			problem := parseErr
			if problem == nil && !ok {
				problem = errors.New("the answer had no card for this ref")
			}
			if problem == nil {
				saved, skipped, perr := a.accept(ctx, t, card)
				if perr == nil {
					markHandled(i)
					s.finish(t, saved, skipped, nil)
					continue
				}
				if fatalSave(perr) {
					return perr
				}
				problem = perr
			}
			problems[i] = problem
			retry = append(retry, i)
		}
		pending = retry
	}
	for _, i := range pending {
		markHandled(i)
		s.finish(s.targets[i], note.Note{}, false, problems[i])
	}
	return nil
}

type saveError struct{ err error }

func (e saveError) Error() string { return e.err.Error() }
func (e saveError) Unwrap() error { return e.err }

func fatalSave(err error) bool {
	var s saveError
	return errors.As(err, &s)
}

func Apply(draft note.Note, c prompt.Card) note.Note {
	trim := strings.TrimSpace
	n := draft
	n.Lemma = trim(c.Arabic)
	if pos := note.POS(trim(c.POS)); pos != draft.POS {
		if pos == note.Verb {
			n.Gender = ""
		} else {
			n.VerbForm = ""
		}
		n.POS = pos
	}
	n.English = trim(c.English)
	n.Hint = trim(c.Hint)
	n.Forms = nil
	for _, f := range c.Forms {
		n.Forms = append(n.Forms, note.Form{Label: trim(f.Label), Arabic: trim(f.Arabic)})
	}
	n.Example = trim(c.Example)
	n.ExampleEN = trim(c.ExampleEN)
	n.Comment = trim(c.Comment)
	return n
}

func (a Author) accept(ctx context.Context, t Target, card prompt.Card) (note.Note, bool, error) {
	n := Apply(t.Draft, card)
	if err := note.ValidateContent(n); err != nil {
		return note.Note{}, false, err
	}
	if err := Gate(t, n); err != nil {
		return note.Note{}, false, err
	}
	if err := a.checkVocabulary(n); err != nil {
		return note.Note{}, false, err
	}
	var saved note.Note
	skipped := false
	err := a.Notes.Update(ctx, func(tx *store.Tx) error {
		set := tx.Notes()
		if n.Rank > 0 {
			if _, taken := set.ByRank(n.Rank); taken {
				skipped = true
				return nil
			}
		}
		if _, exists := set.ByLexeme(n.LexemeKey()); exists {
			skipped = true
			return nil
		}
		var err error
		saved, err = tx.Add(n)
		return err
	})
	if err != nil {
		return note.Note{}, false, saveError{fmt.Errorf("saving %s: %w", n.Lemma, err)}
	}
	return saved, skipped, nil
}

func isCallError(err error) bool {
	var c *llm.CallError
	return errors.As(err, &c)
}
