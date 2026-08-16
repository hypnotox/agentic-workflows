// Command versioncheck exposes project-owned canonical version and schema-floor validation as an unconditional gate stage.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/project"
)

func main() { // coverage-ignore: os.Exit wrapper; run is unit-tested
	os.Exit(run(os.Stdout, os.Stderr, project.CheckVersionAuthority))
}

func run(stdout, stderr io.Writer, check func() error) int {
	if err := check(); err != nil {
		fmt.Fprintf(stderr, "versioncheck: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "versioncheck: version authority valid")
	return 0
}
