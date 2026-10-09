package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/scuba-plaza/mufradat/internal/jsonl"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/workspace"
)

var (
	ErrNotFound = errors.New("note not found")
	ErrConflict = errors.New("the note changed since it was read")
)

type Store struct {
	ws  workspace.Workspace
	now func() time.Time
	ids note.IDSource
}

func New(ws workspace.Workspace, now func() time.Time, ids note.IDSource) *Store {
	return &Store{ws: ws, now: now, ids: ids}
}

func (s *Store) Load() (*note.Set, error) {
	notes, err := jsonl.ReadFile[note.Note](s.ws.NotesFile())
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, n := range notes {
		if !note.ValidID(n.ID) {
			return nil, fmt.Errorf("%s: note %q has a malformed id", s.ws.NotesFile(), n.ID)
		}
		if seen[n.ID] {
			return nil, fmt.Errorf("%s: id %s appears twice", s.ws.NotesFile(), n.ID)
		}
		seen[n.ID] = true
	}
	return note.NewSet(notes), nil
}

type Tx struct {
	set *note.Set
	now time.Time
	ids note.IDSource
}

func (t *Tx) Notes() *note.Set { return t.set }

func (t *Tx) Add(n note.Note) (note.Note, error) {
	for attempt := 0; n.ID == "" && attempt < 8; attempt++ {
		id, err := t.ids.NewID()
		if err != nil {
			return note.Note{}, err
		}
		if !t.set.HasID(id) {
			n.ID = id
		}
	}
	if n.ID == "" {
		return note.Note{}, errors.New("could not mint an unused note id")
	}
	if t.set.HasID(n.ID) {
		return note.Note{}, fmt.Errorf("note %s already exists", n.ID)
	}
	n.Seq = t.set.NextSeq()
	n.Created = t.now
	n.Updated = t.now
	if err := note.Validate(n); err != nil {
		return note.Note{}, err
	}
	t.set.Put(n)
	return n, nil
}

func (t *Tx) Modify(id, ifRev string, fn func(*note.Note) error) (note.Note, error) {
	cur, ok := t.set.Get(id)
	if !ok {
		return note.Note{}, ErrNotFound
	}
	if ifRev != "" && cur.Rev() != ifRev {
		return note.Note{}, ErrConflict
	}
	next := cur
	if err := fn(&next); err != nil {
		return note.Note{}, err
	}
	next.ID, next.Seq, next.Created = cur.ID, cur.Seq, cur.Created
	if next.Rev() == cur.Rev() {
		return cur, nil
	}
	next.Updated = t.now
	if err := note.Validate(next); err != nil {
		return note.Note{}, err
	}
	t.set.Put(next)
	return next, nil
}

func (t *Tx) Delete(id string) error {
	if !t.set.Delete(id) {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Update(ctx context.Context, fn func(*Tx) error) error {
	unlock, err := s.ws.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	set, err := s.Load()
	if err != nil {
		return err
	}
	tx := &Tx{set: set, now: s.now().UTC().Truncate(time.Second), ids: s.ids}
	if err := fn(tx); err != nil {
		return err
	}
	return jsonl.WriteFile(s.ws.NotesFile(), set.All())
}

func (t *Tx) Revert(id, ifRev string, fn func(*note.Note)) (note.Note, error) {
	cur, ok := t.set.Get(id)
	if !ok {
		return note.Note{}, ErrNotFound
	}
	if ifRev != "" && cur.Rev() != ifRev {
		return note.Note{}, ErrConflict
	}
	next := cur
	fn(&next)
	next.ID, next.Seq, next.Created = cur.ID, cur.Seq, cur.Created
	if next.Rev() == cur.Rev() {
		return cur, nil
	}
	next.Updated = t.now
	t.set.Put(next)
	return next, nil
}
