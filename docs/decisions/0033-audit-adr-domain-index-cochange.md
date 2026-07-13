---
status: Implemented
date: 2026-06-29
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [audit, tooling, publication-safety]
related: [17, 19]
domains: [tooling]
---
# ADR-0033: Audit ADR→Domain-Index Co-change

## Context

A real escape this session: an ADR-proposal commit added ADRs carrying `domains:`
frontmatter and staged `docs/decisions/ACTIVE.md`, but **not** the regenerated
`docs/domains/<domain>.md` index docs. The committed snapshot was internally
inconsistent (stale domain indexes), yet every gate passed. The fix that session
was a follow-up "regenerate domain indexes" commit — exactly the kind of
after-the-fact patch the deterministic checks are supposed to prevent.

Why each existing net missed it:

- **`awf check` is git-blind.** `Project.Check` compares rendered output against the
  **on-disk** files (drift), not the **committed/staged** snapshot. The working tree
  had been `./x sync`'d, so on disk the domain indexes matched; only the staged set
  omitted them. `awf check` owns generated-artifact drift (the domain indexes are
  lock-tracked, `checkDomainDocs`), but it cannot see that they were left unstaged.
- **`domain-doc-staleness` (ADR-0019) is the wrong tool.** It targets the
  hand-authored narrative part (`.awf/domains/parts/<d>/current-state.md`), fires only
  when an ADR reaches **Implemented**, and is a Warning. The slip was on **Proposed**
  ADRs and concerned the **generated index**, a different file on a different trigger.
- **`adr-status-cochange` (ADR-0017) is half the rule.** It requires an ADR add or
  status flip to co-change `ACTIVE.md` — but the *same* ADR frontmatter (`status`,
  `domains`, grouped by status) also regenerates each `docs/domains/<d>.md` index, and
  nothing requires those. ADR→generated-index co-change was enforced for one index
  (`ACTIVE.md`) and silently not for the others.

Grounding discoveries that shape the design:

- `ruleADRStatusCochange` (`internal/audit/audit.go`) is **per-commit**, **unconditional**
  (no config toggle), fires on `ch.Action == Added || statusOf(OldText) != statusOf(NewText)`,
  requires `in.ActiveMd` touched in the same commit, and emits an `Error`. Helpers
  `domainsOf(text)`, `isADRFile(path, dir)`, `statusOf(text)` already exist.
- `layout().DomainsDir` is `docs/domains`; the per-domain index renders to
  `<DomainsDir>/<domain>.md` for each **configured** domain only and is lock-tracked.
  `audit.Inputs` does not yet carry the rendered domains-index dir; `Project.Audit`
  assembles `Inputs` from the layout.
- Adding or flipping an ADR in a configured domain **always** changes that domain's
  index (the index lists every ADR in the domain, grouped by status), so requiring the
  co-change does not false-positive. An ADR domain outside the configured set has no
  index doc and is handled by `ruleUndocumentedDomain` (ADR-0019).

## Decision

1. **Extend `ruleADRStatusCochange` to also require domain-index co-change.** On the
   triggers it already fires (an ADR **added**, or its `status:` **changed**), the same
   commit must also change `docs/domains/<d>.md` for every `d` in the ADR's `domains:`
   frontmatter that is a configured domain. A missing domain index yields an **Error**
   finding labelled `adr-domain-cochange` (one finding per missing index). The existing
   `ACTIVE.md` requirement (the `adr-status-cochange` finding) is unchanged.

2. **New invariant, ADR-0017 left intact.** The new contract is tagged
   `inv: audit-adr-domain-cochange` and backed by new cases in
   `TestRuleADRStatusCochange`. ADR-0017's `inv: audit-adr-status-cochange` (the
   `ACTIVE.md` contract) is untouched — its text and backing stand. One function now
   enforces two distinct, separately-tagged contracts.

3. **Wire the rendered domains-index dir.** Add `audit.Inputs.DomainsIndexDir`, set by
   `Project.Audit` from `layout().DomainsDir`. The domain-index check is a no-op when
   `DomainsIndexDir` is empty or no domains are configured, so the rule's behaviour is
   unchanged for callers that do not set it (preserving the ACTIVE.md-only contract and
   the existing unit tests).

4. **Per-commit and unconditional**, matching the `ACTIVE.md` sibling: each commit that
   introduces or flips an ADR carries its regenerated domain indexes in that same commit
   (docs-travel-with-change). No config toggle.

## Invariants

- `invariant: audit-adr-domain-cochange` — a commit that adds an ADR, or changes its `status:`,
  without also changing each `docs/domains/<d>.md` index for the ADR's configured
  domains, yields one `adr-domain-cochange` `Error` per missing index; the same change with those indexes
  co-changed yields none. When `DomainsIndexDir` is empty or no domains are configured,
  no `adr-domain-cochange` finding is emitted.

## Consequences

Easier:
- The ADR→generated-index co-change net now covers the domain indexes, not just
  `ACTIVE.md`. A commit that ships stale domain indexes is an `Error` at the
  reviewing-impl audit (which treats audit Errors as blocking) — closing the exact gap
  that required a follow-up regenerate commit this session.
- Reuses the tested co-change machinery and existing helpers; no new dependency, no new
  config surface (unconditional, like its `ACTIVE.md` sibling).

Harder / accepted trade-offs:
- It is a heuristic over the **same triggers** as the `ACTIVE.md` sibling (add /
  status-flip). An edit that changes an index without one of those triggers — e.g. a
  `domains:`-set change, a title change, or a `superseded_by` change with no status flip
  — is not caught, exactly matching the existing `ACTIVE.md` rule's scope. Widening the
  trigger set is deferred to avoid false positives on index-irrelevant frontmatter edits.
- This does **not** make `awf check` itself git/commit-aware. A commit could still in
  principle omit a non-ADR-driven generated output (e.g. a rendered skill after a
  `.awf/` part edit). The byte-precise alternative (a git-aware `awf check`) is recorded
  below and deferred.
- The audit reports per-commit over a range, so on a feature branch a historically
  non-conforming commit is flagged even if a later commit fixed it — which is accurate
  (that commit was incomplete) and consistent with the `ACTIVE.md` sibling.

Doc-currency obligations the implementing commit(s) must satisfy:
- The `tooling` domain narrative (`.awf/domains/parts/tooling/current-state.md`) gains
  the domain-index co-change rule in its enumeration of audit rules.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` and
  `docs/domains/tooling.md`.
- No `docs/decisions/README.md` row is owed — the index is the generated `ACTIVE.md`; the
  README is a how-to (ADR-0005).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Git-aware `awf check --staged` (validate the committed snapshot byte-for-byte) | Tightest fix, blocks at commit, but heavier (adds git to the check path) and, post-ADR-0032, commit-time blocking depends on an adopter-wired hook. Deferred, not precluded. |
| A new parallel rule instead of extending `ruleADRStatusCochange` | Fragments one concern — "an ADR add/flip co-changes its generated indexes" — across two rules. Extending keeps it cohesive while a distinct finding label + invariant slug keep the contracts separable. |
| Broaden ADR-0017's `audit-adr-status-cochange` invariant text to include domain indexes | Rewrites an Implemented ADR (append-only). A new slug recorded here via `related` leaves ADR-0017 intact and accurate. |
| Coarse rule: any `.awf/` authored input changed without any lock-tracked output | False-positive-prone (an input edit that produces no render change) and does not pinpoint the missing output. The actual incident was ADR-driven; this precise rule covers it. |
