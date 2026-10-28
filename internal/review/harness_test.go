package review

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/decisions"
	"github.com/scuba-plaza/mufradat/internal/jsonl"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/store"
	"github.com/scuba-plaza/mufradat/internal/verify"
	"github.com/scuba-plaza/mufradat/internal/workspace"
)

const (
	idTired = "n_aaaaaaaaaaa2"
	idBook  = "n_aaaaaaaaaaa3"
)

var clock = func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }

func tiredNote() note.Note {
	return note.Note{
		ID: idTired, Seq: 1, Rank: 10, Origin: note.OriginRanked, Lemma: "تَعِبٌ", POS: note.Adj, English: "tired",
		Example: "بَدَا أَنَّهُ [تَعِب].", ExampleEN: "He seemed tired.", Created: clock(), Updated: clock(),
	}
}

func bookNote() note.Note {
	return note.Note{
		ID: idBook, Seq: 2, Rank: 11, Origin: note.OriginRanked, Lemma: "كِتَابٌ", POS: note.Noun, English: "book",
		Example: "قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ.", ExampleEN: "I read the book at home.", Created: clock(), Updated: clock(),
	}
}

func endingIssue() verify.Issue {
	return verify.Issue{Field: verify.FieldExample, Kind: verify.KindUnmarked, Severity: verify.Major, Word: "تَعِب", Missing: []int{2}, CATT: "تَعِبٌ", CAMeL: "تَعِبَ", Detail: "no ending"}
}

func diacriticsIssue() verify.Issue {
	return verify.Issue{Field: verify.FieldExample, Kind: verify.KindDiacritics, Severity: verify.Major, Word: "الْبَيْتِ", CATT: "الْبَيْتُ", CAMeL: "الْبَيْتُ", Detail: "other vowels"}
}

func lemmaIssue() verify.Issue {
	return verify.Issue{Field: verify.FieldLemma, Kind: verify.KindUnknown, Severity: verify.Minor, Word: "كِتَابٌ", Detail: "unknown"}
}

type fakeVerifier struct {
	mu         sync.Mutex
	issues     map[string][]verify.Issue
	err        error
	unverified bool
	seen       []string
}

func (f *fakeVerifier) Check(ctx context.Context, notes []note.Note) (verify.Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	report := verify.Report{Results: map[string]verify.NoteResult{}}
	for _, n := range notes {
		report.Results[n.ID] = verify.NoteResult{Verified: !f.unverified, Issues: f.issues[n.ID]}
		f.seen = append(f.seen, n.Example)
	}
	return report, f.err
}

type fakeRewriter func(ctx context.Context, n note.Note, feedback string) (note.Note, error)

func (f fakeRewriter) Rewrite(ctx context.Context, n note.Note, feedback string) (note.Note, error) {
	return f(ctx, n, feedback)
}

type fakeAudio struct {
	mu   sync.Mutex
	dir  string
	made []string
}

func (a *fakeAudio) file(n note.Note, kind string) string {
	return filepath.Join(a.dir, n.ID+"-"+kind+".mp3")
}

func (a *fakeAudio) Clips(n note.Note) []ClipInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []ClipInfo
	for _, kind := range []string{"word", "sentence"} {
		_, err := os.Stat(a.file(n, kind))
		out = append(out, ClipInfo{Kind: kind, Ready: err == nil})
	}
	return out
}

func (a *fakeAudio) Path(n note.Note, kind string) (string, bool) {
	path := a.file(n, kind)
	_, err := os.Stat(path)
	return path, err == nil
}

func (a *fakeAudio) Remake(ctx context.Context, n note.Note, kind string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.made = append(a.made, n.ID+":"+kind)
	return os.WriteFile(a.file(n, kind), []byte("mp3"), 0o644)
}

func (a *fakeAudio) Remove(n note.Note, kind string) error {
	return os.Remove(a.file(n, kind))
}

