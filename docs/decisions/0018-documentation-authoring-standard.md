---
status: Implemented
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [docs, adoption, rendering]
related: [4, 11, 15]
domains: [rendering]
---
# ADR-0018: Documentation Authoring Standard — `doc-standard.md` and `agents-md-standard.md`

## Context

awf ships a suite of managed docs (ADR-0011) and an always-loaded agent guide (`AGENTS.md`,
ADR-0004). ADR-0011 gave each doc a per-section *content* prompt — the italic skeletons that say
*what* a section should contain (e.g. architecture/overview: "One paragraph: what this system is
and its shape at a glance"). What is missing is a cross-cutting layer of *how-to-write* rules:
terseness, voice, what to leave out, when to link instead of restate. A grep across `docs/` and
`templates/docs/` finds no prose/style guidance at all.

Two concrete symptoms:

- **`AGENTS.md` is loaded every session and narrates mechanism it should not.** Its Invariants
  section runs ~270 words across nine bullets; several of its six adopter-supplied bullets
  (`data.invariants` in `.awf/agents-doc.yaml`) spend two or three sentences
  explaining the *mechanism* of a rule that tooling already enforces and an ADR already documents
  (missingkey=zero, the drift oracle, `coverage-ignore`, frontmatter validation). This is the
  "linter-rules in prose" anti-pattern: the agent hits the gate regardless, so the always-loaded
  file pays context budget, every session, to re-narrate what a deterministic check enforces.
- **There is no documented standard for how an adopter should author their `AGENTS.md`** — its
  layout, what goes in each section, or how to write it. Authoring guidance exists for exactly one
  section (`identity`'s placeholder hint); the pattern is real but applied once.

The field consensus awf aligns to is explicit: instruction/context files should be short,
hand-crafted, progressively disclosed, with linter-rules kept out of prose. awf publishes a
standard and is its own first adopter, so the rules it would hand an adopter are the rules it must
apply to itself.

## Decision

1. **Ship `doc-standard.md`** — a general managed doc holding the how-to-write rules for *all*
   awf-managed prose (docs and the agent guide). Sections `[principles, rules, structure]`:
   *principles* (docs orient and link; detail lives in the most specific doc; the document map is
   the index — progressive disclosure); *rules* (terse, since managed docs are read repeatedly;
   linter-rules out of prose — state a tooling-enforced rule once and cite its ADR rather than
   narrating the mechanism; no editorializing or dating (a discipline this standard itself
   establishes; no existing ADR owns it); reference ADRs by id rather than restating them;
   present-tense, authoritative voice);
   *structure* (`awf:section` markers, per-section content prompts, override via convention parts,
   docs travel with the change).

2. **Ship `agents-md-standard.md`** — an `AGENTS.md`-specific managed doc that references
   `doc-standard.md` for the writing rules and adds what is specific to the agent guide. Sections
   `[layout, content, rules]`: *layout* (the fixed section taxonomy, its order, and which sections
   are awf-given versus adopter-authored); *content* (the per-section spec for the three
   adopter-authored sections — identity, you-and-this-project, invariants); *rules* (the
   `AGENTS.md`-only deltas: it is loaded every session, so it is held to an extra-terse bar, and
   invariants are stated as one-line imperatives).

3. **Both are ordinary catalog docs** using the ADR-0011 mechanism — a `templates/catalog.yaml`
   entry (`title`/`desc`/`sections`), a `templates/docs/<name>.md.tmpl` template (auto-embedded;
   `templates/docs` is already in `templates/embed.go`), and enablement via the `.awf/config.yaml`
   `docs:` array. They auto-appear in the agent guide's document map through `resolvedDocs()`
   (`internal/project`). No render-engine, catalog-schema, or lock-format change.

4. **Add in-context authoring hints to the agents-doc template defaults**
   (`templates/agents-doc/AGENTS.md.tmpl`): refine the `identity` default's placeholder hint to
   match the standard and point to `agents-md-standard.md`; add a one-line **HTML-comment** hint
   (invisible in rendered markdown, the ADR-0015 precedent) inside the `you-and-this-project` and
   `invariants` section defaults, pointing to the same doc. Hints live within existing section
   markers — no new sections, no visible bloat.

5. **Dogfood the standard on awf's own agent guide.** Compress this repo's `AGENTS.md` Invariants
   (the `data.invariants` entries in `.awf/agents-doc.yaml`) from multi-sentence mechanism
   explanations to one-line imperatives plus an ADR reference (target ~90 words). **Scope
   boundary:** the dogfood is this leanness pass plus writing the two new docs to conform to their
   own rules — it is **not** a re-audit or rewrite of awf's other six managed docs. Bringing the
   existing docs into full conformance is a separate, later sweep.

## Invariants

These are textual quality contracts; the mechanical protection for the two new docs (catalog
section-parity and the `<no value>` publication-safety guard) is **inherited from ADR-0011** and
applies to every catalog doc automatically, so this ADR introduces no new tagged slug.

- `doc-standard.md` states the cross-cutting writing rules; `agents-md-standard.md` references it
  rather than duplicating them.
- The two new docs are themselves written to conform to `doc-standard.md` (terse, no editorializing,
  ADR-referenced).
- awf's own `AGENTS.md` Invariants are stated as one-line imperatives — no bullet narrates the
  mechanism of a rule that a gate, drift check, or ADR already owns.

## Consequences

- The always-loaded `AGENTS.md` gets lighter: the six adopter-supplied Invariants bullets
  (`data.invariants`) drop from ~200 to ~90 words. The three universal bullets (append-only ADRs,
  docs-travel, green-gate) are template-fixed and unchanged, so the rendered Invariants section
  shrinks from ~270 to ~160 words and stops paying, every session, to re-explain machine-enforced
  rules.
- awf now ships an opinionated, documented authoring standard adopters can follow, with the writing
  rules in one canonical place and the rule shown in-context where a section is edited.
- Two more managed docs to maintain; the catalog and document map grow by two entries each.
- A canonical writing standard now exists, which surfaces that awf's six existing docs predate it.
  Their conformance sweep is created as known downstream work, explicitly deferred (Decision 5).
- The pre-existing gap that agents-doc section-parity is untested (`docs_sections_test.go` covers
  `cat.Docs`, not `cat.AgentsDoc`) is unchanged by this ADR and remains out of scope.

Doc-currency obligations the implementing commit(s) must satisfy:

- The Proposed→Accepted status flip regenerates `docs/decisions/ACTIVE.md` via `./x sync` in the
  same commit. No `docs/decisions/README.md` index row is owed — the README is a how-to guide;
  `ACTIVE.md` is the generated index (ADR-0005).
- The dogfood edit to `.awf/agents-doc.yaml` (Decision 5) re-renders `AGENTS.md`/`CLAUDE.md` via
  `./x sync` in the same commit, kept green by `./x check`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Template-default hints only, no doc | No holistic, discoverable "how to write it" reference; rules scattered across section placeholders. |
| One doc only, no in-context hints | The rule is invisible at the moment an adopter edits a section's part. |
| A single combined `agents-md-standard.md` carrying the general rules too | The writing rules apply to *all* docs; burying them under AGENTS.md specifics makes them non-reusable and mis-homed. A general `doc-standard.md` that the AGENTS.md doc references keeps each rule where it belongs. |
| Re-audit all existing docs to the new standard now | Scope; define and dogfood the standard first, then sweep existing docs as separate, reviewable work. |
| Add a tagged machine-enforced invariant for the new docs | Redundant — ADR-0011's section-parity and publication-safety tests already cover every catalog doc, including these two. |
