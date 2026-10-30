package app

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/review"
)

func reviewedNote() note.Note {
	return note.Note{
		Lemma: "كِتَاب", POS: note.Noun, English: "book",
		Forms:   []note.Form{{Label: "pl.", Arabic: "كُتُب"}, {Label: "pl.", Arabic: "كُتْب"}},
		Example: "قَرَأَ [الْوَلَدُ] الْكِتَابَ.", ExampleEN: "The boy read the book.",
	}
}

func reviewAudioFor(t *testing.T, fx *googleFixture) (review.Audio, *App) {
	t.Helper()
	a, _ := appWith(t, map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": serviceAccountFile(t, fx.URL+"/token")})
	withWorkspace(t, a)
	setUser(t, a, map[string]string{"google.endpoint": fx.URL})
	audio, err := a.reviewAudio()
	if err != nil {
		t.Fatal(err)
	}
	return audio, a
}

func TestTheReviewListsMakesFindsAndRemovesTheClipsOfANote(t *testing.T) {
	fx := newGoogleFixture(t)
	audio, _ := reviewAudioFor(t, fx)
	n := reviewedNote()
	ctx := context.Background()

	want := func(label string, ready ...bool) {
		t.Helper()
		got := audio.Clips(n)
		exp := []review.ClipInfo{{Kind: "word", Ready: ready[0]}, {Kind: "forms", Ready: ready[1]}, {Kind: "sentence", Ready: ready[2]}}
		if !reflect.DeepEqual(got, exp) {
			t.Errorf("%s: clips %+v, want %+v", label, got, exp)
		}
	}
	want("before anything is made", false, false, false)
	if path, ok := audio.Path(n, "forms"); ok || path != "" {
		t.Errorf("a clip that was not made has path %q", path)
	}

	if err := audio.Remake(ctx, n, "forms"); err != nil {
		t.Fatal(err)
	}
	want("after making the forms clip", false, true, false)
	fx.mu.Lock()
	body := fx.synthBody
	fx.mu.Unlock()
	wantBody := map[string]any{
		"input":       map[string]any{"text": "كُتُب، كُتْب"},
		"voice":       map[string]any{"languageCode": "ar-XA", "name": "ar-XA-Chirp3-HD-Achernar"},
		"audioConfig": map[string]any{"audioEncoding": "MP3", "speakingRate": 0.9},
	}
	if !reflect.DeepEqual(body, wantBody) {
		t.Errorf("the service was asked for %v, want %v", body, wantBody)
	}
	path, ok := audio.Path(n, "forms")
	if data, err := os.ReadFile(path); !ok || err != nil || string(data) != "a" {
		t.Errorf("path %q, %v, content %q, %v", path, ok, data, err)
	}
	if path, ok := audio.Path(n, "word"); ok || path != "" {
		t.Errorf("the word clip was never made but has path %q", path)
	}

	if err := audio.Remove(n, "forms"); err != nil {
		t.Fatal(err)
	}
	want("after removing it", false, false, false)
	if err := audio.Remove(n, "forms"); err != nil {
		t.Errorf("removing a clip that is not there is not an error: %v", err)
	}

	for name, call := range map[string]func() error{
		"remake": func() error { return audio.Remake(ctx, n, "banana") },
		"remove": func() error { return audio.Remove(n, "banana") },
	} {
		if err := call(); err == nil || err.Error() != "this note has no banana to speak" {
			t.Errorf("%s of an unknown kind: %v", name, err)
		}
	}
	if _, ok := audio.Path(n, "banana"); ok {
		t.Error("an unknown kind has a path")
	}
}

func TestTheReviewOffersNoSentenceClipForANoteWithoutAnExample(t *testing.T) {
	audio, _ := reviewAudioFor(t, newGoogleFixture(t))
	n := reviewedNote()
	n.Example, n.Forms = "", nil
	got := audio.Clips(n)
	if want := []review.ClipInfo{{Kind: "word"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("clips %+v, want %+v", got, want)
	}
	if err := audio.Remake(context.Background(), n, "sentence"); err == nil || err.Error() != "this note has no sentence to speak" {
		t.Errorf("err = %v", err)
	}
}

func TestARemadeClipThatTheServiceRefusesExplainsHowToFixIt(t *testing.T) {
	for _, c := range []struct {
		name    string
		status  int
		reply   string
		code    int
		message string
		hint    string
	}{
		{"bad key", 401, `{"error":{"code":401,"message":"Request had invalid authentication credentials","status":"UNAUTHENTICATED"}}`, failure.ExitNotSetUp, "invalid authentication credentials", "auth google --import KEY.json"},
		{"api disabled", 403, `{"error":{"code":403,"message":"Text-to-Speech API has not been used","status":"PERMISSION_DENIED","details":[{"reason":"SERVICE_DISABLED"}]}}`, failure.ExitTemporary, "has not been used", "google.quota_project"},
	} {
		fx := newGoogleFixture(t)
		fx.synthStatus, fx.synthReply = c.status, c.reply
		audio, _ := reviewAudioFor(t, fx)
		err := audio.Remake(context.Background(), reviewedNote(), "word")
		if err == nil || failure.Code(err) != c.code || !strings.Contains(err.Error(), c.message) || !strings.Contains(failure.Hint(err), c.hint) {
			t.Errorf("%s: %v (code %d, hint %q)", c.name, err, failure.Code(err), failure.Hint(err))
		}
		if audio.Clips(reviewedNote())[0].Ready {
			t.Errorf("%s: a refused clip must not be stored", c.name)
		}
	}
}

func TestTheReviewVoiceListNamesTheCurrentVoiceAndSavesAChoice(t *testing.T) {
	fx := newGoogleFixture(t)
	_, a := reviewAudioFor(t, fx)
	voices := reviewVoices{app: a}
	if got := voices.Current(); got != "ar-XA-Chirp3-HD-Achernar" {
		t.Errorf("current voice %q", got)
	}
	list, err := voices.List(context.Background())
	want := []review.VoiceOption{
		{Name: "ar-XA-A", Label: "ar-XA-A", Gender: "female"},
		{Name: "ar-XA-B", Label: "ar-XA-B", Gender: "male"},
	}
	if err != nil || !reflect.DeepEqual(list, want) {
		t.Fatalf("list %+v, %v; want %+v", list, err, want)
	}
	if err := voices.Set("ar-XA-B"); err != nil {
		t.Fatal(err)
	}
	if got := voices.Current(); got != "ar-XA-B" {
		t.Errorf("after choosing, the current voice is %q", got)
	}
	ws, err := a.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(ws.ConfigFile()); !strings.Contains(string(raw), "voice = 'ar-XA-B'") {
		t.Errorf("the choice is not in the workspace config:\n%s", raw)
	}
	if _, err := config.Load(ws.ConfigFile()); err != nil {
		t.Errorf("the saved config no longer loads: %v", err)
	}
}

func TestClosingAReviewSetupRunsItsCleanupAndToleratesHavingNone(t *testing.T) {
	calls := 0
	ReviewSetup{close: func() { calls++ }}.Close()
	if calls != 1 {
		t.Errorf("cleanup ran %d times", calls)
	}
	ReviewSetup{}.Close()
}
