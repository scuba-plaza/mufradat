package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/scuba-plaza/mufradat/assets"
	"github.com/scuba-plaza/mufradat/internal/author"
	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/config"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/proc"
	"github.com/scuba-plaza/mufradat/internal/prompt"
	"github.com/scuba-plaza/mufradat/internal/review"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
	"github.com/scuba-plaza/mufradat/internal/tts"
)

const browserCommand = "xdg-open"

type ReviewOptions struct {
	Minor   bool
	All     bool
	NoCheck bool
}

type ReviewSetup struct {
	Deps     review.Deps
	Items    []review.Item
	Filter   review.Filter
	Notices  []string
	Unlisted int
	close    func()
}

func (r ReviewSetup) Close() {
	if r.close != nil {
		r.close()
	}
}

func (a *App) PrepareReview(ctx context.Context, opts ReviewOptions) (ReviewSetup, error) {
	st, err := a.Store()
	if err != nil {
		return ReviewSetup{}, err
	}
	set, err := st.Load()
	if err != nil {
		return ReviewSetup{}, err
	}
	log, err := a.Decisions()
	if err != nil {
		return ReviewSetup{}, err
	}
	notes := set.Study()
	setup := ReviewSetup{
		Filter: review.Filter{Minor: opts.Minor, All: opts.All},
		Deps:   review.Deps{Notes: st, Decisions: log, Prefix: a.CommandPrefix(), Now: a.Env.Now, Font: assets.Font},
	}

	var closers []func()
	setup.close = func() {
		for _, c := range closers {
			c()
		}
	}
	fail := func(err error) (ReviewSetup, error) {
		setup.Close()
		return ReviewSetup{}, err
	}

	verification, err := a.LoadVerification(notes)
	if err != nil {
		return fail(err)
	}
	report := verification.Report
	switch checker, err := a.NewChecker(); {
	case err == nil:
		closers = append(closers, func() { checker.Close() })
		setup.Deps.Verifier = checker
		if !opts.NoCheck && len(report.Pending) > 0 {
			r := a.Reporter()
			r.Event("checking %d items that have not been checked yet", len(report.Pending))
			report, err = checker.Check(ctx, notes)
			r.Done()
			if err != nil {
				return fail(err)
			}
		}
	case !verification.Status.Enabled:
		if !opts.All {
			setup.Notices = append(setup.Notices, fmt.Sprintf("nothing is flagged because vowel checking is not available: %s; `%s review --all` still lets you go through every note", verification.Status.Reason, a.CommandPrefix()))
		}
	default:
		return fail(err)
	}

	setup.Items = review.Items(notes, report, verification.Dismissed, setup.Filter)
	if verification.Status.Enabled {
		for _, n := range notes {
			if !report.Results[n.ID].Verified {
				setup.Unlisted++
			}
		}
	}

	audio, err := a.reviewAudio()
	if err != nil {
		return fail(err)
	}
	setup.Deps.Audio = audio
	setup.Deps.Voices = reviewVoices{app: a}

	if rewriter, err := a.reviewRewriter(); err == nil {
		setup.Deps.Rewriter = rewriter
	} else {
		a.Logger().Debug("asking Claude Code is not available", "error", err)
	}
	return setup, nil
}

func (a *App) reviewRewriter() (review.Rewriter, error) {
	backend, err := a.Claude()
	if err != nil {
		return nil, err
	}
	system, err := a.AuthorSystem()
	if err != nil {
		return nil, err
	}
	return author.Rewriter{LLM: backend, Schema: prompt.Schema(), System: system}, nil
}

func (a *App) OpenBrowser(ctx context.Context, url string) error {
	if _, err := a.Env.Runner.LookPath(browserCommand); err != nil {
		return err
	}
	go func() {
		if err := a.Env.Runner.Run(ctx, proc.Cmd{Name: browserCommand, Args: []string{url}}); err != nil && ctx.Err() == nil {
			a.Logger().Debug("opening the browser failed", "error", err)
		}
	}()
	return nil
}

var clipKinds = []struct {
	Kind  string
	Field clips.Field
}{
	{"word", clips.Word},
	{"forms", clips.Forms},
	{"sentence", clips.Example},
}

func clipKindOf(field clips.Field) string {
	for _, k := range clipKinds {
		if k.Field == field {
			return k.Kind
		}
	}
	return ""
}

type reviewAudio struct {
	app   *App
	store *clips.Store
}

func (a *App) reviewAudio() (review.Audio, error) {
	cs, err := a.Clips()
	if err != nil {
		return nil, err
	}
	return reviewAudio{app: a, store: cs}, nil
}

func (r reviewAudio) item(n note.Note, kind string) (clips.Item, error) {
	voice, err := r.app.Voice()
	if err != nil {
		return clips.Item{}, err
	}
	for _, it := range clips.Wanted(n, voice) {
		if clipKindOf(it.Field) == kind {
			return it, nil
		}
	}
	return clips.Item{}, fmt.Errorf("this note has no %s to speak", kind)
}

