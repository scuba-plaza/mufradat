package ranking

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	l, err := Parse(strings.NewReader("rank\tid\tpos\tgloss\n1\tفِي\tprep\tin\n2\tمِنْ\tprep\tfrom\n"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() != 2 || l.Rank("مِنْ") != 2 || l.Rank("x") != 0 {
		t.Errorf("unexpected list: %+v", l)
	}
	if id, ok := l.ID(1); !ok || id != "فِي" {
		t.Errorf("ID(1) = %q %v", id, ok)
	}
	if _, ok := l.ID(3); ok {
		t.Error("rank 3 should not exist")
	}
	if _, ok := l.ID(0); ok {
		t.Error("rank 0 should not exist")
	}
}

func TestParseRejectsGapsAndDuplicates(t *testing.T) {
	for name, in := range map[string]string{
		"gap":       "1\ta\n3\tb\n",
		"duplicate": "1\ta\n2\ta\n",
		"short":     "1\n",
		"not a num": "x\ta\n",
	} {
		if _, err := Parse(strings.NewReader(in)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestEmbeddedIsConsistentWithLexicon(t *testing.T) {
	l, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() < 4000 {
		t.Fatalf("embedded ranking has only %d words", l.Len())
	}
	if id, _ := l.ID(1); id != "فِي" {
		t.Errorf("first ranked word = %q", id)
	}
}
