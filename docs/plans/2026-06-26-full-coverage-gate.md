# Plan: Full Coverage Gate and the `// coverage-ignore` Convention

Implements **[ADR-0012](../decisions/0012-full-coverage-gate-and-conventions.md)** (Accepted).
Design rationale lives in the ADR — this plan is the execution record.

## Goal

Reach and enforce 100% statement coverage. Build `internal/coverage` + `cmd/covercheck`
(a Go coverage checker honouring `// coverage-ignore: <reason>`), refactor the `cmd/awf`
entrypoint into a testable `run()` seam, drive every package to 100% via fixture tests, wire
the checker into `./x gate`, record the convention in `AGENTS.md`, and flip ADR-0012 to
Implemented. After this, `./x gate` fails whenever any non-ignored statement is uncovered.

## Architecture summary

- `internal/coverage`: parses a coverprofile (`mode: set` + `path:sL.sC,eL.eC numStmt count`
  lines), reads each profiled source file to locate `// coverage-ignore` markers, drops marked
  blocks from both covered and total counts, and reports `Report{Covered, Total}`. A marker
  without a non-empty `: <reason>` is an error. `CheckProfile` resolves the module root (via a
  `getwd` seam + go.mod) and runs `Check`.
- `cmd/covercheck`: thin `run(args, stdout, stderr) int` over `coverage.CheckProfile`; exits
  non-zero below 100%. `main` is `os.Exit(run(...))` with a `// coverage-ignore`.
- `cmd/awf`: `main` becomes `os.Exit(run(os.Args, os.Stdout, os.Stderr))`; `run(args []string,
  stdout, stderr io.Writer) int` owns dispatch + error→exit-code mapping; `fatal`/`fatalIf`
  deleted; a `getwd` seam covers the cwd-failure path. Handlers thread the writer(s) they use.
- `./x gate`: runs `go test ./... -coverpkg=./... -coverprofile=<tmp>`, then
  `go run ./cmd/covercheck <tmp>`, then `go vet` + lint.
- `-coverpkg=./...` (not a per-package profile) so every statement-bearing package appears and a
  statement counts as covered when *any* test exercises it.

## Tech stack

- Go 1.26; packages: `internal/coverage` (new), `cmd/covercheck` (new), `cmd/awf`, and every
  `internal/*` package for coverage fill. No new external deps (stdlib only).
- Gate: `./x gate` per code commit. **The coverage step is wired in only at Phase 4** — until
  then the gate is unchanged, so incremental commits are not blocked by sub-100% interim state.

## File structure

**Created:** `internal/coverage/coverage.go`, `internal/coverage/coverage_test.go`,
`cmd/covercheck/main.go`, `cmd/covercheck/main_test.go`, `cmd/awf/run_test.go`, and per-package
`*_test.go` additions (see Phase 3).

**Modified:** `cmd/awf/main.go` (run seam, delete fatal/fatalIf, getwd), `cmd/awf/check.go`,
`cmd/awf/invariants.go`, `cmd/awf/list_add.go`, `cmd/awf/upgrade.go`, `cmd/awf/setup.go`
(writer threading), existing `cmd/awf/*_test.go` (call new signatures), `x` (coverage step),
`internal/*` source where a `// coverage-ignore` is unavoidable, many `internal/*_test.go`
(coverage fill), `.claude/awf/agents-doc.yaml` + `AGENTS.md` (convention row),
`docs/decisions/0012-*.md` (Implemented flip), `docs/decisions/ACTIVE.md` (regenerated).

**Slug ↔ backing-test map** (authoritative):

| slug | backing test (file) |
|---|---|
| `coverage-gate-100` | `TestCheckFailsBelow100` (internal/coverage/coverage_test.go) |
| `coverage-ignore-reason` | `TestCheckRejectsReasonlessMarker` (internal/coverage/coverage_test.go) |
| `single-os-exit` | `TestNoOsExitOutsideMain` (cmd/awf/run_test.go) |

---

## Phase 1 — `internal/coverage` + `cmd/covercheck`

### Task 1.1 — Create `internal/coverage/coverage.go`

- [ ] Create `internal/coverage/coverage.go`:

