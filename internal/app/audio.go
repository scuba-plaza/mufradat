package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"golang.org/x/time/rate"

	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/doctor"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/googleauth"
	"github.com/scuba-plaza/mufradat/internal/httpretry"
	"github.com/scuba-plaza/mufradat/internal/mistral"
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

func (a *App) Speaker(ctx context.Context) (clips.Speaker, error) {
	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}
	return a.speakerFor(ctx, cfg, cfg.Audio.Provider)
}

func (a *App) speakerFor(ctx context.Context, cfg config.Config, provider string) (clips.Speaker, error) {
	perMinute := max(cfg.TTS.RequestsPerMinute, 1)
	limiter := rate.NewLimiter(rate.Limit(float64(perMinute)/60), max(cfg.TTS.Concurrency, 1))
	switch provider {
	case config.ProviderGoogle:
		return a.googleSpeaker(ctx, cfg, limiter)
	case config.ProviderMistral:
		client, err := a.mistralClient(true, limiter)
		if err != nil {
			return nil, err
		}
		return mistralSpeaker{client: client}, nil
	}
	return nil, failure.Usage(fmt.Errorf("audio.provider %q is not supported", provider))
}

func (a *App) googleSpeaker(ctx context.Context, cfg config.Config, limiter *rate.Limiter) (clips.Speaker, error) {
	creds, err := a.GoogleCredentials()
	if err != nil {
		return nil, err
	}
	authed, err := googleauth.Transport(ctx, a.transport(), creds, cfg.Google.QuotaProject)
	if err != nil {
		return nil, failure.New(failure.ExitNotSetUp, fmt.Sprintf("re-import the key with `%s auth google --import KEY.json`", a.CommandPrefix()), err)
	}
	client := &tts.Client{BaseURL: cfg.Google.Endpoint, HTTP: &http.Client{Transport: httpretry.New(authed)}, Limit: limiter}
	return googleSpeaker{client: client}, nil
}

type googleSpeaker struct{ client *tts.Client }

func (s googleSpeaker) Synthesize(ctx context.Context, req clips.SpeechRequest) ([]byte, error) {
	return s.client.Synthesize(ctx, tts.Request{Text: req.Text, Voice: req.Voice, Rate: req.Rate})
}

func (s googleSpeaker) Voices(ctx context.Context) ([]clips.VoiceInfo, error) {
	voices, err := s.client.Voices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]clips.VoiceInfo, 0, len(voices))
	for _, v := range voices {
		detail := ""
		if v.SampleRate > 0 {
			detail = fmt.Sprintf("%g kHz", float64(v.SampleRate)/1000)
		}
		out = append(out, clips.VoiceInfo{Ref: v.Name, Name: v.Name, Gender: v.Gender, Detail: detail})
	}
	return out, nil
}

type mistralSpeaker struct{ client *mistral.Client }

func (s mistralSpeaker) Synthesize(ctx context.Context, req clips.SpeechRequest) ([]byte, error) {
	return s.client.Synthesize(ctx, mistral.SpeechRequest{Text: req.Text, Voice: req.Voice})
}

func (s mistralSpeaker) Voices(ctx context.Context) ([]clips.VoiceInfo, error) {
	voices, err := s.client.Voices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]clips.VoiceInfo, 0, len(voices))
	for _, v := range voices {
		info := clips.VoiceInfo{Ref: v.ID, Name: v.Name, Gender: v.Gender, Aliases: []string{v.ID}}
		if v.Slug != "" {
			info.Ref = v.Slug
		}
		out = append(out, info)
	}
	return out, nil
}

func (a *App) SpeechFailure(err error) error {
	var google *tts.APIError
	var service *mistral.APIError
	var voice *mistral.VoiceError
	p := a.CommandPrefix()
	switch {
	case errors.As(err, &google):
		switch google.Kind() {
		case tts.KindAuth:
			return failure.New(failure.ExitNotSetUp, fmt.Sprintf("check the key with `%s doctor`, or import another with `%s auth google --import KEY.json`", p, p), err)
		case tts.KindQuota:
			return failure.Temporary("enable the Text-to-Speech API and billing for the project, or set google.quota_project", err)
		case tts.KindRateLimited, tts.KindTransient:
			return failure.Temporary("wait a little and run the command again; clips already made are kept", err)
		}
	case errors.As(err, &service):
		switch service.Kind() {
		case mistral.KindAuth:
			return failure.New(failure.ExitNotSetUp, fmt.Sprintf("check the key with `%s doctor`, or import another with `%s auth mistral --import FILE`", p, p), err)
		case mistral.KindRateLimited, mistral.KindTransient:
			return failure.Temporary("wait a little and run the command again; clips already made are kept", err)
		case mistral.KindBlocked:
			return failure.New(failure.ExitFailure, fmt.Sprintf("Mistral's moderation rejected this text; edit the note, or switch with `%s config set audio.provider google`", p), err)
		}
	case errors.As(err, &voice):
		return failure.New(failure.ExitFailure, fmt.Sprintf("see the choices with `%s voices list`, then `%s config set mistral.voice NAME`", p, p), err)
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
			Status: doctor.Fail,
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
