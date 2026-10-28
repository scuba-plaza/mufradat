package config

import (
	"os"
	"path/filepath"
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

func TestDefaultsChooseMistralForNotesAndSpeech(t *testing.T) {
	cfg := Default()
	if cfg.Audio.Provider != "mistral" {
		t.Errorf("audio.provider = %q, want mistral", cfg.Audio.Provider)
	}
	if cfg.Author.Model != "mistral-medium-latest" {
		t.Errorf("author.model = %q", cfg.Author.Model)
	}
	if cfg.Mistral.Voice != "voice_479" {
		t.Errorf("mistral.voice = %q", cfg.Mistral.Voice)
	}
	if cfg.Google.Voice != "ar-XA-Chirp3-HD-Achernar" {
		t.Errorf("google.voice = %q", cfg.Google.Voice)
	}
	if cfg.TTS.RequestsPerMinute != 300 || cfg.TTS.Concurrency != 4 {
		t.Errorf("tts = %+v", cfg.TTS)
	}
	if cfg.Audio.Rate != 0.9 {
		t.Errorf("audio.rate = %v", cfg.Audio.Rate)
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
		{"mistral endpoint", func(c *Config) { c.Mistral.Endpoint = "ftp://x" }, `mistral.endpoint "ftp://x"`},
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
	for key, value := range map[string]string{
		"author.batch_size": "many",
		"audio.rate":        "99",
		"audio.provider":    "azure",
		"no.such.key":       "1",
		"audio.voice":       "x",
		"author.effort":     "high",
	} {
		if err := SetFile(path, key, value); err == nil {
			t.Errorf("%s=%s should fail", key, value)
		}
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a rejected set must not create the file")
	}
}

func TestKeysCoverEveryField(t *testing.T) {
	keys := Keys()
	want := []string{
		"deck_name", "anki.deck_id", "workspace",
		"audio.provider", "audio.rate",
		"tts.requests_per_minute", "tts.concurrency",
		"google.credentials_file", "google.quota_project", "google.endpoint", "google.voice",
		"mistral.api_key", "mistral.endpoint", "mistral.voice",
		"author.model", "author.batch_size", "author.concurrency", "author.allow_rank",
		"verify.enabled",
	}
	for _, k := range want {
		if !slices.Contains(keys, k) {
			t.Errorf("missing key %s in %v", k, keys)
		}
	}
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
