package deck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/apkg"
)

func TestTheBuildStateRoundTripsAndDistinguishesMissingFromBroken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "last-build.json")
	if s, ok, err := LoadState(path); s != (State{}) || ok || err != nil {
		t.Fatalf("a missing state: %+v, %v, %v", s, ok, err)
	}
	built := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	want := State{Fingerprint: "abc123", Path: "/out/deck.apkg", Notes: 42, Built: built}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadState(path)
	if err != nil || !ok || got.Fingerprint != want.Fingerprint || got.Path != want.Path || got.Notes != 42 || !got.Built.Equal(built) {
		t.Fatalf("round trip: %+v, %v, %v", got, ok, err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.HasSuffix(string(raw), "}\n") || !strings.Contains(string(raw), "\n  \"notes\": 42") {
		t.Errorf("the file is indented JSON ending in a newline:\n%q", raw)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("mode %v, %v", info.Mode(), err)
	}

	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, ok, err := LoadState(path)
	if err == nil || ok || s != (State{}) || !strings.HasPrefix(err.Error(), path+": ") {
		t.Errorf("a damaged state: %+v, %v, %v", s, ok, err)
	}

	if s, ok, err := LoadState(dir); err == nil || ok || s != (State{}) {
		t.Errorf("a path that cannot be read is an error, not a missing state: %+v, %v, %v", s, ok, err)
	}
}

func fingerprintFixture() *apkg.Package {
	return &apkg.Package{
		Deck:    apkg.Deck{ID: 10, Name: "Deck", Description: "desc"},
		Model:   apkg.Model{ID: 20, Name: "Model", CSS: ".a{}", Fields: []string{"A"}, Templates: []apkg.Template{{Name: "T", Front: "{{A}}", Back: "back"}}},
		Created: time.Unix(1_700_000_000, 0).UTC(),
		Notes:   []apkg.Note{{GUID: "g1", Fields: []string{"a"}, Tags: []string{"t"}, Cards: []apkg.Card{{Ord: 0, Due: 1}}, Mod: time.Unix(1_700_000_100, 0).UTC()}},
		Media:   []apkg.Media{{Name: "a.mp3"}},
	}
}

func TestTheFingerprintCoversEverythingThatChangesThePackage(t *testing.T) {
	base, err := Fingerprint(fingerprintFixture())
	if err != nil || len(base) != 16 || strings.Trim(base, "0123456789abcdef") != "" {
		t.Fatalf("fingerprint %q, %v: want 16 lowercase hex digits", base, err)
	}
	if again, _ := Fingerprint(fingerprintFixture()); again != base {
		t.Error("the same package must give the same fingerprint")
	}
	for name, change := range map[string]func(*apkg.Package){
		"deck name":        func(p *apkg.Package) { p.Deck.Name = "Other" },
		"deck description": func(p *apkg.Package) { p.Deck.Description = "other" },
		"model name":       func(p *apkg.Package) { p.Model.Name = "Other" },
		"stylesheet":       func(p *apkg.Package) { p.Model.CSS = ".b{}" },
		"deck id":          func(p *apkg.Package) { p.Deck.ID = 11 },
		"model id":         func(p *apkg.Package) { p.Model.ID = 21 },
		"creation time":    func(p *apkg.Package) { p.Created = p.Created.Add(time.Second) },
		"template name":    func(p *apkg.Package) { p.Model.Templates[0].Name = "U" },
		"template front":   func(p *apkg.Package) { p.Model.Templates[0].Front = "{{B}}" },
		"template back":    func(p *apkg.Package) { p.Model.Templates[0].Back = "other" },
		"note guid":        func(p *apkg.Package) { p.Notes[0].GUID = "g2" },
		"note field":       func(p *apkg.Package) { p.Notes[0].Fields[0] = "b" },
		"note tag":         func(p *apkg.Package) { p.Notes[0].Tags[0] = "u" },
		"card order":       func(p *apkg.Package) { p.Notes[0].Cards[0].Ord = 1 },
		"card due":         func(p *apkg.Package) { p.Notes[0].Cards[0].Due = 2 },
		"note time":        func(p *apkg.Package) { p.Notes[0].Mod = p.Notes[0].Mod.Add(time.Second) },
		"another note":     func(p *apkg.Package) { p.Notes = append(p.Notes, apkg.Note{GUID: "g3", Fields: []string{"c"}}) },
		"media name":       func(p *apkg.Package) { p.Media[0].Name = "b.mp3" },
		"another file":     func(p *apkg.Package) { p.Media = append(p.Media, apkg.Media{Name: "c.mp3"}) },
	} {
		p := fingerprintFixture()
		change(p)
		if got, _ := Fingerprint(p); got == base {
			t.Errorf("changing the %s does not change the fingerprint", name)
		}
	}
}

func TestSlugTurnsADeckNameIntoAFileName(t *testing.T) {
	for in, want := range map[string]string{
		"Arabic MSA Core":    "arabic-msa-core",
		"  Hello,  World! ":  "hello-world",
		"Deck 2":             "deck-2",
		"a--b__c":            "a-b-c",
		"UPPER":              "upper",
		"x":                  "x",
		"-lead and trail-":   "lead-and-trail",
		"café":               "caf",
		"عربي":               "",
		"":                   "",
		"123":                "123",
		"a1b2":               "a1b2",
		"one/two\\three.txt": "one-two-three-txt",
	} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
