---
status: Proposed
date: 2026-06-29
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [tooling, commit, gate, hooks, audit]
related: [0017, 0032]
domains: [tooling]
---
# ADR-0036: Deterministic Commit-Message Gate

## Context

awf already encodes Conventional Commits rules — subject format `type(scope)?: subject`, an
allow-list of types and scopes, and a 72-char subject ceiling — in the `conventional-commits`
audit rule (`internal/audit/audit.go` `ruleConventionalCommits`, settings from
`audit.Resolve(cfg.Audit)`: `AllowedTypes`, `AllowedScopes`, `SubjectMaxLength: 72`). But that rule
runs only through `awf audit`, which by ADR-0017 is **advisory**, **range-based**, and invoked at
the *end* of the workflow chain (the reviewing-impl terminal step). So a non-conforming subject —
e.g. an 84-char one written by a review subagent — is not caught when the commit is made; it is
merely *reported* later, after it has already landed, when fixing it means rewriting history.

The missing tier is a **deterministic, blocking, commit-time** check: the commit-side analog of
`./x gate` (which blocks on tests/coverage/lint but knows nothing about the commit message). awf's
identity is "deterministic checks that wrap the probabilistic agent"; a blocking commit-message
gate is squarely one of those, and an agent committing on its own is exactly who benefits.

Two existing decisions bound the shape. **ADR-0032** removed hook *rendering* — awf ships no git
hooks; an adopter owns their hook wiring (`.githooks/` here are plain checked-in files). So the gate
must be a **command** the adopter calls from their own hook, not a rendered hook. **ADR-0017** fixes
the audit as advisory and never-gating; the new gate must therefore be a *separate* command, not a
mode of `audit`, so ADR-0017's contract is untouched. The rule itself, however, must not be
duplicated: a second copy of "72" and the type/scope lists would drift from the audit's. This ADR
keeps one rule definition behind two entry points.

A git `commit-msg` hook receives the path to the prepared message file as `$1`. That file is *raw*:
it contains `#` comment lines (and, under verbose commit, a diff below a scissors line), and the
commit may be a merge or an autosquash (`fixup!`/`squash!`/`amend!`) whose subject git itself
generated. The gate must clean and exempt these the way git does, or it will block legitimate
commits.

## Decision

1. **New `awf commit-gate` command — blocking, single-message.** It reads one commit message (the
   file path given as the sole positional argument, as a `commit-msg` hook passes `$1`; or stdin
   when no path is given), validates the subject, prints any violation to stderr, and exits
   non-zero on any violation, zero otherwise. It is the deterministic, blocking commit-time gate;
   adopters wire it into their own `commit-msg` hook.

2. **One rule, two entry points.** The per-commit Conventional Commits check is extracted from
   `ruleConventionalCommits` into a single shared function consuming a `Commit` and the resolved
   `audit.Settings`. `awf audit` (advisory, over a range) and `awf commit-gate` (blocking, one
   message) both call it — same regex, same `AllowedTypes`/`AllowedScopes`/`SubjectMaxLength`,
   sourced from the same `audit.Resolve(cfg.Audit)`. There is no second definition of the rule or
   the limit.

3. **Git-style message cleaning.** Before extracting the subject, `commit-gate` strips lines whose
   first non-whitespace character is `#` and ignores everything at or below a verbose scissors line
   (`# ------------------------ >8 ------------------------`), then takes the first non-blank line as
   the subject — mirroring git's default `commit.cleanup=strip`. The audit path, which reads
   already-parsed commit subjects, is unaffected.

4. **Merge and autosquash exemption.** `commit-gate` exempts a subject beginning with `Merge `,
   `fixup!`, `squash!`, or `amend!` — git-generated merge and autosquash subjects — and reports them
   as conforming. This extends to the message-only context the merge exemption the audit expresses
   via `Commit.IsMerge`. No commit is blocked for a subject git itself produced or will rewrite.

5. **No new config; no hook rendering.** `commit-gate` introduces no new configuration: it reads the
   existing `audit` settings, so loosening works through the existing knobs (empty `AllowedTypes`
   accepts any type; `SubjectMaxLength: 0` skips the length check). awf still renders no hook
   (ADR-0032 stands); it ships the command and an adopter wires it. This repository adds a
   checked-in `.githooks/commit-msg` calling `./x commit-gate "$1"`, alongside its existing
   `pre-commit`/`pre-push`, and a `commit-gate` case in the `./x` runner.

## Invariants

- `inv: commit-gate-shared-rule` — the Conventional Commits subject check has exactly one definition
  (a single shared function), consumed by both the `awf audit` range loop and the `awf commit-gate`
  command; neither re-implements the regex, the type/scope allow-lists, or the subject-length limit.
  Backed by a test asserting a subject the audit rejects is also rejected by `commit-gate` (and a
  conforming one accepted by both) under identical settings.
- `commit-gate` exits non-zero if and only if the cleaned, non-exempt subject violates the resolved
  `audit` settings; a clean or exempt subject exits zero. (Textual contract, backed by command-level
  tests.)

## Consequences

- The 84-char-subject class of defect is caught at commit creation — the earliest possible point —
  for anyone (human or agent) who wires the hook, instead of surfacing late in an advisory audit.
- `audit` and `commit-gate` cannot drift on what "conformant" means: the limit and the allow-lists
  live once. Extracting the shared function is a behaviour-preserving refactor of the audit; the
  zero-change to audit output is verified by its existing tests.
- The gate is opt-in by construction: it only runs where an adopter wires it. awf imposes no hook,
  preserving ADR-0032's "adopter owns hooks" boundary, and `awf audit` stays advisory per ADR-0017.
- New surface to cover under the 100% gate (ADR-0012): the shared function, the `commit-gate`
  handler, message cleaning, and each exemption/violation branch need tests or justified
  `// coverage-ignore`.
- The gate validates only the **subject**; body rules (blank line, wrapping, trailers) are out of
  scope here. The command is named and shaped as the general deterministic commit gate, so later
  per-message checks can join it without a new command — but none are added now.
- A subject git cannot pre-clean (e.g. an adopter using `commit.cleanup=verbatim` with stray `#`
  content) is the adopter's configuration choice; `commit-gate` cleans with the `strip` default and
  does not attempt to honour arbitrary per-repo cleanup modes.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Make `awf audit` itself blocking (a `--gate` flag) | Violates ADR-0017's advisory contract and conflates range-history analysis with single-commit gating; a separate command keeps both contracts clean. |
| Inline the 72-char check in `.githooks/commit-msg` (bash) | A second source of truth that drifts from `SubjectMaxLength` and ignores the type/scope rules; defeats the single-definition goal. |
| Render a `commit-msg` hook from awf | Contradicts ADR-0032 (awf renders no hooks; adopters own hook wiring). awf ships the command instead. |
| Validate a committed range (like audit) wired into `pre-push` | Catches violations only after the commit exists; the message-file form blocks at creation, the earliest point, and is the natural `commit-msg` integration. |
| Add a per-rule `disabled` flag for conventional-commits | Unneeded — existing settings already loosen the rule (empty types, zero length), and not wiring the hook disables the gate entirely. |
