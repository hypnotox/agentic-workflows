---
status: Implemented
date: 2026-07-01
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [check, drift, skills, adoption]
related: [0013, 0020, 0022, 0028, 0034, 0045]
domains: [rendering, config]
---
# ADR-0046: Skill-reference integrity

## Context

Rendered artifacts reference skills by name — `{{ .prefix }}-<skill>` handoffs are the
connective tissue of the workflow chain — but nothing verifies a referenced skill is
actually enabled. Two failure modes exist today:

- **Broken by default.** `templates/agents-doc/AGENTS.md.tmpl` unconditionally lists
  `tdd`, `bugfix`, and `debugging` as task skills, yet all three are non-core
  ([ADR-0022](0022-curated-init-default.md) curated init enables core only) — a default
  install's agent guide points at three skills that do not exist, violating the guide's own
  "disable them as a unit" warning. The non-core skills also cross-reference each other
  (`bugfix`→`tdd`/`debugging`, `debugging`→`bugfix`/`tdd`), so enabling one without the others
  produces the same dead handoffs.
- **Unchecked by design gap.** The dead-reference check
  ([ADR-0020](0020-dead-reference-check.md), `checkDeadRefs`,
  `internal/project/check.go:288-308`) covers inline markdown links only; skill-name
  references are plain prose tokens it never sees.

Grounding discoveries that shape the design:

- A sweep counts 116 `{{ .prefix }}-<skill>` references across 17 templates, concentrated
  in the agent guide and the 10 core chain skills (`AGENTS.md.tmpl` 16,
  `subagent-driven-development` 11, `brainstorming`/`reviewing-adr`/`executing-plans` 9
  each, …). Conditionalizing them all would be large churn and would force handoff prose to be
  written vaguely ("if a review skill is enabled…"). Non-core references live only in the
  non-core skills themselves plus `AGENTS.md.tmpl`.
- The render context (`internal/project/render.go:34-42`) exposes exactly
  `prefix`/`vars`/`data`/`layout`/`version` — templates cannot see which skills are enabled,
  so no template can conditionalize a cross-skill reference today. A new `skills` key
  collides with nothing; `data["docs"]` (render.go:179) is the extension precedent.
- The enabled set and the rendered set differ: a doc-gated skill can sit in the enable array
  yet render nothing (`roadmap-graduation` with `requiresDoc: roadmap`,
  `inv: doc-gated-skill-suppressed`, [ADR-0013](0013-doc-cross-references-via-layout.md)).
  References must be judged against what is actually on disk.
- Prefix-adjacent tokens are not all skill references: `{{ .prefix }}-specific` in the
  debugging skill renders as `awf-specific`, and docs mention `awf-bootstrap.sh`. Naive
  `<prefix>-<word>` matching would false-positive.
- The check scans rendered content, which includes raw convention-part bodies
  ([ADR-0034](0034-convention-parts-are-raw-input.md)) — a project part that names a disabled
  skill fails the check with no template-side fix. ADR-0020 accepts the identical exposure
  for dead links.

The user chose machine enforcement over maximal flexibility: a partially-trimmed core chain
should fail loudly, not render vague prose around missing steps.

## Decision

1. **Enabled-skills render context.** The template context gains a `skills` key: the set of
   skill names whose files exist on disk under awf's model — enabled skills minus doc-gate-
   suppressed ones, keeping `local`-declared ones (hand-maintained but present) even where a
   doc gate would suppress the render. Templates use it to conditionalize references whose
   target is legitimately optional. The effective set participates in `artifactConfigHash`
   for every artifact whose assembled template references `.skills` — mirroring the
   referenced-vars projection — so an enable-array change flags dependent artifacts stale
   instead of leaving `check` clean over an out-of-date render (the
   `catalog-data-in-confighash` precedent,
   [ADR-0045](0045-out-of-box-render-completeness.md)).

2. **Conditionalization is scoped to non-core references only.** The non-core skills'
   cross-references (`bugfix`↔`tdd`↔`debugging`) and the per-name task-skill list in
   `AGENTS.md.tmpl` render a name only when that skill is in `skills`. The agent guide's
   chain prose (the canonical-chain block and the chain-skills sentence, whose presentation
   [ADR-0028](0028-workflow-chain-adr-first-visible-resync.md) settled) renders as a unit,
   gated on the chain's entry skill (`brainstorming`); the task-skills clause anchors on the
   core `adr-lifecycle` and is omitted entirely when it is disabled. A chain-less config
   (no chain skills at all) is therefore legal and its guide simply omits the chain; a
   *partially* trimmed chain still renders the prose and hard-fails via item 4 — the
   individual chain handoffs inside the prose and inside the chain skills stay
   unconditional. *(Amended while Proposed during implementation: the original "chain list
   stays unconditional" wording made every chain-less config an illegal state, which was
   never the intent.)*

