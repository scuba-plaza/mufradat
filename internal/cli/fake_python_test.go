package cli_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/pyenv"
)

var fakeVersions = map[string]any{"python": "3.13.0", "camel-tools": "1.6.0", "torch": "2.14.0+cpu"}

func fakePython() int {
	args := os.Args[1:]
	switch {
	case len(args) >= 2 && args[0] == "-c":
		fmt.Println("3.13.0")
		return 0
	case len(args) >= 3 && args[0] == "-m" && args[1] == "venv":
		return makeVenv(args[2])
	case len(args) >= 3 && args[0] == "-u" && args[1] == "-m" && args[2] == "mufradat_sidecar":
		return fakeSidecar(args[3:])
	}
	fmt.Fprintln(os.Stderr, "fake python: unexpected arguments", args)
	return 2
}

func makeVenv(dir string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, name := range []string{"python", "camel_data"} {
		if err := os.Symlink(self, filepath.Join(bin, name)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}

func fakeUV() int {
	args := os.Args[1:]
	logLine(os.Getenv("FAKE_SIDECAR_LOG"), "uv "+strings.Join(args, " "))
	switch {
	case len(args) >= 2 && args[0] == "venv":
		return makeVenv(args[len(args)-1])
	case len(args) >= 2 && args[0] == "pip":
		return 0
	}
	fmt.Fprintln(os.Stderr, "fake uv: unexpected arguments", args)
	return 2
}

func fakeCamelData() int {
	args := os.Args[1:]
	if len(args) != 2 || args[0] != "-i" {
		fmt.Fprintln(os.Stderr, "fake camel_data: unexpected arguments", args)
		return 2
	}
	logLine(os.Getenv("FAKE_SIDECAR_LOG"), "camel_data "+args[1])
	for _, p := range pyenv.CamelPackages {
		if p.Name == args[1] {
			if err := os.MkdirAll(filepath.Join(os.Getenv("CAMELTOOLS_DATA"), "data", filepath.FromSlash(p.Dir)), 0o755); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		}
	}
	fmt.Fprintln(os.Stderr, "fake camel_data: unknown package", args[1])
	return 1
}

func logLine(path, line string) {
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

type sidecarItem struct {
	Key     string   `json:"key"`
	Field   string   `json:"field"`
	Tokens  []string `json:"tokens"`
	Context bool     `json:"context"`
}

type sidecarRequest struct {
	Type  string        `json:"type"`
	ID    int           `json:"id"`
	Items []sidecarItem `json:"items"`
}

var fakeDisagreements = map[string]string{"الْبَيْتِ": "الْبَيْتُ"}

func fakeSidecar(args []string) int {
	log := os.Getenv("FAKE_SIDECAR_LOG")
	mode := os.Getenv("FAKE_SIDECAR_MODE")
	if mode == "missing-lib" {
		fmt.Fprintln(os.Stderr, "ImportError: libstdc++.so.6: cannot open shared object file: No such file or directory")
		return 1
	}
	if len(args) > 0 && (args[0] == "selftest" || args[0] == "probe") {
		logLine(log, args[0])
		raw, _ := json.Marshal(map[string]any{"ok": true, "problems": []string{}, "versions": fakeVersions})
		fmt.Println(string(raw))
		return 0
	}
	logLine(log, "serve")
	out := bufio.NewWriter(os.Stdout)
	send := func(m map[string]any) {
		raw, _ := json.Marshal(m)
		out.Write(raw)
		out.WriteByte('\n')
		out.Flush()
	}
	send(map[string]any{"type": "hello", "protocol": 1, "version": "1", "versions": fakeVersions})
	in := bufio.NewReader(os.Stdin)
	for {
		line, err := in.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			var req sidecarRequest
			if jerr := json.Unmarshal(line, &req); jerr != nil {
				send(map[string]any{"type": "error", "message": jerr.Error()})
				continue
			}
			if req.Type == "shutdown" {
				return 0
			}
			logLine(log, fmt.Sprintf("check %d", len(req.Items)))
			for i, it := range req.Items {
				analyses := make([][]string, len(it.Tokens))
				for t, tok := range it.Tokens {
					analyses[t] = []string{tok}
				}
				reading := map[string]any{"type": "reading", "id": req.ID, "key": it.Key, "analyses": analyses, "catt": nil, "bert": nil}
				if it.Context {
					read := make([]string, len(it.Tokens))
					for t, tok := range it.Tokens {
						read[t] = tok
						if other, ok := fakeDisagreements[tok]; ok {
							read[t] = other
						}
					}
					reading["catt"], reading["bert"] = read, read
				}
				send(reading)
				if mode == "crash" && i == 1 {
					return 1
				}
				send(map[string]any{"type": "progress", "id": req.ID, "done": i + 1, "total": len(req.Items)})
			}
			send(map[string]any{"type": "done", "id": req.ID})
		}
		if err == io.EOF || err != nil {
			return 0
		}
	}
}
