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
	"sync"
	"testing"
)

type fakeGoogle struct {
	*httptest.Server
	mu    sync.Mutex
	calls []string
}

func newFakeGoogle(t *testing.T, calls string) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		json.NewEncoder(w).Encode(map[string]any{"access_token": "fake-token", "token_type": "Bearer", "expires_in": 3600})
	})
	mux.HandleFunc("/v1/voices", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-token" {
			http.Error(w, `{"error":{"message":"unauthenticated","status":"UNAUTHENTICATED"}}`, http.StatusUnauthorized)
			return
		}
		io.WriteString(w, `{"voices":[
			{"name":"ar-XA-Chirp3-HD-Achernar","ssmlGender":"FEMALE","naturalSampleRateHertz":24000},
			{"name":"ar-XA-Wavenet-A","ssmlGender":"FEMALE","naturalSampleRateHertz":24000}]}`)
	})
	mux.HandleFunc("/v1/text:synthesize", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-token" {
			http.Error(w, `{"error":{"message":"unauthenticated","status":"UNAUTHENTICATED"}}`, http.StatusUnauthorized)
			return
		}
		var req struct {
			Input struct{ Text string } `json:"input"`
			Voice struct{ Name string } `json:"voice"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.calls = append(f.calls, req.Voice.Name)
		if calls != "" {
			appendLine(calls, req.Voice.Name)
		}
		f.mu.Unlock()
		audio := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("MP3[%s]%s", req.Voice.Name, req.Input.Text)))
		fmt.Fprintf(w, `{"audioContent":%q}`, audio)
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func appendLine(path, line string) {
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer fh.Close()
	fmt.Fprintln(fh, line)
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
