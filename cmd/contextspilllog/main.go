//go:build linux || darwin

// Command contextspilllog is the repository runner's private context-spill
// observability helper. It is not part of the public awf CLI.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/contextspill"
)

const maxNoticeBytes = 64 * 1024

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run is unit-tested

func run(args []string, stdout, stderr io.Writer) int {
	_ = stdout
	if len(args) == 3 && args[0] == "--check-log" && args[1] == "--root" && args[2] != "" {
		nonempty, err := contextspill.HasSafeLog(args[2])
		if err != nil {
			fmt.Fprintln(stderr, "contextspilllog:", err)
			return 1
		}
		if nonempty {
			fmt.Fprintln(stderr, "check: advisory: context spills were observed; resolve or promote the issue, then remove .awf/local/context-spills.log")
		}
		return 0
	}
	if len(args) < 5 || args[0] != "--root" || args[1] == "" || args[2] != "--notice-file" || args[3] == "" || args[4] != "--" {
		fmt.Fprintln(stderr, "usage: contextspilllog --check-log --root <root> | contextspilllog --root <root> --notice-file <capture> -- <invocation...>")
		return 2
	}
	file, err := os.Open(args[3])
	if err != nil {
		fmt.Fprintln(stderr, "contextspilllog:", err)
		return 1
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxNoticeBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		fmt.Fprintln(stderr, "contextspilllog:", readErr)
		return 1
	}
	if closeErr != nil { // coverage-ignore: closing a successfully read regular capture cannot fail
		fmt.Fprintln(stderr, "contextspilllog:", closeErr)
		return 1
	}
	if len(data) > maxNoticeBytes {
		fmt.Fprintln(stderr, "contextspilllog: notice capture exceeds size limit")
		return 1
	}
	notice, recognized, err := contextspill.ParseNotice(data)
	if err != nil {
		fmt.Fprintln(stderr, "contextspilllog:", err)
		return 1
	}
	if !recognized {
		return 0
	}
	if err := contextspill.Log(args[1], notice, args[5:]); err != nil {
		fmt.Fprintln(stderr, "contextspilllog:", err)
		return 1
	}
	return 0
}
