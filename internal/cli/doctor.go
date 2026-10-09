package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/doctor"
	"github.com/scuba-plaza/mufradat/internal/failure"
)

var errDoctorFailed = errors.New("some checks failed")

func newDoctor(a *app.App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check that everything the tool needs is in place",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			results := doctor.Run(cmd.Context(), a.DoctorChecks())
			if asJSON {
				if err := writeJSON(a.Env.Stdout, results); err != nil {
					return err
				}
			} else {
				doctor.Render(a.Env.Stdout, results, a.StdoutColor())
			}
			if doctor.Failed(results) {
				return failure.NotSetUp("fix the failed checks above", errDoctorFailed)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	return cmd
}
