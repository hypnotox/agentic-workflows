---
status: Implemented
date: 2026-06-27
tags: [domain-staleness, audit-rules]
related: [14, 17]
domains: [tooling]
---
# ADR-0019: Domain-Doc Currency Audit Rules

## Context

awf checks config↔rendered drift rigorously, and the machine-checkable ADR invariants are backed
(ADR-0007/0008: a tagged `inv:` slug losing its source comment fails `awf check`). What nothing
checks is whether hand-authored prose that describes the *present* stays current as decisions land:
the 2026 "stale-spec silent misleading" failure mode, where an agent confidently trusts an
outdated narrative.

The naive realization (scan plans and ADRs for references to files or symbols that no longer
exist) is the wrong tool. Plans freeze on completion and ADR `Context` sections are written at
decision time; both are **append-only historical records**. A frozen plan or an ADR that mentions a
since-renamed package is *correct* (it records the past), so reference-scanning them produces false
positives, not signal. The machine-checkable ADR invariants are, separately, already enforced.

The surface that (a) describes the present, (b) is hand-authored, and (c) has a *documented*
manual-refresh gap is the **domain current-state narrative** (`.awf/domains/parts/<X>/current-state.md`).
ADR-0014 auto-generates each domain doc's Decisions index but deliberately leaves the current-state
prose to a hand refresh "when a domain's position materially shifts." That gap has two failure
modes: the narrative of an existing domain goes **stale** when a decision lands without refreshing
it; and a domain referenced by ADRs may have **no narrative at all** (the ADR carries a `domains`
tag the project never enabled as a domain doc). Both are forms of domain-doc *currency*, and both
are detectable from ADR `domains` frontmatter via the advisory, git-history `awf audit` engine
(ADR-0017): the low-false-positive co-change mechanism the existing `adr-status-cochange` rule
already uses. The grounding-check confirmed the engine's rule list, `Inputs`/`AuditConfig` shape,
frontmatter parsing, generated-paths set, and hermetic test infra all support this as localized new
rules with no engine-architecture, schema-version, or lock-format change; the added
`Inputs`/`AuditConfig` fields are additive and backward-compatible, needing no `awf upgrade`
(consistent with the additive-optional-field precedent of ADR-0014/0017).

## Decision

Add **two** advisory `Warning` rules to the audit engine, both keyed on ADR `domains` frontmatter
and the project's configured domain set (`config.Domains`). Each is range-level, advisory (never
changes the exit code), and does not exempt merge commits, mirroring the sibling status-detecting
rule `adr-status-cochange`.

1. **`domain-doc-staleness`.** For the commit range, when an in-range commit brings an ADR (carrying
   `domains: [X, ...]`) to `status: Implemented` (the ADR is added directly as `Implemented`, or its
   status changes to `Implemented` from a non-`Implemented` prior), then for each domain `X` in
   that ADR's `domains` list **that is also a configured domain** (`X ∈ config.Domains`), if no
   in-range commit changed `.awf/domains/parts/<X>/current-state.md`, emit one branch-level
   `Warning` per uncovered domain. Domains not in `config.Domains` are handled by Rule 2, not this
   rule.

2. **`undocumented-domain`.** For the commit range, when an in-range commit adds or changes an ADR
   tagged with a domain `X` that is **not** a configured domain (`X ∉ config.Domains`, so no
   `docs/domains/<X>.md` renders), emit one branch-level `Warning` per such domain: the project has
   a decision filed under `X` but no domain doc for it; consider adding `X` to `config.Domains` and
   authoring its current-state narrative. No status trigger: merely referencing an unconfigured
   domain in a committed ADR is the signal.

