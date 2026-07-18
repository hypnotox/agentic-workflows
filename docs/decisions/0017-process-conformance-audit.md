---
status: Implemented
date: 2026-06-26
supersedes: []
superseded_by: ""
tags: [audit-rules, commit-conformance]
related: [8, 12, 127]
domains: [tooling, config]
---
# ADR-0017: Process-conformance audit (`awf audit`)

## Context

awf verifies *code* rigorously: the gate, 100% statement coverage (ADR-0012), rendered-file
drift, and invariant-backing all wrap the probabilistic agent in deterministic checks. It does not
verify the *agent's process*: nothing confirms the agent actually followed the workflow it was
handed (Conventional Commits, one concern per commit, an ADR for a load-bearing change, docs
travelling with the change). The field consensus this project is benchmarked against treats evals
and observability as the binding constraint ("verification is the new bottleneck") and the
determinism/drift/provenance pillar is precisely where awf already leads. Extending that pillar from
*artifacts* (the working tree) to *history* (the commit log) is the largest available gap.

The broader "verify the agent" goal has two halves: a **process-conformance audit** derived from git
history (this ADR) and a **golden-task eval corpus** run against a live agent (deferred: it needs
agent execution and a scoring harness, a separate concern from a renderer/drift-checker).

Four constraints shape the audit:

- **Git history is immutable.** A pushed commit's message cannot be fixed, so the audit must not
  gate on history: it would block permanently on un-amendable commits.
- **No imposed branching strategy.** The user requires that awf not dictate how adopters branch, so
  the audit cannot live in a pre-push hook or assume a particular base-branch workflow as policy.
- **Language-agnostic standard (ADR-0008).** The audit renders into any ecosystem; anything
  ecosystem-specific (dependency-manifest filenames) must be a config-overridable default, not
  hardwired.
- **100% coverage gate (ADR-0012).** Whatever reads git must be testable hermetically, without a
  `git` binary in CI and without `coverage-ignore` escape hatches on every git-failure branch.

The audit is non-redundant with existing checks: drift verifies the *current* tree matches its
sources and invariant-backing verifies ADR contracts are tested; neither inspects *per-commit
process properties over history*, which is the audit's sole concern.

## Decision

1. **New `awf audit` subcommand backed by a new `internal/audit` package**, modelled on
   `internal/invariants` (a `Run` returning `[]Finding`, a sibling `Project.Audit()` method, a
   `cmd/awf/audit.go` `runAudit`). It is **standalone and advisory**: it exits non-zero only when at
   least one `Error`-severity finding is present, and exits 0 (still printing) on `Warning`-only. It
   is wired into **no** hook and **not** folded into `awf check` or the gate.

2. **Git access via `github.com/go-git/go-git/v5`** (a new direct dependency, recorded here). Chosen
   for hermetic **in-memory test repos** (which make the 100% coverage gate tractable without a
   `git` binary in CI) and a typed API covering commit message, per-file name-status, add/delete
   stats, and file-content-at-commit in one library, avoiding hand-rolled plumbing-output parsing.

3. **Audit range = commits reachable from `HEAD` but not from the base branch** (via merge-base),
   with the base configurable. An empty range (branch even with base) is clean. Not-a-git-repo, an
   unresolvable base ref, and unrelated histories are **hard errors** from the command, not findings.

4. **Rule set, curated for non-redundancy with drift.** Each rule encodes a process property that
   only the commit history reveals:

   | Severity | Rule | Fires when |
   |---|---|---|
   | Error | `conventional-commits` | a range commit's subject is not `type(scope)?: subject`, or its type ∉ `allowedTypes`, or its scope ∉ `allowedScopes` (when non-empty), or its length > `subjectMaxLength` |
   | Error | `adr-status-cochange` | a commit changes an ADR file's `status:` frontmatter but does not also change `ACTIVE.md` |
   | Warn | `dependency-adr` | a `dependencyManifests` file changed somewhere on the branch but no ADR file changed on the branch |
   | Warn | `plan-for-large-change` | branch-aggregate **non-generated** changed-lines exceed `diffThreshold` but no file under `plansDir` was touched |

   Merge commits (more than one parent) are exempt from `conventional-commits`: a merge subject
   ("Merge branch ...") is not a Conventional Commit and the audit imposes no branching strategy
   (constraint 2), so flagging merges would penalise a legitimate merge-based workflow.

   Rules deliberately excluded as redundant with working-tree drift: per-commit rendered-file
   hand-edit detection and stale-`ACTIVE.md`-now.

5. **New `Audit *AuditConfig` config block** (strict `KnownFields` parse, validated in
   `config.Validate`): `baseBranch` (default `main`), `allowedTypes` (default the conventional set),
   `allowedScopes` (default empty = any scope; this repo sets `[adr, awf, plans]`), `subjectMaxLength` (default
   72), `dependencyManifests` (the broad cross-ecosystem default set), `diffThreshold` (default 400).
   String lists are case-insensitive. An empty `allowedTypes` or `allowedScopes` means *accept any*
   (the corresponding membership sub-check passes, not the whole rule disabled); an empty
   `dependencyManifests` disables `dependency-adr`; a `0` `subjectMaxLength` skips the length
   sub-check and a `0` `diffThreshold` disables `plan-for-large-change`.
   `dependencyManifests` reuses the ADR-0008 basename-glob validation (`filepath.Match` on basename;
   reject path separators and malformed patterns).