```go
// Package coverage parses a Go coverprofile and reports statement coverage over
// blocks not marked with a coverage-ignore directive. It backs the awf coverage
// gate (ADR-0012): a directive of the form "<slashes> coverage-ignore: <reason>"
// drops its block from both the covered and total counts; a directive with no
// non-empty reason is an error.
package coverage

import (
	"bufio"
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
// not marked for ignore. srcRoot is the module root on disk; modulePath is the
// go.mod module path, used to map profile paths to files on disk.
func Check(profilePath, srcRoot, modPath string) (Report, error) {
	blocks, err := parseProfile(profilePath)
	if err != nil {
		return Report{}, err
	}
	ignored, err := ignoredLines(blocks, srcRoot, modPath)
	if err != nil {
		return Report{}, err
	}
	var rep Report
	for _, b := range blocks {
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
	return block{file: line[:colon], startLine: startLine, numStmt: numStmt, count: count}, nil
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

// ignoredLines returns, per file, the set of block start lines to drop. A block
// is dropped when its start line, or the source line directly above it, carries
// the ignore directive. A directive without a non-empty reason is an error.
func ignoredLines(blocks []block, srcRoot, modPath string) (map[string]map[int]bool, error) {
	files := map[string]bool{}
	for _, b := range blocks {
		files[b.file] = true
	}
	markerLines := map[string]map[int]bool{}
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
			set[i+1] = true // 1-based line numbers
		}
		markerLines[file] = set
	}
	ignored := map[string]map[int]bool{}
	for _, b := range blocks {
		if ignored[b.file] == nil {
			ignored[b.file] = map[int]bool{}
		}
		if markerLines[b.file][b.startLine] || markerLines[b.file][b.startLine-1] {
			ignored[b.file][b.startLine] = true
		}
	}
	return ignored, nil
}

func moduleRoot() (string, error) {
	dir, err := getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("coverage: go.mod not found from working directory")
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
```

### Task 1.2 — Create `internal/coverage/coverage_test.go`

White-box (`package coverage`) so it can drive the `getwd` seam and unexported helpers.
Covers every branch: parse success/malformed, ignore-with-reason, reasonless-marker error,
missing source file, module-root/module-path resolution and their error paths.

- [ ] Create `internal/coverage/coverage_test.go`:

