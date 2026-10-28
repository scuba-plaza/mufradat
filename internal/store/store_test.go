package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/jsonl"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/workspace"
)

func newStore(t *testing.T) (*Store, workspace.Workspace) {
	t.Helper()
	ws, err := workspace.Init(workspace.InitOptions{Root: filepath.Join(t.TempDir(), "ws"), DeckID: 1})
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }
	return New(ws, clock, note.RandomIDs{}), ws
}

func draft(lemma string) note.Note {
	return note.Note{
		Origin:    note.OriginNamed,
		Lemma:     lemma,
		POS:       note.Noun,
		English:   "thing",
		Example:   "الْكِتَابُ [فِي] الْحَقِيبَةِ.",
		ExampleEN: "The book is in the bag.",
	}
}

func TestAddAssignsIdentityAndPersists(t *testing.T) {
	s, ws := newStore(t)
	var added note.Note
	err := s.Update(context.Background(), func(tx *Tx) error {
		var err error
		added, err = tx.Add(draft("كِتَاب"))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !note.ValidID(added.ID) || added.Seq != 1 || added.Created.IsZero() {
		t.Errorf("identity not assigned: %+v", added)
	}
	set, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := set.Get(added.ID)
	if !ok || got.Rev() != added.Rev() {
		t.Errorf("round trip changed the note: %+v vs %+v", got, added)
	}
	if _, err := os.Stat(ws.NotesFile()); err != nil {
		t.Error(err)
	}
}

func TestAddRejectsInvalidNotesAndWritesNothing(t *testing.T) {
	s, ws := newStore(t)
	bad := draft("كِتَاب")
	bad.English = ""
	err := s.Update(context.Background(), func(tx *Tx) error {
		_, err := tx.Add(bad)
		return err
	})
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if _, err := os.Stat(ws.NotesFile()); err == nil {
		t.Error("a failed update must not create the notes file")
	}
}

func TestModifyDetectsConflicts(t *testing.T) {
	s, _ := newStore(t)
	var n note.Note
	if err := s.Update(context.Background(), func(tx *Tx) error {
		var err error
		n, err = tx.Add(draft("كِتَاب"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	stale := n.Rev()
	if err := s.Update(context.Background(), func(tx *Tx) error {
		_, err := tx.Modify(n.ID, stale, func(x *note.Note) error { x.English = "book"; return nil })
		return err
	}); err != nil {
		t.Fatal(err)
	}
	err := s.Update(context.Background(), func(tx *Tx) error {
		_, err := tx.Modify(n.ID, stale, func(x *note.Note) error { x.English = "volume"; return nil })
		return err
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected a conflict, got %v", err)
	}
	set, _ := s.Load()
	if got, _ := set.Get(n.ID); got.English != "book" {
		t.Errorf("the conflicting write leaked: %q", got.English)
	}
	err = s.Update(context.Background(), func(tx *Tx) error {
		_, err := tx.Modify("n_zzzzzzzzzzzz", "", func(*note.Note) error { return nil })
		return err
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestModifyKeepsIdentityAndTouchesUpdated(t *testing.T) {
	s, ws := newStore(t)
	var n note.Note
	_ = s.Update(context.Background(), func(tx *Tx) error {
		n, _ = tx.Add(draft("كِتَاب"))
		return nil
	})
	later := New(ws, func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }, note.RandomIDs{})
	_ = later.Update(context.Background(), func(tx *Tx) error {
		_, err := tx.Modify(n.ID, "", func(x *note.Note) error {
			x.ID, x.Seq, x.English = "n_hijacked0000", 99, "book"
			return nil
		})
		return err
	})
	set, _ := later.Load()
	got, ok := set.Get(n.ID)
	if !ok || got.Seq != n.Seq || got.English != "book" {
		t.Fatalf("identity not preserved: %+v", got)
	}
	if !got.Updated.After(got.Created) {
		t.Errorf("updated not advanced: %v vs %v", got.Updated, got.Created)
	}
}

func TestConcurrentWritersLoseNothing(t *testing.T) {
	_, ws := newStore(t)
	const writers, each = 6, 5
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := New(ws, time.Now, note.RandomIDs{})
			for i := 0; i < each; i++ {
				err := s.Update(context.Background(), func(tx *Tx) error {
					_, err := tx.Add(draft(fmt.Sprintf("كِتَاب%d", w*each+i)))
					return err
				})
				if err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	s := New(ws, time.Now, note.RandomIDs{})
	set, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if set.Len() != writers*each {
		t.Fatalf("notes = %d, want %d", set.Len(), writers*each)
	}
	seqs := map[int]bool{}
	for _, n := range set.All() {
		if seqs[n.Seq] {
			t.Fatalf("duplicate seq %d", n.Seq)
		}
		seqs[n.Seq] = true
	}
}

func TestLoadRejectsDuplicateAndMalformedIDs(t *testing.T) {
	s, ws := newStore(t)
	line := `{"id":"n_aaaaaaaaaaaa","seq":1,"origin":"named","lemma":"x","pos":"noun","english":"e","example":"x","example_en":"x","created":"2026-01-01T00:00:00Z","updated":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(ws.NotesFile(), []byte(line+"\n"+line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil {
		t.Error("duplicate ids must be rejected")
	}
	if err := os.WriteFile(ws.NotesFile(), []byte(`{"id":"كتاب","seq":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil {
		t.Error("malformed ids must be rejected")
	}
	if err := os.WriteFile(ws.NotesFile(), []byte(`{"id":"n_aaaaaaaaaaaa","mystery":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil {
		t.Error("unknown fields must be rejected")
	}
}

func TestRevertRestoresWhatValidationWouldRefuse(t *testing.T) {
	s, ws := newStore(t)
	legacy := draft("كِتَاب")
	legacy.ID = "n_aaaaaaaaaaa2"
	legacy.Seq = 1
	legacy.Example = "قَرَأْتُ [الْكِتَاب] فِي الْبَيْتِ."
	if err := jsonl.WriteFile(ws.NotesFile(), []note.Note{legacy}); err != nil {
		t.Fatal(err)
	}
	var fixed note.Note
	if err := s.Update(context.Background(), func(tx *Tx) error {
		var err error
		fixed, err = tx.Modify(legacy.ID, "", func(x *note.Note) error {
			x.Example = "قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ."
			return nil
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	err := s.Update(context.Background(), func(tx *Tx) error {
		_, err := tx.Modify(legacy.ID, "", func(x *note.Note) error { x.Example = legacy.Example; return nil })
		return err
	})
	if err == nil {
		t.Fatal("Modify must still refuse the invalid example")
	}
	err = s.Update(context.Background(), func(tx *Tx) error {
		_, err := tx.Revert(legacy.ID, "0000000000000000", func(x *note.Note) { x.Example = legacy.Example })
		return err
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Revert must still detect a conflict, got %v", err)
	}
	if err := s.Update(context.Background(), func(tx *Tx) error {
		_, err := tx.Revert(legacy.ID, fixed.Rev(), func(x *note.Note) { x.Example = legacy.Example; x.ID = "n_zzzzzzzzzzz2" })
		return err
	}); err != nil {
		t.Fatal(err)
	}
	set, _ := s.Load()
	got, ok := set.Get(legacy.ID)
	if !ok || got.Example != legacy.Example || got.Seq != 1 {
		t.Errorf("revert gave %+v", got)
	}
	err = s.Update(context.Background(), func(tx *Tx) error {
		_, err := tx.Revert("n_zzzzzzzzzzzz", "", func(*note.Note) {})
		return err
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected not found, got %v", err)
	}
}

type scriptedIDs struct {
	ids   []string
	calls int
}

func (s *scriptedIDs) NewID() (string, error) {
	id := s.ids[s.calls%len(s.ids)]
	s.calls++
	return id, nil
}

func TestAddMintsIdsUntilOneIsFreeAndGivesUpAfterEightCollisions(t *testing.T) {
	_, ws := newStore(t)
	clock := func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }
	add := func(ids *scriptedIDs) (note.Note, error) {
		var added note.Note
		err := New(ws, clock, ids).Update(context.Background(), func(tx *Tx) error {
			var err error
			added, err = tx.Add(draft("كِتَاب"))
			return err
		})
		return added, err
	}

	first := &scriptedIDs{ids: []string{"n_aaaaaaaaaaaa"}}
	if added, err := add(first); err != nil || added.ID != "n_aaaaaaaaaaaa" || first.calls != 1 {
		t.Fatalf("a free id is taken at once: %+v, %v after %d draws", added, err, first.calls)
	}

	second := &scriptedIDs{ids: []string{"n_aaaaaaaaaaaa", "n_aaaaaaaaaaaa", "n_bbbbbbbbbbbb", "n_cccccccccccc"}}
	if added, err := add(second); err != nil || added.ID != "n_bbbbbbbbbbbb" || second.calls != 3 {
		t.Fatalf("a taken id is skipped and drawing stops at the first free one: %+v, %v after %d draws", added, err, second.calls)
	}

	stuck := &scriptedIDs{ids: []string{"n_aaaaaaaaaaaa"}}
	if _, err := add(stuck); err == nil || err.Error() != "could not mint an unused note id" || stuck.calls != 8 {
		t.Fatalf("eight collisions in a row are an error, got %v after %d draws", err, stuck.calls)
	}

	last := &scriptedIDs{ids: append(slices.Repeat([]string{"n_aaaaaaaaaaaa"}, 7), "n_dddddddddddd")}
	if added, err := add(last); err != nil || added.ID != "n_dddddddddddd" {
		t.Fatalf("the eighth draw still counts: %+v, %v", added, err)
	}
}

func TestDeleteRemovesTheNoteForGoodAndReportsAMissingOne(t *testing.T) {
	s, _ := newStore(t)
	var keep, drop note.Note
	if err := s.Update(context.Background(), func(tx *Tx) error {
		var err error
		if keep, err = tx.Add(draft("كِتَاب")); err != nil {
			return err
		}
		drop, err = tx.Add(draft("قَلَم"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(context.Background(), func(tx *Tx) error { return tx.Delete(drop.ID) }); err != nil {
		t.Fatal(err)
	}
	set, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Get(drop.ID); ok {
		t.Error("the deleted note is still on disk")
	}
	if _, ok := set.Get(keep.ID); !ok || set.Len() != 1 {
		t.Errorf("deleting one note must leave the other: %d notes", set.Len())
	}
	err = s.Update(context.Background(), func(tx *Tx) error { return tx.Delete(drop.ID) })
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting a note twice: %v, want ErrNotFound", err)
	}
}
