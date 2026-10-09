package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var draftRef = regexp.MustCompile(`(?m)^\{"ref":\d+.*$`)

type fakeCard struct {
	Ref       int               `json:"ref"`
	Arabic    string            `json:"arabic"`
	POS       string            `json:"pos"`
	English   string            `json:"english"`
	Hint      string            `json:"hint"`
	Forms     []json.RawMessage `json:"forms"`
	Example   string            `json:"example"`
	ExampleEN string            `json:"example_en"`
	Comment   string            `json:"comment"`
}

const (
	fakeExample     = "قَرَأَ [الْوَلَدُ] الْكِتَابَ فِي الْبَيْتِ."
	fakeRareExample = "قَرَأَ [الْوَلَدُ] الْكِتَابَ عَلَى الطَّاوِلَةِ."
)

func fakeClaude() int {
	prompt, _ := io.ReadAll(os.Stdin)
	call := bump(os.Getenv("FAKE_CLAUDE_COUNT"))
	if log := os.Getenv("FAKE_CLAUDE_LOG"); log != "" {
		appendLine(log, flagLine(os.Args[1:]))
	}
	emit := func(v map[string]any) int {
		raw, _ := json.Marshal(v)
		fmt.Println(string(raw))
		return 0
	}
	failure := func(status int, text string) int {
		emit(map[string]any{"type": "result", "subtype": "success", "is_error": true, "api_error_status": status, "result": text})
		return 1
	}

	switch mode := os.Getenv("FAKE_CLAUDE_MODE"); {
	case mode == "garbage":
		fmt.Println("this is not json")
		return 1
	case mode == "overloaded":
		return failure(529, "API Error: Overloaded")
	case strings.HasPrefix(mode, "limit-after-"):
		n, _ := strconv.Atoi(strings.TrimPrefix(mode, "limit-after-"))
		if call > n {
			return failure(429, "You've hit your limit · resets 3pm (UTC)")
		}
	}

	var cards []fakeCard
	for _, line := range draftRef.FindAllString(string(prompt), -1) {
		var c fakeCard
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			fmt.Fprintln(os.Stderr, "fake claude: bad draft line:", err)
			return 2
		}
		c.English = "meaning of " + c.Arabic
		c.Example = fakeExample
		c.ExampleEN = "The boy read the book at home."
		if c.Forms == nil {
			c.Forms = []json.RawMessage{}
		}
		if mode := os.Getenv("FAKE_CLAUDE_MODE"); mode == "rare-always" || mode == "rare-first" && call == 1 {
			c.Example = fakeRareExample
		}
		if os.Getenv("FAKE_CLAUDE_MODE") == "bad-first" && call == 1 {
			c.Example = "قَرَأَ الْوَلَدُ الْكِتَابَ"
		}
		if os.Getenv("FAKE_CLAUDE_MODE") == "switch-pos" && strings.Contains(string(prompt), "asked for this entry") && call == 1 {
			c.POS = "verb"
		}
		cards = append(cards, c)
	}
	return emit(map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"structured_output": map[string]any{"cards": cards},
		"usage":             map[string]any{"input_tokens": 100, "output_tokens": 10, "cache_read_input_tokens": 5, "cache_creation_input_tokens": 0},
		"modelUsage":        map[string]any{"fake-model": map[string]any{}},
		"total_cost_usd":    0.01,
	})
}

func bump(path string) int {
	if path == "" {
		return 1
	}
	raw, _ := os.ReadFile(path)
	n, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	n++
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0o644)
	return n
}

func flagLine(args []string) string {
	var parts []string
	for i, a := range args {
		if (a == "--model" || a == "--effort") && i+1 < len(args) {
			parts = append(parts, strings.TrimPrefix(a, "--")+"="+args[i+1])
		}
	}
	return strings.Join(parts, " ")
}