```go
package coverage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProfile writes a coverprofile and returns its path.
func writeProfile(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "cover.out")
	if err := os.WriteFile(p, []byte("mode: set\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// module builds a temp module root: go.mod + one source file, and returns root + modpath.
func module(t *testing.T, src string) (root, modPath string) {
	t.Helper()
	root = t.TempDir()
	modPath = "example.com/m"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modPath+"\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "f.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, modPath
}

func TestCheckCountsCoveredAndTotal(t *testing.T) {
	root, modPath := module(t, "package m\nfunc F() {}\n")
	prof := writeProfile(t, root,
		modPath+"/f.go:2.10,2.12 2 1\n"+
			"\n"+ // blank line: exercises parseProfile's empty-line skip
			modPath+"/f.go:3.1,3.5 3 0\n")
	rep, err := Check(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 5 || rep.Covered != 2 {
		t.Fatalf("got %+v, want Covered 2 Total 5", rep)
	}
	if rep.OK() {
		t.Error("OK should be false below 100%")
	}
}

// invariant: coverage-gate-100
func TestCheckFailsBelow100(t *testing.T) {
	root, modPath := module(t, "package m\nfunc F() {}\n")
	prof := writeProfile(t, root, modPath+"/f.go:2.1,2.5 1 0\n")
	rep, err := Check(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OK() {
		t.Fatal("expected OK false for an uncovered statement")
	}
	// A fully covered profile is OK and 100%.
	prof2 := writeProfile(t, root, modPath+"/f.go:2.1,2.5 1 1\n")
	rep2, err := Check(prof2, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if !rep2.OK() || rep2.Percent() != 100 {
		t.Fatalf("expected OK 100%%, got %+v (%.1f)", rep2, rep2.Percent())
	}
}

func TestCheckHonoursIgnoreMarker(t *testing.T) {
	// Line 2 carries the directive; its block is dropped from both counts,
	// leaving the line-3 covered block as the only statement -> 100%.
	src := "package m\nvar x = 1 //" + " coverage-ignore: defensive\nvar y = 2\n"
	root, modPath := module(t, src)
	prof := writeProfile(t, root,
		modPath+"/f.go:2.1,2.10 1 0\n"+
			modPath+"/f.go:3.1,3.10 1 1\n")
	rep, err := Check(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 1 || !rep.OK() {
		t.Fatalf("ignored block not dropped: %+v", rep)
	}
}

func TestCheckIgnoreMarkerLineAbove(t *testing.T) {
	// Directive on the line directly above the block also drops it.
	src := "package m\n//" + " coverage-ignore: panic guard\npanicline\n"
	root, modPath := module(t, src)
	prof := writeProfile(t, root, modPath+"/f.go:3.1,3.10 1 0\n")
	rep, err := Check(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 0 || !rep.OK() {
		t.Fatalf("line-above directive not honoured: %+v", rep)
	}
}

// invariant: coverage-ignore-reason
func TestCheckRejectsReasonlessMarker(t *testing.T) {
	src := "package m\nvar x = 1 //" + " coverage-ignore\n"
	root, modPath := module(t, src)
	prof := writeProfile(t, root, modPath+"/f.go:2.1,2.10 1 0\n")
	if _, err := Check(prof, root, modPath); err == nil {
		t.Fatal("expected error for a reasonless coverage-ignore directive")
	}
}

func TestCheckMissingProfile(t *testing.T) {
	root, modPath := module(t, "package m\n")
	if _, err := Check(filepath.Join(root, "nope.out"), root, modPath); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestCheckMissingSourceFile(t *testing.T) {
	root, modPath := module(t, "package m\n")
	prof := writeProfile(t, root, modPath+"/ghost.go:2.1,2.5 1 0\n")
	if _, err := Check(prof, root, modPath); err == nil {
		t.Fatal("expected error for missing source file")
	}
}

func TestCheckMalformedLines(t *testing.T) {
	root, modPath := module(t, "package m\n")
	for name, body := range map[string]string{
		"no-colon":    "garbage line\n",
		"few-fields":  modPath + "/f.go:2.1,2.5 1\n",
		"bad-span":    modPath + "/f.go:nope 1 0\n",
		"bad-stmt":    modPath + "/f.go:2.1,2.5 x 0\n",
		"bad-count":   modPath + "/f.go:2.1,2.5 1 x\n",
		"no-comma":    modPath + "/f.go:2.1 1 0\n",
		"no-dot":      modPath + "/f.go:2,3.4 1 0\n", // span has a comma but no dot before it
		"bad-start":   modPath + "/f.go:x.1,2.3 1 0\n", // start line is non-numeric
	} {
		t.Run(name, func(t *testing.T) {
			prof := writeProfile(t, root, body)
			if _, err := Check(prof, root, modPath); err == nil {
				t.Errorf("%s: expected parse error", name)
			}
		})
	}
}

func TestCheckScannerError(t *testing.T) {
	// A line longer than bufio.Scanner's default 64KiB token makes the scanner
	// error, exercising parseProfile's sc.Err() branch.
	root, modPath := module(t, "package m\n")
	prof := writeProfile(t, root, strings.Repeat("a", 70000)+"\n")
	if _, err := Check(prof, root, modPath); err == nil {
		t.Fatal("expected scanner error for an over-long profile line")
	}
}

func TestModulePathReadError(t *testing.T) {
	// modulePath's ReadFile error branch (the go.mod path is unreadable).
	if _, err := modulePath(filepath.Join(t.TempDir(), "absent.mod")); err == nil {
		t.Fatal("expected read error for a missing go.mod")
	}
}

func TestCheckProfileResolvesModule(t *testing.T) {
	root, modPath := module(t, "package m\nfunc F() {}\n")
	prof := writeProfile(t, root, modPath+"/f.go:2.1,2.5 1 1\n")
	swapGetwd(t, func() (string, error) { return root, nil })
	rep, err := CheckProfile(prof)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("expected OK, got %+v", rep)
	}
}

func TestCheckProfileGetwdError(t *testing.T) {
	swapGetwd(t, func() (string, error) { return "", errors.New("boom") })
	if _, err := CheckProfile("x"); err == nil {
		t.Fatal("expected getwd error to propagate")
	}
}

func TestCheckProfileNoModule(t *testing.T) {
	// A working directory with no go.mod ancestor -> moduleRoot fails.
	dir := t.TempDir()
	swapGetwd(t, func() (string, error) { return dir, nil })
	if _, err := CheckProfile("x"); err == nil {
		t.Fatal("expected go.mod-not-found error")
	}
}

func TestCheckProfileNoModuleLine(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("// no module line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	swapGetwd(t, func() (string, error) { return root, nil })
	if _, err := CheckProfile("x"); err == nil {
		t.Fatal("expected no-module-line error")
	}
}

func TestPercentEmptyIs100(t *testing.T) {
	if (Report{}).Percent() != 100 {
		t.Fatal("empty report should be 100%")
	}
}

// swapGetwd overrides the package getwd seam for the duration of a test.
func swapGetwd(t *testing.T, fn func() (string, error)) {
	t.Helper()
	orig := getwd
	getwd = fn
	t.Cleanup(func() { getwd = orig })
}
```

