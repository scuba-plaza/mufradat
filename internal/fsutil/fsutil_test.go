package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomicCreatesFoldersSetsTheModeAndReplacesWithoutLeftovers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "file.txt")
	if err := WriteFileAtomic(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "second" {
		t.Fatalf("content %q, %v", got, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("mode %v, %v", info.Mode(), err)
	}
	if entries, err := os.ReadDir(filepath.Dir(path)); err != nil || len(entries) != 1 {
		t.Errorf("the folder holds %v, %v: temporary files must not be left behind", entries, err)
	}
	private := filepath.Join(dir, "private")
	if err := WriteFileAtomic(private, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(private); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("a private file must stay private: %v, %v", info.Mode(), err)
	}
}

func TestWriteFileAtomicReportsAFailureAndLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(filepath.Join(target, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(target, []byte("x"), 0o644); err == nil {
		t.Error("replacing a folder with a file reported success")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("a failed write left %v behind", entries)
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(filepath.Join(blocker, "sub", "file"), []byte("x"), 0o644); err == nil {
		t.Error("a folder that cannot be made reported success")
	}
}

func TestReadFileIfExistsTellsMissingFromUnreadable(t *testing.T) {
	dir := t.TempDir()
	if data, err := ReadFileIfExists(filepath.Join(dir, "missing")); data != nil || err != nil {
		t.Errorf("a missing file: %q, %v", data, err)
	}
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadFileIfExists(path); data == nil || len(data) != 0 || err != nil {
		t.Errorf("an empty file is not a missing one: %#v, %v", data, err)
	}
	if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadFileIfExists(path); string(data) != "hi" || err != nil {
		t.Errorf("%q, %v", data, err)
	}
	if _, err := ReadFileIfExists(dir); err == nil || !strings.HasPrefix(err.Error(), "read "+dir+": ") {
		t.Errorf("reading a folder: %v", err)
	}
}

func TestExistsAndIsDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		path        string
		exists, dir bool
	}{
		{dir, true, true},
		{file, true, false},
		{filepath.Join(dir, "none"), false, false},
	} {
		if Exists(c.path) != c.exists || IsDir(c.path) != c.dir {
			t.Errorf("%s: Exists %v IsDir %v, want %v %v", c.path, Exists(c.path), IsDir(c.path), c.exists, c.dir)
		}
	}
}
