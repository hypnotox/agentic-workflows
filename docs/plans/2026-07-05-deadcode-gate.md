# Plan: Whole-Program Dead-Code Gate via deadcode

**ADR:** [ADR-0063](../decisions/0063-whole-program-dead-code-gate-via-deadcode.md) (Proposed)
**Date:** 2026-07-05

## Goal

Add `golang.org/x/tools/cmd/deadcode` as a pinned `go tool` dependency and wire a blocking
`./x gate` step (`go tool deadcode -json ./... | go run ./cmd/deadcodecheck`) that fails on any
production function unreachable from a `main` outside `internal/testsupport/`. Resolve the two
day-one findings by relocating test-only helpers into `_test.go` files. No escape hatch.

## Architecture summary

deadcode runs **without `-test`** (the `-test` mode is always-empty under the ADR-0012 100%
coverage gate). `cmd/deadcodecheck` reads deadcode's JSON from stdin, drops findings whose
`Position.File` begins with `internal/testsupport/` (the ADR-0044 leaf test-helper package — the
one structural exemption), and exits non-zero on anything else. Design rationale lives in the
ADR; this plan is the execution record.

## Tech stack

- Go 1.26; module `github.com/hypnotox/agentic-workflows`.
- New dep: `golang.org/x/tools/cmd/deadcode` (x/tools v0.44.0 already indirect).
- Model for the wrapper: `cmd/covercheck` (stdin/arg → `run()` → exit code; `main()` carries a
  `coverage-ignore`).

## File structure

**Created**
- `cmd/deadcodecheck/main.go`
- `cmd/deadcodecheck/main_test.go`
- `internal/project/export_test.go`
- `internal/adr/export_test.go`

**Modified**
- `go.mod`, `go.sum` (tool directive + telemetry/sys tool deps)
- `internal/project/project.go` (remove `Sync`)
- `internal/adr/adr.go` (remove `SetNowForTest`)
- `internal/evals/fixture_test.go` (migrate the single cross-package `p.Sync()` caller)
- `cmd/awf/run_test.go` (stale-comment fix)
- `x` (gate step + `deadcode` subcommand + usage)
- `.awf/agents-doc.yaml`, `.awf/domains/parts/tooling/current-state.md`,
  `.awf/docs/parts/testing/layout.md`, `.awf/docs/parts/architecture/dependencies.md` (doc sources)
- Rendered outputs via `awf sync`: `AGENTS.md`, `docs/domains/tooling.md`, `docs/testing.md`,
  `docs/architecture.md`, `docs/decisions/ACTIVE.md`, `.awf/awf.lock`
- `docs/decisions/0063-*.md` (status flip, final commit)

---

## Phase 1 — Pin the deadcode tool dependency

- [ ] **Task 1.1 — Add the tool directive.** Run exactly:
  ```
  go get -tool golang.org/x/tools/cmd/deadcode@v0.44.0
  go mod tidy
  ```
  Expected: `go.mod`'s single `tool github.com/golangci/...golangci-lint` line becomes a
  `tool ( ... )` block also listing `golang.org/x/tools/cmd/deadcode`; `go.sum` gains
  `golang.org/x/telemetry` entries.
- [ ] **Task 1.2 — Verify the tool is runnable.** Run:
  ```
  grep -A3 '^tool (' go.mod
  go tool deadcode -json ./... | head -c 40
  ```
  Expected: the grep shows `golang.org/x/tools/cmd/deadcode` inside the block; the deadcode run
  emits JSON (a `[` … array) with no "missing go.sum entry" error.
- [ ] **Task 1.3 — Gate + commit.** Run `./x gate` (still the pre-deadcode gate; must pass).
  Stage `go.mod go.sum` and commit:
  ```
  build(tooling): pin deadcode as a go tool dependency
  ```

## Phase 2 — Relocate test-only helpers out of production

