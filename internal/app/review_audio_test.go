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

func reviewAudioFor(t *testing.T, fx *mistralFixture) (review.Audio, *App) {
	t.Helper()
	a, _ := appWith(t, map[string]string{"MISTRAL_API_KEY": "secret-key"})
	withWorkspace(t, a)
	setUser(t, a, map[string]string{"mistral.endpoint": fx.URL})
	audio, err := a.reviewAudio()
	if err != nil {
		t.Fatal(err)
	}
	return audio, a
}

func TestTheReviewListsMakesFindsAndRemovesTheClipsOfANote(t *testing.T) {
	fx := newMistralFixture(t)
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
	sent, voiceID := fx.speak["input"], fx.speak["voice_id"]
	fx.mu.Unlock()
	if sent != "كُتُب، كُتْب" || voiceID != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("the service was asked to say %q with voice %q", sent, voiceID)
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
	audio, _ := reviewAudioFor(t, newMistralFixture(t))
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
		{"bad key", 401, `{"message":"Unauthorized"}`, failure.ExitNotSetUp, "Unauthorized", "auth mistral --import FILE"},
		{"moderation", 403, `{"object":"error","message":"blocked","type":"guardrail_violation","code":1920}`, failure.ExitFailure, "blocked", "config set audio.provider google"},
	} {
		fx := newMistralFixture(t)
		fx.speechStatus, fx.speechReply = c.status, c.reply
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
	fx := newMistralFixture(t)
	_, a := reviewAudioFor(t, fx)
	voices := reviewVoices{app: a}
	if got := voices.Current(); got != "voice_479" {
		t.Errorf("current voice %q", got)
	}
	list, err := voices.List(context.Background())
	want := []review.VoiceOption{
		{Name: "voice_479", Label: "Maryam", Gender: "female"},
		{Name: "22222222-2222-4222-8222-222222222222", Label: "Omar", Gender: "male"},
	}
	if err != nil || !reflect.DeepEqual(list, want) {
		t.Fatalf("list %+v, %v; want %+v", list, err, want)
	}
	if err := voices.Set("Omar"); err != nil {
		t.Fatal(err)
	}
	if got := voices.Current(); got != "Omar" {
		t.Errorf("after choosing, the current voice is %q", got)
	}
	ws, err := a.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(ws.ConfigFile()); !strings.Contains(string(raw), "voice = 'Omar'") {
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
