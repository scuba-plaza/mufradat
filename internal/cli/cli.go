package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/ui"
)

func Execute(ctx context.Context, e env.Env, args []string) int {
	a := app.New(e)
	root := NewRoot(a)
	root.SetArgs(args)
	root.SetOut(e.Stdout)
	root.SetErr(e.Stderr)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	reportError(e.Stderr, a, err)
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return failure.ExitInterrupted
	}
	if isCobraUsageError(err) && failure.Code(err) == failure.ExitFailure {
		return failure.ExitUsage
	}
	return failure.Code(err)
}

func isCobraUsageError(err error) bool {
	msg := err.Error()
	for _, p := range []string{"unknown command", "unknown flag", "unknown shorthand flag", "invalid argument", "accepts ", "requires at least", "requires at most", "flag needs an argument", "required flag"} {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

func reportError(w io.Writer, a *app.App, err error) {
	color := a.Reporter().Color()
	fmt.Fprintf(w, "%s %s\n", ui.Paint(color, ui.Red, "error:"), err)
	if hint := failure.Hint(err); hint != "" {
		fmt.Fprintf(w, "%s %s\n", ui.Paint(color, ui.Cyan, "hint:"), hint)
	}
}

func NewRoot(a *app.App) *cobra.Command {
	root := &cobra.Command{
		Use:           app.ProgramName,
		Short:         "Build an Anki deck of the most common Modern Standard Arabic words",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: false,
		},
	}
	pf := root.PersistentFlags()
	pf.StringVarP(&a.Opts.Workspace, "workspace", "C", "", "workspace directory (default: found from the current directory, or the user default)")
	pf.CountVarP(&a.Opts.Verbose, "verbose", "v", "print diagnostics (repeat for more)")
	pf.BoolVarP(&a.Opts.Quiet, "quiet", "q", false, "print only errors")
	pf.BoolVar(&a.Opts.NoColor, "no-color", false, "disable coloured output")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return failure.Usage(err) })

	root.AddCommand(
		newInit(a),
		newStatus(a),
		newAdd(a),
		newSetup(a),
		newCheck(a),
		newReview(a),
		newBuild(a),
		newAudio(a),
		newVoices(a),
		newAuth(a),
		newConfig(a),
		newDoctor(a),
		newVersion(a),
		newLicenses(a),
	)
	return root
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
