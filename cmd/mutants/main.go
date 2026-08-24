// Command mutants reads a gremlins -o JSON report and prints the surviving
// (LIVED) mutants as an advisory triage list, backing the awf `./x mutants`
// command (ADR-0066). The validate mode is a separate fail-closed interface
// for the targeted covercheck mutation caller. Advisory only; never wired into
// ./x gate.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	coveragepolicy "github.com/hypnotox/agentic-workflows/internal/coverage"
)

const maxRunSeconds = 900
const maxRenewalSeconds = 1500
const repositoryModule = "github.com/hypnotox/agentic-workflows"

type mutation struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type mutatedFile struct {
	FileName  string     `json:"file_name"`
	Mutations []mutation `json:"mutations"`
}

// report is Gremlins v0.6.0's JSON output shape.
type report struct {
	GoModule          string          `json:"go_module"`
	Files             []mutatedFile   `json:"files"`
	TestEfficacy      float64         `json:"test_efficacy"`
	MutationsCoverage float64         `json:"mutations_coverage"`
	MutantsTotal      int             `json:"mutants_total"`
	MutantsKilled     int             `json:"mutants_killed"`
	MutantsLived      int             `json:"mutants_lived"`
	MutantsNotViable  int             `json:"mutants_not_viable"`
	MutantsNotCovered int             `json:"mutants_not_covered"`
	ElapsedTime       float64         `json:"elapsed_time"`
	MutatorStatistics json.RawMessage `json:"mutator_statistics"`
}

// mutationIdentity is stable across dry discovery and actual execution.
type mutationIdentity struct {
	File    string
	Line    int
	Column  int
	Mutator string
}

type trustedReport struct {
	statuses map[mutationIdentity]string
}

type timedReport struct {
	report         trustedReport
	elapsedSeconds float64
}

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) >= 2 && args[1] == "validate" {
		return runValidate(args, stdout, stderr)
	}
	if len(args) >= 2 && args[1] == "operators" {
		return runOperators(args, stdout, stderr)
	}
	if len(args) >= 2 && args[1] == "renewal" {
		return runRenewal(args, stdout, stderr)
	}
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: mutants <gremlins-json> | mutants validate <dry-json> <actual-json> <coverage-baseline-json|-> <target-root> | mutants renewal <coverage-baseline-json|-> <target-root> <seconds-1> <dry-1> <actual-1> <seconds-2> <dry-2> <actual-2> <seconds-3> <dry-3> <actual-3> | mutants operators")
		return 2
	}
	// ./x mutants pre-creates the report via mktemp, so a nonexistent path is a
	// caller error. Only a present-but-empty file means an empty advisory run.
	data, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "mutants:", err)
		return 1
	}
	if len(strings.TrimSpace(string(data))) == 0 {
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
		fmt.Fprintf(stderr, "mutants: %d mutant(s) timed out, so the result is untrustworthy; raise timeout-coefficient and rerun\n", timedOut)
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

func runValidate(args []string, stdout, stderr io.Writer) int {
	if len(args) != 6 {
		fmt.Fprintln(stderr, "usage: mutants validate <dry-json> <actual-json> <coverage-baseline-json|-> <target-root>")
		return 2
	}
	dry, err := os.ReadFile(args[2])
	if err != nil {
		fmt.Fprintln(stderr, "mutants: reading dry report:", err)
		return 1
	}
	actual, err := os.ReadFile(args[3])
	if err != nil {
		fmt.Fprintln(stderr, "mutants: reading actual report:", err)
		return 1
	}
	equivalents, err := equivalentMutants(args[4])
	if err != nil {
		fmt.Fprintln(stderr, "mutants: parsing coverage baseline:", err)
		return 1
	}
	trusted, err := validateDryActual(dry, actual, equivalents, args[5])
	if err != nil {
		fmt.Fprintln(stderr, "mutants: untrusted report:", err)
		return 1
	}
	fmt.Fprintf(stdout, "trusted mutation reports: %d identities; status-sha256=%s\n", len(trusted.statuses), statusSetDigest(trusted.statuses))
	return 0
}

func runRenewal(args []string, stdout, stderr io.Writer) int {
	if len(args) != 13 {
		fmt.Fprintln(stderr, "usage: mutants renewal <coverage-baseline-json|-> <target-root> <seconds-1> <dry-1> <actual-1> <seconds-2> <dry-2> <actual-2> <seconds-3> <dry-3> <actual-3>")
		return 2
	}
	equivalents, err := equivalentMutants(args[2])
	if err != nil {
		fmt.Fprintln(stderr, "mutants: parsing coverage baseline:", err)
		return 1
	}
	runs := make([]timedReport, 0, 3)
	for i := range 3 {
		elapsed, err := strconv.ParseFloat(args[4+i*3], 64)
		if err != nil || math.IsNaN(elapsed) || math.IsInf(elapsed, 0) {
			fmt.Fprintf(stderr, "mutants: parsing run %d elapsed seconds: finite number required\n", i+1)
			return 1
		}
		dry, err := os.ReadFile(args[5+i*3])
		if err != nil {
			fmt.Fprintf(stderr, "mutants: reading run %d dry report: %v\n", i+1, err)
			return 1
		}
		actual, err := os.ReadFile(args[6+i*3])
		if err != nil {
			fmt.Fprintf(stderr, "mutants: reading run %d actual report: %v\n", i+1, err)
			return 1
		}
		report, err := validateDryActual(dry, actual, equivalents, args[3])
		if err != nil {
			fmt.Fprintf(stderr, "mutants: untrusted run %d report pair: %v\n", i+1, err)
			return 1
		}
		runs = append(runs, timedReport{report: report, elapsedSeconds: elapsed})
	}
	if err := validateRenewal(runs); err != nil {
		fmt.Fprintln(stderr, "mutants: invalid renewal:", err)
		return 1
	}
	fmt.Fprintf(stdout, "trusted mutation renewal: 3 runs; status-sha256=%s\n", statusSetDigest(runs[0].report.statuses))
	return 0
}

func runOperators(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: mutants operators")
		return 2
	}
	operators := mutationOperatorValues()
	names := make([]string, 0, len(operators))
	for name := range operators {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(stdout, "--%s=%t\n", name, operators[name])
	}
	return 0
}

