package pyenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/proc"
)

const (
	tailSize     = 4096
	libraryHint  = "this system keeps C++ libraries outside the default search path (NixOS does): run mufradat from its Nix package, which sets LD_LIBRARY_PATH, or point LD_LIBRARY_PATH at the directories holding libstdc++.so.6 and libz.so.1"
	pythonOrder  = "python3.13 python3.12 python3.11 python3"
	installerUV  = "uv"
	installerPip = "pip"
)

type SetupOptions struct {
	Python string
	Force  bool
	Step   func(message string)
	Output io.Writer
}

type tail struct {
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > tailSize {
		t.buf = t.buf[len(t.buf)-tailSize:]
	}
	return len(p), nil
}

func (t *tail) String() string { return strings.TrimSpace(string(t.buf)) }

func Diagnose(output string) string {
	if strings.Contains(output, "libstdc++.so.6") || strings.Contains(output, "libz.so.1") {
		return libraryHint
	}
	return ""
}

func (m *Manager) step(opts SetupOptions, message string) {
	if opts.Step != nil {
		opts.Step(message)
	}
}

func (m *Manager) exec(ctx context.Context, opts SetupOptions, c proc.Cmd) (string, error) {
	var t tail
	var out bytes.Buffer
	sinks := []io.Writer{&t}
	if opts.Output != nil {
		sinks = append(sinks, opts.Output)
	}
	c.Stderr = io.MultiWriter(sinks...)
	if c.Stdout == nil {
		c.Stdout = &out
	}
	if err := m.Runner.Run(ctx, c); err != nil {
		detail := t.String()
		if hint := Diagnose(detail); hint != "" {
			return out.String(), failure.New(failure.ExitFailure, hint, fmt.Errorf("%s failed: %s", filepath.Base(c.Name), lastLines(detail, 6)))
		}
		return out.String(), fmt.Errorf("%s failed: %w\n%s", filepath.Base(c.Name), err, lastLines(detail, 12))
	}
	return out.String(), nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func parseVersion(s string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	return major, minor, err1 == nil && err2 == nil
}

func (m *Manager) FindPython(ctx context.Context, wanted string) (string, string, error) {
	candidates := strings.Fields(pythonOrder)
	if wanted != "" {
		candidates = []string{wanted}
	}
	var problems []string
	for _, name := range candidates {
		path, err := m.Runner.LookPath(name)
		if err != nil {
			problems = append(problems, name+": not found")
			continue
		}
		var out bytes.Buffer
		if err := m.Runner.Run(ctx, proc.Cmd{Name: path, Args: []string{"-c", probeVersionPy}, Stdout: &out}); err != nil {
			problems = append(problems, name+": does not run")
			continue
		}
		major, minor, ok := parseVersion(out.String())
		if !ok || major < MinPythonMajor || major == MinPythonMajor && minor < MinPythonMinor {
			problems = append(problems, fmt.Sprintf("%s: version %s is older than %d.%d", name, strings.TrimSpace(out.String()), MinPythonMajor, MinPythonMinor))
			continue
		}
		return path, strings.TrimSpace(out.String()), nil
	}
	return "", "", failure.New(failure.ExitNotSetUp,
		fmt.Sprintf("install Python %d.%d or newer, or pass one with --python", MinPythonMajor, MinPythonMinor),
		fmt.Errorf("no suitable Python found (%s)", strings.Join(problems, "; ")))
}

type SelftestResult struct {
	OK       bool               `json:"ok"`
	Problems []string           `json:"problems"`
	Versions map[string]*string `json:"versions"`
}

func (r SelftestResult) Version(name string) string {
	if v := r.Versions[name]; v != nil {
		return *v
	}
	return "?"
}

func parseSelftest(stdout string) (SelftestResult, error) {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var r SelftestResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return SelftestResult{}, fmt.Errorf("unreadable selftest result: %w", err)
		}
		return r, nil
	}
	return SelftestResult{}, errors.New("the selftest printed no result")
}

func (m *Manager) Setup(ctx context.Context, opts SetupOptions) (Ready, error) {
	digest, err := m.LockDigest()
	if err != nil {
		return Ready{}, err
	}
	envDir, err := m.EnvDir()
	if err != nil {
		return Ready{}, err
	}
	if existing, ok, err := m.ReadyState(); err != nil {
		return Ready{}, err
	} else if ok && !opts.Force {
		m.step(opts, "the environment is already installed; checking it")
		if _, err := m.selftest(ctx, opts); err != nil {
			return Ready{}, err
		}
		return existing, nil
	}

	m.step(opts, "looking for Python")
	python, version, err := m.FindPython(ctx, opts.Python)
	if err != nil {
		return Ready{}, err
	}
	if err := os.RemoveAll(envDir); err != nil {
		return Ready{}, err
	}
	if err := os.MkdirAll(filepath.Dir(envDir), 0o755); err != nil {
		return Ready{}, err
	}
	lock, err := m.read(lockName)
	if err != nil {
		return Ready{}, err
	}
	lockPath := filepath.Join(filepath.Dir(envDir), digest+".lock")
	if err := fsutil.WriteFileAtomic(lockPath, lock, 0o644); err != nil {
		return Ready{}, err
	}

	installer, err := m.install(ctx, opts, python, envDir, lockPath)
	if err != nil {
		_ = os.RemoveAll(envDir)
		return Ready{}, err
	}
	if err := m.fetchData(ctx, opts, envDir); err != nil {
		return Ready{}, err
	}
	m.step(opts, "running a test check (the first run also fetches the CATT model)")
	if _, err := m.selftest(ctx, opts); err != nil {
		return Ready{}, err
	}
	ready := Ready{
		Schema:        readySchema,
		LockDigest:    digest,
		PythonVersion: version,
		CamelPackages: camelNames(),
		Installer:     installer,
		Created:       m.Now().UTC(),
	}
	raw, err := json.MarshalIndent(ready, "", "  ")
	if err != nil {
		return Ready{}, err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(envDir, readyFile), append(raw, '\n'), 0o644); err != nil {
		return Ready{}, err
	}
	_ = os.Remove(lockPath)
	srcDigest, err := m.SourceDigest()
	if err != nil {
		return Ready{}, err
	}
	m.pruneOthers(digest, srcDigest)
	return ready, nil
}

