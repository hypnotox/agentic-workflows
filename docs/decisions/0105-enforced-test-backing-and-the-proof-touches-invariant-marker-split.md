---
status: Proposed
date: 2026-07-13
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [invariants, testing, config]
related: [7, 8, 31, 64, 77, 104]
domains: [invariants, config, tooling]
---
# ADR-0105: Enforced Test-Backing and the Proof-Touches Invariant Marker Split

## Context

An invariant is backed today when a `<marker> invariant: <slug>` comment for a declared
Implemented-ADR slug appears at the start of a line in any source file matching
`invariants.sources` (ADR-0008, ADR-0064). This is a *presence-of-string* check. It reliably
catches one failure — a declared slug with **zero** backing anywhere — and it powers the
`awf context` Tier-1 governance join (ADR-0104). But it guarantees nothing about the code the
marker sits on. The marker is a comment: deleting the enforcing production logic while leaving the
comment passes `awf check`. In this repo 350 markers span 121 files, only 70 of which are test
files — a large share are inert production annotations that would survive their own logic being
removed. Markers are now placed by the implementing agent (the same agent closing the task), so the
"conscious backing" the original design assumed is weaker than intended: the system is an
accountability index, not a proof.

Two further frictions compound this:

- **A cosmetic token split.** The ADR side declares `inv: <slug>` while the source side backs with
  `invariant: <slug>`. The corpora are disjoint and the anchors differ, so the two tokens carry no
  semantic distinction — only cognitive load ("why two names for one concept?").
