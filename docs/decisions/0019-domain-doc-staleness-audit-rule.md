---
status: Proposed
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [audit, conformance, staleness]
related: [0014, 0017]
domains: [tooling]
---
# ADR-0019: Domain-Doc Staleness Audit Rule

## Context

awf checks config↔rendered drift rigorously, and the machine-checkable ADR invariants are backed
(ADR-0007/0008: a tagged `inv:` slug losing its source comment fails `awf check`). What nothing
checks is whether hand-authored prose that describes the *present* stays current as decisions land
— the 2026 "stale-spec silent misleading" failure mode, where an agent confidently trusts an
outdated narrative.

The naive realization — scan plans and ADRs for references to files or symbols that no longer
exist — is the wrong tool. Plans freeze on completion and ADR `Context` sections are written at
decision time; both are **append-only historical records**. A frozen plan or an ADR that mentions a
since-renamed package is *correct* (it records the past), so reference-scanning them produces false
positives, not signal. The machine-checkable ADR invariants are, separately, already enforced.

The one surface that (a) describes the present, (b) is hand-authored, and (c) has a *documented*
manual-refresh gap is the **domain current-state narrative** (`.awf/domains/parts/<X>/current-state.md`).
ADR-0014 auto-generates each domain doc's Decisions index but deliberately leaves the current-state
prose to a hand refresh "when a domain's position materially shifts." That refresh can silently be
skipped. The low-false-positive way to catch it is **co-change**, exactly the mechanism the existing
`adr-status-cochange` rule uses — and the advisory, git-history `awf audit` engine (ADR-0017) is its
natural home. The grounding-check confirmed the engine's rule list, `Inputs`/`AuditConfig` shape,
frontmatter parsing, generated-paths set, and hermetic test infra all support this as a localized
fifth rule with no engine, schema, or lock change.

## Decision

1. **Add a `domain-doc-staleness` rule (Severity `Warning`) to the audit engine.** For the commit
   range, when an in-range commit brings an ADR (carrying `domains: [X, …]`) to `status:
   Implemented` — the ADR is added directly as `Implemented`, or its status changes to `Implemented`
   from a non-`Implemented` prior — then for **each** domain `X` in that ADR's `domains` list, if no
   in-range commit changed `.awf/domains/parts/<X>/current-state.md`, emit one branch-level `Warning`
   per uncovered domain.

2. **Watch the source part, never the rendered doc.** The rule keys on
   `.awf/domains/parts/<X>/current-state.md` (the hand-authored narrative source). It must not key on
   the rendered `docs/domains/<X>.md`, which regenerates its Decisions index on any ADR change (it is
   in the audit's generated-paths set) and would therefore always falsely satisfy the co-change.

3. **Range-level, advisory, merges not exempt.** Like `dependency-adr` and `plan-for-large-change`,
   the rule accumulates over the range and reports once per uncovered domain (empty `Commit`), and it
   never changes the exit code (Warning). Merge commits are not exempt, mirroring the sibling
   status-detecting rule `adr-status-cochange`.

4. **Supply the source-part location as an input.** Add `Inputs.DomainsPartsDir` (value
   `.awf/domains/parts`), built by `Project.Audit()` from the config root, keeping the audit package
   decoupled from config/manifest. Add a `domainsOf(text) []string` frontmatter helper parallel to
   the existing `statusOf`.

5. **Make the rule disable-able.** Add a `*bool` toggle to `AuditConfig` (nil = enabled), following
   the established nil-means-default per-rule config semantics, so an adopter who finds the nudge
   noisy can switch it off. The rule is also naturally inert for any project that declares no domains.

## Invariants

- `inv: audit-domain-doc-staleness` — the rule emits a `Warning` for domain `X` exactly when an
  in-range commit brings an ADR with `X` in its `domains` to `status: Implemented` and no in-range
  commit changes `.awf/domains/parts/<X>/current-state.md`; it is silent when the part is co-changed,
  when the status reaches only `Accepted`/`Proposed`, and when the ADR carries no `domains`.
- The rule is advisory: it produces only `Warning` findings and never makes `awf audit` exit
  non-zero (textual; inherited from ADR-0017's `warn-exit-zero`).
- The rule keys on the source part `.awf/domains/parts/<X>/current-state.md`, never the generated
  `docs/domains/<X>.md` (textual).

## Consequences

- Closes the ADR-0014 manual-refresh gap with an advisory nudge: shipping a decision into a domain
  without refreshing that domain's narrative is now surfaced.
- Like every audit rule, it is advisory and branch-oriented — a no-op on awf's own `main` (empty
  range), and wired into no gate or hook (ADR-0017). It informs; it does not block.
- False positives are possible (an `Implemented` ADR whose domain narrative genuinely needs no
  change). They are mitigated by the `Warning` severity and the disable toggle; the human judges
  materiality.
- The deliberate rejection of content reference-scanning is recorded here so a future contributor
  does not "complete" staleness detection by adding the noisy scanner this ADR sets aside.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `Proposed → Implemented` flip regenerates `docs/decisions/ACTIVE.md` via `./x sync` in the
  same commit; no `docs/decisions/README.md` row is owed (ADR-0005).
- Adding the rule materially shifts the `tooling` domain's current state, so the `tooling`
  current-state narrative (`.awf/domains/parts/tooling/current-state.md`) is refreshed in the
  implementing range — which also dogfoods the rule itself.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Content reference-liveness scan of plans/ADRs | Plans and ADR `Context` are append-only historical records; scanning them for dead references flags correct historical prose — false positives, not signal. |
| Commit-level co-change (part refreshed in the *same* commit as the flip) | Penalizes the natural multi-commit flow where the ADR flip and the narrative refresh land in different commits of the same range. |
| Trigger on ADR added, or on `Accepted` | Premature — the decision is not yet reality. `Implemented` is the unambiguous moment the domain's current state has shifted. |
| A new standalone staleness command | The advisory, git-history, branch-scoped `awf audit` (ADR-0017) is the right home and reuses the rule engine, Inputs, and test infra; a separate command would duplicate all of it. |
| Make it an `Error` that gates | Staleness is heuristic and materiality is a judgement; an advisory `Warning` matches the audit's design and avoids blocking on a nudge. |
