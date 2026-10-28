package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetPrefersTheStampedValuesAndFillsTheRestFromTheRuntime(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = oldVersion, oldCommit, oldDate })

	Version, Commit, Date = "v1.2.3", "abcdef0123456789", "2026-01-02T03:04:05Z"
	got := Get()
	want := Info{Version: "v1.2.3", Commit: "abcdef0123456789", Date: "2026-01-02T03:04:05Z", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}
	if got != want {
		t.Errorf("stamped: %+v, want %+v", got, want)
	}

	Version, Commit, Date = "", "", ""
	got = Get()
	if got.Version != "dev" {
		t.Errorf("an unstamped build is %q, want dev", got.Version)
	}
	if got.Go != runtime.Version() || got.OS != runtime.GOOS || got.Arch != runtime.GOARCH {
		t.Errorf("runtime facts: %+v", got)
	}
}

func TestTheVersionLineShortensTheCommitAndNamesThePlatform(t *testing.T) {
	platform := " " + runtime.GOOS + "/" + runtime.GOARCH + " " + runtime.Version()
	for _, c := range []struct {
		name string
		info Info
		want string
	}{
		{"no commit", Info{Version: "v1", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}, "v1" + platform},
		{"a short commit is kept whole", Info{Version: "v1", Commit: "abc", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}, "v1 (abc)" + platform},
		{"twelve characters are kept whole", Info{Version: "v1", Commit: "0123456789ab", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}, "v1 (0123456789ab)" + platform},
		{"thirteen characters are cut to twelve", Info{Version: "v1", Commit: "0123456789abc", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}, "v1 (0123456789ab)" + platform},
		{"a longer commit is cut to twelve", Info{Version: "v1", Commit: "0123456789abcdef", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}, "v1 (0123456789ab)" + platform},
	} {
		if got := c.info.String(); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	if !strings.HasPrefix(Get().String(), Get().Version) {
		t.Error("the line starts with the version")
	}
}
