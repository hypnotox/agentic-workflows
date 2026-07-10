---
status: Proposed
date: 2026-07-10
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [rendering, advisory, config, adoption]
related: [22, 26, 29, 34, 45, 49, 57, 84, 86]
domains: [rendering, config]
---
# ADR-0087: Deletion-as-acknowledgement for unset-var notes

## Context

The unset-var advisory ([ADR-0045](0045-out-of-box-render-completeness.md) Decision 4)
prints a non-failing note per rendered artifact that references unset vars. "Unset" today
means *missing key or empty value* — `unsetVarNotes` indexes the vars map without the
presence flag (`internal/project/check.go:52`), so the two states are indistinguishable.
An adopter who considers a var and deliberately declines it has no way to say so: the
note prints on every `awf check` and `awf init`, forever. The fleet repository carries
two such standing `invariantTestPath` notes. Permanent unactionable output trains readers
to skim the notes channel, which buries the one note that is new and matters.

ADR-0045 saw this coming and pre-sanctioned the fix path — its Consequences accept that
"advisory notes are unsuppressible" and state: "a suppression knob, if the noise proves
real, is its own ADR". The noise has proven real. This is that ADR, and it deliberately
avoids a suppression *knob*: the user direction is that removing the key should be the
acknowledgement ("if not necessary, just remove it") — no second config surface listing
exceptions to the first.

Grounding discoveries that shape the design:

- `Cfg.Vars` is `map[string]any`; YAML `key:` (explicit null) parses to a *present* key
  with a nil value, distinguishable from an absent key only via the map `ok` flag —
  mechanically verified. A present-null var is unrepresentable by awf's own tooling
  (`Skeleton.Vars` is `map[string]string`, [ADR-0026](0026-config-scaffold-serialization.md)
  Decision 3); it arises only from hand-editing.
- The notes pipeline has exactly two consumers: `awf check` (`cmd/awf/check.go:28`) and
  `awf init`'s post-scaffold orientation (`cmd/awf/init.go:120`). No other command prints
  advisories.
