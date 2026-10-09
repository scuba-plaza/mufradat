package cli

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/failure"
)

var userOnlyKeys = []string{"workspace", "google.credentials_file", "verify.python"}

func newConfig(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or change settings",
		Long: `Settings come from built-in defaults, then the user config file, then the
workspace file (mufradat.toml). 'config set' writes to the workspace file
unless --user is given or the key can only live in the user file.`,
	}
	cmd.AddCommand(newConfigGet(a), newConfigSet(a), newConfigPath(a))
	return cmd
}

func newConfigGet(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Print one setting, or all of them",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := a.Config()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				v, err := config.Get(cfg, args[0])
				if err != nil {
					return failure.Usage(err)
				}
				fmt.Fprintln(a.Env.Stdout, v)
				return nil
			}
			flat, err := config.Flatten(cfg)
			if err != nil {
				return err
			}
			for _, k := range slices.Sorted(maps.Keys(flat)) {
				fmt.Fprintf(a.Env.Stdout, "%s = %v\n", k, flat[k])
			}
			return nil
		},
	}
}

func newConfigSet(a *app.App) *cobra.Command {
	var user bool
	cmd := &cobra.Command{
		Use:   "set key value",
		Short: "Change a setting",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			target := ""
			if user || slices.Contains(userOnlyKeys, key) {
				target = a.Dirs().UserConfigFile()
			} else {
				ws, err := a.Workspace()
				if err != nil {
					return err
				}
				target = ws.ConfigFile()
			}
			if err := config.SetFile(target, key, value); err != nil {
				return failure.Usage(err)
			}
			a.InvalidateConfig()
			a.Reporter().Event("%s = %s (%s)", key, value, target)
			return nil
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "write to the user config file instead of the workspace")
	return cmd
}

func newConfigPath(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print where settings and the workspace live",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			d := a.Dirs()
			var b strings.Builder
			fmt.Fprintf(&b, "user config  %s\n", d.UserConfigFile())
			fmt.Fprintf(&b, "data         %s\n", d.Data)
			fmt.Fprintf(&b, "cache        %s\n", d.Cache)
			if res, err := a.Resolution(); err == nil {
				fmt.Fprintf(&b, "workspace    %s (%s)\n", res.Workspace.Root, res.Source)
			} else {
				fmt.Fprintf(&b, "workspace    none (%v)\n", err)
			}
			_, err := fmt.Fprint(a.Env.Stdout, b.String())
			return err
		},
	}
}
