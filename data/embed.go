package data

import (
	"bytes"
	_ "embed"
	"io"
)

//go:embed ranking.tsv
var rankingTSV []byte

//go:embed lexicon.jsonl.gz
var lexiconGZ []byte

//go:embed forms.tsv.gz
var formsGZ []byte

func Ranking() io.Reader { return bytes.NewReader(rankingTSV) }

func Lexicon() io.Reader { return bytes.NewReader(lexiconGZ) }

func Forms() io.Reader { return bytes.NewReader(formsGZ) }
