// Package coverage parses a Go coverprofile and reports statement coverage over
// blocks not marked with a coverage-ignore directive. It backs the awf coverage
// gate (ADR-0012): a directive of the form "<slashes> coverage-ignore: <reason>"
// drops its block from both the covered and total counts; a directive with no
// non-empty reason is an error.
package coverage

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// marker is the ignore directive in its comment form. It is assembled by
// concatenation so this source line does not itself contain the literal
// directive - otherwise the scanner, when reading this very file out of a
// coverprofile, would treat this line as a reasonless directive and error.
const marker = "//" + " coverage-ignore"

// Report is the result of checking a coverprofile.
type Report struct {
	Covered int // statements in non-ignored blocks executed at least once
	Total   int // statements in non-ignored blocks
}

// Percent returns the covered percentage; an empty Report is 100.
func (r Report) Percent() float64 {
	if r.Total == 0 {
		return 100
	}
	return 100 * float64(r.Covered) / float64(r.Total)
}

// OK reports whether every non-ignored statement is covered.
func (r Report) OK() bool { return r.Covered == r.Total }

var getwd = os.Getwd

// hasGoMod reports whether dir contains a go.mod. It is a package var so the
// module-root walk's "reached the filesystem root without finding a go.mod"
// branch is testable hermetically: the directory walk itself is pure string
// manipulation (filepath.Dir), so stubbing this is the only thing needed to
// drive the walk to the root regardless of what actually sits above the test's
// working directory (e.g. a stray go.mod under /tmp).
var hasGoMod = func(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

// mergeBlocks collapses the per-test-binary duplication of a `-coverpkg` profile:
// blocks sharing a file+span identity merge into one, OR-ing the counts (mode: set)
// exactly as `go tool cover` does, so the denominator is not inflated.
func mergeBlocks(blocks []block) []block {
	merged := map[string]block{}
	for _, b := range blocks {
		k := fmt.Sprintf("%s:%s:%d", b.file, b.span, b.numStmt)
		if prev, ok := merged[k]; ok {
			if b.count > prev.count {
				prev.count = b.count
				merged[k] = prev
			}
			continue
		}
		merged[k] = b
	}
	uniq := make([]block, 0, len(merged))
	for _, b := range merged {
		uniq = append(uniq, b)
	}
	return uniq
}

// MergeProfiles parses nonempty set-mode shard profiles and returns their
// canonical union. Execution counts are OR-merged and duplicate emissions
// collapse to one line; the policy consumer validates the complete union digest.
func MergeProfiles(profilePaths []string) (string, error) {
	if len(profilePaths) < 2 {
		return "", errors.New("coverage: merge requires at least two profiles")
	}

	var mode string
	merged := map[string]block{}
	for index, profilePath := range profilePaths {
		profileMode, blocks, err := parseCoverageProfile(profilePath)
		if err != nil {
			return "", err
		}
		if len(blocks) == 0 {
			return "", fmt.Errorf("%s: coverage: profile contains no blocks", profilePath)
		}
		if index == 0 {
			mode = profileMode
		} else if profileMode != mode {
			return "", fmt.Errorf("%s: coverage: mixed profile modes: %q and %q", profilePath, mode, profileMode)
		}

		shard := make(map[string]block, len(blocks))
		for _, current := range blocks {
			if err := validateMergeBlock(current, profileMode == "set"); err != nil {
				return "", fmt.Errorf("%s:%d: %w", profilePath, current.profileLine, err)
			}
			key := blockKey(current)
			if previous, ok := shard[key]; ok {
				if previous.numStmt != current.numStmt {
					return "", fmt.Errorf("%s:%d: coverage: conflicting statement count for %q: %d and %d", profilePath, current.profileLine, key, previous.numStmt, current.numStmt)
				}
				previous.count = setCount(previous.count, current.count)
				shard[key] = previous
				continue
			}
			shard[key] = current
		}

		for key, current := range shard {
			if previous, ok := merged[key]; ok {
				if previous.numStmt != current.numStmt {
					return "", fmt.Errorf("%s:%d: coverage: conflicting statement count for %q: %d and %d", profilePath, current.profileLine, key, previous.numStmt, current.numStmt)
				}
				previous.count = setCount(previous.count, current.count)
				merged[key] = previous
				continue
			}
			merged[key] = current
		}
	}
	if mode != "set" {
		return "", fmt.Errorf("coverage: unsupported merge mode %q; want %q", mode, "set")
	}

	canonical := make([]block, 0, len(merged))
	for _, current := range merged {
		canonical = append(canonical, current)
	}
	slices.SortFunc(canonical, compareBlock)
	var output strings.Builder
	output.WriteString("mode: set\n")
	for _, current := range canonical {
		fmt.Fprintf(&output, "%s:%s %d %d\n", current.file, current.span, current.numStmt, current.count)
	}
	return output.String(), nil
}

func blockKey(current block) string { return current.file + ":" + current.span }

func setCount(left, right int) int {
	if left > 0 || right > 0 {
		return 1
	}
	return 0
}

func validateMergeBlock(current block, setMode bool) error {
	if current.file == "" || path.IsAbs(current.file) || path.Clean(current.file) != current.file ||
		strings.ContainsAny(current.file, `\:`) || !strings.Contains(current.file, "/") ||
		strings.IndexFunc(current.file, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return fmt.Errorf("coverage: noncanonical profile path %q", current.file)
	}
	canonicalSpan := fmt.Sprintf("%d.%d,%d.%d", current.startLine, current.startColumn, current.endLine, current.endColumn)
	if current.span != canonicalSpan {
		return fmt.Errorf("coverage: noncanonical profile span %q; want %q", current.span, canonicalSpan)
	}
	if current.endLine < current.startLine ||
		(current.endLine == current.startLine && current.endColumn < current.startColumn) {
		return fmt.Errorf("coverage: reversed profile span %q", current.span)
	}
	if current.numStmt < 0 {
		return fmt.Errorf("coverage: negative statement count for %q", blockKey(current))
	}
	if setMode && current.count != 0 && current.count != 1 {
		return fmt.Errorf("coverage: set-mode execution count for %q must be 0 or 1", blockKey(current))
	}
	return nil
}

// FilterProfile resolves the module root from the working directory and returns
// the profile at profilePath with its coverage-ignored blocks removed (see Filter).
func FilterProfile(profilePath string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	modPath, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	return Filter(profilePath, root, modPath)
}

// Filter returns a coverprofile containing exactly the blocks of the profile at
// profilePath that are NOT dropped by a coverage-ignore directive - the same
// blocks the ADR-0012 gate holds accountable, sharing ignoredLines verbatim so the
// two can never disagree on what "ignored" means (ADR-0065's covered report). The
// output carries a "mode: set" header (the mode parseProfile discards; awf's gate
// profile is always set) and merged-unique blocks in deterministic (file, startLine,
// span) order, so it round-trips as valid Codecov input.
func Filter(profilePath, srcRoot, modPath string) (string, error) {
	blocks, err := parseProfile(profilePath)
	if err != nil {
		return "", err
	}
	uniq := mergeBlocks(blocks)
	ignored, err := ignoredLines(uniq, srcRoot, modPath)
	if err != nil {
		return "", err
	}
	kept := make([]block, 0, len(uniq))
	for _, b := range uniq {
		if ignored[b.file][b.startLine] {
			continue
		}
		kept = append(kept, b)
	}
	slices.SortFunc(kept, func(a, b block) int {
		if a.file != b.file {
			return strings.Compare(a.file, b.file)
		}
		if a.startLine != b.startLine {
			return a.startLine - b.startLine
		}
		return strings.Compare(a.span, b.span)
	})
	var sb strings.Builder
	sb.WriteString("mode: set\n")
	for _, b := range kept {
		fmt.Fprintf(&sb, "%s:%s %d %d\n", b.file, b.span, b.numStmt, b.count)
	}
	return sb.String(), nil
}

// block is one parsed coverprofile line.
type block struct {
	file        string // module-qualified source path, e.g. mod/pkg/file.go
	span        string // raw "startLine.col,endLine.col" - block identity within a file
	startLine   int
	startColumn int
	endLine     int
	endColumn   int
	numStmt     int
	count       int
	profileLine int
}

func parseProfile(profilePath string) ([]block, error) {
	_, blocks, err := parseCoverageProfile(profilePath)
	return blocks, err
}

func parseCoverageProfile(profilePath string) (string, []block, error) {
	f, err := os.Open(profilePath)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = f.Close() }()

	var mode string
	var blocks []block
	sc := bufio.NewScanner(f)
	lineNumber := 0
	for sc.Scan() {
		lineNumber++
		line := sc.Text()
		if lineNumber == 1 {
			var ok bool
			mode, ok = strings.CutPrefix(line, "mode: ")
			if !ok || (mode != "set" && mode != "count" && mode != "atomic") {
				return "", nil, fmt.Errorf("%s:1: coverage: malformed profile header %q", profilePath, line)
			}
			continue
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "mode:") {
			return "", nil, fmt.Errorf("%s:%d: coverage: unexpected profile header %q", profilePath, lineNumber, line)
		}
		current, err := parseLine(line)
		if err != nil {
			return "", nil, fmt.Errorf("%s:%d: %w", profilePath, lineNumber, err)
		}
		current.profileLine = lineNumber
		blocks = append(blocks, current)
	}
	if err := sc.Err(); err != nil {
		return "", nil, fmt.Errorf("%s: coverage: scan profile: %w", profilePath, err)
	}
	if lineNumber == 0 {
		return "", nil, fmt.Errorf("%s:1: coverage: missing profile header", profilePath)
	}
	return mode, blocks, nil
}

