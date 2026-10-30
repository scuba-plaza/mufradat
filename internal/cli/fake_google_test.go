package cli_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

type fakeGoogle struct {
	*httptest.Server
	synth    string
	mu       sync.Mutex
	problems []string
	inflight int
	peak     int
}

func newFakeGoogle(t *testing.T, synth string) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{synth: synth}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		json.NewEncoder(w).Encode(map[string]any{"access_token": "fake-token", "token_type": "Bearer", "expires_in": 3600})
	})
	mux.HandleFunc("GET /v1/voices", f.voices)
	mux.HandleFunc("POST /v1/text:synthesize", f.synthesize)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.problem(w, http.StatusNotFound, "unexpected request %s %s", r.Method, r.URL.RequestURI())
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeGoogle) enter() func() {
	f.mu.Lock()
	f.inflight++
	f.peak = max(f.peak, f.inflight)
	f.mu.Unlock()
	time.Sleep(40 * time.Millisecond)
	return func() {
		f.mu.Lock()
		f.inflight--
		f.mu.Unlock()
	}
}

func (f *fakeGoogle) resetPeak() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.peak = 0
}

func (f *fakeGoogle) peakSynthesis() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.peak
}

func (f *fakeGoogle) violations() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.problems, "\n")
}

func (f *fakeGoogle) problem(w http.ResponseWriter, status int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	f.mu.Lock()
	f.problems = append(f.problems, msg)
	f.mu.Unlock()
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":%d,"message":%q,"status":"INVALID_ARGUMENT"}}`, status, "fake server: "+msg)
}

func (f *fakeGoogle) authorized(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != "Bearer fake-token" {
		http.Error(w, `{"error":{"message":"unauthenticated","status":"UNAUTHENTICATED"}}`, http.StatusUnauthorized)
		return false
	}
	return true
}

func (f *fakeGoogle) voices(w http.ResponseWriter, r *http.Request) {
	if !f.authorized(w, r) {
		return
	}
	if q := r.URL.Query(); len(q) != 1 || q.Get("languageCode") != "ar-XA" {
		f.problem(w, http.StatusBadRequest, "query %v", q)
		return
	}
	io.WriteString(w, `{"voices":[
		{"languageCodes":["ar-XA"],"name":"ar-XA-Chirp3-HD-Achernar","ssmlGender":"FEMALE","naturalSampleRateHertz":24000},
		{"languageCodes":["ar-XA"],"name":"ar-XA-Wavenet-A","ssmlGender":"FEMALE","naturalSampleRateHertz":24000}]}`)
}

type synthesizeBody struct {
	Input struct {
		Text string `json:"text"`
	} `json:"input"`
	Voice struct {
		LanguageCode string `json:"languageCode"`
		Name         string `json:"name"`
	} `json:"voice"`
	AudioConfig struct {
		AudioEncoding string  `json:"audioEncoding"`
		SpeakingRate  float64 `json:"speakingRate"`
	} `json:"audioConfig"`
}

func (f *fakeGoogle) synthesize(w http.ResponseWriter, r *http.Request) {
	defer f.enter()()
	if !f.authorized(w, r) {
		return
	}
	if got := r.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		f.problem(w, http.StatusBadRequest, "Content-Type %q", got)
		return
	}
	var req synthesizeBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		f.problem(w, http.StatusBadRequest, "bad body: %v", err)
		return
	}
	switch {
	case strings.TrimSpace(req.Input.Text) == "":
		f.problem(w, http.StatusBadRequest, "empty text")
		return
	case req.Voice.LanguageCode != "ar-XA" || req.Voice.Name == "":
		f.problem(w, http.StatusBadRequest, "voice %+v", req.Voice)
		return
	case req.AudioConfig.AudioEncoding != "MP3" || req.AudioConfig.SpeakingRate < 0.25 || req.AudioConfig.SpeakingRate > 2:
		f.problem(w, http.StatusBadRequest, "audioConfig %+v", req.AudioConfig)
		return
	}
	appendLine(f.synth, req.Voice.Name)
	audio := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("MP3[%s]%s", req.Voice.Name, req.Input.Text)))
	fmt.Fprintf(w, `{"audioContent":%q}`, audio)
}

func serviceAccountKey(t *testing.T, key *rsa.PrivateKey, tokenURL string) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "test-project",
		"private_key_id": "k1",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   "tts@test-project.iam.gserviceaccount.com",
		"token_uri":      tokenURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func generateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func writeKey(t *testing.T, dir string, key *rsa.PrivateKey, tokenURL string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "key.json"), serviceAccountKey(t, key, tokenURL), 0o600); err != nil {
		t.Fatal(err)
	}
}

func googleOf(ts *testscript.TestScript) *fakeGoogle {
	f, _ := ts.Value(googleKey{}).(*fakeGoogle)
	if f == nil {
		ts.Fatalf("no fake Google server in this script")
	}
	return f
}

type googleKey struct{}

func googleCommands() map[string]func(ts *testscript.TestScript, neg bool, args []string) {
	return map[string]func(ts *testscript.TestScript, neg bool, args []string){
		"google-parallel": func(ts *testscript.TestScript, neg bool, args []string) {
			if neg || len(args) != 2 {
				ts.Fatalf("usage: google-parallel MIN MAX")
			}
			lo, err1 := strconv.Atoi(args[0])
			hi, err2 := strconv.Atoi(args[1])
			if err1 != nil || err2 != nil {
				ts.Fatalf("google-parallel: numbers expected")
			}
			if got := googleOf(ts).peakSynthesis(); got < lo || got > hi {
				ts.Fatalf("at most %d speech requests were in flight at once, want %d to %d", got, lo, hi)
			}
		},
		"google-reset": func(ts *testscript.TestScript, neg bool, args []string) {
			googleOf(ts).resetPeak()
		},
	}
}
