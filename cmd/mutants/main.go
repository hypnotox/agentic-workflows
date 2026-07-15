// Command mutants reads a gremlins -o JSON report and prints the surviving
// (LIVED) mutants as an advisory triage list, backing the awf `./x mutants`
// command (ADR-0066). It exits non-zero when any mutant timed out: a timeout is
// scored outside efficacy and can hide a real survivor, so the whole run is
// untrustworthy. An empty report file is an empty run; a missing file is a
// caller error (./x mutants pre-creates the report via mktemp).
// Advisory only; never wired into ./x gate.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type mutation struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Line   int    `json:"line"`
}

type mutatedFile struct {
	FileName  string     `json:"file_name"`
	Mutations []mutation `json:"mutations"`
}

type report struct {
	Files []mutatedFile `json:"files"`
}

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: mutants <gremlins-json>")
		return 2
	}
	// ./x mutants pre-creates the report via mktemp, so a nonexistent path is a
	// caller error - never a clean run. Only a present-but-empty file means an
	// empty run (gremlins wrote nothing into the pre-created file).
	data, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "mutants:", err)
		return 1
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		// A pre-created temp file (mktemp) left empty by an empty run.
		fmt.Fprintln(stdout, "no survived mutants")
		return 0
	}
	var rep report
	if err := json.Unmarshal(data, &rep); err != nil {
		fmt.Fprintln(stderr, "mutants: parsing gremlins json:", err)
		return 1
	}
	var lived []string
	timedOut := 0
	for _, f := range rep.Files {
		for _, m := range f.Mutations {
			switch m.Status {
			case "TIMED OUT":
				timedOut++
			case "LIVED":
				lived = append(lived, fmt.Sprintf("%s:%d  %s", f.FileName, m.Line, m.Type))
			}
		}
	}
	if timedOut > 0 {
		fmt.Fprintf(stderr, "mutants: %d mutant(s) timed out, so the result is untrustworthy; "+
			"raise timeout-coefficient and rerun\n", timedOut)
		return 1
	}
	if len(lived) == 0 {
		fmt.Fprintln(stdout, "no survived mutants")
		return 0
	}
	sort.Strings(lived)
	fmt.Fprintln(stdout, "survived mutants (triage each: some may be equivalent):")
	for _, l := range lived {
		fmt.Fprintln(stdout, "  "+l)
	}
	return 0
}
