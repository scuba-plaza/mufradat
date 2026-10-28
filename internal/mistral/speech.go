package mistral

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	SpeechModel = "voxtral-mini-tts-3"
	voicePage   = "100"
	voiceLang   = "ar"
)

type Voice struct {
	ID        string
	Name      string
	Slug      string
	Languages []string
	Gender    string
}

type SpeechRequest struct {
	Text  string
	Voice string
}

type VoiceError struct {
	Voice  string
	Reason string
}

func (e *VoiceError) Error() string {
	return fmt.Sprintf("Mistral has no Arabic voice %q%s", e.Voice, e.Reason)
}

func (e *VoiceError) Fatal() bool { return true }

type speechRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	VoiceID        string `json:"voice_id"`
	ResponseFormat string `json:"response_format"`
	Stream         bool   `json:"stream"`
}

type speechResponse struct {
	AudioData string `json:"audio_data"`
}

type voicesResponse struct {
	Data []struct {
		ID        string   `json:"id"`
		Name      string   `json:"name"`
		Slug      *string  `json:"slug"`
		Languages []string `json:"languages"`
		Gender    string   `json:"gender"`
	} `json:"data"`
	NextPageToken *string `json:"next_page_token"`
}

func (c *Client) Voices(ctx context.Context) ([]Voice, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.voices != nil {
		return c.voices, nil
	}
	voices := []Voice{}
	token := ""
	for {
		query := url.Values{"page_size": {voicePage}, "language": {voiceLang}}
		if token != "" {
			query.Set("page_token", token)
		}
		var page voicesResponse
		if err := c.do(ctx, http.MethodGet, PathVoices, query, nil, &page); err != nil {
			return nil, err
		}
		for _, v := range page.Data {
			if !speaksArabic(v.Languages) {
				continue
			}
			voice := Voice{ID: v.ID, Name: v.Name, Languages: v.Languages, Gender: strings.ToLower(v.Gender)}
			if v.Slug != nil {
				voice.Slug = *v.Slug
			}
			voices = append(voices, voice)
		}
		if page.NextPageToken == nil || *page.NextPageToken == "" {
			break
		}
		token = *page.NextPageToken
	}
	c.voices = voices
	return voices, nil
}

func speaksArabic(languages []string) bool {
	for _, l := range languages {
		l = strings.ToLower(l)
		if l == voiceLang || strings.HasPrefix(l, voiceLang+"_") || strings.HasPrefix(l, voiceLang+"-") {
			return true
		}
	}
	return false
}

func (c *Client) resolve(ctx context.Context, ref string) (Voice, error) {
	voices, err := c.Voices(ctx)
	if err != nil {
		return Voice{}, err
	}
	for _, v := range voices {
		if v.ID == ref || (v.Slug != "" && v.Slug == ref) {
			return v, nil
		}
	}
	var named []Voice
	for _, v := range voices {
		if strings.EqualFold(v.Name, ref) {
			named = append(named, v)
		}
	}
	switch len(named) {
	case 1:
		return named[0], nil
	case 0:
		return Voice{}, &VoiceError{Voice: ref}
	}
	return Voice{}, &VoiceError{Voice: ref, Reason: fmt.Sprintf(": the name matches %d voices, so use a slug or id", len(named))}
}

func (c *Client) Synthesize(ctx context.Context, req SpeechRequest) ([]byte, error) {
	if strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("cannot synthesize empty text")
	}
	voice, err := c.resolve(ctx, req.Voice)
	if err != nil {
		return nil, err
	}
	var reply speechResponse
	body := speechRequest{Model: SpeechModel, Input: req.Text, VoiceID: voice.ID, ResponseFormat: "mp3"}
	if err := c.do(ctx, http.MethodPost, PathSpeech, nil, body, &reply); err != nil {
		return nil, err
	}
	audio, err := base64.StdEncoding.DecodeString(reply.AudioData)
	if err != nil {
		return nil, fmt.Errorf("the speech reply has invalid audio: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("the speech reply has no audio")
	}
	return audio, nil
}
