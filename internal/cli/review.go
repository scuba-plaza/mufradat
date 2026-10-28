package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/review"
)

func newReview(a *app.App) *cobra.Command {
	var minor, all, noCheck, noOpen bool
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Go through flagged notes in your browser, or every note with --all",
		Long: `Open a page in your browser, served only on this machine, with every note that
'check' flagged and you have not dealt with yet. Each flag says in plain words
what disagreed: the card's vowels next to CATT's and CAMeL's readings, with the
differing letters highlighted. For each note you can
  enter  say the card is right, so its flags stay out of the next build
  1 2    add the missing ending CATT or CAMeL read, when that is the whole problem
  e      edit the note
  c      ask Mistral for a better version, and keep it or not
  w/f/s  listen to the word, the forms or the sentence
  u      undo your last decision

With --all every note is listed, flagged or not, so you can go through the whole
deck and edit any of it.

Each clip of a note also has buttons that remove its MP3 or make it again with
Google Text-to-Speech, as 'audio' does. The voice can be picked from Google's
Arabic voices next to those buttons; it is saved in the workspace and speaks
every clip made from then on.

Every decision is saved straight away, and notes that 'add' writes meanwhile are
never lost. An edited note is checked again in the background. Click 'Finish
review' on the page or press Ctrl+C here when you are done.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReview(cmd, a, app.ReviewOptions{Minor: minor, All: all, NoCheck: noCheck}, noOpen)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&minor, "minor", true, "include notes whose only flags are minor disagreements")
	f.BoolVar(&all, "all", false, "list every note, not only the flagged ones, to review and edit")
	f.BoolVar(&noCheck, "no-check", false, "do not check notes that have not been checked yet; use the remembered results")
	f.BoolVar(&noOpen, "no-open", false, "only print the address instead of opening a browser")
	return cmd
}

func runReview(cmd *cobra.Command, a *app.App, opts app.ReviewOptions, noOpen bool) error {
	ctx := cmd.Context()
	setup, err := a.PrepareReview(ctx, opts)
	if err != nil {
		return err
	}
	defer setup.Close()
	r := a.Reporter()
	for _, notice := range setup.Notices {
		r.Warn("%s", notice)
	}
	if setup.Unlisted > 0 && !opts.All {
		r.Warn("%s not checked yet, so they are not listed; run `%s check`", counted(setup.Unlisted, "note is", "notes are"), a.CommandPrefix())
	}
	noun := func(n int) string {
		if opts.All {
			return counted(n, "note", "notes")
		}
		return counted(n, "flagged note", "flagged notes")
	}
	if len(setup.Items) == 0 {
		r.Event("nothing to review")
		return nil
	}
	sum, err := review.Serve(ctx, setup.Deps, setup.Items, setup.Filter, func(url string) {
		fmt.Fprintf(a.Env.Stdout, "reviewing %s at %s\n", noun(len(setup.Items)), url)
		if !noOpen {
			if err := a.OpenBrowser(ctx, url); err != nil {
				r.Warn("could not open a browser (%v); open the address above yourself", err)
			}
		}
		r.Event("click 'Finish review' on the page or press Ctrl+C here when you are done")
	})
	r.Event("%s: %d marked right, %d edited, %d rewritten by Mistral, %d still open", noun(sum.Total), sum.Kept, sum.Edited, sum.Rewritten, sum.Open)
	if next := review.NextSteps(sum, a.CommandPrefix()); next != "" {
		r.Event("%s", next)
	}
	return err
}
