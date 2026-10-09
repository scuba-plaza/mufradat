package rankgen

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/morph"
	"github.com/scuba-plaza/mufradat/internal/wiktionary"
)

type form struct {
	Form   string   `json:"form"`
	Tags   []string `json:"tags"`
	Source string   `json:"source,omitempty"`
}

type sense struct {
	Glosses []string `json:"glosses"`
	Tags    []string `json:"tags,omitempty"`
}

type entry struct {
	Word   string  `json:"word"`
	Pos    string  `json:"pos"`
	Forms  []form  `json:"forms"`
	Senses []sense `json:"senses"`
}

func dump(t *testing.T, entries ...entry) string {
	t.Helper()
	var b strings.Builder
	for _, e := range entries {
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String()
}

func lemmasOf(t *testing.T, jsonlDump string) []*lexicon.Lemma {
	t.Helper()
	entries, err := wiktionary.Read(strings.NewReader(jsonlDump))
	if err != nil {
		t.Fatal(err)
	}
	return wiktionary.Group(entries)
}

func fixtureLemmas(t *testing.T) []*lexicon.Lemma {
	t.Helper()
	return lemmasOf(t, dump(t,
		entry{Word: "كتاب", Pos: "noun", Forms: []form{
			{Form: "كِتَاب", Tags: []string{"canonical", "masculine"}},
			{Form: "كُتُب", Tags: []string{"plural"}},
			{Form: "الْكِتَاب", Tags: []string{"definite", "singular"}, Source: "declension"},
			{Form: "كِتَابًا", Tags: []string{"indefinite", "accusative"}, Source: "declension"},
			{Form: "الْكُتُب", Tags: []string{"definite", "plural"}, Source: "declension"},
		}, Senses: []sense{{Glosses: []string{"verbal noun of كَتَبَ"}, Tags: []string{"form-of"}}, {Glosses: []string{"book"}}}},
		entry{Word: "كتب", Pos: "verb", Forms: []form{
			{Form: "كَتَبَ", Tags: []string{"canonical", "form-i"}},
			{Form: "يَكْتُبُ", Tags: []string{"non-past"}},
			{Form: "كِتَابَة", Tags: []string{"noun-from-verb"}},
			{Form: "كَتَبْتُ", Tags: []string{"active", "first-person", "past"}, Source: "conjugation"},
			{Form: "يَكْتُبُ", Tags: []string{"active", "indicative", "third-person"}, Source: "conjugation"},
			{Form: "كَاتِب", Tags: []string{"active", "participle"}, Source: "conjugation"},
		}, Senses: []sense{{Glosses: []string{"to write"}}}},
		entry{Word: "ليس", Pos: "verb", Forms: []form{{Form: "لَيْسَ", Tags: []string{"canonical"}}}, Senses: []sense{{Glosses: []string{"not to be"}}}},
		entry{Word: "ليس", Pos: "verb", Forms: []form{{Form: "لَيِسَ", Tags: []string{"canonical", "form-i"}}}, Senses: []sense{{Glosses: []string{"to be valiant"}}}},
		entry{Word: "في", Pos: "prep", Forms: []form{
			{Form: "فِي", Tags: []string{"canonical"}},
			{Form: "فِيهِ", Tags: []string{"third-person"}, Source: "inflection"},
		}, Senses: []sense{{Glosses: []string{"in"}}}},
		entry{Word: "محمد", Pos: "name", Forms: []form{{Form: "مُحَمَّد", Tags: []string{"canonical"}}}, Senses: []sense{{Glosses: []string{"Muhammad"}}}},
		entry{Word: "أبث", Pos: "verb", Forms: []form{{Form: "أَبَثَ", Tags: []string{"canonical"}}}, Senses: []sense{{Glosses: []string{"to slander"}, Tags: []string{"obsolete"}}}},
		entry{Word: "و", Pos: "conj", Forms: []form{{Form: "وَ", Tags: []string{"canonical"}}}, Senses: []sense{{Glosses: []string{"and"}}}},
		entry{Word: "و", Pos: "character", Forms: []form{{Form: "و", Tags: []string{"canonical"}}}, Senses: []sense{{Glosses: []string{"letter waw"}}}},
	))
}

func fixture(t *testing.T) *Index {
	t.Helper()
	return NewIndex(fixtureLemmas(t))
}

func ids(ix *Index, lemmas []int) []string {
	var out []string
	for _, i := range lemmas {
		out = append(out, ix.Lemmas[i].ID)
	}
	slices.Sort(out)
	return out
}

func TestAnalyzeStripsClitics(t *testing.T) {
	ix := fixture(t)
	cases := map[string][]string{
		"الكتاب":  {"كِتَاب"},
		"بالكتاب": {"كِتَاب"},
		"للكتاب":  {"كِتَاب"},
		"وكتبها":  {"كَتَبَ", "كِتَاب"},
		"سيكتب":   {"كَتَبَ"},
		"فيه":     {"فِي"},
		"ليس":     {"لَيْسَ", "لَيِسَ"},
	}
	for word, want := range cases {
		got := ids(ix, ix.Analyze(word))
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("Analyze(%s) = %v, want %v", word, got, want)
		}
	}
	if got := ix.Analyze("بكتبت"); len(got) != 0 {
		t.Errorf("a preposition cannot attach to a verb, got %v", ids(ix, got))
	}
}

