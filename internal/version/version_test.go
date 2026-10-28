package version

import (
	"runtime"
	"runtime/debug"
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

func TestResolveFillsWhatWasNotStampedFromTheBuildInfo(t *testing.T) {
	build := func(main string, settings ...debug.BuildSetting) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Version: main}, Settings: settings}
	}
	vcs := []debug.BuildSetting{{Key: "vcs.revision", Value: "rev123"}, {Key: "vcs.time", Value: "2026-02-03T04:05:06Z"}, {Key: "vcs.modified", Value: "true"}}
	for _, c := range []struct {
		name    string
		stamped Info
		bi      *debug.BuildInfo
		ok      bool
		want    Info
	}{
		{"no build info and nothing stamped", Info{}, nil, false, Info{Version: "dev"}},
		{"no build info keeps what was stamped", Info{Version: "v9", Commit: "c", Date: "d"}, nil, false, Info{Version: "v9", Commit: "c", Date: "d"}},
		{"a module version from go install", Info{}, build("v1.4.0"), true, Info{Version: "v1.4.0"}},
		{"a development build", Info{}, build("(devel)"), true, Info{Version: "dev"}},
		{"a build without a module version", Info{}, build(""), true, Info{Version: "dev"}},
		{"stamped version wins over the module version", Info{Version: "v9"}, build("v1.4.0"), true, Info{Version: "v9"}},
		{"stamped version wins over a development build", Info{Version: "v9"}, build("(devel)"), true, Info{Version: "v9"}},
		{"commit and date come from the version control settings", Info{}, build("v1.4.0", vcs...), true, Info{Version: "v1.4.0", Commit: "rev123", Date: "2026-02-03T04:05:06Z"}},
		{"stamped commit and date win over the settings", Info{Commit: "c", Date: "d"}, build("v1.4.0", vcs...), true, Info{Version: "v1.4.0", Commit: "c", Date: "d"}},
		{"only the commit is stamped", Info{Commit: "c"}, build("v1.4.0", vcs...), true, Info{Version: "v1.4.0", Commit: "c", Date: "2026-02-03T04:05:06Z"}},
		{"only the date is stamped", Info{Date: "d"}, build("v1.4.0", vcs...), true, Info{Version: "v1.4.0", Commit: "rev123", Date: "d"}},
	} {
		if got := resolve(c.stamped, c.bi, c.ok); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}