3. **Failing skill-reference check.** `awf check` gains a `dead-skill-reference` drift kind:
   scan the same managed rendered markdown set as ADR-0020 for `<prefix>-<name>` tokens
   where `<name>` is a *catalog-known or local-declared* skill name, and fail when that name
   is not in the effective set of item 1. Tokens whose `<name>` matches no known skill are
   ignored (the `awf-specific` guard). Matching is whole-token: a scanned token is the maximal
   `<prefix>-`-anchored word run, compared for exact membership against the known set — so
   `<prefix>-reviewing-plan-resync` is one reference to `reviewing-plan-resync`, never a
   substring hit on `reviewing-plan`. References inside fenced code blocks are treated the
   same as ADR-0020 treats links (skipped).

4. **A partially-trimmed core chain hard-fails by design.** Disabling a core chain skill
   while other enabled artifacts still reference it is a failing state — this machine-
   enforces the agent guide's existing "disable them as a unit" rule. The escape hatch for a
   deliberate trim is the existing override surface: replace the referencing section with a
   convention part that drops the reference (or disable the referencing artifact). No
   suppression flag is added.

## Invariants

- `inv: skill-ref-dead-fails` — a managed rendered artifact referencing a known skill name
  outside the effective rendered set produces a failing `awf check` finding.
- `inv: skill-ref-unknown-ignored` — a `<prefix>-<word>` token whose word matches no
  catalog or local skill name produces no finding.
- `inv: skills-context-effective-set` — the render context's `skills` set equals enabled
  skills minus doc-gate-suppressed, with `local`-declared skills always kept.
- `inv: skills-set-in-confighash` — a change to the skills enable array changes the lock
  `configHash` of every artifact whose assembled template references `.skills`, so
  `awf check` flags those artifacts stale.
- `inv: curated-init-skill-refs-clean` — a default curated `awf init` render passes the
  check with zero `dead-skill-reference` findings (backed on the empty-init regression
  surface of [ADR-0045](0045-out-of-box-render-completeness.md)).

## Consequences

Easier:
- Dead handoffs can no longer ship silently — the default install's agent guide stops
  pointing at nonexistent skills, and future template edits that add a reference to a
  non-enabled skill are caught at `check` time like any other drift.
- "Disable them as a unit" graduates from prose warning to enforced contract.

Harder / accepted trade-offs:
- An adopter who trims a core chain skill gets a failing check until they re-enable it,
  disable the unit, or override the referencing sections — intended friction, but friction.
- Raw convention parts and domain-doc narratives naming a disabled skill fail with no
  template-side fix (same accepted exposure as ADR-0020 dead links).
- The scanner is prefix-anchored: it cannot catch a reference that spells a skill name
  without the prefix ("the tdd skill"), and it goes quiet for names dropped from the catalog
  entirely. Accepted — known-name matching is what makes the check false-positive-free.
- Templates conditionalizing per-name lists get more intricate (punctuation/empty handling),
  covered by golden tests under the 100% gate.

Doc-currency obligations the implementing commit(s) must satisfy:

- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.
- The new failing check gains an AGENTS.md invariants entry (the `data.invariants` list in
  `.awf/agents-doc.yaml`), citing this ADR, in the implementing range.
- The check and the `skills` render context materially shift the `rendering` and `config`
  domain narratives; both are refreshed in the implementing range.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Conditionalize all ~116 cross-references | Large churn; forces chain handoff prose into vagueness; the failing check makes it unnecessary. |
| Advisory (non-failing) skill-ref findings | A dead handoff is a broken agent guide — the same severity class as ADR-0020's dead links, which fail. |
| Scan template sources instead of rendered output | Misses raw parts and local content and diverges from the ADR-0020 rendered-content model. |
| Keep the prose-only "disable as a unit" warning | It already shipped a broken default install; prose demonstrably does not enforce. |
| A `trimmedChain: true` suppression flag | Adds a config knob whose only purpose is to un-enforce the invariant; part overrides already provide a deliberate escape. |
