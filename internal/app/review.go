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
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/note"
	"github.com/scuba-plaza/mufradat/internal/proc"
	"github.com/scuba-plaza/mufradat/internal/prompt"
	"github.com/scuba-plaza/mufradat/internal/review"
	"github.com/scuba-plaza/mufradat/internal/tashkeel"
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
		a.Logger().Debug("asking Mistral is not available", "error", err)
	}
	return setup, nil
}

func (a *App) reviewRewriter() (review.Rewriter, error) {
	backend, err := a.LLM()
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
	speaker, err := r.app.Speaker(ctx)
	if err != nil {
		return err
	}
	data, err := speaker.Synthesize(ctx, clips.SpeechRequest{Text: it.Text, Voice: voice.Name, Rate: voice.Rate})
	if err != nil {
		return r.app.SpeechFailure(err)
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
	speaker, err := v.app.Speaker(ctx)
	if err != nil {
		return nil, err
	}
	voices, err := speaker.Voices(ctx)
	if err != nil {
		return nil, v.app.SpeechFailure(err)
	}
	return voiceOptions(v.app.verifyConfig().Audio.Provider, voices), nil
}

func voiceOptions(provider string, voices []clips.VoiceInfo) []review.VoiceOption {
	out := make([]review.VoiceOption, 0, len(voices))
	for _, vi := range voices {
		opt := review.VoiceOption{Name: vi.Ref, Label: vi.Name, Gender: vi.Gender}
		if provider == config.ProviderGoogle {
			opt.Label, opt.Tier = googleVoiceLabel(vi.Ref)
		}
		out = append(out, opt)
	}
	return out
}

func googleVoiceLabel(name string) (label, tier string) {
	parts := strings.Split(name, "-")
	if len(parts) >= 4 {
		return strings.Join(parts[2:], "-"), parts[2]
	}
	return name, ""
}

func (v reviewVoices) Set(name string) error {
	ws, err := v.app.Workspace()
	if err != nil {
		return err
	}
	key, err := v.app.VoiceSetting()
	if err != nil {
		return err
	}
	if err := config.SetFile(ws.ConfigFile(), key, name); err != nil {
		return err
	}
	v.app.InvalidateConfig()
	return nil
}

type compareChoices struct{ app *App }

func (c compareChoices) Current() review.Choice {
	voice, err := c.app.Voice()
	if err != nil {
		return review.Choice{}
	}
	return review.Choice{Provider: voice.Provider, Voice: voice.Name}
}

func (c compareChoices) List(ctx context.Context) ([]review.VoiceOption, []string, error) {
	cfg, err := c.app.Config()
	if err != nil {
		return nil, nil, err
	}
	order := []string{cfg.Audio.Provider}
	for _, p := range []string{config.ProviderMistral, config.ProviderGoogle} {
		if p != cfg.Audio.Provider {
			order = append(order, p)
		}
	}
	var options []review.VoiceOption
	var notices []string
	var first error
	for _, provider := range order {
		listed, err := c.providerVoices(ctx, cfg, provider)
		if err != nil {
			if first == nil {
				first = err
			}
			notices = append(notices, notListed(provider, err))
			continue
		}
		options = append(options, listed...)
	}
	if len(options) == 0 {
		return nil, nil, first
	}
	return options, notices, nil
}

func (c compareChoices) providerVoices(ctx context.Context, cfg config.Config, provider string) ([]review.VoiceOption, error) {
	speaker, err := c.app.speakerFor(ctx, cfg, provider)
	if err != nil {
		return nil, err
	}
	voices, err := speaker.Voices(ctx)
	if err != nil {
		return nil, c.app.SpeechFailure(err)
	}
	options := voiceOptions(provider, voices)
	for i := range options {
		options[i].Provider = provider
	}
	return options, nil
}

func notListed(provider string, err error) string {
	label := strings.ToUpper(provider[:1]) + provider[1:]
	msg := fmt.Sprintf("%s voices are not listed: %s", label, err.Error())
	if hint := failure.Hint(err); hint != "" {
		msg += " (" + hint + ")"
	}
	return msg
}

func (c compareChoices) Use(choice review.Choice) error {
	cfg, err := c.app.Config()
	if err != nil {
		return err
	}
	if _, err := voiceFor(cfg, choice.Provider, choice.Voice); err != nil {
		return err
	}
	ws, err := c.app.Workspace()
	if err != nil {
		return err
	}
	if err := config.SetFile(ws.ConfigFile(), choice.Provider+".voice", choice.Voice); err != nil {
		return err
	}
	if choice.Provider != cfg.Audio.Provider {
		if err := config.SetFile(ws.ConfigFile(), "audio.provider", choice.Provider); err != nil {
			return err
		}
	}
	c.app.InvalidateConfig()
	return nil
}

type voiceSampler struct {
	app      *App
	mu       sync.Mutex
	speakers map[string]clips.Speaker
}

func (s *voiceSampler) speak(ctx context.Context, cfg config.Config, provider string) (clips.Speaker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if speaker, ok := s.speakers[provider]; ok {
		return speaker, nil
	}
	speaker, err := s.app.speakerFor(context.WithoutCancel(ctx), cfg, provider)
	if err != nil {
		return nil, err
	}
	if s.speakers == nil {
		s.speakers = map[string]clips.Speaker{}
	}
	s.speakers[provider] = speaker
	return speaker, nil
}

func (s *voiceSampler) Sample(ctx context.Context, choice review.Choice, text string) (string, error) {
	ws, err := s.app.Workspace()
	if err != nil {
		return "", err
	}
	cfg, err := s.app.Config()
	if err != nil {
		return "", err
	}
	voice, err := voiceFor(cfg, choice.Provider, choice.Voice)
	if err != nil {
		return "", err
	}
	key := clips.Key(voice, text)
	path := filepath.Join(ws.VoiceSamplesDir(), clips.FileName(key))
	if fsutil.Exists(path) {
		return path, nil
	}
	speaker, err := s.speak(ctx, cfg, choice.Provider)
	if err != nil {
		return "", err
	}
	data, err := speaker.Synthesize(ctx, clips.SpeechRequest{Text: text, Voice: choice.Voice, Rate: voice.Rate})
	if err != nil {
		return "", s.app.SpeechFailure(err)
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
	return review.CompareDeps{Choices: compareChoices{app: a}, Sampler: &voiceSampler{app: a}, Samples: samples, Font: assets.Font}, nil
}
