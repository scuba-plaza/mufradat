package clips

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/note"
)

type fakeSynth struct {
	mu    sync.Mutex
	calls []string
	fail  map[string]error
}

func (f *fakeSynth) Synthesize(ctx context.Context, item Item, v Voice) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, item.Text)
	err := f.fail[item.Text]
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return []byte(v.Name + ":" + item.Text), nil
}

type fatalErr struct{}

func (fatalErr) Error() string { return "credentials rejected" }
func (fatalErr) Fatal() bool   { return true }

func notesFor(texts ...string) []note.Note {
	out := make([]note.Note, len(texts))
	for i, t := range texts {
		out[i] = note.Note{ID: "n_aaaaaaaaaaa" + string(rune('a'+i)), Lemma: t}
	}
	return out
}

func TestPlanDedupesAndSkipsPresent(t *testing.T) {
	v := Voice{Name: "v1", Rate: 0.9}
	notes := notesFor("كِتَاب", "قَلَم", "كِتَاب")
	have := map[string]bool{Key(v, "قَلَم"): true}
	plan := NewPlan(notes, v, func(k string) bool { return have[k] })
	if plan.Wanted != 2 || plan.Present != 1 || len(plan.Missing) != 1 {
		t.Fatalf("plan %+v", plan)
	}
	if plan.Missing[0].Text != "كِتَاب" || plan.Chars != len([]rune("كِتَاب")) {
		t.Fatalf("missing %+v chars %d", plan.Missing, plan.Chars)
	}
}

