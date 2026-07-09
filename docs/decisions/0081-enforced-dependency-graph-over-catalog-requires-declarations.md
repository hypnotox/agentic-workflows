---
status: Proposed
date: 2026-07-09
supersedes: []
retires_invariants: [doc-gated-skill-suppressed]
superseded_by: ""
tags: [dependency-graph, cli, catalog, validation, migration]
related: [13, 24, 31, 46, 49, 50, 68, 76, 77, 80]
domains: [config, rendering, tooling]
---
# ADR-0081: Enforced dependency graph over catalog Requires declarations

## Context

ADR-0080 made the chain's coupling machine-readable — `RequiresSkills` on
`catalog.SkillSpec` and `catalog.TargetSpec` — but deliberately deferred gated
enforcement: the field feeds tests only, and a project can still enable a skill
without its referenced siblings, learning only at `awf check` time via the
ADR-0046 dead-reference drift. The three `Requires*` fields today carry three
*different* enforcement strengths:

- `RequiresAgent` (ADR-0050): hard error at project open; `awf add skill`
  auto-enables the agent; `awf remove agent` refuses upfront.
- `RequiresSkills` (ADR-0080): check-time dead-reference drift only.
- `RequiresDoc` (ADR-0013 Decision item 4): silent render suppression plus an
  add-time advisory note — a skill enabled without its doc is a valid state
  that renders nothing.

The user directed full dependency-graph handling. One structural fact shapes
the design: **the `RequiresSkills` closure is cyclic** — the 11-skill closure
(the 10 `Chain`-flagged skills plus `adr-lifecycle`) condenses to two
mutually-requiring cores, a 5-skill planning core (`proposing-adr`,
`reviewing-adr`, `writing-plans`, `reviewing-plan`, `reviewing-plan-resync`)
and a 3-skill execution core (`executing-plans`,
`subagent-driven-development`, `reviewing-impl`), with edges only from the
planning core toward the execution core, plus `brainstorming` as a pure
source and `retrospective`/`adr-lifecycle` as pure sinks. Most closure
members are therefore unremovable one-at-a-time — every core member's
removal is blocked by its co-members — and graph handling must answer unit
removal explicitly. This is the mechanized form of the agent guide's
"disable them as a unit".

Grounding facts (verified 2026-07-09): `awf add`/`awf remove` dispatch on raw
`len(args)` in `cmd/awf/main.go` (a flag needs positional-count rework, though
the `argSpecs` bool-flag machinery exists); `runUpgrade` never opens the
project before migrating (`ProjectPresent → GateState → Upgrade → runSync`),
so a migration can repair states the new validation refuses; `awf init`'s
curated core default is already closure-closed; the evals fixture enables the
full catalog and is trivially closed; the migration editor precedent is
schema 7's `AnchorNoSlashGlobs` via the atomic `editConfig` path (ADR-0076,
ADR-0077); and the render-time suppression predicate (`skillDocGateOpen`,
`effectiveSkills`) also anchors ADR-0046's `skills-context-effective-set`
invariant, so folding docs into the graph reaches that ADR too.

## Decision

1. **Typed edge layer in `internal/catalog`.** A `Node{Kind, Name}` model
   (kind ∈ `skill`, `agent`, `doc`; docs are pure sinks — `DocEntry` declares
   no requirements) with a single edge enumerator `RequiresOf(cat, node)`
   reading the three `Requires*` fields, and a pure forward `Closure(cat,
   seeds)` walk (visited-set, cycle-safe). This is the only place edges are
   enumerated; the ADR-0080 test fixture `chainClosureConfig` retires its
   inline walk in favor of it. Project-local artifacts (ADR-0068 `Base`
   entries) declare no requirements and are leaves.

2. **Resolver with plan/apply in `internal/project`.** `ResolveAdd` /
   `ResolveRemove` compute a **plan** — ordered `PlanOp`s (`{Node, add|remove,
   requiredBy}` provenance) — that leaves the enabled set closed: an add-plan
   is the requested node plus its missing forward closure; a remove-plan is
   the requested node plus its enabled transitive dependents (reverse
   closure). Plans are applied in **one** config rewrite (the ADR-0050
   `rewriteConfig` multi-edit), and every applied or refused plan prints one
   line per op with its `requiredBy` provenance.

3. **Hard closure validation at project open.** `validateAgainstCatalog`
   checks every enabled, non-`local` artifact's **direct** catalog
   requirements are enabled (transitive closure follows by induction), failing
   every gated command with a repair hint that names the exact config edit and
   `awf upgrade` as the pre-migration recovery path. This generalizes ADR-0050
   Decision item 2 (its `RequiresAgent` check becomes the skill→agent edge of
   the same loop) and supersedes ADR-0046 Decision item 4 in two respects: the
   check-time failure strengthens to open time, and its documented
   trim-with-overrides escape hatch (disable a chain skill, override the
   referencing sections via convention parts) is **deliberately foreclosed** —
   validation reads catalog edges, not rendered output, so a trimmed config
   is refused regardless of part overrides and the schema-8 migration
   re-completes it. The sanctioned escape for a deliberate trim is the
   `local: true` sidecar: the validator skips local artifacts, so an adopter
   takes ownership of the referencing skill and trims freely. The self-repair
   trap is accepted as under ADR-0050: a hand-broken config locks out the CLI
   (including `awf add` and `awf new`) and is repaired by hand or by
   `awf upgrade`.

