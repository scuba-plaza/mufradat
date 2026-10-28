package review

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

type fakeSampler struct {
	mu    sync.Mutex
	dir   string
	calls []string
	err   error
}

func (f *fakeSampler) Sample(ctx context.Context, c Choice, text string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	f.calls = append(f.calls, c.Provider+"/"+c.Voice+"|"+text)
	path := filepath.Join(f.dir, "sample.mp3")
	return path, os.WriteFile(path, []byte("mp3"), 0o644)
}

type fakeChoices struct {
	mu      sync.Mutex
	current Choice
	options []VoiceOption
	notices []string
	listErr error
	useErr  error
	used    []Choice

	listCalls int
}

func twoProviders() []VoiceOption {
	return []VoiceOption{
		{Provider: "mistral", Name: "voice_479", Label: "Maryam", Gender: "female"},
		{Provider: "mistral", Name: "ar-XA-Wavenet-A", Label: "Same name, other provider"},
		{Provider: "google", Name: "ar-XA-Wavenet-A", Label: "Wavenet-A", Tier: "Wavenet"},
		{Provider: "google", Name: "ar-XA-Wavenet-B", Label: "Wavenet-B", Tier: "Wavenet"},
	}
}

func (f *fakeChoices) Current() Choice {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current
}

func (f *fakeChoices) List(ctx context.Context) ([]VoiceOption, []string, error) {
	f.mu.Lock()
	f.listCalls++
	f.mu.Unlock()
	return f.options, f.notices, f.listErr
}

func (f *fakeChoices) Use(c Choice) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.useErr != nil {
		return f.useErr
	}
	f.current = c
	f.used = append(f.used, c)
	return nil
}

func newComparison(t *testing.T, choices *fakeChoices) (*comparison, *fakeSampler, string) {
	t.Helper()
	sampler := &fakeSampler{dir: t.TempDir()}
	c := &comparison{deps: CompareDeps{
		Choices: choices, Sampler: sampler, Font: []byte("font"),
		Samples: []Sample{{ID: "s1", Label: "word", Text: "كِتَابٌ"}, {ID: "s2", Label: "sentence", Text: "قَرَأْتُ الْكِتَابَ."}},
	}}
	srv := httptest.NewUnstartedServer(nil)
	finished := make(chan struct{}, 1)
	srv.Config.Handler = c.handler("tok", srv.Listener.Addr().String(), func() { finished <- struct{}{} })
	srv.Start()
	t.Cleanup(srv.Close)
	return c, sampler, srv.URL + "/tok/"
}

func defaultChoices() *fakeChoices {
	return &fakeChoices{current: Choice{Provider: "mistral", Voice: "voice_479"}, options: twoProviders()}
}

func TestComparePageServesItsAssets(t *testing.T) {
	_, _, base := newComparison(t, defaultChoices())
	resp, err := http.Get(base)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("page = %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	for _, name := range []string{"compare.js", "compare.css", "app.css", "dom.js"} {
		resp, err := http.Get(base + name)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s = %d", name, resp.StatusCode)
		}
	}
}

func getView(t *testing.T, base string) (int, compareView) {
	t.Helper()
	resp, err := http.Get(base + "api/compare")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var view compareView
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, view
}

func TestTheComparisonListsTheVoicesOfEveryProviderAndWhichOneIsInUse(t *testing.T) {
	choices := defaultChoices()
	choices.notices = []string{"Google is not listed: no Google credentials found"}
	_, _, base := newComparison(t, choices)
	code, view := getView(t, base)
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if view.Provider != "mistral" || view.Voice != "voice_479" || len(view.Samples) != 2 {
		t.Errorf("in use: %+v", view)
	}
	if !slices.Equal(view.Voices, twoProviders()) {
		t.Errorf("voices = %+v", view.Voices)
	}
	if !slices.Equal(view.Notices, choices.notices) {
		t.Errorf("notices = %v", view.Notices)
	}
}

func TestAComparisonThatCannotListAnyVoiceSaysWhy(t *testing.T) {
	choices := defaultChoices()
	choices.listErr = errors.New("no Mistral API key found")
	_, _, base := newComparison(t, choices)
	resp, err := http.Get(base + "api/compare")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res result
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadGateway || res.Message != "the voices could not be listed: no Mistral API key found" || res.Tone != "bad" {
		t.Errorf("%d %+v", resp.StatusCode, res)
	}
}

