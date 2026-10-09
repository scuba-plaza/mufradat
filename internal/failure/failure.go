package failure

import (
	"errors"
	"fmt"
)

const (
	ExitFailure       = 1
	ExitUsage         = 2
	ExitNotSetUp      = 3
	ExitChecksFailed  = 4
	ExitTemporary     = 75
	ExitInterrupted   = 130
	exitSuccess       = 0
	defaultFailureMsg = "failed"
)

type Error struct {
	Err  error
	Code int
	Hint string
}

func (e *Error) Error() string {
	if e.Err == nil {
		return defaultFailureMsg
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

func New(code int, hint string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Err: err, Code: code, Hint: hint}
}

func Newf(code int, hint, format string, args ...any) error {
	return &Error{Err: fmt.Errorf(format, args...), Code: code, Hint: hint}
}

func NotSetUp(hint string, err error) error { return New(ExitNotSetUp, hint, err) }

func Usage(err error) error { return New(ExitUsage, "", err) }

func Temporary(hint string, err error) error { return New(ExitTemporary, hint, err) }

func Code(err error) int {
	if err == nil {
		return exitSuccess
	}
	var f *Error
	if errors.As(err, &f) && f.Code != 0 {
		return f.Code
	}
	return ExitFailure
}

func Hint(err error) string {
	var f *Error
	if errors.As(err, &f) {
		return f.Hint
	}
	return ""
}
