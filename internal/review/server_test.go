package review

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/store"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

func TestGuardRefusesStrangers(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)

	get := func(path, host string) *http.Response {
		req, _ := http.NewRequest(http.MethodGet, h.srv.URL+path, nil)
		if host != "" {
			req.Host = host
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	if resp := get("/tok/api/state", ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("state = %d", resp.StatusCode)
	} else if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") || resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("headers missing: %v", resp.Header)
	}
	if resp := get("/tok/api/state", "evil.example"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("a foreign Host must be refused, got %d", resp.StatusCode)
	}
	if resp := get("/wrong/api/state", ""); resp.StatusCode != http.StatusNotFound {
		t.Errorf("a wrong token must 404, got %d", resp.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodPost, h.base+"api/keep?id="+idTired, strings.NewReader(`{}`))
	req.Header.Set("Origin", "http://evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("a foreign Origin must be refused, got %d", resp.StatusCode)
	}
	if h.state(idTired).Summary.Open != 2 {
		t.Error("the refused request changed something")
	}
}

func TestStateListsEntriesAndTheOpenOne(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)
	v := h.state(idBook)
	if len(v.List) != 2 || v.List[0].ID != idTired || v.List[1].Lemma != "كِتَابٌ" || v.List[0].Tone != "bad" || v.List[1].Flags != 2 {
		t.Fatalf("list = %+v", v.List)
	}
	if v.Entry == nil || v.Entry.ID != idBook || v.Entry.Rev != h.rev(idBook) || len(v.Entry.Flags) != 2 {
		t.Fatalf("entry = %+v", v.Entry)
	}
	if v.Mistral || v.Audio || v.Verify || v.Summary.Open != 2 || v.Summary.Total != 2 {
		t.Errorf("optional features should be off: %+v", v)
	}
	if v := h.state("n_zzzzzzzzzzz2"); v.Entry != nil {
		t.Errorf("an unknown id has no entry: %+v", v.Entry)
	}
	if v := h.state(""); v.Entry != nil || len(v.List) != 2 {
		t.Errorf("no id means the list only: %+v", v)
	}
}

func TestKeepDismissesEveryOpenFlagAndUndoRestoresThem(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)

	res := h.mustOK("keep", idBook, map[string]any{"if_rev": h.rev(idBook)})
	if res.Show != idTired || !strings.Contains(res.Message, "flags stay out") {
		t.Errorf("result = %+v", res)
	}
	set, err := h.log.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !set.Dismissed(idBook, "الْبَيْتِ") || !set.Dismissed(idBook, "كِتَابٌ") || set.Dismissed(idTired, "تَعِب") {
		t.Errorf("dismissals = %+v", set)
	}
	v := h.state(idBook)
	if v.Entry.State != "kept" || !v.Entry.Flags[0].Faded || v.Summary.Kept != 1 || v.Summary.Open != 1 || v.History != 1 {
		t.Errorf("after keep: %+v / %+v", v.Entry, v.Summary)
	}
	if !strings.Contains(v.Next, "mufradat build") {
		t.Errorf("next = %q", v.Next)
	}

	h.mustOK("undo", "", nil)
	set, _ = h.log.Load()
	if set.Dismissed(idBook, "الْبَيْتِ") || set.Count() != 0 {
		t.Errorf("undo must restore the flags: %+v", set)
	}
	if v := h.state(idBook); v.Entry.State != "open" || v.History != 0 {
		t.Errorf("after undo: %+v", v.Entry)
	}
	if code, res := h.post("undo", "", nil); code != http.StatusBadRequest || !strings.Contains(res.Message, "nothing to undo") {
		t.Errorf("empty undo = %d %+v", code, res)
	}
}