- [ ] Verify: `go test ./internal/coverage/` → `ok`, and
  `go test ./internal/coverage/ -coverprofile=/tmp/cov.out && go tool cover -func=/tmp/cov.out | grep -v 100.0%` prints only the `total:` line at `100.0%`.

### Task 1.3 — Create `cmd/covercheck/main.go`

- [ ] Create `cmd/covercheck/main.go`:

```go
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
```

  The trailing `// coverage-ignore: …` on `main` is the canonical ignore directive and must be
  written verbatim.

### Task 1.4 — Create `cmd/covercheck/main_test.go`

- [ ] Create `cmd/covercheck/main_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modWith builds a temp module (go.mod + f.go) and a coverprofile, then chdir's
// into it so coverage.CheckProfile resolves this module. Returns the profile path.
func modWith(t *testing.T, profileBody string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "f.go"), []byte("package m\nfunc F() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prof := filepath.Join(root, "cover.out")
	if err := os.WriteFile(prof, []byte("mode: set\n"+profileBody), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)
	return prof
}

func TestRunUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for missing arg, got %d", code)
	}
	if !strings.Contains(errb.String(), "usage:") {
		t.Errorf("missing usage text: %q", errb.String())
	}
}

func TestRunHundredPercent(t *testing.T) {
	prof := modWith(t, "example.com/m/f.go:2.1,2.5 1 1\n")
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", prof}, &out, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "100.0%") {
		t.Errorf("expected 100%% report, got %q", out.String())
	}
}

func TestRunBelowHundred(t *testing.T) {
	prof := modWith(t, "example.com/m/f.go:2.1,2.5 1 0\n")
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", prof}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 below 100%%, got %d", code)
	}
	if !strings.Contains(errb.String(), "below 100%") {
		t.Errorf("expected below-100 message, got %q", errb.String())
	}
}

func TestRunCheckError(t *testing.T) {
	prof := modWith(t, "example.com/m/ghost.go:2.1,2.5 1 0\n") // source file missing
	var out, errb bytes.Buffer
	if code := run([]string{"covercheck", prof}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on Check error, got %d", code)
	}
}

// chdir switches to dir for the test, restoring the prior cwd afterward.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}
```

- [ ] Verify: `go test ./cmd/covercheck/` → `ok`. `main`'s only uncovered line is the
  ignored `os.Exit` wrapper.

### Task 1.5 — Commit

- [ ] `./x gate` → `0 issues.` (coverage step not yet wired; ordinary gate).
- [ ] `git add internal/coverage cmd/covercheck`
- [ ] `git commit -m "feat(awf): add internal/coverage and cmd/covercheck"`

---

## Phase 2 — `cmd/awf` `run()` seam

### Task 2.1 — Rewrite `cmd/awf/main.go`

- [ ] Replace the whole of `cmd/awf/main.go` with:

