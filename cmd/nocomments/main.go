package main

import (
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var skipDirs = []string{".git", ".venv", ".env", ".idea", "node_modules", "dist", "out", "deck-data", "__pycache__", ".pytest_cache", "testdata"}

var extensions = []string{".go", ".py", ".sh", ".nix", ".yml", ".yaml", ".js", ".mjs", ".css", ".html"}

var hashComment = regexp.MustCompile(`^\s*#`)

var lineSlash = regexp.MustCompile(`^\s*//`)

var blockOpen = regexp.MustCompile(`^\s*/\*(\*?)`)

var docPrefix = regexp.MustCompile(`^\s*\*?\s?`)

var htmlComment = regexp.MustCompile(`<!--`)

type finding struct {
	file string
	line int
	text string
}

func (f finding) String() string {
	return fmt.Sprintf("%s:%d: %s", f.file, f.line, strings.TrimSpace(f.text))
}

func goComments(path string, src []byte) []finding {
	fset := token.NewFileSet()
	file := fset.AddFile(path, fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)
	var out []finding
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.COMMENT && !strings.HasPrefix(lit, "//go:") {
			out = append(out, finding{path, fset.Position(pos).Line, lit})
		}
	}
}

func lineComments(path string, src []byte, patterns ...*regexp.Regexp) []finding {
	var out []finding
	for i, line := range strings.Split(string(src), "\n") {
		if i == 0 && strings.HasPrefix(line, "#!") {
			continue
		}
		for _, p := range patterns {
			if p.MatchString(line) {
				out = append(out, finding{path, i + 1, line})
				break
			}
		}
	}
	return out
}

func check(path string, src []byte) []finding {
	switch filepath.Ext(path) {
	case ".go":
		return goComments(path, src)
	case ".py", ".sh", ".nix", ".yml", ".yaml":
		return lineComments(path, src, hashComment)
	case ".js", ".mjs":
		return scriptComments(path, src, true)
	case ".css":
		return scriptComments(path, src, false)
	case ".html":
		return lineComments(path, src, htmlComment)
	}
	return nil
}

func scan(root string) ([]finding, error) {
	var out []finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && slices.Contains(skipDirs, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !slices.Contains(extensions, filepath.Ext(path)) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out = append(out, check(path, src)...)
		return nil
	})
	return out, err
}

func run(args []string, stdout, stderr io.Writer) int {
	roots := args
	if len(roots) == 0 {
		roots = []string{"."}
	}
	var all []finding
	for _, root := range roots {
		found, err := scan(root)
		if err != nil {
			fmt.Fprintln(stderr, "nocomments:", err)
			return 1
		}
		all = append(all, found...)
	}
	for _, f := range all {
		fmt.Fprintln(stdout, f)
	}
	if len(all) > 0 {
		fmt.Fprintf(stderr, "nocomments: %d comments found; this project ships code without them\n", len(all))
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func scriptComments(path string, src []byte, annotations bool) []finding {
	var out []finding
	inDoc, depth := false, 0
	for i, line := range strings.Split(string(src), "\n") {
		flag := func() { out = append(out, finding{path, i + 1, line}) }
		if inDoc {
			body := strings.TrimSuffix(strings.TrimSpace(line), "*/")
			text := strings.TrimSpace(docPrefix.ReplaceAllString(body, ""))
			if text != "" && depth == 0 && !strings.HasPrefix(text, "@") {
				flag()
			}
			depth += strings.Count(text, "{") - strings.Count(text, "}")
			if strings.Contains(line, "*/") {
				inDoc, depth = false, 0
			}
			continue
		}
		if lineSlash.MatchString(line) {
			flag()
			continue
		}
		m := blockOpen.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[1] == "" || !annotations {
			flag()
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "/**"))
		closed := strings.HasSuffix(rest, "*/")
		text := strings.TrimSpace(strings.TrimSuffix(rest, "*/"))
		if text != "" && !strings.HasPrefix(text, "@") {
			flag()
		}
		if !closed {
			inDoc = true
			depth = strings.Count(text, "{") - strings.Count(text, "}")
		}
	}
	return out
}