- **ADR-0008 Decision item 4** ("Untestable invariants stay untagged … every tagged slug is
  enforced; there is no per-slug exemption mechanism") forces a genuinely-untestable but
  load-bearing property — e.g. `context-read-only`, structural single-source properties — to shed
  its slug and become prose. That removes it from the `awf context` governance signal precisely when
  a reader most needs to know it exists.

Invariant backing is a load-bearing input to the context system, which the project values highly.
Strengthening its teeth and enriching what it feeds context are one effort. This ADR owns the
backing-model redesign; a companion ADR amends ADR-0104 for how the two markers surface in context.

The unifying token is a public surface of the standard: adopters type `inv:` into ADRs that awf does
not render or migrate (`docs/decisions/**` is user-owned; schema migrations only touch the `.awf/`
tree), and adopters place source markers awf likewise cannot rewrite. The token rename and the
marker relocation are therefore a two-front, un-auto-migratable break — cheap now (pre-1.0, two known
adopters) and effectively frozen after 1.0.

## Decision

1. **Unify the declaration token to `invariant:`.** The ADR Invariants-section declaration token
   becomes `invariant: <slug>` (leading a markdown list item), matching the source proof marker
   verbatim. `declRe` is updated to recognise `invariant:` in place of `inv:`; the source-side
   `invariant:` marker is unchanged. This is a corpus-wide rewrite of ADR declarations, carried by
   the mechanism plan (self-applying: this ADR's own declarations migrate with the rest).

2. **Two source markers with a strong/weak gradient.**
   - **`invariant: <slug>`** — the *proof* marker. It backs a slug only when it sits in a file
     matching a configured **test glob** (item 3). This is the authoritative, enforced marker; the
     plain token carries the strong meaning so the reflexive choice is the enforced one.
   - **`touches-invariant: <slug>[ — <note>]`** — the *context* marker. It never satisfies backing
     and never fails `awf check`; it records a code site that relates to the invariant, with an
     optional free-form note (everything after the slug, trimmed) describing *how* the site relates.
     It may appear any number of times, anywhere a source glob matches (typically production code).
     "Touches" is deliberately broad, to invite annotating tangentially-related sites.

3. **Add a `testGlobs` config surface for structural teeth.** `InvariantConfig` gains
   `TestGlobs []string` (`yaml:"testGlobs"`), anchored doublestar globs matched against the
   slash-separated repo-relative path exactly as `invariants.sources[].globs` (ADR-0077).
   `Config.Validate` rejects a malformed or path-separatorless `testGlobs` pattern on the same rule
   as source globs. A proof `invariant:` marker counts as backing **only** when its file matches
   both a source glob (so the source marker is scanned) and a `testGlobs` pattern. awf's own config
   sets `testGlobs: ['**/*_test.go']`.

4. **Backed vs. unbacked invariants (a sanctioned exemption).** A declared slug is **backed** — it
   must have ≥1 proof marker in a test-glob file — unless it appears in its declaring ADR's
   `unbacked_invariants: [<slug>]` frontmatter, mirroring `retires_invariants:` (ADR-0031). An
   **unbacked** slug is exempt from the proof requirement: it is declared, may carry
   `touches-invariant:` markers, and remains context-tracked, but `awf check` never reports it as
   Unbacked for lacking a test proof. An `unbacked_invariants:` entry naming a slug no Implemented
   ADR declares is a dangling exemption and fails `awf check`, mirroring dangling-retirement. **This
   supersedes ADR-0008 Decision item 4** via partial-item supersedence: ADR-0008 stays
   `Implemented`/live and this ADR carries `related: [8]`; untestable invariants may now keep their
   slug as explicitly-unbacked rather than being demoted to prose.

5. **Dangling-marker advisory.** A source `invariant:` or `touches-invariant:` marker naming a slug
   that no Implemented ADR declares yields a non-failing advisory note keyed by output path
   (routed through the existing `awf check` `note:` channel, ADR-0070), never a hard finding — so a
   stray or renamed slug is surfaced without blocking the gate. A `touches-invariant:` marker with
   no note is likewise an advisory note (the note is the payload; a bare touches-marker is
   low-signal), never a failure.

6. **De-Go-ify preserved.** `testGlobs` is glob-and-string only; nothing assumes Go. Adopters
   configure their own test-file globs (`**/*_test.py`, `**/*.spec.ts`). The standard surfaces
   (`proposing-adr` skill, `docs/decisions/template.md`) update to the unified `invariant:` token and
   the proof/touches vocabulary with per-language examples.

## Invariants

Checkable constraints that must hold while this decision stands. Tagged bullets are enforced once
this ADR is `Implemented`; each is backed by a proof marker in a test file under the new model.

- `inv: proof-marker-test-scoped` — a proof `invariant: <slug>` marker backs a declared slug only
  when its file matches a configured `testGlobs` pattern; the identical marker in a non-test source
  file does not back the slug.
- `inv: touches-marker-advisory` — a `touches-invariant: <slug>` marker never satisfies the backing
  requirement and never produces a hard `awf check` finding, regardless of whether the slug is
  otherwise backed.
- `inv: unbacked-exempt-from-proof` — a slug listed in its declaring Implemented ADR's
  `unbacked_invariants:` frontmatter is exempt from the proof requirement: it is reported neither
  Unbacked nor Unchecked for lacking a test proof.
- `inv: dangling-exemption-refused` — an `unbacked_invariants:` entry naming a slug no Implemented
  ADR declares fails `awf check`, mirroring dangling-retirement.
- `inv: dangling-marker-advisory` — a source `invariant:`/`touches-invariant:` marker naming a slug
  no Implemented ADR declares yields a non-failing advisory note, never a hard finding.
- `inv: bare-touches-note` — a `touches-invariant:` marker carrying no note yields a non-failing
  advisory note, never a hard finding.
- `inv: testglobs-anchored-validated` — `Config.Validate` rejects a `testGlobs` pattern that is
  malformed or contains no path separator, on the same anchored-glob rule as `invariants.sources`.

## Consequences

- **Backing gains real teeth.** Backing now means an *executed test line* (test-glob file + the
  100%-coverage gate, ADR-0012), not any comment. The semantic gap — whether the test asserts the
  right thing — remains unclosable trust, but the floor rises from "a string exists" to "a test
  exercises this."
- **Context becomes a risk map.** With backed/unbacked explicit, `awf context` can tell an editing
  agent which governing invariants auto-trigger on breakage and which need manual reasoning
  (surfaced by the companion ADR-0104 amendment).
- **A large, deliberate migration.** ~27% of this repo's declared slugs (73 of 268) are backed only
  in production today; each needs a proof marker added on its test and its production annotation
  rewritten to `touches-invariant:`. This is sliced into its own plan behind the mechanism, landing
  additively (both markers accepted, `testGlobs` optional) before enforcement flips to hard failure.
- **A two-front adopter break.** Adopters must rename `inv:`→`invariant:` in their ADRs and relocate
  production `invariant:` markers to tests + `touches-invariant:`; neither is auto-migratable. The
  schema migration can only emit an advisory note. Accepted as a pre-1.0 cost; this ADR owns the
  migration-note obligation.
- **`Check` return shape widens.** It must return advisory notes alongside hard findings, so
  dangling-marker and bare-touches notes flow to the `note:` channel. `MarkersUnder` (context) must
  scan the union of `sources` and `testGlobs`, or splitting the globs would silently narrow Tier-1
  governance — this repo's current single `**/*.go` source already includes test files.
- **New branches to cover.** proof-only, touches-only, `testGlobs`-set-but-`sources`-empty,
  dangling-marker, bare-touches, and `Disabled`/`Unchecked` interactions each need an explicit test
  under the 100% gate.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep one marker, add teeth by requiring markers on covered lines | Requires ingesting coverage data into the checker — heavy, and awf ingests none. Glob-based test-scoping is language-neutral and cheap. |
| Plain `invariant:` = context, `backing-invariant:` = proof (initial sketch) | The plainest token is the reflexive choice; making it the *weak* one turns it into an attractive nuisance (drop it on prod, feel "backed", never write the proof). Flipped so plain = proof. |
| `affects-invariant:` for the context marker | "Affects" reads as a causal, enforced-ish claim it isn't; "touches" is honestly advisory and lowers the bar to annotate tangential sites, which enriches context. |
| Hold ADR-0008 item 4 — untestable invariants stay untagged prose | Removes load-bearing-but-untestable properties from the `awf context` governance signal, working against the context goal. The `unbacked_invariants` exemption keeps them visible while honest about their unproven status. |
| Distinct inline bullet token for unbacked declarations | Locally visible but unprecedented; `unbacked_invariants:` frontmatter reuses the proven `retires_invariants:` pattern and keeps every Invariants bullet uniform. |
| One ADR for the whole redesign incl. the context change | Couples a context-relevance decision to the backing-model decision; split into this ADR + an ADR-0104 amendment, one decision each. |
