package deck

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/scuba-plaza/mufradat/internal/clips"
	"github.com/scuba-plaza/mufradat/internal/note"
)

var voice = clips.Voice{Name: "ar-XA-Test", Rate: 0.9}

func mk(id string, seq, rank int, lemma, english string) note.Note {
	return note.Note{
		ID: id, Seq: seq, Rank: rank, Origin: note.OriginRanked, Lemma: lemma, POS: note.Noun,
		Forms:     []note.Form{{Label: "pl.", Arabic: lemma + "ات"}},
		English:   english,
		Example:   "الْكِتَابُ [فِي] " + lemma + ".",
		ExampleEN: "The book is in the bag.",
		Created:   time.Unix(1_700_000_000, 0).UTC(),
		Updated:   time.Unix(1_700_000_000+int64(seq), 0).UTC(),
	}
}

type fakeClips struct {
	dir  string
	have map[string]bool
}

func (f fakeClips) Has(key string) bool    { return f.have[key] }
func (f fakeClips) Path(key string) string { return filepath.Join(f.dir, key+".mp3") }

func fixture(t *testing.T) ([]note.Note, Options) {
	t.Helper()
	notes := []note.Note{
		mk("n_aaaaaaaaaaaa", 1, 3, "كِتَاب", "book"),
		mk("n_bbbbbbbbbbbb", 2, 1, "قَلَم", "pen"),
		mk("n_cccccccccccc", 3, 0, "بَيْت", "house"),
	}
	notes[2].Origin = note.OriginNamed
	dir := t.TempDir()
	have := map[string]bool{}
	for _, n := range notes[:2] {
		for _, it := range clips.Wanted(n, voice) {
			have[it.Key] = true
			if err := os.WriteFile(filepath.Join(dir, it.Key+".mp3"), []byte("mp3:"+it.Text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return notes, Options{
		DeckName: "Arabic MSA Core", DeckID: 1_760_000_000_000, ModelID: 1_760_000_000_001,
		ProductionTop: 2, UnrankedBase: 5000, Voice: voice, Clips: fakeClips{dir: dir, have: have},
		Check: func(n note.Note) Check {
			if n.ID == "n_aaaaaaaaaaaa" {
				return Check{Verified: true, Flags: []Flag{{Severity: Major, Word: "فِي", Detail: "CATT reads فِى"}}}
			}
			if n.ID == "n_bbbbbbbbbbbb" {
				return Check{Verified: true}
			}
			return Check{}
		},
	}
}

func buildBytes(t *testing.T, notes []note.Note, opts Options) ([]byte, Summary) {
	t.Helper()
	pkg, sum, err := Build(notes, opts)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := pkg.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), sum
}

func TestBuildIsByteIdentical(t *testing.T) {
	notes, opts := fixture(t)
	a, _ := buildBytes(t, notes, opts)
	b, _ := buildBytes(t, notes, opts)
	if !bytes.Equal(a, b) {
		t.Fatalf("two builds of the same input differ (%d vs %d bytes)", len(a), len(b))
	}
	reversed := []note.Note{notes[2], notes[1], notes[0]}
	c, _ := buildBytes(t, reversed, opts)
	if !bytes.Equal(a, c) {
		t.Fatal("input order must not change the package")
	}
}

func TestEpochOverridesModTimes(t *testing.T) {
	notes, opts := fixture(t)
	opts.Epoch = time.Unix(1_800_000_000, 0).UTC()
	a, _ := buildBytes(t, notes, opts)
	for i := range notes {
		notes[i].Updated = notes[i].Updated.Add(time.Hour)
	}
	b, _ := buildBytes(t, notes, opts)
	if !bytes.Equal(a, b) {
		t.Fatal("with an epoch set, edits to Updated must not change the bytes")
	}
}

type collection struct {
	notes map[string]noteRow
	cards map[string][]cardRow
	media map[string]string
	model map[string]any
}

type noteRow struct {
	fields []string
	tags   string
}

type cardRow struct{ ord, due int }

func readPackage(t *testing.T, data []byte) collection {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		files[f.Name], _ = io.ReadAll(rc)
		_ = rc.Close()
	}
	dbPath := filepath.Join(t.TempDir(), "c.anki2")
	if err := os.WriteFile(dbPath, files["collection.anki2"], 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	col := collection{notes: map[string]noteRow{}, cards: map[string][]cardRow{}, media: map[string]string{}}
	rows, err := db.Query(`SELECT id, guid, tags, flds FROM notes`)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[int64]string{}
	for rows.Next() {
		var id int64
		var guid, tags, flds string
		if err := rows.Scan(&id, &guid, &tags, &flds); err != nil {
			t.Fatal(err)
		}
		ids[id] = guid
		col.notes[guid] = noteRow{fields: strings.Split(flds, "\x1f"), tags: tags}
	}
	_ = rows.Close()
	crows, err := db.Query(`SELECT nid, ord, due FROM cards ORDER BY ord`)
	if err != nil {
		t.Fatal(err)
	}
	for crows.Next() {
		var nid int64
		var ord, due int
		if err := crows.Scan(&nid, &ord, &due); err != nil {
			t.Fatal(err)
		}
		col.cards[ids[nid]] = append(col.cards[ids[nid]], cardRow{ord, due})
	}
	_ = crows.Close()
	var mapping map[string]string
	if err := json.Unmarshal(files["media"], &mapping); err != nil {
		t.Fatal(err)
	}
	for k, name := range mapping {
		col.media[name] = string(files[k])
	}
	var models string
	if err := db.QueryRow(`SELECT models FROM col`).Scan(&models); err != nil {
		t.Fatal(err)
	}
	var all map[string]map[string]any
	if err := json.Unmarshal([]byte(models), &all); err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		col.model = m
	}
	return col
}

func TestPackageContents(t *testing.T) {
	notes, opts := fixture(t)
	data, sum := buildBytes(t, notes, opts)
	col := readPackage(t, data)

	if sum.Notes != 3 || sum.Cards != 4 || sum.Production != 1 {
		t.Errorf("summary = %+v", sum)
	}
	if sum.AudioFiles != 6 || sum.MissingAudio != 3 {
		t.Errorf("audio summary = %+v", sum)
	}
	book := col.notes["n_aaaaaaaaaaaa"]
	f := func(name string) string { return book.fields[fieldIndex(name)] }
	if f("Arabic") != "كِتَاب" || f("English") != "book" || f("Pos") != "noun" || f("Rank") != "3" || f("NoteID") != "n_aaaaaaaaaaaa" {
		t.Errorf("fields = %q", book.fields)
	}
	if f("Example") != "الْكِتَابُ <b>فِي</b> كِتَاب." {
		t.Errorf("example = %q", f("Example"))
	}
	if !strings.Contains(f("Forms"), `class="lbl">pl.</span>`) || !strings.Contains(f("Forms"), "كِتَابات") {
		t.Errorf("forms = %q", f("Forms"))
	}
	if !strings.HasPrefix(f("WordAudio"), "[sound:") || !strings.HasPrefix(f("ExampleAudio"), "[sound:") {
		t.Errorf("audio fields = %q %q", f("WordAudio"), f("ExampleAudio"))
	}
	if !strings.Contains(f("Check"), "CATT reads") {
		t.Errorf("check = %q", f("Check"))
	}
	for _, tag := range []string{" pos::noun ", " rank::0001-0500 ", " check::diacritics "} {
		if !strings.Contains(book.tags, tag) {
			t.Errorf("tags %q lack %q", book.tags, tag)
		}
	}
	if pen := col.notes["n_bbbbbbbbbbbb"]; strings.Contains(pen.tags, "check::") || pen.fields[fieldIndex("Check")] != "" {
		t.Errorf("a clean note must carry no check tag: %q", pen.tags)
	}
	house := col.notes["n_cccccccccccc"]
	if !strings.Contains(house.tags, "rank::unranked") || !strings.Contains(house.tags, "check::unverified") {
		t.Errorf("unranked/unverified tags missing: %q", house.tags)
	}
	if house.fields[fieldIndex("WordAudio")] != "" {
		t.Errorf("a note without clips must have empty audio fields")
	}
}

func TestCardsAndDueOrder(t *testing.T) {
	notes, opts := fixture(t)
	data, _ := buildBytes(t, notes, opts)
	col := readPackage(t, data)
	if got := col.cards["n_bbbbbbbbbbbb"]; len(got) != 2 || got[0] != (cardRow{0, 1}) || got[1] != (cardRow{1, 21}) {
		t.Errorf("rank 1 cards = %+v", got)
	}
	if got := col.cards["n_aaaaaaaaaaaa"]; len(got) != 1 || got[0].due != 3 {
		t.Errorf("rank 3 is past production_top and must have one card: %+v", got)
	}
	if got := col.cards["n_cccccccccccc"]; len(got) != 1 || got[0].due != 5003 {
		t.Errorf("unranked due = %+v", got)
	}
}

func TestProductionOverride(t *testing.T) {
	notes, opts := fixture(t)
	yes, no := true, false
	notes[2].Production = &yes
	notes[1].Production = &no
	data, _ := buildBytes(t, notes, opts)
	col := readPackage(t, data)
	if len(col.cards["n_cccccccccccc"]) != 2 || len(col.cards["n_bbbbbbbbbbbb"]) != 1 {
		t.Errorf("overrides ignored: %+v", col.cards)
	}
}

func TestFieldsAreEscaped(t *testing.T) {
	notes, opts := fixture(t)
	notes[0].English = `<script>alert("x")</script> & more`
	notes[0].ExampleEN = "a < b"
	notes[0].Hint = "(not سَنَة) <i>"
	data, _ := buildBytes(t, notes, opts)
	book := readPackage(t, data).notes["n_aaaaaaaaaaaa"]
	f := func(name string) string { return book.fields[fieldIndex(name)] }
	if strings.Contains(f("English"), "<script>") || !strings.Contains(f("English"), "&lt;script&gt;") {
		t.Errorf("english not escaped: %q", f("English"))
	}
	if f("ExampleEnglish") != "a &lt; b" {
		t.Errorf("example_en = %q", f("ExampleEnglish"))
	}
	if strings.Contains(f("Hint"), "<i>") || !strings.Contains(f("Hint"), `<span class="ar"`) {
		t.Errorf("hint = %q", f("Hint"))
	}
}

func TestMediaIncludesFontAndOnlyExistingClips(t *testing.T) {
	notes, opts := fixture(t)
	data, _ := buildBytes(t, notes, opts)
	col := readPackage(t, data)
	if _, ok := col.media[FontMediaName]; !ok {
		t.Error("the font must ship in the package")
	}
	clipCount := 0
	for name, content := range col.media {
		if strings.HasSuffix(name, ".mp3") {
			clipCount++
			if !strings.HasPrefix(content, "mp3:") {
				t.Errorf("clip %s has wrong content", name)
			}
		}
	}
	if clipCount != 6 {
		t.Errorf("clips in package = %d", clipCount)
	}
}

func TestModelIDFollowsSchemaVersion(t *testing.T) {
	if ModelID(100) == ModelID(101) {
		t.Error("different configured ids must map to different models")
	}
	first, again := ModelID(100), ModelID(100)
	if first != again {
		t.Error("model id must be stable")
	}
	nt := NoteType(ModelID(100))
	if len(nt.Fields) != len(fields) || len(nt.Templates) != 2 || !strings.Contains(nt.CSS, "Scheherazade") {
		t.Errorf("note type incomplete: %+v", nt)
	}
}

func TestBuildWithoutNotesFails(t *testing.T) {
	_, opts := fixture(t)
	if _, _, err := Build(nil, opts); err == nil {
		t.Error("an empty deck should be an error")
	}
}

func TestTemplateDigestIsStable(t *testing.T) {
	first, again := TemplateDigest(), TemplateDigest()
	if first != again || len(first) != 16 {
		t.Error("digest must be deterministic")
	}
}

func TestStylesheetInThePackageIsWellFormed(t *testing.T) {
	notes, opts := fixture(t)
	data, _ := buildBytes(t, notes, opts)
	css, _ := readPackage(t, data).model["css"].(string)
	if !strings.HasPrefix(css, "@font-face {") {
		t.Fatalf("the stylesheet must open with the font rule, it starts with %.40q", css)
	}
	depth, start := 0, 0
	for i, r := range css {
		switch r {
		case '{':
			if depth == 0 && strings.TrimSpace(css[start:i]) == "" {
				t.Fatalf("the rule opening at offset %d has no selector", i)
			}
			depth++
		case '}':
			depth--
			if depth < 0 {
				t.Fatalf("unbalanced closing brace at offset %d", i)
			}
			if depth == 0 {
				start = i + 1
			}
		}
	}
	if depth != 0 {
		t.Fatalf("the stylesheet leaves %d braces open", depth)
	}
	if !strings.Contains(css, ".card {") || !strings.Contains(css, ".sentence b {") {
		t.Error("expected rules are missing")
	}
}

func TestTemplateRevisionIsBumpedWithTheTemplates(t *testing.T) {
	const recorded = "419e2bea43d1e17d"
	if got := TemplateDigest(); got != recorded {
		t.Fatalf("the templates, stylesheet or font changed (digest %s): set TemplatesRevised to the current time and record this digest here, so that Anki replaces the note type it already has", got)
	}
}

func TestPackageModelIsNeverOlderThanTheTemplates(t *testing.T) {
	notes, opts := fixture(t)
	data, _ := buildBytes(t, notes, opts)
	mod, _ := readPackage(t, data).model["mod"].(float64)
	if int64(mod) < TemplatesRevised().Unix() {
		t.Fatalf("the model is dated %d, before the templates were revised at %d", int64(mod), TemplatesRevised().Unix())
	}
}

func TestCheckTagsFollowTheWorstFlagAndAreCounted(t *testing.T) {
	notes, opts := fixture(t)
	minor := Flag{Severity: Minor, Word: "فِي", Detail: "CATT reads فِى; CAMeL agrees with the card"}
	major := Flag{Severity: Major, Detail: "the ending has no vowel mark"}
	opts.Check = func(n note.Note) Check {
		switch n.ID {
		case "n_aaaaaaaaaaaa":
			return Check{Verified: true, Flags: []Flag{minor}}
		case "n_bbbbbbbbbbbb":
			return Check{Verified: true, Flags: []Flag{minor, major, minor}}
		}
		return Check{Verified: true}
	}
	data, sum := buildBytes(t, notes, opts)
	col := readPackage(t, data)
	tags := func(id string) string { return col.notes[id].tags }
	if !strings.Contains(tags("n_aaaaaaaaaaaa"), " check::diacritics-minor ") || strings.Contains(tags("n_aaaaaaaaaaaa"), " check::diacritics ") {
		t.Errorf("only minor flags: %q", tags("n_aaaaaaaaaaaa"))
	}
	if !strings.Contains(tags("n_bbbbbbbbbbbb"), " check::diacritics ") || strings.Contains(tags("n_bbbbbbbbbbbb"), "diacritics-minor") {
		t.Errorf("a major flag outranks minor ones: %q", tags("n_bbbbbbbbbbbb"))
	}
	if strings.Contains(tags("n_cccccccccccc"), "check::") {
		t.Errorf("a checked note with no flags carries no check tag: %q", tags("n_cccccccccccc"))
	}
	if want := map[string]int{"check::diacritics-minor": 1, "check::diacritics": 1}; !reflect.DeepEqual(sum.Tagged, want) {
		t.Errorf("tag counts %v, want %v", sum.Tagged, want)
	}
	check := col.notes["n_bbbbbbbbbbbb"].fields[fieldIndex("Check")]
	if strings.Count(check, "<br>") != 3 || !strings.HasPrefix(check, "<b>Check:</b><br>") || !strings.Contains(check, "the ending has no vowel mark") {
		t.Errorf("the check field lists every flag: %q", check)
	}
}

func TestTheStableIdentitiesAndDatesAreTheRecordedOnes(t *testing.T) {
	if got := ModelID(100); got != 1767785938850 {
		t.Errorf("ModelID(100) = %d: changing it makes Anki see a different note type (SchemaVersion %d)", got, SchemaVersion)
	}
	if got := TemplatesRevised(); !got.Equal(time.Date(2026, time.October, 9, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("TemplatesRevised = %v: it moves only together with the recorded template digest", got)
	}
	if ProductionDelay != 20 {
		t.Errorf("ProductionDelay = %d", ProductionDelay)
	}
}
