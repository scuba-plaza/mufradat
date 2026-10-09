package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/workspace"
)

func newInit(a *app.App) *cobra.Command {
	var git bool
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Create a workspace for your notes, audio and decks",
		Long: `Create a workspace: the directory that holds everything that belongs to you
(notes, review decisions, audio clips, the built deck). The tool itself never
writes next to its own files.

Without an argument the workspace is created at the -C directory or, failing
that, in the user data directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := a.Dirs().DefaultWorkspace()
			switch {
			case len(args) == 1:
				root = args[0]
			case a.Opts.Workspace != "":
				root = a.Opts.Workspace
			}
			ws, err := workspace.Init(workspace.InitOptions{
				Root:   root,
				Git:    git,
				DeckID: a.Env.Now().UnixMilli(),
			})
			if err != nil {
				return err
			}
			a.Reporter().Event("created workspace %s", ws.Root)
			fmt.Fprintln(a.Env.Stdout, ws.Root)
			a.Reporter().Event("next: mufradat -C %s status", ws.Root)
			return nil
		},
	}
	cmd.Flags().BoolVar(&git, "git", false, "write a .gitignore for state/ and out/")
	return cmd
}
