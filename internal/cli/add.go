package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/scuba-plaza/mufradat/internal/app"
	"github.com/scuba-plaza/mufradat/internal/author"
	"github.com/scuba-plaza/mufradat/internal/failure"
	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/note"
)

const defaultAddCount = 10

type addFlags struct {
	file    string
	pick    int
	entries []string
	pos     string
	yes     bool
	dryRun  bool
	asJSON  bool
}

func newAdd(a *app.App) *cobra.Command {
	var f addFlags
	cmd := &cobra.Command{
		Use:   "add [N | WORD...]",
		Short: "Write the next most common words, or the words you name",
		Long: `Write notes with Claude Code and save each one as soon as it is finished.

Without words, add writes the N (default 10) most common words that have no
note yet. With words, it writes exactly those: give them as arguments,
separated by commas, or one per line in --file (- reads standard input).

A spelling can stand for several words, such as the noun ماء "water" and the
verb ماء "to meow". When more than one entry fits, add asks which you mean on a
terminal. Without one, pick with --entry WORD#N, --pos, --pick N (a single
word) or --yes (the first entry, which is the likeliest).

The headword, forms, gender and root come from the shipped Wiktionary data.
Claude writes the meaning, a hint where one is needed and a fully vowelled
example from common words. It runs 'claude -p' with your own Claude Code login,
so a Pro or Max plan is enough and no API key is needed.

If a run stops early, for example at your plan's usage limit, every finished
note is already saved and the message names the command that writes the rest.`,
		Example: `  mufradat add                 the next 10 most common words
  mufradat add 25              the next 25
  mufradat add كتاب عين        these two words, asking about senses
  mufradat add ماء --entry ماء#2   the verb
  mufradat add -f words.txt    every word in the file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, a, args, f)
		},
	}
	fl := cmd.Flags()
	fl.StringVarP(&f.file, "file", "f", "", "read the words from this file, one per line (# starts a comment; - is standard input)")
	fl.IntVar(&f.pick, "pick", 0, "take entry N of the only word given")
	fl.StringArrayVar(&f.entries, "entry", nil, "take entry N of a word, written WORD#N (repeatable)")
	fl.StringVar(&f.pos, "pos", "", "only consider entries of this part of speech")
	fl.BoolVarP(&f.yes, "yes", "y", false, "never ask: take the first entry of every word")
	fl.BoolVar(&f.dryRun, "dry-run", false, "show what would be written without calling Claude")
	fl.BoolVar(&f.asJSON, "json", false, "print machine-readable output")
	return cmd
}

type addReport struct {
	Written []addedNote `json:"written"`
	Skipped int         `json:"skipped"`
	Failed  []addFailed `json:"failed"`
	Present []string    `json:"already_present"`
	Missing []string    `json:"not_found"`
	Usage   llm.Usage   `json:"usage"`
}

type addedNote struct {
	ID      string `json:"id"`
	Rank    int    `json:"rank,omitempty"`
	Lemma   string `json:"lemma"`
	English string `json:"english"`
}

type addFailed struct {
	Lemma string `json:"lemma"`
	Error string `json:"error"`
}

func runAdd(cmd *cobra.Command, a *app.App, args []string, f addFlags) error {
	ctx := cmd.Context()
	r := a.Reporter()
	count, words, err := parseAddArgs(a, args, f.file)
	if err != nil {
		return err
	}
	st, err := a.Store()
	if err != nil {
		return err
	}
	set, err := st.Load()
	if err != nil {
		return err
	}
	list, err := a.Ranking()
	if err != nil {
		return err
	}
	ix, err := a.Lexicon()
	if err != nil {
		return err
	}
	cfg, err := a.Config()
	if err != nil {
		return err
	}

	var report addReport
	var targets []author.Target
	if len(words) == 0 {
		targets = author.NextRanked(set, list, ix, count)
		if len(targets) == 0 {
			r.Event("every common word that has an entry already has a note")
			return nil
		}
	} else {
		sel, err := newSelector(a, f, len(words))
		if err != nil {
			return err
		}
		var ambiguous []*author.AmbiguousError
		for _, w := range words {
			cand, err := sel.Select(w, ix.Find(w))
			var amb *author.AmbiguousError
			var nf *author.NotFoundError
			switch {
			case errors.As(err, &amb):
				ambiguous = append(ambiguous, amb)
				continue
			case errors.As(err, &nf):
				report.Missing = append(report.Missing, w)
				continue
			case errors.Is(err, errStopped):
				r.Event("stopped; nothing was added")
				return nil
			case err != nil:
				return failure.Usage(err)
			}
			pl, ok := author.Place(cand, w, set, list)
			switch {
			case !ok:
				report.Missing = append(report.Missing, w)
			case pl.Existing != nil:
				report.Present = append(report.Present, pl.Existing.Lemma)
			default:
				targets = append(targets, pl.Target)
			}
		}
		if len(ambiguous) > 0 {
			return ambiguityError(a, ambiguous)
		}
		targets = author.Dedupe(targets)
		for _, p := range report.Present {
			r.Event("%s is already in the deck", p)
		}
		if len(targets) == 0 {
			if len(report.Missing) > 0 {
				return notFoundError(report.Missing)
			}
			r.Event("nothing to add")
			return nil
		}
	}

	if f.dryRun {
		for _, t := range targets {
			fmt.Fprintf(a.Env.Stdout, "%s\t%s\t%s\n", rankLabel(t.Draft.Rank), t.Draft.Lemma, t.Draft.POS)
		}
		r.Event("would write %s", counted(len(targets), "note", "notes"))
		return nil
	}

	backend, err := a.Claude()
	if err != nil {
		return err
	}
	au, err := a.Author(backend)
	if err != nil {
		return err
	}
	system, err := a.AuthorSystem()
	if err != nil {
		return err
	}
	r.Event("writing %s with Claude Code (%s, %s effort, %d per request)", counted(len(targets), "note", "notes"), cfg.Author.Model, cfg.Author.Effort, cfg.Author.BatchSize)

	res, runErr := au.Run(ctx, targets, author.Options{
		Batch: cfg.Author.BatchSize, Concurrency: cfg.Author.Concurrency, System: system,
		Progress: func(e author.Event) {
			switch {
			case e.Err != nil:
				r.Warn("[%d/%d] %s failed: %v", e.Done, e.Total, e.Target.Draft.Lemma, e.Err)
			case e.Skipped:
				r.Event("[%d/%d] %s was added by someone else meanwhile", e.Done, e.Total, e.Target.Draft.Lemma)
			default:
				r.Event("[%d/%d] %s  %s", e.Done, e.Total, e.Note.Lemma, e.Note.English)
			}
		},
	})
	report.Skipped = res.Skipped
	report.Usage = res.Usage
	for _, n := range res.Written {
		report.Written = append(report.Written, addedNote{ID: n.ID, Rank: n.Rank, Lemma: n.Lemma, English: n.English})
	}
	for _, fl := range res.Failed {
		report.Failed = append(report.Failed, addFailed{Lemma: fl.Target.Draft.Lemma, Error: fl.Err.Error()})
	}
	if f.asJSON {
		if err := writeJSON(a.Env.Stdout, report); err != nil {
			return err
		}
	} else {
		for _, n := range report.Written {
			fmt.Fprintf(a.Env.Stdout, "%s\t%s\t%s\n", n.ID, n.Lemma, n.English)
		}
	}
	u := res.Usage
	if u.Calls > 0 {
		r.Event("wrote %s in %s; tokens: %d input, %d cached, %d output", counted(len(res.Written), "note", "notes"), counted(u.Calls, "request", "requests"), u.Input, u.CacheRead, u.Output)
	}

	if runErr != nil {
		return a.AuthorFailure(withResume(runErr, a, res.Unwritten, len(words) > 0))
	}
	if len(report.Failed) > 0 {
		return failure.Newf(failure.ExitFailure, "run "+resumeCommand(a, retryTargets(res), len(words) > 0)+" to try again", "%s could not be written", counted(len(report.Failed), "note", "notes"))
	}
	if len(report.Missing) > 0 {
		return notFoundError(report.Missing)
	}
	r.Event("next: %s status", a.CommandPrefix())
	return nil
}

func retryTargets(res author.Result) []author.Target {
	out := slices.Clone(res.Unwritten)
	for _, f := range res.Failed {
		out = append(out, f.Target)
	}
	return out
}

func withResume(err error, a *app.App, rest []author.Target, named bool) error {
	if len(rest) > 0 {
		a.Reporter().Event("to write the rest, run: %s", resumeCommand(a, rest, named))
	}
	return err
}

func resumeCommand(a *app.App, targets []author.Target, named bool) string {
	cmd := a.CommandPrefix() + " add"
	if !named {
		return fmt.Sprintf("%s %d", cmd, len(targets))
	}
	var words []string
	for _, t := range targets {
		if t.Typed != "" && !slices.Contains(words, t.Typed) {
			words = append(words, t.Typed)
		}
	}
	for _, w := range words {
		if strings.ContainsAny(w, " \t") {
			w = `"` + w + `"`
		}
		cmd += " " + w
	}
	return cmd
}

func parseAddArgs(a *app.App, args []string, file string) (int, []string, error) {
	count := defaultAddCount
	if len(args) == 1 && file == "" {
		if n, err := strconv.Atoi(args[0]); err == nil {
			if n < 1 {
				return 0, nil, failure.Usage(errors.New("the number of words must be at least 1"))
			}
			return n, nil, nil
		}
	}
	var raw []string
	raw = append(raw, args...)
	if file != "" {
		lines, err := readLines(a, file)
		if err != nil {
			return 0, nil, err
		}
		raw = append(raw, lines...)
	}
	var words []string
	for _, chunk := range raw {
		for _, w := range strings.FieldsFunc(chunk, func(r rune) bool { return r == ',' || r == '،' || r == '\n' }) {
			w = lexicon.NormalizeWord(w)
			if w != "" && !slices.Contains(words, w) {
				words = append(words, w)
			}
		}
	}
	if (len(args) > 0 || file != "") && len(words) == 0 {
		return 0, nil, failure.Usage(errors.New("there are no words to add"))
	}
	return count, words, nil
}

func readLines(a *app.App, file string) ([]string, error) {
	in := a.Env.Stdin
	if file != "-" {
		fh, err := os.Open(file)
		if err != nil {
			return nil, failure.Usage(err)
		}
		defer fh.Close()
		in = fh
	}
	var lines []string
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, sc.Err()
}

var errStopped = errors.New("stopped")

func newSelector(a *app.App, f addFlags, words int) (author.Selector, error) {
	sel := author.Selector{Pick: f.pick, Yes: f.yes, Entries: map[string]int{}}
	if f.pick > 0 && words != 1 {
		return sel, failure.Usage(errors.New("--pick works with exactly one word; use --entry WORD#N for several"))
	}
	if f.pos != "" {
		sel.POS = note.POS(f.pos)
		if !sel.POS.Valid() {
			return sel, failure.Usage(fmt.Errorf("--pos %q must be one of %s", f.pos, strings.Join(note.POSStrings(), ", ")))
		}
	}
	for _, e := range f.entries {
		word, n, err := author.ParseEntrySelector(e)
		if err != nil {
			return sel, failure.Usage(err)
		}
		sel.Entries[word] = n
	}
	if a.Env.Interactive && !f.yes {
		sel.Chooser = &promptChooser{in: bufio.NewReader(a.Env.Stdin), out: a.Env.Stderr}
	}
	return sel, nil
}

type promptChooser struct {
	in  *bufio.Reader
	out io.Writer
}

func (c *promptChooser) Choose(word string, cands []lexicon.Candidate) (int, error) {
	fmt.Fprintf(c.out, "%s matches %d entries:\n", word, len(cands))
	for i, cand := range cands {
		fmt.Fprintf(c.out, "  %d. %s\n", i+1, author.Describe(cand, 3))
	}
	for {
		fmt.Fprint(c.out, "which one? [1, q to stop] ")
		line, err := c.in.ReadString('\n')
		line = strings.TrimSpace(line)
		switch {
		case line == "q" || (err != nil && line == ""):
			return 0, errStopped
		case line == "":
			return 1, nil
		}
		if n, convErr := strconv.Atoi(line); convErr == nil && n >= 1 && n <= len(cands) {
			return n, nil
		}
		fmt.Fprintf(c.out, "enter a number from 1 to %d\n", len(cands))
		if err != nil {
			return 0, errStopped
		}
	}
}

func ambiguityError(a *app.App, all []*author.AmbiguousError) error {
	var b strings.Builder
	if len(all) == 1 {
		b.WriteString("1 word needs a choice\n")
	} else {
		fmt.Fprintf(&b, "%d words need a choice\n", len(all))
	}
	for _, amb := range all {
		fmt.Fprintf(&b, "\n  %s matches %d entries:\n", amb.Word, len(amb.Candidates))
		for i, c := range amb.Candidates {
			fmt.Fprintf(&b, "    %s#%d  %s\n", amb.Word, i+1, author.Describe(c, 3))
		}
	}
	return failure.Newf(failure.ExitUsage, "choose with --entry WORD#N or --pos, or use --yes to take the first entry of every word", "%s", strings.TrimRight(b.String(), "\n"))
}

func notFoundError(words []string) error {
	return failure.Newf(failure.ExitFailure, "the shipped data covers Modern Standard Arabic words; check the spelling", "no Modern Standard Arabic entry matches: %s", strings.Join(words, "، "))
}

func rankLabel(rank int) string {
	if rank <= 0 {
		return "-"
	}
	return strconv.Itoa(rank)
}

func counted(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
