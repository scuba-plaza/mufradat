package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/scuba-plaza/mufradat/internal/proc"
)

//go:embed camel_lemmas.py
var camelScript []byte

func analyse(ctx context.Context, runner proc.Runner, python string, environ []string, out string, lists []string, stderr io.Writer) error {
	dir := filepath.Dir(out)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	script := filepath.Join(dir, "camel_lemmas.py")
	if err := os.WriteFile(script, camelScript, 0o644); err != nil {
		return err
	}
	partial := out + ".partial"
	args := append([]string{script, partial}, lists...)
	err := runner.Run(ctx, proc.Cmd{Name: python, Args: args, Env: environ, Stdout: stderr, Stderr: stderr})
	if err != nil {
		os.Remove(partial)
		return fmt.Errorf("analysing the frequent word forms with CAMeL Tools: %w", err)
	}
	return os.Rename(partial, out)
}
