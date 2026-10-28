package failure

import (
	"errors"
	"fmt"
	"testing"
)

func TestTheExitCodesAreTheDocumentedOnes(t *testing.T) {
	for name, c := range map[string]struct{ got, want int }{
		"failure":       {ExitFailure, 1},
		"usage":         {ExitUsage, 2},
		"not set up":    {ExitNotSetUp, 3},
		"checks failed": {ExitChecksFailed, 4},
		"temporary":     {ExitTemporary, 75},
		"interrupted":   {ExitInterrupted, 130},
	} {
		if c.got != c.want {
			t.Errorf("%s is %d, the README documents %d", name, c.got, c.want)
		}
	}
}

func TestEachConstructorCarriesItsCodeHintAndMessage(t *testing.T) {
	base := errors.New("boom")
	for _, c := range []struct {
		name string
		err  error
		code int
		hint string
	}{
		{"New", New(ExitChecksFailed, "fix it", base), 4, "fix it"},
		{"NotSetUp", NotSetUp("run init", base), 3, "run init"},
		{"Usage", Usage(base), 2, ""},
		{"Temporary", Temporary("later", base), 75, "later"},
		{"Newf", Newf(ExitFailure, "see docs", "%d things went %s", 2, "wrong"), 1, "see docs"},
	} {
		if Code(c.err) != c.code || Hint(c.err) != c.hint {
			t.Errorf("%s: code %d hint %q, want %d %q", c.name, Code(c.err), Hint(c.err), c.code, c.hint)
		}
	}
	if got := Newf(ExitFailure, "", "%d things went %s", 2, "wrong").Error(); got != "2 things went wrong" {
		t.Errorf("Newf message %q", got)
	}
	if got := Usage(base).Error(); got != "boom" {
		t.Errorf("the message is the wrapped error's own: %q", got)
	}
}

func TestCodeAndHintSeeThroughWrappingAndDefaultSensibly(t *testing.T) {
	inner := Temporary("try later", errors.New("busy"))
	wrapped := fmt.Errorf("while adding: %w", inner)
	if Code(wrapped) != ExitTemporary || Hint(wrapped) != "try later" {
		t.Errorf("wrapped: code %d hint %q", Code(wrapped), Hint(wrapped))
	}
	if Code(nil) != 0 || Hint(nil) != "" {
		t.Errorf("nil: code %d hint %q", Code(nil), Hint(nil))
	}
	if plain := errors.New("plain"); Code(plain) != ExitFailure || Hint(plain) != "" {
		t.Errorf("plain: code %d hint %q", Code(plain), Hint(plain))
	}
	if err := (&Error{Err: errors.New("no code set")}); Code(err) != ExitFailure {
		t.Errorf("an error without a code is a plain failure: %d", Code(err))
	}
	if New(ExitUsage, "h", nil) != nil || Usage(nil) != nil || NotSetUp("h", nil) != nil || Temporary("h", nil) != nil {
		t.Error("wrapping nothing gives nothing")
	}
	if got := (&Error{Code: ExitUsage}).Error(); got != "failed" {
		t.Errorf("an error without a message says %q", got)
	}
	if cause := errors.New("x"); !errors.Is(Usage(cause), cause) {
		t.Error("the wrapped error must stay reachable")
	}
}
