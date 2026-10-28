package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

var (
	Version = ""
	Commit  = ""
	Date    = ""
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func Get() Info {
	bi, ok := debug.ReadBuildInfo()
	info := resolve(Info{Version: Version, Commit: Commit, Date: Date}, bi, ok)
	info.Go, info.OS, info.Arch = runtime.Version(), runtime.GOOS, runtime.GOARCH
	return info
}

func resolve(info Info, bi *debug.BuildInfo, ok bool) Info {
	if !ok {
		if info.Version == "" {
			info.Version = "dev"
		}
		return info
	}
	if info.Version == "" {
		info.Version = bi.Main.Version
		if info.Version == "" || info.Version == "(devel)" {
			info.Version = "dev"
		}
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if info.Commit == "" {
				info.Commit = s.Value
			}
		case "vcs.time":
			if info.Date == "" {
				info.Date = s.Value
			}
		}
	}
	return info
}

func (i Info) String() string {
	s := i.Version
	if i.Commit != "" {
		c := i.Commit
		if len(c) > 12 {
			c = c[:12]
		}
		s += fmt.Sprintf(" (%s)", c)
	}
	return fmt.Sprintf("%s %s/%s %s", s, i.OS, i.Arch, i.Go)
}
