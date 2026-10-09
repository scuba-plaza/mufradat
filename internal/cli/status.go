package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/status"
	"github.com/scuba-plaza/mufradat/internal/ui"
)

func newStatus(a *app.App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show where the workspace stands and what to run next",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := a.StatusSnapshot()
			if err != nil {
				return err
			}
			state := status.Compute(snap)
			if asJSON {
				return writeJSON(a.Env.Stdout, state)
			}
			color := a.StdoutColor()
			rows := make([][]string, 0, len(state.Steps))
			for _, s := range state.Steps {
				rows = append(rows, []string{mark(color, s.Status), s.Name, s.Detail})
			}
			ui.Table(a.Env.Stdout, rows)
			fmt.Fprintf(a.Env.Stdout, "\nnext: %s\n", state.Next)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	return cmd
}

func mark(color bool, s status.StepStatus) string {
	switch s {
	case status.Done:
		return ui.Paint(color, ui.Green, "✓")
	case status.Todo:
		return ui.Paint(color, ui.Yellow, "•")
	default:
		return ui.Paint(color, ui.Dim, "-")
	}
}
