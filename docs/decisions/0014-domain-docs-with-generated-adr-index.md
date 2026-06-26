---
status: Accepted
date: 2026-06-26
supersedes: []
superseded_by: ""
tags: [docs, adr-system, rendering]
related: [0005, 0011, 0013]
domains: [adr-system, rendering]
---
# ADR-0014: Domain Docs with a Generated Per-Domain ADR Index

## Context

ADR-0013 reserved `<docsDir>/domains` (`.layout.domainsDir`) as the awf-given home for
per-domain "current state" docs, and `proposing-adr`/`adr-lifecycle` already instruct
authors to "update the domain doc under `docs/domains` when an ADR shifts a domain's state."
Today those skills also mandate hand-maintaining a "Load-bearing ADRs table" inside each
domain doc — a manual index that drifts the moment an ADR is added, retagged, or superseded
without someone remembering to edit the table.

The data to generate that table automatically already exists. Every ADR's frontmatter carries
`tags`, `supersedes`, `superseded_by`, and `related` (e.g. ADR-0013), and a superseded ADR
records its successor in `superseded_by`. But `internal/adr.adrFrontmatter` parses **only**
`status` — the rest is authored and discarded. So an auto-generated per-domain index is
mostly "parse fields that already exist and render," reusing the exact grouping/link machinery
that already builds `ACTIVE.md` (`internal/adr.RenderActiveMD`).

A domain doc is a hybrid: hand-authored narrative ("current state") plus a generated index.
Its index content depends on **external ADR frontmatter state**, not just its own template +
config + parts. That breaks the normal managed-doc drift check, which compares on-disk content
against the *locked* hash: retag an ADR without re-syncing and the domain doc's on-disk content
still matches its lock, so `check` would pass while the index is silently stale. This is the
exact problem `ACTIVE.md` already solves with a **regenerate-and-compare** drift path
(`internal/project` Check, the `activeMd` special-case) that regenerates fresh from current
ADRs and diffs against disk, bypassing the lock-hash loop.

A grounding sweep confirmed: `frontmatter.Parse` ignores unknown YAML keys (so retro-tagging
ADRs with `domains:` is safe before the struct is extended); adding an optional
`config.Domains []string` is backward-safe and needs no schema-version bump (precedent: the
optional `Invariants` block); `.layout.domainsDir` exists and nothing renders there today; the
data-injection, convention-part-resolution, and regenerate-and-compare patterns all generalize.

**Linkage decision (settled in brainstorm):** the domain↔ADR linkage uses a **dedicated
`domains:` frontmatter field**, not the existing `tags`. `tags` are granular freeform keywords
(`[docs, layout, templates]`); a domain index is only as trustworthy as the linkage feeding
it, so a controlled coarse key keeps it clean and `tags` free for keywords.

**Ownership decision (settled in brainstorm):** domain docs are **fully awf-managed** (awf owns
what it renders) — Approach B — rather than a generated region spliced into a user-owned file
(partial-ownership) or a separate generated file beside a hand-authored one (two files, breaks
single-read). A more direct partial-ownership model may be worth a future general engine change
applied uniformly, but not as a one-off exception here.

## Decision

1. **New `domains: [<name>...]` config array** in `.claude/awf/config.yaml` — a
   *project-defined* enable list. Unlike `skills`/`agents`/`docs`, domain names are **not**
   validated against the catalog (they are arbitrary project concepts). `config.Validate`
   instead applies name-sanity: a domain name containing a path separator (`/` or `\`, matching
   the existing `prefix` check in `config.Validate`) or `..` is rejected.

2. **New domain-doc artifact kind.** A single template `templates/domains/domain.md.tmpl` is
   instantiated once per declared domain, rendered to `<docsDir>/domains/<name>.md`. It has
   exactly **one overlay section**, `current-state` (hand-authored narrative —
   convention-part-overridable at `.claude/awf/domains/parts/<name>/current-state.md`,
   template default = skeleton prompt). The generated `## Decisions` index is **forced body**,
   not an overlay section: it is plain template (outside any `awf:section` marker) with the
   index injected as render data, rendered last, beneath the narrative — always present and
   structurally un-overridable (no marker → no overlay → a convention part cannot replace it).
   It is treated like frontmatter: awf-owned and forced, not author-controlled. Because
   `decisions` is not a declared section, `orphans()` flags any domain part other than
   `current-state` (e.g. a stray `decisions.md`) rather than letting it silently shadow the
   index.

3. **`internal/adr` extension.** `adrFrontmatter` gains `domains []string` and starts parsing
   the already-present `superseded_by`; `ADR` carries both. A new
   `RenderDomainIndex(decisionsDir, domain)` keeps the ADRs whose parsed `domains` includes
   `domain`, groups them by status (reusing `RenderActiveMD`'s order and link rendering with
   `../decisions/<file>` paths), annotates each superseded entry with its `superseded_by`
   successor, and returns a placeholder line when the domain has no ADRs.

4. **Regenerate-and-compare drift for domain docs.** Each domain doc is excluded from the
   generic lock-hash drift loop and instead regenerated fresh in `Check` and diffed against
   disk — the `ACTIVE.md` pattern generalized from one file to N. Drift kinds: `stale`
   (content diverged), `missing`, `orphaned` (domain removed from config). Lock entries carry
   empty `TemplateID`/`TemplateHash`, like `ACTIVE.md`.

5. **Reconciliation with ADR-0011.** ADR-0011's "doc default content stays static (no `.vars`
   / `.data` interpolation)" governs the fixed catalog docs. The domain-doc `decisions` section
   is deliberately **data-driven**, following the always-on agents-doc pattern (which already
   ranges over injected `.data`), not the static catalog-doc rule. Domain docs are a distinct
   artifact kind, outside ADR-0011's static-content scope.

