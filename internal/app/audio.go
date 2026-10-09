package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"golang.org/x/time/rate"

	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/doctor"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/googleauth"
	"github.com/scuba-plaza/mufradat/internal/tts"
)

const googleEnv = "GOOGLE_APPLICATION_CREDENTIALS"

func (a *App) googleCandidates() ([]googleauth.Candidate, error) {
	user, err := a.UserConfig()
	if err != nil {
		return nil, err
	}
	gcloud := a.Env.Getenv("CLOUDSDK_CONFIG")
	if gcloud == "" {
		gcloud = filepath.Join(a.Env.Home, ".config", "gcloud")
	}
	return []googleauth.Candidate{
		{Source: googleEnv, Path: a.Env.Getenv(googleEnv), Required: true},
		{Source: "google.credentials_file", Path: user.Google.CredentialsFile, Required: true},
		{Source: "imported key", Path: a.Dirs().ImportedGoogleKey()},
		{Source: "application default credentials", Path: filepath.Join(gcloud, "application_default_credentials.json")},
	}, nil
}

func (a *App) GoogleCredentials() (googleauth.Credentials, error) {
	cands, err := a.googleCandidates()
	if err != nil {
		return googleauth.Credentials{}, err
	}
	c, err := googleauth.Resolve(cands)
	if errors.Is(err, googleauth.ErrNone) {
		return c, failure.NotSetUp(
			fmt.Sprintf("create a service-account key for a project with the Text-to-Speech API enabled, then run `%s auth google --import KEY.json`", a.CommandPrefix()),
			err,
		)
	}
	return c, err
}

func (a *App) ImportGoogleKey(ctx context.Context, src string) (googleauth.Info, error) {
	return googleauth.Import(ctx, src, a.Dirs().ImportedGoogleKey())
}

func (a *App) TTS(ctx context.Context) (*tts.Client, error) {
	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}
	creds, err := a.GoogleCredentials()
	if err != nil {
		return nil, err
	}
	var base http.RoundTripper
	if a.Env.HTTP != nil {
		base = a.Env.HTTP.Transport
	}
	authed, err := googleauth.Transport(ctx, base, creds, cfg.TTS.QuotaProject)
	if err != nil {
		return nil, failure.New(failure.ExitNotSetUp, fmt.Sprintf("re-import the key with `%s auth google --import KEY.json`", a.CommandPrefix()), err)
	}
	perMinute := max(cfg.TTS.RequestsPerMinute, 1)
	limiter := rate.NewLimiter(rate.Limit(float64(perMinute)/60), max(cfg.TTS.Concurrency, 1))
	return &tts.Client{BaseURL: cfg.TTS.Endpoint, HTTP: &http.Client{Transport: tts.NewRetryTransport(authed)}, Limit: limiter}, nil
}

type speaker struct{ client *tts.Client }

func (s speaker) Synthesize(ctx context.Context, item clips.Item, v clips.Voice) ([]byte, error) {
	return s.client.Synthesize(ctx, tts.Request{Text: item.Text, Voice: v.Name, Rate: v.Rate})
}

func (a *App) Speaker(client *tts.Client) clips.Synthesizer { return speaker{client: client} }

func (a *App) TTSFailure(err error) error {
	var api *tts.APIError
	if !errors.As(err, &api) {
		return err
	}
	switch api.Kind() {
	case tts.KindAuth:
		return failure.New(failure.ExitNotSetUp, fmt.Sprintf("check the key with `%s doctor`, or import another with `%s auth google --import KEY.json`", a.CommandPrefix(), a.CommandPrefix()), err)
	case tts.KindQuota:
		return failure.Temporary("enable the Text-to-Speech API and billing for the project, or set tts.quota_project", err)
	case tts.KindRateLimited, tts.KindTransient:
		return failure.Temporary("wait a little and run the command again; clips already made are kept", err)
	}
	return err
}

func (a *App) checkGoogle(ctx context.Context) doctor.Result {
	cands, err := a.googleCandidates()
	if err != nil {
		return doctor.Result{Status: doctor.Fail, Detail: err.Error()}
	}
	creds, err := googleauth.Resolve(cands)
	switch {
	case errors.Is(err, googleauth.ErrNone):
		return doctor.Result{
			Status: doctor.Warn,
			Detail: "no Google credentials; audio cannot be made",
			Hint:   fmt.Sprintf("run `%s auth google --import KEY.json`", a.CommandPrefix()),
		}
	case err != nil:
		return doctor.Result{Status: doctor.Fail, Detail: err.Error()}
	}
	info, err := googleauth.Validate(ctx, creds.JSON)
	if err != nil {
		return doctor.Result{Status: doctor.Fail, Detail: fmt.Sprintf("%s: %v", creds.Source, err)}
	}
	who := info.Type
	if info.Account != "" {
		who += " " + info.Account
	}
	return doctor.Result{Status: doctor.OK, Detail: fmt.Sprintf("%s (%s)", who, creds.Source)}
}
