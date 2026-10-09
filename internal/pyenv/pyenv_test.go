package pyenv

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/proc"
	"github.com/scuba-plaza/mufradat/internal/xdg"
)

type fakeRunner struct {
	paths   map[string]string
	calls   []proc.Cmd
	onRun   func(c proc.Cmd) error
	started []proc.Cmd
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if p, ok := f.paths[name]; ok {
		return p, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(_ context.Context, c proc.Cmd) error {
	f.calls = append(f.calls, c)
	if f.onRun != nil {
		return f.onRun(c)
	}
	return nil
}

func (f *fakeRunner) Start(_ context.Context, c proc.Cmd) (proc.Process, error) {
	f.started = append(f.started, c)
	return nil, errors.New("not scripted")
}

func (f *fakeRunner) names() []string {
	var out []string
	for _, c := range f.calls {
		out = append(out, filepath.Base(c.Name)+" "+firstArgs(c.Args))
	}
	return out
}

func firstArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func source(lock string) fstest.MapFS {
	return fstest.MapFS{
		"requirements.lock":             {Data: []byte(lock)},
		"mufradat_sidecar/__init__.py":  {Data: []byte("PROTOCOL = 1\n")},
		"mufradat_sidecar/__main__.py":  {Data: []byte("print('main')\n")},
		"mufradat_sidecar/not_python.x": {Data: []byte("ignored")},
	}
}

func manager(t *testing.T, src fstest.MapFS, r *fakeRunner) *Manager {
	t.Helper()
	root := t.TempDir()
	env := map[string]string{"XDG_DATA_HOME": filepath.Join(root, "data"), "XDG_CONFIG_HOME": filepath.Join(root, "config"), "XDG_CACHE_HOME": filepath.Join(root, "cache"), "XDG_STATE_HOME": filepath.Join(root, "state")}
	return &Manager{
		Dirs:   xdg.New(func(k string) string { return env[k] }, root),
		Source: src,
		Runner: r,
		Now:    func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) },
	}
}

func healthy(m *Manager) func(c proc.Cmd) error {
	return func(c proc.Cmd) error {
		base := filepath.Base(c.Name)
		switch {
		case base == "python3.13" && len(c.Args) > 0 && c.Args[0] == "-c":
			_, _ = io.WriteString(c.Stdout, "3.13.1\n")
		case base == "uv" && len(c.Args) > 0 && c.Args[0] == "venv":
			return fakeVenv(c.Args[len(c.Args)-1])
		case base == "camel_data":
			name := c.Args[1]
			for _, p := range CamelPackages {
				if p.Name == name {
					return os.MkdirAll(filepath.Join(m.Dirs.CamelData(), "data", filepath.FromSlash(p.Dir)), 0o755)
				}
			}
		case len(c.Args) > 3 && c.Args[len(c.Args)-1] == "selftest":
			_, _ = io.WriteString(c.Stdout, "noise\n{\"ok\": true, \"problems\": []}\n")
		}
		return nil
	}
}

func fakeVenv(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "bin", "python"), nil, 0o755)
}

func installPython(m *Manager, r *fakeRunner) {
	r.paths = map[string]string{"python3.13": "/usr/bin/python3.13", "uv": "/usr/bin/uv"}
	r.onRun = healthy(m)
}

func TestDigestsFollowContent(t *testing.T) {
	a := manager(t, source("one"), &fakeRunner{})
	b := manager(t, source("one"), &fakeRunner{})
	c := manager(t, source("two"), &fakeRunner{})
	da, _ := a.LockDigest()
	db, _ := b.LockDigest()
	dc, _ := c.LockDigest()
	if da != db || da == dc || len(da) != 12 {
		t.Fatalf("lock digests %s %s %s", da, db, dc)
	}
	changed := source("one")
	changed["mufradat_sidecar/__main__.py"] = &fstest.MapFile{Data: []byte("print('other')\n")}
	sa, _ := a.SourceDigest()
	sb, _ := manager(t, changed, &fakeRunner{}).SourceDigest()
	if sa == sb {
		t.Fatal("the sidecar digest ignored a source change")
	}
	withExtra := source("one")
	withExtra["mufradat_sidecar/notes.txt"] = &fstest.MapFile{Data: []byte("x")}
	se, _ := manager(t, withExtra, &fakeRunner{}).SourceDigest()
	if sa != se {
		t.Fatal("non-python files must not change the sidecar digest")
	}
}

