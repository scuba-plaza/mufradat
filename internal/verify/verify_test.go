package verify

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/note"
)

func sampleNote(id, lemma, example string) note.Note {
	return note.Note{ID: id, Lemma: lemma, POS: note.Noun, English: "x", Example: example, ExampleEN: "y"}
}

func TestItems(t *testing.T) {
	n := sampleNote("n_a", "كِتَاب", "قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ.")
	n.Forms = []note.Form{{Label: "pl.", Arabic: "كُتُب"}}
	items := Items(n)
	if len(items) != 3 {
		t.Fatalf("got %d items: %+v", len(items), items)
	}
	fields := []string{items[0].Field, items[1].Field, items[2].Field}
	if !slices.Equal(fields, []string{FieldLemma, FieldForms, FieldExample}) {
		t.Fatalf("fields = %v", fields)
	}
	if !items[0].Citation || !items[1].Citation || items[2].Citation || !items[2].Context {
		t.Fatalf("citation/context flags wrong: %+v", items)
	}
	if !slices.Equal(items[2].Tokens, []string{"قَرَأْتُ", "الْكِتَابَ", "فِي", "الْبَيْتِ"}) {
		t.Fatalf("example tokens = %v", items[2].Tokens)
	}
}

func TestItemKeyDependsOnContentOnly(t *testing.T) {
	a := ItemKey([]string{"كِتَاب"}, false)
	if a != ItemKey([]string{"كِتَاب"}, false) {
		t.Fatal("key is not stable")
	}
	if a == ItemKey([]string{"كِتَاب"}, true) {
		t.Fatal("context and citation checks must not share a key")
	}
	if a == ItemKey([]string{"كُتُب"}, false) {
		t.Fatal("different words must not share a key")
	}
	if ItemKey([]string{"ab", "c"}, false) == ItemKey([]string{"a", "bc"}, false) {
		t.Fatal("token boundaries must be part of the key")
	}
}

func TestDeriveReportsPendingOnceAndVerifiedNotes(t *testing.T) {
	a := sampleNote("n_a", "كِتَاب", "")
	b := sampleNote("n_b", "كِتَاب", "")
	c := sampleNote("n_c", "قَلَم", "")
	readings := map[string]Reading{
		Items(a)[0].Key: {Key: Items(a)[0].Key, Analyses: [][]string{{"كِتَاب"}}},
	}
	report := Derive([]note.Note{a, b, c}, readings)
	if !report.Results["n_a"].Verified || !report.Results["n_b"].Verified || report.Results["n_c"].Verified {
		t.Fatalf("verified flags wrong: %+v", report.Results)
	}
	if len(report.Pending) != 1 || report.Pending[0].Tokens[0] != "قَلَم" {
		t.Fatalf("pending = %+v", report.Pending)
	}
	counts := report.Count(nil)
	if counts.Unchecked != 1 || counts.Major != 0 || counts.Minor != 0 {
		t.Fatalf("counts = %+v", counts)
	}
}

type dismissed map[string]bool

func (d dismissed) Dismissed(noteID, key string) bool { return d[noteID+"/"+key] }

func TestCountHonoursDismissals(t *testing.T) {
	n := sampleNote("n_a", "كتاب", "")
	it := Items(n)[0]
	readings := map[string]Reading{it.Key: {Key: it.Key, Analyses: [][]string{{"كِتَاب"}}}}
	report := Derive([]note.Note{n}, readings)
	if c := report.Count(nil); c.Major != 1 {
		t.Fatalf("counts = %+v", c)
	}
	if c := report.Count(dismissed{"n_a/كتاب": true}); c.Major != 0 || c.Minor != 0 || c.Unchecked != 0 {
		t.Fatalf("dismissed counts = %+v", c)
	}
}