6. **Generated-file exclusion is caller-supplied.** `Project.Audit()` iterates the lock's
   `Files` keys and passes the generated-path set into `audit.Run`, so the threshold counts only
   non-generated lines and the `audit` package stays decoupled from `internal/manifest`.
   `Project.Audit()` likewise supplies the layout paths the rules key off (the ADR directory and
   `ACTIVE.md` (for `adr-status-cochange`) and `plansDir` (for `plan-for-large-change`)) from
   `p.layout()`, so the `audit` package stays decoupled from `internal/project`'s layout too. An
   *ADR file* is one matching the `NNNN-*.md` ADR-naming convention under the ADR directory, so
   `ACTIVE.md`, `README.md`, and `template.md` are not themselves treated as ADRs.

7. **The terminal reviewer runs the audit.** `awf-reviewing-impl` gains a catalog-declared section
   that runs `awf audit` and routes its findings through the existing review-discipline spine.

## Invariants

Checkable contracts, each backed by an `internal/audit` / `internal/config` test added at
implementation (`// invariant: <slug>`, `*.go` per `invariants.sources`):

- `invariant: audit-conventional-commits`: a range commit whose subject is malformed, carries a
  disallowed type or scope, or exceeds `subjectMaxLength` yields an `Error` finding; a conforming
  commit yields none.
- `invariant: audit-adr-status-cochange`: a commit that changes an ADR file's `status:` frontmatter
  without also changing `ACTIVE.md` yields an `Error`; the same flip with `ACTIVE.md` co-changed
  yields none.
- `invariant: audit-dependency-warn`: a `dependencyManifests`-matching file changed on the branch with no
  ADR file changed on the branch yields a finding of severity `Warning` (never `Error`).
- `invariant: audit-plan-threshold-warn`: branch-aggregate non-generated changed-lines exceeding
  `diffThreshold` with no `plansDir` file touched yields a finding of severity `Warning`.
- `invariant: audit-warn-exit-zero`: a run whose findings are all `Warning` returns no error from
  `runAudit` (exit 0); any single `Error` finding makes it return non-zero.
- `invariant: audit-empty-range-clean`: a branch with no commits beyond the base yields zero findings.

The `dependencyManifests` basename-glob validation is the same contract already backed by ADR-0008's
`invariants-glob-basename`; it is reused, not re-tagged. The shared validator is unit-tested at the
`audit` config call site as well, so the 100% coverage gate (ADR-0012) is met without minting a new
slug.

## Consequences

- **Easier:** awf gains its first concrete "verify the agent" capability; the terminal reviewer
  surfaces workflow-conformance feedback automatically without a human remembering to ask. Adopters
  in ~12 ecosystems get the dependency warning with zero config.
- **Cost (new heavy dependency):** go-git pulls a large transitive tree into a previously
  dependency-light tool. Accepted for the testability win against the coverage gate. Pleasingly
  self-consistent: adding it is exactly the "new dependency → expect an ADR" event the audit's own
  `dependency-adr` rule warns about, and this ADR satisfies it.
- **Advisory by design:** because history is immutable and awf imposes no branching strategy, the
  audit never gates. Findings on already-pushed commits cannot be "fixed"; surfacing the policy
  decision (block or not) is left to the adopter's CI. Adopters who never run `awf audit` are wholly
  unaffected: it is outside the gate.
- **Doc-currency:** when this ADR flips to Implemented, `./x sync` regenerates `ACTIVE.md`; the new
  `awf audit` command and the `awf-reviewing-impl` catalog-section change re-render `AGENTS.md` and
  that agent from `.awf` config in the same commit; and the six `inv:` slugs must be backed before
  the flip. No `docs/decisions/README.md` row is owed: the index is the generated `ACTIVE.md`; the
  README is a how-to (ADR-0003/0004).
- **Ruled out:** gating on history; a pre-push/branch-policy hook; per-format dependency parsing
  (the warning is a manifest-changed heuristic, so a pure version bump may false-positive; hence
  `Warning` severity and wording that says so).
- **Unblocks:** the deferred golden-task eval corpus (phase 2), and a future config-driven rule
  registry if adopter demand for custom rules appears.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Shell out to `git` (os/exec) | No new dependency, but hand-rolled NUL-delimited plumbing parsing, real temp repos in every test, and `coverage-ignore` on git-failure branches; the 100% coverage gate made hermetic in-memory testing decisive. |
| Config-driven rule registry (rules as data) | Adopter-tunable, but a premature DSL maintenance surface for v1; the highest-value rules (frontmatter co-change) need real logic, not regex/threshold data. |
| Fold the audit into `awf check` / the gate | Would block permanently on immutable pushed commits and force a particular branching strategy, both explicitly rejected. |
| LLM-assisted heuristic judging (subagent per commit) | Non-reproducible and needs agent execution; it belongs to the golden-task-eval half, not a deterministic history audit. |
| Include rendered-file-hand-edit / stale-`ACTIVE.md` rules | Redundant with working-tree drift, which already catches both on the current tree. |
