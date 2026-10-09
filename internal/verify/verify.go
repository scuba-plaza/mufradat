package verify

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
)

const (
	FieldLemma   = "lemma"
	FieldForms   = "forms"
	FieldExample = "example"
)

const keyLength = 16

var keyEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

type Item struct {
	Key      string   `json:"key"`
	Field    string   `json:"field"`
	Tokens   []string `json:"tokens"`
	Context  bool     `json:"context"`
	Citation bool     `json:"-"`
}

type Reading struct {
	Key      string     `json:"key"`
	Analyses [][]string `json:"analyses"`
	CATT     []string   `json:"catt"`
	BERT     []string   `json:"bert"`
}

type Events struct {
	Reading  func(Reading)
	Failure  func(key, message string)
	Progress func(done, total int)
	Status   func(message string)
}

type Engine interface {
	Check(ctx context.Context, items []Item, ev Events) error
	Close() error
}

func ItemKey(tokens []string, context bool) string {
	h := sha256.New()
	h.Write([]byte("v1\x00"))
	if context {
		h.Write([]byte("context\x00"))
	} else {
		h.Write([]byte("citation\x00"))
	}
	h.Write([]byte(strings.Join(tokens, "\x00")))
	return keyEncoding.EncodeToString(h.Sum(nil))[:keyLength]
}

func newItem(field string, tokens []string, context bool) (Item, bool) {
	if len(tokens) == 0 {
		return Item{}, false
	}
	return Item{Key: ItemKey(tokens, context), Field: field, Tokens: tokens, Context: context, Citation: !context}, true
}

func Items(n note.Note) []Item {
	var items []Item
	add := func(field string, tokens []string, context bool) {
		if it, ok := newItem(field, tokens, context); ok {
			items = append(items, it)
		}
	}
	add(FieldLemma, tashkeel.Words(n.Lemma), false)
	if len(n.Forms) > 0 {
		add(FieldForms, tashkeel.Words(n.FormsText()), false)
	}
	if n.Example != "" {
		add(FieldExample, tashkeel.Words(note.PlainExample(n.Example)), true)
	}
	return items
}

func Unique(items []Item) []Item {
	seen := make(map[string]bool, len(items))
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if seen[it.Key] {
			continue
		}
		seen[it.Key] = true
		out = append(out, it)
	}
	return out
}
