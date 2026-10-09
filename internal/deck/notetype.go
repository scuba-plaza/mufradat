package deck

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/scuba-plaza/mufradat/assets"
	"github.com/scuba-plaza/mufradat/internal/apkg"
)

const SchemaVersion = 1

const ModelName = "Arabic MSA Word"

const FontMediaName = "_" + assets.FontFileName

const (
	fieldArabic         = "Arabic"
	fieldEnglish        = "English"
	fieldHint           = "Hint"
	fieldForms          = "Forms"
	fieldExample        = "Example"
	fieldExampleEnglish = "ExampleEnglish"
	fieldWordAudio      = "WordAudio"
	fieldFormsAudio     = "FormsAudio"
	fieldExampleAudio   = "ExampleAudio"
	fieldPos            = "Pos"
	fieldDetails        = "Details"
	fieldProduction     = "Production"
	fieldCheck          = "Check"
	fieldRank           = "Rank"
	fieldNoteID         = "NoteID"
)

var fields = []string{
	fieldArabic, fieldEnglish, fieldHint, fieldForms, fieldExample, fieldExampleEnglish,
	fieldWordAudio, fieldFormsAudio, fieldExampleAudio, fieldPos, fieldDetails, fieldProduction,
	fieldCheck, fieldRank, fieldNoteID,
}

func fieldIndex(name string) int {
	i := slices.Index(fields, name)
	if i < 0 {
		panic("unknown note type field " + name)
	}
	return i
}

//go:embed templates/*
var templateFS embed.FS

func readTemplate(name string) string {
	raw, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func ModelID(configured int64) int64 {
	return apkg.StableID("model", strconv.FormatInt(configured, 10), strconv.Itoa(SchemaVersion))
}

func NoteType(modelID int64) apkg.Model {
	panels := readTemplate("panels.html")
	return apkg.Model{
		ID:     modelID,
		Name:   ModelName,
		Fields: slices.Clone(fields),
		RTL:    []string{fieldArabic, fieldExample},
		Templates: []apkg.Template{
			{Name: "Recognition", Front: readTemplate("recognition_front.html"), Back: readTemplate("recognition_back.html") + panels},
			{Name: "Production", Front: readTemplate("production_front.html"), Back: readTemplate("production_back.html") + panels},
		},
		CSS:     readTemplate("card.css"),
		Revised: TemplatesRevised(),
		Required: [][]int{
			{fieldIndex(fieldArabic)},
			{fieldIndex(fieldProduction), fieldIndex(fieldEnglish)},
		},
	}
}

func TemplatesRevised() time.Time {
	return time.Date(2026, time.October, 9, 12, 0, 0, 0, time.UTC)
}

func TemplateDigest() string {
	h := sha256.New()
	nt := NoteType(0)
	for _, t := range nt.Templates {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", t.Name, t.Front, t.Back)
	}
	fmt.Fprintf(h, "%s\x00%v\x00%d\x00", nt.CSS, nt.Fields, SchemaVersion)
	h.Write(assets.Font)
	return hex.EncodeToString(h.Sum(nil))[:16]
}
