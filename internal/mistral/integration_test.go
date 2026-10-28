//go:build integration

package mistral_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/itest"
	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/mistral"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/prompt"
)

const (
	cheapModel = "mistral-small-latest"
	tinySchema = `{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`
)

func isMP3(audio []byte) bool {
	return bytes.HasPrefix(audio, []byte("ID3")) || (len(audio) > 2 && audio[0] == 0xFF && audio[1]&0xE0 == 0xE0)
}

func TestChatCompletionsWithATinySchema(t *testing.T) {
	client := itest.Mistral(t)
	backend := mistral.Backend{Client: client, Model: cheapModel, MaxTokens: 40}
	resp, err := backend.Complete(itest.Context(t), llm.Request{System: "Answer with JSON only.", Prompt: "Reply with the word ok.", Schema: []byte(tinySchema)})
	if err != nil {
		t.Fatal(err)
	}
	var out struct{ Answer string }
	if err := json.Unmarshal(resp.Output, &out); err != nil || out.Answer == "" {
		t.Fatalf("output %s: %v", resp.Output, err)
	}
	u := resp.Usage
	if u.Calls != 1 || u.Input <= 0 || u.Output <= 0 || len(u.Models) != 1 || !strings.HasPrefix(u.Models[0], "mistral-small") {
		t.Fatalf("usage %+v: Mistral should report tokens and the model", u)
	}
}

func TestChatCompletionsAcceptsThePromptSchemaInStrictMode(t *testing.T) {
	client := itest.Mistral(t)
	draft := prompt.Draft{Ref: 1, Card: prompt.CardOf(note.Note{Lemma: "قَلَم", POS: note.Noun}, 1), Rank: 1000}
	backend := mistral.Backend{Client: client, Model: cheapModel, MaxTokens: 1500}
	resp, err := backend.Complete(itest.Context(t), llm.Request{System: "Complete each card.", Prompt: prompt.User([]prompt.Draft{draft}), Schema: prompt.Schema()})
	if err != nil {
		t.Fatal(err)
	}
	cards, err := prompt.ParseCards(resp.Output)
	if err != nil {
		t.Fatalf("%v: %s", err, resp.Output)
	}
	if card, ok := cards[1]; !ok || card.Arabic == "" || card.English == "" {
		t.Fatalf("cards %+v", cards)
	}
}

func TestChatCompletionsRejectsAnUnknownModelAsACallError(t *testing.T) {
	client := itest.Mistral(t)
	backend := mistral.Backend{Client: client, Model: "no-such-model-xyz", MaxTokens: 10}
	_, err := backend.Complete(itest.Context(t), llm.Request{System: "S", Prompt: "P", Schema: []byte(tinySchema)})
	var call *llm.CallError
	if !errors.As(err, &call) {
		t.Fatalf("got %T %v, want *llm.CallError", err, err)
	}
}

func TestBadKeyOnChatIsAnAuthError(t *testing.T) {
	backend := mistral.Backend{Client: &mistral.Client{Key: "not-a-real-key", HTTP: http.DefaultClient}, Model: cheapModel, MaxTokens: 10}
	_, err := backend.Complete(itest.Context(t), llm.Request{System: "S", Prompt: "P", Schema: []byte(tinySchema)})
	if !llm.AsAuth(err) {
		t.Fatalf("got %T %v, want an authentication error", err, err)
	}
}

func TestModelsEndpointAcceptsTheKeyAndRejectsABadOne(t *testing.T) {
	good := itest.Mistral(t)
	if err := good.CheckKey(itest.Context(t)); err != nil {
		t.Fatal(err)
	}
	bad := &mistral.Client{Key: "not-a-real-key", HTTP: http.DefaultClient}
	var api *mistral.APIError
	if err := bad.CheckKey(itest.Context(t)); !errors.As(err, &api) || api.Kind() != mistral.KindAuth {
		t.Fatalf("got %v, want an authentication error", err)
	}
}

func TestVoicesListsTheArabicVoicesAndTheDefaultExists(t *testing.T) {
	client := itest.Mistral(t)
	voices, err := client.Voices(itest.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(voices) == 0 {
		t.Fatal("no Arabic voices")
	}
	found := false
	for _, v := range voices {
		if !slices.ContainsFunc(v.Languages, func(l string) bool { return strings.HasPrefix(strings.ToLower(l), "ar") }) {
			t.Errorf("voice %s (%v) is not Arabic", v.Name, v.Languages)
		}
		if v.ID == "" || v.Name == "" {
			t.Errorf("incomplete voice %+v", v)
		}
		found = found || v.Slug == config.DefaultMistralVoice
	}
	if !found {
		t.Errorf("the default voice %q is not among %d Arabic voices", config.DefaultMistralVoice, len(voices))
	}
}

func TestSpeechSynthesizesOneArabicWordAsMP3(t *testing.T) {
	client := itest.Mistral(t)
	audio, err := client.Synthesize(itest.Context(t), mistral.SpeechRequest{Text: "قَلَم", Voice: config.DefaultMistralVoice})
	if err != nil {
		t.Fatal(err)
	}
	if !isMP3(audio) {
		t.Fatalf("%d bytes that do not start like an MP3: % x", len(audio), audio[:min(8, len(audio))])
	}
}
