package review

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func (f *fakeSampler) Sample(ctx context.Context, voice, text string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	f.calls = append(f.calls, voice+"|"+text)
	path := filepath.Join(f.dir, "sample.mp3")
	return path, os.WriteFile(path, []byte("mp3"), 0o644)
}

func newComparison(t *testing.T) (*comparison, *fakeVoices, *fakeSampler, *httptest.Server, string) {
	t.Helper()
	voices := &fakeVoices{current: "ar-XA-Wavenet-A"}
	sampler := &fakeSampler{dir: t.TempDir()}
	c := &comparison{deps: CompareDeps{
		Voices: voices, Sampler: sampler, Font: []byte("font"),
		Samples: []Sample{{ID: "s1", Label: "word", Text: "كِتَابٌ"}, {ID: "s2", Label: "sentence", Text: "قَرَأْتُ الْكِتَابَ."}},
	}}
	srv := httptest.NewUnstartedServer(nil)
	finished := make(chan struct{}, 1)
	srv.Config.Handler = c.handler("tok", srv.Listener.Addr().String(), func() { finished <- struct{}{} })
	srv.Start()
	t.Cleanup(srv.Close)
	return c, voices, sampler, srv, srv.URL + "/tok/"
}

func TestComparePageListsVoicesAndSamples(t *testing.T) {
	_, _, _, _, base := newComparison(t)
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
	resp, err = http.Get(base + "api/compare")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var view compareView
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if view.Voice != "ar-XA-Wavenet-A" || len(view.Voices) != 2 || len(view.Samples) != 2 {
		t.Errorf("view = %+v", view)
	}
}

func TestSamplesAreMadeOnDemandForKnownVoicesOnly(t *testing.T) {
	_, _, sampler, _, base := newComparison(t)
	get := func(query string) int {
		resp, err := http.Get(base + "sample?" + query)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := get("voice=ar-XA-Wavenet-B&id=s2"); code != http.StatusOK {
		t.Fatalf("a listed voice = %d", code)
	}
	if len(sampler.calls) != 1 || sampler.calls[0] != "ar-XA-Wavenet-B|قَرَأْتُ الْكِتَابَ." {
		t.Errorf("calls = %v", sampler.calls)
	}
	for _, query := range []string{"voice=en-US-Wavenet-A&id=s1", "voice=ar-XA-Wavenet-B&id=nope", "voice=&id=s1"} {
		if code := get(query); code != http.StatusNotFound {
			t.Errorf("%s = %d", query, code)
		}
	}
	if len(sampler.calls) != 1 {
		t.Errorf("a refused request still reached the paid service: %v", sampler.calls)
	}
	sampler.err = errors.New("quota")
	if code := get("voice=ar-XA-Wavenet-A&id=s1"); code != http.StatusBadGateway {
		t.Errorf("a failing service = %d", code)
	}
}

func TestUseWritesTheVoiceBack(t *testing.T) {
	c, voices, _, _, base := newComparison(t)
	post := func(voice string) int {
		resp, err := http.Post(base+"api/use", "application/json", strings.NewReader(`{"voice":"`+voice+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := post("ar-XA-Wavenet-B"); code != http.StatusOK || voices.Current() != "ar-XA-Wavenet-B" {
		t.Fatalf("use = %d, voice %q", code, voices.Current())
	}
	if code := post("made-up"); code != http.StatusBadRequest || voices.Current() != "ar-XA-Wavenet-B" {
		t.Errorf("an unknown voice = %d, voice %q", code, voices.Current())
	}
	if c.used != "ar-XA-Wavenet-B" {
		t.Errorf("used = %q", c.used)
	}
}

func TestCompareRunsUntilDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	voices := &fakeVoices{current: "ar-XA-Wavenet-A"}
	urls := make(chan string, 1)
	done := make(chan string, 1)
	go func() {
		used, err := Compare(ctx, CompareDeps{Voices: voices, Sampler: &fakeSampler{dir: t.TempDir()}, Samples: []Sample{{ID: "s1", Label: "word", Text: "x"}}}, func(u string) { urls <- u })
		if err != nil {
			t.Error(err)
		}
		done <- used
	}()
	url := <-urls
	resp, err := http.Post(url+"api/use", "application/json", strings.NewReader(`{"voice":"ar-XA-Wavenet-B"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resp, err = http.Post(url+"api/finish", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if used := <-done; used != "ar-XA-Wavenet-B" {
		t.Errorf("used = %q", used)
	}
	if _, err := Compare(ctx, CompareDeps{}, func(string) {}); err == nil {
		t.Error("Compare needs voices, a sampler and samples")
	}
}

func TestCompareRefusesToStartWithoutAnythingItNeeds(t *testing.T) {
	full := func() CompareDeps {
		return CompareDeps{Voices: &fakeVoices{current: "v"}, Sampler: &fakeSampler{dir: t.TempDir()}, Samples: []Sample{{ID: "s1", Text: "x"}}}
	}
	without := map[string]func(*CompareDeps){
		"voices":  func(d *CompareDeps) { d.Voices = nil },
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
