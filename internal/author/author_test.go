package author

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/prompt"
	"github.com/scuba-plaza/mufradat/internal/ranking"
	"github.com/scuba-plaza/mufradat/internal/store"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
	"github.com/scuba-plaza/mufradat/internal/vocab"
	"github.com/scuba-plaza/mufradat/internal/workspace"
)

const goodExample = "قَرَأَ [الْوَلَدُ] الْكِتَابَ فِي الْبَيْتِ."

func entry(canonical, pos string, glosses ...string) *lexicon.Entry {
	e := &lexicon.Entry{Title: tashkeel.Skeleton(canonical), Pos: pos, Canonical: canonical}
	for _, g := range glosses {
		e.Senses = append(e.Senses, lexicon.Sense{Gloss: g, MSA: true})
	}
	return e
}

func lemma(entries ...*lexicon.Entry) *lexicon.Lemma {
	return &lexicon.Lemma{ID: entries[0].Canonical, Entries: entries}
}

type world struct {
	ix    *lexicon.Index
	list  *ranking.List
	store *store.Store
	ws    workspace.Workspace
}

func newWorld(t *testing.T) world {
	t.Helper()
	noun := entry("كِتَاب", "noun", "book")
	noun.Plurals = []string{"كُتُب", "كُتْب", "ثَالِث"}
	noun.Gender = "m"
	verb := entry("كَتَبَ", "verb", "to write")
	verb.NonPast = "يَكْتُبُ"
	verb.VerbalNouns = []string{"كِتَابَة"}
	water := entry("مَاء", "noun", "water")
	meow := entry("مَاءَ", "verb", "to meow")
	person := &lexicon.Entry{Title: "أحمد", Pos: "name", Canonical: "أَحْمَد", Senses: []lexicon.Sense{{Gloss: "Ahmad", MSA: true}}}
	dialect := &lexicon.Entry{Title: "شلون", Pos: "adv", Canonical: "شُلُون", Senses: []lexicon.Sense{{Gloss: "how", MSA: false}}}
	ix := lexicon.NewIndex([]*lexicon.Lemma{lemma(noun), lemma(verb), lemma(water, meow), lemma(person), lemma(dialect)})
	list, err := ranking.Parse(strings.NewReader("rank\tid\tpos\tgloss\n1\tكِتَاب\tnoun\tbook\n2\tأَحْمَد\tname\tx\n3\tكَتَبَ\tverb\twrite\n4\tشُلُون\tadv\thow\n5\tمَاء\tnoun\twater\n6\tغَائِب\tnoun\tmissing\n"))
	if err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Init(workspace.InitOptions{Root: filepath.Join(t.TempDir(), "ws"), DeckID: 1})
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }
	return world{ix: ix, list: list, store: store.New(ws, clock, note.RandomIDs{}), ws: ws}
}

func (w world) load(t *testing.T) *note.Set {
	t.Helper()
	set, err := w.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return set
}

var draftLine = regexp.MustCompile(`(?m)^\{"ref":\d+.*$`)

type fakeLLM struct {
	mu      sync.Mutex
	prompts []string
	answer  func(call int, cards []prompt.Card) (json.RawMessage, error)
}

func (f *fakeLLM) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	f.mu.Lock()
	call := len(f.prompts)
	f.prompts = append(f.prompts, req.Prompt)
	f.mu.Unlock()
	var cards []prompt.Card
	for _, line := range draftLine.FindAllString(req.Prompt, -1) {
		var c prompt.Card
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return llm.Response{}, err
		}
		cards = append(cards, c)
	}
	out, err := f.answer(call, cards)
	if err != nil {
		return llm.Response{}, err
	}
	return llm.Response{Output: out, Usage: llm.Usage{Calls: 1, Input: 100, Output: 10}}, nil
}

