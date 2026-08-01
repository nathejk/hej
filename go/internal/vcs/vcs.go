// Package vcs reports the binary's build version. It prefers a value injected
// at build time via -ldflags "-X nathejk.dk/internal/vcs.version=..." and
// otherwise derives one from the embedded VCS build info.
package vcs

import "runtime/debug"

// version may be set at build/link time.
var version string

// Version returns a short build identifier, or "unknown" if none is available.
func Version() string {
	if version != "" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	var revision string
	var modified bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}

	if revision == "" {
		return "unknown"
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if modified {
		revision += "-dirty"
	}
	return revision
}