func mutationOperatorValues() map[string]bool {
	return map[string]bool{
		"arithmetic-base": true, "conditionals-boundary": true, "conditionals-negation": true,
		"increment-decrement": true, "invert-negatives": true, "invert-assignments": false,
		"invert-bitwise": false, "invert-bwassign": false, "invert-logical": false,
		"invert-loopctrl": false, "remove-self-assignments": false,
	}
}

func validateDryActual(dryData, actualData []byte, equivalents map[mutationIdentity]struct{}, targetRoot string) (trustedReport, error) {
	dry, err := validateReport(dryData, true, nil, targetRoot)
	if err != nil {
		return trustedReport{}, fmt.Errorf("dry report: %w", err)
	}
	actual, err := validateActual(actualData, equivalents, targetRoot)
	if err != nil {
		return trustedReport{}, fmt.Errorf("actual report: %w", err)
	}
	if !sameIdentitySet(dry.statuses, actual.statuses) {
		return trustedReport{}, errors.New("dry and actual mutant identities differ")
	}
	return actual, nil
}

func validateActual(data []byte, equivalents map[mutationIdentity]struct{}, targetRoot string) (trustedReport, error) {
	return validateReport(data, false, equivalents, targetRoot)
}

func validateReport(data []byte, dry bool, equivalents map[mutationIdentity]struct{}, targetRoot string) (trustedReport, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return trustedReport{}, errors.New("empty report")
	}
	var raw map[string]json.RawMessage
	var rep report
	if err := json.Unmarshal(data, &raw); err != nil {
		return trustedReport{}, fmt.Errorf("malformed JSON: %w", err)
	}
	if err := json.Unmarshal(data, &rep); err != nil {
		return trustedReport{}, fmt.Errorf("malformed Gremlins report: %w", err)
	}
	for _, field := range []string{"go_module", "files", "test_efficacy", "mutations_coverage", "mutants_total", "mutants_killed", "mutants_lived", "mutants_not_viable", "mutants_not_covered", "elapsed_time", "mutator_statistics"} {
		value, ok := raw[field]
		if !ok || string(value) == "null" {
			return trustedReport{}, fmt.Errorf("incomplete report: missing %s", field)
		}
	}
	var mutatorStatistics map[string]int
	if err := json.Unmarshal(rep.MutatorStatistics, &mutatorStatistics); err != nil {
		return trustedReport{}, fmt.Errorf("incomplete mutator statistics: %w", err)
	}
	if rep.GoModule != repositoryModule || len(rep.Files) == 0 || math.IsNaN(rep.ElapsedTime) || math.IsInf(rep.ElapsedTime, 0) || rep.ElapsedTime < 0 || rep.ElapsedTime > maxRunSeconds {
		return trustedReport{}, errors.New("incomplete or over-budget report")
	}
	cleanTarget := path.Clean(targetRoot)
	if !moduleRelative(cleanTarget) || cleanTarget == "." {
		return trustedReport{}, fmt.Errorf("invalid target root %q", targetRoot)
	}
	result := trustedReport{statuses: make(map[mutationIdentity]string)}
	killed, lived, notViable, notCovered := 0, 0, 0, 0
	for _, file := range rep.Files {
		fileName, err := mutationFileIdentity(file.FileName, cleanTarget)
		if err != nil || len(file.Mutations) == 0 {
			return trustedReport{}, fmt.Errorf("incomplete file record: %w", err)
		}
		for _, m := range file.Mutations {
			identity := mutationIdentity{File: fileName, Line: m.Line, Column: m.Column, Mutator: m.Type}
			if identity.Line <= 0 || identity.Column <= 0 || identity.Mutator == "" {
				return trustedReport{}, errors.New("incomplete mutation identity")
			}
			if _, exists := result.statuses[identity]; exists {
				return trustedReport{}, errors.New("duplicate mutation identity")
			}
			if dry && m.Status != "RUNNABLE" {
				return trustedReport{}, fmt.Errorf("expected RUNNABLE, got %s", m.Status)
			}
			switch m.Status {
			case "RUNNABLE":
				if !dry {
					return trustedReport{}, errors.New("unexpected RUNNABLE actual mutant")
				}
			case "KILLED":
				killed++
			case "LIVED":
				lived++
				if _, ok := equivalents[identity]; !ok {
					return trustedReport{}, errors.New("unreviewed LIVED mutant")
				}
			case "NOT VIABLE":
				return trustedReport{}, errors.New("NOT VIABLE mutant")
			case "NOT COVERED":
				return trustedReport{}, errors.New("NOT COVERED mutant")
			case "SKIPPED", "TIMED OUT":
				return trustedReport{}, fmt.Errorf("%s mutant", m.Status)
			default:
				return trustedReport{}, fmt.Errorf("unknown mutation status %q", m.Status)
			}
			result.statuses[identity] = m.Status
		}
	}
	wantTotal := killed + lived + notViable
	if dry {
		wantTotal = 0
	}
	if rep.MutantsTotal != wantTotal || rep.MutantsKilled != killed || rep.MutantsLived != lived || rep.MutantsNotViable != notViable || rep.MutantsNotCovered != notCovered {
		return trustedReport{}, errors.New("inconsistent mutation totals")
	}
	mutatorCount := 0
	for _, count := range mutatorStatistics {
		if count < 0 {
			return trustedReport{}, errors.New("negative mutator statistic")
		}
		mutatorCount += count
	}
	if mutatorCount != len(result.statuses) {
		return trustedReport{}, errors.New("inconsistent mutator statistics")
	}
	return result, nil
}

