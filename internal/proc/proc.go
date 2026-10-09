package proc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Cmd struct {
	Name   string
	Args   []string
	Dir    string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Process interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Wait() error
	Stop()
}

type Runner interface {
	Run(ctx context.Context, c Cmd) error
	Start(ctx context.Context, c Cmd) (Process, error)
	LookPath(name string) (string, error)
}

var ErrNotFound = exec.ErrNotFound

type ExitError struct {
	Name     string
	Code     int
	Stderr   string
	Signaled bool
}

func (e *ExitError) Error() string {
	switch {
	case e.Signaled:
		return fmt.Sprintf("%s was killed", e.Name)
	case e.Stderr != "":
		return fmt.Sprintf("%s exited with status %d: %s", e.Name, e.Code, e.Stderr)
	}
	return fmt.Sprintf("%s exited with status %d", e.Name, e.Code)
}

func IsExit(err error) (*ExitError, bool) {
	var e *ExitError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

type OS struct {
	BaseEnv []string
}

func NewOS() OS { return OS{BaseEnv: os.Environ()} }

func (r OS) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (r OS) command(ctx context.Context, c Cmd) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(append([]string(nil), r.BaseEnv...), c.Env...)
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

func (r OS) Run(ctx context.Context, c Cmd) error {
	cmd := r.command(ctx, c)
	if err := cmd.Run(); err != nil {
		return wrap(c.Name, err)
	}
	return nil
}

type osProcess struct {
	cmd   *exec.Cmd
	name  string
	stdin io.WriteCloser
	out   io.ReadCloser
}

func (p *osProcess) Stdin() io.WriteCloser { return p.stdin }

func (p *osProcess) Stdout() io.ReadCloser { return p.out }

func (p *osProcess) Wait() error {
	if err := p.cmd.Wait(); err != nil {
		return wrap(p.name, err)
	}
	return nil
}

func (p *osProcess) Stop() {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
	}
}

func (r OS) Start(ctx context.Context, c Cmd) (Process, error) {
	cmd := r.command(ctx, c)
	cmd.Stdin = nil
	cmd.Stdout = nil
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, wrap(c.Name, err)
	}
	return &osProcess{cmd: cmd, name: c.Name, stdin: in, out: out}, nil
}

func wrap(name string, err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		status, _ := ee.Sys().(syscall.WaitStatus)
		return &ExitError{
			Name:     name,
			Code:     ee.ExitCode(),
			Stderr:   string(ee.Stderr),
			Signaled: status.Signaled(),
		}
	}
	return fmt.Errorf("%s: %w", name, err)
}