```go
// Command awf renders standardised .claude skills, review agents, and git hooks into a project from embedded templates plus a per-project .claude/awf/ config tree.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func main() { os.Exit(run(os.Args, os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit wrapper; run() is unit-tested

var getwd = os.Getwd

// run dispatches a subcommand and returns a process exit code. All user-facing
// output goes to the injected writers so the dispatch is unit-testable.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: awf <init|sync|check|invariants|list|add|setup|upgrade> [args]")
		return 2
	}
	cwd, err := getwd()
	if err != nil {
		fmt.Fprintln(stderr, "awf:", err)
		return 1
	}
	var cmdErr error
	switch args[1] {
	case "init":
		cmdErr = runInit(cwd, stdout, stderr)
	case "sync":
		cmdErr = runSync(cwd, stdout)
	case "check":
		cmdErr = runCheck(cwd, stdout)
	case "invariants":
		cmdErr = runInvariants(cwd, stdout)
	case "list":
		cmdErr = runList(cwd, stdout)
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "awf:", errors.New("usage: awf add <skill>"))
			return 1
		}
		cmdErr = runAdd(cwd, args[2], stdout)
	case "setup":
		cmdErr = runSetup(cwd, stdout, stderr)
	case "upgrade":
		cmdErr = runUpgrade(cwd, stdout)
	default:
		fmt.Fprintln(stderr, "awf:", fmt.Errorf("unknown command %q", args[1]))
		return 1
	}
	if cmdErr != nil {
		fmt.Fprintln(stderr, "awf:", cmdErr)
		return 1
	}
	return 0
}

func runInit(root string, stdout, stderr io.Writer) error {
	cfgPath := filepath.Join(root, ".claude", "awf", "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return err
		}
		scaffold, err := project.ScaffoldConfig(filepath.Base(root))
		if err != nil {
			return err
		}
		if err := os.WriteFile(cfgPath, scaffold, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "scaffolded %s\n", cfgPath)
	}
	if err := runSync(root, stdout); err != nil {
		return err
	}
	if err := runSetup(root, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "awf init: hook setup skipped:", err)
	}
	return nil
}

// gate refuses to operate against a stale config layout. It runs before
// project.Open (which cannot open a pre-tree project): on a covered schema gap it
// errors with a "run awf upgrade" message; an uncovered gap ("autobump") proceeds
// and the subsequent sync stamps the current schema version.
func gate(root string) error {
	if migrate.GateState(root) == "gate" {
		return fmt.Errorf("config schema is behind (generation %d < %d); run awf upgrade",
			migrate.Generation(root), migrate.Current())
	}
	return nil
}

func runSync(root string, stdout io.Writer) error {
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	if err := p.Sync(); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "awf sync: done")
	return nil
}
```

  Note: `fatal`/`fatalIf` are deleted; error→exit mapping now lives in `run`.

### Task 2.2 — Thread writers into the handlers

- [ ] `cmd/awf/check.go` — change signature and prints:

```go
func runCheck(root string, stdout io.Writer) error {
```
  add `"io"` to imports; replace the three `fmt.Printf`/`fmt.Println` calls with
  `fmt.Fprintf(stdout, …)` / `fmt.Fprintln(stdout, …)` (same format strings). The existing
  `gate(root)` call stays unchanged at the top of `runCheck`.

- [ ] `cmd/awf/invariants.go` — `func runInvariants(root string, stdout io.Writer) error`;
  add `"io"`; `fmt.Println`→`fmt.Fprintln(stdout, …)`, `fmt.Printf`→`fmt.Fprintf(stdout, …)`.

- [ ] `cmd/awf/list_add.go` — `func runList(root string, stdout io.Writer) error` and
  `func runAdd(root, skill string, stdout io.Writer) error`; add `"io"`; the `fmt.Printf` in
  `runList` → `fmt.Fprintf(stdout, …)`; `runAdd`'s final `return runSync(root)` →
  `return runSync(root, stdout)`. `appendSkill` is unchanged.

- [ ] `cmd/awf/upgrade.go` — `func runUpgrade(root string, stdout io.Writer) error`; add
  `"io"`; both prints → `fmt.Fprintln/Fprintf(stdout, …)`; `return runSync(root)` →
  `return runSync(root, stdout)`.

