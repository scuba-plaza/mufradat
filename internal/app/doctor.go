package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/doctor"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/mistral"
	"github.com/scuba-plaza/mufradat/internal/pyenv"
)

func (a *App) DoctorChecks() []doctor.Check {
	checks := []doctor.Check{
		{Name: "workspace", Run: a.checkWorkspace},
		{Name: "configuration", Run: a.checkConfig},
		{Name: "mistral key", Run: a.checkMistral},
	}
	if a.verifyConfig().Audio.Provider == config.ProviderGoogle {
		checks = append(checks, doctor.Check{Name: "google credentials", Run: a.checkGoogle})
	}
	return append(checks, doctor.Check{Name: "vowel checking", Run: a.checkVerify})
}

func (a *App) checkWorkspace(context.Context) doctor.Result {
	res, err := a.Resolution()
	if err != nil {
		return doctor.Result{Status: doctor.Fail, Detail: err.Error(), Hint: "run `mufradat init`"}
	}
	return doctor.Result{Status: doctor.OK, Detail: fmt.Sprintf("%s (%s)", res.Workspace.Root, res.Source)}
}

func (a *App) checkConfig(context.Context) doctor.Result {
	if _, err := a.Workspace(); err != nil {
		return doctor.Result{Status: doctor.Skip, Detail: "no workspace"}
	}
	if _, err := a.Config(); err != nil {
		return doctor.Result{Status: doctor.Fail, Detail: err.Error()}
	}
	return doctor.Result{Status: doctor.OK, Detail: "valid"}
}

func (a *App) verifyConfig() config.Config {
	if cfg, err := a.Config(); err == nil {
		return cfg
	}
	cfg, _ := a.UserConfig()
	return cfg
}

func (a *App) checkVerify(ctx context.Context) doctor.Result {
	cfg := a.verifyConfig()
	if cfg.Verify.Enabled == config.VerifyOff {
		return doctor.Result{Status: doctor.Skip, Detail: verifyOffReason}
	}
	missing := doctor.Warn
	if cfg.Verify.Enabled == config.VerifyOn {
		missing = doctor.Fail
	}
	mgr := a.PyEnv()
	rt, err := mgr.Resolve(cfg.Verify.Python)
	if err != nil {
		return doctor.Result{Status: missing, Detail: err.Error(), Hint: failure.Hint(err)}
	}
	if rt.Managed {
		for _, p := range pyenv.CamelPackages {
			if !fsutil.IsDir(filepath.Join(a.Dirs().CamelData(), "data", filepath.FromSlash(p.Dir))) {
				return doctor.Result{Status: missing, Detail: "CAMeL data " + p.Name + " is missing", Hint: "run `" + a.CommandPrefix() + " setup --force`"}
			}
		}
	}
	res, err := mgr.Probe(ctx, rt, pyenv.SetupOptions{})
	if err != nil {
		hint := failure.Hint(err)
		if hint == "" {
			hint = pyenv.Diagnose(err.Error())
		}
		return doctor.Result{Status: doctor.Fail, Detail: firstLine(err.Error()), Hint: hint}
	}
	kind := "managed"
	if !rt.Managed {
		kind = "verify.python"
	}
	return doctor.Result{Status: doctor.OK, Detail: fmt.Sprintf("%s environment: Python %s, camel-tools %s, torch %s", kind, res.Version("python"), res.Version("camel-tools"), res.Version("torch"))}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

const keyCheckTimeout = 20 * time.Second

func (a *App) checkMistral(ctx context.Context) doctor.Result {
	cands, err := a.mistralCandidates()
	if err != nil {
		return doctor.Result{Status: doctor.Fail, Detail: err.Error()}
	}
	key, err := mistral.ResolveKey(cands)
	switch {
	case errors.Is(err, mistral.ErrNoKey):
		return doctor.Result{
			Status: doctor.Fail,
			Detail: "no Mistral API key; notes cannot be written",
			Hint:   fmt.Sprintf("set MISTRAL_API_KEY, or run `%s auth mistral --import FILE`", a.CommandPrefix()),
		}
	case err != nil:
		return doctor.Result{Status: doctor.Fail, Detail: err.Error(), Hint: fmt.Sprintf("fix the key, or run `%s auth mistral --import FILE`", a.CommandPrefix())}
	}
	cfg := a.verifyConfig()
	client := &mistral.Client{
		BaseURL: cfg.Mistral.Endpoint, Key: key.Value,
		HTTP: &http.Client{Transport: a.transport(), Timeout: keyCheckTimeout},
	}
	err = client.CheckKey(ctx)
	var api *mistral.APIError
	switch {
	case err == nil:
		return doctor.Result{Status: doctor.OK, Detail: fmt.Sprintf("accepted by Mistral (%s)", key.Source)}
	case errors.As(err, &api) && api.Kind() == mistral.KindAuth:
		return doctor.Result{
			Status: doctor.Fail,
			Detail: fmt.Sprintf("Mistral rejected the key from %s (%s)", key.Source, api.Message),
			Hint:   fmt.Sprintf("set MISTRAL_API_KEY, or run `%s auth mistral --import FILE`", a.CommandPrefix()),
		}
	}
	return doctor.Result{Status: doctor.Warn, Detail: fmt.Sprintf("could not check the key from %s with Mistral: %v", key.Source, err)}
}
