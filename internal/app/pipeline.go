package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/scuba-plaza/mufradat/internal/apkg"
	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/deck"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/googleauth"
	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/ranking"
	"github.com/scuba-plaza/mufradat/internal/status"
	"github.com/scuba-plaza/mufradat/internal/store"
	"github.com/scuba-plaza/mufradat/internal/vocab"
	"github.com/scuba-plaza/mufradat/internal/workspace"
)

func (a *App) Store() (*store.Store, error) {
	ws, err := a.Workspace()
	if err != nil {
		return nil, err
	}
	return store.New(ws, a.Env.Now, note.RandomIDs{}), nil
}

func (a *App) Clips() (*clips.Store, error) {
	ws, err := a.Workspace()
	if err != nil {
		return nil, err
	}
	return clips.NewStore(ws.AudioDir()), nil
}

func (a *App) Voice() (clips.Voice, error) {
	cfg, err := a.Config()
	if err != nil {
		return clips.Voice{}, err
	}
	return clips.Voice{Name: cfg.Google.Voice, Rate: cfg.Audio.Rate}, nil
}

func (a *App) Ranking() (*ranking.List, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ranking == nil {
		l, err := ranking.Embedded()
		if err != nil {
			return nil, err
		}
		a.ranking = l
	}
	return a.ranking, nil
}

func (a *App) Lexicon() (*lexicon.Index, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lexicon == nil {
		ix, err := lexicon.Embedded()
		if err != nil {
			return nil, err
		}
		a.lexicon = ix
	}
	return a.lexicon, nil
}

func (a *App) Vocabulary() (*vocab.Checker, error) {
	lex, err := a.Lexicon()
	if err != nil {
		return nil, err
	}
	list, err := a.Ranking()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vocab == nil {
		c, err := vocab.Embedded(lex, list)
		if err != nil {
			return nil, err
		}
		a.vocab = c
	}
	return a.vocab, nil
}

type BuildPlan struct {
	Package     *apkg.Package
	Voice       clips.Voice
	Summary     deck.Summary
	Fingerprint string
	OutPath     string
}

func (a *App) buildEpoch() time.Time {
	var sec int64
	if _, err := fmt.Sscanf(a.Env.Getenv("SOURCE_DATE_EPOCH"), "%d", &sec); err != nil || sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

func (a *App) PlanBuild(outOverride string) (BuildPlan, error) {
	ws, err := a.Workspace()
	if err != nil {
		return BuildPlan{}, err
	}
	cfg, err := a.Config()
	if err != nil {
		return BuildPlan{}, err
	}
	st, err := a.Store()
	if err != nil {
		return BuildPlan{}, err
	}
	set, err := st.Load()
	if err != nil {
		return BuildPlan{}, err
	}
	list, err := a.Ranking()
	if err != nil {
		return BuildPlan{}, err
	}
	voice, err := a.Voice()
	if err != nil {
		return BuildPlan{}, err
	}
	cs, err := a.Clips()
	if err != nil {
		return BuildPlan{}, err
	}
	ver, err := a.LoadVerification(set.All())
	if err != nil {
		return BuildPlan{}, err
	}
	pkg, sum, err := deck.Build(set.All(), deck.Options{
		DeckName:      cfg.DeckName,
		DeckID:        cfg.Anki.DeckID,
		ModelID:       cfg.Anki.ModelID,
		ProductionTop: cfg.Anki.ProductionTop,
		UnrankedBase:  list.Len(),
		Voice:         voice,
		Clips:         cs,
		Check:         ver.DeckCheck(),
		Epoch:         a.buildEpoch(),
	})
	if err != nil {
		return BuildPlan{}, err
	}
	fp, err := deck.Fingerprint(pkg)
	if err != nil {
		return BuildPlan{}, err
	}
	out := outOverride
	if out == "" {
		out = filepath.Join(ws.OutDir(), deck.Slug(cfg.DeckName)+".apkg")
	}
	return BuildPlan{Package: pkg, Voice: voice, Summary: sum, Fingerprint: fp, OutPath: out}, nil
}

func (a *App) WriteBuild(plan BuildPlan) error {
	ws, err := a.Workspace()
	if err != nil {
		return err
	}
	if err := plan.Package.WriteFile(plan.OutPath); err != nil {
		return err
	}
	abs, err := filepath.Abs(plan.OutPath)
	if err != nil {
		return err
	}
	return deck.SaveState(ws.BuildStateFile(), deck.State{
		Fingerprint: plan.Fingerprint,
		Path:        abs,
		Notes:       plan.Summary.Notes,
		Built:       a.Env.Now().UTC().Truncate(time.Second),
	})
}

func (a *App) CommandPrefix() string {
	res, err := a.Resolution()
	if err == nil && res.Source == workspace.FromFlag {
		return ProgramName + " -C " + res.Workspace.Root
	}
	return ProgramName
}

func (a *App) StatusSnapshot() (status.Snapshot, error) {
	ws, err := a.Workspace()
	if err != nil {
		return status.Snapshot{}, err
	}
	st, err := a.Store()
	if err != nil {
		return status.Snapshot{}, err
	}
	set, err := st.Load()
	if err != nil {
		return status.Snapshot{}, err
	}
	list, err := a.Ranking()
	if err != nil {
		return status.Snapshot{}, err
	}
	snap := status.Snapshot{Prefix: a.CommandPrefix(), Notes: set.Len(), RankedTotal: list.Len()}
	for _, n := range set.All() {
		if n.Rank > 0 {
			snap.RankedWritten++
		}
		if note.Validate(n) != nil {
			snap.InvalidNotes++
		}
	}
	ver, err := a.LoadVerification(set.All())
	if err != nil {
		return snap, err
	}
	snap.VerifyEnabled = ver.Status.Enabled
	snap.VerifyRequired = ver.Status.Mode == config.VerifyOn
	snap.VerifyReason = ver.Status.Reason
	if ver.Status.Enabled {
		counts := ver.Counts()
		snap.Unchecked, snap.OpenMajor, snap.OpenMinor = counts.Unchecked, counts.Major, counts.Minor
	}
	if err := a.fillAudio(&snap, set); err != nil {
		return snap, err
	}
	if set.Len() == 0 {
		return snap, nil
	}
	state, built, err := deck.LoadState(ws.BuildStateFile())
	if err != nil {
		return snap, err
	}
	snap.Built = built && fsutil.Exists(state.Path)
	if snap.Built {
		plan, err := a.PlanBuild(state.Path)
		if err != nil {
			return snap, err
		}
		snap.BuildCurrent = plan.Fingerprint == state.Fingerprint
		snap.Package = state.Path
	}
	return snap, nil
}

func (a *App) fillAudio(snap *status.Snapshot, set *note.Set) error {
	cs, err := a.Clips()
	if err != nil {
		return err
	}
	voice, err := a.Voice()
	if err != nil {
		return err
	}
	plan := clips.NewPlan(set.Study(), voice, cs.Has)
	snap.AudioVoice = voice.Describe()
	snap.ClipsWanted = plan.Wanted
	snap.ClipsMissing = len(plan.Missing)
	reason := a.audioNotReady()
	snap.AudioReady = reason == "" || len(plan.Missing) == 0
	if !snap.AudioReady {
		snap.AudioReason = reason
	}
	return nil
}

func (a *App) audioNotReady() string {
	_, err := a.GoogleCredentials()
	switch {
	case err == nil:
		return ""
	case errors.Is(err, googleauth.ErrNone):
		return "no Google credentials"
	}
	return err.Error()
}
