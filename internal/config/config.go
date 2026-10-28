package config

import (
	"fmt"
	"net/url"
)

const SchemaVersion = 1

const (
	VerifyAuto = "auto"
	VerifyOn   = "on"
	VerifyOff  = "off"
)

const (
	ProviderMistral = "mistral"
	ProviderGoogle  = "google"
)

const (
	DefaultGoogleVoice   = "ar-XA-Chirp3-HD-Achernar"
	DefaultMistralVoice  = "voice_479"
	DefaultAuthorModel   = "mistral-medium-latest"
	DefaultRate          = 0.9
	DefaultProductionTop = 1000
	DefaultDeckName      = "Arabic MSA Core"
)

type Config struct {
	Schema    int     `toml:"schema"`
	Workspace string  `toml:"workspace"`
	DeckName  string  `toml:"deck_name"`
	Anki      Anki    `toml:"anki"`
	Audio     Audio   `toml:"audio"`
	TTS       TTS     `toml:"tts"`
	Author    Author  `toml:"author"`
	Verify    Verify  `toml:"verify"`
	Google    Google  `toml:"google"`
	Mistral   Mistral `toml:"mistral"`
}

type Anki struct {
	DeckID        int64 `toml:"deck_id"`
	ModelID       int64 `toml:"model_id"`
	ProductionTop int   `toml:"production_top"`
}

type Audio struct {
	Provider string  `toml:"provider"`
	Rate     float64 `toml:"rate"`
}

type TTS struct {
	RequestsPerMinute int `toml:"requests_per_minute"`
	Concurrency       int `toml:"concurrency"`
}

type Author struct {
	Model       string `toml:"model"`
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
	QuotaProject    string `toml:"quota_project"`
	Endpoint        string `toml:"endpoint"`
	Voice           string `toml:"voice"`
}

type Mistral struct {
	APIKey   string `toml:"api_key"`
	Endpoint string `toml:"endpoint"`
	Voice    string `toml:"voice"`
}

func Default() Config {
	return Config{
		Schema:   SchemaVersion,
		DeckName: DefaultDeckName,
		Anki:     Anki{ProductionTop: DefaultProductionTop},
		Audio:    Audio{Provider: ProviderMistral, Rate: DefaultRate},
		TTS:      TTS{RequestsPerMinute: 300, Concurrency: 4},
		Google:   Google{Voice: DefaultGoogleVoice},
		Mistral:  Mistral{Voice: DefaultMistralVoice},
		Author: Author{
			Model:       DefaultAuthorModel,
			BatchSize:   10,
			Concurrency: 2,
			AllowRank:   1500,
		},
		Verify: Verify{Enabled: VerifyAuto},
	}
}

func IsSecret(key string) bool { return key == "mistral.api_key" }

func Mask(value string) string {
	if value == "" {
		return ""
	}
	return "(set)"
}

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
	switch c.Audio.Provider {
	case ProviderMistral, ProviderGoogle:
	default:
		return fmt.Errorf("audio.provider %q must be %s or %s", c.Audio.Provider, ProviderMistral, ProviderGoogle)
	}
	if c.Audio.Rate < 0.25 || c.Audio.Rate > 2 {
		return fmt.Errorf("audio.rate %v is outside 0.25 to 2", c.Audio.Rate)
	}
	if c.Google.Voice == "" {
		return fmt.Errorf("google.voice must not be empty")
	}
	if c.Mistral.Voice == "" {
		return fmt.Errorf("mistral.voice must not be empty")
	}
	if err := validateEndpoint("google.endpoint", c.Google.Endpoint, "https://eu-texttospeech.googleapis.com"); err != nil {
		return err
	}
	if err := validateEndpoint("mistral.endpoint", c.Mistral.Endpoint, "https://api.mistral.ai"); err != nil {
		return err
	}
	if c.TTS.RequestsPerMinute < 1 {
		return fmt.Errorf("tts.requests_per_minute must be at least 1")
	}
	if c.TTS.Concurrency < 1 {
		return fmt.Errorf("tts.concurrency must be at least 1")
	}
	if c.Author.Model == "" {
		return fmt.Errorf("author.model must not be empty")
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

func validateEndpoint(key, value, example string) error {
	if value == "" {
		return nil
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("%s %q must be an http(s) URL such as %s", key, value, example)
	}
	return nil
}
