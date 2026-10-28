//go:build integration

package itest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/googleauth"
	"github.com/scuba-plaza/mufradat/internal/mistral"
	"github.com/scuba-plaza/mufradat/internal/tts"
)

const (
	MistralEnv = "MISTRAL_API_KEY"
	GoogleEnv  = "GOOGLE_APPLICATION_CREDENTIALS"
	timeout    = 2 * time.Minute
)

func Context(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}

func MistralKey(t *testing.T) string {
	t.Helper()
	key, err := mistral.ParseKey(MistralEnv, env.OS().Getenv(MistralEnv))
	if err != nil {
		t.Fatalf("integration tests call the real Mistral API but %v", err)
	}
	if key == "" {
		t.Fatalf("integration tests call the real Mistral API: set %s, for example %s=\"$(cat ~/.config/mufradat/mistral-api-key)\"", MistralEnv, MistralEnv)
	}
	return key
}

func Mistral(t *testing.T) *mistral.Client {
	t.Helper()
	client := &mistral.Client{Key: MistralKey(t), HTTP: &http.Client{Timeout: timeout}}
	if err := client.CheckKey(Context(t)); err != nil {
		t.Fatalf("%s was rejected by GET %s, which costs nothing: %v", MistralEnv, mistral.PathModels, err)
	}
	return client
}

func Google(t *testing.T) *tts.Client {
	t.Helper()
	path := env.OS().Getenv(GoogleEnv)
	if path == "" {
		t.Fatalf("integration tests call the real Google Text-to-Speech API: set %s to a service-account key file", GoogleEnv)
	}
	creds, err := googleauth.Resolve([]googleauth.Candidate{{Source: GoogleEnv, Path: path, Required: true}})
	if err != nil {
		t.Fatalf("%s: %v", GoogleEnv, err)
	}
	transport, err := googleauth.Transport(Context(t), nil, creds, "")
	if err != nil {
		t.Fatalf("%s: %v", GoogleEnv, err)
	}
	client := &tts.Client{HTTP: &http.Client{Transport: transport, Timeout: timeout}}
	if _, err := client.Voices(Context(t)); err != nil {
		t.Fatalf("the credentials in %s were refused by GET %s, which costs nothing: %v", GoogleEnv, tts.PathVoices, err)
	}
	return client
}
