package llm

import (
	"errors"
	"time"
)

type RateLimitedError struct {
	RetryAfter time.Duration
	Message    string
}

func (e *RateLimitedError) Error() string { return "rate limited: " + e.Message }

type TransientError struct{ Message string }

func (e *TransientError) Error() string { return "temporary failure: " + e.Message }

type AuthError struct{ Message string }

func (e *AuthError) Error() string { return "the service rejected the credentials: " + e.Message }

type BadOutputError struct{ Message string }

func (e *BadOutputError) Error() string { return "unusable answer: " + e.Message }

type CallError struct{ Message string }

func (e *CallError) Error() string { return e.Message }

func AsAuth(err error) bool {
	var e *AuthError
	return errors.As(err, &e)
}

func Retryable(err error) bool {
	var transient *TransientError
	var limited *RateLimitedError
	return errors.As(err, &transient) || errors.As(err, &limited)
}
