package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://texttospeech.googleapis.com"
	LanguageCode   = "ar-XA"
	maxTextBytes   = 4500
	maxBodyBytes   = 32 << 20
)

type Waiter interface {
	Wait(ctx context.Context) error
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
	Limit   Waiter
}

type Request struct {
	Text  string
	Voice string
	Rate  float64
}

type VoiceInfo struct {
	Name       string
	Gender     string
	SampleRate int
}

type synthesizeRequest struct {
	Input       synthesizeInput `json:"input"`
	Voice       synthesizeVoice `json:"voice"`
	AudioConfig audioConfig     `json:"audioConfig"`
}

type synthesizeInput struct {
	Text string `json:"text"`
}

type synthesizeVoice struct {
	LanguageCode string `json:"languageCode"`
	Name         string `json:"name"`
}

type audioConfig struct {
	AudioEncoding string  `json:"audioEncoding"`
	SpeakingRate  float64 `json:"speakingRate"`
}

type synthesizeResponse struct {
	AudioContent string `json:"audioContent"`
}

type voicesResponse struct {
	Voices []struct {
		Name                   string   `json:"name"`
		SsmlGender             string   `json:"ssmlGender"`
		NaturalSampleRateHertz int      `json:"naturalSampleRateHertz"`
		LanguageCodes          []string `json:"languageCodes"`
	} `json:"voices"`
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return DefaultBaseURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *Client) Synthesize(ctx context.Context, req Request) ([]byte, error) {
	if strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("cannot synthesize empty text")
	}
	if len(req.Text) > maxTextBytes {
		return nil, fmt.Errorf("text is %d bytes; the service accepts at most %d", len(req.Text), maxTextBytes)
	}
	body, err := json.Marshal(synthesizeRequest{
		Input:       synthesizeInput{Text: req.Text},
		Voice:       synthesizeVoice{LanguageCode: LanguageCode, Name: req.Voice},
		AudioConfig: audioConfig{AudioEncoding: "MP3", SpeakingRate: req.Rate},
	})
	if err != nil {
		return nil, err
	}
	raw, err := c.do(ctx, http.MethodPost, "/v1/text:synthesize", body)
	if err != nil {
		return nil, err
	}
	var resp synthesizeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unreadable synthesize response: %w", err)
	}
	audio, err := base64.StdEncoding.DecodeString(resp.AudioContent)
	if err != nil {
		return nil, fmt.Errorf("synthesize response has invalid audio: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("synthesize response has no audio")
	}
	return audio, nil
}

func (c *Client) Voices(ctx context.Context) ([]VoiceInfo, error) {
	raw, err := c.do(ctx, http.MethodGet, "/v1/voices?languageCode="+url.QueryEscape(LanguageCode), nil)
	if err != nil {
		return nil, err
	}
	var resp voicesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unreadable voices response: %w", err)
	}
	out := make([]VoiceInfo, 0, len(resp.Voices))
	for _, v := range resp.Voices {
		out = append(out, VoiceInfo{Name: v.Name, Gender: strings.ToLower(v.SsmlGender), SampleRate: v.NaturalSampleRateHertz})
	}
	return out, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if c.Limit != nil {
		if err := c.Limit.Wait(ctx); err != nil {
			return nil, err
		}
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, newAPIError(resp.StatusCode, raw, retryAfter(resp.Header, time.Now()))
	}
	return raw, nil
}