// parseLine parses "file:startLine.startCol,endLine.endCol numStmt count".
func parseLine(line string) (block, error) {
	colon := strings.LastIndex(line, ":")
	if colon < 0 {
		return block{}, fmt.Errorf("coverage: malformed profile line %q", line)
	}
	fields := strings.Fields(line[colon+1:])
	if len(fields) != 3 {
		return block{}, fmt.Errorf("coverage: malformed profile line %q", line)
	}
	startLine, startColumn, endLine, endColumn, err := positionsOf(fields[0])
	if err != nil {
		return block{}, err
	}
	numStmt, err := strconv.Atoi(fields[1])
	if err != nil {
		return block{}, fmt.Errorf("coverage: bad numStmt in %q: %w", line, err)
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return block{}, fmt.Errorf("coverage: bad count in %q: %w", line, err)
	}
	return block{
		file: line[:colon], span: fields[0], startLine: startLine, startColumn: startColumn,
		endLine: endLine, endColumn: endColumn, numStmt: numStmt, count: count,
	}, nil
}

// positionsOf parses "startLine.startCol,endLine.endCol".
func positionsOf(span string) (startLine, startColumn, endLine, endColumn int, err error) {
	comma := strings.IndexByte(span, ',')
	if comma < 0 {
		return 0, 0, 0, 0, fmt.Errorf("coverage: bad span %q", span)
	}
	parse := func(value string) (int, int, error) {
		dot := strings.IndexByte(value, '.')
		if dot < 0 {
			return 0, 0, fmt.Errorf("coverage: bad span %q", span)
		}
		line, parseErr := strconv.Atoi(value[:dot])
		if parseErr != nil {
			return 0, 0, fmt.Errorf("coverage: bad position %q: %w", span, parseErr)
		}
		column, parseErr := strconv.Atoi(value[dot+1:])
		if parseErr != nil {
			return 0, 0, fmt.Errorf("coverage: bad position %q: %w", span, parseErr)
		}
		if line <= 0 || column <= 0 {
			return 0, 0, fmt.Errorf("coverage: bad position %q", span)
		}
		return line, column, nil
	}
	startLine, startColumn, err = parse(span[:comma])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	endLine, endColumn, err = parse(span[comma+1:])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return startLine, startColumn, endLine, endColumn, nil
}

