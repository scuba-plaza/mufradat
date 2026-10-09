package prompt

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/note"
)

func TestSchemaUsesNotePOSAndIsStable(t *testing.T) {
	var schema struct {
		Properties struct {
			Cards struct {
				Items struct {
					Properties map[string]json.RawMessage `json:"properties"`
					Required   []string                   `json:"required"`
				} `json:"items"`
			} `json:"cards"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(Schema(), &schema); err != nil {
		t.Fatal(err)
	}
	var pos struct{ Enum []string }
	if err := json.Unmarshal(schema.Properties.Cards.Items.Properties["pos"], &pos); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pos.Enum, note.POSStrings()) {
		t.Fatalf("pos enum %v", pos.Enum)
	}
	for _, f := range []string{"ref", "arabic", "english", "example", "example_en", "forms"} {
		if !slices.Contains(schema.Properties.Cards.Items.Required, f) {
			t.Errorf("%s is not required", f)
		}
	}
	first, again := Schema(), Schema()
	if string(first) != string(again) {
		t.Fatal("schema output is not deterministic")
	}
}

func TestSystemIsByteStableAndOrdered(t *testing.T) {
	ex := []note.Note{{Lemma: "كِتَاب", POS: note.Noun, English: "book", Example: "هٰذَا [كِتَابٌ].", ExampleEN: "This is a book.", Comment: "x"}}
	a := System(ex, []string{"فِي", "مِنْ"})
	b := System(ex, []string{"فِي", "مِنْ"})
	if a != b {
		t.Fatal("system prompt differs between calls")
	}
	if !strings.Contains(a, "[square brackets]") || strings.Contains(a, "<b>") {
		t.Fatal("guide must describe bracket markup")
	}
	iGuide, iEx, iVocab := strings.Index(a, "You write flashcards"), strings.Index(a, "Finished cards"), strings.Index(a, "core vocabulary")
	if !(iGuide == 0 && iGuide < iEx && iEx < iVocab) {
		t.Fatalf("sections out of order: %d %d %d", iGuide, iEx, iVocab)
	}
	if strings.Contains(a, `"comment":"x"`) {
		t.Fatal("example cards must not carry reviewer comments")
	}
	if strings.Contains(a, "&lt;") || strings.Contains(a, `\u003c`) {
		t.Fatal("html escaping leaked into the prompt")
	}
}

func TestUserPromptListsEveryDraftWithItsRef(t *testing.T) {
	drafts := []Draft{
		{Ref: 1, Rank: 12, Card: Card{Arabic: "كِتَاب", POS: "noun"}, Entries: []*lexicon.Entry{{Title: "كتاب", Pos: "noun", Canonical: "كِتَاب", Senses: []lexicon.Sense{{Gloss: "book", MSA: true}}}}},
		{Ref: 2, Card: Card{Arabic: "عَيْن", POS: "noun"}, Notice: "Write the card for exactly this entry."},
	}
	got := User(drafts)
	for _, want := range []string{"each of these 2 drafts", "## Ref 1 (frequency rank 12)", "## Ref 2\n", `"ref":1`, `"ref":2`, "Wiktionary entries", "exactly this entry", `"forms":[]`} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt lacks %q:\n%s", want, got)
		}
	}
	if !strings.Contains(User(drafts[:1]), "for this draft") {
		t.Error("single draft wording")
	}
}

func TestParseCards(t *testing.T) {
	raw := json.RawMessage(`{"cards":[{"ref":2,"arabic":"أ","pos":"noun","english":"a","hint":"","forms":[],"example":"x","example_en":"y","comment":""},{"ref":2,"arabic":"dup","pos":"noun","english":"","hint":"","forms":[],"example":"","example_en":"","comment":""}]}`)
	cards, err := ParseCards(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[2].Arabic != "أ" {
		t.Fatalf("cards %+v", cards)
	}
	for _, bad := range []string{``, `null`, `{"cards":"x"}`, `{"cards":[],"extra":1}`} {
		if _, err := ParseCards(json.RawMessage(bad)); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
