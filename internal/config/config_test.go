package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultSettingsAreExactlyTheDocumentedOnes(t *testing.T) {
	flat, err := Flatten(Default())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"schema": int64(1), "workspace": "", "deck_name": "Arabic MSA Core",
		"anki.deck_id": int64(0), "anki.model_id": int64(0), "anki.production_top": int64(1000),
		"audio.provider": "mistral", "audio.rate": 0.9,
		"tts.requests_per_minute": int64(300), "tts.concurrency": int64(4),
		"google.credentials_file": "", "google.quota_project": "", "google.endpoint": "", "google.voice": "ar-XA-Chirp3-HD-Achernar",
		"mistral.api_key": "", "mistral.endpoint": "", "mistral.voice": "voice_479",
		"author.model": "mistral-medium-latest", "author.batch_size": int64(10), "author.concurrency": int64(2), "author.allow_rank": int64(1500),
		"verify.enabled": "auto", "verify.python": "",
	}
	if !reflect.DeepEqual(flat, want) {
		for k, v := range want {
			if flat[k] != v {
				t.Errorf("%s = %#v, want %#v", k, flat[k], v)
			}
		}
		for k := range flat {
			if _, ok := want[k]; !ok {
				t.Errorf("unexpected setting %s", k)
			}
		}
	}
}

func TestLoadLayersWorkspaceOverUserOverDefaults(t *testing.T) {
	dir := t.TempDir()
	user := write(t, dir, "user.toml", "[author]\nmodel = \"mistral-small-latest\"\nbatch_size = 7\n[audio]\nrate = 1.1\n[mistral]\nvoice = \"maryam\"\n")
	ws := write(t, dir, "ws.toml", "[author]\nmodel = \"mistral-large-latest\"\n[google]\nvoice = \"ar-XA-Wavenet-A\"\n")
	cfg, err := Load(user, ws)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Author.Model != "mistral-large-latest" {
		t.Errorf("workspace should win, got model %q", cfg.Author.Model)
	}
	if cfg.Author.BatchSize != 7 {
		t.Errorf("user value should survive when the workspace is silent, got %d", cfg.Author.BatchSize)
	}
	if cfg.Audio.Rate != 1.1 || cfg.Mistral.Voice != "maryam" || cfg.Google.Voice != "ar-XA-Wavenet-A" {
		t.Errorf("unexpected layering: %+v", cfg)
	}
	if cfg.Author.Concurrency != Default().Author.Concurrency {
		t.Errorf("untouched default lost: %d", cfg.Author.Concurrency)
	}
}

func TestLoadIgnoresMissingFiles(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Audio.Provider != "mistral" {
		t.Errorf("provider = %q", cfg.Audio.Provider)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	path := write(t, t.TempDir(), "c.toml", "[mistral]\nvoise = \"x\"\n")
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "voise") {
		t.Fatalf("expected an error naming the unknown key, got %v", err)
	}
}

func TestLoadRejectsKeysThatNoLongerExist(t *testing.T) {
	cases := map[string]string{
		"author.backend":    "[author]\nbackend = \"x\"\n",
		"author.effort":     "[author]\neffort = \"high\"\n",
		"audio.voice":       "[audio]\nvoice = \"x\"\n",
		"tts.endpoint":      "[tts]\nendpoint = \"https://example.com\"\n",
		"tts.quota_project": "[tts]\nquota_project = \"p\"\n",
	}
	for key, content := range cases {
		path := write(t, t.TempDir(), "c.toml", content)
		_, err := Load(path)
		if err == nil {
			t.Errorf("%s: expected the key to be rejected", key)
			continue
		}
		name := key[strings.Index(key, ".")+1:]
		if !strings.Contains(err.Error(), name) {
			t.Errorf("%s: error should name the key, got %v", key, err)
		}
	}
}

