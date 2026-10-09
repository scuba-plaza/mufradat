package env

import (
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/scuba-plaza/mufradat/internal/proc"
)

type Env struct {
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	Getenv      func(string) string
	Now         func() time.Time
	Cwd         func() (string, error)
	Home        string
	HTTP        *http.Client
	Runner      proc.Runner
	Interactive bool
	StdoutTTY   bool
	StderrTTY   bool
}

func OS() Env {
	home, _ := os.UserHomeDir()
	return Env{
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Getenv:      os.Getenv,
		Now:         time.Now,
		Cwd:         os.Getwd,
		Home:        home,
		HTTP:        &http.Client{Timeout: 5 * time.Minute},
		Runner:      proc.NewOS(),
		Interactive: term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())),
		StdoutTTY:   term.IsTerminal(int(os.Stdout.Fd())),
		StderrTTY:   term.IsTerminal(int(os.Stderr.Fd())),
	}
}