func TestEndingFixSavesThroughTheStoreAndUndoRestoresIt(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)

	fixes := h.state(idTired).Entry.Fixes
	if len(fixes) != 2 || fixes[0].Source != "catt" {
		t.Fatalf("fixes = %+v", fixes)
	}
	res := h.mustOK("ending", idTired, map[string]any{"if_rev": h.rev(idTired), "source": "catt"})
	if !strings.Contains(res.Message, "CATT's ending: تَعِبٌ") || !strings.Contains(res.Message, "run check again") {
		t.Errorf("message = %q", res.Message)
	}
	if got := h.note(idTired).Example; got != "بَدَا أَنَّهُ [تَعِبٌ]." {
		t.Fatalf("stored example = %q", got)
	}
	e := h.state(idTired).Entry
	if e.State != "edited" || e.Changed != "edited" || len(e.Fixes) != 0 || !e.Flags[0].Faded {
		t.Errorf("entry = %+v", e)
	}
	if v := h.state(idTired); v.Summary.Edited != 1 || !strings.Contains(v.Next, "mufradat audio") {
		t.Errorf("summary = %+v next = %q", v.Summary, v.Next)
	}
	if code, _ := h.post("ending", idTired, map[string]any{"if_rev": h.rev(idTired), "source": "catt"}); code != http.StatusBadRequest {
		t.Errorf("a second ending must be refused, got %d", code)
	}

	h.mustOK("undo", "", nil)
	if got := h.note(idTired).Example; got != "بَدَا أَنَّهُ [تَعِب]." {
		t.Errorf("undo left %q", got)
	}
	if e := h.state(idTired).Entry; e.State != "open" || e.Changed != "" || len(e.Fixes) != 2 {
		t.Errorf("after undo: %+v", e)
	}
}

func TestChangesAreCheckedAgain(t *testing.T) {
	notes, issues := twoNotes()
	ver := &fakeVerifier{issues: map[string][]verify.Issue{}}
	h := newHarness(t, notes, issues, func(d *Deps) { d.Verifier = ver })

	h.mustOK("ending", idTired, map[string]any{"if_rev": h.rev(idTired), "source": "catt"})
	e := h.wait(idTired, func(e entryView) bool { return !e.Checking })
	if e.State != "edited" || len(e.Flags) != 0 || !strings.Contains(e.Notice, "nothing is flagged") || e.NoticeTone != "good" {
		t.Errorf("a clean recheck leaves the edit alone: %+v", e)
	}
	if len(ver.seen) != 1 || ver.seen[0] != "بَدَا أَنَّهُ [تَعِبٌ]." {
		t.Errorf("the verifier saw %q", ver.seen)
	}

	ver.mu.Lock()
	ver.issues[idBook] = []verify.Issue{diacriticsIssue()}
	ver.mu.Unlock()
	ed := editOf(h.note(idBook))
	ed.Example = "قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ الْكَبِيرِ."
	h.mustOK("save", idBook, map[string]any{"if_rev": h.rev(idBook), "note": ed})
	e = h.wait(idBook, func(e entryView) bool { return !e.Checking })
	if e.State != "open" || e.Changed != "edited" || len(e.Flags) != 1 || !strings.Contains(e.Notice, "Still flagged") {
		t.Errorf("a flagged recheck reopens the entry: %+v", e)
	}
	if s := h.state(idBook).Summary; s.Edited != 2 || s.Open != 1 {
		t.Errorf("summary = %+v", s)
	}
}

func TestRecheckFailureIsReportedNotHidden(t *testing.T) {
	notes, issues := twoNotes()
	ver := &fakeVerifier{err: errors.New("the sidecar crashed")}
	h := newHarness(t, notes, issues, func(d *Deps) { d.Verifier = ver })
	h.mustOK("ending", idTired, map[string]any{"if_rev": h.rev(idTired), "source": "catt"})
	e := h.wait(idTired, func(e entryView) bool { return !e.Checking })
	if e.State != "edited" || !strings.Contains(e.Notice, "sidecar crashed") || e.NoticeTone != "warn" {
		t.Errorf("entry = %+v", e)
	}
}

