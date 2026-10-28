package mistral

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var ErrNoKey = errors.New("no Mistral API key found")

type Candidate struct {
	Source string
	Value  string
}

type Key struct {
	Value  string
	Source string
}

func ParseKey(source, raw string) (string, error) {
	key := strings.TrimSpace(raw)
	for _, r := range key {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", fmt.Errorf("%s contains whitespace or control characters; it must be the API key itself", source)
		}
	}
	return key, nil
}

func ResolveKey(candidates []Candidate) (Key, error) {
	for _, c := range candidates {
		value, err := ParseKey(c.Source, c.Value)
		if err != nil {
			return Key{}, err
		}
		if value == "" {
			continue
		}
		return Key{Value: value, Source: c.Source}, nil
	}
	return Key{}, ErrNoKey
}