type fakeVoices struct {
	mu      sync.Mutex
	current string
}

func (v *fakeVoices) Current() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.current
}

func (v *fakeVoices) List(ctx context.Context) ([]VoiceOption, error) {
	return []VoiceOption{{Name: "ar-XA-Wavenet-A"}, {Name: "ar-XA-Wavenet-B"}}, nil
}

func (v *fakeVoices) Set(name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.current = name
	return nil
}

type harness struct {
	t        *testing.T
	ws       workspace.Workspace
	store    *store.Store
	log      *decisions.Log
	sess     *session
	srv      *httptest.Server
	base     string
	finished chan struct{}
}

func newHarness(t *testing.T, notes []note.Note, issues map[string][]verify.Issue, tweak func(*Deps)) *harness {
	t.Helper()
	ws, err := workspace.Init(workspace.InitOptions{Root: filepath.Join(t.TempDir(), "ws"), DeckID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := jsonl.WriteFile(ws.NotesFile(), notes); err != nil {
		t.Fatal(err)
	}
	st := store.New(ws, clock, note.RandomIDs{})
	log := decisions.NewLog(ws.DecisionsFile(), clock)
	deps := Deps{Notes: st, Decisions: log, Prefix: "mufradat", Now: clock}
	if tweak != nil {
		tweak(&deps)
	}
	var items []Item
	for _, n := range notes {
		items = append(items, Item{ID: n.ID, Issues: issues[n.ID], Verified: true})
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h := &harness{t: t, ws: ws, store: st, log: log, finished: make(chan struct{})}
	h.sess = newSession(ctx, deps, items, Filter{Minor: true})
	h.srv = httptest.NewUnstartedServer(nil)
	var once sync.Once
	h.srv.Config.Handler = h.sess.handler("tok", h.srv.Listener.Addr().String(), func() { once.Do(func() { close(h.finished) }) })
	h.srv.Start()
	t.Cleanup(h.srv.Close)
	h.base = h.srv.URL + "/tok/"
	return h
}

func (h *harness) rev(id string) string {
	h.t.Helper()
	set, err := h.store.Load()
	if err != nil {
		h.t.Fatal(err)
	}
	n, ok := set.Get(id)
	if !ok {
		h.t.Fatalf("note %s is missing", id)
	}
	return n.Rev()
}

func (h *harness) note(id string) note.Note {
	h.t.Helper()
	set, err := h.store.Load()
	if err != nil {
		h.t.Fatal(err)
	}
	n, ok := set.Get(id)
	if !ok {
		h.t.Fatalf("note %s is missing", id)
	}
	return n
}

func (h *harness) state(id string) stateView {
	h.t.Helper()
	resp, err := http.Get(h.base + "api/state?id=" + id)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	var v stateView
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		h.t.Fatal(err)
	}
	return v
}

func (h *harness) post(action, id string, body map[string]any) (int, result) {
	h.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		h.t.Fatal(err)
	}
	resp, err := http.Post(h.base+"api/"+action+"?id="+id, "application/json", bytes.NewReader(raw))
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	var res result
	json.NewDecoder(resp.Body).Decode(&res)
	return resp.StatusCode, res
}

func (h *harness) wait(id string, done func(entryView) bool) entryView {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		v := h.state(id)
		if v.Entry != nil && done(*v.Entry) {
			return *v.Entry
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting; entry = %+v", v.Entry)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (h *harness) mustOK(action, id string, body map[string]any) result {
	h.t.Helper()
	code, res := h.post(action, id, body)
	if code != http.StatusOK {
		h.t.Fatalf("%s: status %d: %s", action, code, res.Message)
	}
	return res
}

func twoNotes() ([]note.Note, map[string][]verify.Issue) {
	return []note.Note{tiredNote(), bookNote()}, map[string][]verify.Issue{
		idTired: {endingIssue()},
		idBook:  {diacriticsIssue(), lemmaIssue()},
	}
}
