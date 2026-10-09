package python

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedFiles(t *testing.T) {
	lock, err := fs.ReadFile(Files, "requirements.lock")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(lock), "--extra-index-url https://download.pytorch.org/whl/cpu") {
		t.Fatalf("the lock must start with the CPU wheel index, got %q", strings.SplitN(string(lock), "\n", 2)[0])
	}
	if strings.Contains(string(lock), "nvidia-") || strings.Contains(string(lock), "triton") {
		t.Fatal("the lock pulls in CUDA packages")
	}
	for _, line := range strings.Split(string(lock), "\n") {
		if strings.Contains(line, "==") && !strings.HasPrefix(line, " ") && !strings.Contains(line, "\\") && !strings.Contains(line, ";") {
			t.Errorf("unhashed requirement: %q", line)
		}
	}
	for _, name := range []string{"__init__.py", "__main__.py", "engine.py", "protocol.py", "selftest.py", "serve.py"} {
		if _, err := fs.Stat(Files, "mufradat_sidecar/"+name); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}