func corpora() []*Corpus {
	subs := &Corpus{Name: "subs", Total: 1000, Types: []Count{{"في", 500}, {"ليس", 100}, {"الكتاب", 50}, {"كتب", 30}, {"محمد", 40}, {"أبث", 5}}}
	msa := &Corpus{Name: "msa", Total: 2000, Types: []Count{{"في", 900}, {"ليس", 60}, {"الكتاب", 150}, {"كتب", 90}, {"محمد", 200}, {"أبث", 1}}}
	return []*Corpus{subs, msa}
}

func rankOf(rows []Row, id string) int {
	for _, r := range rows {
		if r.Lemma.ID == id {
			return r.Rank
		}
	}
	return 0
}

func rowIDs(rows []Row) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Lemma.ID)
	}
	return out
}

func TestRankOrdersByBlendedFrequencyAndFilters(t *testing.T) {
	ix := fixture(t)
	camel := map[string]CamelAnalysis{
		"ليس":    {Lex: "لَيْس", Pos: "verb"},
		"الكتاب": {Lex: "كِتاب", Pos: "noun"},
		"محمد":   {Lex: "مُحَمَّد", Pos: "noun_prop"},
	}
	res, err := Rank(ix, corpora(), camel, nil, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rows := res.Rows
	if rankOf(rows, "فِي") != 1 {
		t.Errorf("فِي should rank first, rows: %v", rowIDs(rows))
	}
	if r, v := rankOf(rows, "لَيْسَ"), rankOf(rows, "لَيِسَ"); r == 0 || (v != 0 && v < r) {
		t.Errorf("CAMeL evidence should put لَيْسَ (%d) above لَيِسَ (%d)", r, v)
	}
	for _, gone := range []string{"مُحَمَّد", "أَبَثَ"} {
		if rankOf(rows, gone) != 0 {
			t.Errorf("%s should be filtered out", gone)
		}
	}
	for _, r := range rows {
		if r.Lemma.ID == "كِتَاب" && r.Agree == 0 {
			t.Errorf("كِتَاب should have agreeing evidence from both methods")
		}
	}
}

func TestOverridesAndEssentials(t *testing.T) {
	ix := fixture(t)
	res, err := Rank(ix, corpora(), nil,
		[]Essential{{Arabic: "وَ", MaxRank: 1}, {Arabic: "مَرْحَبًا", MaxRank: 2, Pos: "intj", Gloss: "hello"}},
		[]Override{{Kind: OverrideLemma, From: "لَيِسَ", To: "لَيْسَ"}},
		Options{})
	if err != nil {
		t.Fatal(err)
	}
	rows := res.Rows
	if rankOf(rows, "وَ") != 1 {
		t.Errorf("وَ should be pinned to rank 1, rows: %v", rowIDs(rows))
	}
	if rankOf(rows, "مَرْحَبًا") != 2 {
		t.Errorf("an essential missing from Wiktionary should still be placed, rows: %v", rowIDs(rows))
	}
	if rankOf(rows, "لَيِسَ") != 0 || rankOf(rows, "لَيْسَ") == 0 {
		t.Errorf("override should fold لَيِسَ into لَيْسَ, rows: %v", rowIDs(rows))
	}
	if _, err := Rank(ix, corpora(), nil, nil, []Override{{Kind: OverrideLemma, From: "كَلْب", To: "-"}}, Options{}); err == nil {
		t.Error("an override naming an unknown lemma should fail")
	}
	if _, err := Rank(ix, corpora(), nil, []Essential{{Arabic: "مَرْحَبًا", MaxRank: 2}}, nil, Options{}); err == nil {
		t.Error("an essential Wiktionary lacks needs a part of speech and a gloss")
	}
}

func TestAnEssentialThatWiktionaryCallsAProperNounShipsAsACommonNoun(t *testing.T) {
	lemmas := lemmasOf(t, dump(t,
		entry{Word: "في", Pos: "prep", Forms: []form{{Form: "فِي", Tags: []string{"canonical"}}}, Senses: []sense{{Glosses: []string{"in"}}}},
		entry{Word: "الأحد", Pos: "name", Forms: []form{{Form: "الْأَحَد", Tags: []string{"canonical", "masculine"}}}, Senses: []sense{{Glosses: []string{"Sunday"}}}},
	))
	ix := NewIndex(lemmas)
	corpus := &Corpus{Name: "msa", Total: 100, Types: []Count{{"في", 50}}}
	res, err := Rank(ix, []*Corpus{corpus}, nil, []Essential{{Arabic: "الْأَحَد", MaxRank: 1}}, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rankOf(res.Rows, "الْأَحَد") != 1 {
		t.Fatalf("rows: %v", rowIDs(res.Rows))
	}
	if got := res.Rows[0].Lemma.Pos(); !slices.Equal(got, []string{"noun"}) {
		t.Errorf("pos = %v", got)
	}
	if !ix.Lemmas[1].Name() {
		t.Error("the index itself must not change")
	}
	shipped, err := Ship(res.Rows, lemmas, 6)
	if err != nil || len(shipped) != 2 || shipped[0].ID != "الْأَحَد" {
		t.Fatalf("shipped %v err %v", shipped, err)
	}
}

func TestReadCountsSkeletonizesAndMerges(t *testing.T) {
	c, err := ReadCounts("subs", strings.NewReader("حسناً 10\nحسنا 5\nhello 3\n، 100\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Types) != 1 || c.Types[0].Word != "حسنا" || c.Types[0].Count != 15 {
		t.Errorf("types = %+v", c.Types)
	}
	if c.Total != 118 {
		t.Errorf("total should count every token, got %v", c.Total)
	}
}

func TestExactCitationMatchBeatsLemmatizedBase(t *testing.T) {
	ix := NewIndex(lemmasOf(t, dump(t,
		entry{Word: "أيضا", Pos: "adv", Forms: []form{{Form: "أَيْضًا", Tags: []string{"canonical"}}}, Senses: []sense{{Glosses: []string{"also"}}}},
		entry{Word: "أيض", Pos: "noun", Forms: []form{{Form: "أَيْض", Tags: []string{"canonical"}}, {Form: "أَيْضًا", Tags: []string{"indefinite", "accusative"}, Source: "declension"}}, Senses: []sense{{Glosses: []string{"returning"}}}},
	)))
	corpus := &Corpus{Name: "msa", Total: 100, Types: []Count{{"أيضا", 50}}}
	res, err := Rank(ix, []*Corpus{corpus}, map[string]CamelAnalysis{"أيضا": {Lex: "أَيْض", Pos: "noun"}}, nil, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := rowIDs(res.Rows); !slices.Equal(got, []string{"أَيْضًا"}) {
		t.Errorf("the adverb entry should take all the mass, got %v", got)
	}
	if res.Decisions[0].Outcome != "wiktionary-exact" {
		t.Errorf("outcome = %s", res.Decisions[0].Outcome)
	}
}

func TestFormOverrideForcesAttribution(t *testing.T) {
	ix := fixture(t)
	res, err := Rank(ix, corpora(), nil, nil, []Override{{Kind: OverrideForm, From: "ليس", To: "لَيْسَ"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rankOf(res.Rows, "لَيِسَ") != 0 {
		t.Errorf("the forced form should not credit لَيِسَ, rows: %v", rowIDs(res.Rows))
	}
}

func TestReadOverridesAndEssentials(t *testing.T) {
	got, err := ReadOverrides(strings.NewReader("# comment\nlemma\tلَيِسَ\tلَيْسَ\nform\tكل\tكُلّ\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Override{{OverrideLemma, "لَيِسَ", "لَيْسَ"}, {OverrideForm, "كل", "كُلّ"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v", got)
	}
	if _, err := ReadOverrides(strings.NewReader("form\tكل\t-\n")); err == nil {
		t.Error("a form override that drops should be rejected")
	}
	ess, err := ReadEssentials(strings.NewReader("مَرْحَبًا\t10\nشُكْرًا\t20\tintj\tthanks\n"))
	if err != nil || len(ess) != 2 || ess[1].Pos != "intj" || ess[1].Gloss != "thanks" {
		t.Errorf("essentials %+v err %v", ess, err)
	}
	if _, err := ReadEssentials(strings.NewReader("مَرْحَبًا\tx\n")); err == nil {
		t.Error("a bad rank should be rejected")
	}
}

func TestShipPutsRankedLemmasFirstAndKeepsOnlyMSASenses(t *testing.T) {
	lemmas := fixtureLemmas(t)
	ix := NewIndex(lemmas)
	res, err := Rank(ix, corpora(), nil, nil, nil, Options{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	shipped, err := Ship(res.Rows, lemmas, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range res.Rows {
		if shipped[i].ID != r.Lemma.ID {
			t.Errorf("position %d is %s, ranked %s", i, shipped[i].ID, r.Lemma.ID)
		}
	}
	for _, l := range shipped {
		if l.Name() || !l.MSA() {
			t.Errorf("%s should not be shipped", l.ID)
		}
		for _, e := range l.Entries {
			if len(e.Senses) > 1 || e.Pos == "name" {
				t.Errorf("%s: %+v", l.ID, e)
			}
		}
	}
	rest := shipped[len(res.Rows):]
	if !slices.IsSortedFunc(rest, func(a, b *lexicon.Lemma) int { return strings.Compare(a.ID, b.ID) }) {
		t.Errorf("unranked lemmas should follow in a fixed order: %v", rowIDs2(rest))
	}
}

func rowIDs2(lemmas []*lexicon.Lemma) []string {
	var out []string
	for _, l := range lemmas {
		out = append(out, l.ID)
	}
	return out
}

func TestWrittenFilesRoundTrip(t *testing.T) {
	lemmas := fixtureLemmas(t)
	res, err := Rank(NewIndex(lemmas), corpora(), nil, nil, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	shipped, err := Ship(res.Rows, lemmas, 6)
	if err != nil {
		t.Fatal(err)
	}

	var lex bytes.Buffer
	if err := WriteLexicon(&lex, shipped); err != nil {
		t.Fatal(err)
	}
	loaded, err := lexicon.Load(&lex)
	if err != nil || loaded.Len() != len(shipped) {
		t.Fatalf("lexicon: %v %v", loaded, err)
	}

	var ranking bytes.Buffer
	if err := WriteRanking(&ranking, shipped[:len(res.Rows)]); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSpace(ranking.String()), "\n"); len(lines) != len(res.Rows)+1 || lines[0] != "rank\tid\tpos\tgloss" {
		t.Errorf("ranking:\n%s", ranking.String())
	}

	var forms bytes.Buffer
	if err := WriteForms(&forms, shipped); err != nil {
		t.Fatal(err)
	}
	facts := make([]morph.Facts, len(shipped))
	for i, l := range shipped {
		facts[i] = morph.FactsOf(l)
	}
	table, err := morph.Decode(&forms, facts)
	if err != nil {
		t.Fatal(err)
	}
	want := NewIndex(shipped)
	for _, w := range []string{"الكتاب", "بالكتاب", "وكتبها", "فيه", "ليس"} {
		if got, exp := table.Analyze(w), want.Analyze(w); !slices.Equal(got, exp) {
			t.Errorf("%s: decoded table says %v, built index says %v", w, got, exp)
		}
	}
}

func TestOutputIsByteStable(t *testing.T) {
	render := func() []byte {
		lemmas := fixtureLemmas(t)
		res, err := Rank(NewIndex(lemmas), corpora(), nil, nil, nil, Options{})
		if err != nil {
			t.Fatal(err)
		}
		shipped, err := Ship(res.Rows, lemmas, 6)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := WriteLexicon(&b, shipped); err != nil {
			t.Fatal(err)
		}
		if err := WriteForms(&b, shipped); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	if !bytes.Equal(render(), render()) {
		t.Error("two runs over the same input differ")
	}
}
