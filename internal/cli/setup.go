package cli

import (
	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
)

func newSetup(a *app.App) *cobra.Command {
	var opts app.SetupOptions
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install the vowel checkers (CAMeL Tools and CATT)",
		Long: `Install what 'check' needs: a private Python environment with CAMeL Tools and
CATT (about 1.4 GB), their language data (about 550 MB), and a test run.

Nothing else needs it. Without it 'add', 'audio' and 'build' work, and cards are
tagged check::unverified. The environment lives under the user data directory,
is shared by every workspace, and is rebuilt only when this program changes the
pinned package list.

Needs Python 3.11 or newer on PATH (--python names one), and uv if present.
To use an environment you manage yourself, set verify.python in the user config.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := a.RunSetup(cmd.Context(), opts)
			if err != nil {
				return err
			}
			r := a.Reporter()
			switch {
			case out.Managed:
				r.Event("ready: Python %s, installed with %s", out.Ready.PythonVersion, out.Ready.Installer)
			default:
				r.Event("ready: %s works", out.Custom)
			}
			r.Event("next: %s check", a.CommandPrefix())
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Python, "python", "", "Python to build the environment from (default: the newest python3 on PATH)")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "rebuild the environment even if it looks fine")
	return cmd
}
