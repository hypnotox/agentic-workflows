package main

import (
	"fmt"
	"io"
	"runtime/debug"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// version is the release version stamped into the binary at build time via
// `-ldflags "-X main.version=<tag>"` (see .goreleaser.yaml). It is empty for
// `go build`, `go run`, and test builds.
var version string

// runVersion prints the awf version.
func runVersion(stdout io.Writer) {
	fmt.Fprintf(stdout, "awf %s\n", awfVersion())
}

// awfVersion returns the awf version, preferring the ldflags-injected release
// version, then the module version stamped by `go install module@version`, and
// finally the embedded project.Version for dev and test builds.
func awfVersion() string {
	// invariant: version-ldflags-precedence
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" { // coverage-ignore: Main.Version is set only by `go install module@version`, never for test or dev builds
		return info.Main.Version
	}
	return project.Version
}
