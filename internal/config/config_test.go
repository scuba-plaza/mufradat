package config

import (
	"os"
	"path/filepath"
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

func TestLoadLayersWorkspaceOverUserOverDefaults(t *testing.T) {
	dir := t.TempDir()
	user := write(t, dir, "user.toml", "[author]\nmodel = \"haiku\"\neffort = \"high\"\n[audio]\nrate = 1.1\n")
	ws := write(t, dir, "ws.toml", "[author]\nmodel = \"opus\"\n")
	cfg, err := Load(user, ws)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Author.Model != "opus" {
		t.Errorf("workspace should win, got model %q", cfg.Author.Model)
	}
	if cfg.Author.Effort != "high" {
		t.Errorf("user value should survive when the workspace is silent, got %q", cfg.Author.Effort)
	}
	if cfg.Audio.Rate != 1.1 {
		t.Errorf("user rate = %v", cfg.Audio.Rate)
	}
	if cfg.Author.BatchSize != Default().Author.BatchSize {
		t.Errorf("untouched default lost: %d", cfg.Author.BatchSize)
	}
}

func TestLoadIgnoresMissingFiles(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Audio.Voice != DefaultVoice {
		t.Errorf("voice = %q", cfg.Audio.Voice)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	path := write(t, t.TempDir(), "c.toml", "[audio]\nvoise = \"x\"\n")
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "voise") {
		t.Fatalf("expected an error naming the unknown key, got %v", err)
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]func(*Config){
		"schema":      func(c *Config) { c.Schema = 99 },
		"rate":        func(c *Config) { c.Audio.Rate = 9 },
		"effort":      func(c *Config) { c.Author.Effort = "turbo" },
		"verify":      func(c *Config) { c.Verify.Enabled = "maybe" },
		"batch":       func(c *Config) { c.Author.BatchSize = 0 },
		"backend":     func(c *Config) { c.Author.Backend = "gpt" },
		"concurrency": func(c *Config) { c.TTS.Concurrency = 0 },
	}
	for name, mutate := range cases {
		cfg := Default()
		mutate(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
	if err := Default().Validate(); err != nil {
		t.Fatalf("defaults must validate: %v", err)
	}
}

func TestSetFileWritesTypedValuesAndKeepsOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ws.toml")
	if err := SetFile(path, "author.batch_size", "4"); err != nil {
		t.Fatal(err)
	}
	if err := SetFile(path, "audio.rate", "1.25"); err != nil {
		t.Fatal(err)
	}
	if err := SetFile(path, "audio.voice", "ar-XA-Standard-A"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Author.BatchSize != 4 || cfg.Audio.Rate != 1.25 || cfg.Audio.Voice != "ar-XA-Standard-A" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestSetFileRejectsBadInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ws.toml")
	for key, value := range map[string]string{
		"author.batch_size": "many",
		"audio.rate":        "99",
		"no.such.key":       "1",
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
	for _, want := range []string{"deck_name", "anki.deck_id", "audio.voice", "tts.quota_project", "author.model", "verify.enabled", "google.credentials_file", "workspace"} {
		found := false
		for _, k := range keys {
			if k == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing key %s in %v", want, keys)
		}
	}
}
