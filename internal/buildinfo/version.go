// Package buildinfo exposes immutable build metadata to all Navo binaries.
package buildinfo

import "strings"

// Version is overridden by release builds with:
//
//	-X navo/internal/buildinfo.Version=<VERSION>
//
// Development builds deliberately remain distinguishable from releases.
var Version = "dev"

// NormalizedVersion returns a stable non-empty version for diagnostics.
func NormalizedVersion() string {
	version := strings.TrimSpace(Version)
	if version == "" {
		return "dev"
	}
	return version
}
