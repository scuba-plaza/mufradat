package doctor

import (
	"bytes"
	"context"
	"testing"
)

func check(name string, r Result) Check {
	return Check{Name: name, Run: func(context.Context) Result { return r }}
}

func TestRunKeepsTheOrderAndStampsEachResultWithItsCheckName(t *testing.T) {
	results := Run(context.Background(), []Check{
		check("first", Result{Status: OK, Detail: "fine", Name: "ignored"}),
		check("second", Result{Status: Warn}),
		check("third", Result{Status: Fail, Hint: "fix it"}),
	})
	if len(results) != 3 || results[0].Name != "first" || results[1].Name != "second" || results[2].Name != "third" {
		t.Fatalf("results %+v", results)
	}
	if results[0].Detail != "fine" || results[2].Hint != "fix it" {
		t.Errorf("a check's own detail and hint are kept: %+v", results)
	}
	if got := Run(context.Background(), nil); got == nil || len(got) != 0 {
		t.Errorf("no checks: %#v", got)
	}
}

func TestOnlyAFailedCheckFailsTheRun(t *testing.T) {
	for name, c := range map[string]struct {
		statuses []Status
		want     bool
	}{
		"nothing":        {nil, false},
		"all fine":       {[]Status{OK, OK}, false},
		"warnings":       {[]Status{OK, Warn}, false},
		"skipped":        {[]Status{Skip, OK}, false},
		"one failure":    {[]Status{OK, Fail, Warn}, true},
		"failure first":  {[]Status{Fail, OK}, true},
		"only a failure": {[]Status{Fail}, true},
	} {
		var results []Result
		for _, s := range c.statuses {
			results = append(results, Result{Status: s})
		}
		if got := Failed(results); got != c.want {
			t.Errorf("%s: Failed = %v, want %v", name, got, c.want)
		}
	}
}

func TestRenderShowsAMarkNameDetailAndHint(t *testing.T) {
	results := []Result{
		{Name: "workspace", Status: OK, Detail: "/home/x/ws"},
		{Name: "mistral key", Status: Warn, Detail: "could not check", Hint: "check the network"},
		{Name: "python", Status: Fail, Detail: "missing", Hint: "run setup"},
		{Name: "google credentials", Status: Skip, Detail: "not needed"},
	}
	var plain bytes.Buffer
	Render(&plain, results, false)
	want := "ok    workspace              /home/x/ws\n" +
		"warn  mistral key            could not check\n" +
		"      hint: check the network\n" +
		"FAIL  python                 missing\n" +
		"      hint: run setup\n" +
		"skip  google credentials     not needed\n"
	if plain.String() != want {
		t.Errorf("plain output:\n%q\nwant:\n%q", plain.String(), want)
	}
	var coloured bytes.Buffer
	Render(&coloured, results, true)
	for _, want := range []string{"\x1b[32mok  \x1b[0m", "\x1b[33mwarn\x1b[0m", "\x1b[31mFAIL\x1b[0m", "\x1b[2mskip\x1b[0m", "\x1b[2mhint: run setup\x1b[0m"} {
		if !bytes.Contains(coloured.Bytes(), []byte(want)) {
			t.Errorf("coloured output lacks %q:\n%q", want, coloured.String())
		}
	}
}
