package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/failure"
)

const maxListedFailures = 10

func newAudio(a *app.App) *cobra.Command {
	var dryRun bool
	var limit int
	cmd := &cobra.Command{
		Use:   "audio",
		Short: "Make the missing audio clips",
		Long: `Make every audio clip the notes need that does not exist yet.

Clips are named after the voice, speaking rate and spoken text, so changing the
voice never reuses another voice's clips and changing it back finds them again.
Clips are paid for: they are kept, never refreshed behind your back.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAudio(cmd.Context(), a, dryRun, limit)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be made and how many characters it is, without calling the service")
	cmd.Flags().IntVar(&limit, "limit", 0, "make at most this many clips")
	cmd.AddCommand(newAudioGC(a))
	return cmd
}

func runAudio(ctx context.Context, a *app.App, dryRun bool, limit int) error {
	if limit < 0 {
		return failure.Usage(errors.New("--limit cannot be negative"))
	}
	st, err := a.Store()
	if err != nil {
		return err
	}
	set, err := st.Load()
	if err != nil {
		return err
	}
	cs, err := a.Clips()
	if err != nil {
		return err
	}
	voice, err := a.Voice()
	if err != nil {
		return err
	}
	plan := clips.NewPlan(set.Study(), voice, cs.Has)
	r := a.Reporter()
	r.Event("%d clips wanted, %d present, %d to make (%d characters) with %s",
		plan.Wanted, plan.Present, len(plan.Missing), plan.Chars, voice.Describe())
	if len(plan.Missing) == 0 {
		return nil
	}
	if limit > 0 && limit < len(plan.Missing) {
		plan.Missing = plan.Missing[:limit]
		r.Event("limited to the first %d", limit)
	}
	if dryRun {
		return nil
	}

	speaker, err := a.Speaker(ctx)
	if err != nil {
		return err
	}
	voices, err := speaker.Voices(ctx)
	if err != nil {
		return a.SpeechFailure(err)
	}
	if !hasVoice(voices, voice.Name) {
		return failure.Newf(failure.ExitFailure, fmt.Sprintf("see the choices with `%s voices list`, then `%s config set google.voice NAME`", a.CommandPrefix(), a.CommandPrefix()),
			"the service has no Arabic voice named %q", voice.Name)
	}
	cfg, err := a.Config()
	if err != nil {
		return err
	}
	res, err := clips.Synthesize(ctx, plan, cs, clips.Speaking(speaker), clips.SynthOptions{
		Concurrency: cfg.TTS.Concurrency,
		Progress:    func(done, total int) { r.Progress("audio %d/%d", done, total) },
	})
	r.Done()
	r.Event("made %d clips", res.Made)
	if err != nil {
		return a.SpeechFailure(err)
	}
	return reportClipFailures(a, res)
}

func reportClipFailures(a *app.App, res clips.Result) error {
	r := a.Reporter()
	for i, f := range res.Failed {
		if i == maxListedFailures {
			r.Warn("and %d more", len(res.Failed)-maxListedFailures)
			break
		}
		r.Warn("%s: %v", f.Item.Text, f.Err)
	}
	if len(res.Failed) > 0 {
		return failure.Newf(failure.ExitFailure, "run the command again to retry the failed clips", "%s", failedClips(len(res.Failed)))
	}
	return nil
}

func failedClips(n int) string {
	if n == 1 {
		return "1 clip failed"
	}
	return fmt.Sprintf("%d clips failed", n)
}

func hasVoice(voices []clips.VoiceInfo, name string) bool {
	for _, v := range voices {
		if v.Name == name {
			return true
		}
	}
	return false
}

func newAudioGC(a *app.App) *cobra.Command {
	var del bool
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Find clips no note needs any more",
		Long: `List the clips that no note needs with the current voice and rate. That
includes clips made with another voice or rate, so they are only deleted with
--delete.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := a.Store()
			if err != nil {
				return err
			}
			set, err := st.Load()
			if err != nil {
				return err
			}
			cs, err := a.Clips()
			if err != nil {
				return err
			}
			voice, err := a.Voice()
			if err != nil {
				return err
			}
			orphans, err := clips.Orphans(cs, set.All(), voice)
			if err != nil {
				return err
			}
			var total int64
			for _, o := range orphans {
				total += o.Size
				if del {
					if err := cs.Remove(o.Key); err != nil {
						return err
					}
				}
			}
			verb := "found"
			if del {
				verb = "deleted"
			}
			a.Reporter().Event("%s %d unused clips (%.1f KiB)", verb, len(orphans), float64(total)/1024)
			if !del && len(orphans) > 0 {
				a.Reporter().Event("run again with --delete to remove them")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&del, "delete", false, "delete the unused clips")
	return cmd
}
