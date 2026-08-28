// Command covercheck enforces the repository's raw-identity coverage policy and
// reports raw and filtered statement coverage. It also generates the canonical
// baseline; its legacy standalone form keeps the filtered 100 percent check.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/coverage"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
)

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) >= 2 {
		switch args[1] {
		case "--emit-filtered":
			if len(args) != 3 {
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
		case "--merge":
			if len(args) < 4 {
				fmt.Fprintln(stderr, "usage: covercheck --merge <coverprofile> <coverprofile> [...]")
				return 2
			}
			merged, err := coverage.MergeProfiles(args[2:])
			if err != nil {
				fmt.Fprintln(stderr, "covercheck:", err)
				return 1
			}
			if _, err := fmt.Fprint(stdout, merged); err != nil {
				fmt.Fprintln(stderr, "covercheck: write merged profile:", err)
				return 1
			}
			return 0
		case "--policy":
			if len(args) != 4 {
				fmt.Fprintln(stderr, "usage: covercheck --policy <coverprofile> <baseline>")
				return 2
			}
			return evaluatePolicy(args[2], args[3], stdout, stderr)
		case "--generate-policy":
			if len(args) != 5 {
				fmt.Fprintln(stderr, "usage: covercheck --generate-policy <coverprofile> <baseline> <review>")
				return 2
			}
			return generatePolicy(args[2], args[3], args[4], stdout, stderr)
		}
	}
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: covercheck <coverprofile>")
		return 2
	}
	analysis, err := coverage.AnalyzeProfile(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "covercheck:", err)
		return 1
	}
	printReports(stdout, analysis)
	if !analysis.Filtered.OK() {
		fmt.Fprintf(stderr, "covercheck: coverage below 100%% (%d uncovered statement(s))\n",
			analysis.Filtered.Total-analysis.Filtered.Covered)
		return 1
	}
	return 0
}

func evaluatePolicy(profilePath, baselinePath string, stdout, stderr io.Writer) int {
	analysis, err := coverage.AnalyzeProfile(profilePath)
	if err != nil {
		fmt.Fprintln(stderr, "covercheck:", err)
		return 1
	}
	baseline, err := coverage.LoadBaseline(baselinePath)
	if err != nil {
		fmt.Fprintln(stderr, "covercheck:", err)
		return 1
	}
	printReports(stdout, analysis)
	findings := coverage.Evaluate(analysis, baseline)
	for _, finding := range findings {
		fmt.Fprintf(stderr, "covercheck: %s: %s\n", finding.Code, finding.Message)
	}
	if len(findings) != 0 {
		return 1
	}
	return 0
}

func generatePolicy(profilePath, baselinePath, reviewPath string, stdout, stderr io.Writer) int {
	analysis, err := coverage.AnalyzeProfile(profilePath)
	if err != nil {
		fmt.Fprintln(stderr, "covercheck:", err)
		return 1
	}
	review, err := coverage.LoadReview(reviewPath)
	if err != nil {
		fmt.Fprintln(stderr, "covercheck:", err)
		return 1
	}
	var previous *coverage.Baseline
	if _, statErr := os.Stat(baselinePath); statErr == nil {
		loaded, loadErr := coverage.LoadBaselineForRegeneration(baselinePath)
		if loadErr != nil {
			fmt.Fprintln(stderr, "covercheck:", loadErr)
			return 1
		}
		previous = &loaded
	} else if !os.IsNotExist(statErr) {
		fmt.Fprintln(stderr, "covercheck: inspect baseline:", statErr)
		return 1
	}
	baseline, err := coverage.Regenerate(analysis, previous, review)
	if err != nil {
		fmt.Fprintln(stderr, "covercheck:", err)
		return 1
	}
	canonical, err := coverage.CanonicalBaseline(baseline)
	if err != nil { // coverage-ignore: Regenerate returned the same already-validated typed baseline
		fmt.Fprintln(stderr, "covercheck:", err)
		return 1
	}
	if err := writeComplete(baselinePath, canonical); err != nil {
		fmt.Fprintln(stderr, "covercheck:", err)
		return 1
	}
	if _, err := coverage.LoadBaseline(baselinePath); err != nil { // coverage-ignore: complete canonical bytes were just atomically replaced and verified in memory
		fmt.Fprintln(stderr, "covercheck: verify generated baseline:", err)
		return 1
	}
	fmt.Fprintf(stdout, "coverage policy: wrote %s\n", baselinePath)
	printReports(stdout, analysis)
	return 0
}

func writeComplete(path string, contents []byte) (returnErr error) {
	handle, err := filesystem.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open baseline directory: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, handle.Close()) }()
	if err := handle.Replace(filepath.Base(path), contents, 0o644); err != nil {
		return fmt.Errorf("publish baseline: %w", err)
	}
	return nil
}

func printReports(out io.Writer, analysis coverage.Analysis) {
	fmt.Fprintf(out, "raw coverage: %.1f%% (%d/%d statements)\n", analysis.Raw.Percent(), analysis.Raw.Covered, analysis.Raw.Total)
	fmt.Fprintf(out, "filtered coverage: %.1f%% (%d/%d statements)\n", analysis.Filtered.Percent(), analysis.Filtered.Covered, analysis.Filtered.Total)
}