// ignoredLines returns, per file, the set of block start lines to drop. A
// trailing directive (code before the comment) drops the block on its own line;
// a standalone directive (only whitespace before the comment) drops the block on
// the line directly below it. A directive without a non-empty reason is an error.
func ignoredLines(blocks []block, srcRoot, modPath string) (map[string]map[int]bool, error) {
	files := map[string]bool{}
	for _, b := range blocks {
		files[b.file] = true
	}
	ignored := map[string]map[int]bool{}
	for file := range files {
		rel := strings.TrimPrefix(file, modPath+"/")
		src, err := os.ReadFile(filepath.Join(srcRoot, rel))
		if err != nil {
			return nil, err
		}
		directives, err := sourceDirectives(rel, src, blocksForFile(blocks, file))
		if err != nil {
			return nil, err
		}
		set := map[int]bool{}
		for _, directive := range directives {
			set[directive.TargetLine] = true
		}
		ignored[file] = set
	}
	return ignored, nil
}

func moduleRoot() (string, error) {
	dir, err := getwd()
	if err != nil {
		return "", err
	}
	for {
		if hasGoMod(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("coverage: go.mod not found from working directory")
		}
		dir = parent
	}
}

func modulePath(goMod string) (string, error) {
	b, err := os.ReadFile(goMod)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if m, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(m), nil
		}
	}
	return "", fmt.Errorf("coverage: no module line in %s", goMod)
}
