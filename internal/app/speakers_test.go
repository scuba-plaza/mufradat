package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"golang.org/x/time/rate"

	"github.com/scuba-plaza/mufradat/internal/clips"
)

type googleFixture struct {
	*httptest.Server
	mu          sync.Mutex
	project     string
	synthBody   map[string]any
	synthStatus int
	synthReply  string
}

func newGoogleFixture(t *testing.T) *googleFixture {
	t.Helper()
	f := &googleFixture{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, `{"access_token":"tok","token_type":"Bearer","expires_in":3600}`)
	})
	mux.HandleFunc("GET /v1/voices", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"voices":[
			{"languageCodes":["ar-XA"],"name":"ar-XA-A","ssmlGender":"FEMALE","naturalSampleRateHertz":24000},
			{"languageCodes":["ar-XA"],"name":"ar-XA-B","ssmlGender":"MALE","naturalSampleRateHertz":0}]}`)
	})
	mux.HandleFunc("POST /v1/text:synthesize", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.project = r.Header.Get("X-Goog-User-Project")
		f.synthBody = body
		status, reply := f.synthStatus, f.synthReply
		f.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			fmt.Fprint(w, reply)
			return
		}
		fmt.Fprint(w, `{"audioContent":"YQ=="}`)
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func serviceAccountFile(t *testing.T, tokenURL string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]string{
		"type": "service_account", "project_id": "p", "private_key_id": "k1",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email": "tts@p.iam.gserviceaccount.com", "token_uri": tokenURL,
	})
	path := filepath.Join(t.TempDir(), "key.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheGoogleSpeakerCarriesTheProjectVoiceRateAndRequestLimit(t *testing.T) {
	fx := newGoogleFixture(t)
	a, _ := appWith(t, map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": serviceAccountFile(t, fx.URL+"/token")})
	withWorkspace(t, a)
	setUser(t, a, map[string]string{
		"google.endpoint": fx.URL, "google.quota_project": "billing-project",
		"tts.requests_per_minute": "120", "tts.concurrency": "3",
	})
	sp, err := a.Speaker(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	audio, err := sp.Synthesize(context.Background(), clips.SpeechRequest{Text: "كِتَاب", Voice: "ar-XA-A", Rate: 0.8})
	if err != nil || string(audio) != "a" {
		t.Fatalf("%q, %v", audio, err)
	}
	fx.mu.Lock()
	defer fx.mu.Unlock()
	if fx.project != "billing-project" {
		t.Errorf("X-Goog-User-Project = %q", fx.project)
	}
	want := map[string]any{
		"input":       map[string]any{"text": "كِتَاب"},
		"voice":       map[string]any{"languageCode": "ar-XA", "name": "ar-XA-A"},
		"audioConfig": map[string]any{"audioEncoding": "MP3", "speakingRate": 0.8},
	}
	if !reflect.DeepEqual(fx.synthBody, want) {
		t.Errorf("request body %v, want %v", fx.synthBody, want)
	}
	limiter, ok := sp.(googleSpeaker).client.Limit.(*rate.Limiter)
	if !ok || limiter.Limit() != 2 || limiter.Burst() != 3 {
		t.Errorf("limiter %+v: 120 requests a minute with 3 at once is 2 per second with a burst of 3", sp.(googleSpeaker).client.Limit)
	}
}

func TestTheGoogleSpeakerListsVoicesWithGenderAndSampleRate(t *testing.T) {
	fx := newGoogleFixture(t)
	a, _ := appWith(t, map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": serviceAccountFile(t, fx.URL+"/token")})
	withWorkspace(t, a)
	setUser(t, a, map[string]string{"google.endpoint": fx.URL})
	sp, err := a.Speaker(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	voices, err := sp.Voices(context.Background())
	want := []clips.VoiceInfo{
		{Name: "ar-XA-A", Gender: "female", Detail: "24 kHz"},
		{Name: "ar-XA-B", Gender: "male"},
	}
	if err != nil || !reflect.DeepEqual(voices, want) {
		t.Errorf("voices %+v, %v; want %+v", voices, err, want)
	}
}

func TestTheVoiceIsTheConfiguredGoogleVoiceAtTheConfiguredRate(t *testing.T) {
	a, _ := appWith(t, nil)
	withWorkspace(t, a)
	setUser(t, a, map[string]string{"audio.rate": "0.7"})
	check := func(label string, want clips.Voice) {
		t.Helper()
		a.InvalidateConfig()
		got, err := a.Voice()
		if err != nil || got != want {
			t.Errorf("%s: voice %+v, %v; want %+v", label, got, err, want)
		}
	}
	check("default", clips.Voice{Name: "ar-XA-Chirp3-HD-Achernar", Rate: 0.7})
	setUser(t, a, map[string]string{"google.voice": "ar-XA-Wavenet-A"})
	check("named voice", clips.Voice{Name: "ar-XA-Wavenet-A", Rate: 0.7})
}
