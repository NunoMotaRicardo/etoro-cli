package version

import (
	"runtime/debug"
	"testing"
)

func TestApplyBuildInfo(t *testing.T) {
	v, c, d := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = v, c, d })

	for _, tc := range []struct{ name, main, want string }{
		{"module version", "v0.2.1", "0.2.1"},
		{"local build", "(devel)", "dev"},
		{"dirty", "v0.2.1+dirty", "dev"},
		{"empty", "", "dev"},
	} {
		Version, Commit, Date = "dev", "dev", "dev"
		applyBuildInfo(&debug.BuildInfo{
			Main:     debug.Module{Version: tc.main},
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}, {Key: "vcs.time", Value: "2026-11-21T00:00:00Z"}},
		})
		if Version != tc.want {
			t.Errorf("%s: Version = %q, want %q", tc.name, Version, tc.want)
		}
		if Commit != "0123456" || Date != "2026-11-21T00:00:00Z" {
			t.Errorf("%s: commit/date = %q %q", tc.name, Commit, Date)
		}
	}
}
