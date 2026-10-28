package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/review"
)

type comparing struct {
	a       *App
	google  *googleFixture
	mistral *mistralFixture
}

func newComparing(t *testing.T, vars map[string]string, settings map[string]string) comparing {
	t.Helper()
	google, mistral := newGoogleFixture(t), newMistralFixture(t)
	env := map[string]string{}
	for k, v := range vars {
		env[k] = v
	}
	if _, ok := env["GOOGLE_APPLICATION_CREDENTIALS"]; !ok {
		env["GOOGLE_APPLICATION_CREDENTIALS"] = serviceAccountFile(t, google.URL+"/token")
	}
	if _, ok := env["MISTRAL_API_KEY"]; !ok {
		env["MISTRAL_API_KEY"] = "secret-key"
	}
	for k, v := range env {
		if v == "-" {
			delete(env, k)
		}
	}
	a, _ := appWith(t, env)
	withWorkspace(t, a)
	user := map[string]string{"google.endpoint": google.URL, "mistral.endpoint": mistral.URL}
	for k, v := range settings {
		user[k] = v
	}
	setUser(t, a, user)
	return comparing{a: a, google: google, mistral: mistral}
}

func (c comparing) deps(t *testing.T) review.CompareDeps {
	t.Helper()
	deps, err := c.a.PrepareCompare([]string{"مَرْحَبًا"})
	if err != nil {
		t.Fatal(err)
	}
	return deps
}

func providers(options []review.VoiceOption) []string {
	var out []string
	for _, o := range options {
		out = append(out, o.Provider+"/"+o.Name)
	}
	return out
}

func TestTheComparisonListsBothProvidersWithTheOneInUseFirst(t *testing.T) {
	c := newComparing(t, nil, nil)
	deps := c.deps(t)
	if got := deps.Choices.Current(); got != (review.Choice{Provider: "mistral", Voice: "voice_479"}) {
		t.Errorf("in use = %+v", got)
	}
	options, notices, err := deps.Choices.List(context.Background())
	if err != nil || len(notices) != 0 {
		t.Fatalf("%v, notices %v", err, notices)
	}
	want := []string{"mistral/voice_479", "mistral/22222222-2222-4222-8222-222222222222", "google/ar-XA-A", "google/ar-XA-B"}
	if got := providers(options); !equalStrings(got, want) {
		t.Errorf("voices = %v, want %v", got, want)
	}
	if options[0].Label != "Maryam" || options[0].Gender != "female" {
		t.Errorf("a Mistral voice keeps its display name and gender: %+v", options[0])
	}

	c.a.InvalidateConfig()
	setUser(t, c.a, map[string]string{"audio.provider": "google"})
	deps = c.deps(t)
	options, _, err = deps.Choices.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"google/ar-XA-A", "google/ar-XA-B", "mistral/voice_479", "mistral/22222222-2222-4222-8222-222222222222"}
	if got := providers(options); !equalStrings(got, want) {
		t.Errorf("with Google in use, its voices come first: %v, want %v", got, want)
	}
	if got := deps.Choices.Current(); got != (review.Choice{Provider: "google", Voice: "ar-XA-Chirp3-HD-Achernar"}) {
		t.Errorf("in use = %+v", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAProviderThatIsNotSetUpIsLeftOutWithANoticeThatSaysHowToSetItUp(t *testing.T) {
	c := newComparing(t, map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "-"}, nil)
	options, notices, err := c.deps(t).Choices.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range options {
		if o.Provider != "mistral" {
			t.Errorf("a voice of %s is listed without credentials", o.Provider)
		}
	}
	if len(notices) != 1 || !strings.HasPrefix(notices[0], "Google voices are not listed: no Google credentials found") || !strings.Contains(notices[0], "auth google --import KEY.json") {
		t.Errorf("notices = %q", notices)
	}

	c = newComparing(t, map[string]string{"MISTRAL_API_KEY": "-"}, nil)
	options, notices, err = c.deps(t).Choices.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := providers(options); len(got) != 2 || got[0] != "google/ar-XA-A" {
		t.Errorf("voices = %v", got)
	}
	if len(notices) != 1 || !strings.HasPrefix(notices[0], "Mistral voices are not listed: no Mistral API key found") || !strings.Contains(notices[0], "auth mistral --import FILE") {
		t.Errorf("notices = %q", notices)
	}
}

func TestAServiceThatRefusesToListItsVoicesIsLeftOutWithItsReason(t *testing.T) {
	c := newComparing(t, nil, nil)
	c.mistral.mu.Lock()
	c.mistral.voicesStatus, c.mistral.voicesReply = 401, `{"message":"Unauthorized"}`
	c.mistral.mu.Unlock()
	options, notices, err := c.deps(t).Choices.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := providers(options); len(got) != 2 || got[0] != "google/ar-XA-A" {
		t.Errorf("voices = %v", got)
	}
	if len(notices) != 1 || !strings.HasPrefix(notices[0], "Mistral voices are not listed: ") || !strings.Contains(notices[0], "Unauthorized") {
		t.Errorf("notices = %q", notices)
	}
}

func TestWhenNoProviderCanListItsVoicesTheComparisonFailsWithTheReasonOfTheOneInUse(t *testing.T) {
	c := newComparing(t, map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "-", "MISTRAL_API_KEY": "-"}, nil)
	_, _, err := c.deps(t).Choices.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no Mistral API key found") {
		t.Errorf("err = %v", err)
	}
}