3. **Watch the source part, never the rendered doc** (Rule 1). The staleness rule keys on
   `.awf/domains/parts/<X>/current-state.md` (the hand-authored narrative source). It must not key
   on the rendered `docs/domains/<X>.md`, which regenerates its Decisions index on any ADR change
   (it is in the audit's generated-paths set) and would therefore always falsely satisfy the
   co-change.

4. **Supply the configured-domain set and source-part location as inputs.** Add
   `Inputs.ConfiguredDomains` (= `config.Domains`) and `Inputs.DomainsPartsDir` (value
   `.awf/domains/parts`), both built by `Project.Audit()` from the config root, keeping the audit
   package decoupled from config/manifest. Add a `domainsOf(text) []string` frontmatter helper
   parallel to the existing `statusOf`.

5. **Make each rule independently disable-able.** Add one `*bool` toggle per rule to `AuditConfig`
   (nil = enabled), following the established nil-means-default per-rule config semantics, so an
   adopter can silence either nudge. Both rules are also naturally inert for any project that
   declares no domains and tags no ADR with one.

## Invariants

- `invariant: audit-domain-doc-staleness`: the staleness rule emits a `Warning` for domain `X` exactly
  when an in-range commit brings an ADR with `X` in its `domains` to `status: Implemented`, `X` is a
  configured domain, and no in-range commit changes `.awf/domains/parts/<X>/current-state.md`; it is
  silent when the part is co-changed, when the status reaches only `Accepted`/`Proposed`, when `X`
  is unconfigured, and when the ADR carries no `domains`.
- `invariant: audit-undocumented-domain`: the undocumented-domain rule emits a `Warning` for domain `X`
  exactly when an in-range commit adds or changes an ADR whose `domains` includes `X` and `X` is not
  in `config.Domains`; it is silent for configured domains and for ADRs with no `domains`.
- Both rules are advisory: they produce only `Warning` findings and never make `awf audit` exit
  non-zero (textual; inherited from ADR-0017's `warn-exit-zero`).
- The staleness rule keys on the source part `.awf/domains/parts/<X>/current-state.md`, never the
  generated `docs/domains/<X>.md` (textual).

## Consequences

- Closes the ADR-0014 manual-refresh gap from both sides: a decision landing in a documented domain
  without a narrative refresh is surfaced (Rule 1), and a domain accruing decisions with no doc at
  all is surfaced (Rule 2).
- Like every audit rule, both are advisory and branch-oriented: a no-op on awf's own `main` (empty
  range), and wired into no gate or hook (ADR-0017). They inform; they do not block.
- False positives are possible (an `Implemented` ADR whose domain narrative genuinely needs no
  change; an intentionally untracked domain tag). They are mitigated by the `Warning` severity and
  the per-rule disable toggles; the human judges materiality.
- The deliberate rejection of content reference-scanning is recorded here so a future contributor
  does not "complete" staleness detection by adding the noisy scanner this ADR sets aside.

Doc-currency obligations the implementing commit(s) must satisfy:

- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync` in the
  same commit; no `docs/decisions/README.md` row is owed (ADR-0005).
- Adding the rules materially shifts the `tooling` domain's current state, so the `tooling`
  current-state narrative (`.awf/domains/parts/tooling/current-state.md`) is refreshed in the
  implementing range, which also dogfoods Rule 1.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Content reference-liveness scan of plans/ADRs | Plans and ADR `Context` are append-only historical records; scanning them for dead references flags correct historical prose: false positives, not signal. |
| Staleness fires for *every* domain in an ADR (ignore `config.Domains`) | An ADR tagged with an unconfigured/typo'd domain would warn "refresh X's narrative" when X has no doc or narrative at all: a misleading false positive. Restricting Rule 1 to configured domains and adding Rule 2 handles the unconfigured case constructively. |
| Undocumented-domain as a repo-wide count threshold (`≥ N` ADRs tag X) | More signal, but it is a current-state count, not range-scoped; it would force the audit to scan all ADRs and carry repo-state inputs, departing from the strictly range-scoped model. The range-based "an ADR introduces an unconfigured-domain tag" trigger is audit-native and sufficient as a nudge. |
| Commit-level co-change (part refreshed in the *same* commit as the flip) | Penalizes the natural multi-commit flow where the ADR flip and the narrative refresh land in different commits of the same range. |
| Trigger staleness on ADR added, or on `Accepted` | Premature: the decision is not yet reality. `Implemented` is the unambiguous moment a domain's current state has shifted. |
| A new standalone staleness command | The advisory, git-history, branch-scoped `awf audit` (ADR-0017) is the right home and reuses the rule engine, Inputs, and test infra; a separate command would duplicate all of it. |
| Make either rule an `Error` that gates | Staleness and domain coverage are heuristic and materiality is a judgement; an advisory `Warning` matches the audit's design and avoids blocking on a nudge. |
