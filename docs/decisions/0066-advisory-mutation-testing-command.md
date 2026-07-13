---
status: Implemented
date: 2026-07-05
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [mutation-testing]
related: [2, 12, 63]
domains: [tooling]
---
# ADR-0066: Advisory mutation-testing command

## Context

awf's gate proves *execution*, not *verification*. The ADR-0012 100% statement-coverage
floor guarantees every statement runs under test; it does not guarantee any test would
**fail** if that statement were wrong — a line covered by a test that never asserts on its
effect passes the gate while a broken result slips through. `docs/testing.md` ("Coverage is
not verification", rendered from `.awf/docs/parts/testing/gate.md`) already records this and
prescribes a manual habit: flip a condition, negate a comparison, change a constant, and
confirm a test turns red. That same file currently states that "an automated
mutation-testing step was evaluated and left out (the available tooling was too
timeout-sensitive and mode-dependent to yield trustworthy, reproducible numbers here)."

That evaluation has been overturned by a concrete finding: **a deterministic gremlins
configuration exists.** The earlier flakiness had a single root cause. gremlins derives a
per-mutant timeout from the baseline suite runtime; awf's suite is sub-second, so the
timeout is tiny, and under parallel-worker CPU contention innocent mutants hit `TIMED OUT`.
`TIMED OUT` is its own bucket, *excluded* from efficacy (`killed / (killed + lived)`,
`internal/report/report.go`): a timed-out would-be **survivor** is reclassified as
`TIMED OUT`, so it vanishes from both the denominator and the survivor list — efficacy
inflates *and a real gap silently disappears*. This is why the old `-i` mode "falsely
reported ~100%": `-i` runs the whole `go test ./...` per mutant (much slower), so at a low
coefficient nearly everything timed out.

The fix removes both variables: **`gremlins unleash -i --workers 1 --timeout-coefficient
20`.** `--workers 1` eliminates contention; a high coefficient drives timeouts to zero.
Measured on `internal/refs` this is byte-identical across three runs (`Killed: 13, Lived: 3,
Timed out: 0`, efficacy 81.25%) and matches the same package's default-mode run; on
`internal/invariants` default and `-i` agree exactly. **The trust signal is `Timed out:
0`** — any nonzero count means the numbers are inflated and a survivor may be hidden, so the
run must be discarded (raise the coefficient).

Two further properties shape the design:

1. **Package-local coverage blindness.** gremlins gathers coverage per-package even in `-i`
   mode, so lines exercised only by cross-package tests report as `NOT COVERED` (false — e.g.
   `refs.go:60-62`, covered via `internal/project`). `-i` mode does fix *kill* detection
   across packages (a mutant killed by another package's test shows `KILLED`, not a false
   `LIVED`); the residual `NOT COVERED` noise must simply be dropped, never surfaced.

2. **Equivalent mutants dominate a mature suite.** On the 100%-covered tree, whole-package
   survivors are mostly *equivalent* mutants — syntactically changed, semantically identical,
   unkillable by any test. Deciding equivalence is formally undecidable, so no tool discards
   them reliably; go-gremlins offers **no** per-mutant suppression (only `--exclude-files`
   regexp and status filters). The community norm is therefore diff-scoping plus an efficacy
   *threshold*, not per-mutant triage. The prior in-repo audit already fixed the genuine
   covered-but-unverified gaps, so today's whole-package survivors are near-entirely
   equivalent — high noise. In **diff mode** the noise collapses: survivors appear only on
   lines just changed, where each is a plausible missing assertion.

A throwaway-worktree spike de-risked the mechanics against the version `go tool` would pin
(**v0.6.0**, vs the `dev` build the recipe was first measured on):

- `go get -tool github.com/go-gremlins/gremlins/cmd/gremlins` pins v0.6.0 and raises shared
  deps module-wide via MVS — `viper 1.12→1.21`, `cast 1.5→1.10`, `fsnotify 1.5.4→1.9.0` —
  which also feed the golangci-lint tool build. **`./x gate` stayed fully green** under the
  bump (golangci-lint built and ran: 0 issues; coverage 100%; deadcode clean). The shipped
  `awf` binary is unaffected: `cmd/awf` imports neither cobra nor viper and `.goreleaser.yaml`
  builds only `./cmd/awf`, so the tool tree touches only `go.sum` on source builds — the same
  trade ADR-0002 and ADR-0063 accepted.
- On v0.6.0, a repo-root `.gremlins.yaml` with the recipe **nested under `unleash:`**
  auto-loads (no flags) and reproduces the `refs` result identically, with an identical `-o`
  JSON shape. The keys are `unleash.integration` / `unleash.workers` /
  `unleash.timeout-coefficient`; a *flat* file is silently ignored by viper.
- Diff mode on clean `main` (`-D $(git merge-base HEAD main) ./...`) exits 0 with "No results
  to report" and **writes no `-o` file at all** — the empty-run case the wrapper must tolerate.

Scope is this repo's own developer tooling only. Promoting mutation testing into the rendered
awf *standard* is explicitly out of scope: gremlins is Go-only, and the standard is
language-agnostic.

## Decision

1. **Adopt gremlins as a pinned Go `tool` dependency** — `go get -tool
   github.com/go-gremlins/gremlins/cmd/gremlins`, added to the `go.mod` `tool (` block and
   run via `go tool gremlins`. This matches the ADR-0002 (golangci-lint) and ADR-0063
   (deadcode) mechanism: pinned in `go.mod`, no separate install. The v0.6.0 MVS bump to
   `viper`/`cast`/`fsnotify` is accepted; the spike confirmed it leaves `./x gate` green and
   the shipped binary untouched.

2. **Commit `.gremlins.yaml` at the repo root** holding the deterministic recipe nested under
   `unleash:` (`integration: true`, `workers: 1`, `timeout-coefficient: 20`) — the single
   source of the recipe, auto-loaded by `go tool gremlins`. It is a hand-maintained
   tool-config file outside the awf render/lock set, like `.golangci.yml` and
   `.goreleaser.yaml`; it is **not** a rendered artifact and `./x sync`/`awf check` ignore it.

3. **Add `cmd/mutants`**, a small Go program that takes the gremlins `-o` JSON **file path as
   an argument** (mirroring `cmd/covercheck`) and:
   - exits **non-zero if any mutation status is `TIMED OUT`** — the trust signal; the numbers
     are inflated and a survivor may be hidden, so the result is untrustworthy and the message
     tells the user to raise `--timeout-coefficient`;
   - treats a **missing or empty (zero-byte) output file as an empty run** ("no survived
     mutants", exit 0) — gremlins writes no `-o` file when there is nothing to report, and when
     the wrapper pre-creates `$tmp` (e.g. via `mktemp`, as `./x gate` does for its profile) that
     file is left empty rather than removed, so both the absent-file and empty-file cases must be
     tolerated;
   - **drops `NOT COVERED`** (package-local coverage noise) and ignores every other status
     (`KILLED`, `NOT VIABLE`, `SKIPPED`, `RUNNABLE`);
   - prints the `LIVED` mutants as a `file:line  TYPE` triage list, or "no survived mutants"
     when the `LIVED` set is empty.
   The file-path→exit shape keeps it a pure, table-testable function.

4. **Add `./x mutants`**, an advisory command:
   - no args → **diff mode**: `go tool gremlins unleash -D "$base" -o "$tmp" ./...` where
     `base=$(git merge-base HEAD main)`, then `go run ./cmd/mutants "$tmp"`;
   - a path arg (e.g. `./x mutants ./internal/refs`) → that package instead of the diff.
   The runner **guards the merge-base**: if `git merge-base HEAD main` fails (detached HEAD, no
   local `main`) it errors with a clear message and never passes `-D ""`. A standalone command,
   like `./x lint`; the bash dispatch is not unit-tested.

5. **Advisory only — never wired into `./x gate`.** This does not replace the manual
   "coverage is not verification" habit; it is a deterministic, low-friction aid for it. The
   human still triages every survivor (equivalent vs. genuine gap).

6. **No equivalent-mutant suppression file and no efficacy threshold.** Diff mode is the
   default because it keeps survivors relevant to just-changed code; whole-package mode is a
   deep-dive that is documented to expect equivalent survivors. Per-mutant suppression is
   neither supported by go-gremlins nor sound (equivalence is undecidable), and a threshold
   belongs to a gate this decision explicitly declines to build.

7. **Correct the `docs/testing.md` stance at its source.** Rewrite the "was evaluated and
   left out" paragraph in `.awf/docs/parts/testing/gate.md` to point at `./x mutants`, state
   the `Timed out: 0` trust signal, and frame diff-mode as the default with whole-package as a
   deep-dive; re-render via `./x sync` and confirm with `./x check`. Never hand-edit the
   rendered `docs/testing.md`.

8. **`cmd/mutants` is subject to the ADR-0012 100% coverage gate and the ADR-0063 dead-code
   gate** like every package (its `main()` carries the standard `// coverage-ignore` on the
   `os.Exit` wrapper, and its `run` is reachable from `main`). `./x`, `.gremlins.yaml`, and the
   `go.mod` tool directive are hand-maintained repo files outside the awf render/lock set, so
   they do not affect `awf check`.

## Invariants

- `invariant: mutants-timeout-untrusted` — `cmd/mutants` exits non-zero when any mutation in its
  input JSON has status `TIMED OUT`, and otherwise reports exactly the `LIVED` mutants —
  dropping `NOT COVERED` and every other status — treating a missing or empty input file as an
  empty run. Backed by a `// invariant: mutants-timeout-untrusted` marker on the `cmd/mutants`
  test that asserts a `TIMED OUT` input fails, a `LIVED`/`NOT COVERED` mix reports only the
  `LIVED`, and a missing or empty file reports no survivors (lands with the implementation).
- `./x mutants` is advisory and is never invoked by `./x gate`; the gate's behaviour is
  unchanged by this decision. (textual)
- The deterministic recipe lives only in `.gremlins.yaml`, nested under `unleash:`, with
  `workers: 1` and a `timeout-coefficient` large enough that runs report `Timed out: 0`;
  results are trusted only when `Timed out: 0`. (textual)
- gremlins is pinned via the `go.mod` `tool` directive; running `./x mutants` needs no
  separate install. (textual)
- No equivalent-mutant suppression list and no efficacy threshold exist; survivor triage is
  manual. (textual)

## Consequences

Easier:
- The "coverage is not verification" habit gains a deterministic, reproducible instrument: a
  developer can point `./x mutants` at their diff and get a stable survivor worklist instead
  of running gremlins raw and misreading timeout-inflated numbers.
- The trust signal is enforced in code — a timeout-corrupted run fails loudly rather than
  silently reporting a falsely clean result.

Harder / accepted trade-offs:
- `go.mod`/`go.sum` gain gremlins' tool tree and a module-wide `viper`/`cast`/`fsnotify` bump
  (relevant to `go mod tidy` and the golangci-lint tool build at publish time) — verified
  green by the spike, and irrelevant to the shipped binary.
- Whole-package runs are slow (`-i --workers 1` ≈ 1s/mutant, serial; ~3 min for
  `internal/project`), so the command is diff-scoped by default and never runs `./...`
  eagerly in the gate.
- On a mature 100%-covered package, most survivors are equivalent mutants; the tool cannot
  distinguish them, so whole-package output carries inherent noise that the user must triage.
  Diff mode is the mitigation, not a suppression list.
- The pinned v0.6.0 differs from the `dev` build the recipe was first measured on; the spike
  reconfirmed reproducibility, but a future gremlins upgrade must re-confirm `Timed out: 0`
  and the JSON shape.

Ruled out (for now):
- A mutation-testing *gate* / efficacy threshold in `./x gate` (Decision 5, 6).
- An equivalent-mutant suppression file (Decision 6).
- Promoting mutation testing into the rendered awf standard (out of scope — Go-only tool).

Downstream work unblocked: an implementation plan covering — add the tool dep + `go mod tidy`;
commit `.gremlins.yaml`; write `cmd/mutants` with tests and the `// invariant:
mutants-timeout-untrusted` backing; add the `./x mutants` case with the merge-base guard;
rewrite `.awf/docs/parts/testing/gate.md` and re-render. When this ADR flips to Implemented,
the same commit backs the tagged invariant and regenerates `docs/decisions/ACTIVE.md`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| A mutation-testing gate (efficacy threshold in `./x gate`) | Whole-module runs are ~10–15 min, and equivalent survivors make any hard threshold either noisy or arbitrary; a blocking gate also contradicts the standing "advisory, manual triage" decision. Diff-scoped advisory matches the community norm. |
| Documentation-only (publish the recipe, no command) | Raw gremlins output is a footgun: `NOT COVERED` noise plus silent timeout-inflated efficacy. The value is precisely the wrapper that enforces `Timed out: 0` and filters to `LIVED`. |
| gremlins as an unpinned external prerequisite | Version drift would silently change verdicts/JSON; the spike proved pinning as a `go tool` is free for the shipped binary, so there is no reason to accept drift. |
| Equivalent-mutant suppression file (`known-equivalent.json`) | go-gremlins has no per-mutant suppression and equivalence is undecidable; a hand-maintained list is a maintenance surface that fights the tool's grain. Diff-mode default keeps noise low without it. |
| Bash + `jq` wrapper instead of `cmd/mutants` | Untested JSON-in-bash logic plus a new `jq` dependency; a tested Go tool matches the established `cmd/covercheck` / `cmd/deadcodecheck` pattern and carries the trust-signal invariant. |
| `cmd/mutants` reads stdin (like `cmd/deadcodecheck`) | gremlins `-o` writes to a *file*, not stdout; taking the file path as an arg (like `cmd/covercheck`) avoids a needless `cat`→stdin hop and handles the no-file empty-run case naturally. |
| A different Go mutation tool (e.g. `go-mutesting`) | go-gremlins is the actively-maintained Go mutation tool and the one the prior in-repo evaluation already measured; peer tools are less maintained and would discard the deterministic `Timed out: 0` recipe verified here. |
