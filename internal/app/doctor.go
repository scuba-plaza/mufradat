package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/doctor"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/proc"
	"github.com/scuba-plaza/mufradat/internal/pyenv"
)

func (a *App) DoctorChecks() []doctor.Check {
	return []doctor.Check{
		{Name: "workspace", Run: a.checkWorkspace},
		{Name: "configuration", Run: a.checkConfig},
		{Name: "claude code", Run: a.checkClaude},
		{Name: "google credentials", Run: a.checkGoogle},
		{Name: "vowel checking", Run: a.checkVerify},
	}
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

func (a *App) checkClaude(ctx context.Context) doctor.Result {
	path, err := a.Env.Runner.LookPath("claude")
	if err != nil {
		return doctor.Result{
			Status: doctor.Fail,
			Detail: "`claude` is not on PATH",
			Hint:   "install Claude Code (https://claude.com/claude-code) and log in; it writes the notes",
		}
	}
	var out strings.Builder
	err = a.Env.Runner.Run(ctx, proc.Cmd{Name: path, Args: []string{"--version"}, Stdout: &out})
	if err != nil {
		return doctor.Result{Status: doctor.Warn, Detail: path + " did not report a version"}
	}
	return doctor.Result{Status: doctor.OK, Detail: strings.TrimSpace(out.String())}
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
