package note

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func sample() Note {
	return Note{
		ID:        "n_abcdefghijkl",
		Seq:       1,
		Rank:      5,
		Origin:    OriginRanked,
		Lemma:     "كِتَاب",
		POS:       Noun,
		Forms:     []Form{{Label: "pl.", Arabic: "كُتُب"}},
		English:   "book",
		Example:   "الْكِتَابُ [فِي] الْحَقِيبَةِ.",
		ExampleEN: "The book is in the bag.",
		Created:   time.Unix(1000, 0).UTC(),
		Updated:   time.Unix(1000, 0).UTC(),
	}
}

func TestSplitExample(t *testing.T) {
	parts, err := SplitExample("الْكِتَابُ [فِي] الْحَقِيبَةِ.")
	if err != nil {
		t.Fatal(err)
	}
	if parts.Target != "فِي" || parts.Before != "الْكِتَابُ " || parts.After != " الْحَقِيبَةِ." {
		t.Errorf("parts = %+v", parts)
	}
	if got := parts.Plain(); got != "الْكِتَابُ فِي الْحَقِيبَةِ." {
		t.Errorf("plain = %q", got)
	}
	for _, bad := range []string{"no marks", "[a] [b]", "]x[", "[ ]", "[x"} {
		if _, err := SplitExample(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestMarkExample(t *testing.T) {
	got, ok := MarkExample("أَنَا مِنْ هُنَا.", "مِنْ")
	if !ok || got != "أَنَا [مِنْ] هُنَا." {
		t.Errorf("got %q %v", got, ok)
	}
	if _, ok := MarkExample("abc", "x"); ok {
		t.Error("absent target must not match")
	}
}

func TestValidateAcceptsAFinishedNote(t *testing.T) {
	if err := Validate(sample()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	n := sample()
	n.English = " "
	n.POS = "thing"
	n.Example = "الْكِتَاب [فِي] الْحَقِيبَة."
	n.Forms = []Form{{Label: "", Arabic: "كُتُب"}, {Label: "pl.", Arabic: "books"}}
	err := Validate(n)
	if err == nil {
		t.Fatal("expected problems")
	}
	for _, want := range []string{"english is empty", `pos "thing"`, "has no label", "no Arabic text", "without a vowel mark"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%v", want, err)
		}
	}
}

func TestValidateRejectsMarkupAndLatin(t *testing.T) {
	n := sample()
	n.Example = "الْكِتَابُ <b>فِي</b> [الْحَقِيبَةِ]."
	if err := Validate(n); err == nil || !strings.Contains(err.Error(), "markup") {
		t.Errorf("markup not rejected: %v", err)
	}
	n = sample()
	n.Lemma = "كِتَابbook"
	if err := Validate(n); err == nil || !strings.Contains(err.Error(), "Latin") {
		t.Errorf("latin not rejected: %v", err)
	}
}

func TestRevChangesWithContent(t *testing.T) {
	a := sample()
	b := sample()
	if a.Rev() != b.Rev() {
		t.Error("identical notes must share a revision")
	}
	b.English = "books"
	if a.Rev() == b.Rev() {
		t.Error("revision must change with content")
	}
}

func TestStudyOrder(t *testing.T) {
	mk := func(id string, seq, rank int) Note { return Note{ID: id, Seq: seq, Rank: rank} }
	s := NewSet([]Note{mk("n_a", 1, 0), mk("n_b", 2, 9), mk("n_c", 3, 2), mk("n_d", 4, 0)})
	var ids []string
	for _, n := range s.Study() {
		ids = append(ids, n.ID)
	}
	if got := strings.Join(ids, ","); got != "n_c,n_b,n_a,n_d" {
		t.Errorf("study order = %s", got)
	}
}

func TestStudyOrderRanksFirstThenTheOrderTheyWereAdded(t *testing.T) {
	mk := func(id string, seq, rank int) Note { return Note{ID: id, Seq: seq, Rank: rank} }
	for _, c := range []struct {
		name string
		a, b Note
		want int
	}{
		{"lower rank first", mk("a", 9, 2), mk("b", 1, 7), -1},
		{"higher rank later", mk("a", 1, 7), mk("b", 9, 2), 1},
		{"same rank falls back to the order added", mk("a", 1, 4), mk("b", 2, 4), -1},
		{"same rank falls back to the order added, reversed", mk("a", 2, 4), mk("b", 1, 4), 1},
		{"ranked before unranked however late it was added", mk("a", 9, 50), mk("b", 1, 0), -1},
		{"unranked after ranked however early it was added", mk("a", 1, 0), mk("b", 9, 50), 1},
		{"unranked keep the order added", mk("a", 1, 0), mk("b", 2, 0), -1},
		{"unranked keep the order added, reversed", mk("a", 2, 0), mk("b", 1, 0), 1},
		{"identical order falls back to the id", mk("a", 3, 0), mk("b", 3, 0), -1},
		{"identical order falls back to the id, reversed", mk("b", 3, 0), mk("a", 3, 0), 1},
		{"the same note", mk("a", 3, 5), mk("a", 3, 5), 0},
	} {
		if got := CompareStudy(c.a, c.b); got != c.want {
			t.Errorf("%s: CompareStudy = %d, want %d", c.name, got, c.want)
		}
		if StudyBefore(c.a, c.b) && StudyBefore(c.b, c.a) {
			t.Errorf("%s: each note claims to come before the other", c.name)
		}
	}
}

func TestWantsProduction(t *testing.T) {
	yes, no := true, false
	n := Note{Rank: 10}
	if !n.WantsProduction(1000) || n.WantsProduction(5) {
		t.Error("rank cut-off wrong")
	}
	if !n.WantsProduction(10) || n.WantsProduction(9) {
		t.Error("the cut-off rank itself is included, the one after is not")
	}
	if (Note{}).WantsProduction(1000) {
		t.Error("unranked notes get no production card by default")
	}
	if !(Note{Production: &yes}).WantsProduction(0) || (Note{Rank: 1, Production: &no}).WantsProduction(1000) {
		t.Error("explicit override ignored")
	}
}

func TestSetPutGetDelete(t *testing.T) {
	s := NewSet(nil)
	s.Put(Note{ID: "n_a", Seq: 2})
	s.Put(Note{ID: "n_b", Seq: 1})
	s.Put(Note{ID: "n_a", Seq: 2, English: "updated"})
	if s.Len() != 2 {
		t.Fatalf("len = %d", s.Len())
	}
	if got, _ := s.Get("n_a"); got.English != "updated" {
		t.Error("Put should replace by id")
	}
	if all := s.All(); all[0].ID != "n_b" {
		t.Error("All is ordered by seq")
	}
	if s.NextSeq() != 3 {
		t.Errorf("next seq = %d", s.NextSeq())
	}
	if !s.Delete("n_a") || s.Delete("n_a") {
		t.Error("delete semantics")
	}
}

func TestLexemeKeyIgnoresVowelsAndEnding(t *testing.T) {
	a := Note{Lemma: "كِتَاب", POS: Noun}
	b := Note{Lemma: "كِتَابٌ", POS: Noun}
	c := Note{Lemma: "كِتَاب", POS: Verb}
	if a.LexemeKey() != b.LexemeKey() {
		t.Error("case endings must not split a lexeme")
	}
	if a.LexemeKey() == c.LexemeKey() {
		t.Error("different parts of speech are different lexemes")
	}
}

func TestRandomIDs(t *testing.T) {
	ids := RandomIDs{}
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id, err := ids.NewID()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidID(id) {
			t.Fatalf("malformed id %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
	fixed := RandomIDs{Reader: bytes.NewReader(make([]byte, 8))}
	id, _ := fixed.NewID()
	if id != "n_aaaaaaaaaaaa" {
		t.Errorf("deterministic source gave %q", id)
	}
}
