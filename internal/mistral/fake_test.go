package mistral

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
)

const testKey = "test-key"

type reply struct {
	Status int
	Body   string
	Header map[string]string
}

type route struct {
	Method   string
	Path     string
	Query    url.Values
	AnyQuery bool
	Body     string
	Reply    func(call int, query url.Values) reply
}

type fake struct {
	*httptest.Server
	t      *testing.T
	mu     sync.Mutex
	routes map[string]route
	calls  map[string]int
	seen   []string
}

func newFake(t *testing.T, routes ...route) *fake {
	t.Helper()
	f := &fake{t: t, routes: map[string]route{}, calls: map[string]int{}}
	for _, r := range routes {
		f.routes[r.Method+" "+r.Path] = r
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fake) count(method, path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[method+" "+path]
}

func (f *fake) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

func (f *fake) serve(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.Path
	f.mu.Lock()
	f.seen = append(f.seen, key)
	rt, ok := f.routes[key]
	f.calls[key]++
	call := f.calls[key]
	f.mu.Unlock()
	if !ok {
		f.t.Errorf("unexpected request %s", key)
		http.Error(w, `{"object":"error","message":"no such route"}`, http.StatusNotFound)
		return
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+testKey {
		f.t.Errorf("%s: Authorization = %q", key, got)
	}
	if rt.Method == http.MethodPost {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			f.t.Errorf("%s: Content-Type = %q", key, got)
		}
	}
	if got, want := r.URL.Query(), rt.Query; !rt.AnyQuery && !reflect.DeepEqual(normalize(got), normalize(want)) {
		f.t.Errorf("%s: query = %v, want %v", key, got, want)
	}
	raw, _ := io.ReadAll(r.Body)
	if rt.Body == "" {
		if len(raw) != 0 {
			f.t.Errorf("%s: unexpected body %s", key, raw)
		}
	} else {
		var got, want any
		if err := json.Unmarshal(raw, &got); err != nil {
			f.t.Errorf("%s: body is not JSON: %v: %s", key, err, raw)
		} else if err := json.Unmarshal([]byte(rt.Body), &want); err != nil {
			f.t.Fatalf("bad expected body: %v", err)
		} else if !reflect.DeepEqual(got, want) {
			f.t.Errorf("%s: body mismatch\n got  %s\n want %s", key, raw, rt.Body)
		}
	}
	rep := rt.Reply(call, r.URL.Query())
	for k, v := range rep.Header {
		w.Header().Set(k, v)
	}
	if rep.Status == 0 {
		rep.Status = http.StatusOK
	}
	w.WriteHeader(rep.Status)
	fmt.Fprint(w, rep.Body)
}

func normalize(v url.Values) map[string][]string {
	if v == nil {
		return map[string][]string{}
	}
	return v
}

func ok(body string) func(int, url.Values) reply {
	return func(int, url.Values) reply { return reply{Body: body} }
}

func failing(status int, body string, header map[string]string) func(int, url.Values) reply {
	return func(int, url.Values) reply { return reply{Status: status, Body: body, Header: header} }
}