func finish(cards []prompt.Card, edit func(*prompt.Card)) json.RawMessage {
	out := make([]prompt.Card, len(cards))
	for i, c := range cards {
		c.English = "meaning of " + c.Arabic
		c.Example = goodExample
		c.ExampleEN = "The boy read the book at home."
		if edit != nil {
			edit(&c)
		}
		out[i] = c
	}
	raw, _ := json.Marshal(map[string]any{"cards": out})
	return raw
}

func goodAnswers(call int, cards []prompt.Card) (json.RawMessage, error) {
	return finish(cards, nil), nil
}

func runAuthor(t *testing.T, w world, backend llm.Backend, targets []Target, opts Options) (Result, error) {
	t.Helper()
	a := Author{LLM: backend, Notes: w.store, Schema: prompt.Schema()}
	return a.Run(context.Background(), targets, opts)
}

func TestNextRankedSkipsWhatCannotOrNeedNotBeWritten(t *testing.T) {
	w := newWorld(t)
	got := NextRanked(w.load(t), w.list, w.ix, 10)
	var lemmas []string
	for _, tg := range got {
		lemmas = append(lemmas, fmt.Sprintf("%d:%s", tg.Draft.Rank, tg.Draft.Lemma))
	}
	want := []string{"1:كِتَاب", "3:كَتَبَ", "5:مَاء"}
	if strings.Join(lemmas, " ") != strings.Join(want, " ") {
		t.Fatalf("targets %v, want %v (names, non-MSA and ids without a lexicon entry are skipped)", lemmas, want)
	}
	if got[0].Draft.Origin != note.OriginRanked || got[0].Pinned {
		t.Fatalf("ranked targets are not pinned: %+v", got[0])
	}

	if two := NextRanked(w.load(t), w.list, w.ix, 2); len(two) != 2 {
		t.Fatalf("count not honoured: %d", len(two))
	}
}

func TestNextRankedResumesAfterWrites(t *testing.T) {
	w := newWorld(t)
	res, err := runAuthor(t, w, &fakeLLM{answer: goodAnswers}, NextRanked(w.load(t), w.list, w.ix, 2), Options{System: "s"})
	if err != nil || len(res.Written) != 2 {
		t.Fatalf("res %+v err %v", res, err)
	}
	rest := NextRanked(w.load(t), w.list, w.ix, 10)
	if len(rest) != 1 || rest[0].Draft.Rank != 5 {
		t.Fatalf("resume must pick up where it stopped: %+v", rest)
	}
}

