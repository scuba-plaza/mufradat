package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/rankgen"
	"github.com/scuba-plaza/mufradat/internal/wiktionary"
)

const (
	subtitlesLimit = 50000
	msaLimit       = 100000
	reportLimit    = 3000
)

type inputs struct {
	Wiktionary string
	Subtitles  string
	MSA        string
	Camel      string
	Essentials string
	Overrides  string
}

type artifacts struct {
	Ranking   []byte
	Lexicon   []byte
	Forms     []byte
	Detail    []byte
	Decisions []byte
	Groups    int
	Ranked    int
	Shipped   int
}

func open[T any](path string, read func(f *os.File) (T, error)) (T, error) {
	f, err := os.Open(path)
	if err != nil {
		var zero T
		return zero, err
	}
	defer f.Close()
	v, err := read(f)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return v, nil
}

func generate(in inputs, limit, senses int) (artifacts, error) {
	entries, err := open(in.Wiktionary, func(f *os.File) ([]*lexicon.Entry, error) { return wiktionary.Read(f) })
	if err != nil {
		return artifacts{}, err
	}
	lemmas := wiktionary.Group(entries)
	ix := rankgen.NewIndex(lemmas)

	var corpora []*rankgen.Corpus
	for _, c := range []struct {
		name  string
		path  string
		limit int
	}{{"subtitles", in.Subtitles, subtitlesLimit}, {"msa", in.MSA, msaLimit}} {
		corpus, err := open(c.path, func(f *os.File) (*rankgen.Corpus, error) { return rankgen.ReadCounts(c.name, f, c.limit) })
		if err != nil {
			return artifacts{}, err
		}
		corpora = append(corpora, corpus)
	}
	camel, err := open(in.Camel, func(f *os.File) (map[string]rankgen.CamelAnalysis, error) { return rankgen.ReadCamel(f) })
	if err != nil {
		return artifacts{}, err
	}
	essentials, err := open(in.Essentials, func(f *os.File) ([]rankgen.Essential, error) { return rankgen.ReadEssentials(f) })
	if err != nil {
		return artifacts{}, err
	}
	overrides, err := open(in.Overrides, func(f *os.File) ([]rankgen.Override, error) { return rankgen.ReadOverrides(f) })
	if err != nil {
		return artifacts{}, err
	}

	result, err := rankgen.Rank(ix, corpora, camel, essentials, overrides, rankgen.Options{Limit: limit})
	if err != nil {
		return artifacts{}, err
	}
	shipped, err := rankgen.Ship(result.Rows, lemmas, senses)
	if err != nil {
		return artifacts{}, err
	}
	ranked := shipped[:len(result.Rows)]

	var names []string
	for _, c := range corpora {
		names = append(names, c.Name)
	}
	var out artifacts
	out.Groups, out.Ranked, out.Shipped = len(lemmas), len(ranked), len(shipped)
	var ranking, lexicon, forms, detail, decisions bytes.Buffer
	steps := []func() error{
		func() error { return rankgen.WriteRanking(&ranking, ranked) },
		func() error { return rankgen.WriteLexicon(&lexicon, shipped) },
		func() error { return rankgen.WriteForms(&forms, shipped) },
		func() error { return rankgen.WriteDetail(&detail, result.Rows, names) },
		func() error {
			return rankgen.WriteDecisions(&decisions, ix, result.Decisions, reportLimit, map[string]bool{"contested": true, "proper-noun": true})
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return artifacts{}, err
		}
	}
	out.Ranking, out.Lexicon, out.Forms = ranking.Bytes(), lexicon.Bytes(), forms.Bytes()
	out.Detail, out.Decisions = detail.Bytes(), decisions.Bytes()
	return out, nil
}
