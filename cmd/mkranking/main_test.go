package main

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/env"
	"github.com/scuba-plaza/mufradat/internal/jsonl"
	"github.com/scuba-plaza/mufradat/internal/lexicon"
	"github.com/scuba-plaza/mufradat/internal/proc"
	"github.com/scuba-plaza/mufradat/internal/ranking"
)

const dumpFixture = `{"word":"في","pos":"prep","forms":[{"form":"فِي","tags":["canonical"]}],"senses":[{"glosses":["in"]}]}
{"word":"كتاب","pos":"noun","forms":[{"form":"كِتَاب","tags":["canonical","masculine"]},{"form":"الْكِتَاب","tags":["definite"],"source":"declension"}],"senses":[{"glosses":["book"]}]}
{"word":"قلم","pos":"noun","forms":[{"form":"قَلَم","tags":["canonical"]}],"senses":[{"glosses":["pen"]}]}
{"word":"محمد","pos":"name","forms":[{"form":"مُحَمَّد","tags":["canonical"]}],"senses":[{"glosses":["Muhammad"]}]}
{"word":"نادر","pos":"adj","forms":[{"form":"نَادِر","tags":["canonical"]}],"senses":[{"glosses":["rare"]}]}
`

const subtitles = "في 500\nالكتاب 50\nمحمد 40\nقلم 10\n"

const msaList = "في\t900\nالكتاب\t150\nمحمد\t200\nقلم\t30\n"

const camelLemmas = "في\tفِي\tprep\tin\nالكتاب\tكِتاب\tnoun\tbook\n"

type fixtureRunner struct {
	calls [][]string
}

func (f *fixtureRunner) Run(ctx context.Context, c proc.Cmd) error {
	f.calls = append(f.calls, append([]string{c.Name}, c.Args...))
	return os.WriteFile(c.Args[1], []byte(camelLemmas), 0o644)
}

func (f *fixtureRunner) Start(context.Context, proc.Cmd) (proc.Process, error) {
	return nil, io.ErrClosedPipe
}

func (f *fixtureRunner) LookPath(name string) (string, error) { return name, nil }