func TestRunWritesNotesAndAggregatesUsage(t *testing.T) {
	w := newWorld(t)
	backend := &fakeLLM{answer: goodAnswers}
	var events []Event
	targets := NextRanked(w.load(t), w.list, w.ix, 3)
	res, err := runAuthor(t, w, backend, targets, Options{Batch: 2, Concurrency: 1, System: "s", Progress: func(e Event) { events = append(events, e) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 3 || len(res.Failed) != 0 || len(res.Unwritten) != 0 {
		t.Fatalf("res %+v", res)
	}
	if len(backend.prompts) != 2 || res.Usage.Calls != 2 || res.Usage.Input != 200 {
		t.Fatalf("calls %d usage %+v", len(backend.prompts), res.Usage)
	}
	if len(events) != 3 || events[2].Done != 3 || events[2].Total != 3 {
		t.Fatalf("events %+v", events)
	}
	set := w.load(t)
	if set.Len() != 3 {
		t.Fatalf("stored %d", set.Len())
	}
	n, ok := set.ByRank(1)
	if !ok || n.Lemma != "كِتَاب" || n.Origin != note.OriginRanked || !note.ValidID(n.ID) || n.English != "meaning of كِتَاب" {
		t.Fatalf("note %+v", n)
	}
	if len(n.Forms) != 2 || n.Forms[0].Arabic != "كُتُب" || n.Gender != "m" {
		t.Fatalf("wiktionary draft was lost: %+v", n)
	}
}

func TestRejectedCardIsCorrectedOnRetry(t *testing.T) {
	w := newWorld(t)
	backend := &fakeLLM{answer: func(call int, cards []prompt.Card) (json.RawMessage, error) {
		return finish(cards, func(c *prompt.Card) {
			if call == 0 && c.Arabic == "كَتَبَ" {
				c.Example = "كَتَبَ الْوَلَدُ الرِّسَالَةَ"
			}
		}), nil
	}}
	res, err := runAuthor(t, w, backend, NextRanked(w.load(t), w.list, w.ix, 2), Options{Batch: 5, System: "s"})
	if err != nil || len(res.Written) != 2 || len(res.Failed) != 0 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if len(backend.prompts) != 2 {
		t.Fatalf("calls %d", len(backend.prompts))
	}
	second := backend.prompts[1]
	if !strings.Contains(second, "previous answer for this card was rejected") || !strings.Contains(second, "square brackets") {
		t.Fatalf("retry prompt does not say why:\n%s", second)
	}
	if strings.Count(second, "## Ref") != 1 {
		t.Fatalf("only the rejected card is sent again:\n%s", second)
	}
}

func TestCardThatNeverPassesIsReportedAndOthersSurvive(t *testing.T) {
	w := newWorld(t)
	backend := &fakeLLM{answer: func(call int, cards []prompt.Card) (json.RawMessage, error) {
		return finish(cards, func(c *prompt.Card) {
			if c.Arabic == "كَتَبَ" {
				c.English = ""
			}
		}), nil
	}}
	res, err := runAuthor(t, w, backend, NextRanked(w.load(t), w.list, w.ix, 3), Options{System: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 2 || len(res.Failed) != 1 || !strings.Contains(res.Failed[0].Err.Error(), "english is empty") {
		t.Fatalf("res %+v", res)
	}
	if res.Failed[0].Target.Draft.Lemma != "كَتَبَ" {
		t.Fatalf("failure names %+v", res.Failed[0].Target.Draft)
	}
	if w.load(t).Len() != 2 {
		t.Fatal("a failed card must not be stored")
	}
}

func TestAuthFailureStopsTheRunAndKeepsWhatWasSaved(t *testing.T) {
	w := newWorld(t)
	backend := &fakeLLM{answer: func(call int, cards []prompt.Card) (json.RawMessage, error) {
		if call >= 1 {
			return nil, &llm.AuthError{Message: "Unauthorized"}
		}
		return finish(cards, nil), nil
	}}
	res, err := runAuthor(t, w, backend, NextRanked(w.load(t), w.list, w.ix, 3), Options{Batch: 1, Concurrency: 1, System: "s"})
	if !llm.AsAuth(err) {
		t.Fatalf("err %v", err)
	}
	if len(res.Written) != 1 || len(res.Unwritten) != 2 || len(res.Failed) != 0 {
		t.Fatalf("res written %d unwritten %d failed %d", len(res.Written), len(res.Unwritten), len(res.Failed))
	}
	if w.load(t).Len() != 1 {
		t.Fatal("the finished note must be saved")
	}
	rest := NextRanked(w.load(t), w.list, w.ix, 5)
	if len(rest) != 2 {
		t.Fatalf("rerunning must continue with the unwritten words: %d", len(rest))
	}
}

func TestRateLimitThatOutlastsTheRetriesStopsTheRunAndKeepsWhatWasSaved(t *testing.T) {
	w := newWorld(t)
	backend := &fakeLLM{answer: func(call int, cards []prompt.Card) (json.RawMessage, error) {
		if call >= 1 {
			return nil, &llm.RateLimitedError{RetryAfter: time.Hour, Message: "Rate limit exceeded"}
		}
		return finish(cards, nil), nil
	}}
	res, err := runAuthor(t, w, backend, NextRanked(w.load(t), w.list, w.ix, 3), Options{Batch: 1, Concurrency: 1, System: "s"})
	var limited *llm.RateLimitedError
	if !errors.As(err, &limited) || limited.RetryAfter != time.Hour {
		t.Fatalf("err %v", err)
	}
	if len(res.Written) != 1 || len(res.Unwritten) != 2 || len(res.Failed) != 0 {
		t.Fatalf("res written %d unwritten %d failed %d", len(res.Written), len(res.Unwritten), len(res.Failed))
	}
	if w.load(t).Len() != 1 {
		t.Fatal("the finished note must be saved")
	}
}

func TestOtherCallFailuresDoNotCancelOtherBatches(t *testing.T) {
	w := newWorld(t)
	backend := &fakeLLM{answer: func(call int, cards []prompt.Card) (json.RawMessage, error) {
		if cards[0].Arabic == "كَتَبَ" {
			return nil, &llm.CallError{Message: "boom"}
		}
		return finish(cards, nil), nil
	}}
	res, err := runAuthor(t, w, backend, NextRanked(w.load(t), w.list, w.ix, 3), Options{Batch: 1, Concurrency: 1, System: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 2 || len(res.Failed) != 1 || !strings.Contains(res.Failed[0].Err.Error(), "boom") {
		t.Fatalf("res %+v", res)
	}
	calls := 0
	for _, p := range backend.prompts {
		if strings.Contains(p, "كَتَبَ") {
			calls++
		}
	}
	if calls != 1 {
		t.Fatalf("a hard call error must not be retried: %d calls", calls)
	}
}

func TestBadOutputIsRetried(t *testing.T) {
	w := newWorld(t)
	backend := &fakeLLM{answer: func(call int, cards []prompt.Card) (json.RawMessage, error) {
		if call == 0 {
			return nil, &llm.BadOutputError{Message: "plain text"}
		}
		return finish(cards, nil), nil
	}}
	res, err := runAuthor(t, w, backend, NextRanked(w.load(t), w.list, w.ix, 1), Options{System: "s"})
	if err != nil || len(res.Written) != 1 || len(backend.prompts) != 2 {
		t.Fatalf("res %+v err %v calls %d", res, err, len(backend.prompts))
	}
}

func TestSavingSkipsWhatAnotherWriterAddedMeanwhile(t *testing.T) {
	w := newWorld(t)
	targets := NextRanked(w.load(t), w.list, w.ix, 2)
	backend := &fakeLLM{answer: func(call int, cards []prompt.Card) (json.RawMessage, error) {
		err := w.store.Update(context.Background(), func(tx *store.Tx) error {
			_, err := tx.Add(note.Note{Rank: 1, Origin: note.OriginRanked, Lemma: "كِتَاب", POS: note.Noun, English: "book", Example: goodExample, ExampleEN: "x"})
			return err
		})
		if err != nil {
			return nil, err
		}
		return finish(cards, nil), nil
	}}
	res, err := runAuthor(t, w, backend, targets, Options{System: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 1 || len(res.Written) != 1 || w.load(t).Len() != 2 {
		t.Fatalf("res %+v stored %d", res, w.load(t).Len())
	}
}

func TestPinnedTargetRejectsAnotherEntry(t *testing.T) {
	w := newWorld(t)
	cands := w.ix.Find("ماء")
	if len(cands) != 2 {
		t.Fatalf("fixture: %d candidates", len(cands))
	}
	set := w.load(t)
	pl, ok := Place(cands[0], "ماء", set, w.list)
	if !ok || pl.Existing != nil || !pl.Target.Pinned || pl.Target.Draft.Origin != note.OriginNamed || pl.Target.Typed != "ماء" {
		t.Fatalf("placement %+v", pl)
	}
	backend := &fakeLLM{answer: func(call int, cards []prompt.Card) (json.RawMessage, error) {
		return finish(cards, func(c *prompt.Card) {
			if call == 0 {
				c.POS = "verb"
				c.Arabic = "مَاءَ"
			}
		}), nil
	}}
	res, err := runAuthor(t, w, backend, []Target{pl.Target}, Options{System: "s"})
	if err != nil || len(res.Written) != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if !strings.Contains(backend.prompts[1], "asked for the noun") {
		t.Fatalf("retry does not explain:\n%s", backend.prompts[1])
	}
	if res.Written[0].POS != note.Noun || res.Written[0].Rank != 5 {
		t.Fatalf("written %+v", res.Written[0])
	}
}

func TestExampleLengthGate(t *testing.T) {
	tg := Target{Draft: note.Note{Lemma: "كِتَاب", POS: note.Noun}}
	example := func(words int) note.Note {
		return note.Note{Lemma: "كِتَاب", POS: note.Noun, Example: "[كِتَابٌ] " + strings.TrimSpace(strings.Repeat("جَدِيدٌ ", words-1)) + "."}
	}
	for words, ok := range map[int]bool{1: false, 2: false, 3: true, 4: true, 11: true, 12: true, 13: false, 20: false} {
		err := Gate(tg, example(words))
		if ok && err != nil {
			t.Errorf("%d words rejected: %v", words, err)
		}
		if !ok && err == nil {
			t.Errorf("%d words accepted", words)
		}
	}
	if err := Gate(tg, example(2)); err == nil || err.Error() != "the example has 2 words; write 3 to 9" {
		t.Errorf("the retry prompt must tell the model the target length: %v", err)
	}
	if err := Gate(tg, note.Note{Lemma: "كِتَاب", POS: note.Noun, Example: goodExample}); err != nil {
		t.Fatal(err)
	}
}

func TestPlaceDetectsExistingAndClaimsRankOnlyForPrimaryEntry(t *testing.T) {
	w := newWorld(t)
	set := w.load(t)

	book := w.ix.Find("كتاب")
	pl, ok := Place(book[0], "كتاب", set, w.list)
	if !ok || pl.Target.Draft.Rank != 1 {
		t.Fatalf("the main entry of a ranked lemma keeps its rank: %+v", pl.Target.Draft)
	}

	meow := w.ix.Find("ماء")[1]
	if meow.Primary {
		t.Fatal("fixture: second entry must not be primary")
	}
	pl, ok = Place(meow, "ماء", set, w.list)
	if !ok || pl.Target.Draft.Rank != 0 || pl.Target.Draft.POS != note.Verb {
		t.Fatalf("a secondary entry must not claim the rank: %+v", pl.Target.Draft)
	}

	if err := w.store.Update(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Add(note.Note{Rank: 1, Origin: note.OriginRanked, Lemma: "كِتَاب", POS: note.Noun, English: "book", Example: goodExample, ExampleEN: "x"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	pl, ok = Place(book[0], "كتاب", w.load(t), w.list)
	if !ok || pl.Existing == nil || pl.Existing.Lemma != "كِتَاب" {
		t.Fatalf("existing note not detected: %+v", pl)
	}
}

func TestDraftNoteFormsByPartOfSpeech(t *testing.T) {
	noun := entry("كِتَاب", "noun")
	noun.Plurals = []string{"كُتُب", "كُتْب", "ثالث"}
	n, ok := DraftNote(noun, 3, note.OriginRanked)
	if !ok || len(n.Forms) != 2 || n.Forms[0].Label != "pl." || n.Rank != 3 {
		t.Fatalf("noun %+v", n)
	}
	verb := entry("كَتَبَ", "verb")
	verb.NonPast, verb.VerbalNouns = "يَكْتُبُ", []string{"كِتَابَة", "كَتْب"}
	n, _ = DraftNote(verb, 0, note.OriginNamed)
	if len(n.Forms) != 2 || n.Forms[0].Label != "pres." || n.Forms[1].Label != "masdar" || n.Forms[1].Arabic != "كِتَابَة" {
		t.Fatalf("verb %+v", n.Forms)
	}
	adj := entry("كَبِير", "adj")
	adj.Feminine, adj.Plurals = "كَبِيرَة", []string{"كِبَار"}
	n, _ = DraftNote(adj, 0, note.OriginNamed)
	if len(n.Forms) != 2 || n.Forms[0].Label != "f." || n.Forms[1].Label != "pl." {
		t.Fatalf("adj %+v", n.Forms)
	}
	if _, ok := DraftNote(entry("x", "symbol"), 0, note.OriginNamed); ok {
		t.Fatal("an unknown part of speech must not produce a draft")
	}
}

func TestPickExamplesPrefersDistinctPartsOfSpeech(t *testing.T) {
	notes := []note.Note{
		{ID: "n_aaaaaaaaaaaa", Rank: 1, POS: note.Noun},
		{ID: "n_bbbbbbbbbbbb", Rank: 2, POS: note.Noun},
		{ID: "n_cccccccccccc", Rank: 3, POS: note.Verb},
		{ID: "n_dddddddddddd", Rank: 4, POS: note.Adj},
	}
	got := PickExamples(notes, 3)
	var ids []string
	for _, n := range got {
		ids = append(ids, n.ID)
	}
	if strings.Join(ids, ",") != "n_aaaaaaaaaaaa,n_cccccccccccc,n_dddddddddddd" {
		t.Fatalf("picked %v", ids)
	}
	if got := PickExamples(notes, 4); len(got) != 4 {
		t.Fatalf("fill up with repeats: %d", len(got))
	}
}

type chooserFunc func(string, []lexicon.Candidate) (int, error)

func (f chooserFunc) Choose(w string, c []lexicon.Candidate) (int, error) { return f(w, c) }

func TestSelector(t *testing.T) {
	w := newWorld(t)
	cands := w.ix.Find("ماء")
	one := w.ix.Find("كتاب")

	var nf *NotFoundError
	if _, err := (Selector{}).Select("x", nil); !errors.As(err, &nf) {
		t.Fatalf("no candidates: %v", err)
	}
	if c, err := (Selector{}).Select("كتاب", one); err != nil || c.Main().Pos != "noun" {
		t.Fatalf("single candidate: %v", err)
	}
	var amb *AmbiguousError
	if _, err := (Selector{}).Select("ماء", cands); !errors.As(err, &amb) || len(amb.Candidates) != 2 {
		t.Fatalf("ambiguous without a way to ask: %v", err)
	}
	if c, _ := (Selector{Yes: true}).Select("ماء", cands); c.Main().Pos != "noun" {
		t.Fatal("--yes must take the first candidate")
	}
	if c, _ := (Selector{Pick: 2}).Select("ماء", cands); c.Main().Pos != "verb" {
		t.Fatal("--pick 2")
	}
	if _, err := (Selector{Pick: 3}).Select("ماء", cands); err == nil {
		t.Fatal("pick out of range accepted")
	}
	if c, _ := (Selector{POS: note.Verb}).Select("ماء", cands); c.Main().Pos != "verb" {
		t.Fatal("--pos verb")
	}
	if _, err := (Selector{POS: note.Adj}).Select("ماء", cands); !errors.As(err, &nf) {
		t.Fatalf("pos with no match: %v", err)
	}
	if c, _ := (Selector{Entries: map[string]int{"ماء": 2}}).Select("ماء", cands); c.Main().Pos != "verb" {
		t.Fatal("--entry ماء#2")
	}
	asked := false
	chooser := chooserFunc(func(word string, c []lexicon.Candidate) (int, error) { asked = true; return 2, nil })
	if c, _ := (Selector{Chooser: chooser}).Select("ماء", cands); !asked || c.Main().Pos != "verb" {
		t.Fatal("chooser not used")
	}
	if _, err := (Selector{Chooser: chooser}).Select("كتاب", one); err != nil || len(one) != 1 {
		t.Fatal(err)
	}
}

func TestParseEntrySelector(t *testing.T) {
	word, n, err := ParseEntrySelector("كتاب#2")
	if err != nil || word != "كتاب" || n != 2 {
		t.Fatalf("%q %d %v", word, n, err)
	}
	for _, bad := range []string{"كتاب", "#2", "كتاب#", "كتاب#x", "كتاب#0"} {
		if _, _, err := ParseEntrySelector(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestDedupe(t *testing.T) {
	a := Target{Draft: note.Note{Lemma: "كِتَاب", POS: note.Noun}}
	b := Target{Draft: note.Note{Lemma: "كِتَابٌ", POS: note.Noun}}
	c := Target{Draft: note.Note{Lemma: "كِتَاب", POS: note.Verb}}
	if got := Dedupe([]Target{a, b, c}); len(got) != 2 {
		t.Fatalf("%d", len(got))
	}
}

type fakeVocabulary struct {
	mu       sync.Mutex
	ceilings []int
	flag     func(example string) []vocab.Offender
}

func (f *fakeVocabulary) Beyond(example string, ceiling int) []vocab.Offender {
	f.mu.Lock()
	f.ceilings = append(f.ceilings, ceiling)
	f.mu.Unlock()
	return f.flag(example)
}

func TestVocabularyGateSendsTheOffendingWordsBack(t *testing.T) {
	w := newWorld(t)
	rare := "قَرَأَ [الْوَلَدُ] الْمِجْهَرَ فِي الْبَيْتِ."
	backend := &fakeLLM{answer: func(call int, cards []prompt.Card) (json.RawMessage, error) {
		return finish(cards, func(c *prompt.Card) {
			if call == 0 {
				c.Example = rare
			}
		}), nil
	}}
	vocabulary := &fakeVocabulary{flag: func(example string) []vocab.Offender {
		if example == rare {
			return []vocab.Offender{{Word: "الْمِجْهَرَ", Lemma: "مِجْهَر", Rank: 0}}
		}
		return nil
	}}
	a := Author{LLM: backend, Notes: w.store, Schema: prompt.Schema(), Vocabulary: vocabulary, AllowRank: 1500}
	res, err := a.Run(context.Background(), NextRanked(w.load(t), w.list, w.ix, 1), Options{Batch: 5, System: "s"})
	if err != nil || len(res.Written) != 1 || len(res.Failed) != 0 {
		t.Fatalf("res %+v err %v", res, err)
	}
	second := backend.prompts[1]
	if !strings.Contains(second, "does not know yet") || !strings.Contains(second, "الْمِجْهَرَ (from مِجْهَر, not among the 5000 most frequent words)") {
		t.Fatalf("the retry should name the rare word:\n%s", second)
	}
	if res.Written[0].Example != goodExample {
		t.Errorf("saved %q", res.Written[0].Example)
	}
}

func TestVocabularyCeilingFollowsTheCardsOwnRank(t *testing.T) {
	w := newWorld(t)
	vocabulary := &fakeVocabulary{flag: func(string) []vocab.Offender { return nil }}
	a := Author{LLM: &fakeLLM{answer: goodAnswers}, Notes: w.store, Schema: prompt.Schema(), Vocabulary: vocabulary, AllowRank: 4}
	targets := NextRanked(w.load(t), w.list, w.ix, 3)
	if _, err := a.Run(context.Background(), targets, Options{Batch: 5, System: "s"}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(vocabulary.ceilings)
	if want := []int{4, 4, 5}; !slices.Equal(vocabulary.ceilings, want) {
		t.Errorf("ceilings = %v, want %v: the learner knows every word up to the card's own rank", vocabulary.ceilings, want)
	}
}

func TestVocabularyGateIsOffWithoutAllowRank(t *testing.T) {
	w := newWorld(t)
	vocabulary := &fakeVocabulary{flag: func(string) []vocab.Offender { return []vocab.Offender{{Word: "x"}} }}
	a := Author{LLM: &fakeLLM{answer: goodAnswers}, Notes: w.store, Schema: prompt.Schema(), Vocabulary: vocabulary}
	res, err := a.Run(context.Background(), NextRanked(w.load(t), w.list, w.ix, 1), Options{Batch: 5, System: "s"})
	if err != nil || len(res.Written) != 1 || len(vocabulary.ceilings) != 0 {
		t.Fatalf("res %+v err %v ceilings %v", res, err, vocabulary.ceilings)
	}
}
