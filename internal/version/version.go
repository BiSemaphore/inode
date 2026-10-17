package version

import (
	"fmt"
	"runtime/debug"
)

// Set at build time via -ldflags by goreleaser and the Makefile. When they
// are left at these defaults (go install, plain go build), String reads the
// version Go embeds in the binary instead.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info is the resolved version of the running binary.
type Info struct {
	Version string
	Commit  string
	Date    string
}

// Get resolves the version: ldflags values when set, otherwise the module
// version and VCS stamp from runtime/debug.ReadBuildInfo.
func Get() Info {
	bi, _ := debug.ReadBuildInfo()
	return resolve(Version, Commit, Date, bi)
}

// String is the one-line form printed by `inode version`.
func String() string {
	return Get().String()
}

func (i Info) String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", i.Version, i.Commit, i.Date)
}

// resolve keeps ldflags values when a release build set them. Otherwise it
// uses the build info: `go install module@v0.5.0` records Main.Version
// "v0.5.0"; a build inside a git checkout records a pseudo-version (or
// "(devel)") plus vcs.revision, vcs.time and vcs.modified.
func resolve(version, commit, date string, bi *debug.BuildInfo) Info {
	if version != "dev" || bi == nil {
		return Info{Version: version, Commit: commit, Date: date}
	}
	info := Info{Version: bi.Main.Version, Commit: commit, Date: date}
	if info.Version == "" {
		info.Version = "(devel)"
	}
	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Commit = s.Value
			if len(info.Commit) > 7 {
				info.Commit = info.Commit[:7]
			}
		case "vcs.time":
			info.Date = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if modified && info.Commit != commit {
		info.Commit += "-dirty"
	}
	return info
}
