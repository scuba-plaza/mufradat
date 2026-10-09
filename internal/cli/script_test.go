package cli_test

import (
	"context"
	"os"
	"path/filepath"
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
		"claude":     exiting(fakeClaude),
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
		Dir: "testdata/script",
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
			e.Setenv("FAKE_CLAUDE_COUNT", filepath.Join(e.WorkDir, "claude-calls"))
			e.Setenv("FAKE_CLAUDE_LOG", filepath.Join(e.WorkDir, "claude-args.log"))
			e.Setenv("FAKE_SIDECAR_LOG", filepath.Join(e.WorkDir, "sidecar.log"))
			e.Setenv("FAKE_BROWSER_LOG", filepath.Join(e.WorkDir, "browser.log"))

			google := newFakeGoogle(t, filepath.Join(e.WorkDir, "synth-calls.log"))
			e.Defer(google.Close)
			e.Setenv("TTS_URL", google.URL)
			writeKey(t, e.WorkDir, key, google.URL+"/token")
			return nil
		},
	})
}