- [ ] `cmd/awf/setup.go` — `func runSetup(root string, stdout, stderr io.Writer) error`; add
  `"io"`; `fmt.Fprintln(os.Stderr, …)` → `fmt.Fprintln(stderr, …)`; `fmt.Println(…)` →
  `fmt.Fprintln(stdout, …)`; thread the subprocess output:

```go
	cmd := exec.Command("git", "config", "core.hooksPath", ".githooks")
	cmd.Dir = root
	cmd.Stdout = stdout
	cmd.Stderr = stderr
```

### Task 2.3 — Update existing `cmd/awf` tests to the new signatures

- [ ] In every existing `cmd/awf/*_test.go` that calls a handler, pass writers. Use
  `io.Discard` where output is not asserted, a `*bytes.Buffer` where it is. Concretely:
  `runInit(proj)` → `runInit(proj, io.Discard, io.Discard)`; `runSync(root)` →
  `runSync(root, io.Discard)`; `runCheck(root)` → `runCheck(root, io.Discard)`;
  `runInvariants(root)` → `runInvariants(root, io.Discard)`; `runList`/`runAdd`/`runUpgrade`
  likewise; `runSetup(root)` → `runSetup(root, io.Discard, io.Discard)`. Add `"io"`
  (and `"bytes"` where asserting) imports.
- [ ] Verify: `go test ./cmd/awf/` → `ok` (pre-existing tests pass under new signatures).

### Task 2.4 — Commit

- [ ] `./x gate` → `0 issues.`
- [ ] `git add cmd/awf/main.go cmd/awf/check.go cmd/awf/invariants.go cmd/awf/list_add.go cmd/awf/upgrade.go cmd/awf/setup.go cmd/awf/*_test.go`
- [ ] `git commit -m "refactor(awf): extract testable run() entrypoint seam in cmd/awf"`

---

## Phase 3 — Drive every package to 100%

Methodology for each task: write tests targeting the named uncovered functions/branches, then
run the package's coverage and fill until clean:

```
go test ./<pkg>/ -coverprofile=/tmp/p.out && go tool cover -func=/tmp/p.out | grep -v '100.0%'
```

The only surviving line must be `total: … 100.0%`. Run `./x gate` before each Phase 3 commit
(still the ordinary test+vet+lint gate — the coverage step is wired in only at Phase 4 — so the
green-gate invariant holds without blocking on interim sub-100% state). Prefer real fixtures
(`t.TempDir`, malformed YAML, missing files, empty inputs) over permission tricks. Where a branch is
reachable *only* via a permission error, guard the test with the `skipIfRoot` helper below and
prefer it last; where a branch is genuinely unreachable, add a
`// coverage-ignore: <reason>` and note it in the commit body for the impl-review audit.

`skipIfRoot` (add once, in the first package test file that needs it):

```go
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission-based error path is unreachable as root")
	}
}
```

### Task 3.1 — `cmd/awf` to 100% (new dispatch + handler tests)

- [ ] Create `cmd/awf/run_test.go` exercising `run` for: no-args (exit 2), unknown command
  (exit 1), `add` with no skill arg (exit 1), the `getwd` failure branch (override the
  `getwd` seam to return an error → exit 1), and a happy dispatch (e.g. `sync` in a scaffolded
  temp project → exit 0, asserting `awf sync: done` on the stdout buffer; `run` reads the cwd
  from the `getwd` seam, so point it at the temp project root for this case). Add a
  `swapGetwd` helper in `cmd/awf` mirroring Phase 1's (the `getwd` seam is package-local).
- [ ] Add the `single-os-exit` backing test in `cmd/awf/run_test.go`:

```go
// invariant: single-os-exit
func TestNoOsExitOutsideMain(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "os.Exit") {
				continue
			}
			// The sole permitted os.Exit is main's one-line wrapper.
			if f == "main.go" && strings.Contains(line, "func main()") {
				continue
			}
			t.Errorf("%s:%d: os.Exit outside main's wrapper: %s", f, i+1, strings.TrimSpace(line))
		}
	}
}
```