func (r reviewAudio) Clips(n note.Note) []review.ClipInfo {
	voice, err := r.app.Voice()
	if err != nil {
		return nil
	}
	var out []review.ClipInfo
	for _, it := range clips.Wanted(n, voice) {
		if kind := clipKindOf(it.Field); kind != "" {
			out = append(out, review.ClipInfo{Kind: kind, Ready: r.store.Has(it.Key)})
		}
	}
	return out
}

func (r reviewAudio) Path(n note.Note, kind string) (string, bool) {
	it, err := r.item(n, kind)
	if err != nil || !r.store.Has(it.Key) {
		return "", false
	}
	return r.store.Path(it.Key), true
}

func (r reviewAudio) Remake(ctx context.Context, n note.Note, kind string) error {
	it, err := r.item(n, kind)
	if err != nil {
		return err
	}
	voice, err := r.app.Voice()
	if err != nil {
		return err
	}
	client, err := r.app.TTS(ctx)
	if err != nil {
		return err
	}
	data, err := client.Synthesize(ctx, tts.Request{Text: it.Text, Voice: voice.Name, Rate: voice.Rate})
	if err != nil {
		return r.app.TTSFailure(err)
	}
	return r.store.Put(it.Key, data)
}

func (r reviewAudio) Remove(n note.Note, kind string) error {
	it, err := r.item(n, kind)
	if err != nil {
		return err
	}
	return r.store.Remove(it.Key)
}

type reviewVoices struct{ app *App }

func (v reviewVoices) Current() string {
	voice, err := v.app.Voice()
	if err != nil {
		return ""
	}
	return voice.Name
}

func (v reviewVoices) List(ctx context.Context) ([]review.VoiceOption, error) {
	client, err := v.app.TTS(ctx)
	if err != nil {
		return nil, err
	}
	voices, err := client.Voices(ctx)
	if err != nil {
		return nil, v.app.TTSFailure(err)
	}
	out := make([]review.VoiceOption, 0, len(voices))
	for _, vi := range voices {
		out = append(out, review.VoiceOption{Name: vi.Name, Gender: vi.Gender, Tier: voiceTier(vi.Name)})
	}
	return out, nil
}

func voiceTier(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) >= 4 {
		return parts[2]
	}
	return ""
}

func (v reviewVoices) Set(name string) error {
	ws, err := v.app.Workspace()
	if err != nil {
		return err
	}
	if err := config.SetFile(ws.ConfigFile(), "audio.voice", name); err != nil {
		return err
	}
	v.app.InvalidateConfig()
	return nil
}

type voiceSampler struct {
	app    *App
	mu     sync.Mutex
	client *tts.Client
}

func (s *voiceSampler) speaker(ctx context.Context) (*tts.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	client, err := s.app.TTS(context.WithoutCancel(ctx))
	if err != nil {
		return nil, err
	}
	s.client = client
	return client, nil
}

func (s *voiceSampler) Sample(ctx context.Context, voice, text string) (string, error) {
	ws, err := s.app.Workspace()
	if err != nil {
		return "", err
	}
	current, err := s.app.Voice()
	if err != nil {
		return "", err
	}
	key := clips.Key(clips.Voice{Name: voice, Rate: current.Rate}, text)
	path := filepath.Join(ws.VoiceSamplesDir(), clips.FileName(key))
	if fsutil.Exists(path) {
		return path, nil
	}
	client, err := s.speaker(ctx)
	if err != nil {
		return "", err
	}
	data, err := client.Synthesize(ctx, tts.Request{Text: text, Voice: voice, Rate: current.Rate})
	if err != nil {
		return "", s.app.TTSFailure(err)
	}
	if err := fsutil.WriteFileAtomic(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

const (
	defaultSampleWord     = "مَرْحَبًا"
	defaultSampleSentence = "أَهْلًا وَسَهْلًا، كَيْفَ حَالُكَ الْيَوْمَ؟"
	sampleNotes           = 2
)

func (a *App) CompareSamples(texts []string) ([]review.Sample, error) {
	var samples []review.Sample
	add := func(label, text string) {
		samples = append(samples, review.Sample{ID: fmt.Sprintf("s%d", len(samples)+1), Label: label, Text: text})
	}
	for _, text := range texts {
		add("text", text)
	}
	if len(samples) > 0 {
		return samples, nil
	}
	st, err := a.Store()
	if err != nil {
		return nil, err
	}
	set, err := st.Load()
	if err != nil {
		return nil, err
	}
	for _, n := range set.Study() {
		if len(samples) >= 2*sampleNotes {
			break
		}
		add("word", n.Lemma)
		if n.Example != "" {
			add("sentence", tashkeel.Pausal(note.PlainExample(n.Example)))
		}
	}
	if len(samples) == 0 {
		add("word", defaultSampleWord)
		add("sentence", defaultSampleSentence)
	}
	return samples, nil
}

func (a *App) PrepareCompare(texts []string) (review.CompareDeps, error) {
	samples, err := a.CompareSamples(texts)
	if err != nil {
		return review.CompareDeps{}, err
	}
	return review.CompareDeps{Voices: reviewVoices{app: a}, Sampler: &voiceSampler{app: a}, Samples: samples, Font: assets.Font}, nil
}
