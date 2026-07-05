# Plan: Advisory mutation-testing command (`./x mutants`)

**ADR:** [ADR-0066](../decisions/0066-advisory-mutation-testing-command.md) (Proposed)
**Date:** 2026-07-05

## Goal

Pin `gremlins` as a `go tool` dependency, commit the deterministic `.gremlins.yaml` recipe,
add a tested `cmd/mutants` wrapper that turns a gremlins `-o` JSON report into a clean
survived-mutant triage list (failing loudly on any timeout), and wire an advisory `./x mutants`
command (diff-scoped by default, whole-package on a path arg). Never part of `./x gate`.

## Architecture summary

`gremlins unleash -i --workers 1 --timeout-coefficient 20` is deterministic on this repo's
sub-second suite; the recipe lives in `.gremlins.yaml` (nested under `unleash:`) and auto-loads.
`cmd/mutants` reads the `-o` JSON file path, exits non-zero on any `TIMED OUT` mutation (the
trust signal — a timeout is scored outside efficacy and can hide a survivor), treats a missing
or empty file as an empty run, drops `NOT COVERED` noise, and prints the `LIVED` set. `./x mutants`
runs gremlins over the `main` diff (or a package) into a temp file, then pipes it through the
wrapper. Design rationale lives in the ADR; this plan is the execution record.

## Tech stack

- Go 1.26; module `github.com/hypnotox/agentic-workflows`.
- New dep: `github.com/go-gremlins/gremlins/cmd/gremlins` (v0.6.0), pinned as a `go tool`.
- Model for the wrapper: `cmd/covercheck` (file-path arg → `run()` → exit code; `main()` carries a
  `coverage-ignore`).

## File structure

**Created**
- `cmd/mutants/main.go`
- `cmd/mutants/main_test.go`
- `.gremlins.yaml`

**Modified**
- `go.mod`, `go.sum` (gremlins tool directive + viper/cast/fsnotify MVS bump)
- `x` (`mutants` subcommand + usage)
- `.awf/docs/parts/testing/gate.md`, `.awf/domains/parts/tooling/current-state.md`,
  `.awf/agents-doc.yaml` (doc sources)
- Rendered outputs via `awf sync`: `docs/testing.md`, `docs/domains/tooling.md`, `AGENTS.md`,
  `docs/decisions/ACTIVE.md`, `.awf/awf.lock`
- `docs/decisions/0066-*.md` (status flip, final commit)

---

## Phase 1 — Pin gremlins and the deterministic config

- [ ] **Task 1.1 — Add the tool directive.** Run exactly:
  ```
  go get -tool github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
  go mod tidy
  ```
  Expected: `go.mod`'s `tool ( ... )` block gains a `github.com/go-gremlins/gremlins/cmd/gremlins`
  line; `go.sum` gains gremlins plus the raised `github.com/spf13/viper v1.21.0`,
  `github.com/spf13/cast v1.10.0`, `github.com/fsnotify/fsnotify v1.9.0`. No error.
- [ ] **Task 1.2 — Verify the tool is runnable.** Run:
  ```
  grep -A4 '^tool (' go.mod
  go tool gremlins --version
  ```
  Expected: the grep shows `github.com/go-gremlins/gremlins/cmd/gremlins` inside the block;
  `--version` prints `gremlins version v0.6.0 ...` with no "missing go.sum entry" error.
- [ ] **Task 1.3 — Write `.gremlins.yaml`** at the repo root, exactly:
  ```yaml
  # Deterministic mutation-testing config for `./x mutants` (ADR-0066). Integration
  # mode + a single worker + a generous timeout coefficient drive per-mutant timeouts
  # to zero on this sub-second suite, so verdicts are reproducible. Keys MUST nest
  # under `unleash:` — a flat file is silently ignored by viper. Trust a run only when
  # it reports `Timed out: 0`.
  unleash:
    integration: true
    workers: 1
    timeout-coefficient: 20
  ```
