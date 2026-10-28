package app

import (
	"log/slog"
	"sync"
	"time"

	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/ranking"
	"github.com/scuba-plaza/mufradat/internal/ui"
	"github.com/scuba-plaza/mufradat/internal/vocab"
	"github.com/scuba-plaza/mufradat/internal/workspace"
	"github.com/scuba-plaza/mufradat/internal/xdg"
)

const ProgramName = "mufradat"

type Options struct {
	Workspace string
	Verbose   int
	Quiet     bool
	NoColor   bool
}

type App struct {
	Env  env.Env
	Opts Options

	mu       sync.Mutex
	dirs     *xdg.Dirs
	reporter *ui.Reporter
	logger   *slog.Logger
	ws       *workspace.Resolution
	cfg      *config.Config
	ranking  *ranking.List
	lexicon  *lexicon.Index
	vocab    *vocab.Checker

	llmBackoff time.Duration
}

func New(e env.Env) *App {
	return &App{Env: e, llmBackoff: 2 * time.Second}
}

func (a *App) Dirs() xdg.Dirs {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dirs == nil {
		d := xdg.New(a.Env.Getenv, a.Env.Home)
		a.dirs = &d
	}
	return *a.dirs
}

func (a *App) uiOptions(tty bool) ui.Options {
	color := tty && !a.Opts.NoColor && a.Env.Getenv("NO_COLOR") == "" && a.Env.Getenv("TERM") != "dumb"
	return ui.Options{TTY: tty, Color: color, Verbose: a.Opts.Verbose, Quiet: a.Opts.Quiet}
}

func (a *App) Reporter() *ui.Reporter {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.reporter == nil {
		a.reporter = ui.NewReporter(a.Env.Stderr, a.uiOptions(a.Env.StderrTTY))
	}
	return a.reporter
}

func (a *App) Logger() *slog.Logger {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.logger == nil {
		a.logger = ui.NewLogger(a.Env.Stderr, a.uiOptions(a.Env.StderrTTY))
	}
	return a.logger
}

func (a *App) StdoutColor() bool {
	return a.uiOptions(a.Env.StdoutTTY).Color
}

func (a *App) UserConfig() (config.Config, error) {
	return config.Load(a.Dirs().UserConfigFile())
}

func (a *App) Resolution() (workspace.Resolution, error) {
	a.mu.Lock()
	cached := a.ws
	a.mu.Unlock()
	if cached != nil {
		return *cached, nil
	}
	user, err := a.UserConfig()
	if err != nil {
		return workspace.Resolution{}, err
	}
	cwd, err := a.Env.Cwd()
	if err != nil {
		return workspace.Resolution{}, err
	}
	res, err := workspace.Resolve(workspace.Query{
		Flag:            a.Opts.Workspace,
		Getenv:          a.Env.Getenv,
		Cwd:             cwd,
		UserConfigValue: user.Workspace,
		DefaultRoot:     a.Dirs().DefaultWorkspace(),
	})
	if err != nil {
		return workspace.Resolution{}, err
	}
	a.mu.Lock()
	a.ws = &res
	a.mu.Unlock()
	return res, nil
}

func (a *App) Workspace() (workspace.Workspace, error) {
	res, err := a.Resolution()
	return res.Workspace, err
}

func (a *App) Config() (config.Config, error) {
	a.mu.Lock()
	cached := a.cfg
	a.mu.Unlock()
	if cached != nil {
		return *cached, nil
	}
	ws, err := a.Workspace()
	if err != nil {
		return config.Config{}, err
	}
	cfg, err := config.Load(a.Dirs().UserConfigFile(), ws.ConfigFile())
	if err != nil {
		return cfg, err
	}
	a.mu.Lock()
	a.cfg = &cfg
	a.mu.Unlock()
	return cfg, nil
}

func (a *App) InvalidateConfig() {
	a.mu.Lock()
	a.cfg = nil
	a.mu.Unlock()
}