func TestCacheRoundTripAndFingerprint(t *testing.T) {
	cache := NewCache(filepath.Join(t.TempDir(), "state", "readings.jsonl"))
	loaded, err := cache.Load("fp1")
	if err != nil || len(loaded.Readings) != 0 {
		t.Fatalf("empty cache: %v %+v", err, loaded)
	}
	r1 := Reading{Key: "k1", Analyses: [][]string{{"كِتَاب"}}, CATT: []string{"كِتَاب"}}
	r2 := Reading{Key: "k2", Analyses: [][]string{nil}}
	if err := cache.Append("fp1", []Reading{r1, r2}); err != nil {
		t.Fatal(err)
	}
	if err := cache.Append("fp2", []Reading{{Key: "k1", Analyses: [][]string{{"x"}}}}); err != nil {
		t.Fatal(err)
	}
	loaded, err = cache.Load("fp1")
	if err != nil || len(loaded.Readings) != 2 || loaded.Stale != 1 {
		t.Fatalf("fp1: %v %+v", err, loaded)
	}
	if got := loaded.Readings["k1"]; !slices.Equal(got.CATT, []string{"كِتَاب"}) || got.Analyses[0][0] != "كِتَاب" {
		t.Fatalf("k1 = %+v", got)
	}
	other, _ := cache.Load("fp2")
	if len(other.Readings) != 1 || other.Stale != 2 {
		t.Fatalf("fp2: %+v", other)
	}
}

func TestCacheCompact(t *testing.T) {
	cache := NewCache(filepath.Join(t.TempDir(), "readings.jsonl"))
	_ = cache.Append("fp1", []Reading{{Key: "a"}, {Key: "b"}, {Key: "c"}})
	_ = cache.Append("fp1", []Reading{{Key: "a", CATT: []string{"new"}}})
	_ = cache.Append("old", []Reading{{Key: "a"}})
	dropped, err := cache.Compact("fp1", map[string]bool{"a": true, "b": true})
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 3 {
		t.Fatalf("dropped %d, want 3 (superseded a, unused c, other fingerprint)", dropped)
	}
	loaded, _ := cache.Load("fp1")
	if len(loaded.Readings) != 2 || loaded.Stale != 0 || !slices.Equal(loaded.Readings["a"].CATT, []string{"new"}) {
		t.Fatalf("after compact: %+v", loaded)
	}
	if n, _ := cache.Compact("fp1", map[string]bool{"a": true, "b": true}); n != 0 {
		t.Fatalf("a second compaction dropped %d", n)
	}
}

type fakeEngine struct {
	failKey  string
	breakAt  int
	received []Item
}

func (f *fakeEngine) Check(ctx context.Context, items []Item, ev Events) error {
	f.received = append(f.received, items...)
	for i, it := range items {
		if f.breakAt > 0 && i == f.breakAt {
			return errors.New("sidecar died")
		}
		if it.Key == f.failKey {
			ev.Failure(it.Key, "boom")
		} else {
			ev.Reading(Reading{Key: it.Key, Analyses: make([][]string, len(it.Tokens))})
		}
		if ev.Progress != nil {
			ev.Progress(i+1, len(items))
		}
	}
	return nil
}

func (f *fakeEngine) Close() error { return nil }

func manyItems(n int) []Item {
	items := make([]Item, n)
	for i := range items {
		tokens := []string{"كلمة", string(rune('a' + i%26)), string(rune('A' + i/26))}
		items[i], _ = newItem(FieldLemma, tokens, false)
	}
	return items
}