- `ScaffoldConfig` seeds the union of vars referenced by *every* catalog template as `""`
  (`invariant: scaffold-seeds-all-vars`, [ADR-0022](0022-curated-init-default.md) /
  [ADR-0029](0029-interactive-agent-prefillable-init.md)), so on a scaffolded tree a
  catalog var can never be absent-at-init — absence is always a post-init edit. The
  premise "absence means someone deleted it" fails only for vars introduced *after* the
  adopter's init: a future release adding a catalog var, or a future local-artifact base
  template gaining a var reference (parts are raw and cannot reference vars, ADR-0034 —
  the base templates are the local artifacts' only var channel, varless today).
- Referenced vars fold into the artifact config hash (`internal/project/confighash.go:38`);
  an absent key marshals `null`, an empty one `""` — different hashes, mechanically
  verified. Deleting a key therefore flags every referencing artifact as failing stale
  drift until the next `awf sync`.
- Schema migrations are run-once per generation (lock `SchemaVersion` gate) with atomic
  config-rewriting precedent (anchored-globs, close-enabled-set). Adding a catalog var
  currently forces no schema bump — vars are freeform ([ADR-0084](0084-catalog-vars-carry-functional-values-only.md)) —
  so a seed step is the only forcing function, and it drags [ADR-0049](0049-single-version-authority.md)
  Decision 4 machinery (generation bump → `minVersionBySchema` entry → version bump).
  This is acceptable because `inv: var-descriptor-set-pinned` (ADR-0084) already makes
  any new catalog var a rare, deliberate successor-ADR act.
- `awf new skill|agent` scaffolds a declaring sidecar and a stub part, seeding nothing;
  the local base templates reference zero `.vars.*` tokens today. No config editor exists
  for writing a string vars key (`SetMappingScalar` writes booleans only).
- The ADR-0086 unused-var check skips empty and nil values (`check.go:145`), so a
  kept-empty key and a deleted key are both legal config states; but its code comment and
  test rationale say the exemption "mirrors the ADR-0045 unset definition", which this
  ADR un-mirrors.
- The sidecar *data* namespace declines a catalog default with a **present** null key
  (`inv: sidecar-key-overrides-default`, ADR-0045 Decision 2). This ADR gives vars the
  opposite convention — declining by *absence* — an asymmetry that must be owned, not
  discovered.

## Decision

1. **Present-key note semantics.** `unsetVarNotes` reports a var only when its key is
   *present* in `vars:` with an empty-string or null value, distinguished via the map
   presence flag. An absent key produces no note: deletion is the acknowledgement —
   committed, auditable in git history, reversible by re-adding the key. This narrows the
   note trigger of ADR-0045 Decision 4 (partial-item supersedence recorded via `related`;
   ADR-0045 stays Implemented). The graceful-degradation contract (ADR-0045 Decision 3)
   is untouched: for *rendering*, absent, null, and empty remain equivalent unset states.

2. **The note advertises the exit.** The note line gains a fixed suffix naming both
   resolutions, in the spirit of: "set a value, or delete the key to accept the generic
   prose". An acknowledgement mechanism nobody can discover does not fix alert fatigue.

3. **Seed-on-introduction convention.** A release that adds a catalog var ships, in a
   schema migration, a one-time seed step adding that key as `""` where absent. Run-once
   semantics come free from the generation gate, so a later deletion is never resurrected.
   This is a textual contract on future var-introducing ADRs (precedent: ADR-0084's
   textual policy), not standing machinery: there is no generic scan, and **no seed ships
   for the existing eight vars** — configs already lacking a referenced key flip from
   noting to silent, which is exactly the intended fleet fix.

4. **`awf new` seeds its scaffold's vars.** At creation, `awf new skill|agent` computes
   the vars referenced by the artifact's template source as scaffolded and adds each
   missing key as `""` to `vars:` via a round-trip-preserving config edit (a new
   string-valued vars-key editor in `internal/config`). A brand-new artifact has no
   history, so creation-time seeding is inherently one-time. With today's varless base
   templates this is a no-op; it makes future scaffolds correct by construction.

5. **Parts have no var channel, so Decision 4 closes the local surface.** Convention
   parts are raw input ([ADR-0034](0034-convention-parts-raw-not-templated.md)): a part
   body is never variable-interpolated and is sentinel-substituted out of the assembled
   source the advisory scans, so a part cannot create an unset-var note. The only
   part-side var mechanism is the closed `{{=awf:gateCmd}}`/`{{=awf:checkCmd}}`
   placeholder sandbox ([ADR-0057](0057-template-scoped-render-placeholders.md)), which
   hard-errors when its var is unset rather than noting. Creation-time seeding
   (Decision 4) therefore covers everything a local artifact can reference.
   `docs/working-with-awf.md` documents the deletion-as-acknowledgement exit and the
   placeholder deletion hazard. The implementing change flips this ADR's status and
   regenerates `docs/decisions/ACTIVE.md` via `./x sync` in its final commit.

## Invariants

- `inv: absent-var-acknowledged` — an absent `vars:` key never produces an unset-var
  note; a present key with an empty or null value does.
- `inv: new-seeds-scaffold-vars` — `awf new` adds an empty `vars:` key for every var its
  scaffolded template source references that is absent from config.
- Notes remain non-failing (`inv: completeness-advisory-nonfailing`, ADR-0045 — unchanged
  and still backed).
- Textual: a release introducing a new catalog var descriptor ships a one-time seed step
  for that key in a schema migration. Anchored where every var introduction must tread:
  the `inv: var-descriptor-set-pinned` test (ADR-0084) carries a comment citing this
  obligation, so extending the pinned set surfaces it; the AGENTS.md invariants entry for
  the new note semantics travels with the implementation per the standing pattern.

## Consequences

Easier:
- A deliberate "we don't use this var" is expressible, permanent, and auditable; fleet
  deletes `invariantTestPath` and its two standing notes end. The notes channel stays
  high-signal — every note has a concrete config edit that resolves it.
- No new config surface, no severity model, no suppression list to rot: the ADR-0086
  closed-tree posture ("configs should be clean") is reinforced rather than amended.

Harder / accepted trade-offs:
- **Acknowledgement is two steps.** Deleting a key changes the referenced-var config hash,
  flagging referencing artifacts as failing stale drift until `awf sync` — the normal
  edit-config-then-sync loop, documented alongside the exit in working-with-awf.
- **Deleting a placeholder-consumed var is loud, not silent.** A convention part using
  `{{=awf:gateCmd}}` hard-errors at render when `gateCmd` is deleted (ADR-0057
  publication-safety); deletion-as-acknowledgement applies to prose interpolations, not
  placeholder contracts. The error names the missing var; re-adding the key recovers.
- **One-time reinterpretation at upgrade.** Hand-written configs already lacking a
  referenced key stop noting after this change — an accidental omission is blessed as
  deliberate. Accepted: indistinguishable by construction, and the changelog documents
  the semantic shift.
- **Vars and data decline differently.** Sidecar data declines a default with a present
  null key; vars decline with absence. The asymmetry is owned here: data keys carry
  structured defaults where "explicitly empty" must be expressible distinctly from
  "fall through to default", while vars are flat strings where present-empty already
  means "open to-do" by the seeding contract.
- **Future catalog vars cost a migration.** Seed-on-introduction forces a schema
  generation + `minVersionBySchema` + version bump per var-introducing release. Accepted:
  ADR-0084 made such releases rare and deliberate.
- Present-null keys still note although awf tooling cannot create them (hand-edit only) —
  the "present = open to-do" reading is kept uniform rather than special-cased.
- Same-change updates owed: the `unusedVarDrift` comment and test rationale that cite
  "mirrors the ADR-0045 unset definition"; the rendering-domain current-state part that
  narrates the note; the config-domain current-state part, which states "empty seeds
  stay legal per the ADR-0045 unset definition" and describes `awf new` scaffolding
  (Decisions 1 and 4 both touch it); three `notes_test.go` fixtures that rely on absent
  keys to trigger notes.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| `acknowledgedNotes` config list | A second config surface enumerating exceptions to the first; rots without stale-failing machinery that deletion gives for free via git history. |
| Empty-string-means-acknowledged | Init seeds every referenced var as `""` (ADR-0022/0029); fresh scaffolds would be born fully acknowledged and the advisory would never fire for anyone. |
| Generic scan-at-upgrade seeding (referenced ∩ missing → seed) | Cannot distinguish a release-introduced var from a deliberately deleted one; resurrects every acknowledgement on every upgrade unless given memory. |
| Lock-recorded referenced-var union as that memory | Machinery solving a problem only the generic scan creates; per-release seed steps get run-once semantics free from the generation gate. |
| One-time seed of the existing eight vars at this release | Re-materializes the exact keys adopters like fleet want gone; contradicts the goal. |
| Suppression severity model on notes | ADR-0045 already rejected a severity model for one consumer; notes remain uniformly informational. |
| Present-null-means-acknowledged (data-namespace symmetry) | Unrepresentable by awf's own tooling (`Skeleton.Vars` is `map[string]string`, ADR-0026 Decision 3), contradicts the recorded user direction, and breaks the uniform "present = open to-do" reading. |
| No seed for future catalog vars (changelog-only announcement) | Silences the advisory for exactly the var nobody has considered yet, gutting the discovery premise; the migration cost is rare and already ADR-gated. |
