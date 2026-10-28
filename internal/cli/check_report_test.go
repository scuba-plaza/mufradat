package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/failure"
)

func TestTheCheckReportListsIssuesAndCapsTheCouldNotCheckLines(t *testing.T) {
	a, stderr := reportingApp(t)
	failures := map[string]string{}
	for i := 1; i <= 12; i++ {
		failures[fmt.Sprintf("n_%02d", i)] = "the checker crashed"
	}
	renderCheck(a, checkReport{Notes: 30, Checked: 12, Cached: 18, Unchecked: 4, Major: 2, Minor: 5, Failures: failures})
	got := stderr.String()
	for _, want := range []string{
		"30 notes checked: 12 new, 18 remembered\n",
		"2 notes with major flags, 5 with minor flags, 4 not checked\n",
		"could not check n_01: the checker crashed\n",
		"could not check n_10: the checker crashed\n",
		"and 2 more\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr lacks %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"n_11", "n_12"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("stderr lists %s although only ten are shown:\n%s", unwanted, got)
		}
	}

	a, stderr = reportingApp(t)
	renderCheck(a, checkReport{Notes: 1, Failures: map[string]string{"n_01": "boom"}})
	if strings.Contains(stderr.String(), "more") || !strings.Contains(stderr.String(), "1 note checked") {
		t.Errorf("one failure and one note: %s", stderr.String())
	}
}

func TestTheCheckVerdictSaysWhatToDoAndExitsAccordingly(t *testing.T) {
	a, _ := reportingApp(t)
	for _, c := range []struct {
		name    string
		report  checkReport
		strict  bool
		code    int
		message string
		hint    string
	}{
		{"clean", checkReport{Notes: 3}, true, 0, "", ""},
		{"major flags are fine without --strict", checkReport{Major: 2}, false, 0, "", ""},
		{"major flags fail --strict", checkReport{Major: 2}, true, failure.ExitChecksFailed, "2 notes with major flags", "fix them in `mufradat review`, or dismiss the ones that are fine"},
		{"one major flag", checkReport{Major: 1}, true, failure.ExitChecksFailed, "1 note with major flags", "fix them in `mufradat review`, or dismiss the ones that are fine"},
		{"minor flags never fail", checkReport{Minor: 9}, true, 0, "", ""},
		{"unchecked notes fail", checkReport{Unchecked: 1}, false, failure.ExitFailure, "some notes are still unchecked", "run the command again"},
		{"failed checks fail", checkReport{Failures: map[string]string{"n_1": "x"}}, false, failure.ExitFailure, "1 item could not be checked", "run the command again to retry"},
		{"failed checks outrank major flags", checkReport{Failures: map[string]string{"n_1": "x", "n_2": "y"}, Major: 3}, true, failure.ExitFailure, "2 items could not be checked", "run the command again to retry"},
	} {
		err := checkVerdict(a, c.report, c.strict)
		if code := failure.Code(err); code != c.code {
			t.Errorf("%s: exit code %d, want %d (%v)", c.name, code, c.code, err)
			continue
		}
		if c.code == 0 {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		if err.Error() != c.message || failure.Hint(err) != c.hint {
			t.Errorf("%s: %q / hint %q, want %q / %q", c.name, err, failure.Hint(err), c.message, c.hint)
		}
	}
}
