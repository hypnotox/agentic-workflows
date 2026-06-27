package main

import (
	"fmt"
	"io"
	"runtime/debug"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

// runVersion prints the awf version.
func runVersion(stdout io.Writer) {
	fmt.Fprintf(stdout, "awf %s\n", awfVersion())
}

// awfVersion returns the module version stamped by `go install module@version`,
// falling back to the embedded schema-era version for dev and test builds.
func awfVersion() string {
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" { // coverage-ignore: Main.Version is set only by `go install module@version`, never for test or dev builds
		return info.Main.Version
	}
	return project.Version
}
