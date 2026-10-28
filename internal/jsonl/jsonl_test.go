package jsonl

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type row struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func TestMarshalWritesOneLineAndLeavesMarkupAndArabicAlone(t *testing.T) {
	raw, err := Marshal(row{ID: "n_1", Text: "<b>a & b</b> كِتَاب\nsecond"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"id":"n_1","text":"<b>a & b</b> كِتَاب\nsecond"}`; string(raw) != want {
		t.Errorf("Marshal = %s, want %s", raw, want)
	}
	if _, err := Marshal(make(chan int)); err == nil {
		t.Error("an unencodable value must be an error")
	}
}

func TestEncodeAndDecodeRoundTripOneRecordPerLine(t *testing.T) {
	rows := []row{{"a", "x"}, {"b", "y"}, {"c", "z"}}
	var buf bytes.Buffer
	if err := Encode(&buf, rows); err != nil {
		t.Fatal(err)
	}
	if want := "{\"id\":\"a\",\"text\":\"x\"}\n{\"id\":\"b\",\"text\":\"y\"}\n{\"id\":\"c\",\"text\":\"z\"}\n"; buf.String() != want {
		t.Errorf("Encode = %q", buf.String())
	}
	got, err := Decode[row](&buf)
	if err != nil || len(got) != 3 || got[2] != rows[2] {
		t.Errorf("Decode = %+v, %v", got, err)
	}
	if got, err := Decode[row](strings.NewReader("")); err != nil || got != nil {
		t.Errorf("empty input: %+v, %v", got, err)
	}
}

func TestDecodeSkipsBlankLinesAndNamesTheBadOne(t *testing.T) {
	good := "\n  {\"id\":\"a\",\"text\":\"x\"}  \n\n\t\n{\"id\":\"b\",\"text\":\"y\"}"
	got, err := Decode[row](strings.NewReader(good))
	if err != nil || len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("Decode = %+v, %v", got, err)
	}
	for name, c := range map[string]struct{ in, want string }{
		"malformed":     {"{\"id\":\"a\"}\n{oops\n", "line 2: "},
		"unknown field": {"\n\n{\"id\":\"a\",\"extra\":1}\n", "line 3: json: unknown field \"extra\""},
		"wrong type":    {"{\"id\":5}\n", "line 1: "},
	} {
		got, err := Decode[row](strings.NewReader(c.in))
		if err == nil || !strings.HasPrefix(err.Error(), c.want) || got != nil {
			t.Errorf("%s: %+v, %v; want an error starting %q", name, got, err, c.want)
		}
	}
}

func TestDecodeHandlesAVeryLongLine(t *testing.T) {
	long := strings.Repeat("ك", 3<<20)
	var buf bytes.Buffer
	if err := Encode(&buf, []row{{"a", long}, {"b", "short"}}); err != nil {
		t.Fatal(err)
	}
	got, err := Decode[row](&buf)
	if err != nil || len(got) != 2 || got[0].Text != long || got[1].Text != "short" {
		t.Errorf("a %d byte line: %d rows, %v", len(long)*2, len(got), err)
	}
}

func TestFilesRoundTripAppendAndReportTheirPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "rows.jsonl")
	if got, err := ReadFile[row](path); got != nil || err != nil {
		t.Fatalf("a missing file: %+v, %v", got, err)
	}
	if err := WriteFile(path, []row{{"a", "x"}}); err != nil {
		t.Fatal(err)
	}
	if err := Append(path, row{"b", "y"}); err != nil {
		t.Fatal(err)
	}
	if err := Append(filepath.Join(dir, "new", "dir", "log.jsonl"), row{"c", "z"}); err != nil {
		t.Fatalf("Append must create the folder: %v", err)
	}
	got, err := ReadFile[row](path)
	if err != nil || len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("ReadFile = %+v, %v", got, err)
	}
	if raw, _ := os.ReadFile(path); !strings.HasSuffix(string(raw), "}\n") {
		t.Errorf("every line, the last included, ends with a newline: %q", raw)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("mode %v, %v", info.Mode(), err)
	}

	if err := os.WriteFile(path, []byte("{\"id\":\"a\"}\nnot json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile[row](path); err == nil || !strings.HasPrefix(err.Error(), path+": line 2: ") {
		t.Errorf("a damaged file: %v", err)
	}
	if err := Append(dir, row{"d", "w"}); err == nil {
		t.Error("appending to a folder reported success")
	}
	if _, err := ReadFile[row](dir); err == nil {
		t.Error("reading a folder reported success")
	}
	if err := Append(path, make(chan int)); err == nil {
		t.Error("appending an unencodable value reported success")
	}
}

func TestStringJoinsRecordsLineByLine(t *testing.T) {
	if got := String(row{"a", "x"}, map[string]int{"n": 1}); got != "{\"id\":\"a\",\"text\":\"x\"}\n{\"n\":1}\n" {
		t.Errorf("String = %q", got)
	}
	if String() != "" {
		t.Error("no records, no text")
	}
}
