package config

import (
	"fmt"
	"net/url"
	"slices"
)

const SchemaVersion = 1

const (
	VerifyAuto = "auto"
	VerifyOn   = "on"
	VerifyOff  = "off"
)

const (
	DefaultVoice         = "ar-XA-Chirp3-HD-Achernar"
	DefaultRate          = 0.9
	DefaultProductionTop = 1000
	DefaultDeckName      = "Arabic MSA Core"
)

type Config struct {
	Schema    int    `toml:"schema"`
	Workspace string `toml:"workspace"`
	DeckName  string `toml:"deck_name"`
	Anki      Anki   `toml:"anki"`
	Audio     Audio  `toml:"audio"`
	TTS       TTS    `toml:"tts"`
	Author    Author `toml:"author"`
	Verify    Verify `toml:"verify"`
	Google    Google `toml:"google"`
}

type Anki struct {
	DeckID        int64 `toml:"deck_id"`
	ModelID       int64 `toml:"model_id"`
	ProductionTop int   `toml:"production_top"`
}

type Audio struct {
	Voice string  `toml:"voice"`
	Rate  float64 `toml:"rate"`
}

type TTS struct {
	QuotaProject      string `toml:"quota_project"`
	Endpoint          string `toml:"endpoint"`
	RequestsPerMinute int    `toml:"requests_per_minute"`
	Concurrency       int    `toml:"concurrency"`
}

type Author struct {
	Backend     string `toml:"backend"`
	Model       string `toml:"model"`
	Effort      string `toml:"effort"`
	BatchSize   int    `toml:"batch_size"`
	Concurrency int    `toml:"concurrency"`
	AllowRank   int    `toml:"allow_rank"`
}

type Verify struct {
	Enabled string `toml:"enabled"`
	Python  string `toml:"python"`
}

type Google struct {
	CredentialsFile string `toml:"credentials_file"`
}

func Default() Config {
	return Config{
		Schema:   SchemaVersion,
		DeckName: DefaultDeckName,
		Anki:     Anki{ProductionTop: DefaultProductionTop},
		Audio:    Audio{Voice: DefaultVoice, Rate: DefaultRate},
		TTS:      TTS{RequestsPerMinute: 300, Concurrency: 4},
		Author: Author{
			Backend:     "claude-code",
			Model:       "sonnet",
			Effort:      "medium",
			BatchSize:   10,
			Concurrency: 2,
			AllowRank:   1500,
		},
		Verify: Verify{Enabled: VerifyAuto},
	}
}

var efforts = []string{"low", "medium", "high", "xhigh", "max"}

func (c Config) Validate() error {
	if c.Schema != SchemaVersion {
		return fmt.Errorf("schema %d is not supported (this version reads schema %d)", c.Schema, SchemaVersion)
	}
	if c.DeckName == "" {
		return fmt.Errorf("deck_name must not be empty")
	}
	if c.Anki.ProductionTop < 0 {
		return fmt.Errorf("anki.production_top must not be negative")
	}
	if c.Audio.Voice == "" {
		return fmt.Errorf("audio.voice must not be empty")
	}
	if c.Audio.Rate < 0.25 || c.Audio.Rate > 2 {
		return fmt.Errorf("audio.rate %v is outside 0.25 to 2", c.Audio.Rate)
	}
	if c.TTS.Endpoint != "" {
		u, err := url.Parse(c.TTS.Endpoint)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("tts.endpoint %q must be an http(s) URL such as https://eu-texttospeech.googleapis.com", c.TTS.Endpoint)
		}
	}
	if c.TTS.RequestsPerMinute < 1 {
		return fmt.Errorf("tts.requests_per_minute must be at least 1")
	}
	if c.TTS.Concurrency < 1 {
		return fmt.Errorf("tts.concurrency must be at least 1")
	}
	if c.Author.Backend != "claude-code" {
		return fmt.Errorf("author.backend %q is not supported (use claude-code)", c.Author.Backend)
	}
	if c.Author.Effort != "" && !slices.Contains(efforts, c.Author.Effort) {
		return fmt.Errorf("author.effort %q must be one of %v", c.Author.Effort, efforts)
	}
	if c.Author.BatchSize < 1 {
		return fmt.Errorf("author.batch_size must be at least 1")
	}
	if c.Author.Concurrency < 1 {
		return fmt.Errorf("author.concurrency must be at least 1")
	}
	if c.Author.AllowRank < 0 {
		return fmt.Errorf("author.allow_rank must not be negative")
	}
	switch c.Verify.Enabled {
	case VerifyAuto, VerifyOn, VerifyOff:
	default:
		return fmt.Errorf("verify.enabled %q must be auto, on or off", c.Verify.Enabled)
	}
	return nil
}