func TestExtractSidecarIsIdempotentAndRepairs(t *testing.T) {
	m := manager(t, source("one"), &fakeRunner{})
	root, err := m.ExtractSidecar()
	if err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(root, "mufradat_sidecar", "__main__.py")
	if got, _ := os.ReadFile(main); string(got) != "print('main')\n" {
		t.Fatalf("content = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "mufradat_sidecar", "not_python.x")); err == nil {
		t.Fatal("a non-python file was extracted")
	}
	if err := os.WriteFile(main, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := m.ExtractSidecar()
	if err != nil || again != root {
		t.Fatalf("second extract: %v %s", err, again)
	}
	if got, _ := os.ReadFile(main); string(got) != "print('main')\n" {
		t.Fatalf("tampered file was not repaired: %q", got)
	}
}

func TestResolveBeforeSetupSaysSetup(t *testing.T) {
	m := manager(t, source("one"), &fakeRunner{})
	_, err := m.Resolve("")
	if failure.Code(err) != failure.ExitNotSetUp || !strings.Contains(failure.Hint(err), "setup") {
		t.Fatalf("err = %v (code %d, hint %q)", err, failure.Code(err), failure.Hint(err))
	}
}

func TestResolveCustomPython(t *testing.T) {
	r := &fakeRunner{paths: map[string]string{"mypy": "/opt/py/bin/python"}}
	m := manager(t, source("one"), r)
	rt, err := m.Resolve("mypy")
	if err != nil || rt.Managed || rt.Python != "/opt/py/bin/python" || rt.Fingerprint == "" {
		t.Fatalf("rt=%+v err=%v", rt, err)
	}
	if _, err := m.Resolve("missing"); failure.Code(err) != failure.ExitNotSetUp {
		t.Fatalf("err = %v", err)
	}
	r.paths["other"] = "/opt/other/bin/python"
	other, _ := m.Resolve("other")
	if other.Fingerprint == rt.Fingerprint {
		t.Fatal("different interpreters must not share readings")
	}
}

func TestSetupRunsEveryStepInOrder(t *testing.T) {
	r := &fakeRunner{}
	m := manager(t, source("one"), r)
	installPython(m, r)
	var steps []string
	ready, err := m.Setup(context.Background(), SetupOptions{Step: func(s string) { steps = append(steps, s) }})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"python3.13 -c",
		"uv venv",
		"uv pip",
		"camel_data -i", "camel_data -i", "camel_data -i",
		"python -u",
	}
	if got := r.names(); !slices.Equal(got, want) {
		t.Fatalf("commands:\n got %v\nwant %v", got, want)
	}
	install := r.calls[2]
	for _, flag := range []string{"--require-hashes", "--no-deps", "unsafe-best-match"} {
		if !slices.Contains(install.Args, flag) {
			t.Errorf("uv pip install lacks %s: %v", flag, install.Args)
		}
	}
	for _, c := range r.calls[3:6] {
		if !slices.Contains(c.Env, "CAMELTOOLS_DATA="+m.Dirs.CamelData()) {
			t.Errorf("camel_data runs without CAMELTOOLS_DATA: %v", c.Env)
		}
	}
	if len(steps) < 5 {
		t.Fatalf("too few progress steps: %v", steps)
	}
	if ready.PythonVersion != "3.13.1" || ready.Installer != "uv" || !slices.Equal(ready.CamelPackages, camelNames()) {
		t.Fatalf("ready = %+v", ready)
	}
	rt, err := m.Resolve("")
	if err != nil || !rt.Managed || !strings.HasSuffix(rt.Python, filepath.Join("bin", "python")) {
		t.Fatalf("rt=%+v err=%v", rt, err)
	}
	again, _ := m.Resolve("")
	if again.Fingerprint != rt.Fingerprint {
		t.Fatal("the fingerprint is not stable")
	}
}

func TestSecondSetupOnlyRunsTheSelftest(t *testing.T) {
	r := &fakeRunner{}
	m := manager(t, source("one"), r)
	installPython(m, r)
	if _, err := m.Setup(context.Background(), SetupOptions{}); err != nil {
		t.Fatal(err)
	}
	r.calls = nil
	if _, err := m.Setup(context.Background(), SetupOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := r.names(); len(got) != 1 || !strings.HasPrefix(got[0], "python -u") {
		t.Fatalf("commands = %v", got)
	}
	r.calls = nil
	if _, err := m.Setup(context.Background(), SetupOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	if got := r.names(); !slices.Contains(got, "uv venv") || !slices.Contains(got, "uv pip") {
		t.Fatalf("--force did not rebuild: %v", got)
	}
	for _, name := range r.names() {
		if strings.HasPrefix(name, "camel_data") {
			t.Fatalf("--force downloaded the CAMeL data again: %v", r.names())
		}
	}
}

func TestSetupFallsBackToVenvAndPip(t *testing.T) {
	r := &fakeRunner{}
	m := manager(t, source("one"), r)
	installPython(m, r)
	delete(r.paths, "uv")
	inner := r.onRun
	r.onRun = func(c proc.Cmd) error {
		if filepath.Base(c.Name) == "python3.13" && len(c.Args) > 1 && c.Args[0] == "-m" && c.Args[1] == "venv" {
			return fakeVenv(c.Args[2])
		}
		return inner(c)
	}
	ready, err := m.Setup(context.Background(), SetupOptions{})
	if err != nil || ready.Installer != "pip" {
		t.Fatalf("ready=%+v err=%v", ready, err)
	}
	if got := r.names(); got[1] != "python3.13 -m" || got[2] != "python -m" {
		t.Fatalf("commands = %v", got)
	}
}

func TestSetupRejectsOldOrMissingPython(t *testing.T) {
	r := &fakeRunner{paths: map[string]string{"python3": "/usr/bin/python3"}}
	m := manager(t, source("one"), r)
	r.onRun = func(c proc.Cmd) error {
		_, _ = io.WriteString(c.Stdout, "3.9.2\n")
		return nil
	}
	_, err := m.Setup(context.Background(), SetupOptions{})
	if failure.Code(err) != failure.ExitNotSetUp || !strings.Contains(err.Error(), "older than 3.11") {
		t.Fatalf("err = %v", err)
	}
	r.paths = nil
	_, err = m.Setup(context.Background(), SetupOptions{})
	if failure.Code(err) != failure.ExitNotSetUp || !strings.Contains(err.Error(), "no suitable Python") {
		t.Fatalf("err = %v", err)
	}
}

func TestSetupExplainsMissingSystemLibraries(t *testing.T) {
	r := &fakeRunner{}
	m := manager(t, source("one"), r)
	installPython(m, r)
	inner := r.onRun
	r.onRun = func(c proc.Cmd) error {
		if len(c.Args) > 3 && c.Args[len(c.Args)-1] == "selftest" {
			_, _ = io.WriteString(c.Stderr, "ImportError: libstdc++.so.6: cannot open shared object file\n")
			return &proc.ExitError{Name: "python", Code: 1}
		}
		return inner(c)
	}
	_, err := m.Setup(context.Background(), SetupOptions{})
	if err == nil || !strings.Contains(failure.Hint(err), "LD_LIBRARY_PATH") || !strings.Contains(err.Error(), "libstdc++") {
		t.Fatalf("err = %v, hint = %q", err, failure.Hint(err))
	}
	if _, ok, _ := m.ReadyState(); ok {
		t.Fatal("a failed setup must not leave a READY marker")
	}
}

func TestSetupReportsSelftestProblems(t *testing.T) {
	r := &fakeRunner{}
	m := manager(t, source("one"), r)
	installPython(m, r)
	inner := r.onRun
	r.onRun = func(c proc.Cmd) error {
		if len(c.Args) > 3 && c.Args[len(c.Args)-1] == "selftest" {
			_, _ = io.WriteString(c.Stdout, "{\"ok\": false, \"problems\": [\"CATT returned no vowelled reading\"]}\n")
			return nil
		}
		return inner(c)
	}
	_, err := m.Setup(context.Background(), SetupOptions{})
	if err == nil || !strings.Contains(err.Error(), "CATT returned no vowelled reading") {
		t.Fatalf("err = %v", err)
	}
	if _, ok, _ := m.ReadyState(); ok {
		t.Fatal("a failed selftest must not leave a READY marker")
	}
}

func TestSetupPrunesOlderEnvironments(t *testing.T) {
	r := &fakeRunner{}
	m := manager(t, source("one"), r)
	installPython(m, r)
	oldEnv := filepath.Join(m.Dirs.PythonEnvs(), "0123456789ab")
	oldSidecar := filepath.Join(m.Dirs.Sidecar(), "ba9876543210")
	unrelated := filepath.Join(m.Dirs.PythonEnvs(), "keep-me")
	for _, d := range []string{oldEnv, oldSidecar, unrelated} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.Setup(context.Background(), SetupOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{oldEnv, oldSidecar} {
		if _, err := os.Stat(d); err == nil {
			t.Errorf("%s was not pruned", d)
		}
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("a directory that is not ours was removed: %v", err)
	}
	cur, _ := m.EnvDir()
	if _, err := os.Stat(cur); err != nil {
		t.Errorf("the current environment was removed: %v", err)
	}
}

func TestLauncherStartsTheSidecarModule(t *testing.T) {
	r := &fakeRunner{}
	m := manager(t, source("one"), r)
	installPython(m, r)
	if _, err := m.Setup(context.Background(), SetupOptions{}); err != nil {
		t.Fatal(err)
	}
	rt, _ := m.Resolve("")
	root, _ := m.ExtractSidecar()
	_, _ = m.Launcher(rt)(context.Background(), io.Discard)
	if len(r.started) != 1 {
		t.Fatalf("started %d processes", len(r.started))
	}
	c := r.started[0]
	if !slices.Equal(c.Args, []string{"-u", "-m", "mufradat_sidecar"}) || c.Name != rt.Python {
		t.Fatalf("cmd = %+v", c)
	}
	for _, want := range []string{"PYTHONPATH=" + root, "PYTHONNOUSERSITE=1", "CAMELTOOLS_DATA=" + m.Dirs.CamelData(), "TOKENIZERS_PARALLELISM=false"} {
		if !slices.Contains(c.Env, want) {
			t.Errorf("environment lacks %s", want)
		}
	}
}

func TestProbeThatExitsWithProblemsOnStdoutExplainsMissingLibraries(t *testing.T) {
	r := &fakeRunner{}
	m := manager(t, source("one"), r)
	installPython(m, r)
	if _, err := m.Setup(context.Background(), SetupOptions{}); err != nil {
		t.Fatal(err)
	}
	numpy := "ImportError: \n\nIMPORTANT: PLEASE READ THIS\n\nOriginal error was: libstdc++.so.6: cannot open shared object file: No such file or directory\n"
	report, _ := json.Marshal(map[string]any{"ok": false, "problems": []string{"numpy: " + numpy, "torch: ImportError: libstdc++.so.6: cannot open shared object file", "onnxruntime: ImportError: libstdc++.so.6: cannot open shared object file"}})
	r.onRun = func(c proc.Cmd) error {
		_, _ = io.WriteString(c.Stdout, string(report)+"\n")
		return &proc.ExitError{Name: "python", Code: 1}
	}
	rt, err := m.Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Probe(context.Background(), rt, SetupOptions{})
	if err == nil || !strings.Contains(failure.Hint(err), "LD_LIBRARY_PATH") {
		t.Fatalf("err = %v, hint = %q", err, failure.Hint(err))
	}
	if msg := err.Error(); !strings.Contains(msg, "numpy: libstdc++.so.6: cannot open shared object file") || strings.Contains(msg, "IMPORTANT") || strings.Count(msg, "libstdc++.so.6: cannot open shared object file") != 3 {
		t.Errorf("each problem is summarised by its last line: %q", msg)
	}
}

func TestProbeThatExitsWithoutAReportKeepsTheProcessError(t *testing.T) {
	r := &fakeRunner{}
	m := manager(t, source("one"), r)
	installPython(m, r)
	if _, err := m.Setup(context.Background(), SetupOptions{}); err != nil {
		t.Fatal(err)
	}
	r.onRun = func(c proc.Cmd) error {
		_, _ = io.WriteString(c.Stderr, "Traceback: boom\n")
		return &proc.ExitError{Name: "python", Code: 1}
	}
	rt, _ := m.Resolve("")
	_, err := m.Probe(context.Background(), rt, SetupOptions{})
	if err == nil || !strings.Contains(err.Error(), "Traceback: boom") || failure.Hint(err) != "" {
		t.Fatalf("err = %v, hint = %q", err, failure.Hint(err))
	}
}