6. **Workflow change.** `docs/decisions/template.md` gains `domains: []` and the `## Frontmatter`
   reference block in `docs/decisions/README.md` documents the new field; `proposing-adr` and
   `adr-lifecycle` require setting it. `adr-lifecycle`'s domain-doc step today reads "Add to the
   Load-bearing ADRs table; refresh the Current-state prose" (the explicit table mandate lives
   there; `proposing-adr` carries only a generic "update the domain doc" instruction). The
   table-maintenance half is **dropped** — the per-domain index now maintains that table
   automatically — while the current-state prose-refresh obligation is **retained**. Setting
   `domains:` on a new ADR is thus the single action that keeps every affected domain doc's
   generated `decisions` index current; the hand-authored `current-state` narrative is still
   refreshed by hand when a domain's position materially shifts.

7. **First-adopter dogfood.** This repo declares domains `[rendering, config, invariants,
   tooling, adr-system]`, retro-tags all existing ADRs with `domains:`, and ships a brief
   `current-state` narrative per domain.

## Invariants

Constraints that must hold while this decision stands; a violation should trigger a new ADR.

- `inv: domain-index-matches-domains` — `RenderDomainIndex(dir, d)` lists exactly the ADRs under
  `dir` whose parsed `domains` frontmatter includes `d`, and no others.
- `inv: domain-doc-regenerated` — `Check` regenerates each enabled domain doc from current ADR
  state and reports it `stale` when on-disk content diverges; retagging or superseding an ADR
  without `sync` is detected (the domain doc is not validated by the lock hash alone).
- `inv: domain-name-validated` — `config.Validate` rejects a domain name containing a path
  separator (`/` or `\`, as the `prefix` check already does) or `..`.
- **Publication-safe** (textual) — a domain doc with zero matching ADRs renders a placeholder,
  never a `<no value>` token or an empty `## Decisions` index.

## Consequences

- The manual "Load-bearing ADRs table" doc-currency burden is removed: tagging an ADR's
  `domains:` is the one action that keeps every affected domain doc's index current, enforced
  by regenerate-and-compare drift.
- A new managed artifact kind and an optional config field are added; both are backward-safe —
  no `awf upgrade` migration (additive optional field, like the `Invariants` block).
- `domains` is the first awf enable-array that is *not* catalog-bound; validation is name-sanity
  only, and `Open` must route it around the catalog `checkKind` path.
- The `domains` kind must be wired into orphan/section-parity detection. Parity inverts the
  catalog model (one template → N instances, vs. the catalog's 1↔1), needing a multi-instance
  parity test rather than the existing per-catalog-entry one.
- Every future ADR must set `domains:` or its domains' indices stay incomplete; `proposing-adr`
  enforces it, and the ADR template seeds the field.
- New branches (the domains render loop, the regenerate-and-compare `Check` path, the adr
  domain-filter) must be exercised both ways under the 100% coverage gate (ADR-0012).
- Unblocks richer per-domain views later (supersession chains, related-ADR graphs) without
  re-deciding the ownership model.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Partial-ownership: generated region spliced into a user-owned `docs/domains/<d>.md` (markers; only the region regenerated/drift-checked) | Best single-read ergonomics, but introduces a one-off partial-file ownership model awf has never had; conflicts with the established "awf owns what it renders" convention. Revisit as a *general* engine change applied uniformly, not a domain-docs exception. |
| Separate generated index file beside a hand-authored narrative (`<d>.md` + `<d>.adrs.md`) | Reuses the `ACTIVE.md` whole-file generator verbatim with zero new capability, but two files per domain breaks the "a single read gives the full picture" requirement. |
| Reuse the existing `tags` frontmatter for the domain linkage | `tags` are granular freeform keywords, not a controlled coarse key; matching a domain index against noisy tags makes the generated table untrustworthy and forces tag-as-domain discipline on every author. |
| Plain managed doc (whole-file hash drift like `architecture.md`) | A domain doc's content depends on external ADR state not captured in the lock hashes, so an ADR retag without re-sync would pass `check` while the index is silently stale. Regenerate-and-compare is required. |
| Bundle the `domains:` frontmatter/workflow change as its own ADR | The field exists *for* the domain index — the two are tightly coupled, not independently load-bearing. One ADR. |
