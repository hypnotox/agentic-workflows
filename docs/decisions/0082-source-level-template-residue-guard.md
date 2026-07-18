---
status: Implemented
date: 2026-07-09
tags: [publication-safety, template-residue]
related: [1, 45, 80]
domains: [rendering, tooling]
---
# ADR-0082: Source-level template residue guard

## Context

Shipped template defaults are awf's publication surface: whatever they carry renders into
every adopter's repo. ADR-0001/ADR-0045 promise publication-safe output: no unresolved
tokens, coherent generic prose under unset data. That promise has so far been enforced
against *render mechanics* (the `<no value>` hard error, ADR-0080's empty-data sweep), not
against *content residue*: prose that renders cleanly but only makes sense in awf's own
repo.

A 2026-07-09 sweep found exactly four such leaks, concrete citations of awf's own decision
records in template defaults:

- `templates/agents-doc/AGENTS.md.tmpl`: "(ADR-0051)" inside the `{{ with $.commitScopes }}`
  branch, so it lands in the Invariants section of any adopter that configures commit scopes;
  in that repo "ADR-0051" names a different decision or nothing at all.
- `templates/bootstrap/awf-bootstrap.sh.tmpl`: "(ADR-0049)" in two comments.
- `templates/docs/working-with-awf.md.tmpl`: "(ADR-0081)" in the command overview.

The same doc template also used awf's own repo layout (`cmd/**`, `internal/audit/*.go`) as
the worked examples for the anchored-glob dialect, not a correctness leak, but
awf-flavoured where the standard claims language-agnosticism.

Two structural facts explain why ADR-0080's sweep could not have caught these and shaped
the enforcement choice:

1. **Coverage:** the sweep iterates `catalog.Standard` skills and agents; the agents-doc,
   bootstrap, and docs templates all sit outside it.
2. **Render blindness:** the sweep renders under empty adopter data, so a populated-only
   branch like `{{ with $.commitScopes }}` never renders; the worst leak is invisible to
   any empty-data render check.

Not everything matching a naive residue grep is a leak. The `ADR-NNNN` placeholder and
generic ADR prose in the adr-system templates are the shipped standard working as intended.
And two repo-identity literals are functionally required: the bootstrap's
`REPO="hypnotox/agentic-workflows"` (it downloads the awf binary from awf's releases for
every adopter) and the agent guide's link to awf's GitHub home (it names the tool that
manages the setup). All four leaks survived multiple reviewed efforts, including the
reviews that introduced them; human review is demonstrably insufficient for this class.

## Decision

1. **Residue rule.** Shipped template sources (every file in the embedded `templates.FS`)
   never carry a concrete awf ADR citation: the token `ADR-` followed by four digits. The `ADR-NNNN` placeholder and
   generic prose about ADRs remain legal; the digits-only pattern separates the two exactly.
   Decision rationale belongs in `docs/decisions/`, never in shipped prose; a template
   comment is source too and is equally banned.

2. **Repo-identity rule.** The literals `hypnotox` and `agentic-workflows` are banned in
   template sources, except where the reference is to awf-the-product rather than residue.
   Exactly two exemptions ship: the bootstrap's `REPO` slug (the download source) and the
   agent guide's awf-home link. Exemptions are explicit per-file entries that fail when
   stale, per ADR-0080 Decision 7; no guard grows an implicit or silently-skipped
   exclusion.

3. **Enforcement is source-level.** A gate test walks the embedded `templates.FS` and scans
   raw template sources (shape precedent: `TestCommitScopeSingleStorage`). Source-level is
   load-bearing, not incidental: render-based checks structurally miss non-catalog
   templates (Context point 1) and unpopulated conditional branches (Context point 2), and
   this guard exists precisely because those blind spots shipped real leaks. The ADR-0080
   render sweep is unchanged; the two guards are complementary layers.

4. **The standing residue is removed.** The four citations are dropped (surrounding prose
   kept), and the working-with-awf glob examples switch to a neutral `src/`-layout project
   (`*.ts`, `**/*.ts`, `src/**`, `src/api/*.ts`). This deliberately also drops the accurate
   "(ADR-0051)" citation from awf's own rendered `AGENTS.md`: pristine adopter defaults
   outrank house-style citation in the one repo where the number happens to resolve, and no
   convention-part override is added to restore it.

## Invariants

- `invariant: template-source-residue`: every file in the embedded templates FS is free of
  `ADR-[0-9]{4}` tokens, and free of repo-identity literals (`hypnotox`,
  `agentic-workflows`) outside an explicit exemption list whose entries each fail when the
  named file no longer carries the literal.
- `invariant: residue-exemptions-pinned`: the identity-exemption list contains exactly two
  entries, the bootstrap template and the agents-doc template, asserted by the guard test;
  extending the list requires a successor ADR amending this item.

## Consequences

- Template authors can no longer cite the decision that shaped a passage, even in
  authoring comments; the owning ADR is discoverable via `docs/decisions/` and the domain
  docs instead. This is the cost of a simple, exact rule.
- awf's own rendered `AGENTS.md` loses one accurate citation (Decision 4); its guide entry
  still names `audit.allowedScopes`, so the pointer survives without the number.
- A future template that legitimately needs a repo-identity mention (e.g. a new
  install-path doc) must add a stale-failing exemption entry via a successor ADR amending
  `residue-exemptions-pinned`, keeping every identity mention a deliberate, reviewed act.
- The guard is byte-level and dumb by design: it cannot judge whether prose is
  awf-flavoured (the glob-example class). Content neutrality beyond the two banned token
  classes remains a review concern.
- Adopter-facing rendered output changes (guide invariants line, working-with-awf command
  overview and glob examples, bootstrap comments); a changelog entry travels with the
  implementation.
- The commit that flips this ADR to `Implemented` adds the new invariant bullets to the
  agent guide's Invariants list (via `.awf/agents-doc.yaml` + `./x sync`) and regenerates
  `docs/decisions/ACTIVE.md` (`./x sync`), per standing convention.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Extend the ADR-0080 render sweep with residue regexes and more render units | Empty-data rendering never enters the `{{ with $.commitScopes }}` branch: it structurally misses the worst of the four leaks; widening the render loop to non-catalog units is more machinery for a weaker guarantee |
| Render sweep under populated data (seed fixtures so conditional branches render) | Guaranteeing branch coverage would require a populating fixture for every conditional in every template: a per-branch fixture matrix that rots exactly like the hand lists ADR-0080 eliminated; a source scan is total over all branches by construction |
| Narrow identity regex (allow full-slug/URL forms, ban bare mentions) | Fiddlier than two explicit exemptions and weaker: a new full-slug mention anywhere would pass silently instead of failing for review |
| No guard; rely on review | The four leaks survived every review that touched their templates; this class is exactly what deterministic checks are for |
| Ban only ADR citations, skip the identity rule | Identity mentions are the same leak class with the same review blindness; the two legitimate sites are cheap to exempt explicitly |
