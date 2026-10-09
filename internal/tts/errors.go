package tts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Kind int

const (
	KindOther Kind = iota
	KindAuth
	KindQuota
	KindRateLimited
	KindTransient
	KindBadRequest
)

type APIError struct {
	Status     int
	Reason     string
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	return fmt.Sprintf("text-to-speech: %s (HTTP %d)", msg, e.Status)
}

func (e *APIError) Kind() Kind {
	switch {
	case e.Status == http.StatusUnauthorized, e.Status == http.StatusForbidden && !e.quota():
		return KindAuth
	case e.quota():
		return KindQuota
	case e.Status == http.StatusTooManyRequests:
		return KindRateLimited
	case e.Status >= 500:
		return KindTransient
	case e.Status == http.StatusBadRequest:
		return KindBadRequest
	}
	return KindOther
}

func (e *APIError) quota() bool {
	r := strings.ToUpper(e.Reason)
	return strings.Contains(r, "QUOTA") || strings.Contains(r, "BILLING") || strings.Contains(r, "SERVICE_DISABLED")
}

func (e *APIError) Fatal() bool {
	switch e.Kind() {
	case KindAuth, KindQuota:
		return true
	}
	return false
}

func (e *APIError) Retryable() bool {
	switch e.Kind() {
	case KindRateLimited, KindTransient:
		return true
	}
	return false
}

type apiErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			Reason string `json:"reason"`
		} `json:"details"`
	} `json:"error"`
}

func newAPIError(status int, body []byte, wait time.Duration) *APIError {
	e := &APIError{Status: status, RetryAfter: wait}
	var parsed apiErrorBody
	if json.Unmarshal(body, &parsed) == nil {
		e.Message = parsed.Error.Message
		e.Reason = parsed.Error.Status
		for _, d := range parsed.Error.Details {
			if d.Reason != "" {
				e.Reason += " " + d.Reason
			}
		}
	}
	if e.Message == "" {
		e.Message = strings.TrimSpace(string(body))
		if len(e.Message) > 200 {
			e.Message = e.Message[:200]
		}
	}
	return e
}

func retryAfter(h http.Header, now time.Time) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}