- [ ] **Task 1.4 — Confirm the recipe reproduces.** Run:
  ```
  go tool gremlins unleash -o /tmp/mut-refs.json ./internal/refs
  ```
  Expected output includes `Killed: 13, Lived: 3` and — the load-bearing trust signal —
  `Timed out: 0,`; the config auto-loaded (single worker, integration mode, zero timeouts).
  (The `Not covered:` count is package-local coverage noise the wrapper drops; don't gate this
  check on its exact value.)
- [ ] **Task 1.5 — Gate + commit.** Run `./x gate` (must pass — the MVS bump leaves lint,
  coverage, and deadcode green; spike-verified). Stage `go.mod go.sum .gremlins.yaml` and commit:
  ```
  build(tooling): pin gremlins tool and deterministic config

  Pin github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 as a go tool and
  commit .gremlins.yaml holding the deterministic recipe (integration,
  workers 1, timeout-coefficient 20, nested under `unleash:`). The MVS bump
  to viper/cast/fsnotify leaves the shipped awf binary untouched and ./x gate
  green (ADR-0066).
  ```

## Phase 2 — Add the `cmd/mutants` wrapper

- [ ] **Task 2.1 — Write `cmd/mutants/main.go`:**
  ```go
  // Command mutants reads a gremlins -o JSON report and prints the surviving
  // (LIVED) mutants as an advisory triage list, backing the awf `./x mutants`
  // command (ADR-0066). It exits non-zero when any mutant timed out: a timeout is
  // scored outside efficacy and can hide a real survivor, so the whole run is
  // untrustworthy. A missing or empty report file is treated as an empty run.
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
  	data, err := os.ReadFile(args[1])
  	if err != nil {
  		if os.IsNotExist(err) {
  			// gremlins writes no -o file when there is nothing to report.
  			fmt.Fprintln(stdout, "no survived mutants")
  			return 0
  		}
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
  		fmt.Fprintf(stderr, "mutants: %d mutant(s) timed out — result untrustworthy; "+
  			"raise timeout-coefficient and rerun\n", timedOut)
  		return 1
  	}
  	if len(lived) == 0 {
  		fmt.Fprintln(stdout, "no survived mutants")
  		return 0
  	}
  	sort.Strings(lived)
  	fmt.Fprintln(stdout, "survived mutants (triage each — some may be equivalent):")
  	for _, l := range lived {
  		fmt.Fprintln(stdout, "  "+l)
  	}
  	return 0
  }
  ```
