package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
)

func newBuild(a *app.App) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Write the Anki package (.apkg)",
		Long: `Build the Anki package from the notes and the audio clips that exist.

The package is written to <workspace>/out/ unless -o is given, and its path is
printed on standard output. Building twice from the same inputs produces
identical bytes; set SOURCE_DATE_EPOCH to pin the note modification times.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := a.PlanBuild(out)
			if err != nil {
				return err
			}
			if err := a.WriteBuild(plan); err != nil {
				return err
			}
			r := a.Reporter()
			sum := plan.Summary
			r.Event("%d notes, %d cards (%d production), %d audio clips with %s", sum.Notes, sum.Cards, sum.Production, sum.AudioFiles, plan.Voice.Describe())
			if sum.MissingAudio > 0 {
				r.Warn("%d clips are missing; run `%s audio`", sum.MissingAudio, a.CommandPrefix())
			}
			tags := make([]string, 0, len(sum.Tagged))
			for t := range sum.Tagged {
				tags = append(tags, t)
			}
			sort.Strings(tags)
			for _, t := range tags {
				r.Event("%s: %d", t, sum.Tagged[t])
			}
			fmt.Fprintln(a.Env.Stdout, plan.OutPath)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "write the package to this path")
	return cmd
}
