package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/failure"
)

func reportingApp(t *testing.T) (*app.App, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	stderr := &bytes.Buffer{}
	return app.New(env.Env{
		Getenv: func(string) string { return "" },
		Now:    time.Now,
		Home:   home,
		Stdout: io.Discard,
		Stderr: stderr,
		Cwd:    func() (string, error) { return home, nil },
	}), stderr
}

func clipFailures(n int, prefix string) []clips.Failure {
	out := make([]clips.Failure, n)
	for i := range out {
		out[i] = clips.Failure{Item: clips.Item{Text: fmt.Sprintf("%s%d", prefix, i+1)}, Err: errors.New("refused")}
	}
	return out
}

func TestClipFailuresAreListedCappedAndCounted(t *testing.T) {
	const retryFailed = "run the command again to retry the failed clips"
	cases := []struct {
		name         string
		failed       int
		wantMessage  string
		wantHint     string
		wantWarnings []string
		wantNot      []string
	}{
		{name: "nothing failed"},
		{"one failed", 1, "1 clip failed", retryFailed, []string{"f1: refused"}, []string{"more"}},
		{"ten failed are all listed", 10, "10 clips failed", retryFailed, []string{"f1: refused", "f10: refused"}, []string{"more"}},
		{"eleven failed list ten and count the rest", 11, "11 clips failed", retryFailed, []string{"f10: refused", "and 1 more"}, []string{"f11"}},
		{"twelve failed", 12, "12 clips failed", retryFailed, []string{"and 2 more"}, []string{"f11", "f12"}},
	}
	for _, tc := range cases {
		a, stderr := reportingApp(t)
		res := clips.Result{Failed: clipFailures(tc.failed, "f")}
		err := reportClipFailures(a, res)
		if tc.wantMessage == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		if err == nil || err.Error() != tc.wantMessage || failure.Code(err) != failure.ExitFailure || failure.Hint(err) != tc.wantHint {
			t.Errorf("%s: error %v, exit %d, hint %q; want %q, exit 1, hint %q", tc.name, err, failure.Code(err), failure.Hint(err), tc.wantMessage, tc.wantHint)
		}
		for _, w := range tc.wantWarnings {
			if !strings.Contains(stderr.String(), w) {
				t.Errorf("%s: stderr lacks %q:\n%s", tc.name, w, stderr)
			}
		}
		for _, w := range tc.wantNot {
			if strings.Contains(stderr.String(), w) || strings.Contains(err.Error(), w) {
				t.Errorf("%s: output must not contain %q:\n%s", tc.name, w, stderr)
			}
		}
	}
}
