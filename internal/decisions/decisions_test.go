package decisions

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDismissAndRestore(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }
	log := NewLog(filepath.Join(t.TempDir(), "state", "decisions.jsonl"), now)

	set, err := log.Load()
	if err != nil || set.Count() != 0 {
		t.Fatalf("empty log: %v, %d", err, set.Count())
	}
	if err := log.Dismiss("n_a", "كِتَاب"); err != nil {
		t.Fatal(err)
	}
	if err := log.Dismiss("n_b", "example:unchecked"); err != nil {
		t.Fatal(err)
	}
	set, _ = log.Load()
	if !set.Dismissed("n_a", "كِتَاب") || !set.Dismissed("n_b", "example:unchecked") || set.Dismissed("n_a", "other") {
		t.Fatalf("dismissals not read back: %+v", set)
	}
	if err := log.Restore("n_a", "كِتَاب"); err != nil {
		t.Fatal(err)
	}
	set, _ = log.Load()
	if set.Dismissed("n_a", "كِتَاب") || !set.Dismissed("n_b", "example:unchecked") {
		t.Fatalf("restore did not apply: %+v", set)
	}
}

func TestRejectsEmptyAndUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	log := NewLog(path, time.Now)
	if err := log.Dismiss("", "x"); err == nil {
		t.Fatal("an empty note id should be rejected")
	}
	if err := log.Dismiss("n_a", ""); err == nil {
		t.Fatal("an empty key should be rejected")
	}
	if err := writeRaw(path, `{"kind":"mystery","note":"n_a","key":"k","at":"2026-10-08T12:00:00Z"}`+"\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Load(); err == nil {
		t.Fatal("an unknown kind should fail to load")
	}
}
