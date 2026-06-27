---
status: Proposed
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [tooling, audit, workflow]
related: [0017, 0019]
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
threads through five known locations; and that go-git's status honours `.gitignore`, so build
artifacts (the gitignored `awf` binary) and other ignored files never trip it.

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

4. **Record the live-state broadening.** The audit package doc and ADR-0017's "over a branch's git
   history" framing are extended to note that one rule (`uncommitted-changes`) additionally inspects
   the live working tree; the other rules remain pure over the commit range.

## Invariants

- `inv: audit-uncommitted-changes` — when enabled, `awf audit` reports an `Error` finding if the
  working tree has any uncommitted change (tracked modification or untracked, non-ignored file).

## Consequences

- The terminal review deterministically surfaces a forgotten or partially-staged change — including
  the rendered-without-source case that prompted this ADR — instead of letting it slip into history.
- The rule adds a live-working-tree read to a package that was previously pure over commit history; it
  is isolated to `Run` and the one rule, and uses the go-git dependency the package already carries
  (no new dependency, no shell-out).
- `awf audit` run ad hoc on a deliberately-dirty tree now exits non-zero on this rule; the toggle and
  the advisory-only nature (it gates nothing directly) keep that from being disruptive.
- Coverage: the rule and its toggle need direct tests (clean tree → no finding, dirty tree → finding,
  disabled → no finding), reusing the existing `initRepo`/`commit` git-repo test helpers.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `tooling` domain narrative gains the new audit rule.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| A clean-tree check in the `awf-reviewing-impl` skill prose only | Probabilistic — the agent might skip it; awf prefers a deterministic check it can surface mechanically. |
| Pre-commit hook checking the staged tree for drift | Catches the rendered-without-source case at commit time (a hard gate), but is narrower (only drift, not a forgotten test file) and needs awf to render against staged blobs; the audit rule is generic and reuses existing wiring. Left as possible future hardening. |
| Pre-push hook refusing a dirty tree | Wrong boundary (fires on any push, including unrelated WIP) and false-positive prone; the terminal review is the meaningful end-of-implementation moment. |
| Severity `Warning` | A dirty tree at the audit's intended use is a real end-of-implementation violation, not a nudge; `Warning` would not block at reviewing-impl. |
