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

type Choice struct {
	Provider string `json:"provider"`
	Voice    string `json:"voice"`
}

type Sampler interface {
	Sample(ctx context.Context, c Choice, text string) (string, error)
}

type Candidates interface {
	Current() Choice
	List(ctx context.Context) ([]VoiceOption, []string, error)
	Use(c Choice) error
}

type CompareDeps struct {
	Choices Candidates
	Sampler Sampler
	Samples []Sample
	Font    []byte
}

type comparison struct {
	mu      sync.Mutex
	deps    CompareDeps
	listed  bool
	voices  []VoiceOption
	notices []string
	used    Choice
}

type compareView struct {
	Provider string        `json:"provider"`
	Voice    string        `json:"voice"`
	Samples  []Sample      `json:"samples"`
	Voices   []VoiceOption `json:"voices"`
	Notices  []string      `json:"notices"`
}

func Compare(ctx context.Context, deps CompareDeps, started func(url string)) (Choice, error) {
	if deps.Choices == nil || deps.Sampler == nil || len(deps.Samples) == 0 {
		return Choice{}, errors.New("comparing voices needs voices, a way to speak and something to say")
	}
	c := &comparison{deps: deps}
	err := listen(ctx, c.handler, started)
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.used, err
}

func (c *comparison) list(ctx context.Context) ([]VoiceOption, []string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.listed {
		return c.voices, c.notices, nil
	}
	voices, notices, err := c.deps.Choices.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	c.listed, c.voices, c.notices = true, voices, notices
	return voices, notices, nil
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
		voices, notices, err := c.list(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, result{Message: "the voices could not be listed: " + err.Error(), Tone: "bad"})
			return
		}
		current := c.deps.Choices.Current()
		writeJSON(w, http.StatusOK, compareView{Provider: current.Provider, Voice: current.Voice, Samples: c.deps.Samples, Voices: voices, Notices: notices})
	})
	mux.HandleFunc("GET "+base+"sample", c.sample)
	mux.HandleFunc("POST "+base+"api/use", c.use)
	mux.HandleFunc("POST "+base+"api/finish", func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		used := c.used
		c.mu.Unlock()
		writeJSON(w, http.StatusOK, used)
		finish()
	})
	return guard(host, mux)
}

func (c *comparison) known(ctx context.Context, choice Choice) bool {
	voices, _, err := c.list(ctx)
	return err == nil && slices.ContainsFunc(voices, func(v VoiceOption) bool {
		return v.Provider == choice.Provider && v.Name == choice.Voice
	})
}

func (c *comparison) sample(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	choice := Choice{Provider: query.Get("provider"), Voice: query.Get("voice")}
	i := slices.IndexFunc(c.deps.Samples, func(s Sample) bool { return s.ID == query.Get("id") })
	if i < 0 || !c.known(r.Context(), choice) {
		http.NotFound(w, r)
		return
	}
	path, err := c.deps.Sampler.Sample(r.Context(), choice, c.deps.Samples[i].Text)
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
	choice := Choice{Provider: req.Provider, Voice: req.Voice}
	if !c.known(r.Context(), choice) {
		writeJSON(w, http.StatusBadRequest, result{Message: "the service has no Arabic voice named " + req.Voice + " for " + req.Provider, Tone: "bad"})
		return
	}
	if err := c.deps.Choices.Use(choice); err != nil {
		writeJSON(w, http.StatusInternalServerError, result{Message: err.Error(), Tone: "bad"})
		return
	}
	c.mu.Lock()
	c.used = choice
	c.mu.Unlock()
	writeJSON(w, http.StatusOK, result{Message: "♪ " + choice.Voice + " (" + choice.Provider + ") speaks every clip made from now on; clips already made keep their voice until you remake them", Tone: "audio"})
}
