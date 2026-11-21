package version

import (
	"runtime/debug"
	"strings"
)

// When the release ldflags were not applied (for example `go install ...@latest`),
// fall back to the module version and VCS data that the Go toolchain embeds.
func init() {
	if Version != "dev" {
		return
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		applyBuildInfo(bi)
	}
}

func applyBuildInfo(bi *debug.BuildInfo) {
	if v := bi.Main.Version; v != "" && v != "(devel)" && !strings.Contains(v, "+dirty") {
		Version = strings.TrimPrefix(v, "v")
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if Commit == "dev" && len(s.Value) >= 7 {
				Commit = s.Value[:7]
			}
		case "vcs.time":
			if Date == "dev" {
				Date = s.Value
			}
		}
	}
}
