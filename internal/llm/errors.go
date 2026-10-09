package llm

import (
	"errors"
	"fmt"
	"time"
)

type UsageLimitError struct {
	ResetAt time.Time
	Message string
}

func (e *UsageLimitError) Error() string { return "usage limit reached: " + e.Message }

type RateLimitedError struct {
	RetryAfter time.Duration
	Message    string
}

func (e *RateLimitedError) Error() string { return "rate limited: " + e.Message }

type TransientError struct{ Message string }

func (e *TransientError) Error() string { return "temporary failure: " + e.Message }

type AuthError struct{ Message string }

func (e *AuthError) Error() string { return "not signed in: " + e.Message }

type BadOutputError struct{ Message string }

func (e *BadOutputError) Error() string { return "unusable answer: " + e.Message }

type UnavailableError struct{ Err error }

func (e *UnavailableError) Error() string { return fmt.Sprintf("backend unavailable: %v", e.Err) }

func (e *UnavailableError) Unwrap() error { return e.Err }

type CallError struct{ Message string }

func (e *CallError) Error() string { return e.Message }

func AsUsageLimit(err error) (*UsageLimitError, bool) {
	var e *UsageLimitError
	return e, errors.As(err, &e)
}

func AsAuth(err error) bool {
	var e *AuthError
	return errors.As(err, &e)
}

func AsUnavailable(err error) bool {
	var e *UnavailableError
	return errors.As(err, &e)
}

func Retryable(err error) bool {
	var transient *TransientError
	var limited *RateLimitedError
	return errors.As(err, &transient) || errors.As(err, &limited)
}