func (m *Manager) install(ctx context.Context, opts SetupOptions, python, envDir, lockPath string) (string, error) {
	venvPython := filepath.Join(envDir, "bin", "python")
	if uv, err := m.Runner.LookPath("uv"); err == nil {
		m.step(opts, "creating the environment with uv")
		if _, err := m.exec(ctx, opts, proc.Cmd{Name: uv, Args: []string{"venv", "--python", python, envDir}}); err != nil {
			return "", err
		}
		m.step(opts, "installing the pinned packages (about 1.4 GB)")
		_, err := m.exec(ctx, opts, proc.Cmd{Name: uv, Args: []string{
			"pip", "install", "--python", venvPython,
			"--index-strategy", "unsafe-best-match", "--require-hashes", "--no-deps", "-r", lockPath,
		}})
		return installerUV, err
	}
	m.step(opts, "creating the environment with venv")
	if _, err := m.exec(ctx, opts, proc.Cmd{Name: python, Args: []string{"-m", "venv", envDir}}); err != nil {
		return "", err
	}
	m.step(opts, "installing the pinned packages (about 1.4 GB)")
	_, err := m.exec(ctx, opts, proc.Cmd{Name: venvPython, Args: []string{
		"-m", "pip", "install", "--disable-pip-version-check", "--require-hashes", "--no-deps", "-r", lockPath,
	}})
	return installerPip, err
}

func (m *Manager) fetchData(ctx context.Context, opts SetupOptions, envDir string) error {
	data := m.Dirs.CamelData()
	for _, d := range []string{data, filepath.Join(m.Dirs.ModelCache(), "hf"), filepath.Join(m.Dirs.ModelCache(), "torch")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	tool := filepath.Join(envDir, "bin", "camel_data")
	for _, p := range CamelPackages {
		if fsutil.IsDir(filepath.Join(data, "data", filepath.FromSlash(p.Dir))) {
			continue
		}
		m.step(opts, "downloading CAMeL data: "+p.Name)
		root, err := m.ExtractSidecar()
		if err != nil {
			return err
		}
		if _, err := m.exec(ctx, opts, proc.Cmd{Name: tool, Args: []string{"-i", p.Name}, Env: m.Environ(root)}); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) managedRuntime() (Runtime, error) {
	envDir, err := m.EnvDir()
	if err != nil {
		return Runtime{}, err
	}
	return Runtime{Python: filepath.Join(envDir, "bin", "python"), Managed: true}, nil
}

func (m *Manager) selftest(ctx context.Context, opts SetupOptions) (SelftestResult, error) {
	rt, err := m.managedRuntime()
	if err != nil {
		return SelftestResult{}, err
	}
	return m.Selftest(ctx, rt, opts)
}

func (m *Manager) Selftest(ctx context.Context, rt Runtime, opts SetupOptions) (SelftestResult, error) {
	return m.runCommand(ctx, rt, "selftest", opts)
}

func (m *Manager) Probe(ctx context.Context, rt Runtime, opts SetupOptions) (SelftestResult, error) {
	return m.runCommand(ctx, rt, "probe", opts)
}

func (m *Manager) runCommand(ctx context.Context, rt Runtime, command string, opts SetupOptions) (SelftestResult, error) {
	root, err := m.ExtractSidecar()
	if err != nil {
		return SelftestResult{}, err
	}
	stdout, err := m.exec(ctx, opts, proc.Cmd{
		Name: rt.Python,
		Args: []string{"-u", "-m", sidecarPackage, command},
		Env:  m.Environ(root),
	})
	result, perr := parseSelftest(stdout)
	if err != nil {
		if perr == nil && len(result.Problems) > 0 {
			return result, problemError(command, result.Problems)
		}
		return SelftestResult{}, err
	}
	if perr != nil {
		return result, perr
	}
	if !result.OK {
		return result, problemError(command, result.Problems)
	}
	return result, nil
}

const maxProblems = 4

func summarize(problem string) string {
	var lines []string
	for _, l := range strings.Split(problem, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) < 2 {
		return strings.TrimSpace(problem)
	}
	module, _, _ := strings.Cut(lines[0], ":")
	return module + ": " + strings.TrimPrefix(lines[len(lines)-1], "Original error was: ")
}

func problemError(command string, problems []string) error {
	var shown []string
	for _, p := range problems {
		if s := summarize(p); !slices.Contains(shown, s) {
			shown = append(shown, s)
		}
	}
	if len(shown) > maxProblems {
		shown = append(shown[:maxProblems], fmt.Sprintf("and %d more", len(shown)-maxProblems))
	}
	err := fmt.Errorf("the %s failed: %s", command, strings.Join(shown, "; "))
	if hint := Diagnose(strings.Join(problems, "\n")); hint != "" {
		return failure.New(failure.ExitFailure, hint, err)
	}
	return err
}
