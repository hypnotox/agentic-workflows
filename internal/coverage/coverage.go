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
	"path/filepath"
	"strconv"
	"strings"
)

// marker is the ignore directive in its comment form. It is assembled by
// concatenation so this source line does not itself contain the literal
// directive — otherwise the scanner, when reading this very file out of a
// coverprofile, would treat this line as a reasonless directive and error.
var marker = "//" + " coverage-ignore"

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

// CheckProfile resolves the module root from the working directory (nearest
// ancestor with a go.mod) and checks profilePath against the module sources.
func CheckProfile(profilePath string) (Report, error) {
	root, err := moduleRoot()
	if err != nil {
		return Report{}, err
	}
	modPath, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return Report{}, err
	}
	return Check(profilePath, root, modPath)
}

// Check parses the coverprofile at profilePath and returns a Report over blocks
// not marked for ignore. srcRoot is the module root on disk; modPath is the
// go.mod module path, used to map profile paths to files on disk.
//
// A profile produced by `go test ./... -coverpkg=./...` emits each instrumented
// block once per test binary, so the same block recurs many times with differing
// counts. Blocks are first merged by identity (file + span), OR-ing the counts
// (mode: set), exactly as `go tool cover` does — otherwise the denominator is
// inflated by the duplication.
func Check(profilePath, srcRoot, modPath string) (Report, error) {
	blocks, err := parseProfile(profilePath)
	if err != nil {
		return Report{}, err
	}
	merged := map[string]block{}
	for _, b := range blocks {
		k := b.file + ":" + b.span
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
	ignored, err := ignoredLines(uniq, srcRoot, modPath)
	if err != nil {
		return Report{}, err
	}
	var rep Report
	for _, b := range uniq {
		if ignored[b.file][b.startLine] {
			continue
		}
		rep.Total += b.numStmt
		if b.count > 0 {
			rep.Covered += b.numStmt
		}
	}
	return rep, nil
}

// block is one parsed coverprofile line.
type block struct {
	file      string // module-qualified source path, e.g. mod/pkg/file.go
	span      string // raw "startLine.col,endLine.col" — block identity within a file
	startLine int
	numStmt   int
	count     int
}

func parseProfile(path string) ([]block, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var blocks []block
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first { // "mode: set" header
			first = false
			continue
		}
		if line == "" {
			continue
		}
		b, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return blocks, nil
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
	startLine, err := startLineOf(fields[0])
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
	return block{file: line[:colon], span: fields[0], startLine: startLine, numStmt: numStmt, count: count}, nil
}

// startLineOf extracts startLine from "startLine.startCol,endLine.endCol".
func startLineOf(span string) (int, error) {
	comma := strings.IndexByte(span, ',')
	if comma < 0 {
		return 0, fmt.Errorf("coverage: bad span %q", span)
	}
	dot := strings.IndexByte(span[:comma], '.')
	if dot < 0 {
		return 0, fmt.Errorf("coverage: bad span %q", span)
	}
	n, err := strconv.Atoi(span[:dot])
	if err != nil {
		return 0, fmt.Errorf("coverage: bad start line %q: %w", span, err)
	}
	return n, nil
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
		set := map[int]bool{}
		for i, line := range strings.Split(string(src), "\n") {
			idx := strings.Index(line, marker)
			if idx < 0 {
				continue
			}
			reason := strings.TrimSpace(line[idx+len(marker):])
			if !strings.HasPrefix(reason, ":") || strings.TrimSpace(reason[1:]) == "" {
				return nil, fmt.Errorf("%s:%d: %s requires a non-empty reason (use %q)",
					rel, i+1, marker, marker+": <why>")
			}
			lineNo := i + 1 // 1-based
			if strings.TrimSpace(line[:idx]) == "" {
				set[lineNo+1] = true // standalone directive -> block on the line below
			} else {
				set[lineNo] = true // trailing directive -> block on this line
			}
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
