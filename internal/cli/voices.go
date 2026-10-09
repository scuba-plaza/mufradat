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
		Short: "List the Arabic voices the service offers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.TTS(cmd.Context())
			if err != nil {
				return err
			}
			voices, err := client.Voices(cmd.Context())
			if err != nil {
				return a.TTSFailure(err)
			}
			current, err := a.Voice()
			if err != nil {
				return err
			}
			if asJSON {
				type row struct {
					Name       string `json:"name"`
					Gender     string `json:"gender"`
					SampleRate int    `json:"sample_rate"`
					Current    bool   `json:"current"`
				}
				rows := make([]row, 0, len(voices))
				for _, v := range voices {
					rows = append(rows, row{v.Name, v.Gender, v.SampleRate, v.Name == current.Name})
				}
				return writeJSON(a.Env.Stdout, rows)
			}
			rows := make([][]string, 0, len(voices))
			for _, v := range voices {
				mark := " "
				if v.Name == current.Name {
					mark = "*"
				}
				rows = append(rows, []string{mark, v.Name, v.Gender, formatRate(v.SampleRate)})
			}
			ui.Table(a.Env.Stdout, rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	return cmd
}

func formatRate(hz int) string {
	if hz == 0 {
		return ""
	}
	return fmt.Sprintf("%g kHz", float64(hz)/1000)
}

func newVoicesCompare(a *app.App) *cobra.Command {
	var texts []string
	var noOpen bool
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Listen to the Arabic voices side by side and pick one",
		Long: `Open a page in your browser, served only on this machine, where each Arabic voice
of the service can say a word and a sentence from your notes. Clips are made
when you press play and kept in the workspace, so hearing a voice twice costs
nothing. 'Use this voice' saves it as audio.voice in the workspace.

Clips already made keep their voice until you remake them in 'review'.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			deps, err := a.PrepareCompare(texts)
			if err != nil {
				return err
			}
			r := a.Reporter()
			used, err := review.Compare(ctx, deps, func(url string) {
				fmt.Fprintf(a.Env.Stdout, "comparing voices at %s\n", url)
				if !noOpen {
					if err := a.OpenBrowser(ctx, url); err != nil {
						r.Warn("could not open a browser (%v); open the address above yourself", err)
					}
				}
				r.Event("press 'Done' on the page or Ctrl+C here when you have chosen")
			})
			if used != "" {
				r.Event("audio.voice is now %s; `%s audio` makes the clips with it", used, a.CommandPrefix())
			} else {
				r.Event("the voice was not changed")
			}
			return err
		},
	}
	cmd.Flags().StringArrayVar(&texts, "text", nil, "say this fully vowelled text instead of words from your notes (repeatable)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "only print the address instead of opening a browser")
	return cmd
}