func TestFillStoresReadingsAndCollectsFailures(t *testing.T) {
	cache := NewCache(filepath.Join(t.TempDir(), "r.jsonl"))
	items := manyItems(40)
	engine := &fakeEngine{failKey: items[3].Key}
	var last [2]int
	res, err := Fill(context.Background(), engine, cache, items, FillOptions{
		Fingerprint: "fp",
		Progress:    func(done, total int) { last = [2]int{done, total} },
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 39 || res.Failures[items[3].Key] != "boom" || last != [2]int{40, 40} {
		t.Fatalf("result = %+v, progress = %v", res, last)
	}
	loaded, _ := cache.Load("fp")
	if len(loaded.Readings) != 39 {
		t.Fatalf("cached %d readings", len(loaded.Readings))
	}
}

func TestFillKeepsWhatArrivedBeforeAnEngineError(t *testing.T) {
	cache := NewCache(filepath.Join(t.TempDir(), "r.jsonl"))
	items := manyItems(30)
	res, err := Fill(context.Background(), &fakeEngine{breakAt: 20}, cache, items, FillOptions{Fingerprint: "fp"})
	if err == nil {
		t.Fatal("the engine error was swallowed")
	}
	loaded, _ := cache.Load("fp")
	if res.Added != 20 || len(loaded.Readings) != 20 {
		t.Fatalf("added %d, cached %d, want 20 each", res.Added, len(loaded.Readings))
	}
}

func TestFillWithNothingToDoNeverStartsTheEngine(t *testing.T) {
	engine := &fakeEngine{}
	res, err := Fill(context.Background(), engine, NewCache(filepath.Join(t.TempDir(), "r.jsonl")), nil, FillOptions{})
	if err != nil || res.Added != 0 || len(engine.received) != 0 {
		t.Fatalf("res=%+v err=%v received=%d", res, err, len(engine.received))
	}
}

func TestItemKeysAreStableBecauseRememberedResultsAreFoundByThem(t *testing.T) {
	for _, c := range []struct {
		tokens  []string
		context bool
		want    string
	}{
		{[]string{"كتاب"}, false, "QWIDFLI4OXNMRYCX"},
		{[]string{"كتاب"}, true, "WDDPPXTMINI5WJZE"},
		{[]string{"قرأ", "الولد"}, true, "IS3C3C65ZUDLPKZR"},
	} {
		if got := ItemKey(c.tokens, c.context); got != c.want {
			t.Errorf("ItemKey(%q, %v) = %s, want %s: changing it forgets every remembered check", c.tokens, c.context, got, c.want)
		}
	}
}

func TestAFieldWithNoWordsIsNotChecked(t *testing.T) {
	n := sampleNote("n_a", "كِتَاب", "")
	n.Forms = []note.Form{{Label: "pl.", Arabic: "-"}}
	if items := Items(n); len(items) != 1 || items[0].Field != FieldLemma {
		t.Errorf("items %+v: only the headword has words", items)
	}
	if items := Items(sampleNote("n_b", "!", "[؟] ؟")); len(items) != 0 {
		t.Errorf("items %+v: punctuation is not checked", items)
	}
}

func TestUniqueKeepsTheFirstOfEachKeyInOrder(t *testing.T) {
	a, b := Item{Key: "a", Field: FieldLemma}, Item{Key: "b", Field: FieldForms}
	got := Unique([]Item{a, b, {Key: "a", Field: FieldExample}, b, a})
	if len(got) != 2 || got[0].Key != "a" || got[0].Field != FieldLemma || got[1].Key != "b" {
		t.Errorf("Unique = %+v", got)
	}
	if got := Unique(nil); len(got) != 0 {
		t.Errorf("Unique(nil) = %+v", got)
	}
}

func TestFillSavesEverySixteenReadingsSoACrashLosesLittle(t *testing.T) {
	cache := NewCache(filepath.Join(t.TempDir(), "r.jsonl"))
	stored := map[int]int{}
	_, err := Fill(context.Background(), &fakeEngine{}, cache, manyItems(40), FillOptions{
		Fingerprint: "fp",
		Progress: func(done, total int) {
			loaded, _ := cache.Load("fp")
			stored[done] = len(loaded.Readings)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for done, want := range map[int]int{1: 0, 15: 0, 16: 16, 17: 16, 31: 16, 32: 32, 33: 32, 40: 32} {
		if stored[done] != want {
			t.Errorf("after %d readings %d are saved, want %d", done, stored[done], want)
		}
	}
	if loaded, _ := cache.Load("fp"); len(loaded.Readings) != 40 {
		t.Errorf("at the end %d are saved", len(loaded.Readings))
	}
}

func TestFillReportsReadingsThatCannotBeSavedEvenWhenTheEngineSucceeds(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := NewCache(filepath.Join(blocker, "readings.jsonl"))
	_, err := Fill(context.Background(), &fakeEngine{}, cache, manyItems(3), FillOptions{Fingerprint: "fp"})
	if err == nil || !strings.HasPrefix(err.Error(), "saving readings: ") {
		t.Fatalf("err = %v", err)
	}
	_, err = Fill(context.Background(), &fakeEngine{breakAt: 2}, cache, manyItems(3), FillOptions{Fingerprint: "fp"})
	if err == nil || !strings.HasPrefix(err.Error(), "saving readings: ") || strings.Contains(err.Error(), "sidecar died") {
		t.Fatalf("a failure to save is reported before the engine's own: %v", err)
	}
}