- [ ] **Task 2.2 — Write `cmd/mutants/main_test.go`** (the `// invariant: mutants-timeout-untrusted`
  marker backs ADR-0066; sibling tests cover the LIVED-only filtering and empty-run cases):
  ```go
  package main

  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )

  // writeJSON writes body to a temp file and returns its path.
  func writeJSON(t *testing.T, body string) string {
  	t.Helper()
  	p := filepath.Join(t.TempDir(), "out.json")
  	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	return p
  }

  func TestRunUsage(t *testing.T) {
  	var out, errb bytes.Buffer
  	if code := run([]string{"mutants"}, &out, &errb); code != 2 {
  		t.Fatalf("expected exit 2 for missing arg, got %d", code)
  	}
  	if !strings.Contains(errb.String(), "usage:") {
  		t.Errorf("missing usage text: %q", errb.String())
  	}
  }

  func TestRunMissingFile(t *testing.T) {
  	var out, errb bytes.Buffer
  	if code := run([]string{"mutants", filepath.Join(t.TempDir(), "nope.json")}, &out, &errb); code != 0 {
  		t.Fatalf("expected exit 0 for missing file, got %d (%s)", code, errb.String())
  	}
  	if !strings.Contains(out.String(), "no survived mutants") {
  		t.Errorf("expected empty-run message, got %q", out.String())
  	}
  }

  func TestRunEmptyFile(t *testing.T) {
  	for _, body := range []string{"", "   \n"} {
  		var out, errb bytes.Buffer
  		if code := run([]string{"mutants", writeJSON(t, body)}, &out, &errb); code != 0 {
  			t.Fatalf("body %q: expected exit 0, got %d", body, code)
  		}
  		if !strings.Contains(out.String(), "no survived mutants") {
  			t.Errorf("body %q: expected empty-run message, got %q", body, out.String())
  		}
  	}
  }

  func TestRunReadError(t *testing.T) {
  	// A directory path is not IsNotExist but os.ReadFile still errors.
  	var out, errb bytes.Buffer
  	if code := run([]string{"mutants", t.TempDir()}, &out, &errb); code != 1 {
  		t.Fatalf("expected exit 1 on read error, got %d", code)
  	}
  	if !strings.Contains(errb.String(), "mutants:") {
  		t.Errorf("expected error prefix, got %q", errb.String())
  	}
  }

  func TestRunMalformed(t *testing.T) {
  	var out, errb bytes.Buffer
  	if code := run([]string{"mutants", writeJSON(t, "{not json")}, &out, &errb); code != 1 {
  		t.Fatalf("expected exit 1 on malformed json, got %d", code)
  	}
  	if !strings.Contains(errb.String(), "parsing") {
  		t.Errorf("expected parse error, got %q", errb.String())
  	}
  }

  // invariant: mutants-timeout-untrusted
  func TestRunTimedOutIsUntrusted(t *testing.T) {
  	// A LIVED survivor is present, but the timeout makes the whole run untrustworthy.
  	const j = `{"files":[{"file_name":"refs.go","mutations":[
  		{"type":"CONDITIONALS_BOUNDARY","status":"LIVED","line":85},
  		{"type":"ARITHMETIC_BASE","status":"TIMED OUT","line":92}]}]}`
  	var out, errb bytes.Buffer
  	if code := run([]string{"mutants", writeJSON(t, j)}, &out, &errb); code != 1 {
  		t.Fatalf("expected exit 1 when a mutant timed out, got %d", code)
  	}
  	if !strings.Contains(errb.String(), "timed out") {
  		t.Errorf("expected timeout message, got %q", errb.String())
  	}
  }

  func TestRunReportsOnlyLived(t *testing.T) {
  	const j = `{"files":[{"file_name":"refs.go","mutations":[
  		{"type":"ARITHMETIC_BASE","status":"KILLED","line":63},
  		{"type":"CONDITIONALS_NEGATION","status":"NOT COVERED","line":60},
  		{"type":"CONDITIONALS_BOUNDARY","status":"NOT VIABLE","line":70},
  		{"type":"ARITHMETIC_BASE","status":"LIVED","line":92}]}]}`
  	var out, errb bytes.Buffer
  	if code := run([]string{"mutants", writeJSON(t, j)}, &out, &errb); code != 0 {
  		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
  	}
  	o := out.String()
  	if !strings.Contains(o, "refs.go:92  ARITHMETIC_BASE") {
  		t.Errorf("LIVED mutant not reported: %q", o)
  	}
  	if strings.Contains(o, "NOT COVERED") || strings.Contains(o, ":60") || strings.Contains(o, ":70") {
  		t.Errorf("only LIVED should be reported, got %q", o)
  	}
  }

  func TestRunNoSurvivors(t *testing.T) {
  	const j = `{"files":[{"file_name":"refs.go","mutations":[
  		{"type":"ARITHMETIC_BASE","status":"KILLED","line":63},
  		{"type":"CONDITIONALS_NEGATION","status":"NOT COVERED","line":60}]}]}`
  	var out, errb bytes.Buffer
  	if code := run([]string{"mutants", writeJSON(t, j)}, &out, &errb); code != 0 {
  		t.Fatalf("expected exit 0, got %d", code)
  	}
  	if !strings.Contains(out.String(), "no survived mutants") {
  		t.Errorf("expected no-survivors message, got %q", out.String())
  	}
  }
  ```
- [ ] **Task 2.3 — Verify coverage and behaviour.** Run:
  ```
  go test ./cmd/mutants/ -cover
  ```
  Expected: `ok` with `coverage: 100.0% of statements`.
