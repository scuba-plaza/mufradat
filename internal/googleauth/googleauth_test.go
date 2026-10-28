package googleauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func serviceAccountJSON(t *testing.T, tokenURL string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustPKCS8(t, key)})
	raw, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "proj-1",
		"private_key_id": "k1",
		"private_key":    string(pemKey),
		"client_email":   "tts@proj-1.iam.gserviceaccount.com",
		"token_uri":      tokenURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func write(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveOrderAndRequired(t *testing.T) {
	dir := t.TempDir()
	envFile := write(t, dir, "env.json", []byte(`{"type":"service_account"}`))
	cfgFile := write(t, dir, "cfg.json", []byte(`{"type":"authorized_user"}`))
	missing := filepath.Join(dir, "missing.json")

	got, err := Resolve([]Candidate{
		{Source: "env", Path: envFile, Required: true},
		{Source: "config", Path: cfgFile, Required: true},
	})
	if err != nil || got.Source != "env" {
		t.Fatalf("got %+v, %v", got, err)
	}

	got, err = Resolve([]Candidate{{Source: "env"}, {Source: "config", Path: cfgFile, Required: true}})
	if err != nil || got.Source != "config" {
		t.Fatalf("empty path must be skipped: %+v, %v", got, err)
	}

	got, err = Resolve([]Candidate{{Source: "imported", Path: missing}, {Source: "adc", Path: cfgFile}})
	if err != nil || got.Source != "adc" {
		t.Fatalf("optional missing must fall through: %+v, %v", got, err)
	}

	_, err = Resolve([]Candidate{{Source: "config", Path: missing, Required: true}, {Source: "adc", Path: cfgFile}})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("required missing must fail loudly: %v", err)
	}

	_, err = Resolve([]Candidate{{Source: "imported", Path: missing}})
	if err != ErrNone {
		t.Fatalf("want ErrNone, got %v", err)
	}
}

func TestValidateAndImport(t *testing.T) {
	dir := t.TempDir()
	raw := serviceAccountJSON(t, "http://127.0.0.1:1/token")
	info, err := Validate(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != "service_account" || info.Account != "tts@proj-1.iam.gserviceaccount.com" || info.ProjectID != "proj-1" {
		t.Fatalf("info %+v", info)
	}

	for name, bad := range map[string]string{"not json": "hello", "no type": `{"client_email":"x"}`, "unknown type": `{"type":"banana"}`, "external account": `{"type":"external_account","audience":"x","subject_token_type":"y","token_url":"http://127.0.0.1:1/t","credential_source":{"executable":{"command":"touch /tmp/x"}}}`} {
		if _, err := Validate(context.Background(), []byte(bad)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}

	src := write(t, dir, "key.json", raw)
	dest := filepath.Join(dir, "config", "google-credentials.json")
	imported, err := Import(context.Background(), src, dest)
	if err != nil || imported != info {
		t.Fatalf("Import returned %+v, %v; want the key's details %+v", imported, err, info)
	}
	if saved, _ := os.ReadFile(dest); !bytes.Equal(saved, raw) {
		t.Fatal("the imported file is not a copy of the key")
	}
	st, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
	if _, err := Import(context.Background(), filepath.Join(dir, "nope.json"), dest); err == nil {
		t.Fatal("missing source accepted")
	}
	bad := write(t, dir, "bad.json", []byte(`{"type":"banana"}`))
	before, _ := os.ReadFile(dest)
	if _, err := Import(context.Background(), bad, dest); err == nil {
		t.Fatal("invalid key accepted")
	}
	after, _ := os.ReadFile(dest)
	if string(before) != string(after) {
		t.Fatal("a failed import overwrote the existing key")
	}
	blocker := write(t, dir, "blocker", []byte("a file where a folder is needed"))
	if _, err := Import(context.Background(), src, filepath.Join(blocker, "google-credentials.json")); err == nil {
		t.Fatal("an import that cannot be saved reported success")
	}
}

func TestTransportSignsRequestsAndSetsQuotaProject(t *testing.T) {
	var tokenCalls int
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		io.Copy(io.Discard, r.Body)
		json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-123", "token_type": "Bearer", "expires_in": 3600})
	}))
	defer token.Close()

	var gotAuth, gotProject string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotProject = r.Header.Get("x-goog-user-project")
		io.WriteString(w, "ok")
	}))
	defer api.Close()

	creds := Credentials{JSON: serviceAccountJSON(t, token.URL), Source: "test"}
	rt, err := Transport(context.Background(), http.DefaultTransport, creds, "billing-proj")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: rt}
	for range 2 {
		resp, err := client.Get(api.URL)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if gotAuth != "Bearer tok-123" {
		t.Fatalf("authorization %q", gotAuth)
	}
	if gotProject != "billing-proj" {
		t.Fatalf("quota project %q", gotProject)
	}
	if tokenCalls != 1 {
		t.Fatalf("token fetched %d times, want it cached", tokenCalls)
	}

	rt, err = Transport(context.Background(), http.DefaultTransport, creds, "")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Transport: rt}).Get(api.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotProject != "" {
		t.Fatalf("quota header sent without configuration: %q", gotProject)
	}
}
