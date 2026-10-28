package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/failure"
)

func TestInterruptingARunningCommandExitsWith130AndSaysSo(t *testing.T) {
	arrived := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(arrived) })
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	e := env.Env{
		Getenv: func(k string) string {
			if k == "MISTRAL_API_KEY" {
				return "secret-key"
			}
			return ""
		},
		Now:    time.Now,
		Home:   home,
		Stdin:  bytes.NewReader(nil),
		Stdout: &stdout,
		Stderr: &stderr,
		Cwd:    func() (string, error) { return home, nil },
	}
	ws := filepath.Join(home, "ws")
	for _, args := range [][]string{{"init", ws}, {"-C", ws, "config", "set", "--user", "mistral.endpoint", srv.URL}} {
		if code := Execute(context.Background(), e, args); code != 0 {
			t.Fatalf("%v exited with %d: %s", args, code, stderr.String())
		}
	}
	stdout.Reset()
	stderr.Reset()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- Execute(ctx, e, []string{"-C", ws, "add", "1"}) }()
	select {
	case <-arrived:
	case <-time.After(10 * time.Second):
		t.Fatal("the command never reached the service")
	}
	cancel()
	select {
	case code := <-done:
		if code != failure.ExitInterrupted {
			t.Errorf("exit code %d, want %d; stderr: %s", code, failure.ExitInterrupted, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the command ignored the interruption")
	}
	if got, _ := io.ReadAll(&stdout); len(got) != 0 {
		t.Errorf("an interrupted run printed %q on stdout", got)
	}
}

func TestAnUnrelatedFailureKeepsItsOwnExitCodeEvenWhenTheContextIsCancelled(t *testing.T) {
	home := t.TempDir()
	var stderr bytes.Buffer
	e := env.Env{
		Getenv: func(string) string { return "" },
		Now:    time.Now,
		Home:   home,
		Stdout: io.Discard,
		Stderr: &stderr,
		Cwd:    func() (string, error) { return home, nil },
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := Execute(ctx, e, []string{"bogus"}); code != failure.ExitUsage {
		t.Errorf("exit code %d, want %d (usage): only a cancelled operation is an interruption", code, failure.ExitUsage)
	}
}

func TestErrorsAreColouredOnlyOnATerminalThatAllowsIt(t *testing.T) {
	run := func(tty bool, vars map[string]string, args ...string) string {
		home := t.TempDir()
		var stderr bytes.Buffer
		e := env.Env{
			Getenv:    func(k string) string { return vars[k] },
			Now:       time.Now,
			Home:      home,
			Stdout:    io.Discard,
			Stderr:    &stderr,
			StderrTTY: tty,
			Cwd:       func() (string, error) { return home, nil },
		}
		if code := Execute(context.Background(), e, args); code == 0 {
			t.Fatalf("%v succeeded", args)
		}
		return stderr.String()
	}
	const plain, coloured = "error:", "\x1b[31merror:\x1b[0m"
	failing := []string{"config", "get", "no.such.key"}
	for _, c := range []struct {
		name  string
		tty   bool
		vars  map[string]string
		flags []string
		want  string
	}{
		{"piped output is plain", false, nil, nil, plain},
		{"a terminal gets colour", true, nil, nil, coloured},
		{"--no-color turns it off", true, nil, []string{"--no-color"}, plain},
		{"NO_COLOR turns it off", true, map[string]string{"NO_COLOR": "1"}, nil, plain},
		{"a dumb terminal gets none", true, map[string]string{"TERM": "dumb"}, nil, plain},
		{"another terminal type keeps it", true, map[string]string{"TERM": "xterm-256color"}, nil, coloured},
	} {
		if got := run(c.tty, c.vars, append(c.flags, failing...)...); !strings.HasPrefix(got, c.want+" ") {
			t.Errorf("%s: stderr %q, want it to start with %q", c.name, got, c.want)
		}
	}
}

func TestAMistypedCommandIsReportedOnceWithoutCobrasUsageDump(t *testing.T) {
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	e := env.Env{
		Getenv: func(string) string { return "" },
		Now:    time.Now,
		Home:   home,
		Stdout: &stdout,
		Stderr: &stderr,
		Cwd:    func() (string, error) { return home, nil },
	}
	if code := Execute(context.Background(), e, []string{"bogus"}); code != failure.ExitUsage {
		t.Errorf("exit code %d", code)
	}
	if want := "error: unknown command \"bogus\" for \"mufradat\"\n"; stderr.String() != want || stdout.Len() != 0 {
		t.Errorf("stderr %q, stdout %q; want exactly %q on stderr", stderr.String(), stdout.String(), want)
	}
}
