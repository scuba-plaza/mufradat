package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	mufradat "github.com/scuba-plaza/mufradat"
	"github.com/scuba-plaza/mufradat/assets"
	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/version"
)

func newVersion(a *app.App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := version.Get()
			if asJSON {
				return writeJSON(a.Env.Stdout, info)
			}
			_, err := fmt.Fprintf(a.Env.Stdout, "%s %s\n", app.ProgramName, info)
			return err
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	return cmd
}

func newLicenses(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "licenses",
		Short: "Print the licences of the code, the shipped data and the bundled font",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(a.Env.Stdout, "%s\n%s\n\n%s\n%s",
				mufradat.License, mufradat.Notice, "Scheherazade New font licence", assets.FontLicense)
			return err
		},
	}
}