func TestUsingAVoiceSavesTheProviderOnlyWhenItChanges(t *testing.T) {
	c := newComparing(t, nil, nil)
	ws, err := c.a.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	raw := func() string {
		b, err := os.ReadFile(ws.ConfigFile())
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	deps := c.deps(t)
	if err := deps.Choices.Use(review.Choice{Provider: "mistral", Voice: "Omar"}); err != nil {
		t.Fatal(err)
	}
	if got := raw(); !strings.Contains(got, "voice = 'Omar'") || strings.Contains(got, "provider") {
		t.Errorf("the provider did not change, so only the voice is saved:\n%s", got)
	}
	if got := deps.Choices.Current(); got != (review.Choice{Provider: "mistral", Voice: "Omar"}) {
		t.Errorf("in use = %+v", got)
	}

	if err := deps.Choices.Use(review.Choice{Provider: "google", Voice: "ar-XA-B"}); err != nil {
		t.Fatal(err)
	}
	if got := raw(); !strings.Contains(got, "provider = 'google'") || !strings.Contains(got, "[google]\nvoice = 'ar-XA-B'") || !strings.Contains(got, "[mistral]\nvoice = 'Omar'") {
		t.Errorf("both the provider and the voice are saved, and the other provider keeps its voice:\n%s", got)
	}
	if v, err := c.a.Voice(); err != nil || v.Provider != "google" || v.Name != "ar-XA-B" {
		t.Errorf("voice %+v, %v", v, err)
	}

	if err := deps.Choices.Use(review.Choice{Provider: "mistral", Voice: "voice_479"}); err != nil {
		t.Fatal(err)
	}
	if v, err := c.a.Voice(); err != nil || v.Provider != "mistral" || v.Name != "voice_479" {
		t.Errorf("switching back: voice %+v, %v", v, err)
	}
	if err := deps.Choices.Use(review.Choice{Provider: "azure", Voice: "x"}); err == nil {
		t.Error("an unknown provider must be refused")
	}
}

func TestEachSampleIsMadeByTheProviderOfItsVoiceKeptApartAndMadeOnlyOnce(t *testing.T) {
	c := newComparing(t, nil, map[string]string{"audio.rate": "0.8"})
	ws, err := c.a.Workspace()
	if err != nil {
		t.Fatal(err)
	}
	sampler := c.deps(t).Sampler
	ctx := context.Background()

	googlePath, err := sampler.Sample(ctx, review.Choice{Provider: "google", Voice: "ar-XA-A"}, "مَرْحَبًا")
	if err != nil {
		t.Fatal(err)
	}
	c.google.mu.Lock()
	voice, _ := c.google.synthBody["voice"].(map[string]any)
	audio, _ := c.google.synthBody["audioConfig"].(map[string]any)
	c.google.synthBody = nil
	c.google.mu.Unlock()
	if voice["name"] != "ar-XA-A" || audio["speakingRate"] != 0.8 {
		t.Errorf("Google was asked for %v at %v", voice, audio)
	}

	mistralPath, err := sampler.Sample(ctx, review.Choice{Provider: "mistral", Voice: "voice_479"}, "مَرْحَبًا")
	if err != nil {
		t.Fatal(err)
	}
	c.mistral.mu.Lock()
	asked := c.mistral.speak["voice_id"]
	c.mistral.speak = nil
	c.mistral.mu.Unlock()
	if asked != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("Mistral was asked for voice %v", asked)
	}

	if googlePath == mistralPath || filepath.Dir(googlePath) != ws.VoiceSamplesDir() || filepath.Dir(mistralPath) != ws.VoiceSamplesDir() {
		t.Errorf("samples of two providers must be separate files in %s: %s and %s", ws.VoiceSamplesDir(), googlePath, mistralPath)
	}
	for _, p := range []string{googlePath, mistralPath} {
		if b, err := os.ReadFile(p); err != nil || string(b) != "a" {
			t.Errorf("%s: %q, %v", p, b, err)
		}
	}

	if _, err := sampler.Sample(ctx, review.Choice{Provider: "google", Voice: "ar-XA-A"}, "مَرْحَبًا"); err != nil {
		t.Fatal(err)
	}
	if _, err := sampler.Sample(ctx, review.Choice{Provider: "mistral", Voice: "voice_479"}, "مَرْحَبًا"); err != nil {
		t.Fatal(err)
	}
	c.google.mu.Lock()
	again := c.google.synthBody
	c.google.mu.Unlock()
	c.mistral.mu.Lock()
	againM := c.mistral.speak
	c.mistral.mu.Unlock()
	if again != nil || againM != nil {
		t.Errorf("a sample that was already made was paid for again: google %v, mistral %v", again, againM)
	}
}

func TestTwoVoicesOfOneProviderGetTheirOwnSamplesAndShareOneLogin(t *testing.T) {
	c := newComparing(t, nil, nil)
	sampler := c.deps(t).Sampler
	ctx := context.Background()
	first, err := sampler.Sample(ctx, review.Choice{Provider: "google", Voice: "ar-XA-A"}, "مَرْحَبًا")
	if err != nil {
		t.Fatal(err)
	}
	c.google.mu.Lock()
	c.google.synthBody = nil
	c.google.mu.Unlock()
	second, err := sampler.Sample(ctx, review.Choice{Provider: "google", Voice: "ar-XA-B"}, "مَرْحَبًا")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("two voices saying the same text share one sample file: %s", first)
	}
	c.google.mu.Lock()
	defer c.google.mu.Unlock()
	voice, _ := c.google.synthBody["voice"].(map[string]any)
	if voice["name"] != "ar-XA-B" {
		t.Errorf("the second voice was served from the first one's sample: Google was asked for %v", voice)
	}
	if c.google.tokens != 1 {
		t.Errorf("Google was logged in to %d times for two samples, one login serves them both", c.google.tokens)
	}
}
