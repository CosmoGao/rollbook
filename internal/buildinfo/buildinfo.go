// Package buildinfo provides one build identity to every Rollbook interface.
package buildinfo

import (
	"runtime/debug"
	"strings"
)

// These values are injected by the release build. Local builds use VCS metadata.
var Version = "dev"
var Commit = ""

func Info() (string, string) {
	version, commit := Version, Commit
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && commit == "" {
				commit = s.Value
			}
		}
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return strings.TrimSpace(version), commit
}
