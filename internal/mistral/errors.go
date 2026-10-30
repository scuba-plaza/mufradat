package mistral

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Kind int

const (
	KindOther Kind = iota
	KindAuth
	KindRateLimited
	KindTransient
	KindBadRequest
	KindBlocked
)

const guardrailType = "guardrail_violation"

type APIError struct {
	Status     int
	Type       string
	Code       string
	Message    string
	RetryAfter time.Duration
	Err        error
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	if e.Status == 0 {
		return "mistral: " + msg
	}
	return fmt.Sprintf("mistral: %s (HTTP %d)", msg, e.Status)
}

func (e *APIError) Unwrap() error { return e.Err }

func (e *APIError) Kind() Kind {
	switch {
	case e.Status == 0:
		return KindTransient
	case e.Status == http.StatusForbidden && e.Type == guardrailType:
		return KindBlocked
	case e.Status == http.StatusUnauthorized, e.Status == http.StatusForbidden:
		return KindAuth
	case e.Status == http.StatusTooManyRequests:
		return KindRateLimited
	case e.Status >= 500:
		return KindTransient
	case e.Status >= 400:
		return KindBadRequest
	}
	return KindOther
}

type errorBody struct {
	Message json.RawMessage `json:"message"`
	Type    string          `json:"type"`
	Code    json.RawMessage `json:"code"`
}

const maxMessageBytes = 300

func newAPIError(status int, body []byte, wait time.Duration) *APIError {
	e := &APIError{Status: status, RetryAfter: wait}
	var parsed errorBody
	if json.Unmarshal(body, &parsed) == nil {
		e.Type = parsed.Type
		e.Code = scalar(parsed.Code)
		e.Message = scalar(parsed.Message)
	}
	if e.Message == "" {
		e.Message = strings.TrimSpace(string(body))
	}
	if len(e.Message) > maxMessageBytes {
		e.Message = e.Message[:maxMessageBytes]
	}
	return e
}

func scalar(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}
