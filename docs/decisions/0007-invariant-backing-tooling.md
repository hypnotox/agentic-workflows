---
status: Accepted
date: 2026-06-25
supersedes: []
superseded_by: ""
tags: [tooling, workflow]
related: [0005, 0006]
---
# ADR-0007: Invariant-Backing Tooling — `inv:` Tags and the `awf invariants` Checker

## Context

Every ADR declares an Invariants section of checkable contracts, but nothing verifies that
each invariant is actually backed by a test. ADR-0005 and ADR-0006 ship their invariants as
prose; a reader cannot tell which are machine-enforced and which are aspirational. The
`proposing-adr` skill already gestures at a `// invariant:` convention ("Each Invariants section
bullet must be accompanied by at least one `// invariant: <normalised bullet title>` test"), but
it is dormant (nothing runs it) and its "normalised bullet title" wording is a fragile
derive-from-prose mapping that breaks silently when a bullet is reworded.

ADR-0006 just landed the two foundations this needs: `internal/frontmatter` (the single
`---`-parser) and `internal/adr` (the ADR domain package, ready to extend with section parsing).
This ADR builds on them — it introduces no new parser.

Grounding discoveries (verified against source):

- **`internal/adr` extends cleanly.** `ADR` (`internal/adr/adr.go`) carries
  `Number/Title/Status/Filename/Path`; `parse()` already has the post-frontmatter body in hand
  (the first return of `frontmatter.Parse`). Adding a `Sections map[string]string` field is purely
  additive — `RenderActiveMD`/`ParseDir` callers read only the existing fields.
- **The binary surface is greenfield.** `cmd/awf/main.go` has no `invariants` case; `runCheck`
  (`cmd/awf/check.go`) opens the project and prints `[]manifest.Drift`. `Project` exposes `Root`
  and `Cfg.DocsDir`, so a checker can compute `<docsDir>/decisions`. There are **zero** existing
  `// invariant:` comments in the repo.
- **All ADRs use a literal `## Invariants` heading**, so a `## `-delimited section parser keys on
  it reliably.
- **Retro-tag is opt-in per bullet (verified mapping).** Only some ADR-0005/0006 invariant bullets
  have a dedicated 1:1 test (e.g. ADR-0005's docsDir-default → `TestDocsDirDefaultsToDocs`,
  sync-generates → `TestSyncGeneratesActiveMDAndCheckDetectsStaleness`; ADR-0006's split-semantics
  → the `frontmatter` `Split` tests, check-invalid → `TestCheckDetectsInvalidFrontmatter`,
  all-templates → `TestAllTemplatesProduceValidFrontmatter`). Others are repo-audit or textual
  contracts with no clean unit test. The convention is therefore **opt-in per bullet**: an author
  tags the bullets they want machine-enforced; untagged bullets remain valid textual contracts.

**User constraint driving the design (verbatim):** "I want invariant tooling to be part of the
binary as well, so adopters can utilise it."

## Decision

1. **Explicit slug convention.** A checkable ADR invariant bullet carries an explicit slug tag
   `` `inv: <slug>` `` (slug matches `[a-z0-9-]+`); the backing test carries a `// invariant: <slug>`
   comment. The slug is the stable key — editing the bullet's prose never breaks the link. Bullets
   without a tag are untagged textual contracts, not machine-checked. Authors tag the bullets they
   want enforced.

2. **Extend `internal/adr` with section parsing.** `ADR` gains `Sections map[string]string`,
   populated in `parse()` by splitting the body on `## ` headings (additive; existing fields and
   callers unaffected).

3. **New `internal/invariants` package.** `Check(decisionsDir, root string) ([]Finding, error)`:
   parse ADRs via `adr.ParseDir`; for each ADR whose status is `Implemented`, extract `inv:` slugs
   from `Sections["Invariants"]`; scan `*.go` files under `root` (skipping `.git`, `vendor`,
   `node_modules`) for `// invariant: <slug>` comments; return a Finding for each Implemented-ADR
   slug with no backing test. A slug declared by more than one ADR is an error. Only `Implemented`
   ADRs are enforced — `Proposed`/`Accepted` may still be landing their tests, and `Superseded`
   invariants no longer bind.

4. **Binary surface.** A new `awf invariants` subcommand operates on the cwd project (like
   `check`): `project.Open` → `Project.CheckInvariants()` → `invariants.Check(<docsDir>/decisions,
   root)`; it prints findings and exits non-zero on any. `awf check` **also** calls
   `CheckInvariants` and fails when it reports findings — so the existing pre-commit `awf check`
   (which adopters already run) enforces invariant backing automatically. `invariants.Finding` and
   `manifest.Drift` stay distinct types: `runCheck` calls `Check` and `CheckInvariants` separately,
   prints each set in its own format, and exits non-zero when **either** is non-empty — no type
   merge, no shared printer.

5. **Convention surfaces.** Rewrite the `proposing-adr` skill's invariant-pairing rule to the
   explicit-slug form and state that `awf check` enforces it for Implemented ADRs; update
   `docs/decisions/template.md`'s Invariants section to show a `` `inv: <slug>` `` bullet; document
   the convention and the checker in `docs/decisions/README.md`.

6. **Retro-tag ADR-0005 and ADR-0006.** Tag each invariant bullet that has a dedicated test with
   `` `inv: <slug>` `` and add the matching `// invariant: <slug>` comment to that test; leave the
   textual/repo-audit-only invariants untagged. Running `awf invariants` on this repo then passes —
   the dogfood proof.

Applying this to awf's own repo (the retro-tagging, re-sync) is mechanical adopter work in the
plan, not a Decision commitment. This earns an ADR because it is load-bearing (new
`internal/invariants` package, new `awf` subcommand, new `check` behaviour, a new ADR-authoring
convention) and a plan because it is multi-commit.

## Invariants

Checkable contracts, tagged per the convention this ADR introduces (each is backed by an
`internal/invariants` test added at implementation):

- `` `inv: invariants-implemented-only` `` — `invariants.Check` derives required slugs only from
  ADRs whose status is `Implemented`; `Proposed`, `Accepted`, and `Superseded` ADRs contribute no
  required slugs.
- `` `inv: invariants-unbacked-detected` `` — an `Implemented` ADR invariant slug with no
  `// invariant: <slug>` comment in any scanned `*.go` file yields a Finding; the same slug with at
  least one such comment yields none.
- `` `inv: invariants-duplicate-slug` `` — the same `inv:` slug declared by two ADRs is reported as
  an error.
- `` `inv: invariants-in-check` `` — `awf check` reports failure when `CheckInvariants` returns any
  Finding and is clean when it returns none.
- `` `inv: adr-sections-parsed` `` — `adr.ParseDir` populates `ADR.Sections["Invariants"]` with the
  body of the Invariants section for an ADR that has one.

## Consequences

Easier:
- Invariant backing is machine-enforced: an Implemented ADR cannot claim a tagged invariant
  without a test, and `awf check` (hence the pre-commit hook) catches a missing one.
- Adopters get `awf invariants` and the check-fold for free with the binary — the tooling is the
  standard, not a per-project script.
- Readers can tell at a glance which invariants are enforced (`inv:`-tagged) versus textual.

Harder / accepted trade-offs:
- ADR authors add a slug tag plus a test comment for each enforced invariant; the `proposing-adr`
  rule makes this explicit.
- `awf check` carries a second concern (invariant findings alongside render/ACTIVE.md drift); they
  are reported together and either failing fails the command.
- The `*.go` scanner is a line-comment regex, so a `// invariant: <slug>` string embedded in a Go
  string literal (e.g. a test fixture) would register as a backing tag. Accepted on two fronts.
  (a) *Test fixtures:* fixture slugs are kept distinct from real ADR slugs, and the
  `internal/invariants` tests scan temp dirs, not the repo, so no real slug is satisfied by a
  fixture. (b) *The scanner's own source (the dogfood run, Decision 6):* `awf invariants` run on
  this repo scans `internal/invariants` and the extended `internal/adr`, whose source carries the
  `// invariant:` marker as part of the regex pattern literal — but that literal is the *pattern*
  (`// invariant: ([a-z0-9-]+)` or similar), not a concrete slug, so it captures no real ADR slug
  and cannot spuriously satisfy one. The mitigation therefore holds for the production self-scan, not
  only for unit tests.
- Enforcement is per-bullet opt-in and `Implemented`-only by design; an un-tagged or
  not-yet-Implemented invariant is not machine-checked.

Doc-currency obligations the implementing commit(s) must satisfy:
- `proposing-adr` skill, `docs/decisions/template.md`, and `docs/decisions/README.md` describe the
  `inv:`/`// invariant:` convention and the checker.
- When this ADR flips to Implemented, `./x sync` regenerates `ACTIVE.md`. No
  `docs/decisions/README.md` index row is owed (this repo's README is a how-to guide; `ACTIVE.md`
  is the generated index — per ADR-0003/0004).

Downstream: future ADRs follow the slug convention; the `proposing-adr` "Pair each Invariant with
a test" step becomes a real, enforced gate.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Derive the slug from normalised bullet prose (the dormant wording) | Fragile: rewording a bullet silently breaks the mapping and the checker cannot tell. An explicit `inv:` tag is a stable key. |
| `awf invariants` standalone only, not folded into `check` | Adopters would not get automatic enforcement through the pre-commit `awf check`; the user wants the tooling usable (and enforced) out of the box. |
| Enforce on `Accepted` ADRs too, not just `Implemented` | Tests land with implementation; an Accepted ADR mid-implementation would fail the gate before its tests exist. `Implemented` is the point where all tagged invariants must be backed. |
| Require every invariant bullet to be tagged and tested | Some invariants are repo-audit (e.g. "no `./x adr` target") or textual contracts with no clean unit test; forcing tags would manufacture fake tests or block legitimate prose invariants. |
| AST/Go-aware scan instead of a regex | The marker is a line comment; a regex over `*.go` is simple and language-portable, and the string-literal false-positive is negligible and mitigated. |
| Keep the check as a Go test (the pre-ADR-0005 pattern) instead of the binary | A test only guards awf's own repo; the user requires adopters get the checker via the installed binary. |
