package main

import (
	"io"
	"runtime/debug"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runVersion renders the awf version plus display-only build provenance.
func runVersion(stdout io.Writer) error {
	info, ok := debug.ReadBuildInfo()
	return writeVersion(stdout, info, ok)
}

// writeVersion selects the value mode for the complete version presentation.
func writeVersion(stdout io.Writer, info *debug.BuildInfo, ok bool) error {
	line := versionLine(info, ok)
	if ok && formatProvenance(info) != "" {
		value, err := presentation.Literal(line)
		if err != nil { // coverage-ignore: normalized provenance and the fixed version prefix cannot produce an invalid literal
			return err
		}
		field, err := presentation.NewField("version", value)
		if err != nil { // coverage-ignore: the fixed version label and validated value cannot be invalid
			return err
		}
		document, err := presentation.NewDocument(field)
		if err != nil { // coverage-ignore: the fixed nonempty field list cannot be invalid
			return err
		}
		return presentation.Render(stdout, document)
	}
	value, err := presentation.Prose(line)
	if err != nil { // coverage-ignore: the fixed version prefix remains nonempty after normalization
		return err
	}
	field, err := presentation.NewField("version", value)
	if err != nil { // coverage-ignore: the fixed version label and validated value cannot be invalid
		return err
	}
	document, err := presentation.NewDocument(field)
	if err != nil { // coverage-ignore: the fixed nonempty field list cannot be invalid
		return err
	}
	return presentation.Render(stdout, document)
}

// versionLine renders the version value, appending display-only build
// provenance when present (ADR-0049 Decision 2). Split from runVersion so
// every branch is reachable from tests regardless of what the test binary's
// own build info carries.
func versionLine(info *debug.BuildInfo, ok bool) string {
	line := awfVersion()
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
	// touches-state: tooling/cli:single-version-authority - sole version-authority return; proof in version_test.go
	return project.Version
}

// formatProvenance renders display-only build metadata - the module version
// when it adds information beyond the embedded project version, and the short VCS revision
// (ADR-0049 Decision 2).
func formatProvenance(info *debug.BuildInfo) string {
	var parts []string
	if v := normalizeProvenance(info.Main.Version); v != "" && v != "(devel)" && v != "v"+project.Version {
		parts = append(parts, v)
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			rev := normalizeProvenance(s.Value)
			if rev == "" {
				continue
			}
			if len(rev) > 12 {
				rev = rev[:12]
			}
			parts = append(parts, "rev "+rev)
			break
		}
	}
	return strings.Join(parts, ", ")
}

func normalizeProvenance(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
