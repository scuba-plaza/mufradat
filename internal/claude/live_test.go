package claude_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/scuba-plaza/mufradat/internal/claude"
	"github.com/scuba-plaza/mufradat/internal/llm"
	"github.com/scuba-plaza/mufradat/internal/proc"
)

func TestLiveContract(t *testing.T) {
	if os.Getenv("MUFRADAT_LIVE") == "" {
		t.Skip("set MUFRADAT_LIVE=1 to call the real claude binary")
	}
	b := claude.Backend{Runner: proc.NewOS(), Model: "haiku", Effort: "low"}
	resp, err := b.Complete(context.Background(), llm.Request{
		System: "You answer in one word.",
		Prompt: "Reply with the word ok.",
		Schema: []byte(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var out struct{ Answer string }
	if err := json.Unmarshal(resp.Output, &out); err != nil || out.Answer == "" {
		t.Fatalf("output %s: %v", resp.Output, err)
	}
	if resp.Usage.Calls != 1 || resp.Usage.Output == 0 {
		t.Fatalf("usage %+v", resp.Usage)
	}
}
