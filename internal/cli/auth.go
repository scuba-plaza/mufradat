package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/failure"
)

func newAuth(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Set up the accounts the tool talks to",
	}
	cmd.AddCommand(newAuthGoogle(a))
	return cmd
}

func newAuthGoogle(a *app.App) *cobra.Command {
	var importFile string
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Show or import the Google Cloud credentials used for audio",
		Long: `Audio comes from Google Cloud Text-to-Speech. Credentials are looked for in this
order: GOOGLE_APPLICATION_CREDENTIALS, the google.credentials_file setting, the
key imported with --import, then gcloud application default credentials.

--import copies a service-account key into the user config directory (mode
0600) so it does not have to stay next to your other files.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if importFile != "" {
				info, err := a.ImportGoogleKey(cmd.Context(), importFile)
				if err != nil {
					return failure.New(failure.ExitFailure, "pass a service-account key file downloaded from the Google Cloud console", err)
				}
				who := info.Type
				if info.Account != "" {
					who += " " + info.Account
				}
				a.Reporter().Event("imported %s", who)
				return nil
			}
			creds, err := a.GoogleCredentials()
			if err != nil {
				return err
			}
			fmt.Fprintf(a.Env.Stdout, "%s\n", creds.Source)
			if creds.Path != "" {
				fmt.Fprintf(a.Env.Stdout, "%s\n", creds.Path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&importFile, "import", "", "copy this credentials file into the user config directory")
	return cmd
}
