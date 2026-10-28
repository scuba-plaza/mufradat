package clips

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/note"
)

func TestKeyDependsOnProviderVoiceRateAndText(t *testing.T) {
	a := Voice{Provider: "google", Name: "ar-XA-A", Rate: 0.9}
	base := Key(a, "كِتَاب")
	if base != Key(a, "كِتَاب") {
		t.Fatal("keys must be stable")
	}
	for name, other := range map[string]string{
		"provider": Key(Voice{Provider: "mistral", Name: "ar-XA-A", Rate: 0.9}, "كِتَاب"),
		"voice":    Key(Voice{Provider: "google", Name: "ar-XA-B", Rate: 0.9}, "كِتَاب"),
		"rate":     Key(Voice{Provider: "google", Name: "ar-XA-A", Rate: 1}, "كِتَاب"),
		"text":     Key(a, "كُتُب"),
	} {
		if other == base {
			t.Errorf("changing the %s must change the key", name)
		}
	}
}

func TestKeysAreStableBecauseSavedClipsAreFoundByThem(t *testing.T) {
	cases := []struct {
		voice Voice
		text  string
		want  string
	}{
		{Voice{Provider: "mistral", Name: "voice_479"}, "كِتَاب", "zmyf4b53zsvjn7qi"},
		{Voice{Provider: "google", Name: "ar-XA-Chirp3-HD-Achernar", Rate: 0.9}, "كِتَاب", "cyuluc46y2sc46fx"},
	}
	for _, tc := range cases {
		if got := Key(tc.voice, tc.text); got != tc.want {
			t.Errorf("Key(%+v) = %s, want %s", tc.voice, got, tc.want)
		}
	}
}

func TestKeyIsNotConfusedByWhereOneFieldEndsAndTheNextBegins(t *testing.T) {
	if Key(Voice{Provider: "ab", Name: "c"}, "d") == Key(Voice{Provider: "a", Name: "bc"}, "d") {
		t.Error("provider and voice must be separated")
	}
	if Key(Voice{Provider: "a", Name: "b"}, "cd") == Key(Voice{Provider: "a", Name: "bc"}, "d") {
		t.Error("voice and text must be separated")
	}
}

func TestNamedKeepsTheProviderAndRate(t *testing.T) {
	v := Voice{Provider: "google", Name: "ar-XA-A", Rate: 0.9}.Named("ar-XA-B")
	if v != (Voice{Provider: "google", Name: "ar-XA-B", Rate: 0.9}) {
		t.Errorf("got %+v", v)
	}
}

func TestSwitchingVoiceAndBackFindsTheOriginalClips(t *testing.T) {
	s := NewStore(t.TempDir())
	a, b := Voice{Name: "A", Rate: 0.9}, Voice{Name: "B", Rate: 0.9}
	if err := s.Put(Key(a, "x"), []byte("clip-a")); err != nil {
		t.Fatal(err)
	}
	if s.Has(Key(b, "x")) {
		t.Fatal("voice B must not see voice A's clip")
	}
	if err := s.Put(Key(b, "x"), []byte("clip-b")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(s.Path(Key(a, "x")))
	if string(got) != "clip-a" {
		t.Errorf("switching back to A returned %q", got)
	}
}

func TestWantedListsWordFormsAndPausalExample(t *testing.T) {
	n := note.Note{
		Lemma:   "كِتَاب",
		Forms:   []note.Form{{Label: "pl.", Arabic: "كُتُب"}, {Label: "x", Arabic: "كُتْب"}},
		Example: "أَنَا [مِنْ] هُنَا الْيَوْمَ",
	}
	v := Voice{Name: "A", Rate: 1}
	items := Wanted(n, v)
	if len(items) != 3 {
		t.Fatalf("items = %+v", items)
	}
	if items[0].Field != Word || items[0].Text != "كِتَاب" {
		t.Errorf("word item = %+v", items[0])
	}
	if items[1].Field != Forms || items[1].Text != "كُتُب، كُتْب" {
		t.Errorf("forms item = %+v", items[1])
	}
	if items[2].Field != Example || items[2].Text != "أَنَا مِنْ هُنَا الْيَوْمْ" {
		t.Errorf("example item = %+v", items[2])
	}
	n.Forms = nil
	if got := Wanted(n, v); len(got) != 2 {
		t.Errorf("no forms means no forms clip: %+v", got)
	}
}

func TestPausalEndingSpokenButNotShown(t *testing.T) {
	n := note.Note{Lemma: "أَمْسِ", Example: "ذَهَبَ [الْوَلَدُ] أَمْسِ."}
	items := Wanted(n, Voice{Name: "A", Rate: 1})
	last := items[len(items)-1]
	if last.Field != Example || last.Text != "ذَهَبَ الْوَلَدُ أَمْسْ." {
		t.Errorf("example text = %q", last.Text)
	}
}

func TestStorePutHasRemoveKeys(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	k1, k2 := Key(Voice{Name: "A", Rate: 1}, "a"), Key(Voice{Name: "A", Rate: 1}, "b")
	for _, k := range []string{k1, k2} {
		if err := s.Put(k, []byte("data")); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Put(k1, nil); err == nil {
		t.Error("an empty clip must be refused")
	}
	keys, err := s.Keys()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{k1, k2}
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}
	if err := s.Remove(k1); err != nil || s.Has(k1) {
		t.Errorf("remove failed: %v", err)
	}
	if err := s.Remove(k1); err != nil {
		t.Errorf("removing an absent clip must not fail: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, k2[:2], k2+".mp3")); err != nil {
		t.Errorf("clips live under a two-letter shard: %v", err)
	}
}

func TestKeysOnMissingDirectory(t *testing.T) {
	keys, err := NewStore(filepath.Join(t.TempDir(), "absent")).Keys()
	if err != nil || len(keys) != 0 {
		t.Errorf("keys=%v err=%v", keys, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSwitchingProviderAndBackFindsTheOriginalClips(t *testing.T) {
	s := NewStore(t.TempDir())
	google := Voice{Provider: "google", Name: "same-name", Rate: 0.9}
	mistral := Voice{Provider: "mistral", Name: "same-name"}
	if err := s.Put(Key(google, "x"), []byte("clip-google")); err != nil {
		t.Fatal(err)
	}
	if s.Has(Key(mistral, "x")) {
		t.Fatal("the other provider must not reuse a clip, even for a voice with the same name")
	}
	if err := s.Put(Key(mistral, "x"), []byte("clip-mistral")); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, s.Path(Key(google, "x"))); got != "clip-google" {
		t.Errorf("switching back returned %q", got)
	}
}

func TestKeysListsOnlyClipFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	key := Key(Voice{Provider: "mistral", Name: "v"}, "x")
	if err := store.Put(key, []byte("audio")); err != nil {
		t.Fatal(err)
	}
	for _, stray := range []string{filepath.Join(dir, key[:2], key+".mp3.tmp"), filepath.Join(dir, "notes.txt")} {
		if err := os.WriteFile(stray, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := store.Keys()
	if err != nil || !slices.Equal(keys, []string{key}) {
		t.Fatalf("keys %v, %v", keys, err)
	}
}