- [ ] **Task 2.4 — Gate + commit.** Run `./x gate` (must pass; `cmd/mutants` reaches 100% and its
  `main()`/`run()` are reachable, so the deadcode gate stays green). Stage `cmd/mutants/` and commit:
  ```
  feat(tooling): add mutants survivor-triage wrapper

  Reads a gremlins -o JSON report, exits non-zero on any TIMED OUT mutation
  (the trust signal — a timeout can hide a survivor), treats a missing or
  empty file as an empty run, drops NOT COVERED noise, and prints the LIVED
  set. Backs inv: mutants-timeout-untrusted (ADR-0066).
  ```

## Phase 3 — Wire the `./x mutants` command

- [ ] **Task 3.1 — Add the `mutants` case.** In `x`, immediately before the `*)` default case (the
  three lines `  *)`, `    echo "usage: ...`, `    exit 2`), insert:
  ```
    mutants)
      # Advisory mutation triage (ADR-0066). No args: mutate production code changed
      # vs main. A path arg (e.g. ./internal/refs): mutate that package. Never gated.
      tmp="$(mktemp)"
      trap 'rm -f "$tmp"' EXIT
      if [ "$#" -gt 0 ]; then
        go tool gremlins unleash -o "$tmp" "$@"
      else
        base="$(git merge-base HEAD main)" || {
          echo "mutants: no merge-base with 'main' (detached HEAD or missing branch); pass a package path, e.g. ./x mutants ./internal/refs" >&2
          exit 2
        }
        go tool gremlins unleash -D "$base" -o "$tmp" ./...
      fi
      go run ./cmd/mutants "$tmp"
      ;;
  ```
  Note: under the committed `.gremlins.yaml` the efficacy/coverage thresholds stay at their `0`
  defaults, so `gremlins unleash` exits `0` even with surviving mutants — `set -e` therefore does
  not abort before `go run ./cmd/mutants` runs. Never add a threshold to the config, or the
  survivor path (Task 3.3) would abort the script before the wrapper prints.
- [ ] **Task 3.2 — Update usage.** In `x`, change the usage line to include `mutants`:
  ```
      echo "usage: ./x <gate [full]|lint|fmt|test|deadcode|sync|check|invariants|audit|commit-gate|new|build|install|mutants>" >&2
  ```
- [ ] **Task 3.3 — Verify package mode.** Run:
  ```
  ./x mutants ./internal/refs
  ```
  Expected: a `survived mutants (triage each — some may be equivalent):` header followed by three
  `refs.go:...` lines (85/92/100), exit 0. (No `TIMED OUT` message.)
- [ ] **Task 3.4 — Verify diff mode on a clean branch.** Run:
  ```
  ./x mutants
  ```
  Expected (on `main` with no local changes vs `main`): `no survived mutants`, exit 0 — the empty
  diff produces an empty/absent report the wrapper tolerates.
- [ ] **Task 3.5 — Commit.** Stage `x` and commit (`x` is outside the awf render/lock set, so no
  `awf check` impact):
  ```
  feat(tooling): add ./x mutants advisory command

  Diff-scoped by default (gremlins -D against the main merge-base, guarded
  for detached HEAD / missing main) or a given package, piped through
  cmd/mutants. Advisory — never part of ./x gate (ADR-0066).
  ```

## Phase 4 — Document the command

- [ ] **Task 4.1 — Rewrite the testing-doc stance.** In `.awf/docs/parts/testing/gate.md`, in the
  "Coverage is not verification" section, replace the final sentence exactly (it is hard-wrapped
  across three source lines in the current file — match the wrapped text, not this single-line
  rendering):
  - `This is a deliberate manual habit, not a gate: an automated mutation-testing step was evaluated and left out (the available tooling was too timeout-sensitive and mode-dependent to yield trustworthy, reproducible numbers here).`
  →
  - `This is a deliberate manual habit. \`./x mutants\` (ADR-0066) makes it reproducible: it runs \`gremlins\` mutation testing under a deterministic config (\`.gremlins.yaml\`, \`-i --workers 1\`) and prints the survived mutants for you to triage — run it with no arguments to check your diff against \`main\`, or pass a package path (e.g. \`./x mutants ./internal/refs\`) for a deep dive. Its numbers are trustworthy only when the run reports \`Timed out: 0\`; a nonzero count can hide a real survivor, so raise the timeout coefficient and rerun. It stays advisory — never part of the gate — and every survivor still needs you to judge whether it is a real gap or an unkillable equivalent mutant.`
