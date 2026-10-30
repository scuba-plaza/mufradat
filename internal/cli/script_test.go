package cli_test

import (
	"context"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/scuba-plaza/mufradat/internal/cli"
	"github.com/scuba-plaza/mufradat/internal/env"
)

func exiting(run func() int) func() {
	return func() { os.Exit(run()) }
}

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"mufradat": exiting(func() int {
			return cli.Execute(context.Background(), env.OS(), os.Args[1:])
		}),
		"python3.13": exiting(fakePython),
		"python":     exiting(fakePython),
		"uv":         exiting(fakeUV),
		"camel_data": exiting(fakeCamelData),
		"xdg-open":   exiting(fakeBrowser),
	})
}

func TestScripts(t *testing.T) {
	key := generateKey(t)
	testscript.Run(t, testscript.Params{
		Dir:  "testdata/script",
		Cmds: scriptCommands(),
		Setup: func(e *testscript.Env) error {
			home := filepath.Join(e.WorkDir, "home")
			if err := os.MkdirAll(home, 0o755); err != nil {
				return err
			}
			e.Setenv("HOME", home)
			for _, k := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "MUFRADAT_WORKSPACE", "GOOGLE_APPLICATION_CREDENTIALS", "CLOUDSDK_CONFIG"} {
				e.Setenv(k, "")
			}
			e.Setenv("NO_COLOR", "1")
			e.Setenv("FAKE_SIDECAR_LOG", filepath.Join(e.WorkDir, "sidecar.log"))
			e.Setenv("FAKE_BROWSER_LOG", filepath.Join(e.WorkDir, "browser.log"))

			synth := filepath.Join(e.WorkDir, "synth-calls.log")
			mistral := newFakeMistral(t, filepath.Join(e.WorkDir, "mistral-requests.log"))
			e.Values[mistralKey{}] = mistral
			e.Setenv("MISTRAL_URL", mistral.URL)
			e.Setenv("MISTRAL_API_KEY", fakeMistralKey)

			google := newFakeGoogle(t, synth)
			e.Values[googleKey{}] = google
			e.Setenv("GOOGLE_URL", google.URL)
			writeKey(t, e.WorkDir, key, google.URL+"/token")

			configDir := filepath.Join(home, ".config", "mufradat")
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				return err
			}
			endpoints := "[mistral]\nendpoint = \"" + mistral.URL + "\"\n[google]\nendpoint = \"" + google.URL + "\"\n"
			if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(endpoints), 0o644); err != nil {
				return err
			}

			e.Defer(func() {
				if v := mistral.violations(); v != "" {
					t.Errorf("%s: the CLI sent the fake Mistral server requests that the real API would refuse:\n%s", filepath.Base(e.WorkDir), v)
				}
				if v := google.violations(); v != "" {
					t.Errorf("%s: the CLI sent the fake Google server requests that the real API would refuse:\n%s", filepath.Base(e.WorkDir), v)
				}
			})
			return nil
		},
	})
}

func scriptCommands() map[string]func(ts *testscript.TestScript, neg bool, args []string) {
	cmds := mistralCommands()
	maps.Copy(cmds, googleCommands())
	cmds["exits"] = func(ts *testscript.TestScript, neg bool, args []string) {
		if neg || len(args) < 2 {
			ts.Fatalf("usage: exits CODE command [args...]")
		}
		want, err := strconv.Atoi(args[0])
		if err != nil {
			ts.Fatalf("exits: %v", err)
		}
		got := 0
		if err := ts.Exec(args[1], args[2:]...); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				ts.Fatalf("exits: %v", err)
			}
			got = exit.ExitCode()
		}
		if got != want {
			ts.Fatalf("%s exited with code %d, want %d", args[1], got, want)
		}
	}
	return cmds
}
