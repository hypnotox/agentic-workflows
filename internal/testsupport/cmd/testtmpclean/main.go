// Command testtmpclean owns repo-private test-temp cleanup invoked by ./x clean-test-tmp and is not part of the shipped awf CLI.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type cleanerFunc func(testsupport.CleanupMode, io.Writer) error

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, testsupport.CleanTestTemps)) }

func run(args []string, stdout, stderr io.Writer, clean cleanerFunc) int {
	mode := testsupport.CleanupStale
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "--all":
		mode = testsupport.CleanupAll
		if _, err := fmt.Fprintln(stderr, "testtmpclean: warning: --all can remove homes used by concurrent test processes"); err != nil {
			return 1
		}
	default:
		// The usage exit remains the failure signal when stderr itself is unwritable.
		_, _ = fmt.Fprintln(stderr, "usage: testtmpclean [--all]")
		return 2
	}
	if err := clean(mode, stdout); err != nil {
		// The cleanup exit remains the failure signal when stderr itself is unwritable.
		_, _ = fmt.Fprintf(stderr, "testtmpclean: %v\n", err)
		return 1
	}
	return 0
}
