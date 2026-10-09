package clips

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

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
