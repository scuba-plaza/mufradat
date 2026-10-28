package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
