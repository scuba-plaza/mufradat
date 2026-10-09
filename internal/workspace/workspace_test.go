package workspace

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/failure"
)

func makeWorkspace(t *testing.T, root string) Workspace {
	t.Helper()
	w, err := Init(InitOptions{Root: root, DeckID: 1000})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func noEnv(string) string { return "" }

func TestResolvePrecedence(t *testing.T) {
	base := t.TempDir()
	flagWS := makeWorkspace(t, filepath.Join(base, "flag"))
	envWS := makeWorkspace(t, filepath.Join(base, "env"))
	cwdWS := makeWorkspace(t, filepath.Join(base, "cwd"))
	cfgWS := makeWorkspace(t, filepath.Join(base, "cfg"))
	defWS := makeWorkspace(t, filepath.Join(base, "def"))
	nested := filepath.Join(cwdWS.Root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	getenv := func(k string) string {
		if k == EnvVar {
			return envWS.Root
		}
		return ""
	}
	cases := []struct {
		name string
		q    Query
		want Workspace
		src  Source
	}{
		{"flag wins", Query{Flag: flagWS.Root, Getenv: getenv, Cwd: nested, UserConfigValue: cfgWS.Root, DefaultRoot: defWS.Root}, flagWS, FromFlag},
		{"env beats cwd", Query{Getenv: getenv, Cwd: nested, UserConfigValue: cfgWS.Root, DefaultRoot: defWS.Root}, envWS, FromEnv},
		{"cwd walks upward", Query{Getenv: noEnv, Cwd: nested, UserConfigValue: cfgWS.Root, DefaultRoot: defWS.Root}, cwdWS, FromCwd},
		{"user config", Query{Getenv: noEnv, Cwd: base, UserConfigValue: cfgWS.Root, DefaultRoot: defWS.Root}, cfgWS, FromConfig},
		{"default", Query{Getenv: noEnv, Cwd: base, DefaultRoot: defWS.Root}, defWS, FromDefault},
	}
	for _, c := range cases {
		got, err := Resolve(c.q)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.Workspace != c.want || got.Source != c.src {
			t.Errorf("%s: got %+v, want %+v from %s", c.name, got, c.want, c.src)
		}
	}
}

func TestResolveWithoutWorkspaceSuggestsInit(t *testing.T) {
	base := t.TempDir()
	_, err := Resolve(Query{Getenv: noEnv, Cwd: base, DefaultRoot: filepath.Join(base, "none")})
	if err == nil {
		t.Fatal("expected an error")
	}
	if failure.Code(err) != failure.ExitNotSetUp {
		t.Errorf("exit code = %d", failure.Code(err))
	}
	if failure.Hint(err) == "" {
		t.Error("expected an init hint")
	}
}

func TestInitCreatesLayoutAndRefusesTwice(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	w, err := Init(InitOptions{Root: root, Git: true, DeckID: 42})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{w.ConfigFile(), w.AudioDir(), w.StateDir(), w.OutDir(), filepath.Join(root, ".gitignore")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s", p)
		}
	}
	if _, err := Init(InitOptions{Root: root, DeckID: 43}); err == nil {
		t.Error("second init must fail")
	}
}

func TestLockSerialisesWriters(t *testing.T) {
	w := makeWorkspace(t, filepath.Join(t.TempDir(), "ws"))
	var mu sync.Mutex
	active, maxActive, total := 0, 0, 0
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := w.Lock(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			active++
			maxActive = max(maxActive, active)
			total++
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
			unlock()
		}()
	}
	wg.Wait()
	if maxActive != 1 || total != 8 {
		t.Errorf("maxActive=%d total=%d", maxActive, total)
	}
}

func TestLockHonoursContext(t *testing.T) {
	w := makeWorkspace(t, filepath.Join(t.TempDir(), "ws"))
	unlock, err := w.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := w.Lock(ctx); err == nil {
		t.Fatal("second lock should time out")
	}
}
