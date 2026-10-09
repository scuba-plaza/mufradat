package jsonl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/scuba-plaza/mufradat/internal/fsutil"
)

const maxLine = 64 << 20

func Decode[T any](r io.Reader) ([]T, error) {
	var out []T
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), maxLine)
	line := 0
	for sc.Scan() {
		line++
		text := bytes.TrimSpace(sc.Bytes())
		if len(text) == 0 {
			continue
		}
		var v T
		dec := json.NewDecoder(bytes.NewReader(text))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		out = append(out, v)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ReadFile[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out, err := Decode[T](f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return out, nil
}

func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func Encode[T any](w io.Writer, items []T) error {
	for _, item := range items {
		line, err := Marshal(item)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func WriteFile[T any](path string, items []T) error {
	var buf bytes.Buffer
	if err := Encode(&buf, items); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, buf.Bytes(), 0o644)
}

func Append(path string, v any) error {
	line, err := Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func String(items ...any) string {
	var b strings.Builder
	for _, item := range items {
		line, err := Marshal(item)
		if err != nil {
			panic(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.String()
}
