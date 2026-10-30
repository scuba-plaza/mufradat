package mistral

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scuba-plaza/mufradat/internal/httpretry"
)

const (
	DefaultBaseURL = "https://api.mistral.ai"
	maxBodyBytes   = 32 << 20
)

const (
	PathChat   = "/v1/chat/completions"
	PathModels = "/v1/models"
)

type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return DefaultBaseURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *Client) CheckKey(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, PathModels, nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encoding the request: %w", err)
		}
		body = bytes.NewReader(raw)
	}
	target := c.base() + path
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return &APIError{Message: "could not reach the service: " + err.Error(), Err: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return &APIError{Message: "the connection broke while reading the reply: " + err.Error(), Err: err}
	}
	if resp.StatusCode/100 != 2 {
		return newAPIError(resp.StatusCode, raw, httpretry.RetryAfter(resp.Header, time.Now()))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("unreadable reply from %s: %w", path, err)
	}
	return nil
}