- [ ] Fill remaining handler branches (currently sub-100): `runInit` (the
  already-scaffolded path where `config.yaml` exists → skips scaffold; and the
  setup-skipped warning path), `runCheck`/`runInvariants` (the non-empty drift/findings print
  loops and error returns), `runList` (the `Sidecar` error branch), `runAdd` (not-a-catalog
  skill, already-enabled, the happy add+sync), `appendSkill` (`skills: []`, bare `skills:`,
  and the no-`skills:`-key error), `runUpgrade` (already-current vs applied), `runSetup`
  (no-`.githooks` error, not-a-git-repo warning, the `git config` happy path), `gate` (the
  stale-schema error branch via a fixture whose lock generation is behind).
- [ ] Verify `cmd/awf` at 100% per the methodology command.
- [ ] Commit: `git add cmd/awf && git commit -m "test(awf): cover cmd/awf dispatch and handlers to 100%"`

### Task 3.2 — `internal/project` to 100%

- [ ] Cover the named gaps: `CheckInvariants` (call it directly on an opened project),
  `declaredSections` (all three `kind` cases — skills/agents/docs — plus the default), `Open`
  (error paths: missing config, malformed config), `validateFrontmatter` / `renderTarget` /
  `localOutPath` / `Check` / `Sync` / `RenderAll` / `generateActiveMD` / `orphans` /
  `validateAgainstCatalog` / `targetConfigHash` / `checkLocalFrontmatter` / `resolvedDocs` /
  `scaffold.go` helpers — drive each remaining branch with a fixture project under `t.TempDir`.
- [ ] Verify `internal/project` at 100%.
- [ ] Commit: `git add internal/project && git commit -m "test(awf): cover internal/project to 100%"`

### Task 3.3 — `internal/migrate` to 100%

- [ ] Cover `treelayout.go` (`applyTreeLayout`, `portAgentsDoc`, `copyPart`, `writeYAML`,
  `writeFile`, `portSidecar`), `legacy.go` (`readLegacy`), `migrate.go` (`Upgrade`). Error
  branches: missing source part, unreadable/garbled legacy config, write-target collisions —
  via temp-dir fixtures; `skipIfRoot` only if a write-permission branch has no other trigger.
- [ ] Verify `internal/migrate` at 100%.
- [ ] Commit: `git add internal/migrate && git commit -m "test(awf): cover internal/migrate to 100%"`

### Task 3.4 — `internal/invariants` to 100%

- [ ] Cover `Detail` (both `Status == Unchecked` and the unbacked branch), `Check` (remaining
  branches), `scanTags` (walk-error and the skip-dir cases).
- [ ] Verify `internal/invariants` at 100%.
- [ ] Commit: `git add internal/invariants && git commit -m "test(awf): cover internal/invariants to 100%"`

### Task 3.5 — Remaining internal packages to 100%

- [ ] `internal/adr` (`ParseDir`, `parse`, `RenderActiveMD` error/edge branches),
  `internal/config` (`Load`, `Sidecar` error branches), `internal/frontmatter` (`Parse`
  malformed cases), `internal/catalog` (`Load` error branch), `internal/manifest`
  (`Load`/`Save` error branches), `internal/render` (`Execute`, `ParseSections` branches).
  One commit per package, each verified at 100%:
  - `git commit -m "test(awf): cover internal/adr to 100%"`
  - `git commit -m "test(awf): cover internal/config to 100%"`
  - `git commit -m "test(awf): cover internal/frontmatter to 100%"`
  - `git commit -m "test(awf): cover internal/catalog to 100%"`
  - `git commit -m "test(awf): cover internal/manifest to 100%"`
  - `git commit -m "test(awf): cover internal/render to 100%"`
- [ ] After all packages: verify the whole-module gate semantics manually —
  `go test ./... -coverpkg=./... -coverprofile=/tmp/all.out && go run ./cmd/covercheck /tmp/all.out`
  → `coverage: 100.0% (… statements)` and exit 0. If any line is uncovered, covercheck names
  the count; return to the relevant package task.

---

## Phase 4 — Wire the gate, document, finalize

### Task 4.1 — Wire the coverage step into `./x gate`

- [ ] In `x`, replace the `gate)` case body with:

