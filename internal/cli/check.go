package cli

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/ui"
	"github.com/scuba-plaza/mufradat/internal/verify"
)

type checkIssue struct {
	Note     string          `json:"note"`
	Lemma    string          `json:"lemma"`
	Field    string          `json:"field"`
	Kind     verify.Kind     `json:"kind"`
	Severity verify.Severity `json:"severity"`
	Word     string          `json:"word,omitempty"`
	Detail   string          `json:"detail"`
}

type checkReport struct {
	Notes     int               `json:"notes"`
	Checked   int               `json:"checked"`
	Cached    int               `json:"cached"`
	Unchecked int               `json:"unchecked"`
	Major     int               `json:"major"`
	Minor     int               `json:"minor"`
	Failures  map[string]string `json:"failures,omitempty"`
	Issues    []checkIssue      `json:"issues"`
}

func newCheck(a *app.App) *cobra.Command {
	var strict, asJSON, force bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Cross-check the vowels with CAMeL Tools and CATT",
		Long: `Check the vowelling of every note against two independent tools and list what
is doubtful. Results are remembered, so only new or changed text is checked and
a repeat run does not start Python at all.

Flags that you have looked at and accept are dismissed in 'review' and stay
dismissed. With --strict the exit status is 4 when major flags remain open.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := a.RunCheck(cmd.Context(), app.CheckOptions{Force: force})
			if err != nil {
				return err
			}
			st, err := a.Store()
			if err != nil {
				return err
			}
			set, err := st.Load()
			if err != nil {
				return err
			}
			report := buildCheckReport(out, set)
			if asJSON {
				if err := writeJSON(a.Env.Stdout, report); err != nil {
					return err
				}
			} else {
				renderCheck(a, report)
			}
			return checkVerdict(a, report, strict)
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "exit with status 4 when major flags are open")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the result as JSON")
	cmd.Flags().BoolVar(&force, "force", false, "check everything again, ignoring remembered results")
	return cmd
}

func buildCheckReport(out app.CheckOutcome, set *note.Set) checkReport {
	counts := out.Verification.Counts()
	report := checkReport{
		Notes: out.Notes, Checked: out.Checked, Cached: out.Cached,
		Unchecked: counts.Unchecked, Major: counts.Major, Minor: counts.Minor,
		Failures: out.Failures, Issues: []checkIssue{},
	}
	for _, n := range set.Study() {
		res := out.Verification.Report.Results[n.ID]
		if !res.Verified {
			continue
		}
		for _, is := range res.Open(n.ID, out.Verification.Dismissed) {
			report.Issues = append(report.Issues, checkIssue{
				Note: n.ID, Lemma: n.Lemma, Field: is.Field, Kind: is.Kind,
				Severity: is.Severity, Word: is.Word, Detail: is.Detail,
			})
		}
	}
	return report
}

func renderCheck(a *app.App, report checkReport) {
	if len(report.Issues) > 0 {
		rows := make([][]string, 0, len(report.Issues))
		for _, is := range report.Issues {
			word := is.Word
			if word == "" {
				word = "(" + is.Field + ")"
			}
			rows = append(rows, []string{is.Note, is.Lemma, string(is.Severity), is.Field, word, is.Detail})
		}
		ui.Table(a.Env.Stdout, rows)
	}
	r := a.Reporter()
	r.Event("%s checked: %d new, %d remembered", counted(report.Notes, "note", "notes"), report.Checked, report.Cached)
	r.Event("%d notes with major flags, %d with minor flags, %d not checked", report.Major, report.Minor, report.Unchecked)
	keys := make([]string, 0, len(report.Failures))
	for k := range report.Failures {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i == maxListedFailures {
			r.Warn("and %d more", len(keys)-maxListedFailures)
			break
		}
		r.Warn("could not check %s: %s", k, report.Failures[k])
	}
}

func checkVerdict(a *app.App, report checkReport, strict bool) error {
	if len(report.Failures) > 0 {
		return failure.Newf(failure.ExitFailure, "run the command again to retry", "%s could not be checked", counted(len(report.Failures), "item", "items"))
	}
	if report.Unchecked > 0 {
		return failure.New(failure.ExitFailure, "run the command again", errors.New("some notes are still unchecked"))
	}
	if strict && report.Major > 0 {
		return failure.Newf(failure.ExitChecksFailed, fmt.Sprintf("fix them in `%s review`, or dismiss the ones that are fine", a.CommandPrefix()),
			"%s with major flags", counted(report.Major, "note", "notes"))
	}
	return nil
}
