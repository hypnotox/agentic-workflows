---
status: Implemented
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [tooling, audit, workflow]
related: [17, 19]
domains: [tooling]
---
# ADR-0025: Uncommitted-Changes Audit Rule

## Context

A recurring implementation failure mode is finishing a change with the working tree not fully
committed — most insidiously, committing a rendered file (e.g. `AGENTS.md`) while leaving its source
(the template or `.awf/` config that produced it) uncommitted. The pre-commit `awf check` does not
catch this: it renders from the *working tree*, which still holds the modified-but-unstaged source,
so the rendered output matches and the check passes. The drift exists only in the committed tree, and
the forgotten change sits in the working tree until someone notices.

This is exactly the class of probabilistic-agent slip that awf's deterministic checks exist to catch.
The natural enforcement point already exists: `awf-reviewing-impl` (the terminal step of an
implementation) runs `awf audit` and treats its `Error` findings as blocking (ADR-0017). The audit is
the project's deterministic conformance reporter, and ADR-0019 established the precedent of adding new
conformance rules to it. What it lacks is a rule asserting the simplest end-of-implementation
invariant: the working tree is clean.

The grounding-check confirmed `internal/audit` runs git through go-git (not a shell), exposing a typed
`Worktree().Status()`; that `Run(repoRoot, in)` is the clean insertion point (it holds the repo root,
where the pure `evaluate(commits, in)` does not); that the toggle pattern of ADR-0019's domain rules
threads through five known locations; and that go-git's `Worktree().Status()` honours the
repository's `.gitignore` and `.git/info/exclude`, so build artifacts (the gitignored `awf` binary)
and other repo-ignored files never trip it (the global-`core.excludesFile` gap is recorded under
Consequences).

## Decision

1. **Add an `uncommitted-changes` audit rule.** It emits a single branch-level `Error` finding (empty
   commit hash) when the working tree is not clean, with a detail reporting the count of tracked
   changes and untracked files. It reads live working-tree state via go-git's `Worktree().Status()`
   (honouring `.gitignore`), so it is evaluated in `Run` — which has the repo root — alongside, but
   distinct from, the commit-history rules in `evaluate`. Being a live-state check, it fires
   independent of the commit range (a clean range with a dirty tree is still flagged).

2. **Severity is `Error`.** The rule's purpose is the end-of-implementation gate `awf-reviewing-impl`
   surfaces, where a dirty tree means implementation changes were left uncommitted — the failure this
   guards. The audit remains advisory and is never wired into the gate (ADR-0017), so the `Error`
   blocks nothing on its own; it is the reviewing-impl step that treats it as blocking.

3. **Toggle via `audit.uncommittedChanges` (default true).** A new `*bool` mirrors ADR-0019's
   `domainDocStaleness`/`undocumentedDomain` toggles, threaded through `config.AuditConfig`,
   `config.AuditSettings`, `config.ResolveAudit`, `audit.Inputs`, and `project.Audit`. A nil value
   means the default (true); an adopter who runs `awf audit` ad hoc against a deliberately-dirty tree
   can disable it.

4. **Record the live-state broadening.** The audit package doc comment (`internal/audit/audit.go`)
   is updated to note that one rule (`uncommitted-changes`) additionally inspects the live working
   tree; the other rules remain pure over the commit range. ADR-0017's body is frozen (append-only
   once Implemented), so it is **not** edited — this ADR is the record of the broadening, linked via
   `related: [0017]`. Because the live-state rule is range-independent it also qualifies ADR-0017's
   `audit-empty-range-clean` invariant: an empty range still yields zero *history-derived* findings,
   but a dirty tree on that same empty range now produces this rule's `Error` (intended). The
   existing empty-range test stays green because its worktree is clean.

## Invariants

- `invariant: audit-uncommitted-changes` — when enabled, `awf audit` reports an `Error` finding if the
  working tree has any uncommitted change: a tracked modification, or an untracked file not matched
  by the repository's `.gitignore` or `.git/info/exclude`. "Not matched" here is exactly what go-git's
  `Worktree().Status()` reports (see the Consequences note on the global-`core.excludesFile` gap), so
  the contract is mechanically checkable against `Status()` output, not against full `git status`
  semantics.

## Consequences

- The terminal review deterministically surfaces a forgotten or partially-staged change — including
  the rendered-without-source case that prompted this ADR — instead of letting it slip into history.
- The rule adds a live-working-tree read to a package that was previously pure over commit history; it
  is isolated to `Run` and the one rule, and uses the go-git dependency the package already carries
  (no new dependency, no shell-out).
- `awf audit` run ad hoc on a deliberately-dirty tree now exits non-zero on this rule; the toggle and
  the advisory-only nature (it gates nothing directly) keep that from being disruptive.
- **go-git ignore-scope gap.** `Worktree().Status()` consults the repository's `.gitignore` and
  `.git/info/exclude`, but **not** the user's global `core.excludesFile` (e.g. `~/.gitignore`) or
  `/etc/gitconfig`. A file ignored only globally shows as untracked and would trip the rule even
  though `git status` stays silent — a live case in this repo, whose `.gitignore` carries `!CLAUDE.md`
  precisely because the user globally ignores `CLAUDE.md`. The toggle and the advisory nature mitigate
  it; loading the global/system exclude patterns into the worktree matcher to fully mirror `git
  status` is possible future hardening if the false positives prove disruptive.
- **Edge cases.** The rule runs only after `Collect` succeeds, so not-a-git-repo stays a hard command
  error, never a finding. A bare or worktree-less repo (no `Worktree()`) is outside `awf audit`'s
  intended use and yields no live-state finding; the `Worktree()`-error branch is covered or
  `coverage-ignore`d to satisfy the 100% gate (ADR-0012).
- Coverage: the rule and its toggle need direct tests (clean tree → no finding, dirty tree → finding,
  disabled → no finding), reusing the existing `initRepo`/`commit` git-repo test helpers.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `tooling` domain narrative gains the new audit rule.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.
- No `docs/decisions/README.md` row is owed — the index is the generated `ACTIVE.md` (ADR-0005).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| A clean-tree check in the `awf-reviewing-impl` skill prose only | Probabilistic — the agent might skip it; awf prefers a deterministic check it can surface mechanically. |
| Pre-commit hook checking the staged tree for drift | Catches the rendered-without-source case at commit time (a hard gate), but is narrower (only drift, not a forgotten test file) and needs awf to render against staged blobs; the audit rule is generic and reuses existing wiring. Left as possible future hardening. |
| Pre-push hook refusing a dirty tree | Wrong boundary (fires on any push, including unrelated WIP) and false-positive prone; the terminal review is the meaningful end-of-implementation moment. |
| Severity `Warning` | A dirty tree at the audit's intended use is a real end-of-implementation violation, not a nudge; `Warning` would not block at reviewing-impl. |
