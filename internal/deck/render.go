package deck

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/note"
)

var arabicRun = regexp.MustCompile(`[\x{0600}-\x{06FF}\x{0750}-\x{077F}]+(?:[\s،]+[\x{0600}-\x{06FF}\x{0750}-\x{077F}]+)*`)

func arabicSpan(escaped string) string {
	return `<span class="ar" lang="ar" dir="rtl">` + escaped + `</span>`
}

func markArabic(s string) string {
	return arabicRun.ReplaceAllStringFunc(html.EscapeString(s), arabicSpan)
}

func formsHTML(n note.Note) string {
	parts := make([]string, 0, len(n.Forms))
	for _, f := range n.Forms {
		parts = append(parts, `<span class="form"><span class="lbl">`+html.EscapeString(f.Label)+`</span>`+arabicSpan(html.EscapeString(f.Arabic))+`</span>`)
	}
	return strings.Join(parts, " ")
}

func exampleHTML(n note.Note) string {
	parts, err := note.SplitExample(n.Example)
	if err != nil {
		return html.EscapeString(note.PlainExample(n.Example))
	}
	return html.EscapeString(parts.Before) + "<b>" + html.EscapeString(parts.Target) + "</b>" + html.EscapeString(parts.After)
}

func details(n note.Note) string {
	var parts []string
	switch n.Gender {
	case "m":
		parts = append(parts, "masc.")
	case "f":
		parts = append(parts, "fem.")
	case "m+f":
		parts = append(parts, "masc./fem.")
	}
	if n.VerbForm != "" {
		parts = append(parts, "form "+html.EscapeString(n.VerbForm))
	}
	if n.Root != "" {
		parts = append(parts, "root "+markArabic(n.Root))
	}
	return strings.Join(parts, " · ")
}

func rankBucket(rank int) string {
	if rank <= 0 {
		return "rank::unranked"
	}
	lo := (rank-1)/500*500 + 1
	return fmt.Sprintf("rank::%04d-%04d", lo, lo+499)
}

func checkHTML(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return "<b>Check:</b><br>" + strings.Join(lines, "<br>")
}
