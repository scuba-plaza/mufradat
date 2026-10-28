package prompt

import (
	"encoding/json"
	"maps"
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

func walkObjects(t *testing.T, path string, node any, visit func(path string, obj map[string]any)) {
	t.Helper()
	obj, ok := node.(map[string]any)
	if !ok {
		return
	}
	if obj["type"] == "object" {
		visit(path, obj)
	}
	if props, ok := obj["properties"].(map[string]any); ok {
		for name, child := range props {
			walkObjects(t, path+"."+name, child, visit)
		}
	}
	if items, ok := obj["items"]; ok {
		walkObjects(t, path+"[]", items, visit)
	}
}

func TestSchemaMeetsMistralsStrictModeAtEveryLevel(t *testing.T) {
	var schema any
	if err := json.Unmarshal(Schema(), &schema); err != nil {
		t.Fatal(err)
	}
	var objects []string
	walkObjects(t, "$", schema, func(path string, obj map[string]any) {
		objects = append(objects, path)
		if extra, ok := obj["additionalProperties"].(bool); !ok || extra {
			t.Errorf("%s must set additionalProperties to false, got %v", path, obj["additionalProperties"])
		}
		props, _ := obj["properties"].(map[string]any)
		var required []string
		for _, r := range obj["required"].([]any) {
			required = append(required, r.(string))
		}
		var names []string
		for name := range props {
			names = append(names, name)
		}
		slices.Sort(required)
		slices.Sort(names)
		if !slices.Equal(required, names) {
			t.Errorf("%s: strict mode needs every property required; properties %v, required %v", path, names, required)
		}
	})
	slices.Sort(objects)
	if want := []string{"$", "$.cards[]", "$.cards[].forms[]"}; !slices.Equal(objects, want) {
		t.Errorf("objects in the schema %v, want %v", objects, want)
	}

	card := schema.(map[string]any)["properties"].(map[string]any)["cards"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	types := map[string]string{}
	for name, p := range card {
		types[name] = p.(map[string]any)["type"].(string)
	}
	want := map[string]string{"ref": "integer", "arabic": "string", "pos": "string", "english": "string", "hint": "string", "forms": "array", "example": "string", "example_en": "string", "comment": "string"}
	if !maps.Equal(types, want) {
		t.Errorf("card field types %v, want %v", types, want)
	}
}

func TestSystemOmitsTheSectionsItHasNothingFor(t *testing.T) {
	ex := []note.Note{{Lemma: "كِتَاب", POS: note.Noun, English: "R&D <b>book</b>", Example: "هٰذَا [كِتَابٌ].", ExampleEN: "This is a book."}}
	for _, c := range []struct {
		name            string
		examples        []note.Note
		vocabulary      []string
		cards, vocabSec bool
	}{
		{"nothing", nil, nil, false, false},
		{"examples only", ex, nil, true, false},
		{"vocabulary only", nil, []string{"فِي"}, false, true},
		{"both", ex, []string{"فِي"}, true, true},
	} {
		got := System(c.examples, c.vocabulary)
		if strings.Contains(got, "Finished cards") != c.cards || strings.Contains(got, "core vocabulary") != c.vocabSec {
			t.Errorf("%s: cards section %v, vocabulary section %v\n%s", c.name, strings.Contains(got, "Finished cards"), strings.Contains(got, "core vocabulary"), got)
		}
	}
	if got := System(ex, nil); !strings.Contains(got, `"english":"R&D <b>book</b>"`) {
		t.Errorf("markup in an example card must reach the model as written:\n%s", got)
	}
}

func TestRewriteShowsAnEmptyFormListNotNullAndAlwaysRefOne(t *testing.T) {
	got := Rewrite(Card{Ref: 9, Arabic: "كِتَاب", POS: "noun", English: "R&D <b>"}, "- fix it", "")
	if !strings.Contains(got, `"forms":[]`) || strings.Contains(got, `"forms":null`) {
		t.Errorf("the strict schema wants an array:\n%s", got)
	}
	if !strings.Contains(got, `"ref":1`) || strings.Contains(got, `"ref":9`) {
		t.Errorf("the card is always ref 1:\n%s", got)
	}
	if !strings.Contains(got, `R&D <b>`) || strings.Contains(got, "\\u003c") {
		t.Errorf("markup must not be escaped:\n%s", got)
	}
	if strings.Contains(got, "previously") || strings.Contains(got, "rejected") {
		t.Errorf("no rejection to report:\n%s", got)
	}
	again := Rewrite(Card{Arabic: "كِتَاب", Forms: []Form{{Label: "pl.", Arabic: "كُتُب"}}}, "- fix it", "the headword must stay x")
	if !strings.Contains(again, `"forms":[{"label":"pl.","arabic":"كُتُب"}]`) || !strings.Contains(again, "A previous answer was rejected: the headword must stay x. Correct it.\n") {
		t.Errorf("forms and the rejection are shown:\n%s", again)
	}
}
