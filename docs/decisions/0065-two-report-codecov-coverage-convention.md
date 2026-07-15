---
status: Implemented
date: 2026-07-05
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [coverage-gate]
related: [12, 63, 79]
domains: [tooling]
---
# ADR-0065: Two-Report Codecov Coverage Convention

## Context

ADR-0012 makes `./x gate` fail below **100% of non-`// coverage-ignore` statements**:
the gate measures *statement* coverage over the blocks awf holds itself accountable
for, and a sanctioned `// coverage-ignore: <reason>` directive excludes a genuinely
unreachable defensive branch. There are ~118 such markers today: overwhelmingly
`if err != nil` arms over already-validated state (go-git object access, `embed.FS`
reads, `yaml.Marshal` of known-good structs), permission-fault arms that root bypasses,
and the three `os.Exit` `main` wrappers. Real statement coverage measured by
`go tool cover -func` is **96.5%**; the ~3.5% gap is exactly those ignored blocks.

CI already generates the gate's profile a second time and uploads it to Codecov:

```
go test ./... -coverpkg=./... -coverprofile=coverage.out
codecov/codecov-action@v5  (files: coverage.out)
```

Codecov reports roughly **94%**. Two facts about that number shaped this decision:

1. **Codecov reports *line* coverage, not *statement* coverage.** A Go coverprofile is
   per-statement-block; Codecov maps each block onto source lines and divides
   lines-hit by lines-total. That is a different metric from `go tool cover`'s statement
   percentage, so Codecov's ~94% legitimately differs from the 96.5% and **cannot be
   made to equal it**: the divergence is line-vs-statement, not a pipeline bug. Any
   promise to "make Codecov match the gate" is therefore unachievable by construction.

2. **Codecov cannot see the `coverage-ignore` convention.** It parses the raw
   coverprofile, which contains every block regardless of our directives, so it counts
   each sanctioned defensive branch as a miss. The single headline number thus reads as
   negligence when it is in fact accountable-coverage-minus-sanctioned-defensive-code.
   The convention that makes the gate honest is invisible to the reporter.

The fix is to publish **two** figures rather than one ambiguous one: a **raw** number
(the honest reality, ~94% line coverage, which climbs only as real branches get covered)
and a **covered** number (coverage over the same non-ignored blocks the gate enforces,
~100% line coverage: the accountability promise). The "ignored" set must have a single
source of truth: it already lives in `internal/coverage` (`parseProfile` +
`ignoredLines`), which backs the ADR-0012 gate. Codecov must never re-implement the
convention as a path/line exclusion in `codecov.yml`; the second number is derived by
*filtering the coverprofile* through the same package, so the reporter and the gate
can never disagree about what "ignored" means.

One Codecov mechanic constrains the shape. Codecov merges every upload for a commit into
a single **project** coverage number; because the covered profile is a strict subset of
the raw profile, that merged headline equals the raw figure. The two numbers therefore
surface not as two headline badges but as two **flags**: each visible in the PR-comment
table, as its own status check, and (the deliverable that satisfies "two reports") as its
own README badge via a per-flag badge URL. The honest headline staying the raw number is
the correct outcome, not a limitation to work around.

Scope is this repo's own coverage *reporting* only. ADR-0012's gate behaviour and the
`coverage-ignore` convention are unchanged. Promoting two-report coverage into the
rendered awf *standard* (a shipped `codecov.yml` template + catalog wiring) is a
separately load-bearing change and is out of scope.

## Decision

