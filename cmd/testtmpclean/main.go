// Command testtmpclean owns repo-private test-temp cleanup invoked by ./x clean-test-tmp and is not part of the shipped awf CLI.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type cleanerFunc func(testsupport.CleanupMode, io.Writer) error

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, testsupport.CleanTestTemps)) } // coverage-ignore: process exit belongs only to the command boundary

func run(args []string, stdout, stderr io.Writer, clean cleanerFunc) int {
	mode := testsupport.CleanupStale
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "--all":
		mode = testsupport.CleanupAll
		fmt.Fprintln(stderr, "testtmpclean: warning: --all can remove homes used by concurrent test processes")
	default:
		fmt.Fprintln(stderr, "usage: testtmpclean [--all]")
		return 2
	}
	if err := clean(mode, stdout); err != nil {
		fmt.Fprintf(stderr, "testtmpclean: %v\n", err)
		return 1
	}
	return 0
}