```
  gate)
    # Full gate: profiled tests + 100% coverage check + vet + lint. The optional
    # `full` arg is accepted for hook compatibility (pre-push runs `./x gate full`).
    prof="$(mktemp)"
    trap 'rm -f "$prof"' EXIT
    go test ./... -coverpkg=./... -coverprofile="$prof"
    go run ./cmd/covercheck "$prof"
    go vet ./...
    go tool golangci-lint run
    ;;
```

- [ ] Verify: `./x gate` → ends with `0 issues.` and prints a `coverage: 100.0%` line; exit 0.
- [ ] Confirm a regression trips it: temporarily add an untested exported function in a scratch
  file, run `./x gate`, observe `covercheck: coverage below 100%` and non-zero exit, then
  remove the scratch file. (Manual check; nothing committed.)

### Task 4.2 — Record the convention in `AGENTS.md`

The `AGENTS.md` "Invariants" list is rendered from `.claude/awf/agents-doc.yaml`
(`data.invariants`, per the existing rows; the template at `templates/agents-doc/AGENTS.md.tmpl`
renders each entry as `- {{ .text }}{{ with .ref }} ({{ . }}){{ end }}`, so `ref:` already
appends the `(ADR-NNNN)` citation — the `text` must NOT repeat it). Add a row so contributors
learn the gate and the escape hatch.

- [ ] In `.claude/awf/agents-doc.yaml`, add to `data.invariants` (after the existing
  backed-invariants row), matching the surrounding 8-space list indentation and `ref:`-first
  single-quoted style:

```yaml
        - ref: ADR-0012
          text: '**100% coverage gate.** `./x gate` fails below 100% statement coverage. A genuinely-unreachable defensive branch may be excluded with a `// coverage-ignore: <reason>` directive (a non-empty reason is mandatory); every use is audited at review.'
```

  (Confirm key/indentation against the file before editing — the rendered `AGENTS.md` is
  regenerated, never hand-edited.)

- [ ] `./x sync` — re-renders `AGENTS.md`.
- [ ] `git add .claude/awf/agents-doc.yaml .claude/awf/awf.lock AGENTS.md x`
- [ ] `git commit -m "feat(awf): wire 100% coverage gate into ./x and document it"`

### Task 4.3 — Flip ADR-0012 to Implemented

- [ ] In `docs/decisions/0012-*.md`, change `status: Accepted` → `status: Implemented`. The
  three slugs (`coverage-gate-100`, `coverage-ignore-reason`, `single-os-exit`) are already
  backed by Phase 1/3 tests, so `./x check` stays clean once 0012 is enforced.
- [ ] `./x sync` — regenerates `ACTIVE.md`.
- [ ] `./x gate` → coverage 100% + `0 issues.`; `./x check` → `awf check: clean`;
  `./x invariants` → `awf invariants: clean`.
- [ ] `git add docs/decisions/0012-full-coverage-gate-and-conventions.md docs/decisions/ACTIVE.md .claude/awf/awf.lock`
- [ ] `git commit -m "docs(adr): mark 0012 Implemented"`

### Task 4.4 — Terminal handoff

- [ ] Invoke `awf-reviewing-impl` against the Phase 1–4 commit range. The reviewer audits every
  `// coverage-ignore` marker for justification.

---

## Notes

- **Gate-wiring order:** the coverage step lands only in Phase 4, after every package is at
  100%. Phases 1–3 commit under the ordinary gate, so interim sub-100% state never blocks them.
- **`-coverpkg=./...`:** a statement counts as covered when any test exercises it, and every
  statement-bearing package appears in the profile (closing the untested-package hole).
  `templates` has zero statements and never appears.
- **The two `main` directives** (`cmd/awf`, `cmd/covercheck`) are the expected canonical
  `// coverage-ignore` uses — one-line `os.Exit(run(...))` wrappers whose logic is tested via
  `run`. Any further directive must be justified at impl review.
- **Self-scan safety:** `internal/coverage` builds its own marker string by concatenation so
  the checker, when reading `coverage.go` from a profile, does not see a literal reasonless
  directive on the definition line. Test files carrying literal directives are never scanned
  (test files are not instrumented, so never appear in the profile).