- [ ] **Task 2.1 — Move `Project.Sync` into a test file.** In `internal/project/project.go`
  delete exactly:
  ```go
  func (p *Project) Sync() error {
  	_, err := p.SyncReport()
  	return err
  }
  ```
  Create `internal/project/export_test.go`:
  ```go
  package project

  // Sync renders and writes the project like SyncReport, discarding the backup
  // report — a test-only convenience for the many in-package tests that only care
  // whether the sync errors. Production uses SyncReport directly (ADR-0063).
  func (p *Project) Sync() error {
  	_, err := p.SyncReport()
  	return err
  }
  ```
  Then reword `SyncReport`'s doc comment in `internal/project/project.go` so production
  prose no longer points at the now test-only `Sync` (godoc for `SyncReport` would otherwise
  reference a symbol absent from the package's non-test API):
  - `// SyncReport renders and writes the project like Sync, additionally backing up any` →
    `// SyncReport renders and writes the project, additionally backing up any`
- [ ] **Task 2.2 — Migrate the single cross-package caller.** In `internal/evals/fixture_test.go`,
  change the comment and call in `syncFullCatalog`:
  - `// real Project.Sync, and returns the project root. It reuses the exported` →
    `// real Project.SyncReport, and returns the project root. It reuses the exported`
  - `if err := p.Sync(); err != nil {` → `if _, err := p.SyncReport(); err != nil {`
- [ ] **Task 2.3 — Move `SetNowForTest` into a test file.** In `internal/adr/adr.go` delete
  exactly (keep the `var now = time.Now` block above it):
  ```go
  // SetNowForTest overrides the now seam for a test and returns the previous
  // value, so the caller can restore it. Exported because adr_test.go is an
  // external test package (package adr_test), mirroring the project's existing
  // var-seam-for-coverage convention rather than adding an internal test file.
  func SetNowForTest(fn func() time.Time) (prev func() time.Time) {
  	prev = now
  	now = fn
  	return prev
  }
  ```
  Create `internal/adr/export_test.go`:
  ```go
  package adr

  import "time"

  // SetNowForTest overrides the now seam for a test and returns the previous
  // value, so the caller can restore it. It lives in an in-package _test.go file
  // (package adr) so the external adr_test package can reach it without the seam
  // shipping in the production binary (ADR-0063).
  func SetNowForTest(fn func() time.Time) (prev func() time.Time) {
  	prev = now
  	now = fn
  	return prev
  }
  ```
- [ ] **Task 2.4 — Fix the stale comment.** In `cmd/awf/run_test.go:280`, change
  `// A directory squatting on a rendered output path makes p.Sync() fail.` →
  `// A directory squatting on a rendered output path makes p.SyncReport() fail.`
- [ ] **Task 2.5 — Update generated-doc sources (Project.Sync → SyncReport).** The eval fixture
  now calls `SyncReport`, so the three narratives naming `Project.Sync` as the eval entry point
  become inaccurate. Edit the `.awf/` sources (never the rendered files):
  - `.awf/agents-doc.yaml:53`: `renders every catalog skill and agent via a full \`Project.Sync\`` →
    `renders every catalog skill and agent via a full \`Project.SyncReport\``
  - `.awf/domains/parts/tooling/current-state.md:17`: `renders the full catalog via \`Project.Sync\`` →
    `renders the full catalog via \`Project.SyncReport\``
  - `.awf/docs/parts/testing/layout.md:11`: `runs a full \`Project.Sync\` over a fixture config` →
    `runs a full \`Project.SyncReport\` over a fixture config`
- [ ] **Task 2.6 — Re-render.** Run `./x sync`. Expected: `AGENTS.md`, `docs/domains/tooling.md`,
  `docs/testing.md`, and `.awf/awf.lock` update; `./x check` reports `awf check: clean`.
- [ ] **Task 2.7 — Verify deadcode is clean except testsupport.** Run:
  ```
  go tool deadcode ./...
  ```
  Expected: every reported line begins with `internal/testsupport/` (16 helpers); no
  `internal/project` or `internal/adr` line remains.
- [ ] **Task 2.8 — Gate + commit.** Run `./x gate` (must pass — relocations are behaviour-
  preserving; coverage stays 100% as the two functions leave the production denominator). Stage
  the modified source + regenerated docs + lock and commit:
  ```
  refactor(tooling): relocate test-only helpers into _test.go files

  Move Project.Sync and adr.SetNowForTest out of production files into
  in-package export_test.go files, and migrate the one cross-package
  internal/evals caller to SyncReport. Both were test-only helpers; they no
  longer ship in the production binary (ADR-0063 Decision 6).
  ```

## Phase 3 — Add the deadcodecheck wrapper

- [ ] **Task 3.1 — Write `cmd/deadcodecheck/main.go`:**
  ```go
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
  ```
- [ ] **Task 3.2 — Write `cmd/deadcodecheck/main_test.go`** (the `// invariant: deadcode-gate`
  marker backs ADR-0063; it asserts a non-testsupport finding fails and a testsupport finding is
  ignored):
  ```go
  package main

  import (
  	"bytes"
  	"errors"
  	"strings"
  	"testing"
  )

  type errReader struct{}

  func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

  // invariant: deadcode-gate
  func TestRunFiltersTestsupportOnly(t *testing.T) {
  	const j = `[{"Path":"x","Funcs":[
  		{"Name":"Helper","Position":{"File":"internal/testsupport/x.go","Line":10}},
  		{"Name":"Dead","Position":{"File":"internal/project/p.go","Line":42}}]}]`
  	var out, errb bytes.Buffer
  	if code := run(strings.NewReader(j), &out, &errb); code != 1 {
  		t.Fatalf("expected exit 1 with a non-testsupport finding, got %d", code)
  	}
  	if !strings.Contains(errb.String(), "internal/project/p.go:42") ||
  		!strings.Contains(errb.String(), "Dead") {
  		t.Errorf("offender not reported: %q", errb.String())
  	}
  	if strings.Contains(errb.String(), "testsupport") {
  		t.Errorf("testsupport finding should be ignored, got %q", errb.String())
  	}
  }

  func TestRunAllTestsupport(t *testing.T) {
  	const j = `[{"Path":"x","Funcs":[
  		{"Name":"H","Position":{"File":"internal/testsupport/gitfixture/g.go","Line":1}}]}]`
  	var out, errb bytes.Buffer
  	if code := run(strings.NewReader(j), &out, &errb); code != 0 {
  		t.Fatalf("expected exit 0 when all findings are testsupport, got %d (%s)", code, errb.String())
  	}
  	if !strings.Contains(out.String(), "no production dead code") {
  		t.Errorf("expected clean message, got %q", out.String())
  	}
  }

  func TestRunEmptyInputs(t *testing.T) {
  	for _, in := range []string{"", "  \n", "null", "[]"} {
  		var out, errb bytes.Buffer
  		if code := run(strings.NewReader(in), &out, &errb); code != 0 {
  			t.Fatalf("input %q: expected exit 0, got %d (%s)", in, code, errb.String())
  		}
  	}
  }

  func TestRunMalformed(t *testing.T) {
  	var out, errb bytes.Buffer
  	if code := run(strings.NewReader("{not json"), &out, &errb); code != 1 {
  		t.Fatalf("expected exit 1 on malformed json, got %d", code)
  	}
  	if !strings.Contains(errb.String(), "parsing") {
  		t.Errorf("expected parse error, got %q", errb.String())
  	}
  }

  func TestRunReadError(t *testing.T) {
  	var out, errb bytes.Buffer
  	if code := run(errReader{}, &out, &errb); code != 1 {
  		t.Fatalf("expected exit 1 on read error, got %d", code)
  	}
  }
  ```
- [ ] **Task 3.3 — Verify the wired pipeline manually.** Run:
  ```
  go tool deadcode -json ./... | go run ./cmd/deadcodecheck
  ```
  Expected: `deadcodecheck: no production dead code`, exit 0 (Phase 2 cleaned production).
- [ ] **Task 3.4 — Gate + commit.** Run `./x gate` (must pass; `cmd/deadcodecheck` reaches 100%
  coverage via its tests). Stage `cmd/deadcodecheck/` and commit:
  ```
  feat(tooling): add deadcodecheck gate wrapper

  Reads deadcode -json from stdin, ignores the internal/testsupport/ path
  boundary, and exits non-zero on any other unreachable production func. No
  escape hatch (ADR-0063). Backs inv: deadcode-gate.
  ```

## Phase 4 — Wire the gate

- [ ] **Task 4.1 — Add the gate step.** In `x`, in the `gate)` case, after the line
  `    go tool golangci-lint run` add:
  ```
      go tool deadcode -json ./... | go run ./cmd/deadcodecheck
  ```
- [ ] **Task 4.2 — Add the `deadcode` subcommand.** In `x`, after the `lint)` case block
  (three lines: `  lint)`, then `    go tool golangci-lint run "$@"`, then `    ;;`) add a new case:
  ```
    deadcode)
      go tool deadcode -json ./... | go run ./cmd/deadcodecheck
      ;;
  ```
- [ ] **Task 4.3 — Update usage.** In `x`, change the usage line to include `deadcode`:
  ```
      echo "usage: ./x <gate [full]|lint|fmt|test|deadcode|sync|check|invariants|audit|commit-gate|new|build|install>" >&2
  ```
- [ ] **Task 4.4 — Verify.** Run `./x deadcode` (expect `deadcodecheck: no production dead code`)
  and `./x gate` (full gate now includes deadcode; must pass).
- [ ] **Task 4.5 — Commit.** Stage `x` and commit (`x` is outside the awf render/lock set, so no
  `awf check` impact):
  ```
  feat(tooling): run the dead-code gate in ./x gate

  Append `go tool deadcode -json ./... | go run ./cmd/deadcodecheck` as the
  final gate step and add a `./x deadcode` alias. pipefail makes either stage
  fail the gate (ADR-0063 Decision 4).
  ```

## Phase 5 — Document the gate

- [ ] **Task 5.1 — Dependencies doc.** In `.awf/docs/parts/architecture/dependencies.md`, after the
  `golangci-lint` bullet (line 7) add:
  ```
  - **`deadcode`** (`golang.org/x/tools/cmd/deadcode`) — pinned as a `go tool` dependency; the
    gate runs it (no `-test`) and `cmd/deadcodecheck` fails on any production function unreachable
    from a `main` outside `internal/testsupport/` (ADR-0063).
  ```
- [ ] **Task 5.2 — Tooling current-state.** In `.awf/domains/parts/tooling/current-state.md`, change
  the gate sentence in the first paragraph:
  `The gate is \`go test ./... && go vet && golangci-lint\`, with a hard 100% statement-coverage floor (\`cmd/covercheck\`; a genuinely-unreachable branch may carry a justified \`// coverage-ignore:\`).`
  →
  `The gate is \`go test ./... && go vet && golangci-lint\` plus a whole-program dead-code pass (\`deadcode\` without \`-test\`, piped through \`cmd/deadcodecheck\`; ADR-0063), with a hard 100% statement-coverage floor (\`cmd/covercheck\`; a genuinely-unreachable branch may carry a justified \`// coverage-ignore:\`). The dead-code gate fails on any production function unreachable from a \`main\` outside \`internal/testsupport/\` and carries no escape hatch.`
- [ ] **Task 5.3 — AGENTS.md invariant.** In `.awf/agents-doc.yaml`, in `data.invariants`, after the
  ADR-0012 entry (the `**100% coverage gate.**` block) add:
  ```yaml
          - ref: ADR-0063
            text: '**Dead-code gate.** `./x gate` runs `deadcode` (no `-test`) over `./...` and fails on any production function unreachable from a `main` outside `internal/testsupport/`; `cmd/deadcodecheck` enforces this with no `//deadcode:ignore` escape hatch.'
  ```
- [ ] **Task 5.4 — Testing layout (optional gate mention).** In `.awf/docs/parts/testing/layout.md`,
  no gate-tier text change is required beyond the Task 2.5 rename; skip unless a gate-steps
  sentence exists to extend.
- [ ] **Task 5.5 — Re-render + verify.** Run `./x sync` then `./x check`. Expected:
  `AGENTS.md`, `docs/architecture.md`, `docs/domains/tooling.md`, `.awf/awf.lock` update; check
  reports `awf check: clean`.
- [ ] **Task 5.6 — Gate + commit.** Run `./x gate`. Stage the `.awf/` sources + regenerated docs +
  lock and commit:
  ```
  docs(tooling): document the dead-code gate
  ```

## Phase 6 — Mark ADR-0063 Implemented

- [ ] **Task 6.1 — Flip status.** In `docs/decisions/0063-whole-program-dead-code-gate-via-deadcode.md`
  change frontmatter `status: Proposed` → `status: Implemented`.
- [ ] **Task 6.2 — Regenerate ACTIVE.md.** Run `./x sync`. Expected: `docs/decisions/ACTIVE.md`
  moves ADR-0063 to the Implemented section; `docs/domains/tooling.md` moves it out of Proposed;
  `.awf/awf.lock` updates.
- [ ] **Task 6.3 — Verify invariant backing.** Run `./x check`. Expected `awf check: clean` — the
  `inv: deadcode-gate` slug is now backed by the `// invariant: deadcode-gate` marker in
  `cmd/deadcodecheck/main_test.go` (added Phase 3), so the Implemented-ADR invariant check passes.
- [ ] **Task 6.4 — Gate + commit.** Run `./x gate`. Stage the ADR + regenerated `ACTIVE.md` +
  `docs/domains/tooling.md` + lock and commit:
  ```
  docs(adr): mark ADR-0063 Implemented
  ```

## Verification (whole plan)

- `./x gate` passes at every commit and the final gate includes the deadcode step.
- `go tool deadcode ./...` reports only `internal/testsupport/` lines.
- Introducing a throwaway unreferenced exported function in any `internal/*` production file makes
  `./x deadcode` (and `./x gate`) fail; removing it restores green.
- `./x check` reports `awf check: clean`; ADR-0063 is Implemented with its invariant backed.
