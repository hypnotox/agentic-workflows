// Command covercheck fails when a Go coverprofile shows less than 100% statement
// coverage over blocks not marked with a coverage-ignore directive. It backs the
// awf coverage gate (ADR-0012).
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/coverage"
)

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) >= 2 && args[1] == "--emit-filtered" {
		if len(args) < 3 {
			fmt.Fprintln(stderr, "usage: covercheck --emit-filtered <coverprofile>")
			return 2
		}
		filtered, err := coverage.FilterProfile(args[2])
		if err != nil {
			fmt.Fprintln(stderr, "covercheck:", err)
			return 1
		}
		fmt.Fprint(stdout, filtered)
		return 0
	}
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: covercheck <coverprofile>")
		return 2
	}
	rep, err := coverage.CheckProfile(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "covercheck:", err)
		return 1
	}
	fmt.Fprintf(stdout, "coverage: %.1f%% (%d/%d statements)\n", rep.Percent(), rep.Covered, rep.Total)
	if !rep.OK() {
		fmt.Fprintf(stderr, "covercheck: coverage below 100%% — %d uncovered statement(s)\n",
			rep.Total-rep.Covered)
		return 1
	}
	return 0
}
