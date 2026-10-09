package note

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

type Origin string

const (
	OriginRanked Origin = "ranked"
	OriginNamed  Origin = "named"
	OriginManual Origin = "manual"
)

type Form struct {
	Label  string `json:"label"`
	Arabic string `json:"ar"`
}

type Note struct {
	ID         string    `json:"id"`
	Seq        int       `json:"seq"`
	Rank       int       `json:"rank,omitempty"`
	Origin     Origin    `json:"origin"`
	Lemma      string    `json:"lemma"`
	POS        POS       `json:"pos"`
	Gender     string    `json:"gender,omitempty"`
	VerbForm   string    `json:"verb_form,omitempty"`
	Root       string    `json:"root,omitempty"`
	Forms      []Form    `json:"forms,omitempty"`
	English    string    `json:"english"`
	Hint       string    `json:"hint,omitempty"`
	Example    string    `json:"example"`
	ExampleEN  string    `json:"example_en"`
	Production *bool     `json:"production,omitempty"`
	Comment    string    `json:"comment,omitempty"`
	Created    time.Time `json:"created"`
	Updated    time.Time `json:"updated"`
}

func (n Note) FormsText() string {
	parts := make([]string, len(n.Forms))
	for i, f := range n.Forms {
		parts[i] = f.Arabic
	}
	return strings.Join(parts, "، ")
}

func (n Note) Rev() string {
	raw, err := json.Marshal(n)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16]
}

func (n Note) LexemeKey() string {
	return tashkeel.LexKey(n.Lemma) + "/" + string(n.POS)
}

func (n Note) WantsProduction(top int) bool {
	if n.Production != nil {
		return *n.Production
	}
	return n.Rank > 0 && n.Rank <= top
}

func StudyBefore(a, b Note) bool {
	switch {
	case a.Rank > 0 && b.Rank > 0:
		if a.Rank != b.Rank {
			return a.Rank < b.Rank
		}
	case a.Rank > 0:
		return true
	case b.Rank > 0:
		return false
	}
	return a.Seq < b.Seq
}

func CompareStudy(a, b Note) int {
	switch {
	case StudyBefore(a, b):
		return -1
	case StudyBefore(b, a):
		return 1
	}
	return strings.Compare(a.ID, b.ID)
}