func TestStaleRevisionIsAConflictAndChangesNothing(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)
	stale := h.rev(idBook)
	err := h.sess.deps.Notes.Update(context.Background(), func(tx *store.Tx) error {
		_, err := tx.Modify(idBook, "", func(n *note.Note) error {
			n.Hint = "changed elsewhere"
			return nil
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"keep", "use", "save", "ending"} {
		body := map[string]any{"if_rev": stale, "source": "catt", "note": editOf(bookNote())}
		code, res := h.post(action, idBook, body)
		if code != http.StatusConflict || !res.Conflict || res.Show != idBook {
			t.Errorf("%s with a stale revision = %d %+v", action, code, res)
		}
	}
	if set, _ := h.log.Load(); set.Count() != 0 {
		t.Error("a conflicting keep must not record anything")
	}
	if got := h.note(idBook).Hint; got != "changed elsewhere" {
		t.Errorf("hint = %q", got)
	}
	if code, res := h.post("keep", idBook, map[string]any{}); code != http.StatusBadRequest || !strings.Contains(res.Message, "which version") {
		t.Errorf("a missing if_rev is a usage error: %d %+v", code, res)
	}
	h.mustOK("keep", idBook, map[string]any{"if_rev": h.rev(idBook)})
}

func TestNotesAddedWhileReviewingSurvive(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)
	var added note.Note
	err := h.sess.deps.Notes.Update(context.Background(), func(tx *store.Tx) error {
		var err error
		added, err = tx.Add(note.Note{
			Origin: note.OriginNamed, Lemma: "قَلَمٌ", POS: note.Noun, English: "pen",
			Example: "هَذَا [الْقَلَمُ] جَدِيدٌ.", ExampleEN: "This pen is new.",
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	h.mustOK("ending", idTired, map[string]any{"if_rev": h.rev(idTired), "source": "catt"})
	ed := editOf(h.note(idBook))
	ed.Hint = "a hint"
	h.mustOK("save", idBook, map[string]any{"if_rev": h.rev(idBook), "note": ed})
	h.mustOK("undo", "", nil)

	set, err := h.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := set.Get(added.ID); !ok || got.Lemma != "قَلَمٌ" || set.Len() != 3 {
		t.Errorf("the note added in the meantime was lost: %d notes", set.Len())
	}
	if h.note(idBook).Hint != "" || h.note(idTired).Example != "بَدَا أَنَّهُ [تَعِبٌ]." {
		t.Error("the undo must revert only the last change and keep the earlier edit")
	}
}

func TestSaveAppliesOnlyEditableFieldsAndValidates(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)
	before := h.note(idBook)

	ed := editOf(before)
	if res := h.mustOK("save", idBook, map[string]any{"if_rev": h.rev(idBook), "note": ed}); res.Message != "No changes" {
		t.Errorf("an unchanged save = %+v", res)
	}
	if h.state(idBook).History != 0 {
		t.Error("an unchanged save must not enter the history")
	}

	ed.English = "  a book  "
	ed.Forms = []note.Form{{Label: "pl.", Arabic: "كُتُبٌ"}, {}}
	h.mustOK("save", idBook, map[string]any{"if_rev": h.rev(idBook), "note": ed})
	after := h.note(idBook)
	if after.English != "a book" || len(after.Forms) != 1 || after.Forms[0].Arabic != "كُتُبٌ" {
		t.Errorf("saved %+v", after)
	}
	if after.Rank != before.Rank || after.ID != before.ID || after.Seq != before.Seq || !after.Created.Equal(before.Created) {
		t.Errorf("identity fields changed: %+v", after)
	}

	bad := editOf(after)
	bad.Example = "قَرَأْتُ الْكِتَابَ"
	code, res := h.post("save", idBook, map[string]any{"if_rev": h.rev(idBook), "note": bad})
	if code != http.StatusBadRequest || res.Message == "" {
		t.Errorf("an invalid example = %d %+v", code, res)
	}
	if h.note(idBook).Example != before.Example {
		t.Error("an invalid save wrote something")
	}
	if code, _ := h.post("save", idBook, map[string]any{"if_rev": h.rev(idBook), "bogus": 1}); code != http.StatusBadRequest {
		t.Errorf("unknown fields are refused, got %d", code)
	}
}

func TestAskProposesAndUseSavesTheSuggestion(t *testing.T) {
	notes, issues := twoNotes()
	var feedback string
	rewriter := fakeRewriter(func(ctx context.Context, n note.Note, fb string) (note.Note, error) {
		feedback = fb
		n.Example = "قَرَأْتُ [الْكِتَابَ] فِي الْمَكْتَبَةِ."
		return n, nil
	})
	h := newHarness(t, notes, issues, func(d *Deps) { d.Rewriter = rewriter })

	h.mustOK("ask", idBook, nil)
	e := h.wait(idBook, func(e entryView) bool { return !e.Asking })
	if e.Proposal == nil || len(e.Changes) != 1 || e.Changes[0].Label != "sentence" || !strings.Contains(feedback, "الْبَيْتِ") {
		t.Fatalf("entry = %+v\nfeedback = %q", e, feedback)
	}
	if !h.state(idBook).List[1].Ready {
		t.Error("the list should show that a suggestion is ready")
	}
	if code, _ := h.post("use", idBook, map[string]any{"if_rev": "0000000000000000"}); code != http.StatusConflict {
		t.Errorf("using a suggestion on a stale revision = %d", code)
	}

	res := h.mustOK("use", idBook, map[string]any{"if_rev": e.Rev})
	if !strings.Contains(res.Message, "Mistral's version") || res.Tone != "mistral" {
		t.Errorf("result = %+v", res)
	}
	if got := h.note(idBook).Example; got != "قَرَأْتُ [الْكِتَابَ] فِي الْمَكْتَبَةِ." {
		t.Errorf("stored %q", got)
	}
	e = *h.state(idBook).Entry
	if e.State != "rewritten" || e.Proposal != nil {
		t.Errorf("entry = %+v", e)
	}
	if code, res := h.post("ask", idBook, nil); code != http.StatusBadRequest || !strings.Contains(res.Message, "undo the change") {
		t.Errorf("asking about a changed note = %d %+v", code, res)
	}
	h.mustOK("undo", "", nil)
	if got := h.note(idBook).Example; got != bookNote().Example {
		t.Errorf("undo left %q", got)
	}
}

func TestSuggestionGoesStaleWhenTheNoteChanges(t *testing.T) {
	notes, issues := twoNotes()
	rewriter := fakeRewriter(func(ctx context.Context, n note.Note, fb string) (note.Note, error) {
		n.English = "the book"
		return n, nil
	})
	h := newHarness(t, notes, issues, func(d *Deps) { d.Rewriter = rewriter })
	h.mustOK("ask", idBook, nil)
	h.wait(idBook, func(e entryView) bool { return e.Proposal != nil })

	ed := editOf(h.note(idBook))
	ed.Hint = "edited by hand"
	h.mustOK("save", idBook, map[string]any{"if_rev": h.rev(idBook), "note": ed})
	if e := h.state(idBook).Entry; e.Proposal != nil {
		t.Errorf("an edit must discard the suggestion: %+v", e.Proposal)
	}
	code, res := h.post("use", idBook, map[string]any{"if_rev": h.rev(idBook)})
	if code != http.StatusBadRequest || !strings.Contains(res.Message, "no suggestion") {
		t.Errorf("use = %d %+v", code, res)
	}
}

func TestAskOutcomesBecomeNotices(t *testing.T) {
	notes, issues := twoNotes()
	mode := "fail"
	rewriter := fakeRewriter(func(ctx context.Context, n note.Note, fb string) (note.Note, error) {
		switch mode {
		case "fail":
			return note.Note{}, errors.New("the service is unavailable")
		case "same":
			return n, nil
		}
		<-ctx.Done()
		return note.Note{}, ctx.Err()
	})
	h := newHarness(t, notes, issues, func(d *Deps) { d.Rewriter = rewriter })

	h.mustOK("ask", idBook, nil)
	e := h.wait(idBook, func(e entryView) bool { return !e.Asking && e.Notice != "" })
	if e.NoticeTone != "bad" || !strings.Contains(e.Notice, "the service is unavailable") || e.Proposal != nil {
		t.Errorf("failure: %+v", e)
	}
	mode = "same"
	h.mustOK("ask", idBook, nil)
	e = h.wait(idBook, func(e entryView) bool { return !e.Asking && strings.Contains(e.Notice, "keep this note") })
	if e.NoticeTone != "info" {
		t.Errorf("unchanged: %+v", e)
	}
	mode = "hang"
	h.mustOK("ask", idBook, nil)
	h.wait(idBook, func(e entryView) bool { return e.Asking })
	h.mustOK("stop", idBook, nil)
	e = h.wait(idBook, func(e entryView) bool { return !e.Asking })
	if e.Notice != "" || e.Proposal != nil {
		t.Errorf("stopped: %+v", e)
	}
}

func TestAskWithoutMistralSaysSo(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)
	if code, res := h.post("ask", idBook, nil); code != http.StatusBadRequest || !strings.Contains(res.Message, "Mistral is not set up") {
		t.Errorf("ask = %d %+v", code, res)
	}
}

func TestClipsCanBeRemadeRemovedAndPlayed(t *testing.T) {
	notes, issues := twoNotes()
	audio := &fakeAudio{dir: t.TempDir()}
	h := newHarness(t, notes, issues, func(d *Deps) { d.Audio = audio })

	if clips := h.state(idBook).Entry.Clips; len(clips) != 2 || clips[0].Ready {
		t.Fatalf("clips = %+v", clips)
	}
	get := func() int {
		resp, err := http.Get(h.base + "audio?id=" + idBook + "&clip=word")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if get() != http.StatusNotFound {
		t.Error("a missing clip must 404")
	}
	res := h.mustOK("remake", idBook, map[string]any{"clip": "word"})
	if !strings.Contains(res.Message, "word audio of كِتَابٌ") || len(audio.made) != 1 || audio.made[0] != idBook+":word" {
		t.Errorf("remake = %+v made %v", res, audio.made)
	}
	if !h.state(idBook).Entry.Clips[0].Ready || get() != http.StatusOK {
		t.Error("the remade clip should be ready and playable")
	}
	h.mustOK("remove", idBook, map[string]any{"clip": "word"})
	if h.state(idBook).Entry.Clips[0].Ready {
		t.Error("the clip should be gone")
	}
	if _, err := os.Stat(audio.file(bookNote(), "word")); !os.IsNotExist(err) {
		t.Errorf("file still there: %v", err)
	}
	for _, action := range []string{"remake", "remove"} {
		if code, _ := h.post(action, idBook, map[string]any{"clip": "../x"}); code != http.StatusBadRequest {
			t.Errorf("%s with a bad clip = %d", action, code)
		}
	}
	resp, err := http.Get(h.base + "audio?id=" + idBook + "&clip=../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("a bad clip name = %d", resp.StatusCode)
	}
}

func TestAudioActionsNeedAnAudioBackend(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)
	for _, action := range []string{"remake", "remove"} {
		if code, res := h.post(action, idBook, map[string]any{"clip": "word"}); code != http.StatusBadRequest || !strings.Contains(res.Message, "cannot") {
			t.Errorf("%s = %d %+v", action, code, res)
		}
	}
}

func TestVoiceCanBeListedAndChanged(t *testing.T) {
	notes, issues := twoNotes()
	voices := &fakeVoices{current: "ar-XA-Wavenet-A"}
	h := newHarness(t, notes, issues, func(d *Deps) { d.Voices = voices })

	if v := h.state(idBook); v.Voice != "ar-XA-Wavenet-A" {
		t.Errorf("voice = %q", v.Voice)
	}
	resp, err := http.Get(h.base + "api/voices")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("voices = %d", resp.StatusCode)
	}
	res := h.mustOK("voice", idBook, map[string]any{"voice": "ar-XA-Wavenet-B"})
	if voices.Current() != "ar-XA-Wavenet-B" || !strings.Contains(res.Message, "speaks every clip made from now on") {
		t.Errorf("voice = %q result %+v", voices.Current(), res)
	}
	if code, _ := h.post("voice", idBook, map[string]any{"voice": ""}); code != http.StatusBadRequest {
		t.Errorf("an empty voice = %d", code)
	}
}

func TestFinishEndsTheSessionWithASummary(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)
	h.mustOK("keep", idBook, map[string]any{"if_rev": h.rev(idBook)})
	h.mustOK("ending", idTired, map[string]any{"if_rev": h.rev(idTired), "source": "catt"})
	h.mustOK("finish", "", nil)
	select {
	case <-h.finished:
	default:
		t.Error("finish must signal the server to stop")
	}
	if s := h.sess.summary(); s != (Summary{Total: 2, Kept: 1, Edited: 1}) {
		t.Errorf("summary = %+v", s)
	}
	if code, _ := h.post("nonsense", "", nil); code != http.StatusNotFound {
		t.Errorf("an unknown action = %d", code)
	}
}

func TestServeRunsUntilFinished(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	urls := make(chan string, 1)
	done := make(chan Summary, 1)
	go func() {
		sum, err := Serve(ctx, h.sess.deps, []Item{{ID: idBook, Issues: []verify.Issue{diacriticsIssue()}, Verified: true}}, Filter{}, func(u string) { urls <- u })
		if err != nil {
			t.Error(err)
		}
		done <- sum
	}()
	url := <-urls
	resp, err := http.Get(url + "api/state")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("url %s status %d", url, resp.StatusCode)
	}
	resp, err = http.Post(url+"api/finish", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if sum := <-done; sum.Total != 1 || sum.Open != 1 {
		t.Errorf("summary = %+v", sum)
	}
	if _, err := Serve(ctx, Deps{}, nil, Filter{}, func(string) {}); err == nil {
		t.Error("Serve needs notes and decisions")
	}
}

func TestItemsSelectFlaggedNotes(t *testing.T) {
	notes, issues := twoNotes()
	report := verify.Report{Results: map[string]verify.NoteResult{
		idTired: {Verified: true, Issues: issues[idTired]},
		idBook:  {Verified: true, Issues: issues[idBook]},
	}}
	got := Items(notes, report, nil, Filter{})
	if len(got) != 2 || len(got[1].Issues) != 1 || got[1].Issues[0].Word != "الْبَيْتِ" {
		t.Fatalf("major only: %+v", got)
	}
	if got := Items(notes, report, nil, Filter{Minor: true}); len(got[1].Issues) != 2 {
		t.Errorf("minor too: %+v", got)
	}
	report.Results[idTired] = verify.NoteResult{Verified: true}
	if got := Items(notes, report, nil, Filter{}); len(got) != 1 || got[0].ID != idBook {
		t.Errorf("clean notes are skipped: %+v", got)
	}
	if got := Items(notes, report, nil, Filter{All: true}); len(got) != 2 {
		t.Errorf("--all keeps every note: %+v", got)
	}
	report.Results[idBook] = verify.NoteResult{Verified: false}
	if got := Items(notes, report, nil, Filter{}); len(got) != 0 {
		t.Errorf("unverified notes have no flags to review: %+v", got)
	}
}

func TestFeedbackNamesEachFlag(t *testing.T) {
	text := Feedback([]verify.Issue{diacriticsIssue(), {Field: verify.FieldExample, Kind: verify.KindUnchecked, Detail: "no reading"}})
	for _, want := range []string{"الْبَيْتِ in the sentence: other vowels", "the sentence: no reading", "reads only one way"} {
		if !strings.Contains(text, want) {
			t.Errorf("feedback lacks %q:\n%s", want, text)
		}
	}
	if !strings.Contains(Feedback(nil), "Nothing was flagged") {
		t.Error("an unflagged note says so")
	}
}

func TestPageAndModulesAreServed(t *testing.T) {
	notes, issues := twoNotes()
	h := newHarness(t, notes, issues, func(d *Deps) { d.Font = []byte("font-bytes") })
	fetch := func(path string) (int, string, string) {
		resp, err := http.Get(h.base + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return resp.StatusCode, resp.Header.Get("Content-Type"), b.String()
	}
	code, kind, page := fetch("")
	if code != http.StatusOK || !strings.HasPrefix(kind, "text/html") || !strings.Contains(page, `type="module" src="app.js"`) {
		t.Fatalf("index = %d %s", code, kind)
	}
	if code, kind, _ := fetch("app.css"); code != http.StatusOK || !strings.HasPrefix(kind, "text/css") {
		t.Errorf("css = %d %s", code, kind)
	}
	seen := map[string]bool{}
	queue := []string{"app.js"}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		code, kind, body := fetch(name)
		if code != http.StatusOK || !strings.HasPrefix(kind, "text/javascript") {
			t.Errorf("%s = %d %s", name, code, kind)
			continue
		}
		for _, line := range strings.Split(body, "\n") {
			if i := strings.Index(line, "from './"); i >= 0 && strings.HasPrefix(strings.TrimSpace(line), "import") {
				queue = append(queue, strings.TrimSuffix(line[i+len("from './"):], "';"))
			}
		}
	}
	if len(seen) < 7 {
		t.Errorf("only %d modules were reachable: %v", len(seen), seen)
	}
	if code, kind, body := fetch("font.ttf"); code != http.StatusOK || kind != "font/ttf" || body != "font-bytes" {
		t.Errorf("font = %d %s %q", code, kind, body)
	}
}