func TestSamplesAreMadeOnDemandForListedVoicesOfTheRightProviderOnly(t *testing.T) {
	_, sampler, base := newComparison(t, defaultChoices())
	get := func(provider, voice, id string) int {
		q := url.Values{"provider": {provider}, "voice": {voice}, "id": {id}}
		resp, err := http.Get(base + "sample?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := get("google", "ar-XA-Wavenet-B", "s2"); code != http.StatusOK {
		t.Fatalf("a listed voice = %d", code)
	}
	if code := get("mistral", "ar-XA-Wavenet-A", "s1"); code != http.StatusOK {
		t.Fatalf("the same name under the other provider is another voice, and it is listed = %d", code)
	}
	want := []string{"google/ar-XA-Wavenet-B|قَرَأْتُ الْكِتَابَ.", "mistral/ar-XA-Wavenet-A|كِتَابٌ"}
	if !slices.Equal(sampler.calls, want) {
		t.Errorf("calls = %v, want %v", sampler.calls, want)
	}
	for _, c := range [][3]string{
		{"google", "voice_479", "s1"},
		{"mistral", "ar-XA-Wavenet-B", "s1"},
		{"azure", "ar-XA-Wavenet-A", "s1"},
		{"", "ar-XA-Wavenet-A", "s1"},
		{"google", "ar-XA-Wavenet-B", "nope"},
		{"google", "", "s1"},
	} {
		if code := get(c[0], c[1], c[2]); code != http.StatusNotFound {
			t.Errorf("%v = %d", c, code)
		}
	}
	if len(sampler.calls) != 2 {
		t.Errorf("a refused request still reached the paid service: %v", sampler.calls)
	}
	sampler.err = errors.New("quota")
	if code := get("google", "ar-XA-Wavenet-A", "s1"); code != http.StatusBadGateway {
		t.Errorf("a failing service = %d", code)
	}
}

func TestUsingAVoiceOfTheOtherProviderSwitchesProviderAndVoice(t *testing.T) {
	choices := defaultChoices()
	c, _, base := newComparison(t, choices)
	post := func(body string) (int, result) {
		resp, err := http.Post(base+"api/use", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var res result
		_ = json.NewDecoder(resp.Body).Decode(&res)
		return resp.StatusCode, res
	}
	code, res := post(`{"provider":"google","voice":"ar-XA-Wavenet-B"}`)
	want := Choice{Provider: "google", Voice: "ar-XA-Wavenet-B"}
	if code != http.StatusOK || choices.Current() != want {
		t.Fatalf("use = %d, in use %+v", code, choices.Current())
	}
	if res.Tone != "audio" || !strings.Contains(res.Message, "ar-XA-Wavenet-B (google) speaks every clip made from now on") {
		t.Errorf("message = %+v", res)
	}
	for _, body := range []string{
		`{"provider":"mistral","voice":"ar-XA-Wavenet-B"}`,
		`{"provider":"google","voice":"made-up"}`,
		`{"provider":"","voice":"ar-XA-Wavenet-B"}`,
		`{"voice":"ar-XA-Wavenet-B"}`,
	} {
		if code, res := post(body); code != http.StatusBadRequest || !strings.Contains(res.Message, "no Arabic voice") {
			t.Errorf("%s = %d %+v", body, code, res)
		}
	}
	if choices.Current() != want || len(choices.used) != 1 {
		t.Errorf("a refused choice changed something: %+v after %+v", choices.Current(), choices.used)
	}
	if c.used != want {
		t.Errorf("used = %+v", c.used)
	}
}

func TestAChoiceThatCannotBeSavedIsReported(t *testing.T) {
	choices := defaultChoices()
	choices.useErr = errors.New("the config file is read-only")
	c, _, base := newComparison(t, choices)
	resp, err := http.Post(base+"api/use", "application/json", strings.NewReader(`{"provider":"google","voice":"ar-XA-Wavenet-A"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res result
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if resp.StatusCode != http.StatusInternalServerError || res.Message != "the config file is read-only" || res.Tone != "bad" {
		t.Errorf("%d %+v", resp.StatusCode, res)
	}
	if c.used != (Choice{}) {
		t.Errorf("a choice that was not saved is reported as used: %+v", c.used)
	}
}

func TestCompareRunsUntilDoneAndReturnsWhatWasChosen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	choices := defaultChoices()
	urls := make(chan string, 1)
	done := make(chan Choice, 1)
	go func() {
		used, err := Compare(ctx, CompareDeps{Choices: choices, Sampler: &fakeSampler{dir: t.TempDir()}, Samples: []Sample{{ID: "s1", Label: "word", Text: "x"}}}, func(u string) { urls <- u })
		if err != nil {
			t.Error(err)
		}
		done <- used
	}()
	url := <-urls
	resp, err := http.Post(url+"api/use", "application/json", strings.NewReader(`{"provider":"google","voice":"ar-XA-Wavenet-B"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resp, err = http.Post(url+"api/finish", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var finished Choice
	if err := json.NewDecoder(resp.Body).Decode(&finished); err != nil {
		t.Fatal(err)
	}
	want := Choice{Provider: "google", Voice: "ar-XA-Wavenet-B"}
	if finished != want {
		t.Errorf("the page is told %+v", finished)
	}
	if used := <-done; used != want {
		t.Errorf("used = %+v", used)
	}
}

func TestCompareRefusesToStartWithoutAnythingItNeeds(t *testing.T) {
	full := func() CompareDeps {
		return CompareDeps{Choices: defaultChoices(), Sampler: &fakeSampler{dir: t.TempDir()}, Samples: []Sample{{ID: "s1", Text: "x"}}}
	}
	without := map[string]func(*CompareDeps){
		"voices":  func(d *CompareDeps) { d.Choices = nil },
		"sampler": func(d *CompareDeps) { d.Sampler = nil },
		"samples": func(d *CompareDeps) { d.Samples = nil },
	}
	for name, drop := range without {
		deps := full()
		drop(&deps)
		ctx, cancel := context.WithCancel(context.Background())
		started := false
		_, err := Compare(ctx, deps, func(string) { started = true; cancel() })
		cancel()
		if err == nil || err.Error() != "comparing voices needs voices, a way to speak and something to say" {
			t.Errorf("without %s: error %v", name, err)
		}
		if started {
			t.Errorf("without %s: the page was served anyway", name)
		}
	}
}

func TestTheVoicesAreListedOnlyOncePerComparison(t *testing.T) {
	choices := defaultChoices()
	_, _, base := newComparison(t, choices)
	getView(t, base)
	getView(t, base)
	for _, query := range []string{"provider=google&voice=ar-XA-Wavenet-A&id=s1", "provider=mistral&voice=voice_479&id=s2"} {
		resp, err := http.Get(base + "sample?" + query)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	resp, err := http.Post(base+"api/use", "application/json", strings.NewReader(`{"provider":"google","voice":"ar-XA-Wavenet-B"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	choices.mu.Lock()
	defer choices.mu.Unlock()
	if choices.listCalls != 1 {
		t.Errorf("the services were asked for their voices %d times, once is enough", choices.listCalls)
	}
}