func archive(t *testing.T, name, content string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	w, err := zw.Create("lists/" + name)
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(content))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func server(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/wiktionary.jsonl", func(w http.ResponseWriter, r *http.Request) { hits++; io.WriteString(w, dumpFixture) })
	mux.HandleFunc("/subtitles.txt", func(w http.ResponseWriter, r *http.Request) { hits++; io.WriteString(w, subtitles) })
	mux.HandleFunc("/msa.zip", func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write(archive(t, "MSA_freq_lists.tsv", msaList+"extra\t1\nextra2\t1\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &hits
}

func testSources(base string) []source {
	return []source{
		{Name: "wiktionary", URL: base + "/wiktionary.jsonl", File: "wiktionary.jsonl"},
		{Name: "subtitles", URL: base + "/subtitles.txt", File: "subtitles.txt"},
		{Name: "msa", URL: base + "/msa.zip", File: "msa.tsv", ZipEntry: "MSA_freq_lists.tsv", MaxLines: 4},
	}
}

func workDir(t *testing.T) (string, options) {
	t.Helper()
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	if err := os.MkdirAll(filepath.Join(data, curationDir), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"essentials.tsv": "نَادِر\t1\n", "overrides.tsv": ""} {
		if err := os.WriteFile(filepath.Join(data, curationDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, options{cache: filepath.Join(dir, "cache"), data: data, python: "fake-python", limit: 10, senses: 6}
}

func testEnv(runner proc.Runner, stderr io.Writer) env.Env {
	return env.Env{
		Stderr: stderr, Getenv: func(string) string { return "" }, Home: "/nonexistent",
		HTTP: http.DefaultClient, Runner: runner,
	}
}

func TestBuildDownloadsAnalysesRanksAndRecordsTheSources(t *testing.T) {
	srv, hits := server(t)
	_, opts := workDir(t)
	opts.sources = testSources(srv.URL)
	runner := &fixtureRunner{}
	var stderr bytes.Buffer
	if err := build(context.Background(), testEnv(runner, &stderr), opts); err != nil {
		t.Fatalf("%v\n%s", err, stderr.String())
	}
	if *hits != 3 || len(runner.calls) != 1 || runner.calls[0][0] != "fake-python" {
		t.Fatalf("hits %d calls %v", *hits, runner.calls)
	}
	if got := runner.calls[0][3:]; len(got) != 2 || !strings.HasSuffix(got[0], "subtitles.txt:50000") || !strings.HasSuffix(got[1], "msa.tsv:100000") {
		t.Errorf("the analysis should read both frequency lists: %v", got)
	}
	msa, err := os.ReadFile(filepath.Join(opts.cache, "msa.tsv"))
	if err != nil || strings.Count(string(msa), "\n") != 4 {
		t.Errorf("the archive entry is cut to MaxLines: %q %v", msa, err)
	}

	f, err := os.Open(filepath.Join(opts.data, "ranking.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	list, err := ranking.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := list.ID(1); id != "نَادِر" {
		t.Errorf("the essential is pinned to rank 1, got %q", id)
	}
	if list.Rank("فِي") == 0 || list.Rank("كِتَاب") == 0 || list.Rank("مُحَمَّد") != 0 {
		t.Errorf("ranking %v", list.IDs())
	}
	lf, err := os.Open(filepath.Join(opts.data, "lexicon.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Close()
	lex, err := lexicon.Load(lf)
	if err != nil || lex.Len() != list.Len() {
		t.Fatalf("lexicon %v err %v", lex, err)
	}
	records, err := jsonl.ReadFile[locked](filepath.Join(opts.data, lockFile))
	if err != nil || len(records) != 4 {
		t.Fatalf("records %+v err %v", records, err)
	}
	for _, r := range records {
		if len(r.SHA256) != 64 || r.Bytes == 0 {
			t.Errorf("record %+v", r)
		}
	}

	if err := build(context.Background(), testEnv(runner, &stderr), opts); err != nil {
		t.Fatal(err)
	}
	if *hits != 3 || len(runner.calls) != 1 {
		t.Errorf("a second run reuses the cache: hits %d calls %d", *hits, len(runner.calls))
	}
}

func TestBuildNotesASourceThatChanged(t *testing.T) {
	srv, _ := server(t)
	_, opts := workDir(t)
	opts.sources = testSources(srv.URL)
	runner := &fixtureRunner{}
	var stderr bytes.Buffer
	e := testEnv(runner, &stderr)
	if err := build(context.Background(), e, opts); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opts.cache, "subtitles.txt"), []byte(subtitles+"قلم 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if err := build(context.Background(), e, opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "note: subtitles differs from the checksum recorded in sources.jsonl") {
		t.Errorf("stderr:\n%s", stderr.String())
	}
}

func TestBuildFailsClearlyWhenADownloadFails(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	_, opts := workDir(t)
	opts.sources = testSources(srv.URL)
	err := build(context.Background(), testEnv(&fixtureRunner{}, io.Discard), opts)
	if err == nil || !strings.Contains(err.Error(), "downloading wiktionary: 404") {
		t.Fatalf("err = %v", err)
	}
	if entries, _ := os.ReadDir(opts.cache); len(entries) != 0 {
		t.Errorf("a failed download leaves nothing behind: %v", entries)
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	var stderr bytes.Buffer
	e := testEnv(&fixtureRunner{}, &stderr)
	for _, args := range [][]string{{"-limit", "0"}, {"extra"}, {"-nope"}} {
		if code := run(context.Background(), e, args); code != 2 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if code := run(context.Background(), e, []string{"-h"}); code != 0 {
		t.Errorf("-h: exit %d", code)
	}
}
