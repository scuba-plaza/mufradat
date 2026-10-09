package verify

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/scuba-plaza/mufradat/internal/jsonl"
)

type cacheLine struct {
	Fingerprint string `json:"fp"`
	Reading
}

type Cache struct {
	path string
}

func NewCache(path string) *Cache { return &Cache{path: path} }

type Loaded struct {
	Readings map[string]Reading
	Stale    int
}

func (c *Cache) Load(fingerprint string) (Loaded, error) {
	lines, err := jsonl.ReadFile[cacheLine](c.path)
	if err != nil {
		return Loaded{}, err
	}
	out := Loaded{Readings: make(map[string]Reading, len(lines))}
	for _, l := range lines {
		if l.Fingerprint != fingerprint {
			out.Stale++
			continue
		}
		out.Readings[l.Key] = l.Reading
	}
	return out, nil
}

func (c *Cache) Append(fingerprint string, readings []Reading) error {
	if len(readings) == 0 {
		return nil
	}
	lines := make([]cacheLine, len(readings))
	for i, r := range readings {
		lines[i] = cacheLine{Fingerprint: fingerprint, Reading: r}
	}
	var buf bytes.Buffer
	if err := jsonl.Encode(&buf, lines); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(c.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (c *Cache) Compact(fingerprint string, keep map[string]bool) (int, error) {
	lines, err := jsonl.ReadFile[cacheLine](c.path)
	if err != nil {
		return 0, err
	}
	latest := make(map[string]int, len(lines))
	for i, l := range lines {
		if l.Fingerprint == fingerprint && keep[l.Key] {
			latest[l.Key] = i
		}
	}
	kept := make([]cacheLine, 0, len(latest))
	for i, l := range lines {
		if idx, ok := latest[l.Key]; ok && idx == i && l.Fingerprint == fingerprint {
			kept = append(kept, l)
		}
	}
	dropped := len(lines) - len(kept)
	if dropped == 0 {
		return 0, nil
	}
	return dropped, jsonl.WriteFile(c.path, kept)
}
