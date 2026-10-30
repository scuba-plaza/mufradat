package mistral

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/llm"
)

const (
	DefaultTemperature = 0.3
	DefaultMaxTokens   = 16384
	schemaName         = "response"
)

type Backend struct {
	Client      *Client
	Model       string
	Temperature float64
	MaxTokens   int
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type jsonSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

type responseFormat struct {
	Type       string     `json:"type"`
	JSONSchema jsonSchema `json:"json_schema"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	MaxTokens      int             `json:"max_tokens"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int64 `json:"prompt_tokens"`
		CompletionTokens    int64 `json:"completion_tokens"`
		PromptTokensDetails struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

func (b Backend) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	body := chatRequest{
		Model:       b.Model,
		Messages:    []message{{Role: "system", Content: req.System}, {Role: "user", Content: req.Prompt}},
		Temperature: b.Temperature,
		MaxTokens:   b.MaxTokens,
	}
	if body.Temperature == 0 {
		body.Temperature = DefaultTemperature
	}
	if body.MaxTokens == 0 {
		body.MaxTokens = DefaultMaxTokens
	}
	if len(req.Schema) > 0 {
		if !json.Valid(req.Schema) {
			return llm.Response{}, &llm.CallError{Message: "the answer schema is not valid JSON"}
		}
		body.ResponseFormat = &responseFormat{Type: "json_schema", JSONSchema: jsonSchema{Name: schemaName, Schema: req.Schema, Strict: true}}
	}
	var reply chatResponse
	if err := b.Client.do(ctx, http.MethodPost, PathChat, body, &reply); err != nil {
		return llm.Response{}, chatError(err)
	}
	if len(reply.Choices) == 0 {
		return llm.Response{}, &llm.BadOutputError{Message: "the reply had no choices"}
	}
	choice := reply.Choices[0]
	text := strings.TrimSpace(contentText(choice.Message.Content))
	if choice.FinishReason == "length" {
		return llm.Response{}, &llm.BadOutputError{Message: fmt.Sprintf("the answer was cut off at %d tokens", body.MaxTokens)}
	}
	if text == "" {
		return llm.Response{}, &llm.BadOutputError{Message: "the answer was empty"}
	}
	if !json.Valid([]byte(text)) {
		return llm.Response{}, &llm.BadOutputError{Message: "the answer is not JSON: " + clip(text, 120)}
	}
	model := reply.Model
	if model == "" {
		model = b.Model
	}
	usage := llm.Usage{
		Calls:     1,
		Input:     reply.Usage.PromptTokens,
		Output:    reply.Usage.CompletionTokens,
		CacheRead: reply.Usage.PromptTokensDetails.CachedTokens,
		Models:    []string{model},
	}
	return llm.Response{Output: json.RawMessage(text), Usage: usage}, nil
}

func contentText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	case '[':
		var chunks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &chunks) == nil {
			var b strings.Builder
			for _, c := range chunks {
				if c.Type == "text" {
					b.WriteString(c.Text)
				}
			}
			return b.String()
		}
	}
	return ""
}

func chatError(err error) error {
	var api *APIError
	if !errors.As(err, &api) {
		return err
	}
	switch api.Kind() {
	case KindAuth:
		return &llm.AuthError{Message: api.Message}
	case KindRateLimited:
		return &llm.RateLimitedError{RetryAfter: api.RetryAfter, Message: api.Message}
	case KindTransient:
		return &llm.TransientError{Message: api.Message}
	}
	return &llm.CallError{Message: api.Message}
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
