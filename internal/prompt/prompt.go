package prompt

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/note"
)

//go:embed guide.md
var guide string

type Form struct {
	Label  string `json:"label"`
	Arabic string `json:"arabic"`
}

type Card struct {
	Ref       int    `json:"ref"`
	Arabic    string `json:"arabic"`
	POS       string `json:"pos"`
	English   string `json:"english"`
	Hint      string `json:"hint"`
	Forms     []Form `json:"forms"`
	Example   string `json:"example"`
	ExampleEN string `json:"example_en"`
	Comment   string `json:"comment"`
}

type Draft struct {
	Ref     int
	Card    Card
	Rank    int
	Entries []*lexicon.Entry
	Notice  string
}

func CardOf(n note.Note, ref int) Card {
	forms := make([]Form, 0, len(n.Forms))
	for _, f := range n.Forms {
		forms = append(forms, Form{Label: f.Label, Arabic: f.Arabic})
	}
	return Card{
		Ref: ref, Arabic: n.Lemma, POS: string(n.POS), English: n.English, Hint: n.Hint, Forms: forms,
		Example: n.Example, ExampleEN: n.ExampleEN, Comment: n.Comment,
	}
}

func marshal(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	return strings.TrimRight(b.String(), "\n")
}

func System(examples []note.Note, vocabulary []string) string {
	var b strings.Builder
	b.WriteString(guide)
	if len(examples) > 0 {
		b.WriteString("\nFinished cards, in the format you return:\n")
		for i, n := range examples {
			c := CardOf(n, i+1)
			c.Comment = ""
			b.WriteString(marshal(c))
			b.WriteByte('\n')
		}
	}
	if len(vocabulary) > 0 {
		b.WriteString("\nThe learner's core vocabulary, most frequent first. Build example sentences from these words and others as common; a sentence with rarer words is rejected:\n")
		b.WriteString(strings.Join(vocabulary, "\n"))
		b.WriteByte('\n')
	}
	return b.String()
}

func User(drafts []Draft) string {
	var b strings.Builder
	if len(drafts) == 1 {
		b.WriteString("Write the finished card for this draft. Return it in \"cards\" with the ref of the draft.\n")
	} else {
		fmt.Fprintf(&b, "Write the finished card for each of these %d drafts. Return them in \"cards\", each with the ref of its draft.\n", len(drafts))
	}
	for _, d := range drafts {
		fmt.Fprintf(&b, "\n## Ref %d", d.Ref)
		if d.Rank > 0 {
			fmt.Fprintf(&b, " (frequency rank %d)", d.Rank)
		}
		b.WriteString("\nDraft generated from Wiktionary; keep what is right and fix what is wrong:\n")
		c := d.Card
		c.Ref = d.Ref
		if c.Forms == nil {
			c.Forms = []Form{}
		}
		b.WriteString(marshal(c))
		b.WriteByte('\n')
		if len(d.Entries) > 0 {
			b.WriteString("Wiktionary entries for this word, the likeliest first:\n")
			for _, e := range d.Entries {
				b.WriteString(marshal(e))
				b.WriteByte('\n')
			}
		}
		if d.Notice != "" {
			b.WriteString(d.Notice)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func Schema() []byte {
	str := map[string]any{"type": "string"}
	card := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ref":        map[string]any{"type": "integer"},
			"arabic":     str,
			"pos":        map[string]any{"type": "string", "enum": note.POSStrings()},
			"english":    str,
			"hint":       str,
			"forms":      map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"label": str, "arabic": str}, "required": []string{"label", "arabic"}, "additionalProperties": false}},
			"example":    str,
			"example_en": str,
			"comment":    str,
		},
		"required":             []string{"ref", "arabic", "pos", "english", "hint", "forms", "example", "example_en", "comment"},
		"additionalProperties": false,
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"cards": map[string]any{"type": "array", "items": card}},
		"required":             []string{"cards"},
		"additionalProperties": false,
	}
	return []byte(marshal(schema))
}

func ParseCards(raw json.RawMessage) (map[int]Card, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, errors.New("the answer contained no cards")
	}
	var out struct {
		Cards []Card `json:"cards"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("the answer is not a list of cards: %w", err)
	}
	cards := map[int]Card{}
	for _, c := range out.Cards {
		if _, dup := cards[c.Ref]; !dup {
			cards[c.Ref] = c
		}
	}
	return cards, nil
}

func Rewrite(card Card, feedback, rejection string) string {
	var b strings.Builder
	b.WriteString("Review this finished card and return the corrected card in \"cards\" with ref 1.\n\n")
	b.WriteString(feedback)
	b.WriteString("\n\nThe card:\n")
	if card.Forms == nil {
		card.Forms = []Form{}
	}
	card.Ref = 1
	b.WriteString(marshal(card))
	b.WriteByte('\n')
	if rejection != "" {
		b.WriteString("\nA previous answer was rejected: ")
		b.WriteString(rejection)
		b.WriteString(". Correct it.\n")
	}
	return b.String()
}
