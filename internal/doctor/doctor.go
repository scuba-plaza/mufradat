package doctor

import (
	"context"
	"fmt"
	"io"

	"github.com/scuba-plaza/mufradat/internal/ui"
)

type Status string

const (
	OK   Status = "ok"
	Warn Status = "warn"
	Fail Status = "fail"
	Skip Status = "skip"
)

type Result struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

type Check struct {
	Name string
	Run  func(ctx context.Context) Result
}

func Run(ctx context.Context, checks []Check) []Result {
	results := make([]Result, 0, len(checks))
	for _, c := range checks {
		r := c.Run(ctx)
		r.Name = c.Name
		results = append(results, r)
	}
	return results
}

func Failed(results []Result) bool {
	for _, r := range results {
		if r.Status == Fail {
			return true
		}
	}
	return false
}

func Render(w io.Writer, results []Result, color bool) {
	for _, r := range results {
		mark, style := "ok  ", ui.Green
		switch r.Status {
		case Warn:
			mark, style = "warn", ui.Yellow
		case Fail:
			mark, style = "FAIL", ui.Red
		case Skip:
			mark, style = "skip", ui.Dim
		}
		fmt.Fprintf(w, "%s  %-22s %s\n", ui.Paint(color, style, mark), r.Name, r.Detail)
		if r.Hint != "" {
			fmt.Fprintf(w, "      %s\n", ui.Paint(color, ui.Dim, "hint: "+r.Hint))
		}
	}
}
