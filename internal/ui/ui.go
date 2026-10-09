package ui

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

type Options struct {
	TTY     bool
	Color   bool
	Verbose int
	Quiet   bool
}

type Reporter struct {
	w        io.Writer
	opts     Options
	mu       sync.Mutex
	transLen int
}

func NewReporter(w io.Writer, opts Options) *Reporter {
	return &Reporter{w: w, opts: opts}
}

func (r *Reporter) Quiet() bool { return r.opts.Quiet }

func (r *Reporter) Color() bool { return r.opts.Color }

func (r *Reporter) Event(format string, args ...any) {
	if r.opts.Quiet {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearLocked()
	fmt.Fprintf(r.w, format+"\n", args...)
}

func (r *Reporter) Warn(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearLocked()
	fmt.Fprintf(r.w, r.Paint(Yellow, "warning:")+" "+format+"\n", args...)
}

func (r *Reporter) Progress(format string, args ...any) {
	if r.opts.Quiet || !r.opts.TTY {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	line := fmt.Sprintf(format, args...)
	pad := ""
	if n := r.transLen - utf8.RuneCountInString(line); n > 0 {
		pad = strings.Repeat(" ", n)
	}
	fmt.Fprintf(r.w, "\r%s%s", line, pad)
	r.transLen = utf8.RuneCountInString(line)
}

func (r *Reporter) Done() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearLocked()
}

func (r *Reporter) clearLocked() {
	if r.transLen == 0 {
		return
	}
	fmt.Fprintf(r.w, "\r%s\r", strings.Repeat(" ", r.transLen))
	r.transLen = 0
}

type Style string

const (
	Bold   Style = "1"
	Dim    Style = "2"
	Red    Style = "31"
	Green  Style = "32"
	Yellow Style = "33"
	Cyan   Style = "36"
)

func (r *Reporter) Paint(s Style, text string) string {
	return Paint(r.opts.Color, s, text)
}

func Paint(color bool, s Style, text string) string {
	if !color {
		return text
	}
	return "\x1b[" + string(s) + "m" + text + "\x1b[0m"
}

func Width(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		n++
	}
	return n
}

func Table(w io.Writer, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	widths := make([]int, 0)
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], Width(cell))
		}
	}
	for _, row := range rows {
		var b strings.Builder
		for i, cell := range row {
			b.WriteString(cell)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-Width(cell)+2))
			}
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}

func NewLogger(w io.Writer, opts Options) *slog.Logger {
	level := slog.LevelWarn
	switch {
	case opts.Quiet:
		level = slog.LevelError
	case opts.Verbose == 1:
		level = slog.LevelInfo
	case opts.Verbose >= 2:
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
