package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestEventsAreSilencedByQuietButWarningsAreNot(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out, Options{})
	r.Event("wrote %d notes", 3)
	r.Warn("%s failed", "kitab")
	if want := "wrote 3 notes\nwarning: kitab failed\n"; out.String() != want {
		t.Errorf("output %q, want %q", out.String(), want)
	}

	out.Reset()
	quiet := NewReporter(&out, Options{Quiet: true})
	quiet.Event("hidden")
	quiet.Warn("still shown")
	if want := "warning: still shown\n"; out.String() != want {
		t.Errorf("quiet output %q, want %q", out.String(), want)
	}
	if !quiet.Quiet() || r.Quiet() {
		t.Error("Quiet reports the option")
	}
}

func TestColourPaintsOnlyWhenAsked(t *testing.T) {
	var out bytes.Buffer
	coloured := NewReporter(&out, Options{Color: true})
	coloured.Warn("x")
	if want := "\x1b[33mwarning:\x1b[0m x\n"; out.String() != want {
		t.Errorf("coloured warning %q, want %q", out.String(), want)
	}
	if got := coloured.Paint(Red, "bad"); got != "\x1b[31mbad\x1b[0m" || !coloured.Color() {
		t.Errorf("Paint = %q", got)
	}
	plain := NewReporter(&out, Options{})
	if got := plain.Paint(Red, "bad"); got != "bad" || plain.Color() {
		t.Errorf("an uncoloured reporter must not paint: %q", got)
	}
	for style, code := range map[Style]string{Bold: "1", Dim: "2", Red: "31", Green: "32", Yellow: "33", Cyan: "36"} {
		if got := Paint(true, style, "t"); got != "\x1b["+code+"mt\x1b[0m" {
			t.Errorf("style %s paints %q", code, got)
		}
	}
}

func TestProgressRewritesOneLineOnATerminalAndIsClearedByTheNextMessage(t *testing.T) {
	var out bytes.Buffer
	r := NewReporter(&out, Options{TTY: true})
	r.Progress("[%d/%d] writing", 1, 10)
	r.Progress("[%d/%d] done", 2, 10)
	if want := "\r[1/10] writing" + "\r[2/10] done" + "   "; out.String() != want {
		t.Errorf("progress %q, want %q: a shorter line is padded over the longer one", out.String(), want)
	}
	out.Reset()
	r.Event("finished")
	if want := "\r" + strings.Repeat(" ", len("[2/10] done")) + "\rfinished\n"; out.String() != want {
		t.Errorf("event after progress %q, want %q", out.String(), want)
	}
	out.Reset()
	r.Progress("working")
	out.Reset()
	r.Done()
	if want := "\r" + strings.Repeat(" ", len("working")) + "\r"; out.String() != want {
		t.Errorf("Done %q, want %q", out.String(), want)
	}
	out.Reset()
	r.Done()
	if out.Len() != 0 {
		t.Errorf("Done with nothing showing wrote %q", out.String())
	}
}

func TestProgressIsHiddenWhenNotOnATerminalOrQuiet(t *testing.T) {
	for name, opts := range map[string]Options{"piped": {}, "quiet terminal": {TTY: true, Quiet: true}, "quiet piped": {Quiet: true}} {
		var out bytes.Buffer
		r := NewReporter(&out, opts)
		r.Progress("[1/2] working")
		r.Done()
		if out.Len() != 0 {
			t.Errorf("%s: wrote %q", name, out.String())
		}
	}
}

func TestWidthIgnoresVowelMarksAndTableAlignsByIt(t *testing.T) {
	for in, want := range map[string]int{"": 0, "abc": 3, "كِتَاب": 4, "كتاب": 4, "مَدْرَسَة": 5} {
		if got := Width(in); got != want {
			t.Errorf("Width(%q) = %d, want %d", in, got, want)
		}
	}
	var out bytes.Buffer
	Table(&out, [][]string{
		{"id", "word", "gloss"},
		{"n_1", "كِتَاب", "book"},
		{"n_22", "قَلَم", ""},
		{"n_3"},
	})
	want := "id    word  gloss\n" +
		"n_1   كِتَاب  book\n" +
		"n_22  قَلَم\n" +
		"n_3\n"
	if got := out.String(); got != want {
		t.Errorf("table:\n%q\nwant:\n%q", got, want)
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if line != strings.TrimRight(line, " ") {
			t.Errorf("line %q keeps trailing spaces", line)
		}
	}
	out.Reset()
	Table(&out, nil)
	if out.Len() != 0 {
		t.Errorf("an empty table wrote %q", out.String())
	}
}

func TestTheLogLevelFollowsVerbosityAndQuiet(t *testing.T) {
	for _, c := range []struct {
		name                 string
		opts                 Options
		debug, info, warn, e bool
	}{
		{"default", Options{}, false, false, true, true},
		{"verbose", Options{Verbose: 1}, false, true, true, true},
		{"very verbose", Options{Verbose: 2}, true, true, true, true},
		{"extremely verbose", Options{Verbose: 3}, true, true, true, true},
		{"quiet", Options{Quiet: true}, false, false, false, true},
		{"quiet wins over verbose", Options{Quiet: true, Verbose: 2}, false, false, false, true},
	} {
		var out bytes.Buffer
		log := NewLogger(&out, c.opts)
		log.Debug("dbg")
		log.Info("inf")
		log.Warn("wrn")
		log.Error("err")
		got := out.String()
		for level, want := range map[string]bool{"dbg": c.debug, "inf": c.info, "wrn": c.warn, "err": c.e} {
			if strings.Contains(got, "msg="+level) != want {
				t.Errorf("%s: %s shown = %v, want %v\n%s", c.name, level, !want, want, got)
			}
		}
	}
}
