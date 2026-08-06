---
format: current-state-v4
slug: separate-structural-markdown-headings-from-section-bodies
status: Implemented
date: 2026-08-05
---
# ADR-0237: Separate structural Markdown headings from section bodies


## Context

An awf section currently has one replaceable text body. When a Markdown heading sits inside that
body, a convention part replaces the heading along with the prose; when the heading sits immediately
before the section marker, dropping the section removes the body but leaves its heading behind. The
same conceptual structure therefore behaves differently according to incidental marker placement,
and a project can accidentally delete or duplicate document hierarchy while making an ordinary body
override.

The inconsistency is systemic. A census of 272 section-marker sites in the embedded templates found
136 headings at the start of section bodies, 41 headings immediately before their markers, and 95
intentional headingless fragments. Treating every section as headed would break fragments, while
leaving headings in replaceable bodies preserves the override and drop hazards.

The render engine already owns the correct separation point. It parses structural section markers,
chooses one body source, emits the surviving edit pointer, and omits a dropped section. Convention
parts are deliberately raw body inputs, and in-place sections preserve only the adopter-owned region
while regeneration owns surrounding structure. A heading that defines the template's document
hierarchy belongs with that structure, not with whichever body source wins.

Changing the boundary must preserve existing adopter intent. Many convention parts begin with the
same heading the old template body supplied because replacing the body previously required carrying
that heading. Removing an exact known heading is mechanical; interpreting a different leading
heading as either customized structure or intentional body content is not. Migration must refuse
that ambiguity rather than silently choose.

## Decision

1. `decision: optional-structural-heading` A section in an output whose declared policy is Markdown may carry one optional awf-owned ATX heading as the first structural line after its opening marker. The parsed section model stores that heading separately from the replaceable body. Headingless sections remain first-class, and non-Markdown outputs do not acquire heading inference from hash-prefixed comment lines.
2. `decision: heading-body-assembly` A surviving section assembles in the order edit pointer, optional structural heading, then the winning body source. Template defaults, convention parts, section-default reinjection, and in-place read-back replace or preserve only the body. For an in-place section, the pointer guidance states that the optional heading immediately below it is awf-owned and that only the body after that heading is preserved; it never makes the broader current promise that every line below the pointer is adopter-owned. A dropped section emits none of its pointer, heading, or body. Structural markers remain consumed and absent from rendered output.
3. `decision: structural-heading-drift` The template is the sole authority for a structural heading. Regeneration emits it from template source, never from a convention part or in-place body. For an in-place section, read-back excludes the expected structural-heading slot while preserving the body interior verbatim; a changed heading is awf-owned tamper and surfaces as drift rather than becoming adopter-owned body content.
4. `decision: exact-heading-migration` The embedded Markdown templates are normalized exhaustively: a heading already first inside a section becomes its structural heading, a heading immediately before a section marker moves into that section's structural slot, and a genuinely headingless section remains headingless. A checked census classifies every live site and fails on an unclassified or multiply associated heading instead of relying on a broad preceding-line heuristic. Structural headings remain in the existing missingkey=zero template execution path and retain coherent empty-value fallbacks; extraction and reassembly may not introduce an unresolved no-value token.
5. `decision: adopter-part-migration` A schema-generation migration uses the exact section-to-heading snapshot shipped at the cutover. For each affected convention part, an exact legacy heading at the body's leading structural position is removed while the remaining body bytes are preserved; a part with no leading heading is unchanged. A different or ambiguous leading heading refuses the complete preflight before mutation through the actionable outcome protocol: category `operation`, a present-tense condition naming the part and expected heading, every changed axis reported false because preflight has mutated nothing, no cause, and an ordered remedy to edit that part so the heading is either the exact removable legacy heading or unambiguously body content, then rerun `awf upgrade`. After preflight, changed parts are replaced atomically and the migration is idempotent and safely retryable without claiming transaction-wide file atomicity. Future structural headings are not retroactively stripped by this fixed snapshot.
6. `decision: no-heading-configuration` Structural headings are template declarations, not sidecar fields or a new override channel. Projects that need additional headings may author them as body content, but cannot replace or suppress the section's structural heading independently of dropping the complete section. Identity-aware Markdown hierarchy or arbitrary heading customization remains outside the generic section model.

## State changes

- update `rendering/render-engine:section-edit-pointer`
- add `rendering/render-engine:structural-heading-owned`
- update `rendering/render-engine:section-default-splice`
- update `rendering/inplace-and-placeholders:in-place-readback`
- update `rendering/inplace-and-placeholders:in-place-spacing-owned`
- update `rendering/inplace-and-placeholders:in-place-tamper-drift`
- add `config/migrations-and-locks:structural-heading-part-migration`

## Consequences

Body overrides no longer need to copy template hierarchy, and dropping a headed section cannot leave
an orphan heading. The model represents the real optional structure found by the census without
forcing headings onto fragments. In-place editing retains its body-only ownership boundary, while a
heading change follows the existing regeneration-drift path for awf-owned structure.

The render parser and assembly model become more explicit, and every live Markdown template site
must be normalized and exhaustively classified. Parts that carried exact copied headings become
smaller during upgrade without changing rendered output. A custom leading heading blocks migration
until its owner resolves the ambiguity; this is more friction than preserving it blindly, accepted
to avoid either duplicated hierarchy or silent deletion of authored content.

The heading is no longer independently customizable. A project can still add subordinate or
additional headings inside its body, but the template-owned section heading changes only when awf
changes the template, and disappears only with the whole section. The policy-aware parser also keeps
shell and other non-Markdown section bodies from misclassifying comment lines as headings.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep headings in replaceable bodies and standardize marker placement | Parts would still need to copy headings, and body replacement could still erase document hierarchy. |
| Associate any heading immediately preceding a marker at render time | Preceding text belongs to the literal stream and association becomes fragile around blank lines, prose, conditionals, and multiple candidates. |
| Require every section to have a heading | Ninety-five current sites are intentional fragments, including non-Markdown content and substructures that should not create document hierarchy. |
| Add a sidecar heading override | It introduces another configuration channel for structure and restores the ambiguity this separation removes. |
| Silently preserve any leading part heading as body content | Exact legacy copies would duplicate the new structural heading, while custom headings could produce accidental sibling hierarchy; intent cannot be inferred safely. |

## Status history

- 2026-08-05: Proposed
- 2026-08-05: Accepted; content-sha256: e1d2f50c8b35c7669642facb1a55c113e46eb83ffe98d838b6fb08367d9fecfb
- 2026-08-05: Implementing; content-sha256: e1d2f50c8b35c7669642facb1a55c113e46eb83ffe98d838b6fb08367d9fecfb
- 2026-08-05: Applied; operations: update `rendering/render-engine:section-edit-pointer`, add `rendering/render-engine:structural-heading-owned`, update `rendering/render-engine:section-default-splice`, update `rendering/inplace-and-placeholders:in-place-readback`, update `rendering/inplace-and-placeholders:in-place-spacing-owned`, update `rendering/inplace-and-placeholders:in-place-tamper-drift`, add `config/migrations-and-locks:structural-heading-part-migration`
- 2026-08-06: Implemented; content-sha256: e1d2f50c8b35c7669642facb1a55c113e46eb83ffe98d838b6fb08367d9fecfb