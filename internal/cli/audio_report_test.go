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

func TestClipFailuresAreListedCappedAndSummarisedByKind(t *testing.T) {
	const (
		retryFailed  = "run the command again to retry the failed clips"
		switchToGoog = "edit those notes, or use the Google provider with `mufradat config set audio.provider google`"
	)
	cases := []struct {
		name         string
		failed       int
		blocked      int
		wantMessage  string
		wantHint     string
		wantWarnings []string
		wantNot      []string
	}{
		{name: "nothing failed"},
		{"one failed", 1, 0, "1 clip failed", retryFailed, []string{"f1: refused"}, []string{"more"}},
		{"ten failed are all listed", 10, 0, "10 clips failed", retryFailed, []string{"f1: refused", "f10: refused"}, []string{"more"}},
		{"eleven failed list ten and count the rest", 11, 0, "11 clips failed", retryFailed, []string{"f10: refused", "and 1 more"}, []string{"f11"}},
		{"twelve failed", 12, 0, "12 clips failed", retryFailed, []string{"and 2 more"}, []string{"f11", "f12"}},
		{"one blocked", 0, 1, "1 clip was blocked by Mistral's moderation", switchToGoog, []string{"b1: refused"}, []string{"failed"}},
		{"several blocked", 0, 3, "3 clips were blocked by Mistral's moderation", switchToGoog, []string{"b3: refused"}, []string{"failed"}},
		{"both kinds", 2, 1, "2 clips failed; 1 clip was blocked by Mistral's moderation", switchToGoog + "; run the command again to retry the others", []string{"f2: refused", "b1: refused"}, nil},
		{"blocked are capped on their own", 0, 12, "12 clips were blocked by Mistral's moderation", switchToGoog, []string{"b10: refused", "and 2 more"}, []string{"b11"}},
	}
	for _, tc := range cases {
		a, stderr := reportingApp(t)
		res := clips.Result{Failed: clipFailures(tc.failed, "f"), Blocked: clipFailures(tc.blocked, "b")}
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

func TestTheVoiceInUseIsDescribedWithItsRateOnlyWhenThereIsOne(t *testing.T) {
	if got := describeVoice(clips.Voice{Provider: "mistral", Name: "voice_479"}); got != "voice_479 (mistral)" {
		t.Errorf("%q", got)
	}
	if got := describeVoice(clips.Voice{Provider: "google", Name: "ar-XA-Chirp3-HD-Achernar", Rate: 0.9}); got != "ar-XA-Chirp3-HD-Achernar (google, rate 0.9)" {
		t.Errorf("%q", got)
	}
}
