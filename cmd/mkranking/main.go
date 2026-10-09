package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/jsonl"
	"github.com/scuba-plaza/mufradat/internal/pyenv"
	"github.com/scuba-plaza/mufradat/internal/xdg"
	"github.com/scuba-plaza/mufradat/python"
)

const (
	camelFile   = "camel-lemmas.tsv"
	lockFile    = "sources.jsonl"
	curationDir = "curation"
)

type options struct {
	cache   string
	data    string
	python  string
	sources []source
	limit   int
	senses  int
	refresh bool
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, env.OS(), os.Args[1:])
	stop()
	os.Exit(code)
}

func parse(e env.Env, args []string) (options, error) {
	dirs := xdg.New(e.Getenv, e.Home)
	opts := options{}
	fs := flag.NewFlagSet("mkranking", flag.ContinueOnError)
	fs.SetOutput(e.Stderr)
	opts.sources = sources
	fs.StringVar(&opts.cache, "cache", filepath.Join(dirs.Cache, "mkranking"), "where the downloaded sources and the CAMeL analysis are kept")
	fs.StringVar(&opts.data, "data", "data", "the repository's data directory to write")
	fs.StringVar(&opts.python, "python", "", "interpreter with camel-tools (default: the one `mufradat setup` installed)")
	fs.IntVar(&opts.limit, "limit", 5000, "number of words to rank")
	fs.IntVar(&opts.senses, "senses", 6, "senses kept per Wiktionary entry")
	fs.BoolVar(&opts.refresh, "refresh", false, "download the sources and analyse them again")
	fs.Usage = func() {
		fmt.Fprintln(e.Stderr, "usage: mkranking [flags]\n\nRebuilds ranking.tsv, lexicon.jsonl.gz and forms.tsv.gz in the data directory from\nthe Wiktionary dump and the two frequency lists, and records their checksums in\nsources.jsonl. Maintainers only; users never run it.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if opts.limit < 1 || opts.senses < 1 {
		return options{}, errors.New("-limit and -senses must be at least 1")
	}
	return opts, nil
}

func run(ctx context.Context, e env.Env, args []string) int {
	opts, err := parse(e, args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(e.Stderr, "mkranking:", err)
		return 2
	}
	if err := build(ctx, e, opts); err != nil {
		fmt.Fprintln(e.Stderr, "mkranking:", err)
		return 1
	}
	return 0
}

func build(ctx context.Context, e env.Env, opts options) error {
	say := func(format string, a ...any) { fmt.Fprintf(e.Stderr, format+"\n", a...) }
	previous, err := readLock(filepath.Join(opts.data, lockFile))
	if err != nil {
		return err
	}
	var records []locked
	paths := map[string]string{}
	for _, s := range opts.sources {
		path := filepath.Join(opts.cache, s.File)
		paths[s.File] = path
		if opts.refresh || !fsutil.Exists(path) {
			say("downloading %s", s.Name)
			if err := fetch(ctx, e.HTTP, s, path); err != nil {
				return err
			}
		}
		rec, err := lockOf(s.Name, s.URL, path)
		if err != nil {
			return err
		}
		for _, p := range previous {
			if p.File == rec.File && p.SHA256 != rec.SHA256 {
				say("note: %s differs from the checksum recorded in %s", s.Name, lockFile)
			}
		}
		records = append(records, rec)
	}

	camel := filepath.Join(opts.cache, camelFile)
	if opts.refresh || !fsutil.Exists(camel) {
		say("analysing the frequent word forms with CAMeL Tools; this takes a few minutes")
		if err := runAnalysis(ctx, e, opts, paths, camel); err != nil {
			return err
		}
	}
	rec, err := lockOf("CAMeL Tools MLE analysis of the frequency lists", "", camel)
	if err != nil {
		return err
	}
	records = append(records, rec)

	say("ranking")
	out, err := generate(inputs{
		Wiktionary: paths[opts.sources[0].File],
		Subtitles:  paths[opts.sources[1].File],
		MSA:        paths[opts.sources[2].File],
		Camel:      camel,
		Essentials: filepath.Join(opts.data, curationDir, "essentials.tsv"),
		Overrides:  filepath.Join(opts.data, curationDir, "overrides.tsv"),
	}, opts.limit, opts.senses)
	if err != nil {
		return err
	}

	files := []struct {
		path string
		data []byte
	}{
		{filepath.Join(opts.data, "ranking.tsv"), out.Ranking},
		{filepath.Join(opts.data, "lexicon.jsonl.gz"), out.Lexicon},
		{filepath.Join(opts.data, "forms.tsv.gz"), out.Forms},
		{filepath.Join(opts.cache, "ranking-detail.tsv"), out.Detail},
		{filepath.Join(opts.cache, "rank-report.tsv"), out.Decisions},
	}
	for _, f := range files {
		if err := fsutil.WriteFileAtomic(f.path, f.data, 0o644); err != nil {
			return err
		}
	}
	if err := jsonl.WriteFile(filepath.Join(opts.data, lockFile), records); err != nil {
		return err
	}
	say("%d lemma groups; ranked %d; shipped %d (ranking %s, lexicon %s, forms %s)",
		out.Groups, out.Ranked, out.Shipped, size(out.Ranking), size(out.Lexicon), size(out.Forms))
	say("disputed word forms, most frequent first: %s", filepath.Join(opts.cache, "rank-report.tsv"))
	return nil
}

func size(b []byte) string {
	switch n := len(b); {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
}

func runAnalysis(ctx context.Context, e env.Env, opts options, paths map[string]string, out string) error {
	dirs := xdg.New(e.Getenv, e.Home)
	mgr := &pyenv.Manager{Dirs: dirs, Source: python.Files, Runner: e.Runner, Now: e.Now}
	interpreter := opts.python
	if interpreter == "" {
		rt, err := mgr.Resolve("")
		if err != nil {
			return fmt.Errorf("%w; run `mufradat setup`, or pass -python", err)
		}
		interpreter = rt.Python
	}
	lists := []string{
		fmt.Sprintf("%s:%d", paths[opts.sources[1].File], subtitlesLimit),
		fmt.Sprintf("%s:%d", paths[opts.sources[2].File], msaLimit),
	}
	return analyse(ctx, e.Runner, interpreter, mgr.Environ(""), out, lists, io.Writer(e.Stderr))
}
