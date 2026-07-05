// Command deadcodecheck fails when `deadcode -json` (read from stdin) reports any
// unreachable function outside the internal/testsupport/ tree. It backs the awf
// dead-code gate (ADR-0063).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ignorePrefix is the one structural exemption: internal/testsupport is the
// ADR-0044 shared cross-package test-helper package, prod-unreachable by design.
const ignorePrefix = "internal/testsupport/"

type deadFunc struct {
	Name     string `json:"Name"`
	Position struct {
		File string `json:"File"`
		Line int    `json:"Line"`
	} `json:"Position"`
}

type deadPackage struct {
	Path  string     `json:"Path"`
	Funcs []deadFunc `json:"Funcs"`
}

func main() { os.Exit(run(os.Stdin, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintln(stderr, "deadcodecheck:", err)
		return 1
	}
	var pkgs []deadPackage
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &pkgs); err != nil {
			fmt.Fprintln(stderr, "deadcodecheck: parsing deadcode -json:", err)
			return 1
		}
	}
	var offenders []string
	for _, pkg := range pkgs {
		for _, fn := range pkg.Funcs {
			if strings.HasPrefix(fn.Position.File, ignorePrefix) {
				continue
			}
			offenders = append(offenders,
				fmt.Sprintf("%s:%d: unreachable func: %s", fn.Position.File, fn.Position.Line, fn.Name))
		}
	}
	if len(offenders) == 0 {
		fmt.Fprintln(stdout, "deadcodecheck: no production dead code")
		return 0
	}
	sort.Strings(offenders)
	fmt.Fprintf(stderr, "deadcodecheck: %d unreachable production func(s):\n", len(offenders))
	for _, o := range offenders {
		fmt.Fprintln(stderr, "  "+o)
	}
	return 1
}
