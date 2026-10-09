package llm

import (
	"context"
	"encoding/json"
	"slices"
	"time"
)

type Request struct {
	System string
	Prompt string
	Schema []byte
}

type Usage struct {
	Calls      int      `json:"calls"`
	Input      int64    `json:"input"`
	Output     int64    `json:"output"`
	CacheRead  int64    `json:"cache_read"`
	CacheWrite int64    `json:"cache_write"`
	CostUSD    float64  `json:"cost_usd,omitempty"`
	Models     []string `json:"models,omitempty"`
}

func (u *Usage) Add(v Usage) {
	u.Calls += v.Calls
	u.Input += v.Input
	u.Output += v.Output
	u.CacheRead += v.CacheRead
	u.CacheWrite += v.CacheWrite
	u.CostUSD += v.CostUSD
	for _, m := range v.Models {
		if !slices.Contains(u.Models, m) {
			u.Models = append(u.Models, m)
		}
	}
	slices.Sort(u.Models)
}

type Response struct {
	Output json.RawMessage
	Usage  Usage
}

type Backend interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

type Record struct {
	Time  time.Time `json:"time"`
	Usage Usage     `json:"usage"`
}
