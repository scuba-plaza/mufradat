package note

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"regexp"
	"slices"
)

const idPrefix = "n_"

var idEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

type IDSource interface {
	NewID() (string, error)
}

type RandomIDs struct {
	Reader io.Reader
}

func (r RandomIDs) NewID() (string, error) {
	src := r.Reader
	if src == nil {
		src = rand.Reader
	}
	buf := make([]byte, 8)
	if _, err := io.ReadFull(src, buf); err != nil {
		return "", fmt.Errorf("mint note id: %w", err)
	}
	return idPrefix + idEncoding.EncodeToString(buf)[:12], nil
}

var idPattern = regexp.MustCompile(`^n_[a-z2-7]{12}$`)

func ValidID(id string) bool { return idPattern.MatchString(id) }

type Set struct {
	notes []Note
}

func NewSet(notes []Note) *Set {
	s := &Set{notes: slices.Clone(notes)}
	s.sort()
	return s
}

func (s *Set) sort() {
	slices.SortStableFunc(s.notes, func(a, b Note) int { return a.Seq - b.Seq })
}

func (s *Set) Len() int { return len(s.notes) }

func (s *Set) All() []Note { return slices.Clone(s.notes) }

func (s *Set) Study() []Note {
	out := slices.Clone(s.notes)
	slices.SortFunc(out, CompareStudy)
	return out
}

func (s *Set) Get(id string) (Note, bool) {
	for _, n := range s.notes {
		if n.ID == id {
			return n, true
		}
	}
	return Note{}, false
}

func (s *Set) ByRank(rank int) (Note, bool) {
	if rank <= 0 {
		return Note{}, false
	}
	for _, n := range s.notes {
		if n.Rank == rank {
			return n, true
		}
	}
	return Note{}, false
}

func (s *Set) ByLexeme(key string) (Note, bool) {
	for _, n := range s.notes {
		if n.LexemeKey() == key {
			return n, true
		}
	}
	return Note{}, false
}

func (s *Set) Put(n Note) {
	for i := range s.notes {
		if s.notes[i].ID == n.ID {
			s.notes[i] = n
			return
		}
	}
	s.notes = append(s.notes, n)
	s.sort()
}

func (s *Set) Delete(id string) bool {
	before := len(s.notes)
	s.notes = slices.DeleteFunc(s.notes, func(n Note) bool { return n.ID == id })
	return len(s.notes) != before
}

func (s *Set) NextSeq() int {
	next := 1
	for _, n := range s.notes {
		next = max(next, n.Seq+1)
	}
	return next
}

func (s *Set) HasID(id string) bool {
	_, ok := s.Get(id)
	return ok
}