func TestValidateNamesTheOffendingSetting(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"schema", func(c *Config) { c.Schema = 99 }, "schema 99"},
		{"provider", func(c *Config) { c.Audio.Provider = "elevenlabs" }, `audio.provider "elevenlabs" must be mistral or google`},
		{"empty provider", func(c *Config) { c.Audio.Provider = "" }, "audio.provider"},
		{"rate", func(c *Config) { c.Audio.Rate = 9 }, "audio.rate 9 is outside 0.25 to 2"},
		{"verify", func(c *Config) { c.Verify.Enabled = "maybe" }, `verify.enabled "maybe"`},
		{"batch", func(c *Config) { c.Author.BatchSize = 0 }, "author.batch_size"},
		{"model", func(c *Config) { c.Author.Model = "" }, "author.model must not be empty"},
		{"tts concurrency", func(c *Config) { c.TTS.Concurrency = 0 }, "tts.concurrency"},
		{"tts rpm", func(c *Config) { c.TTS.RequestsPerMinute = 0 }, "tts.requests_per_minute"},
		{"google voice", func(c *Config) { c.Google.Voice = "" }, "google.voice must not be empty"},
		{"mistral voice", func(c *Config) { c.Mistral.Voice = "" }, "mistral.voice must not be empty"},
		{"google endpoint", func(c *Config) { c.Google.Endpoint = "not-a-url" }, `google.endpoint "not-a-url"`},
		{"mistral endpoint", func(c *Config) { c.Mistral.Endpoint = "ftp://x" }, `mistral.endpoint "ftp://x" must be an http(s) URL such as https://api.mistral.ai`},
		{"google endpoint example", func(c *Config) { c.Google.Endpoint = "not-a-url" }, "such as https://eu-texttospeech.googleapis.com"},
		{"endpoint without a host", func(c *Config) { c.Google.Endpoint = "https://" }, `google.endpoint "https://"`},
		{"endpoint that does not parse", func(c *Config) { c.Mistral.Endpoint = "http://[bad" }, `mistral.endpoint "http://[bad"`},
		{"deck name", func(c *Config) { c.DeckName = "" }, "deck_name must not be empty"},
		{"production top", func(c *Config) { c.Anki.ProductionTop = -1 }, "anki.production_top must not be negative"},
		{"author concurrency", func(c *Config) { c.Author.Concurrency = 0 }, "author.concurrency must be at least 1"},
		{"allow rank", func(c *Config) { c.Author.AllowRank = -1 }, "author.allow_rank must not be negative"},
		{"rate just below", func(c *Config) { c.Audio.Rate = 0.24 }, "audio.rate 0.24 is outside"},
		{"rate just above", func(c *Config) { c.Audio.Rate = 2.01 }, "audio.rate 2.01 is outside"},
	}
	for _, tc := range cases {
		cfg := Default()
		tc.mutate(&cfg)
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want an error containing %q", tc.name, err, tc.want)
		}
	}
	if err := Default().Validate(); err != nil {
		t.Fatalf("defaults must validate: %v", err)
	}
	cfg := Default()
	cfg.Audio.Provider = "google"
	cfg.Mistral.Endpoint = "http://127.0.0.1:9"
	cfg.Google.Endpoint = "https://eu-texttospeech.googleapis.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("google provider with endpoints must validate: %v", err)
	}
}

