package note

import (
	"errors"
	"strings"
)

var ErrExampleMarkup = errors.New("the example must mark the target word with exactly one pair of [square brackets]")

type ExampleParts struct {
	Before string
	Target string
	After  string
}

func SplitExample(s string) (ExampleParts, error) {
	if strings.Count(s, "[") != 1 || strings.Count(s, "]") != 1 {
		return ExampleParts{}, ErrExampleMarkup
	}
	open, closing := strings.Index(s, "["), strings.Index(s, "]")
	if closing < open {
		return ExampleParts{}, ErrExampleMarkup
	}
	parts := ExampleParts{Before: s[:open], Target: s[open+1 : closing], After: s[closing+1:]}
	if strings.TrimSpace(parts.Target) == "" {
		return ExampleParts{}, errors.New("the marked word in the example is empty")
	}
	return parts, nil
}

func (p ExampleParts) Plain() string {
	return p.Before + p.Target + p.After
}

func PlainExample(s string) string {
	parts, err := SplitExample(s)
	if err != nil {
		return strings.NewReplacer("[", "", "]", "").Replace(s)
	}
	return parts.Plain()
}

func MarkExample(sentence, target string) (string, bool) {
	i := strings.Index(sentence, target)
	if i < 0 || target == "" {
		return "", false
	}
	return sentence[:i] + "[" + target + "]" + sentence[i+len(target):], true
}
