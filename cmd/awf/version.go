package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runVersion prints the awf version plus display-only build provenance.
func runVersion(stdout io.Writer) {
	info, ok := debug.ReadBuildInfo()
	fmt.Fprintln(stdout, versionLine(info, ok))
}

// versionLine renders the "awf <version>" line, appending display-only build
// provenance when present (ADR-0049 Decision 2). Split from runVersion so
// every branch is reachable from tests regardless of what the test binary's
// own build info carries.
func versionLine(info *debug.BuildInfo, ok bool) string {
	line := "awf " + awfVersion()
	if !ok {
		return line
	}
	if p := formatProvenance(info); p != "" {
		line += " (" + p + ")"
	}
	return line
}

// awfVersion returns the awf version. project.Version is the single version
// authority (ADR-0049): no ldflags var or module build info feeds version
// gating, lock stamping, or bootstrap pinning.
func awfVersion() string {
	// touches-invariant: single-version-authority — sole version-authority return; proof in version_test.go
	return project.Version
}

// formatProvenance renders display-only build metadata — the module version
// when it adds information beyond the const, and the short VCS revision
// (ADR-0049 Decision 2).
func formatProvenance(info *debug.BuildInfo) string {
	var parts []string
	if v := info.Main.Version; v != "" && v != "(devel)" && v != "v"+project.Version {
		parts = append(parts, v)
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			rev := s.Value
			if len(rev) > 12 {
				rev = rev[:12]
			}
			parts = append(parts, "rev "+rev)
			break
		}
	}
	return strings.Join(parts, ", ")
}
