package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/review"
	"github.com/scuba-plaza/mufradat/internal/ui"
)

func newVoices(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voices",
		Short: "Work with the Arabic text-to-speech voices",
	}
	cmd.AddCommand(newVoicesList(a), newVoicesCompare(a))
	return cmd
}

func newVoicesList(a *app.App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the Arabic voices of the audio provider",
		Long: `List the Arabic voices the provider in audio.provider offers. The first column
is what to put in google.voice or mistral.voice; * marks the voice in use.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			speaker, err := a.Speaker(cmd.Context())
			if err != nil {
				return err
			}
			voices, err := speaker.Voices(cmd.Context())
			if err != nil {
				return a.SpeechFailure(err)
			}
			current, err := a.Voice()
			if err != nil {
				return err
			}
			if asJSON {
				type row struct {
					Voice   string `json:"voice"`
					Name    string `json:"name"`
					Gender  string `json:"gender"`
					Detail  string `json:"detail"`
					Current bool   `json:"current"`
				}
				rows := make([]row, 0, len(voices))
				for _, v := range voices {
					rows = append(rows, row{v.Ref, v.Name, v.Gender, v.Detail, v.Is(current.Name)})
				}
				return writeJSON(a.Env.Stdout, rows)
			}
			rows := make([][]string, 0, len(voices))
			for _, v := range voices {
				mark := " "
				if v.Is(current.Name) {
					mark = "*"
				}
				name := v.Name
				if name == v.Ref {
					name = ""
				}
				rows = append(rows, []string{mark, v.Ref, name, v.Gender, v.Detail})
			}
			ui.Table(a.Env.Stdout, rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	return cmd
}

func newVoicesCompare(a *app.App) *cobra.Command {
	var texts []string
	var noOpen bool
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Listen to the Arabic voices side by side and pick one",
		Long: `Open a page in your browser, served only on this machine, where each Arabic voice
of both providers, Mistral and Google, can say a word and a sentence from your
notes. A provider that is not set up is left out, with a note on the page that
says how to set it up. Clips are made when you press play and kept in the
workspace, so hearing a voice twice costs nothing.

'Use this voice' saves it as google.voice or mistral.voice in the workspace and,
for a voice of the other provider, switches audio.provider to it. Clips the
other provider already made are kept and found again if you switch back.

Clips already made keep their voice until you remake them in 'review'.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			deps, err := a.PrepareCompare(texts)
			if err != nil {
				return err
			}
			r := a.Reporter()
			before := deps.Choices.Current()
			used, err := review.Compare(ctx, deps, func(url string) {
				fmt.Fprintf(a.Env.Stdout, "comparing voices at %s\n", url)
				if !noOpen {
					if err := a.OpenBrowser(ctx, url); err != nil {
						r.Warn("could not open a browser (%v); open the address above yourself", err)
					}
				}
				r.Event("press 'Done' on the page or Ctrl+C here when you have chosen")
			})
			switch {
			case used.Voice == "":
				r.Event("the voice was not changed")
			case used.Provider == before.Provider:
				r.Event("%s.voice is now %s; `%s audio` makes the clips with it", used.Provider, used.Voice, a.CommandPrefix())
			default:
				r.Event("audio.provider is now %s and %s.voice is %s; `%s audio` makes the clips with it, and the clips of the other provider are kept", used.Provider, used.Provider, used.Voice, a.CommandPrefix())
			}
			return err
		},
	}
	cmd.Flags().StringArrayVar(&texts, "text", nil, "say this fully vowelled text instead of words from your notes (repeatable)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "only print the address instead of opening a browser")
	return cmd
}
