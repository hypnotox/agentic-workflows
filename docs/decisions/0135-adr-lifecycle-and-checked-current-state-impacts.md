---
status: Proposed
date: 2026-07-19
tags: [adr-lifecycle, audit-rules, commit-gate, cross-references]
related: [17, 20, 25, 28, 33, 36, 42, 73, 92, 111, 116, 120, 127, 128, 129, 130, 131, 133, 134]
domains: [adr-system, tooling]
---
# ADR-0135: ADR Lifecycle and Checked Current-State Impacts

## Context

ADR-0133 separates historical rationale from current authority, and ADR-0134 gives active claims stable identities. A decision can become history safely only when its intended effects have reached those claims. Review prose alone cannot distinguish a forgotten current-state update from an intentional decision with no lasting normative effect.

A structural handshake can make the intent explicit. The ADR names each claim it intends to add, update, or remove; the resulting claim names the ADR as origin or revision. Static checking can validate the resulting graph, while a Git before-and-after comparison can verify that the declared mutation actually occurred during the implementation transition. Neither check claims that the new prose faithfully expresses the rationale, which remains a review responsibility.

The existing four-state lifecycle treats Implemented ADRs as continuing active guidance and uses Superseded plus anchor coverage to express later changes. That state machine no longer matches historical ADRs. The new lifecycle needs a terminal state for decisions that stop before implementation, a precise freeze boundary, and an index that separates in-flight work from history without reconstructing currentness.

## Decision

1. New-format ADRs have frontmatter containing exactly `format: current-state-v1`, `status`, and `date`. The upgraded lock records `adrFormatV1From`, the first ADR number created after the project-atomic migration; lower numbers form the closed legacy-format set, while that number and every successor must carry the new marker. Their statuses are `Proposed`, `Accepted`, `Implemented`, and `Abandoned`; `Superseded` is not legal. awf validates the enum and the legal transitions: Proposed to Accepted, Implemented, or Abandoned; and Accepted to Implemented or Abandoned. Implemented and Abandoned are terminal. ADR number and title remain derived from the filename and heading rather than duplicated in frontmatter.

2. A new-format ADR has the ordered sections Context, Decision, State changes, Consequences, Alternatives Considered, and Status history. It no longer declares active invariants: intended rules and invariants appear as claim operations. Decision items remain column-zero sequential numbers as readable discrete commitments, but no active system treats them as supersession anchors.

3. `## State changes` contains either the single paragraph `None.` or a nonempty list whose exact entries are `- add <qualified-id>`, `- update <qualified-id>`, or `- remove <qualified-id>`, with the qualified ID written as one inline code span. `None.` and operations are mutually exclusive. One ADR may affect several topics and domains, but it names each claim at most once. A rename or move is remove plus add; a split is one remove plus several adds; a merge is several removes plus one add. A removed identity can never be added again.

4. State operations are the authoritative ADR-to-topic relationship. New ADRs have no `topics`, `domains`, `tags`, or `related` frontmatter. awf derives affected topics and domains from the qualified IDs. Historical prose can link to other ADRs normally, while claim Origin, Revised-by, References, and explicit topic history queries provide structured navigation without a general ADR relation graph.

5. Proposed ADRs are editable and non-authoritative. Accepted freezes Context, Decision, State changes, Consequences, and Alternatives Considered and is normative only as instruction for executing the pending change; it never overrides the current claims describing project reality. Every operation's destination topic metadata must exist before acceptance, including an empty topic shell for a pending add. Empty topics do not satisfy scoped coverage. Normal context uses that metadata to select a bounded Accepted notice. Temporary implementation authority ends at Implemented or Abandoned.