func mutationFileIdentity(file, targetRoot string) (string, error) {
	clean := path.Clean(strings.ReplaceAll(file, "\\", "/"))
	if !moduleRelative(clean) || clean == "." {
		return "", fmt.Errorf("invalid mutation file %q", file)
	}
	if !strings.Contains(clean, "/") {
		return path.Join(targetRoot, clean), nil
	}
	if clean != targetRoot && !strings.HasPrefix(clean, targetRoot+"/") {
		return "", fmt.Errorf("mutation file %q is outside %s", file, targetRoot)
	}
	return clean, nil
}

func moduleRelative(file string) bool {
	return file != "." && !strings.HasPrefix(file, "/") && !strings.HasPrefix(file, "../") && path.Clean(file) == file
}

func sameIdentitySet(left, right map[mutationIdentity]string) bool {
	if len(left) != len(right) {
		return false
	}
	for identity := range left {
		if _, ok := right[identity]; !ok {
			return false
		}
	}
	return true
}

func validateRenewal(runs []timedReport) error {
	if len(runs) != 3 {
		return errors.New("renewal requires exactly three runs")
	}
	total := 0.0
	for i, run := range runs {
		if math.IsNaN(run.elapsedSeconds) || math.IsInf(run.elapsedSeconds, 0) || run.elapsedSeconds < 0 || run.elapsedSeconds > maxRunSeconds {
			return fmt.Errorf("run %d exceeds %d seconds", i+1, maxRunSeconds)
		}
		total += run.elapsedSeconds
		if i > 0 && !sameStatusSet(runs[0].report.statuses, run.report.statuses) {
			return fmt.Errorf("run %d has different trusted statuses", i+1)
		}
	}
	if total > maxRenewalSeconds {
		return fmt.Errorf("renewal exceeds %d seconds", maxRenewalSeconds)
	}
	return nil
}

func statusSetDigest(statuses map[mutationIdentity]string) string {
	lines := make([]string, 0, len(statuses))
	for identity, status := range statuses {
		lines = append(lines, fmt.Sprintf("%s:%d:%d:%s=%s", identity.File, identity.Line, identity.Column, identity.Mutator, status))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
	return hex.EncodeToString(sum[:])
}

func sameStatusSet(left, right map[mutationIdentity]string) bool {
	if len(left) != len(right) {
		return false
	}
	for identity, status := range left {
		if right[identity] != status {
			return false
		}
	}
	return true
}

func equivalentMutants(baselinePath string) (map[mutationIdentity]struct{}, error) {
	if baselinePath == "-" {
		return map[mutationIdentity]struct{}{}, nil
	}
	baseline, err := coveragepolicy.LoadBaseline(baselinePath)
	if err != nil {
		return nil, err
	}
	if baseline.ModulePath != repositoryModule {
		return nil, fmt.Errorf("baseline module %q does not match %q", baseline.ModulePath, repositoryModule)
	}
	result := make(map[mutationIdentity]struct{}, len(baseline.EquivalentMutants))
	for _, mutant := range baseline.EquivalentMutants {
		identity := mutationIdentity{File: mutant.File, Line: mutant.Line, Column: mutant.Column, Mutator: mutant.Mutator}
		result[identity] = struct{}{}
	}
	return result, nil
}
