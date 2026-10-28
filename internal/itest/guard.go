package itest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var testName = regexp.MustCompile(`\bTest\w+`)

func EndpointsAreTested(t *testing.T, pkg, doc string) {
	t.Helper()
	endpoints, unnamed := endpointsOf(t)
	for _, pos := range unnamed {
		t.Errorf("%s: a request is made without a Path constant, so this guard cannot see which endpoint it is", pos)
	}
	if len(endpoints) == 0 {
		t.Fatal("found no Path constants; the client must name its endpoints")
	}
	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("integration_test.go")
	if err != nil {
		t.Fatalf("a package that calls a real API needs an integration_test.go: %v", err)
	}
	if !strings.HasPrefix(string(source), "//go:build integration\n") {
		t.Error("integration_test.go must start with //go:build integration so it never runs by default")
	}
	rows := tableRows(string(raw))
	documented := map[string]bool{}
	for name, value := range endpoints {
		found := false
		for _, cells := range rows {
			if cells[0] != pkg || !strings.Contains(cells[1], "`"+value+"`") {
				continue
			}
			found = true
			names := testName.FindAllString(cells[2], -1)
			if len(names) == 0 {
				t.Errorf("%s (%s) is listed in %s without a test", name, value, doc)
			}
			for _, n := range names {
				documented[n] = true
				if !strings.Contains(string(source), "func "+n+"(t *testing.T)") {
					t.Errorf("%s (%s) lists %s, which integration_test.go does not define", name, value, n)
				}
			}
		}
		if !found {
			t.Errorf("endpoint %s (%s) has no row for package %s in %s: add an integration test and list it", name, value, pkg, doc)
		}
	}
	for _, n := range testName.FindAllString(string(source), -1) {
		if n != "Test" && strings.Contains(string(source), "func "+n+"(t *testing.T)") && !documented[n] && !strings.Contains(string(raw), n) {
			t.Errorf("%s is not described in %s", n, doc)
		}
	}
}

func tableRows(doc string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 3 {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

func endpointsOf(t *testing.T) (map[string]string, []string) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	endpoints := map[string]string{}
	var unnamed []string
	for _, p := range pkgs {
		for _, file := range p.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.ValueSpec:
					for i, name := range n.Names {
						if i >= len(n.Values) {
							continue
						}
						lit, ok := n.Values[i].(*ast.BasicLit)
						if strings.HasPrefix(name.Name, "Path") && ok && lit.Kind == token.STRING {
							value, _ := strconv.Unquote(lit.Value)
							endpoints[name.Name] = value
						}
					}
				case *ast.CallExpr:
					sel, ok := n.Fun.(*ast.SelectorExpr)
					if ok && sel.Sel.Name == "do" && len(n.Args) >= 3 && !mentionsPath(n.Args[2]) {
						unnamed = append(unnamed, fset.Position(n.Pos()).String())
					}
				}
				return true
			})
		}
	}
	return endpoints, unnamed
}

func mentionsPath(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && strings.HasPrefix(id.Name, "Path") {
			found = true
		}
		return !found
	})
	return found
}