6. `## Status history` is append-only bookkeeping. Scaffolding writes `- YYYY-MM-DD: Proposed`. Accepted and terminal entries use `- YYYY-MM-DD: <status>; content-sha256: <digest>`, with an Abandoned entry additionally ending in `; rationale: <nonempty text>`. The digest covers canonical Context, Decision, State changes, Consequences, and Alternatives Considered, excluding frontmatter and Status history. Accepted establishes the frozen digest; a later terminal entry must repeat it. Direct Proposed-to-Implemented or Proposed-to-Abandoned establishes the terminal digest. Dates never descend, adjacent statuses follow the legal matrix, the final log status equals frontmatter, and static checking recomputes the digest. awf reports the exact expected digest for an incomplete transition entry.

7. Each Implemented ADR with operations adds `; state-sequence: <positive integer>` to its terminal history entry. Sequences are unique and contiguous across new-format mutation ADRs; awf reports the next value, and an Implemented `None.` ADR has no sequence. The sequence orders all operations in one ADR together. Revised-by follows increasing sequence. Each identity has exactly one add, zero or more ordered updates, and at most one terminal remove; no operation follows its remove and a removed identity is never reused.

8. In the pre-implementation state, every `add` target is absent and every `update` or `remove` target exists. An Abandoned ADR retains its intended operations as history, but they remain unapplied; partial work must be reverted or reconciled before abandonment. An Implemented add has one active claim whose Origin is that ADR unless a later remove exists. An Implemented update appears in Revised-by unless a remove exists. An Implemented remove has no active claim. Each active Origin or revision has the inverse new-format ADR operation. Origins at numbers below the lock's `adrFormatV1From` boundary are the closed migration bootstrap exemption because legacy ADRs have no State changes section.

9. The implementation transition is one checked Git transaction. `awf check --staged` compares HEAD with the index. A transitioned add must be absent before and present after with matching Origin. An update must exist on both sides, preserve Origin, preserve the complete prior Revised-by list as an exact prefix, append the transitioning ADR exactly once, and change at least one canonical claim field other than formatting or provenance alone. A remove must exist before and be absent after. Every staged claim mutation belongs to exactly one ADR becoming Implemented, and every declared operation has its matching mutation. A direct Proposed-to-Implemented transition follows the same rules.

10. Static `awf check` validates the resulting lifecycle, sequence, operation history, claim provenance, digest, and absence rules. The rendered pre-commit hook runs `awf check --staged`. Audit and implementation review evaluate every parent-to-commit snapshot pair in an explicit range rather than only its endpoints; a merge uses its first parent as the integration baseline, while included branch commits are evaluated individually. Endpoint comparison is reporting only and never proves atomicity. Configuration, topic inputs, ADRs, and markers are read entirely from each compared snapshot rather than mixed with the working tree.

11. At the one project-atomic upgrade boundary, staged validation uses a migration-only cross-schema adapter: it reads only ADR identity/status, legacy invariant declaration and retirement records, proof and touches markers, and the format-cutoff baseline from the legacy HEAD, then validates the index entirely with the new topic engine. It never assembles legacy context, supersession authority, or current guidance and remains only as upgrade-preflight support.

12. awf will replace `<docsDir>/decisions/ACTIVE.md` with generated `<docsDir>/decisions/INDEX.md`. Its In flight section lists Proposed and Accepted ADRs. Its History section lists Implemented and Abandoned ADRs by number, title, and status only. It renders no supersession chains, anchor annotations, partial-currentness claims, or domain ADR indexes.

13. ADR authoring, lifecycle, review, planning, implementation, audit, and retrospective skills will use the new format and authority language. Every status transition runs sync and commits the generated decision index and any navigation actually changed. Claim/topic mutations occur only in an Implemented transaction and must be absent or reverted for Abandoned. The implementation updates `.awf/config.yaml`; the authored sources for AGENTS.md, the ADR guide and template, workflow guidance, reviewer lenses, domain current state, architecture components and data flow, glossary, pitfalls, config reference, and adopter documentation; plus audit and index naming inputs. `./x sync` prunes ACTIVE.md, creates INDEX.md, and regenerates every runtime copy in the same behavior-changing commits.

## Invariants

