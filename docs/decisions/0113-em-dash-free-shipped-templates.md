---
status: Superseded
date: 2026-07-15
tags: [template-residue, doc-standard]
related: [82, 112, 115, 117]
domains: [rendering]
---
# ADR-0113: Em-dash-free shipped templates

## Context

awf renders its shipped templates into every adopter's skills, agents, docs, and agent guide. Em-dash characters (U+2014) in that prose read as machine-set — a tell of unedited AI authoring — so a prior effort removed every em-dash from the shipped templates in favour of plain punctuation (colons, semicolons, commas, parentheses). That effort left `templates/` completely em-dash-free but added **no enforcement**: no gate, no test, no documented rule.

The gap surfaced immediately. The very next change (ADR-0112, which recorded the core-only agent-guide criterion) reintroduced two em-dashes into `templates/docs/agents-md-standard.md.tmpl`, and `awf check` stayed green because nothing scans for them. The regression was caught only because an implementation reviewer happened to recall the just-completed removal. "Removed, then reintroduced the next commit, with nothing to catch it" is the signal to move the rule into a deterministic check.

This is the same publication-cleanliness concern as the source-level residue guard (ADR-0082), which already walks the embedded `templates.FS` and fails on a concrete ADR citation or a repo-identity literal. An em-dash ban is a natural sibling in that family: a property of the shipped prose that no render-based sweep can cover, only a source scan of every template.

## Decision

1. **Shipped templates carry no em-dash.** Every file in the embedded `templates.FS` is free of the U+2014 character. A dedicated gate test (`TestTemplateNoEmDash`, beside the residue guard in `internal/project/residue_scan_test.go`) walks the template FS and fails on any occurrence, so `./x gate` catches a reintroduction.

2. **Scope is the shipped templates only.** The ban applies to `templates.FS` and nothing else. Hand-authored ADRs under the decisions directory and plans under the plans directory keep em-dashes freely — they are authored records, never rendered output. Adopter-authored convention parts and sidecar `data:` are likewise out of scope: their house style is the adopter's own, consistent with the residue guard's and the part-marker advisory's deliberate exclusion of adopter content. Only U+2014 is banned; the en-dash (U+2013) and the ellipsis (U+2026) remain legitimate and are untouched.

3. **No escape hatch.** The templates are already em-dash-free, so the check ships with no exemption list — mirroring the strict, escape-hatch-free posture of the dead-code gate. A template that genuinely needs an em-dash would be a reason to revisit this decision, not to add a bypass.

4. **The convention is documented, not only enforced.** The documentation authoring standard gains the plain-punctuation rule in prose, so an author learns it before a failing gate teaches it. Because that standard is itself a shipped template scanned by the new gate, its rule text names the character by word and codepoint rather than typing the glyph.

## Invariants

- `` `invariant: template-em-dash-free` `` — no file in the embedded `templates.FS` contains the U+2014 em-dash character. Backed by `TestTemplateNoEmDash` in `internal/project/residue_scan_test.go`.

## Consequences

- **The regression class is closed for shipped prose.** A reintroduced em-dash now fails `./x gate` deterministically instead of relying on a reviewer's memory.
- **Adopter-injected content is a stated blind spot, not an oversight.** Prose that reaches rendered output through a sidecar `data:` value or a convention part is not scanned, so a future em-dash there would render uncaught. This is the same boundary the residue guard draws around adopter content, and it is accepted deliberately: awf enforces its own shipped prose, not an adopter's house style. awf's current data and parts are em-dash-free.
- **The documentation standard gains a durable rule.** Adopters re-rendering the standard receive the plain-punctuation guidance, so the convention travels beyond this repo.
- **Minor authoring constraint on one template.** The doc-standard template — and any future template that discusses punctuation — must describe the em-dash without typing it, or the gate fails on its own convention doc.
- **Flip-commit doc obligations, diverging from ADR-0082's pattern.** The commit flipping this ADR to Implemented ships the backing test, regenerates `docs/decisions/ACTIVE.md`, re-renders the documentation standard from its edited template, and carries a changelog entry for that adopter-facing rendered change. Unlike its sibling ADR-0082, it does **not** add a bullet to the agent guide's Invariants list: under the core-only guide criterion (ADR-0112), `template-em-dash-free` is a subsystem-specific rendering invariant reached on demand via `awf context`, not one of the every-change rules the guide enumerates.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Plain gate test, no ADR | Lighter, but the convention had no recorded rationale and awf backs every publication-quality guard with an ADR-declared invariant surfaced by `awf context`; recording it prevents the next author from re-litigating it. |
| Fold the check into the existing residue guard | The residue guard's invariant is about ADR citations and repo identity; an em-dash ban has a different rationale, so a separate test and slug keep each guard's meaning clean. |
| Ban all machine-set punctuation (en-dash, ellipsis too) | The removal effort deliberately targeted only em-dashes; en-dashes and ellipses exist legitimately in the templates today, so a broader ban would fail on landing and demand an unrequested cleanup. |
| Scan awf's own authored `.awf/` prose and rendered output too | Broadens to house-style completeness, but couples the check to adopter-shaped content and rendered files; the shipped-template scan is the clean boundary that protects every adopter. |
