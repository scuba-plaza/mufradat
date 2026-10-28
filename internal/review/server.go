package review

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"
)

//go:embed web
var webFS embed.FS

func Serve(ctx context.Context, deps Deps, items []Item, filter Filter, started func(url string)) (Summary, error) {
	if deps.Notes == nil || deps.Decisions == nil {
		return Summary{}, errors.New("review needs the notes and the decisions log")
	}
	s := newSession(ctx, deps, items, filter)
	err := listen(ctx, s.handler, started)
	s.stopAll()
	return s.summary(), err
}

func listen(ctx context.Context, build func(token, host string, finish func()) http.Handler, started func(url string)) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	token := rand.Text()
	finished := make(chan struct{})
	var once sync.Once
	srv := &http.Server{
		Handler:           build(token, ln.Addr().String(), func() { once.Do(func() { close(finished) }) }),
		ReadHeaderTimeout: 10 * time.Second,
	}
	failed := make(chan error, 1)
	go func() { failed <- srv.Serve(ln) }()
	started(fmt.Sprintf("http://%s/%s/", ln.Addr(), token))
	var serveErr error
	select {
	case <-ctx.Done():
	case <-finished:
	case serveErr = <-failed:
	}
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}

func assets(mux *http.ServeMux, base string, font []byte) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET "+base, http.StripPrefix(base, http.FileServerFS(sub)))
	mux.HandleFunc("GET "+base+"font.ttf", func(w http.ResponseWriter, r *http.Request) {
		if len(font) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "font/ttf")
		_, _ = w.Write(font)
	})
}

func (s *session) handler(token, host string, finish func()) http.Handler {
	mux := http.NewServeMux()
	base := "/" + token + "/"
	assets(mux, base, s.deps.Font)
	mux.HandleFunc("GET "+base+"audio", s.audio)
	mux.HandleFunc("GET "+base+"api/state", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.view(r.URL.Query().Get("id"))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, result{Message: err.Error(), Tone: "bad"})
			return
		}
		writeJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("GET "+base+"api/voices", func(w http.ResponseWriter, r *http.Request) {
		voices, err := s.voices(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, result{Message: "the voices could not be listed: " + err.Error(), Tone: "bad", Show: r.URL.Query().Get("id")})
			return
		}
		writeJSON(w, http.StatusOK, voices)
	})
	mux.HandleFunc("POST "+base+"api/{action}", func(w http.ResponseWriter, r *http.Request) {
		s.act(w, r, finish)
	})
	return guard(host, mux)
}

func guard(host string, next http.Handler) http.Handler {
	_, port, _ := net.SplitHostPort(host)
	allowed := map[string]bool{host: true, "localhost:" + port: true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed[r.Host] {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); r.Method != http.MethodGet && origin != "" && origin != "http://"+r.Host {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *session) audio(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("clip")
	path := ""
	if s.deps.Audio != nil && clipKind(kind) {
		s.mu.Lock()
		if _, n, err := s.lookup(r.URL.Query().Get("id"), "", false); err == nil {
			path, _ = s.deps.Audio.Path(n, kind)
		}
		s.mu.Unlock()
	}
	if path == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	http.ServeFile(w, r, path)
}

type request struct {
	IfRev    string `json:"if_rev"`
	Note     *edit  `json:"note"`
	Source   string `json:"source"`
	Clip     string `json:"clip"`
	Voice    string `json:"voice"`
	Provider string `json:"provider"`
}

func readRequest(w http.ResponseWriter, r *http.Request) (request, error) {
	var req request
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return request{}, userErrorf("the request could not be read: %v", err)
	}
	return req, nil
}

func (s *session) act(w http.ResponseWriter, r *http.Request, finish func()) {
	id := r.URL.Query().Get("id")
	action := r.PathValue("action")
	req, err := readRequest(w, r)
	var res result
	if err == nil {
		res, err = s.dispatch(action, id, req)
	}
	switch {
	case errors.Is(err, errUnknownAction):
		http.NotFound(w, r)
	case err == nil && action == "finish":
		writeJSON(w, http.StatusOK, s.summary())
		finish()
	case err == nil:
		writeJSON(w, http.StatusOK, res)
	case isConflict(err):
		writeJSON(w, http.StatusConflict, result{Message: err.Error(), Tone: "warn", Show: id, Conflict: true})
	case isUserError(err):
		writeJSON(w, http.StatusBadRequest, result{Message: err.Error(), Tone: "bad", Show: id})
	default:
		writeJSON(w, http.StatusInternalServerError, result{Message: err.Error(), Tone: "bad", Show: id})
	}
}

var errUnknownAction = errors.New("unknown action")

func (s *session) dispatch(action, id string, req request) (result, error) {
	clip := func(do func(id, ifRev, kind string) (result, error)) (result, error) {
		if !clipKind(req.Clip) {
			return result{}, userErrorf("there is no %q clip on a note", req.Clip)
		}
		return do(id, req.IfRev, req.Clip)
	}
	switch action {
	case "keep":
		return s.keep(id, req.IfRev)
	case "save":
		if req.Note == nil {
			return result{}, userErrorf("the request had no note")
		}
		return s.save(id, req.IfRev, *req.Note)
	case "use":
		return s.use(id, req.IfRev)
	case "drop":
		return s.drop(id)
	case "ending":
		return s.ending(id, req.IfRev, req.Source)
	case "remake":
		return clip(s.remake)
	case "remove":
		return clip(s.removeClip)
	case "voice":
		return s.setVoice(id, req.Voice)
	case "ask":
		return s.ask(id)
	case "stop":
		return s.stop(id)
	case "undo":
		return s.undo()
	case "finish":
		return result{}, nil
	}
	return result{}, errUnknownAction
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
