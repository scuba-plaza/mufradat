package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/decisions"
	"github.com/scuba-plaza/mufradat/internal/deck"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/pyenv"
	"github.com/scuba-plaza/mufradat/internal/sidecar"
	"github.com/scuba-plaza/mufradat/internal/verify"
	"github.com/scuba-plaza/mufradat/python"
)

const verifyOffReason = "turned off (verify.enabled = off)"

func (a *App) PyEnv() *pyenv.Manager {
	return &pyenv.Manager{Dirs: a.Dirs(), Source: python.Files, Runner: a.Env.Runner, Now: a.Env.Now}
}

type VerifyStatus struct {
	Mode    string
	Enabled bool
	Reason  string
	Runtime pyenv.Runtime
}

func (a *App) VerifyStatus() (VerifyStatus, error) {
	cfg, err := a.Config()
	if err != nil {
		return VerifyStatus{}, err
	}
	st := VerifyStatus{Mode: cfg.Verify.Enabled}
	if cfg.Verify.Enabled == config.VerifyOff {
		st.Reason = verifyOffReason
		return st, nil
	}
	rt, err := a.PyEnv().Resolve(cfg.Verify.Python)
	if err != nil {
		if failure.Code(err) == failure.ExitNotSetUp {
			st.Reason = err.Error()
			return st, nil
		}
		return st, err
	}
	st.Enabled, st.Runtime = true, rt
	return st, nil
}

type Verification struct {
	Status    VerifyStatus
	Report    verify.Report
	Dismissed *decisions.Set
	Stale     int
}

func (a *App) Decisions() (*decisions.Log, error) {
	ws, err := a.Workspace()
	if err != nil {
		return nil, err
	}
	return decisions.NewLog(ws.DecisionsFile(), a.Env.Now), nil
}

func (a *App) LoadVerification(notes []note.Note) (Verification, error) {
	st, err := a.VerifyStatus()
	if err != nil {
		return Verification{}, err
	}
	log, err := a.Decisions()
	if err != nil {
		return Verification{}, err
	}
	dismissed, err := log.Load()
	if err != nil {
		return Verification{}, err
	}
	v := Verification{Status: st, Dismissed: dismissed}
	var readings map[string]verify.Reading
	if st.Enabled {
		ws, err := a.Workspace()
		if err != nil {
			return Verification{}, err
		}
		loaded, err := verify.NewCache(ws.ReadingsFile()).Load(st.Runtime.Fingerprint)
		if err != nil {
			return Verification{}, err
		}
		readings, v.Stale = loaded.Readings, loaded.Stale
	}
	v.Report = verify.Derive(notes, readings)
	return v, nil
}

func (v Verification) Counts() verify.Counts { return v.Report.Count(v.Dismissed) }

func (v Verification) DeckCheck() func(note.Note) deck.Check {
	if v.Status.Mode == config.VerifyOff {
		return func(note.Note) deck.Check { return deck.Check{Verified: true} }
	}
	return func(n note.Note) deck.Check {
		res := v.Report.Results[n.ID]
		check := deck.Check{Verified: res.Verified}
		if !res.Verified {
			return check
		}
		for _, is := range res.Open(n.ID, v.Dismissed) {
			sev := deck.Minor
			if is.Severity == verify.Major {
				sev = deck.Major
			}
			check.Flags = append(check.Flags, deck.Flag{Severity: sev, Word: is.Word, Detail: is.Detail})
		}
		return check
	}
}

type CheckOptions struct {
	Force bool
}

type CheckOutcome struct {
	Verification Verification
	Notes        int
	Checked      int
	Cached       int
	Failures     map[string]string
}

func (a *App) RunCheck(ctx context.Context, opts CheckOptions) (CheckOutcome, error) {
	checker, err := a.NewChecker()
	if err != nil {
		return CheckOutcome{}, err
	}
	defer checker.Close()
	store, err := a.Store()
	if err != nil {
		return CheckOutcome{}, err
	}
	set, err := store.Load()
	if err != nil {
		return CheckOutcome{}, err
	}
	notes := set.All()
	all := checker.itemsOf(notes)
	loaded, err := checker.cache.Load(checker.fp)
	if err != nil {
		return CheckOutcome{}, err
	}
	var todo []verify.Item
	for _, it := range all {
		if _, ok := loaded.Readings[it.Key]; !ok || opts.Force {
			todo = append(todo, it)
		}
	}
	outcome := CheckOutcome{Notes: len(notes), Cached: len(all) - len(todo), Failures: map[string]string{}}
	if len(todo) > 0 {
		r := a.Reporter()
		r.Event("checking %d of %d items (%d already known)", len(todo), len(all), outcome.Cached)
		res, err := checker.fill(ctx, todo, func(done, total int) { r.Progress("checking %d/%d", done, total) })
		r.Done()
		outcome.Checked, outcome.Failures = res.Added, res.Failures
		if err != nil {
			return outcome, err
		}
	}
	if err := checker.compact(all); err != nil {
		return outcome, err
	}
	outcome.Verification, err = a.LoadVerification(notes)
	return outcome, err
}

