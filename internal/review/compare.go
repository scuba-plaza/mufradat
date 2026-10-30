package review

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
)

type Sample struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type Sampler interface {
	Sample(ctx context.Context, voice, text string) (string, error)
}

type CompareDeps struct {
	Voices  Voices
	Sampler Sampler
	Samples []Sample
	Font    []byte
}

type comparison struct {
	mu     sync.Mutex
	deps   CompareDeps
	voices []VoiceOption
	used   string
}

type compareView struct {
	Voice   string        `json:"voice"`
	Samples []Sample      `json:"samples"`
	Voices  []VoiceOption `json:"voices"`
}

func Compare(ctx context.Context, deps CompareDeps, started func(url string)) (string, error) {
	if deps.Voices == nil || deps.Sampler == nil || len(deps.Samples) == 0 {
		return "", errors.New("comparing voices needs voices, a way to speak and something to say")
	}
	c := &comparison{deps: deps}
	err := listen(ctx, c.handler, started)
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.used, err
}

func (c *comparison) list(ctx context.Context) ([]VoiceOption, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.voices != nil {
		return c.voices, nil
	}
	voices, err := c.deps.Voices.List(ctx)
	if err != nil {
		return nil, err
	}
	c.voices = voices
	return voices, nil
}

func (c *comparison) handler(token, host string, finish func()) http.Handler {
	mux := http.NewServeMux()
	base := "/" + token + "/"
	assets(mux, base, c.deps.Font)
	mux.HandleFunc("GET "+base+"{$}", func(w http.ResponseWriter, r *http.Request) {
		page, err := webFS.ReadFile("web/compare.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("GET "+base+"api/compare", func(w http.ResponseWriter, r *http.Request) {
		voices, err := c.list(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, result{Message: "the voices could not be listed: " + err.Error(), Tone: "bad"})
			return
		}
		writeJSON(w, http.StatusOK, compareView{Voice: c.deps.Voices.Current(), Samples: c.deps.Samples, Voices: voices})
	})
	mux.HandleFunc("GET "+base+"sample", c.sample)
	mux.HandleFunc("POST "+base+"api/use", c.use)
	mux.HandleFunc("POST "+base+"api/finish", func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		used := c.used
		c.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"voice": used})
		finish()
	})
	return guard(host, mux)
}

func (c *comparison) known(ctx context.Context, name string) bool {
	voices, err := c.list(ctx)
	return err == nil && slices.ContainsFunc(voices, func(v VoiceOption) bool { return v.Name == name })
}

func (c *comparison) sample(w http.ResponseWriter, r *http.Request) {
	voice := r.URL.Query().Get("voice")
	i := slices.IndexFunc(c.deps.Samples, func(s Sample) bool { return s.ID == r.URL.Query().Get("id") })
	if i < 0 || !c.known(r.Context(), voice) {
		http.NotFound(w, r)
		return
	}
	path, err := c.deps.Sampler.Sample(r.Context(), voice, c.deps.Samples[i].Text)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, result{Message: err.Error(), Tone: "bad"})
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	http.ServeFile(w, r, path)
}

func (c *comparison) use(w http.ResponseWriter, r *http.Request) {
	req, err := readRequest(w, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, result{Message: err.Error(), Tone: "bad"})
		return
	}
	if !c.known(r.Context(), req.Voice) {
		writeJSON(w, http.StatusBadRequest, result{Message: "the service has no Arabic voice named " + req.Voice, Tone: "bad"})
		return
	}
	if err := c.deps.Voices.Set(req.Voice); err != nil {
		writeJSON(w, http.StatusInternalServerError, result{Message: err.Error(), Tone: "bad"})
		return
	}
	c.mu.Lock()
	c.used = req.Voice
	c.mu.Unlock()
	writeJSON(w, http.StatusOK, result{Message: "♪ " + req.Voice + " speaks every clip made from now on; clips already made keep their voice until you remake them", Tone: "audio"})
}
