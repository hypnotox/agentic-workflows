// Command testperformance validates and reports qualification evidence.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/testperformance"
)

func main() { // coverage-ignore: os.Exit wrapper; run is unit-tested
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || len(args) > 4 {
		fmt.Fprintln(stderr, "usage: testperformance <validate|report> [--machine] [record]")
		return 2
	}
	command := args[1]
	if command != "validate" && command != "report" {
		fmt.Fprintln(stderr, "usage: testperformance <validate|report> [--machine] [record]")
		return 2
	}
	machine := false
	path := "test-performance.json"
	pathSet := false
	for _, arg := range args[2:] {
		if arg == "--machine" {
			if command != "report" || machine {
				fmt.Fprintln(stderr, "usage: testperformance <validate|report> [--machine] [record]")
				return 2
			}
			machine = true
			continue
		}
		if !pathSet {
			path = arg
			pathSet = true
			continue
		}
		fmt.Fprintln(stderr, "usage: testperformance <validate|report> [--machine] [record]")
		return 2
	}
	record, err := testperformance.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, "testperformance:", err)
		return 1
	}
	if command == "validate" {
		fmt.Fprintf(stdout, "test-performance: valid %s\n", path)
		return 0
	}
	report := testperformance.BuildReport(record)
	if machine {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil { // coverage-ignore: validated Report contains only JSON-native finite values
			fmt.Fprintln(stderr, "testperformance:", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
	} else {
		testperformance.WriteHuman(stdout, report)
	}
	if testperformance.HasComponentRegressions(report) {
		fmt.Fprintln(stderr, "testperformance: component regression blocks qualification")
		return 1
	}
	return 0
}
