package review

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/note"
)

var (
	commentBlock = regexp.MustCompile(`(?s)/\*\*(.*?)\*/`)
	linePrefix   = regexp.MustCompile(`(?m)^\s*\*\s?`)
	propertyHead = regexp.MustCompile(`^(\w+)\??\s*:`)
	typedefName  = regexp.MustCompile(`^\s*(\w+)`)
)

func typedefProperties(source, name string) ([]string, bool) {
	for _, block := range commentBlock.FindAllStringSubmatch(source, -1) {
		text := strings.TrimSpace(linePrefix.ReplaceAllString(block[1], ""))
		rest, ok := strings.CutPrefix(text, "@typedef")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, "{") {
			continue
		}
		end := closing(rest, 0)
		if end < 0 {
			continue
		}
		if m := typedefName.FindStringSubmatch(rest[end+1:]); m == nil || m[1] != name {
			continue
		}
		inner := strings.TrimSpace(rest[1:end])
		if !strings.HasPrefix(inner, "{") {
			return nil, false
		}
		objEnd := closing(inner, 0)
		return topLevelProperties(inner[1:objEnd]), true
	}
	return nil, false
}

func closing(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func topLevelProperties(body string) []string {
	var props []string
	depth, start := 0, 0
	flush := func(end int) {
		if m := propertyHead.FindStringSubmatch(strings.TrimSpace(body[start:end])); m != nil {
			props = append(props, m[1])
		}
	}
	for i, r := range body {
		switch r {
		case '{', '[', '(', '<':
			depth++
		case '>':
			if i == 0 || body[i-1] != '=' {
				depth--
			}
		case '}', ']', ')':
			depth--
		case ',':
			if depth == 0 {
				flush(i)
				start = i + 1
			}
		}
	}
	flush(len(body))
	slices.Sort(props)
	return props
}

func jsonFields(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		tag := t.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" || name == "" {
			continue
		}
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func TestTypedefParserReadsTopLevelPropertiesOnly(t *testing.T) {
	src := `
/** @typedef {{ a: string, b?: number }} Flat */
/**
 * @typedef {{
 *   id: string, nested?: { inner: string, other: number }, list: Array<{ deep: string }>,
 *   fn: (x: number) => void, last?: string
 * }} Multi
 */
/** @typedef {Record<string, string>} NotAnObject */
`
	if got, ok := typedefProperties(src, "Flat"); !ok || !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("Flat: %v %v", got, ok)
	}
	if got, ok := typedefProperties(src, "Multi"); !ok || !slices.Equal(got, []string{"fn", "id", "last", "list", "nested"}) {
		t.Errorf("Multi: %v %v", got, ok)
	}
	if _, ok := typedefProperties(src, "NotAnObject"); ok {
		t.Error("a non-object typedef has no properties to compare")
	}
	if _, ok := typedefProperties(src, "Missing"); ok {
		t.Error("a typedef that does not exist must not be found")
	}
}

func TestEveryPropertyThePageReadsIsOneTheServerSends(t *testing.T) {
	raw, err := os.ReadFile("web/types.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	cases := []struct {
		server reflect.Type
		page   string
		subset bool
	}{
		{reflect.TypeOf(stateView{}), "View", false},
		{reflect.TypeOf(listView{}), "ListItem", false},
		{reflect.TypeOf(entryView{}), "Entry", false},
		{reflect.TypeOf(clipView{}), "Clip", false},
		{reflect.TypeOf(Summary{}), "Summary", false},
		{reflect.TypeOf(VoiceOption{}), "Voice", false},
		{reflect.TypeOf(Sample{}), "Sample", false},
		{reflect.TypeOf(voicesView{}), "VoiceList", false},
		{reflect.TypeOf(compareView{}), "Comparison", false},
		{reflect.TypeOf(result{}), "Result", false},
		{reflect.TypeOf(span{}), "Span", false},
		{reflect.TypeOf(row{}), "Row", false},
		{reflect.TypeOf(flagView{}), "Flag", false},
		{reflect.TypeOf(formView{}), "FormView", false},
		{reflect.TypeOf(cardView{}), "Card", false},
		{reflect.TypeOf(changeView{}), "Change", false},
		{reflect.TypeOf(endingFix{}), "Fix", false},
		{reflect.TypeOf(note.Form{}), "Form", false},
		{reflect.TypeOf(note.Note{}), "Note", true},
	}
	for _, tc := range cases {
		page, ok := typedefProperties(source, tc.page)
		if !ok {
			t.Errorf("web/types.js has no object typedef %s", tc.page)
			continue
		}
		server := jsonFields(tc.server)
		for _, p := range page {
			if !slices.Contains(server, p) {
				t.Errorf("%s declares %q but %s never sends it (it sends %v)", tc.page, p, tc.server.Name(), server)
			}
		}
		if tc.subset {
			continue
		}
		for _, s := range server {
			if !slices.Contains(page, s) {
				t.Errorf("%s sends %q but the page type %s does not declare it", tc.server.Name(), s, tc.page)
			}
		}
	}
}

func TestThePageAsksForTheRewriteButtonWithTheFlagTheServerSends(t *testing.T) {
	js, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	var flag string
	typ := reflect.TypeOf(stateView{})
	for i := range typ.NumField() {
		if typ.Field(i).Name == "Mistral" {
			flag, _, _ = strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		}
	}
	if flag == "" {
		t.Fatal("stateView has no Mistral flag")
	}
	if !strings.Contains(string(js), "} else if (v."+flag+") {") {
		t.Errorf("web/app.js never reads v.%s, so the Ask button would not appear", flag)
	}
}

var (
	jsTones   = regexp.MustCompile(`(?s)export const TONES = \{(.*?)\};`)
	jsToneKey = regexp.MustCompile(`(\w+):\s*'\w+'`)
	cssTone   = regexp.MustCompile(`\.tone-(\w+)\s*\{`)
)

func TestEveryToneTheServerSendsIsOneThePageKnowsAndStyles(t *testing.T) {
	labels, err := os.ReadFile("web/labels.js")
	if err != nil {
		t.Fatal(err)
	}
	block := jsTones.FindSubmatch(labels)
	if block == nil {
		t.Fatal("web/labels.js has no TONES table")
	}
	known := map[string]bool{}
	for _, m := range jsToneKey.FindAllSubmatch(block[1], -1) {
		known[string(m[1])] = true
	}
	css, err := os.ReadFile("web/app.css")
	if err != nil {
		t.Fatal(err)
	}
	styled := map[string]bool{"info": true, "good": true}
	for _, m := range cssTone.FindAllSubmatch(css, -1) {
		styled[string(m[1])] = true
	}

	sent := map[string]bool{}
	for _, file := range []string{"session.go", "server.go", "compare.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range regexp.MustCompile(`(?:Tone:\s*|[nN]oticeTone\s*=\s*(?:[^,\n]*,\s*)?)"([a-z]+)"`).FindAllSubmatch(source, -1) {
			sent[string(m[1])] = true
		}
	}
	if len(sent) < 5 {
		t.Fatalf("found only the tones %v in the Go sources; the pattern no longer matches how they are written", sent)
	}
	for tone := range sent {
		if !known[tone] {
			t.Errorf("the server sends the tone %q but web/labels.js TONES drops it, so the page shows it as info", tone)
		}
		if !styled[tone] {
			t.Errorf("the server sends the tone %q but web/app.css has no .tone-%s rule", tone, tone)
		}
	}
}
