package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
)

const (
	Marker = "mufradat.toml"
	EnvVar = "MUFRADAT_WORKSPACE"
)

const initHint = "run `mufradat init` to create a workspace"

type Workspace struct {
	Root string
}

func (w Workspace) ConfigFile() string    { return filepath.Join(w.Root, Marker) }
func (w Workspace) NotesFile() string     { return filepath.Join(w.Root, "notes.jsonl") }
func (w Workspace) DecisionsFile() string { return filepath.Join(w.Root, "decisions.jsonl") }
func (w Workspace) AudioDir() string      { return filepath.Join(w.Root, "audio") }
func (w Workspace) StateDir() string      { return filepath.Join(w.Root, "state") }
func (w Workspace) OutDir() string        { return filepath.Join(w.Root, "out") }
func (w Workspace) LockFile() string      { return filepath.Join(w.StateDir(), "lock") }
func (w Workspace) ReadingsFile() string  { return filepath.Join(w.StateDir(), "readings.jsonl") }
func (w Workspace) UsageFile() string     { return filepath.Join(w.StateDir(), "usage.jsonl") }
func (w Workspace) BuildStateFile() string {
	return filepath.Join(w.StateDir(), "last-build.json")
}
func (w Workspace) VoiceSamplesDir() string { return filepath.Join(w.StateDir(), "voice-samples") }

type Source string

const (
	FromFlag    Source = "flag"
	FromEnv     Source = "environment"
	FromCwd     Source = "current directory"
	FromConfig  Source = "user config"
	FromDefault Source = "default location"
)

type Resolution struct {
	Workspace Workspace
	Source    Source
}

type Query struct {
	Flag            string
	Getenv          func(string) string
	Cwd             string
	UserConfigValue string
	DefaultRoot     string
}

func Resolve(q Query) (Resolution, error) {
	if q.Flag != "" {
		return open(q.Flag, FromFlag)
	}
	if v := q.Getenv(EnvVar); v != "" {
		return open(v, FromEnv)
	}
	if root, ok := findUp(q.Cwd); ok {
		return Resolution{Workspace: Workspace{Root: root}, Source: FromCwd}, nil
	}
	if q.UserConfigValue != "" {
		return open(q.UserConfigValue, FromConfig)
	}
	return open(q.DefaultRoot, FromDefault)
}

func open(root string, source Source) (Resolution, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Resolution{}, err
	}
	w := Workspace{Root: abs}
	if !fsutil.Exists(w.ConfigFile()) {
		return Resolution{}, failure.NotSetUp(initHint,
			fmt.Errorf("no workspace at %s (from %s): %s is missing", abs, source, Marker))
	}
	return Resolution{Workspace: w, Source: source}, nil
}

func findUp(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	for {
		if fsutil.Exists(filepath.Join(dir, Marker)) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

type InitOptions struct {
	Root   string
	Git    bool
	DeckID int64
}

func Init(opts InitOptions) (Workspace, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return Workspace{}, err
	}
	w := Workspace{Root: root}
	if fsutil.Exists(w.ConfigFile()) {
		return w, failure.Newf(failure.ExitFailure, "use a different directory or run `mufradat -C "+root+" status`",
			"%s is already a workspace", root)
	}
	for _, dir := range []string{w.Root, w.AudioDir(), w.StateDir(), w.OutDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return w, err
		}
	}
	tree := map[string]any{
		"schema":    int64(config.SchemaVersion),
		"deck_name": config.DefaultDeckName,
		"anki": map[string]any{
			"deck_id":  opts.DeckID,
			"model_id": opts.DeckID + 1,
		},
	}
	if err := config.WriteNew(w.ConfigFile(), tree); err != nil {
		return w, err
	}
	if opts.Git {
		ignore := filepath.Join(w.Root, ".gitignore")
		if !fsutil.Exists(ignore) {
			if err := fsutil.WriteFileAtomic(ignore, []byte("state/\nout/\n"), 0o644); err != nil {
				return w, err
			}
		}
	}
	return w, nil
}