- `unbacked-invariant: adr-status-enum-and-matrix`: Every ADR at or above the lock's format boundary uses the new encoding, has a recognized status, and follows one of the five legal transition edges. **Verify:** exercise missing format markers around the cutoff and every status pair in static and snapshot fixtures; confirm only legacy numbers and the declared matrix succeed.
- `unbacked-invariant: implemented-impact-bidirectional`: Every Implemented state operation has its required current or removed result, and every active claim Origin or revision has the inverse ADR operation. **Verify:** exercise missing, extra, duplicate, and mismatched add/update/remove operations against claim provenance in corpus tests.
- `unbacked-invariant: state-impact-transition-atomic`: A staged claim mutation and the ADR transition implementing it occur in one HEAD-to-index transaction. **Verify:** split each side across snapshot fixtures and confirm `awf check --staged` refuses both partial states, then confirm the combined transition passes.
- `unbacked-invariant: update-requires-substance`: An update preserves Origin and prior revision history, appends its ADR once, and changes a canonical claim field other than formatting or provenance alone. **Verify:** compare staged fixtures containing Origin edits, revision deletion/reordering, whitespace-only, provenance-only, normative prose, reference, and backing changes and confirm only prefix-preserving substantive cases satisfy an update.
- `unbacked-invariant: accepted-does-not-override-current`: Accepted operations appear only as bounded pending instructions and do not replace current claim output. **Verify:** query a fixture with an Accepted update conflicting with its current claim and confirm both are clearly separated with the claim remaining current.
- `unbacked-invariant: removed-claim-id-not-reused`: Once an Implemented remove records a qualified ID, no later add may reuse it. **Verify:** construct add-update-remove-add operation histories and confirm the final add fails regardless of whether an active claim file exists.
- `unbacked-invariant: decision-index-is-historical-not-authoritative`: The generated decision index separates in-flight work from compact history and never renders supersession or currentness inference. **Verify:** render fixtures in every legal status and inspect the exact INDEX.md sections and absence of anchor annotations.

## Consequences

An ADR now states not only what was chosen but exactly which active claims its implementation intends to change. Reviewers can compare a bounded decision and state diff. Forgotten documentation and provenance become deterministic failures, while semantic fidelity remains honestly outside the checker.

The lifecycle becomes smaller and terminal. Abandoned preserves useful negative rationale without keeping a declined proposal active. Implemented means incorporated history, so later refinements never require editing predecessor bodies or computing partial supersession.

Claim wording changes become governed mutations. Explanatory topic prose and summaries may still improve without an ADR, but changing a claim's canonical prose, references, type, or backing requires an update operation. Meaning-preserving formatting normalization does not satisfy or require an update; the snapshot engine compares canonical parsed fields.

The staged check introduces explicit Git index semantics into the blocking gate. It must share snapshot loading with per-commit range audit to avoid two interpretations. The migration-only cross-schema adapter is a narrow additional seam, not a retained compatibility authority engine. This expands hook and fixture complexity but catches removal and substantive-update facts that a current-tree-only check cannot prove.

Old ADRs remain readable legacy-format history. The migration decision owns their normalization, the resolution of existing in-flight ADRs, and the atomic point at which new format becomes mandatory. There is no permanent mixed-format authoring mode after upgrade.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Require only a current claim to cite an ADR | It cannot distinguish a forgotten operation from provenance added without a substantive change. |
| Generate claim provenance from State changes | It removes the bidirectional handshake and makes the ADR the indirect source of current state again. |
| Validate transitions only during review audit | A malformed implementation commit could land before the advisory review runs. |
| Keep Superseded and partial anchor coverage for history | Historical ADRs need no computed currentness once claims own active authority. |
| Leave Accepted ADRs out of normal context | In-flight implementation instructions would become undiscoverable to agents working in the affected topic. |
| Permit removed claim IDs to be reused | History lookup and operation validation could no longer identify one continuous claim lifecycle unambiguously. |
