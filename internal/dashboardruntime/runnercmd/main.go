// Command awf-dashboard-runtime exposes repository-runner operations for the
// immutable Pi dashboard development runtime.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/dashboardruntime"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run is unit-tested

var resolveRuntime = dashboardruntime.Resolve
var advanceRuntime = dashboardruntime.Advance

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: awf-dashboard-runtime <path|advance> <project-root> [commit]")
		return 2
	}
	root := args[1]
	env := dashboardruntime.BuildEnvironment{Stderr: stderr}
	switch args[0] {
	case "path":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: awf-dashboard-runtime path <project-root>")
			return 2
		}
		launcher, err := resolveRuntime(root, env)
		if err != nil {
			fmt.Fprintf(stderr, "dashboard-awf-path: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, launcher.Path)
		return 0
	case "advance":
		if len(args) > 3 {
			fmt.Fprintln(stderr, "usage: awf-dashboard-runtime advance <project-root> [commit]")
			return 2
		}
		revision := "HEAD"
		if len(args) == 3 {
			revision = args[2]
		}
		result, err := advanceRuntime(root, revision, env)
		if err != nil {
			fmt.Fprintf(stderr, "dashboard-awf-advance: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "old commit: %s\nnew commit: %s\nlauncher path: %s\n", result.OldCommit, result.NewCommit, result.LauncherPath)
		return 0
	default:
		fmt.Fprintln(stderr, "usage: awf-dashboard-runtime <path|advance> <project-root> [commit]")
		return 2
	}
}