- [ ] **Task 4.2 — Add the tooling current-state sentence.** In
  `.awf/domains/parts/tooling/current-state.md`, in the first paragraph, after
  `The dead-code gate fails on any production function unreachable from a \`main\` outside \`internal/testsupport/\` and carries no escape hatch.`
  add (same paragraph, following sentence):
  ```
  Separately, `./x mutants` (ADR-0066) is an advisory, never-gated mutation-testing command: it runs `gremlins` under a deterministic config (`.gremlins.yaml`; integration mode, one worker) over the production diff vs `main` or a given package, and `cmd/mutants` prints the survived (`LIVED`) mutants to triage — exiting non-zero when any mutant times out, since a timeout can hide a survivor (trustworthy only when the run reports `Timed out: 0`).
  ```
- [ ] **Task 4.3 — Add the AGENTS.md invariant.** In `.awf/agents-doc.yaml`, in `data.invariants`,
  immediately after the ADR-0063 dead-code-gate entry (the `- ref: ADR-0063` / `text: '**Dead-code
  gate.** ...'` pair), add (indent to match the siblings exactly — 8 spaces before `- ref`, 10
  before `text`):
  ```yaml
        - ref: ADR-0066
          text: '**Advisory mutation triage.** `./x mutants` runs `gremlins` under the deterministic `.gremlins.yaml` config and is never part of `./x gate`; `cmd/mutants` exits non-zero when any mutant times out (the run is untrustworthy) and otherwise reports only survived (`LIVED`) mutants, treating a missing or empty report as no survivors.'
  ```
- [ ] **Task 4.4 — Re-render + verify.** Run `./x sync` then `./x check`. Expected: `docs/testing.md`,
  `docs/domains/tooling.md`, `AGENTS.md`, and `.awf/awf.lock` update; `./x check` reports
  `awf check: clean`.
- [ ] **Task 4.5 — Gate + commit.** Run `./x gate`. Stage the `.awf/` sources + regenerated docs +
  lock and commit:
  ```
  docs(tooling): document the advisory mutation command
  ```

## Phase 5 — Mark ADR-0066 Implemented

- [ ] **Task 5.1 — Flip status.** In `docs/decisions/0066-advisory-mutation-testing-command.md`
  change frontmatter `status: Proposed` → `status: Implemented`.
- [ ] **Task 5.2 — Regenerate ACTIVE.md.** Run `./x sync`. Expected: `docs/decisions/ACTIVE.md`
  moves ADR-0066 to the Implemented section; `docs/domains/tooling.md` moves it out of Proposed;
  `.awf/awf.lock` updates.
- [ ] **Task 5.3 — Verify invariant backing.** Run `./x check`. Expected `awf check: clean` — the
  `inv: mutants-timeout-untrusted` slug is now backed by the `// invariant: mutants-timeout-untrusted`
  marker in `cmd/mutants/main_test.go` (added Phase 2), so the Implemented-ADR invariant check passes.
- [ ] **Task 5.4 — Gate + commit.** Run `./x gate`. Stage the ADR + regenerated `ACTIVE.md` +
  `docs/domains/tooling.md` + lock and commit:
  ```
  docs(adr): mark ADR-0066 Implemented
  ```

## Verification (whole plan)

- `./x gate` passes at every commit; `./x gate` never invokes `./x mutants`.
- `./x mutants ./internal/refs` reports the three `refs.go` survivors with exit 0; `./x mutants`
  on a clean `main` reports `no survived mutants`.
- A gremlins report containing a `TIMED OUT` mutation makes `cmd/mutants` exit non-zero with the
  "result untrustworthy" message (asserted by `TestRunTimedOutIsUntrusted`).
- `./x check` reports `awf check: clean`; ADR-0066 is Implemented with `inv:
  mutants-timeout-untrusted` backed.
