package app

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/httpretry"
	"github.com/scuba-plaza/mufradat/internal/jsonl"
	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/mistral"
)

const (
	mistralEnv       = "MISTRAL_API_KEY"
	llmAttempts      = 3
	llmBackoff       = 2 * time.Second
	llmMaxWait       = 10 * time.Minute
	importedKeySrc   = "imported key"
	configuredKeySrc = "mistral.api_key"
)

func (a *App) mistralCandidates() ([]mistral.Candidate, error) {
	user, err := a.UserConfig()
	if err != nil {
		return nil, err
	}
	imported, err := fsutil.ReadFileIfExists(a.Dirs().ImportedMistralKey())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", importedKeySrc, err)
	}
	return []mistral.Candidate{
		{Source: mistralEnv, Value: a.Env.Getenv(mistralEnv)},
		{Source: configuredKeySrc, Value: user.Mistral.APIKey},
		{Source: importedKeySrc, Value: string(imported)},
	}, nil
}

func (a *App) MistralKey() (mistral.Key, error) {
	cands, err := a.mistralCandidates()
	if err != nil {
		return mistral.Key{}, err
	}
	key, err := mistral.ResolveKey(cands)
	switch {
	case errors.Is(err, mistral.ErrNoKey):
		return key, failure.NotSetUp(
			fmt.Sprintf("set MISTRAL_API_KEY, or save the key in a file and run `%s auth mistral --import FILE`", a.CommandPrefix()),
			err,
		)
	case err != nil:
		return key, failure.NotSetUp(fmt.Sprintf("fix the key, or import another with `%s auth mistral --import FILE`", a.CommandPrefix()), err)
	}
	return key, nil
}

func (a *App) ImportMistralKey(src string) error {
	raw, err := fsutil.ReadFileIfExists(src)
	if err != nil {
		return err
	}
	if raw == nil {
		return fmt.Errorf("%s does not exist", src)
	}
	key, err := mistral.ParseKey(src, string(raw))
	if err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("%s is empty", src)
	}
	return fsutil.WriteFileAtomic(a.Dirs().ImportedMistralKey(), []byte(key+"\n"), 0o600)
}

func (a *App) transport() http.RoundTripper {
	if a.Env.HTTP != nil && a.Env.HTTP.Transport != nil {
		return a.Env.HTTP.Transport
	}
	return http.DefaultTransport
}

func (a *App) timeout() time.Duration {
	if a.Env.HTTP != nil {
		return a.Env.HTTP.Timeout
	}
	return 0
}

func (a *App) mistralClient(retry bool, limit mistral.Waiter) (*mistral.Client, error) {
	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}
	key, err := a.MistralKey()
	if err != nil {
		return nil, err
	}
	rt := a.transport()
	if retry {
		rt = httpretry.New(rt)
	}
	return &mistral.Client{
		BaseURL: cfg.Mistral.Endpoint, Key: key.Value, Limit: limit,
		HTTP: &http.Client{Transport: rt, Timeout: a.timeout()},
	}, nil
}

func (a *App) LLM() (llm.Backend, error) {
	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}
	ws, err := a.Workspace()
	if err != nil {
		return nil, err
	}
	client, err := a.mistralClient(false, nil)
	if err != nil {
		return nil, err
	}
	retrying := llm.Retrying{
		Next: mistral.Backend{Client: client, Model: cfg.Author.Model}, Attempts: llmAttempts, Backoff: llmBackoff, MaxWait: llmMaxWait, Sleep: llm.Sleep,
		OnWait: func(d time.Duration, err error) {
			a.Reporter().Warn("%v; trying again in %s", err, d.Round(time.Second))
		},
	}
	return llm.Recording{
		Next: retrying, Now: a.Env.Now,
		Sink: func(r llm.Record) error { return jsonl.Append(ws.UsageFile(), r) },
	}, nil
}