func TestValidateAcceptsTheEdgesOfEverySetting(t *testing.T) {
	cases := map[string]func(*Config){
		"production top 0":      func(c *Config) { c.Anki.ProductionTop = 0 },
		"rate 0.25":             func(c *Config) { c.Audio.Rate = 0.25 },
		"rate 2":                func(c *Config) { c.Audio.Rate = 2 },
		"requests per minute 1": func(c *Config) { c.TTS.RequestsPerMinute = 1 },
		"tts concurrency 1":     func(c *Config) { c.TTS.Concurrency = 1 },
		"batch size 1":          func(c *Config) { c.Author.BatchSize = 1 },
		"author concurrency 1":  func(c *Config) { c.Author.Concurrency = 1 },
		"allow rank 0":          func(c *Config) { c.Author.AllowRank = 0 },
		"verify on":             func(c *Config) { c.Verify.Enabled = "on" },
		"verify off":            func(c *Config) { c.Verify.Enabled = "off" },
		"google provider":       func(c *Config) { c.Audio.Provider = "google" },
		"plain http endpoint":   func(c *Config) { c.Mistral.Endpoint = "http://127.0.0.1:8080" },
	}
	for name, mutate := range cases {
		cfg := Default()
		mutate(&cfg)
		if err := cfg.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestLoadReportsTheFileThatCannotBeUsed(t *testing.T) {
	dir := t.TempDir()
	bad := write(t, dir, "bad.toml", "[mistral]\nvoise = \"x\"\n")
	if _, err := Load(bad); err == nil || !strings.Contains(err.Error(), bad) {
		t.Errorf("an unknown key should name the file, got %v", err)
	}
	invalid := write(t, dir, "invalid.toml", "[audio]\nrate = 9\n")
	if _, err := Load(invalid); err == nil || !strings.HasPrefix(err.Error(), "invalid configuration: audio.rate 9") {
		t.Errorf("an invalid value should be reported as an invalid configuration, got %v", err)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), dir) {
		t.Errorf("a directory where a file should be is an error that names it, got %v", err)
	}
}

func TestGetNamesTheKnownKeysWhenAskedForAnUnknownOne(t *testing.T) {
	v, err := Get(Default(), "audio.provider")
	if err != nil || v != "mistral" {
		t.Fatalf("audio.provider = %v, %v", v, err)
	}
	if _, err := Get(Default(), "audio.voice"); err == nil || !strings.Contains(err.Error(), `unknown key "audio.voice"`) || !strings.Contains(err.Error(), "audio.provider, audio.rate") {
		t.Errorf("got %v", err)
	}
}

func TestSetFileWritesTypedValuesAndKeepsOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ws.toml")
	for key, value := range map[string]string{
		"author.batch_size": "4",
		"audio.rate":        "1.25",
		"audio.provider":    "google",
		"google.voice":      "ar-XA-Standard-A",
		"mistral.voice":     "maryam",
	} {
		if err := SetFile(path, key, value); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Author.BatchSize != 4 || cfg.Audio.Rate != 1.25 || cfg.Audio.Provider != "google" ||
		cfg.Google.Voice != "ar-XA-Standard-A" || cfg.Mistral.Voice != "maryam" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestSetFileRejectsBadInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ws.toml")
	cases := []struct{ key, value, want string }{
		{"author.batch_size", "many", `author.batch_size: "many" is not an integer`},
		{"audio.rate", "fast", `audio.rate: "fast" is not a number`},
		{"audio.rate", "99", "audio.rate: audio.rate 99 is outside 0.25 to 2"},
		{"audio.provider", "azure", `audio.provider: audio.provider "azure" must be mistral or google`},
		{"no.such.key", "1", `unknown key "no.such.key"`},
		{"audio.voice", "x", `unknown key "audio.voice"`},
		{"author.effort", "high", `unknown key "author.effort"`},
	}
	for _, tc := range cases {
		err := SetFile(path, tc.key, tc.value)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s=%s: got %v, want an error containing %q", tc.key, tc.value, err, tc.want)
		}
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a rejected set must not create the file")
	}
}

func TestRetiredKeysAreNotSettings(t *testing.T) {
	keys := Keys()
	for _, gone := range []string{"audio.voice", "tts.endpoint", "tts.quota_project", "author.backend", "author.effort"} {
		if slices.Contains(keys, gone) {
			t.Errorf("key %s should not exist any more", gone)
		}
	}
}

func TestSecretKeysAreMasked(t *testing.T) {
	if !IsSecret("mistral.api_key") {
		t.Error("mistral.api_key holds the key itself and must be treated as secret")
	}
	for _, k := range []string{"google.credentials_file", "mistral.voice", "mistral.endpoint"} {
		if IsSecret(k) {
			t.Errorf("%s is not a secret", k)
		}
	}
	if got := Mask("sk-123"); got != "(set)" {
		t.Errorf("Mask of a key = %q", got)
	}
	if got := Mask(""); got != "" {
		t.Errorf("Mask of an empty value = %q", got)
	}
}
