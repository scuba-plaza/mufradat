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

func TestParseRejectsGapsAndDuplicatesAndSaysWhere(t *testing.T) {
	for name, c := range map[string]struct{ in, want string }{
		"gap":         {"1\ta\n3\tb\n", `ranking line 2: rank "3" does not follow 1`},
		"duplicate":   {"1\ta\n2\ta\n", "ranking line 2: a is listed twice"},
		"short":       {"1\n", "ranking line 1: expected rank and id"},
		"not a num":   {"x\ta\n", `ranking line 1: rank "x" does not follow 0`},
		"late header": {"1\ta\nrank\tid\n", `ranking line 2: rank "rank" does not follow 1`},
	} {
		_, err := Parse(strings.NewReader(c.in))
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: error %v, want %q", name, err, c.want)
		}
	}
}

func TestParseAcceptsRankAndIdAloneAndBlankLines(t *testing.T) {
	l, err := Parse(strings.NewReader("\n1\ta\n  \n2\tb\n"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Len() != 2 || l.Rank("a") != 1 || l.Rank("b") != 2 {
		t.Errorf("list: %+v", l)
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