func (a *App) sidecarFailure(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var f *failure.Error
	if errors.As(err, &f) {
		return err
	}
	hint := pyenv.Diagnose(err.Error())
	if hint == "" {
		hint = "run `" + a.CommandPrefix() + " doctor` to see what is wrong with the vowel-checking environment"
	}
	return failure.New(failure.ExitFailure, hint, fmt.Errorf("vowel checking failed: %w", err))
}

type SetupOptions struct {
	Python string
	Force  bool
}

type SetupOutcome struct {
	Ready   pyenv.Ready
	Custom  string
	Managed bool
}

func (a *App) RunSetup(ctx context.Context, opts SetupOptions) (SetupOutcome, error) {
	user, err := a.UserConfig()
	if err != nil {
		return SetupOutcome{}, err
	}
	mgr := a.PyEnv()
	r := a.Reporter()
	setup := pyenv.SetupOptions{
		Python: opts.Python,
		Force:  opts.Force,
		Step:   func(message string) { r.Event("%s", message) },
		Output: sidecar.LogWriter(a.Logger()),
	}
	if user.Verify.Python != "" {
		rt, err := mgr.Resolve(user.Verify.Python)
		if err != nil {
			return SetupOutcome{}, err
		}
		r.Event("using %s (verify.python); testing it", rt.Python)
		if _, err := mgr.Selftest(ctx, rt, setup); err != nil {
			return SetupOutcome{}, a.sidecarFailure(err)
		}
		return SetupOutcome{Custom: rt.Python}, nil
	}
	ready, err := mgr.Setup(ctx, setup)
	if err != nil {
		return SetupOutcome{}, a.sidecarFailure(err)
	}
	return SetupOutcome{Ready: ready, Managed: true}, nil
}

type Checker struct {
	app   *App
	rt    pyenv.Runtime
	cache *verify.Cache
	fp    string

	mu     sync.Mutex
	engine verify.Engine
}

func (a *App) NewChecker() (*Checker, error) {
	st, err := a.VerifyStatus()
	if err != nil {
		return nil, err
	}
	switch {
	case st.Mode == config.VerifyOff:
		return nil, failure.Newf(failure.ExitUsage, "turn it back on with `"+a.CommandPrefix()+" config set verify.enabled auto`", "vowel checking is turned off")
	case !st.Enabled:
		return nil, failure.NotSetUp("run `"+a.CommandPrefix()+" setup` first", errors.New(st.Reason))
	}
	ws, err := a.Workspace()
	if err != nil {
		return nil, err
	}
	return &Checker{app: a, rt: st.Runtime, cache: verify.NewCache(ws.ReadingsFile()), fp: st.Runtime.Fingerprint}, nil
}

func (c *Checker) itemsOf(notes []note.Note) []verify.Item {
	var all []verify.Item
	for _, n := range notes {
		all = append(all, verify.Items(n)...)
	}
	return verify.Unique(all)
}

func (c *Checker) start(ctx context.Context) (verify.Engine, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.engine != nil {
		return c.engine, nil
	}
	e, err := sidecar.Start(ctx, c.app.PyEnv().Launcher(c.rt), sidecar.LogWriter(c.app.Logger()))
	if err != nil {
		return nil, c.app.sidecarFailure(err)
	}
	c.engine = e
	return e, nil
}

func (c *Checker) fill(ctx context.Context, items []verify.Item, progress func(done, total int)) (verify.FillResult, error) {
	engine, err := c.start(ctx)
	if err != nil {
		return verify.FillResult{Failures: map[string]string{}}, err
	}
	res, err := verify.Fill(ctx, engine, c.cache, items, verify.FillOptions{
		Fingerprint: c.fp,
		Progress:    progress,
		Status:      func(message string) { c.app.Logger().Debug(message, "source", "sidecar") },
	})
	if err != nil {
		return res, c.app.sidecarFailure(err)
	}
	return res, nil
}

func (c *Checker) compact(all []verify.Item) error {
	keep := make(map[string]bool, len(all))
	for _, it := range all {
		keep[it.Key] = true
	}
	_, err := c.cache.Compact(c.fp, keep)
	return err
}

func (c *Checker) Check(ctx context.Context, notes []note.Note) (verify.Report, error) {
	loaded, err := c.cache.Load(c.fp)
	if err != nil {
		return verify.Report{}, err
	}
	report := verify.Derive(notes, loaded.Readings)
	if len(report.Pending) == 0 {
		return report, nil
	}
	if _, err := c.fill(ctx, report.Pending, nil); err != nil {
		return report, err
	}
	loaded, err = c.cache.Load(c.fp)
	if err != nil {
		return verify.Report{}, err
	}
	return verify.Derive(notes, loaded.Readings), nil
}

func (c *Checker) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.engine == nil {
		return nil
	}
	err := c.engine.Close()
	c.engine = nil
	return err
}
