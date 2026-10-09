package deck

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/scuba-plaza/mufradat/assets"
	"github.com/scuba-plaza/mufradat/internal/apkg"
	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/note"
)

const (
	ProductionDelay = 20
	description     = "The most common Modern Standard Arabic words in frequency order, with fully vowelled examples. Cards tagged check::diacritics had conflicting vowel readings from independent sources."
)

type Severity string

const (
	Major Severity = "major"
	Minor Severity = "minor"
)

type Flag struct {
	Severity Severity
	Word     string
	Detail   string
}

type Check struct {
	Verified bool
	Flags    []Flag
}

type ClipSource interface {
	Has(key string) bool
	Path(key string) string
}

type Options struct {
	DeckName      string
	DeckID        int64
	ModelID       int64
	ProductionTop int
	UnrankedBase  int
	Voice         clips.Voice
	Clips         ClipSource
	Check         func(note.Note) Check
	Epoch         time.Time
}

type Summary struct {
	Notes        int
	Cards        int
	Production   int
	AudioFiles   int
	MissingAudio int
	Tagged       map[string]int
}

func Build(notes []note.Note, opts Options) (*apkg.Package, Summary, error) {
	pkg := &apkg.Package{
		Deck:    apkg.Deck{ID: opts.DeckID, Name: opts.DeckName, Description: description},
		Model:   NoteType(ModelID(opts.ModelID)),
		Created: time.UnixMilli(opts.DeckID).UTC(),
		Media: []apkg.Media{{
			Name: FontMediaName,
			Open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(assets.Font)), nil },
		}},
	}
	summary := Summary{Tagged: map[string]int{}}
	ordered := slices.Clone(notes)
	slices.SortFunc(ordered, note.CompareStudy)
	mediaSeen := map[string]bool{}

	for _, n := range ordered {
		values := make([]string, len(fields))
		set := func(name, value string) { values[fieldIndex(name)] = value }
		set(fieldArabic, html.EscapeString(n.Lemma))
		set(fieldEnglish, html.EscapeString(n.English))
		set(fieldHint, markArabic(n.Hint))
		set(fieldForms, formsHTML(n))
		set(fieldExample, exampleHTML(n))
		set(fieldExampleEnglish, html.EscapeString(n.ExampleEN))
		set(fieldPos, n.POS.Label())
		set(fieldDetails, details(n))
		set(fieldRank, strconv.Itoa(n.Rank))
		set(fieldNoteID, n.ID)

		if opts.Clips != nil {
			for _, item := range clips.Wanted(n, opts.Voice) {
				if !opts.Clips.Has(item.Key) {
					summary.MissingAudio++
					continue
				}
				file := clips.FileName(item.Key)
				if !mediaSeen[file] {
					mediaSeen[file] = true
					path := opts.Clips.Path(item.Key)
					pkg.Media = append(pkg.Media, apkg.Media{Name: file, Open: func() (io.ReadCloser, error) { return os.Open(path) }})
				}
				summary.AudioFiles++
				set(string(item.Field), "[sound:"+file+"]")
			}
		}

		tags := []string{"pos::" + string(n.POS), rankBucket(n.Rank)}
		check := Check{}
		if opts.Check != nil {
			check = opts.Check(n)
		}
		var lines []string
		if !check.Verified {
			tags = append(tags, "check::unverified")
			lines = append(lines, "Vowels not cross-checked yet.")
		} else {
			major, minor := false, false
			for _, f := range check.Flags {
				if f.Severity == Major {
					major = true
				} else {
					minor = true
				}
				line := html.EscapeString(f.Detail)
				if f.Word != "" {
					line = arabicSpan(html.EscapeString(f.Word)) + ": " + markArabic(f.Detail)
				}
				lines = append(lines, line)
			}
			switch {
			case major:
				tags = append(tags, "check::diacritics")
			case minor:
				tags = append(tags, "check::diacritics-minor")
			}
		}
		set(fieldCheck, checkHTML(lines))
		for _, t := range tags {
			if strings.HasPrefix(t, "check::") {
				summary.Tagged[t]++
			}
		}

		due := n.Rank
		if n.Rank <= 0 {
			due = opts.UnrankedBase + n.Seq
		}
		cards := []apkg.Card{{Ord: 0, Due: due}}
		if n.WantsProduction(opts.ProductionTop) {
			set(fieldProduction, "y")
			cards = append(cards, apkg.Card{Ord: 1, Due: due + ProductionDelay})
			summary.Production++
		}
		mod := n.Updated
		if !opts.Epoch.IsZero() {
			mod = opts.Epoch
		}
		pkg.Notes = append(pkg.Notes, apkg.Note{GUID: n.ID, Fields: values, Tags: tags, Cards: cards, Mod: mod})
		summary.Notes++
		summary.Cards += len(cards)
	}
	if len(pkg.Notes) == 0 {
		return nil, summary, fmt.Errorf("there are no notes to build")
	}
	return pkg, summary, nil
}
