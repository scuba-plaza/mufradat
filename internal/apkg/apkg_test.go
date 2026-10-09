package apkg

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func base() *Package {
	return &Package{
		Deck:    Deck{ID: 10, Name: "D"},
		Model:   Model{ID: 20, Name: "M", Fields: []string{"A", "B"}, Templates: []Template{{Name: "T", Front: "{{A}}", Back: "{{B}}"}}, Required: [][]int{{0}}},
		Created: time.Unix(1_700_000_000, 0).UTC(),
		Notes:   []Note{{GUID: "g1", Fields: []string{"a", "b"}, Cards: []Card{{Ord: 0, Due: 1}}}},
	}
}

func TestWriteRequiresCreatedAndIDs(t *testing.T) {
	p := base()
	p.Created = time.Time{}
	if err := p.Write(io.Discard); err == nil {
		t.Error("missing creation time must fail")
	}
	p = base()
	p.Deck.ID = 0
	if err := p.Write(io.Discard); err == nil {
		t.Error("missing deck id must fail")
	}
}

func TestWriteRejectsFieldCountMismatch(t *testing.T) {
	p := base()
	p.Notes[0].Fields = []string{"only one"}
	err := p.Write(io.Discard)
	if err == nil || !strings.Contains(err.Error(), "fields") {
		t.Errorf("err = %v", err)
	}
}

func TestWriteRejectsDuplicateMedia(t *testing.T) {
	p := base()
	open := func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("x")), nil }
	p.Media = []Media{{Name: "a.mp3", Open: open}, {Name: "a.mp3", Open: open}}
	if err := p.Write(io.Discard); err == nil {
		t.Error("duplicate media names must fail")
	}
}

func TestStableIDsAreStableAndDistinct(t *testing.T) {
	first, again := StableID("note", "a"), StableID("note", "a")
	if first != again {
		t.Error("not stable")
	}
	if StableID("note", "a") == StableID("card", "a") {
		t.Error("kinds must not collide")
	}
	if id := StableID("note", "a"); id < 1_700_000_000_000 {
		t.Errorf("id %d is below the Anki epoch floor", id)
	}
}

func TestStripHTMLAndWriteIsRepeatable(t *testing.T) {
	if got := StripHTML(" <b>x</b> y "); got != "x y" {
		t.Errorf("StripHTML = %q", got)
	}
	var a, b bytes.Buffer
	if err := base().Write(&a); err != nil {
		t.Fatal(err)
	}
	if err := base().Write(&b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("identical packages must produce identical bytes")
	}
}
