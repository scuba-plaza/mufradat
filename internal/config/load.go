package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/scuba-plaza/mufradat/internal/fsutil"
)

func Load(files ...string) (Config, error) {
	cfg := Default()
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return cfg, err
		}
		if err := decode(raw, &cfg); err != nil {
			return cfg, fmt.Errorf("%s: %w", path, err)
		}
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

func decode(raw []byte, cfg *Config) error {
	dec := toml.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			return errors.New(strings.TrimSpace(strict.String()))
		}
		return err
	}
	return nil
}

func Flatten(c Config) (map[string]any, error) {
	raw, err := toml.Marshal(c)
	if err != nil {
		return nil, err
	}
	var tree map[string]any
	if err := toml.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	out := map[string]any{}
	flatten("", tree, out)
	return out, nil
}

func flatten(prefix string, tree map[string]any, out map[string]any) {
	for k, v := range tree {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if sub, ok := v.(map[string]any); ok {
			flatten(key, sub, out)
			continue
		}
		out[key] = v
	}
}

func Keys() []string {
	flat, _ := Flatten(Default())
	return slices.Sorted(maps.Keys(flat))
}

func Get(c Config, key string) (any, error) {
	flat, err := Flatten(c)
	if err != nil {
		return nil, err
	}
	v, ok := flat[key]
	if !ok {
		return nil, unknownKey(key)
	}
	return v, nil
}

func unknownKey(key string) error {
	return fmt.Errorf("unknown key %q (known keys: %s)", key, strings.Join(Keys(), ", "))
}

func SetFile(path, key, value string) error {
	defaults, err := Flatten(Default())
	if err != nil {
		return err
	}
	sample, ok := defaults[key]
	if !ok {
		return unknownKey(key)
	}
	typed, err := parseLike(sample, value)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	raw, err := fsutil.ReadFileIfExists(path)
	if err != nil {
		return err
	}
	tree := map[string]any{}
	if len(raw) > 0 {
		if err := toml.Unmarshal(raw, &tree); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	assign(tree, strings.Split(key, "."), typed)
	out, err := toml.Marshal(tree)
	if err != nil {
		return err
	}
	check := Default()
	if err := decode(out, &check); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if err := check.Validate(); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return fsutil.WriteFileAtomic(path, out, 0o644)
}

func assign(tree map[string]any, path []string, v any) {
	if len(path) == 1 {
		tree[path[0]] = v
		return
	}
	sub, ok := tree[path[0]].(map[string]any)
	if !ok {
		sub = map[string]any{}
		tree[path[0]] = sub
	}
	assign(sub, path[1:], v)
}

func parseLike(sample any, value string) (any, error) {
	switch sample.(type) {
	case int64:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", value)
		}
		return n, nil
	case float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", value)
		}
		return f, nil
	}
	return value, nil
}

func WriteNew(path string, tree map[string]any) error {
	out, err := toml.Marshal(tree)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, out, 0o644)
}
