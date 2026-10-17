package version

import (
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	vcs := func(rev, tm, modified string) []debug.BuildSetting {
		return []debug.BuildSetting{
			{Key: "vcs", Value: "git"},
			{Key: "vcs.revision", Value: rev},
			{Key: "vcs.time", Value: tm},
			{Key: "vcs.modified", Value: modified},
		}
	}
	tests := []struct {
		name                  string
		version, commit, date string
		bi                    *debug.BuildInfo
		want                  Info
	}{
		{
			name:    "go install of a released version",
			version: "dev", commit: "none", date: "unknown",
			bi:   &debug.BuildInfo{Main: debug.Module{Version: "v0.5.0"}},
			want: Info{"v0.5.0", "none", "unknown"},
		},
		{
			name:    "build in a clean checkout",
			version: "dev", commit: "none", date: "unknown",
			bi: &debug.BuildInfo{
				Main:     debug.Module{Version: "v0.5.1-0.20261018120000-0123456789ab"},
				Settings: vcs("0123456789abcdef0123", "2026-10-18T12:00:00Z", "false"),
			},
			want: Info{"v0.5.1-0.20261018120000-0123456789ab", "0123456", "2026-10-18T12:00:00Z"},
		},
		{
			name:    "build in a dirty checkout",
			version: "dev", commit: "none", date: "unknown",
			bi: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: vcs("0123456789abcdef0123", "2026-10-18T12:00:00Z", "true"),
			},
			want: Info{"(devel)", "0123456-dirty", "2026-10-18T12:00:00Z"},
		},
		{
			name:    "build without version or VCS info",
			version: "dev", commit: "none", date: "unknown",
			bi:   &debug.BuildInfo{},
			want: Info{"(devel)", "none", "unknown"},
		},
		{
			name:    "no build info at all",
			version: "dev", commit: "none", date: "unknown",
			bi:   nil,
			want: Info{"dev", "none", "unknown"},
		},
		{
			name:    "release build: ldflags win",
			version: "0.5.0", commit: "abc1234", date: "2026-10-18T00:00:00Z",
			bi: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: vcs("0123456789abcdef0123", "2026-10-17T00:00:00Z", "true"),
			},
			want: Info{"0.5.0", "abc1234", "2026-10-18T00:00:00Z"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolve(tt.version, tt.commit, tt.date, tt.bi); got != tt.want {
				t.Errorf("resolve() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestInfoString(t *testing.T) {
	got := Info{"v0.5.0", "0123456", "2026-10-18T12:00:00Z"}.String()
	want := "v0.5.0 (commit: 0123456, built: 2026-10-18T12:00:00Z)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
