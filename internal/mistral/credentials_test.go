package mistral

import (
	"errors"
	"strings"
	"testing"
)

func TestResolveKeyPrefersEarlierSources(t *testing.T) {
	key, err := ResolveKey([]Candidate{
		{Source: "MISTRAL_API_KEY", Value: "from-env"},
		{Source: "mistral.api_key", Value: "from-config"},
		{Source: "imported key", Value: "from-file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if key.Value != "from-env" || key.Source != "MISTRAL_API_KEY" {
		t.Fatalf("got %+v", key)
	}
	key, err = ResolveKey([]Candidate{
		{Source: "MISTRAL_API_KEY", Value: ""},
		{Source: "mistral.api_key", Value: "from-config"},
		{Source: "imported key", Value: "from-file"},
	})
	if err != nil || key.Value != "from-config" || key.Source != "mistral.api_key" {
		t.Fatalf("got %+v, %v", key, err)
	}
}

func TestResolveKeyTrimsSurroundingWhitespace(t *testing.T) {
	for _, raw := range []string{"abc123\n", "abc123\r\n", "  abc123\t\n", "\nabc123"} {
		key, err := ResolveKey([]Candidate{{Source: "imported key", Value: raw}})
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if key.Value != "abc123" {
			t.Errorf("%q: key %q", raw, key.Value)
		}
	}
}

func TestResolveKeySkipsSourcesThatAreBlankAfterTrimming(t *testing.T) {
	key, err := ResolveKey([]Candidate{
		{Source: "MISTRAL_API_KEY", Value: " \n"},
		{Source: "imported key", Value: "real\n"},
	})
	if err != nil || key.Value != "real" || key.Source != "imported key" {
		t.Fatalf("got %+v, %v", key, err)
	}
}

func TestResolveKeyRejectsInnerWhitespaceAndControlCharactersNamingTheSource(t *testing.T) {
	for _, raw := range []string{"abc def", "abc\tdef", "abc\ndef", "abc\x00def", "abc\x1bdef", "ab cd"} {
		_, err := ResolveKey([]Candidate{
			{Source: "mistral.api_key", Value: raw},
			{Source: "imported key", Value: "fallback"},
		})
		if err == nil {
			t.Errorf("%q: expected an error, the next source must not hide a bad key", raw)
			continue
		}
		if !strings.Contains(err.Error(), "mistral.api_key") || !strings.Contains(err.Error(), "whitespace or control") {
			t.Errorf("%q: error %q should name the source and the problem", raw, err)
		}
		if errors.Is(err, ErrNoKey) {
			t.Errorf("%q: a malformed key is not the same as no key", raw)
		}
	}
}

func TestResolveKeyReportsNoKey(t *testing.T) {
	_, err := ResolveKey([]Candidate{{Source: "MISTRAL_API_KEY"}, {Source: "mistral.api_key", Value: "  "}})
	if !errors.Is(err, ErrNoKey) {
		t.Fatalf("got %v", err)
	}
	if _, err := ResolveKey(nil); !errors.Is(err, ErrNoKey) {
		t.Fatalf("got %v", err)
	}
}
