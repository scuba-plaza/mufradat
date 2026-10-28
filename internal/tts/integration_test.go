//go:build integration

package tts_test

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/itest"
	"github.com/scuba-plaza/mufradat/internal/tts"
)

type bearer struct{ token string }

func (b bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func TestGoogleVoicesListsTheArabicVoicesAndTheDefaultExists(t *testing.T) {
	client := itest.Google(t)
	voices, err := client.Voices(itest.Context(t))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range voices {
		found = found || v.Name == config.DefaultGoogleVoice
	}
	if !found {
		t.Fatalf("the default voice %q is not among %d Arabic voices", config.DefaultGoogleVoice, len(voices))
	}
}

func TestGoogleSynthesizeReturnsMP3(t *testing.T) {
	client := itest.Google(t)
	audio, err := client.Synthesize(itest.Context(t), tts.Request{Text: "قَلَم", Voice: config.DefaultGoogleVoice, Rate: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !(bytes.HasPrefix(audio, []byte("ID3")) || (len(audio) > 2 && audio[0] == 0xFF && audio[1]&0xE0 == 0xE0)) {
		t.Fatalf("%d bytes that do not start like an MP3: % x", len(audio), audio[:min(8, len(audio))])
	}
}

func TestGoogleRejectsABadTokenAsAnAuthError(t *testing.T) {
	client := &tts.Client{HTTP: &http.Client{Transport: bearer{token: "not-a-real-token"}}}
	_, err := client.Voices(itest.Context(t))
	var api *tts.APIError
	if !errors.As(err, &api) || api.Kind() != tts.KindAuth {
		t.Fatalf("got %v, want an authentication error", err)
	}
}