1. **Emit a filtered coverprofile from `internal/coverage`.** Add an exported function
   that reads a coverprofile and returns/writes a new profile containing exactly the
   blocks whose start line is **not** in the `ignoredLines` set, reusing `parseProfile`
   and `ignoredLines` so the definition of "ignored" is shared verbatim with the
   ADR-0012 gate. The emitter writes a `mode: set` header (the mode `parseProfile`
   discards; CI's profile is always `set`) followed by each surviving block re-emitted as
   `file:span numStmt count`. It emits merged-unique blocks (the same `Check` merge), not
   raw per-binary duplicates.

2. **Expose it through `cmd/covercheck`.** Add a `--emit-filtered` mode to
   `covercheck`'s existing `run(args, ...)` entrypoint: given a profile path it writes the
   filtered profile to stdout (or a named output). This keeps the capability reachable
   from a production `main` (satisfying the ADR-0063 dead-code gate) and the logic
   table-testable through `run`.

3. **CI uploads two flagged reports.** `.github/workflows/ci.yml` generates the raw
   profile as today, derives the filtered profile via `covercheck --emit-filtered`, and
   runs `codecov/codecov-action@v5` **twice**: once with `flags: raw` on the full
   profile, once with `flags: covered` on the filtered profile. `fail_ci_if_error: true`
   is retained on both (an upload failure fails the gate job); each step needs the
   `CODECOV_TOKEN`.

4. **Add `codecov.yml`** defining the two flags and a merge-timing rule:
   `codecov.notify.after_n_builds: 2` so the PR comment waits for both uploads. Both
   coverage **status checks are `informational: true`**: `./x gate` (statement, ADR-0012)
   remains the sole hard coverage enforcer; Codecov (line) reports but never blocks a
   merge, avoiding a second gate on a divergent metric. `codecov.yml` carries **no** path
   or line ignore that restates the `coverage-ignore` convention.

5. **Show two README badges** via per-flag badge URLs
   (`.../graph/badge.svg?flag=raw` and `?flag=covered`), labelled so a reader understands
   raw = honest line coverage and covered = accountable (non-ignored) line coverage.

6. **Document the line-vs-statement distinction** where coverage is explained
   (`docs/testing.md`): Codecov figures are line coverage and intentionally differ from
   the gate's statement coverage; the raw badge ≈ 94% is not reconciled to
   `go tool cover`'s 96.5% because the two measure different things.

7. **New code is subject to the existing gates.** The `internal/coverage` emitter and the
   `covercheck` flag branch are themselves under the ADR-0012 100% statement floor and the
   ADR-0063 dead-code gate (reachable via `covercheck`'s `main`). `codecov.yml` and
   `.github/workflows/ci.yml` are hand-maintained repo files outside the awf render/lock
   set, so they do not affect `awf check`.

## Invariants

- `invariant: covered-profile-honors-ignores`: the filtered coverprofile emitted by
  `internal/coverage` contains a block **iff** that block is not `// coverage-ignore`-d
  under the same `ignoredLines` logic the ADR-0012 gate uses; the two never diverge on
  what "ignored" means. Backed by a `// invariant: covered-profile-honors-ignores` marker
  on the emitter's test asserting an ignored block is dropped and a non-ignored block is
  kept (lands with the implementation).
- The Codecov `covered` flag is fed the *filtered* profile and the `raw` flag the *full*
  profile; `codecov.yml` contains no ignore rule that re-implements the `coverage-ignore`
  convention: the convention's single source of truth stays `internal/coverage`. (textual)
- Codecov's numbers are line coverage and are never treated as, or reconciled to, the
  gate's statement coverage; `./x gate` (ADR-0012) remains the sole hard coverage gate and
  both Codecov coverage statuses are informational. (textual)

## Consequences

Easier:
- The headline coverage number becomes honest and legible: `raw` states the real
  line-coverage reality, `covered` states the accountable ~100%, and the sanctioned
  defensive residue no longer reads as negligence.
- The `covered` figure is derived from the same `internal/coverage` logic as the gate, so
  reporter and gate cannot drift on the meaning of "ignored".
- The follow-on restructuring cleanup now has a visible metric to move: shrinking the
  ignored set raises `raw` toward `covered`.

Harder / accepted trade-offs:
- CI runs a second Codecov upload step and one extra `covercheck` invocation per gate job.
- The project headline badge stays the raw (~94%) line figure; the ~100% covered number is
  only visible per-flag (PR comment, status check, badge), never as the headline. This is
  intended, but it means a casual reader sees ~94% first.
- Codecov's line coverage cannot be reconciled to the gate's statement coverage; the repo
  now carries two coverage vocabularies (line for Codecov, statement for the gate) that
  must be kept distinct in docs to avoid confusion.
- `codecov.yml` and the dual upload add repo surface that is not awf-rendered and must be
  maintained by hand.

Ruled out (for now):
- Making Codecov report statement coverage / forcing its number to 96.5% (impossible;
  Codecov is line-based).
- Re-implementing `coverage-ignore` as a `codecov.yml` path/line exclusion (Decision 4:
  would duplicate the convention and invite drift).
- A hard Codecov coverage status that could block a merge (Decision 4: double-gates on a
  divergent metric; the gate is the enforcer).
- Promoting two-report coverage into the rendered awf standard (out of scope, deferred).

Downstream work unblocked: an implementation sequence covering: add the
`internal/coverage` filtered-profile emitter with tests and the
`// invariant: covered-profile-honors-ignores` backing; add the `covercheck
--emit-filtered` mode with tests; add `codecov.yml`; wire the dual flagged upload in
`.github/workflows/ci.yml`; add the two README badges; and document the line-vs-statement
distinction in `docs/testing.md`. When this ADR flips to Implemented, the same commit backs
the tagged invariant, records the line-vs-statement two-report distinction in
`docs/testing.md` (Decision 6), and regenerates `docs/decisions/ACTIVE.md` via `./x sync`.
No `AGENTS.md` (`.awf/agents-doc.yaml`) Invariants bullet is owed: the reporting parity is
CI-side and the contributor-facing coverage rules that bind everyday changes are already the
existing ADR-0012 and ADR-0063 bullets; the `covered-profile-honors-ignores` invariant stays
greppable in source and documented in `docs/testing.md`. No `docs/decisions/README.md` index
row is owed (this repo's README is a how-to guide; `ACTIVE.md` is the generated index). A
separate, later effort
(not gated by this ADR) removes the ~10 genuinely-eliminable `coverage-ignore` markers
(the redundant-re-read cluster in `cmd/awf/list_add.go`) to raise the `raw` figure.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep one number, exclude ignored files via `codecov.yml` `ignore:` | Codecov ignore is path-granular, not line-granular; it cannot express "this one branch on this line". It would also re-implement the convention outside `internal/coverage`, inviting drift, and still not reconcile line-vs-statement. |
| Teach Codecov the `coverage-ignore` convention directly | Codecov has no per-line ignore driven by arbitrary source comments; the only mechanism it accepts is a filtered coverprofile, which is exactly Decision 1. |
| Filter the profile with an inline shell/awk step in CI | The ignored-block logic already exists and is tested in `internal/coverage`; a second bash implementation would duplicate and could drift from the gate's definition. Reusing the package keeps one source of truth. |
| Report only the `covered` (~100%) number | Hides the honest reality and removes the signal the restructuring cleanup is meant to move; a permanent ~100% badge is uninformative. |
| Report only a "fixed" raw number and drop the two-report idea | Leaves the sanctioned defensive residue reading as a coverage miss with no way to distinguish it from real gaps. |
| Make the Codecov `covered` status a hard merge gate | Double-gates coverage on a line metric that diverges from the statement gate; produces confusing second failures. `./x gate` is the single enforcer (Decision 4). |
