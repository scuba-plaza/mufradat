package apkg

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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

func openCollection(t *testing.T, data []byte) *sql.DB {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != "collection.anki2" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "c.anki2")
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		return db
	}
	t.Fatal("the package has no collection.anki2")
	return nil
}

func TestOnlyTheFieldsMarkedRightToLeftAreRightToLeft(t *testing.T) {
	p := base()
	p.Model.Fields = []string{"Arabic", "English", "Hint"}
	p.Model.RTL = []string{"Arabic", "Hint"}
	p.Notes[0].Fields = []string{"a", "b", "c"}
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		t.Fatal(err)
	}
	var models string
	if err := openCollection(t, buf.Bytes()).QueryRow(`SELECT models FROM col`).Scan(&models); err != nil {
		t.Fatal(err)
	}
	var all map[string]struct {
		Flds []struct {
			Name   string `json:"name"`
			RTL    bool   `json:"rtl"`
			Sticky bool   `json:"sticky"`
		} `json:"flds"`
	}
	if err := json.Unmarshal([]byte(models), &all); err != nil {
		t.Fatal(err)
	}
	flds := all["20"].Flds
	if len(flds) != 3 {
		t.Fatalf("fields %+v", flds)
	}
	for i, want := range []bool{true, false, true} {
		if flds[i].RTL != want {
			t.Errorf("field %s: rtl = %v, want %v", flds[i].Name, flds[i].RTL, want)
		}
		if flds[i].Sticky {
			t.Errorf("field %s must not keep its value between notes", flds[i].Name)
		}
	}
}

func TestWriteFileCreatesTheFolderAndReplacesAnOlderPackageWithoutLeavingTempFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out", "deep", "deck.apkg")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("older"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base().WriteFile(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := base().Write(&want); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want.Bytes()) {
		t.Errorf("the file holds %d bytes that are not the package", len(data))
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("mode %v, %v: the package must be readable by whoever imports it", info.Mode(), err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Errorf("the folder holds %v, %v: only the package belongs there", entries, err)
	}

	fresh := filepath.Join(t.TempDir(), "made", "by", "write", "file.apkg")
	if err := base().WriteFile(fresh); err != nil {
		t.Fatalf("WriteFile must create missing folders: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error(err)
	}
}

func TestWriteFileFailureLeavesTheOlderPackageAndNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.apkg")
	if err := os.WriteFile(path, []byte("older"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := base()
	p.Created = time.Time{}
	if err := p.WriteFile(path); err == nil || !strings.Contains(err.Error(), "creation time") {
		t.Fatalf("err = %v", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "older" {
		t.Errorf("a failed write damaged the existing package: %q", data)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("a failed write left %v behind", entries)
	}
}