4. **`awf add` applies the closure plan.** Adding any artifact enables its
   full missing forward closure — skills, agents, **and docs** — in one
   rewrite, printing the plan. This generalizes ADR-0050 Decision item 5 and
   subsumes the ADR-0013 add-time "will not render" advisory note, which is
   deleted: the doc is now simply enabled.

5. **`awf remove` refuses with the dependent plan; a cascade flag applies
   it.** Plain remove refuses whenever the remove-plan exceeds the named
   artifact, printing the full dependent plan (so the cycle's true size is
   visible before any change); `awf remove <kind> <name> --with-dependents`
   applies the whole plan in one rewrite. This generalizes ADR-0050 Decision
   item 4 (the agent guard becomes the reverse walk's length-1 case) and
   covers docs (`awf remove doc roadmap` refuses while `roadmap-graduation` is
   enabled). Consequence stated plainly: cascade size is **seed-dependent** —
   up to 10 of the 11 closure skills plus `plan-reviewer`, depending on where
   the removal starts (a planning-core member pulls the planning core and
   everything upstream of it; a sink like `retrospective` pulls nearly the
   whole closure; `brainstorming`, a pure source, cascades nothing). Agents
   left with no requiring skill stay enabled — agents are legal standalone
   (ADR-0050 Decision item 3 unchanged) — with a note, and the existing
   orphaned-sidecar note loops over every removed plan node.

6. **`--dry-run` on add and remove** prints the computed plan without
   touching the config — the resolver's plan/apply split makes it free.

7. **`RequiresDoc` joins the hard graph; render suppression is removed.**
   A doc-gated skill without its doc becomes a *refused config state*
   (Decision 3), so the ADR-0013 Decision item 4 suppression semantics are
   superseded: the render-time gate (`skillDocGateOpen` and the suppression
   branch in `effectiveSkills`) is deleted and `inv:
   doc-gated-skill-suppressed` retired (`retires_invariants`, ADR-0031). The
   render context's effective skills set becomes exactly the enabled set
   (local synthesis unchanged) — amending the *semantics* of ADR-0046's `inv:
   skills-context-effective-set` in place (partial-item supersedence; the slug
   and marker survive on the simplified code). `local: true` doc sidecars are
   orthogonal: validation reads the enable array, and a locally-owned doc
   still satisfies the edge.

8. **Schema-8 migration `close-enabled-set`.** Two ordered steps: **first**,
   every **dormant doc-gated skill** — enabled while its doc is disabled,
   today's valid silent-suppression state — is dropped from the enable array,
   preserving the adopter's observed rendered output (the one non-additive
   step); **then** the additive fixed point runs over **all three edge
   kinds**, adding every enabled artifact's missing skill, agent, and doc
   requirements — so a dormant skill that something enabled still requires is
   re-added *with its doc* (the closure demand outranks the dormancy drop),
   and a closure-added doc-gated skill can never leave the migrated config in
   a state Decision 3 refuses. Every addition and drop is printed. The migration is idempotent, edits the config via the atomic
   `editConfig` path (ADR-0076), mirrors the validator's `local:` sidecar
   skip so it never adds an edge validation would not demand, and lands with
   a `minVersionBySchema[8]` entry and `project.Version` bump (ADR-0049).
   `internal/migrate` gains its first `internal/catalog` import (no cycle);
   its package doc comment is reconciled.

9. **`awf init` derives agents from the trim, then closes the selection
   silently.** A catalog trim (interactive or `--answers`) first derives its
   agent set from the trimmed skills' requirements — agents nothing in the
   selection requires are dropped, superseding ADR-0050 Decision item 6's
   unconditional all-agents scaffold — and is then closure-completed (missing
   requirements added, each noted) instead of scaffolding a config that
   init's own chained sync refuses. Without the agent derivation, the
   always-enabled `plan-reviewer`'s edge to `reviewing-plan-resync` would
   silently re-complete any planning-core trim, making the skills trim
   meaningless for chain members. The untrimmed curated default keeps all
   agents (a default, not a derived set) and is already closed; a test locks
   that.

## Invariants

- `inv: enabled-set-closed` — every enabled, non-`local` artifact's direct
  catalog requirements (`RequiresSkills`, `RequiresAgent`, `RequiresDoc`) are
  enabled; a violation fails project open, with a repair hint.
- `inv: add-applies-closure-plan` — `awf add` enables the requested artifact's
  full missing forward closure in a single config rewrite, printing one
  provenance line per plan op.
- `inv: remove-refuses-dependents` — without the cascade flag, `awf remove`
  refuses while enabled transitive dependents exist, printing the dependent
  plan; with `--with-dependents` it removes the full reverse closure in a
  single rewrite.
- `inv: close-enabled-set-migration` — the schema-8 migration closes the
  enabled set additively for skill/agent requirements, drops dormant
  doc-gated skills, and is idempotent and atomic.
- `inv: init-set-closed` — `awf init`'s scaffolded enabled set (curated
  default or closure-completed trim) satisfies `enabled-set-closed`.
- Re-anchored, not retired: `reviewing-skill-agent-pairing`,
  `add-skill-pairs-agent`, `remove-agent-pairing-guard` move with the code
  they back into the generalized graph sites; `skills-context-effective-set`
  survives with amended semantics (effective = enabled); ADR-0050's
  `reviewing-skill-specs-paired` (a catalog-shape rule) is untouched.

## Consequences

- The three `Requires*` fields get one uniform enforcement model; "disable
  them as a unit" stops being prose and becomes mechanized, including its
  blunt edge — cascading a core member removes up to 10 closure skills plus
  `plan-reviewer`, which will surprise users; the printed plan before any
  change is the mitigation.
- **Breaking for adopters** (changelog: Breaking): configs that today pass
  gated commands while failing `awf check` (missing chain siblings), and
  configs using dormant doc-gated skills, refuse at open after upgrading the
  binary; `awf upgrade` is the sanctioned repair, and the schema gate on every
  command points to it. Dropping a dormant skill removes a line the adopter
  wrote in their config — accepted as the least-surprise reading (their
  rendered output is unchanged). The additive closure conversely
  **materializes new rendered skill/agent files** in an adopter's repo on
  upgrade — accepted where the doc equivalent was rejected because skills and
  agents are the workflow machinery the config already claimed to want, while
  docs are user-facing content the adopter deliberately curates.
- ADR-0013's suppression machinery is deleted rather than kept as
  defense-in-depth — validation is the single owner of the invalid state; the
  effective-skills simplification ripples into `.skills` context, config
  hashes, and dead-reference checking, and adopters with a previously
  suppressed skill will see referencing artifacts reflag once (settled by the
  migration's terminal sync).
- Implementation sequencing constraints (for the plan): the suppression code
  and its `doc-gated-skill-suppressed` marker are deleted in the commit that
  flips this ADR to Implemented (retirement counts only from an Implemented
  successor, else `./x check` reds mid-effort); the catalog edge layer must
  land in the same commit as its first resolver consumer (the dead-code gate
  runs without `-test`).
- The add/remove CLI grows its first flags; the `len(args)`-keyed dispatch in
  `cmd/awf/main.go` is reworked to filter flag tokens, and help text gains the
  flag forms.
- The commit that flips this ADR to Implemented also adds the new invariant
  bullets to the agent guide's Invariants section (via `.awf/agents-doc.yaml`
  + `./x sync`), regenerates `docs/decisions/ACTIVE.md` (`./x sync`), and adds
  81 to the `related:` frontmatter of ADR-0013, ADR-0046, and ADR-0050 — the
  partial-amendment forward pointers (`docs/pitfalls.md`).
- Prose this ADR obsoletes updates in the same commits that change the
  behavior: the agent-guide awf-setup sentence "disable them as a unit rather
  than piecemeal, or a handoff will point at a skill that isn't enabled"
  (rendered into every adopter's AGENTS.md) is rewritten for the mechanized
  world (`awf remove` refuses or cascades for you), and the working-with-awf
  doc's command section gains the `--with-dependents` and `--dry-run` forms
  alongside the CLI help text.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Check-time enforcement only (keep ADR-0046 drift as the oracle) | Leaves `RequiresSkills` weaker than `RequiresAgent` forever; a silently-thinner chain is the failure mode the workflow exists to prevent (user chose hard-at-open). |
| Refuse-only remove (no cascade flag) | The cyclic chain makes every member permanently unremovable one-at-a-time; "full graph handling" that cannot remove a unit is diagnosis without treatment. |
| Multi-name remove instead of a flag | Forces users to type the 11-member unit by hand; the resolver already knows the set. |
| Keep `RequiresDoc` advisory (suppression semantics) | Two enforcement models over one field family; suppression made "enabled" ambiguous (enabled-but-renders-nothing). User chose folding it in. |
| Three independent in-place walks (no resolver) | Triplicated graph logic in validate/add/remove — the divergence-prone duplication this project keeps promoting checks against. |
| Additive migration for dormant doc-gated skills (enable the doc) | Resurrects a skill the adopter effectively didn't have and materializes new docs in their repo on upgrade; dropping the dormant skill preserves observed output (user chose drop). |
| Init rejects an unclosed trim | Breaks non-interactive `--answers` scripting on any near-miss; silent closure matches add and migration semantics. |
| Keep the ADR-0046 trim-with-overrides escape hatch | Requires render-aware validation (edges conditioned on part overrides) — complex, fragile, and a second source of truth beside the catalog; `local: true` ownership covers the deliberate-trim case. |
| Init keeps the all-agents scaffold under a trim | plan-reviewer's skill edge would closure-complete any planning-core trim right back — the trim dimension would be decorative. |
| Validation-lenient open for add/remove (self-repair) | A second Open mode weakens the single validation choke point for one rare, hand-induced state that `awf upgrade` or a hand edit already repairs. |
