package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lines(fs []finding) []int {
	var out []int
	for _, f := range fs {
		out = append(out, f.line)
	}
	return out
}

func TestGoCommentsAllowOnlyDirectives(t *testing.T) {
	src := "package x\n\n//go:embed a\nvar a string\n\n// doc\nfunc f() {\n\tx := \"// not a comment\"\n\t_ = x // trailing\n\t/* block */\n}\n"
	got := lines(check("x.go", []byte(src)))
	if len(got) != 3 || got[0] != 6 || got[1] != 9 || got[2] != 10 {
		t.Errorf("flagged lines = %v", got)
	}
}

func TestScriptsAllowJSDocTagsButNotProse(t *testing.T) {
	src := strings.Join([]string{
		"/** @typedef {{ a: string }} A */",
		"/**",
		" * @param {string} x",
		" * @returns {void}",
		" */",
		"/**",
		" * @typedef {{",
		" *   a: string,",
		" *   b: number,",
		" * }} B",
		" */",
		"/** explains the thing */",
		"/**",
		" * Explains the thing.",
		" * @param {string} x",
		" */",
		"// line",
		"/* block */",
		"const url = 'https://example.com'",
	}, "\n")
	got := lines(check("x.js", []byte(src)))
	want := []int{12, 14, 17, 18}
	if len(got) != len(want) {
		t.Fatalf("flagged lines = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("flagged lines = %v, want %v", got, want)
		}
	}
}

func TestCSSAllowsNoBlockComments(t *testing.T) {
	if got := lines(check("x.css", []byte("a { }\n/** x */\n/* y */\n"))); len(got) != 2 {
		t.Errorf("flagged lines = %v", got)
	}
}

func TestHashCommentsExceptTheShebang(t *testing.T) {
	src := "#!/bin/sh\necho hi\n# note\n  # indented\nx=1\n"
	if got := lines(check("x.sh", []byte(src))); len(got) != 2 || got[0] != 3 || got[1] != 4 {
		t.Errorf("flagged lines = %v", got)
	}
	if got := check("page.html", []byte("<p>x</p><!-- hi -->")); len(got) != 1 {
		t.Errorf("html = %v", got)
	}
}

func TestRunSkipsVendoredDirectoriesAndReportsFindings(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ok.go", "package x\n")
	write(".venv/lib/x.py", "# fine here\n")
	write("testdata/x.go", "// fine here\n")
	var stdout, stderr bytes.Buffer
	if code := run([]string{dir}, &stdout, &stderr); code != 0 || stdout.Len() != 0 {
		t.Fatalf("clean tree: %d %q %q", code, stdout.String(), stderr.String())
	}
	write("bad.py", "x = 1\n# why\n")
	if code := run([]string{dir}, &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), "bad.py:2:") || !strings.Contains(stderr.String(), "1 comments found") {
		t.Fatalf("dirty tree: %d %q %q", code, stdout.String(), stderr.String())
	}
}
