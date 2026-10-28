package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sh(script string) Cmd { return Cmd{Name: "sh", Args: []string{"-c", script}} }

func TestRunPassesDirectoryEnvironmentAndStandardStreams(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := NewOS()
	r.BaseEnv = append(r.BaseEnv, "MUFRADAT_PROC_A=base", "MUFRADAT_PROC_B=base")
	var out, errOut bytes.Buffer
	c := sh(`pwd; echo "$MUFRADAT_PROC_A $MUFRADAT_PROC_B $MUFRADAT_PROC_C"; cat; echo oops >&2`)
	c.Dir = dir
	c.Env = []string{"MUFRADAT_PROC_B=cmd", "MUFRADAT_PROC_C=cmd"}
	c.Stdin = strings.NewReader("from stdin\n")
	c.Stdout, c.Stderr = &out, &errOut
	if err := r.Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if want := dir + "\nbase cmd cmd\nfrom stdin\n"; out.String() != want {
		t.Errorf("stdout %q, want %q: the command's own variables win over the base environment", out.String(), want)
	}
	if errOut.String() != "oops\n" {
		t.Errorf("stderr %q", errOut.String())
	}
}

func TestRunReportsHowTheCommandFailed(t *testing.T) {
	r := NewOS()
	err := r.Run(context.Background(), sh("exit 3"))
	exit, ok := IsExit(err)
	if !ok || exit.Name != "sh" || exit.Code != 3 || exit.Signaled {
		t.Fatalf("err = %#v", err)
	}
	if err.Error() != "sh exited with status 3" {
		t.Errorf("message %q", err)
	}

	err = r.Run(context.Background(), sh("kill -KILL $$"))
	exit, ok = IsExit(err)
	if !ok || !exit.Signaled {
		t.Fatalf("a killed command must say so: %#v", err)
	}
	if err.Error() != "sh was killed" {
		t.Errorf("message %q", err)
	}

	err = r.Run(context.Background(), Cmd{Name: "mufradat-no-such-program"})
	if _, ok := IsExit(err); ok || !errors.Is(err, ErrNotFound) || !strings.HasPrefix(err.Error(), "mufradat-no-such-program: ") {
		t.Errorf("a missing program is not an exit status: %v", err)
	}
}

func TestExitErrorMessages(t *testing.T) {
	for _, c := range []struct {
		err  ExitError
		want string
	}{
		{ExitError{Name: "ffmpeg", Code: 1}, "ffmpeg exited with status 1"},
		{ExitError{Name: "ffmpeg", Code: 1, Stderr: "no such file"}, "ffmpeg exited with status 1: no such file"},
		{ExitError{Name: "ffmpeg", Code: -1, Signaled: true, Stderr: "ignored"}, "ffmpeg was killed"},
	} {
		if got := c.err.Error(); got != c.want {
			t.Errorf("%+v: %q, want %q", c.err, got, c.want)
		}
	}
}

func TestIsExitFindsTheExitStatusThroughWrapping(t *testing.T) {
	inner := &ExitError{Name: "x", Code: 9}
	got, ok := IsExit(fmt.Errorf("while converting: %w", inner))
	if !ok || got != inner {
		t.Errorf("wrapped exit error: %v, %v", got, ok)
	}
	if got, ok := IsExit(errors.New("plain")); ok || got != nil {
		t.Errorf("a plain error is not an exit error: %v, %v", got, ok)
	}
	if got, ok := IsExit(nil); ok || got != nil {
		t.Errorf("nil: %v, %v", got, ok)
	}
}

func TestCancellingTheContextStopsTheCommandPromptly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := NewOS().Run(ctx, sh("exec sleep 30"))
	exit, ok := IsExit(err)
	if !ok || !exit.Signaled {
		t.Fatalf("err = %#v", err)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Errorf("took %v: the command is asked to stop as soon as the context ends", elapsed)
	}
}

func TestLookPathFindsPrograms(t *testing.T) {
	r := NewOS()
	if path, err := r.LookPath("sh"); err != nil || !filepath.IsAbs(path) {
		t.Errorf("sh: %q, %v", path, err)
	}
	if _, err := r.LookPath("mufradat-no-such-program"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestStartGivesAPipeToTheRunningProgram(t *testing.T) {
	p, err := NewOS().Start(context.Background(), Cmd{Name: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(p.Stdin(), "one\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	if err := p.Stdin().Close(); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(p.Stdout())
	if err != nil || string(got) != "one\ntwo\n" {
		t.Fatalf("read %q, %v", got, err)
	}
	if err := p.Wait(); err != nil {
		t.Fatalf("a clean exit is no error: %v", err)
	}
}

func TestStartReportsAFailedExitOnWait(t *testing.T) {
	p, err := NewOS().Start(context.Background(), sh("exit 4"))
	if err != nil {
		t.Fatal(err)
	}
	exit, ok := IsExit(p.Wait())
	if !ok || exit.Code != 4 || exit.Signaled {
		t.Fatalf("exit %#v", exit)
	}
	if _, err := NewOS().Start(context.Background(), Cmd{Name: "mufradat-no-such-program"}); !errors.Is(err, ErrNotFound) || !strings.HasPrefix(err.Error(), "mufradat-no-such-program: ") {
		t.Errorf("start of a missing program: %v", err)
	}
}

func TestStopEndsARunningProgram(t *testing.T) {
	p, err := NewOS().Start(context.Background(), sh("exec sleep 30"))
	if err != nil {
		t.Fatal(err)
	}
	p.Stop()
	done := make(chan error, 1)
	go func() { done <- p.Wait() }()
	select {
	case err := <-done:
		if exit, ok := IsExit(err); !ok || !exit.Signaled {
			t.Errorf("err = %#v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not end the program")
	}
}

func TestCmdStreamsAreNotUsedByStart(t *testing.T) {
	var sink bytes.Buffer
	c := Cmd{Name: "cat", Stdin: strings.NewReader("ignored"), Stdout: &sink}
	p, err := NewOS().Start(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(p.Stdin(), "piped"); err != nil {
		t.Fatal(err)
	}
	_ = p.Stdin().Close()
	got, _ := io.ReadAll(p.Stdout())
	if string(got) != "piped" || sink.Len() != 0 {
		t.Errorf("pipe %q, sink %q", got, sink.String())
	}
	_ = p.Wait()
}

func TestCancellingAsksTheCommandToStopSoItCanCleanUp(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			if _, err := os.Stat(ready); err == nil {
				cancel()
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	err := NewOS().Run(ctx, sh(`trap 'kill $pid; exit 7' TERM; sleep 30 & pid=$!; : > "`+ready+`"; wait $pid`))
	exit, ok := IsExit(err)
	if !ok || exit.Code != 7 || exit.Signaled {
		t.Fatalf("a command that handles SIGTERM must get to finish: %#v", err)
	}
}