func TestSynthesizeFillsOnlyMissing(t *testing.T) {
	store := NewStore(t.TempDir())
	v := Voice{Name: "v1", Rate: 0.9}
	notes := notesFor("كِتَاب", "قَلَم", "بَيْت")
	if err := store.Put(Key(v, "قَلَم"), []byte("old")); err != nil {
		t.Fatal(err)
	}
	synth := &fakeSynth{}
	var progress atomic.Int32
	plan := NewPlan(notes, v, store.Has)
	res, err := Synthesize(context.Background(), plan, store, synth, SynthOptions{Concurrency: 3, Progress: func(d, total int) {
		progress.Add(1)
		if total != 2 {
			t.Errorf("total %d", total)
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Made != 2 || len(res.Failed) != 0 || progress.Load() != 2 {
		t.Fatalf("result %+v progress %d", res, progress.Load())
	}
	if len(synth.calls) != 2 {
		t.Fatalf("calls %v", synth.calls)
	}
	again := NewPlan(notes, v, store.Has)
	if len(again.Missing) != 0 {
		t.Fatalf("still missing %v", again.Missing)
	}
}

func TestVoiceSwitchKeepsBothVoicesClips(t *testing.T) {
	store := NewStore(t.TempDir())
	a, b := Voice{Name: "A", Rate: 0.9}, Voice{Name: "B", Rate: 0.9}
	notes := notesFor("كِتَاب")
	synth := &fakeSynth{}
	run := func(v Voice) Plan {
		plan := NewPlan(notes, v, store.Has)
		if _, err := Synthesize(context.Background(), plan, store, synth, SynthOptions{Concurrency: 1}); err != nil {
			t.Fatal(err)
		}
		return plan
	}
	run(a)
	if p := NewPlan(notes, b, store.Has); len(p.Missing) != 1 {
		t.Fatalf("voice B must not reuse A's clip: %+v", p)
	}
	run(b)
	back := NewPlan(notes, a, store.Has)
	if len(back.Missing) != 0 {
		t.Fatalf("switching back to A must find A's clip: %+v", back)
	}
	raw := mustRead(t, store.Path(Key(a, "كِتَاب")))
	if raw != "A:كِتَاب" {
		t.Fatalf("A's clip holds %q", raw)
	}
}

func TestSynthesizeRecordsNonFatalFailuresAndContinues(t *testing.T) {
	store := NewStore(t.TempDir())
	v := Voice{Name: "v1", Rate: 0.9}
	boom := errors.New("bad request")
	synth := &fakeSynth{fail: map[string]error{"قَلَم": boom}}
	plan := NewPlan(notesFor("كِتَاب", "قَلَم", "بَيْت"), v, store.Has)
	res, err := Synthesize(context.Background(), plan, store, synth, SynthOptions{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Made != 2 || len(res.Failed) != 1 || res.Failed[0].Item.Text != "قَلَم" || !errors.Is(res.Failed[0].Err, boom) {
		t.Fatalf("result %+v", res)
	}
}

func TestSynthesizeStopsOnFatalError(t *testing.T) {
	store := NewStore(t.TempDir())
	v := Voice{Name: "v1", Rate: 0.9}
	synth := &fakeSynth{fail: map[string]error{"قَلَم": fatalErr{}}}
	plan := NewPlan(notesFor("كِتَاب", "قَلَم", "بَيْت", "دَار"), v, store.Has)
	res, err := Synthesize(context.Background(), plan, store, synth, SynthOptions{Concurrency: 1})
	var f fatalErr
	if !errors.As(err, &f) {
		t.Fatalf("error %v", err)
	}
	if res.Made != 1 || len(synth.calls) != 2 {
		t.Fatalf("made %d calls %v", res.Made, synth.calls)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("the fatal error must be returned, not listed: %+v", res.Failed)
	}
}

type blockingSynth struct{ started chan struct{} }

func (b blockingSynth) Synthesize(ctx context.Context, item Item, v Voice) ([]byte, error) {
	b.started <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSynthesizeHonoursCancellation(t *testing.T) {
	store := NewStore(t.TempDir())
	v := Voice{Name: "v1", Rate: 0.9}
	ctx, cancel := context.WithCancel(context.Background())
	synth := blockingSynth{started: make(chan struct{}, 4)}
	go func() {
		<-synth.started
		cancel()
	}()
	plan := NewPlan(notesFor("كِتَاب", "قَلَم", "بَيْت"), v, store.Has)
	res, err := Synthesize(ctx, plan, store, synth, SynthOptions{Concurrency: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v", err)
	}
	if res.Made != 0 || len(res.Failed) != 0 {
		t.Fatalf("result %+v", res)
	}
}

func TestOrphans(t *testing.T) {
	store := NewStore(t.TempDir())
	a, b := Voice{Name: "A", Rate: 0.9}, Voice{Name: "B", Rate: 0.9}
	notes := notesFor("كِتَاب")
	keep, stale, other := Key(a, "كِتَاب"), Key(a, "قَدِيم"), Key(b, "كِتَاب")
	for _, k := range []string{keep, stale, other} {
		if err := store.Put(k, []byte("12345")); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Orphans(store, notes, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("orphans %+v", got)
	}
	for _, o := range got {
		if o.Key == keep || o.Size != 5 {
			t.Fatalf("bad orphan %+v", o)
		}
	}
}

type blockedErr struct{}

func (blockedErr) Error() string { return "flagged by moderation" }
func (blockedErr) Blocked() bool { return true }

func TestSynthesizeSetsBlockedClipsApartAndKeepsGoing(t *testing.T) {
	store := NewStore(t.TempDir())
	v := Voice{Provider: "mistral", Name: "v1"}
	boom := errors.New("temporary")
	synth := &fakeSynth{fail: map[string]error{"قَلَم": blockedErr{}, "بَيْت": boom}}
	plan := NewPlan(notesFor("كِتَاب", "قَلَم", "بَيْت", "دَار"), v, store.Has)
	res, err := Synthesize(context.Background(), plan, store, synth, SynthOptions{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Made != 2 {
		t.Errorf("made %d, want 2: a blocked clip must not stop the rest", res.Made)
	}
	if len(res.Blocked) != 1 || res.Blocked[0].Item.Text != "قَلَم" {
		t.Errorf("blocked %+v", res.Blocked)
	}
	if len(res.Failed) != 1 || res.Failed[0].Item.Text != "بَيْت" || !errors.Is(res.Failed[0].Err, boom) {
		t.Errorf("failed %+v", res.Failed)
	}
	if len(synth.calls) != 4 {
		t.Errorf("each clip should be tried exactly once, calls %v", synth.calls)
	}
}

func TestIsBlockedLooksThroughWrappedErrors(t *testing.T) {
	if !IsBlocked(fmt.Errorf("clip: %w", blockedErr{})) {
		t.Error("a wrapped block must still be recognised")
	}
	if IsBlocked(errors.New("flagged")) || IsBlocked(nil) {
		t.Error("only errors that say they are blocks count")
	}
}

type barrierSynth struct {
	need    int
	mu      sync.Mutex
	arrived int
	release chan struct{}
}

func (b *barrierSynth) Synthesize(ctx context.Context, item Item, v Voice) ([]byte, error) {
	b.mu.Lock()
	b.arrived++
	if b.arrived == b.need {
		close(b.release)
	}
	b.mu.Unlock()
	select {
	case <-b.release:
		return []byte("audio"), nil
	case <-time.After(2 * time.Second):
		return nil, errors.New("the clips were not made at the same time")
	}
}

func TestSynthesizeMakesAsManyClipsAtOnceAsAsked(t *testing.T) {
	store := NewStore(t.TempDir())
	v := Voice{Name: "v1"}
	plan := NewPlan(notesFor("كِتَاب", "قَلَم", "بَيْت"), v, store.Has)
	res, err := Synthesize(context.Background(), plan, store, &barrierSynth{need: 3, release: make(chan struct{})}, SynthOptions{Concurrency: 3})
	if err != nil || res.Made != 3 || len(res.Failed) != 0 {
		t.Fatalf("made %d, failed %v, err %v", res.Made, res.Failed, err)
	}
}

type plainErr struct{ blocked, fatal bool }

func (plainErr) Error() string   { return "refused" }
func (e plainErr) Blocked() bool { return e.blocked }
func (e plainErr) Fatal() bool   { return e.fatal }
func failingEvery(e error) *fakeSynth {
	return &fakeSynth{fail: map[string]error{"كِتَاب": e, "قَلَم": e, "بَيْت": e}}
}

func TestSynthesizeTreatsErrorsThatDenyBeingBlockedOrFatalAsOrdinaryFailures(t *testing.T) {
	store := NewStore(t.TempDir())
	v := Voice{Name: "v1"}
	plan := NewPlan(notesFor("كِتَاب", "قَلَم", "بَيْت"), v, store.Has)
	synth := failingEvery(plainErr{})
	res, err := Synthesize(context.Background(), plan, store, synth, SynthOptions{Concurrency: 1})
	if err != nil {
		t.Fatalf("an error that says it is not fatal must not stop the run: %v", err)
	}
	if len(res.Failed) != 3 || len(res.Blocked) != 0 || len(synth.calls) != 3 {
		t.Fatalf("failed %d, blocked %d, attempted %d", len(res.Failed), len(res.Blocked), len(synth.calls))
	}
}

func TestSynthesizeStopsWhenAClipCannotBeSaved(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "audio")
	if err := os.WriteFile(dir, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(dir)
	v := Voice{Name: "v1"}
	plan := NewPlan(notesFor("كِتَاب", "قَلَم", "بَيْت"), v, store.Has)
	synth := &fakeSynth{}
	res, err := Synthesize(context.Background(), plan, store, synth, SynthOptions{Concurrency: 1})
	if err == nil || !strings.Contains(err.Error(), "saving ") || !strings.HasSuffix(strings.SplitN(err.Error(), ":", 2)[0], ".mp3") {
		t.Fatalf("error %v should name the clip that could not be saved", err)
	}
	if len(synth.calls) != 1 || res.Made != 0 || len(res.Failed) != 0 {
		t.Fatalf("attempted %d, made %d, failed %v: a full disk would fail every clip, so the run must stop", len(synth.calls), res.Made, res.Failed)
	}
}
