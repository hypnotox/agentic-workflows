---
format: plan-v2
date: 2026-08-06
adrs: [house-standard-configuration-expresses-repo-facts-only, unconditional-gates-and-audit-rules, cli-grammar-expresses-creation-and-inventory]
status: Proposed
---
# Plan: Collapse The Configuration Surface To Repo Facts

## Goal

Reduce `.awf/config.yaml` to keys whose steady-state values differ between the repositories awf
serves. Every catalog skill, agent and doc renders unconditionally for both fixed targets under a
fixed `docs/` root; the prose gate, the memory-citation gate, the hook payloads, the `./awf` wrapper
and the five audit advisories always run; the selection commands retire and domains move under
`awf new` and `awf remove`. Non-goals: `bootstrap.enabled` is untouched and keeps its conditional
render path, the convention-part and sidecar shaping surface is untouched, and ADR-0243's runtime
judgment triggers are out of scope.

## Architecture summary

Three inline phases, ordered by what each one makes possible.

Phase 1 lands the gates record whole: it removes only keys whose consumers are local to the gate,
payload and audit paths, so it needs nothing from the other records. Phases 2 and 3 split the
selection retirement, which cannot be split by compilation: removing a `Config` array field forces
every consumer to change in the same commit. Phase 2 therefore changes the render path to emit the
whole catalog while the fields still exist and still parse, proving the new render behaviour under
test; Phase 3 then deletes the fields, the commands, the resolver, the local-artifact channel and
the requirement graph in one transaction. Each phase carries the authored and generated prose its
behaviour change invalidates in the same closing commit.

Dependency direction only loses edges. `internal/project` stops reading the enable arrays from
`internal/config`, `internal/catalog` stops exposing a requirement graph, and `cmd/awf` stops
holding an enablement resolver. Nothing gains a dependency.

Two records apply incrementally. The house-standard record enters `Implementing` in Phase 2 with its
render-set operations and applies the remainder in Phase 3; the gates and CLI records each apply in
one batch in their own phase. No phase appends `Implemented`: the terminal status-only transaction
belongs to effort finalisation after implementation review settles.

Schema generations: the gates record takes 38, the house-standard record takes 39. The CLI record
needs none, because every key its retired commands wrote is retired by another record and
`bootstrap.enabled` survives.

All three records are pending slug records. Claim provenance is written in slug form throughout
implementation (`Origin: ADR-unconditional-gates-and-audit-rules`), and `awf adr number` substitutes
the assigned numbers at integration under the digest-paired rename rules. No task assigns a number.

Retiring a config field is never only a schema edit. `internal/migrate` reads the retired shape from
the typed struct in steps that must stay frozen, and `loadForMigration` strict-parses the live tree
for every analysis migration, so each retirement carries three obligations: a historical view for
the frozen readers, a `loadForMigration` strip so an old tree still upgrades, and a
`retiredKeyRemovals` entry so historical bytes still parse. Tasks 1.4, 3.1 and 3.5 carry them.

## Phase 1: Unconditional gates and audit rules

**Execution mode: inline.**

Advances: ["records-applied"]
Completes: ["gates-unconditional"]

### Task 1.1: Fix the audit thresholds in the binary
Latitude: exact
Applying: ["unconditional-gates-and-audit-rules:audit-thresholds-fixed", "unconditional-gates-and-audit-rules:audit-advisories-always-run"]

In `internal/audit/settings.go`, delete the `*config.AuditConfig` parameter from `Resolve` and every
field it populated from configuration except the scope list. `Settings` keeps `AllowedScopes`;
delete the `AllowedTypes`, `DependencyManifests`, `SubjectMaxLength`, `DiffThreshold`,
`DomainDocStaleness`, `DomainCodeStaleness`, `UndocumentedDomain`, `PlainPunctuation` and
`UncommittedChanges` fields, replacing each read site with the fixed value.

The fixed values are exactly today's defaults and must not be restated as configuration:

- subject-length limit: `72`
- plan diff threshold: `400`
- accepted commit types: `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`,
  `revert`, `style`, `test`
- dependency-manifest globs: the nineteen-member set currently returned by
  `defaultDependencyManifests`

Keep `defaultAllowedTypes` and `defaultDependencyManifests` as the single home for those two sets;
they stop being defaults and become the values. In `internal/audit/audit.go`, delete every
`if s.DomainDocStaleness`-shaped guard so each rule evaluates unconditionally, and leave both
`severity.Error` sites (`uncommitted-changes` and the subject-length finding) exactly as they are.

`Resolve` keeps a parameter only if the scope list still needs one; if `AllowedScopes` is the sole
survivor, take `[]config.ScopeEntry` rather than the whole `*config.AuditConfig`.

### Task 1.2: Make the gates, payloads and wrapper unconditional
Latitude: exact
Applying: ["unconditional-gates-and-audit-rules:gates-always-run", "unconditional-gates-and-audit-rules:payload-and-wrapper-always-render", "unconditional-gates-and-audit-rules:conditional-units-narrow-to-bootstrap"]

Delete the enablement predicates in front of the prose gate and the memory-citation gate so both
always scan, and delete the disabled-child disclosure from the repository aggregate in
`cmd/awf/checkrepo.go`. Both gates keep reading their exemption lists.

In `internal/project/singleton.go`, keep `conditionalUnits()` as the single declaration home for
every config-tree output. Do not move the hook payloads or the runner wrapper to a second table:
change their entries to declare unconditional enablement and leave the bootstrap pair as the only
conditional member. `runnerSections` stays the descriptor's non-nil section supplier, so the
`sections` facet keeps a live member and the single-source claim stays falsifiable.

In `internal/project/validate.go`, `hooks-commands-resolvable` loses its second arm: the runner
always renders, so the payloads can always use the `./awf` form and `checkCmd`/`commitGateCmd` are
no longer required. The first arm, requiring `vars.gateCmd`, becomes unconditional and keeps
binding at sync and check. Do not move it to init.

In `internal/project/scaffold.go`, remove the `Hooks` and `Runner` blocks from `ScaffoldConfig` and
leave `Bootstrap: &config.BootstrapConfig{Enabled: true}` exactly as it is.

Update every production reader of the four enablement fields in the same commit; a missed one is a
compile error, not a latent bug. The closed set is
`grep -rn 'Cfg\.Runner\|Cfg\.Hooks\|\.ProseGate\|\.MemoryCite\|MaxTopicsPerPath' --include='*.go' . | grep -v _test`,
which at plan time resolves to `cmd/awf/prosegate.go`, `cmd/awf/memorygate.go`, `cmd/awf/commitgate.go`,
`cmd/awf/list_add.go`, `internal/project/list_presentation.go`, `internal/project/render.go` (the
`runnerEnabled` template datum), `internal/project/sweep.go`, `internal/project/configreference.go`
(its live-state accessor rows, which are a separate table from the configspec), and
`internal/project/currentstate.go`.

Remove the `hooks` and `runner` listing categories here rather than in Phase 3, and with them the
`hooks` and `runner` arms of `awf enable` and `awf disable` in `cmd/awf/list_add.go` and their
declarations in `internal/clispec/clispec.go`: the kind enumerations in both commands' summary,
usage and details, the `awf list` kind enumeration, and the help sentence describing a payload
running "when the hooks artifact is enabled". A key's removal, its listing category and its command
arm all read the same field, so they land in one commit. Phase 3 keeps the catalog, domain, target
and bootstrap arms plus the enablement-state vocabulary change.

### Task 1.3: Remove the gate and audit config surface
Latitude: exact
Applying: ["unconditional-gates-and-audit-rules:toggle-key-migration", "unconditional-gates-and-audit-rules:fan-out-budget-fixed"]

In `internal/config/config.go`, delete the `Hooks` and `Runner` fields and their
`HooksConfig`/`RunnerConfig` types, delete `Enabled` from `ProseGateConfig` and `MemoryCiteConfig`
(both keep their exemption lists), delete `MaxTopicsPerPath` from `CurrentStateConfig`, and delete
every `AuditConfig` field except `AllowedScopes`. Leave `Bootstrap` and `BootstrapConfig` untouched.

Fix the path-scoped topic fan-out budget at `8` at its use site in `internal/topic/coverage.go`.

In `internal/configspec/spec.go`, delete the descriptor entries for every key removed above,
including the `currentState.maxTopicsPerPath` entry. The config-reference regeneration in Task 1.7
proves the table matches.

Keep `SetMappingScalar` and `SetMappingInteger` in `internal/config/edit.go`. Both stay reachable
through frozen migration steps that write the historical shape: `enablerunner.go` and
`enablebootstrap.go` call `SetMappingScalar`, `layercataloglists.go` calls it for `dataDefaults`
keys unrelated to any enablement boolean, and `maxclaimspertopic.go` and `dropseveritysettings.go`
call `SetMappingInteger`. This is the same reason Task 3.5 gives for `SetArrayMember`: a frozen step
is what keeps its editor alive. `config-serialization-owned` therefore keeps enumerating both, and
the house-standard record's update to that claim in Phase 3 does not drop them either.

### Task 1.4: Add schema generation 38 and its forward-port entries
Latitude: exact
Applying: ["unconditional-gates-and-audit-rules:toggle-key-migration", "unconditional-gates-and-audit-rules:toggle-keys-forward-ported"]

Add one migration step to `internal/migrate` registered as generation 38, following the shape of
`internal/migrate/dropseveritysettings.go`: edit raw YAML through `editConfig` and
`RemoveKey`/`RemoveMappingKey`, never the typed struct, announce each removal actually performed,
and leave every surviving key, value, comment and key order byte-intact. It removes the top-level
`hooks` and `runner` keys; `enabled` under `proseGate` and under `memoryCite`; `allowedTypes`,
`subjectMaxLength`, `diffThreshold`, `dependencyManifests`, `domainDocStaleness`,
`domainCodeStaleness`, `undocumentedDomain`, `plainPunctuation` and `uncommittedChanges` under
`audit`; and `maxTopicsPerPath` under `currentState`. A block emptied by removal is dropped;
`audit`, `proseGate`, `memoryCite` and `currentState` all retain their surviving members here, so
none of the four is dropped in this repository.

Do not edit any migration step registered at a generation below 38. Generation 25's fan-out seed and
the anchored-glob step both keep operating on their own generation's tree shape.

Add the same fourteen keys to `retiredKeyRemovals` in `internal/migrate/migrate.go` as
`{parent, key}` pairs, so the port-forward strips them unconditionally before the strict decoder
sees historical bytes. The top-level pairs use an empty parent.

### Task 1.5: Apply the gates record's claim operations
Kind: batch
Latitude: exact
Applying: ["unconditional-gates-and-audit-rules:gates-always-run", "unconditional-gates-and-audit-rules:audit-advisories-always-run", "unconditional-gates-and-audit-rules:audit-thresholds-fixed"]
Paths: [".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/topics/parts/tooling/init-and-enablement/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/invariants/topics-and-markers/current-state.md", ".awf/topics/parts/rendering/companion-scripts/current-state.md", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/config/validation/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/adr-system/plan-artifacts/current-state.md", "internal/migrate/enablerunner_test.go", "internal/project/runner_test.go", "internal/project/scaffold_test.go", "cmd/awf/checkgroup_test.go"]
Representative: For the update to `tooling/quality-gates:memory-citation-gate`, delete the opening condition so the claim reads as an unconditional scan and keep every other clause byte-identical, then append `, ADR-unconditional-gates-and-audit-rules` to its existing `Revised-by:` line, creating that line if absent.
Edge: For the add of `rendering/companion-scripts:runner-wrapper-rendered`, author a new claim asserting that a full render emits exactly one wrapper file at the repo-root path `awf`, mirroring the shape of `rendering/singletons-and-payloads:hook-payloads-rendered`, with `Origin: ADR-unconditional-gates-and-audit-rules` and `Backing: test`, and land its proof marker in `internal/project/runner_test.go` in this same commit. It carries the render-existence half of the removed `runner-singleton-toggle`; the toggle half is not carried forward.
Post-check: `./x render && ./x check` reports zero errors, and `grep -rn "proseGate.enabled\|memoryCite.enabled\|domainDocStaleness\|domainCodeStaleness\|undocumentedDomain\|plainPunctuation\|uncommittedChanges\|allowedTypes\|subjectMaxLength" .awf/topics/parts/` returns no output.

Apply every operation in the gates record's State changes section, and assert the Applied event's
operation list equals that section with nothing Remaining. Do not reconcile against a count; the
section is the authority. Author each added claim's prose with `Origin: ADR-unconditional-gates-and-audit-rules`
and, for an invariant, `Backing: test` plus its proof marker in the same commit. Preserve `Origin:`
and the existing `Revised-by:` prefix on every update.

Three audit claims take no operation and must be left byte-identical: `audit-domain-doc-staleness`,
`audit-undocumented-domain` and `audit-dependency-warn`.

Two key names survive in claim prose by design and are the sanctioned residual the post-check
excludes: `maxTopicsPerPath` in `config/migrations-and-locks:severity-keys-dropped` and
`dependencyManifests` in `config/validation:glob-migration-anchored`. Both describe what a frozen
generation did to its own tree shape, both stay true verbatim under the record's freeze item, and
neither is operated on.

Every removed claim's proof marker must go with it, or the marker check fails on an unknown claim
ID. The markers for this batch's removals live in `internal/migrate/enablerunner_test.go`,
`internal/project/runner_test.go`, `internal/project/scaffold_test.go` and
`cmd/awf/checkgroup_test.go`; confirm the closed set with `grep -rn --include='*.go' '<claim-id>' .`
per removed claim before editing.

When rewriting `tooling/audit-and-snapshots:audit-plain-punctuation`, also replace its two `docsDir`
references with the documentation root, which the house-standard record fixes; the key is gone by
the end of Phase 3 and this claim is not operated on again.

### Task 1.6: Test the values this record fixes
Latitude: exact
Applying: ["unconditional-gates-and-audit-rules:audit-thresholds-fixed", "unconditional-gates-and-audit-rules:fan-out-budget-fixed", "unconditional-gates-and-audit-rules:gates-always-run", "unconditional-gates-and-audit-rules:audit-advisories-always-run"]

Every claim this phase adds needs a test that would fail if the pinned value changed silently, and
the proof marker lands with the test. Without them the phase lands the claim and not the proof, and
the record's stated reason for pinning is unmet.

In `internal/audit/settings_test.go`, assert the four fixed values: subject limit `72`, diff
threshold `400`, the eleven-member type set, and the nineteen-member manifest glob set, each as an
exact expected value rather than a length. In `internal/topic/coverage_test.go`, assert the
path-scoped fan-out budget is `8`. In `internal/project/runner_test.go`, assert a full render emits
exactly one wrapper at the repo-root path `awf`. In `internal/migrate`, assert generation 38 removes
each key it declares and leaves surviving keys, comments and key order byte-intact, and assert a
config carrying every retired key still parses through `ConfigForCurrentSchema`.

Also assert both gates scan with no configuration present and that the repository aggregate emits no
disabled-child note, which is what makes `gates-always-run` falsifiable. In `internal/audit`, assert
that all five advisory rules evaluate on a run with no audit settings present, and that
uncommitted-changes still emits at error severity; without that, `audit-advisories-always-run` lands
in Task 1.5 with nothing behind it.

### Task 1.7: Set the gates record to Implementing and regenerate
Latitude: exact
Applying: ["unconditional-gates-and-audit-rules:toggle-key-migration"]

Append two events to the gates record's Status history, in order: an `Implementing` status event
carrying the record's current content digest, then one `Applied` event naming every operation in its
State changes. Do not append `Implemented`; the terminal status-only transaction belongs to effort
finalisation.

Remove `hooks`, `runner`, `proseGate.enabled`, `memoryCite.enabled`, the nine audit tuning keys and
`currentState.maxTopicsPerPath` from this repository's own `.awf/config.yaml`, keeping
`audit.allowedScopes`, both exemption lists, `currentState.sources`, `currentState.testGlobs` and
`bootstrap.enabled: false`.

Run `./x render` to regenerate `docs/config-reference.md` and `docs/decisions/INDEX.md`.

Perform a focused reading of the regenerated `docs/config-reference.md` and of every rewritten claim
for contradictory fragments and concept-preserving paraphrase: confirm the reference no longer lists
a retired key, that its `bootstrap.enabled` row still describes a live toggle, and that no rewritten
claim asserts both that a rule always runs and that a knob controls it.

### Task 1.8: Carry the unconditional-gate prose with the change
Latitude: exact
Paths: [".awf/topics/parts/rendering/companion-scripts/current-state.md", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", "README.md"]
Applying: ["unconditional-gates-and-audit-rules:conditional-units-narrow-to-bootstrap"]

In `.awf/topics/parts/rendering/companion-scripts/`, rewrite the narrative opening "When hooks are
enabled, awf renders five inert payloads", which is false once the payloads are unconditional. In
`.awf/topics/parts/rendering/singletons-and-payloads/`, keep the always-on and toggleable partition,
which stays accurate because bootstrap survives as the toggleable one, and move the hook payloads to
the always-on side.

Update `README.md` in this transaction wherever it describes the runner, hooks, prose gate,
memory-citation gate or audit advisories as optional or configurable. Perform a focused read of the
regenerated prose for any adjacent sentence that still presents one of those outputs or checks as
disabled.

### Phase close

Stage the complete transaction and create its one closing commit after `awf check staged` and
`./x gate` pass.

```commit
feat(config): make gates, payloads and audit rules unconditional
```

## Phase 2: Full-catalog render

**Execution mode: inline.**

Advances: ["records-applied"]
Completes: ["full-catalog-render"]

### Task 2.1: Render the whole catalog regardless of configuration
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render", "house-standard-configuration-expresses-repo-facts-only:fixed-targets-and-docs-root"]

In `internal/project/output_plan.go`, build the skill, agent and doc render sets from the catalog
rather than from `cfg.Skills`, `cfg.Agents` and `cfg.Docs`. Remove the `requiresDoc` suppression so
`roadmap-graduation` renders, and stop partitioning docs by the catalog `Mandatory` flag for
membership. The flag keeps its two surviving roles: a singleton is a catalog document that declares
its own output `Path`, and the sidecar location stays `.awf/<name>.yaml` for a singleton and
`.awf/docs/<name>.yaml` otherwise. Do not collapse those two.

Render both targets unconditionally. Leave `cfg.Targets` and `cfg.DocsDir` parsed and validated;
this task stops reading `cfg.Targets`, Task 2.2 stops reading `cfg.DocsDir`, and Phase 3 deletes
both.

Local artifacts still synthesize in this phase. `internal/project/local.go` keeps working, and a
`local: true` sidecar still suppresses its catalog entry, so the intermediate stays coherent for a
tree that uses the flag. This repository uses it nowhere.

### Task 2.2: Fix the documentation root
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:fixed-targets-and-docs-root"]

Introduce one exported constant naming the fixed documentation root, `docs`, and place it in
`internal/config` beside the other structural path constants so a single home owns it; every package
below reads that constant rather than restating the literal.

Rebind every production reader of the config field to it, and assert a terminal state rather than a
file list: `grep -rn 'Cfg\.DocsDir\|cfg\.DocsDir' --include='*.go' internal cmd | grep -v _test`
returns no output outside `internal/migrate` when the task is done. Enumerate the current readers
with that grep before starting rather than trusting a list written at plan time.

Two distinctions matter while working through it. A reader of `Layout.DocsDir` needs no change once
`Layout` derives from the constant; only direct config-field readers rebind. And
`internal/project/configreference.go`'s live-state datum for the `docsDir` key is not rebound at
all: it is deleted in Phase 3 Task 3.2 with the descriptor it reports on.

`internal/migrate` is excluded and handled in Phase 3 Task 3.1: its readers are frozen steps that
must keep seeing the historical shape.

`internal/project/layout.go` also carries the docs map that `layout-docs-full-catalog` replaces, so
its change lands here with the render-set change rather than in a later phase.

### Task 2.3: Update the render-set tests to the full catalog
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render", "house-standard-configuration-expresses-repo-facts-only:fixed-targets-and-docs-root"]

Update the tests that assert a selected subset so they assert the whole catalog: the fixture in
`internal/evals` already enables every catalog skill and agent, so its expectations should now be
derived from the catalog rather than from an enable list. Assert a method, not a count: prove the
rendered skill set equals `catalog` membership rather than pinning a number.

Assert the documentation root resolves to the constant `docs` with no configuration present, and
that the layout docs map equals catalog membership; those two back the `docs-root-fixed` and
`layout-docs-full-catalog` claims this phase adds, which would otherwise land unproven.

Add a test proving `roadmap-graduation` renders with no doc gate, and one proving the `debugging`
doc renders. Each lacks a convention-parts directory, so each raises a non-failing stub advisory;
assert the advisory is non-failing rather than absent.

### Task 2.4: Apply the house-standard record's render-set operations
Kind: batch
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render", "house-standard-configuration-expresses-repo-facts-only:fixed-targets-and-docs-root"]
Paths: [".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/rendering/catalog-and-targets/current-state.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", ".awf/topics/parts/rendering/adapter-outputs/current-state.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/topics/parts/tooling/evaluations/current-state.md", "internal/project/scaffold.go", "internal/project/layout.go"]
Representative: For the update to `rendering/catalog-and-targets:target-dialect-render`, replace "Each enabled target renders every selected catalog skill and agent exactly once" with the unconditional form naming every target and every catalog skill and agent, keep the rest byte-identical, and append this record to `Revised-by:`.
Edge: For the add of `rendering/project-output-plan:full-catalog-render`, author the claim in `project-output-plan` rather than `catalog-and-targets`, because that topic's selectors (`internal/project/**`) cover where its proof marker lives while `catalog-and-targets` selectors do not. It replaces `catalog-trim-applied`, `scaffold-core-only`, `skills-context-effective-set`, `enabled-set-closed` and `mandatory-doc-pool-exclusion`, all removed in this batch.
Post-check: `./x render && ./x check` reports zero errors, and `grep -rn "enabled set\|doc gate\|requiresDoc\|toggleable doc pool" .awf/topics/parts/rendering/` returns no output.

This batch owns every operation in the house-standard record's State changes **except** the
following, which Phase 3 owns. The exclusion list is the authority; any operation not on it belongs
here, which makes the two batches an exhaustive partition by construction.

Excluded, owned by Phase 3: the adds `config/configuration:config-expresses-repo-facts-only`,
`config/configuration:no-artifact-selection-surface`,
`config/configuration:root-sidecar-keys-rejected`,
`config/migrations-and-locks:selection-keys-dropped`,
`config/migrations-and-locks:sidecar-local-field-dropped` and
`config/migrations-and-locks:retired-keys-forward-ported`; the removes
`config/configuration:enable-arrays`, `config/configuration:docsdir-default`,
`config/configuration:targets-default-claude`, `config/validation:duplicate-target-rejected`,
`config/validation:local-doc-name-path-validated`, `config/validation:local-name-validated`, all ten
`rendering/local-artifacts:*`, `rendering/sync-and-drift:skills-set-in-confighash` and
`tooling/init-and-enablement:init-set-closed`; and the updates
`config/configuration:config-serialization-owned`,
`config/configuration:sidecar-data-defaults-control`,
`config/configspec-and-reference:config-reference-data-rejected`,
`config/configspec-and-reference:config-reference-regen-drift`,
`config/migrations-and-locks:upgrade-migrates-retirements`,
`config/migrations-and-locks:upgrade-migrates-supersession-keys`,
`rendering/project-output-plan:inert-sidecar-field-rejected` and
`tooling/init-and-enablement:init-prompts-enabled-vars`.

Two exclusions are worth their reason. `skills-set-in-confighash` stays true through Phase 2 because
`internal/project/confighash.go` still folds the skills set until the field is gone. `init-set-closed`
states a fact about the curated `core` set that Phase 3 deletes, so removing it in Phase 2 would
retire a claim whose subject still exists.

Author each added claim with slug-form `Origin: ADR-house-standard-configuration-expresses-repo-facts-only`
and, for an invariant, `Backing: test` plus its proof marker in this same commit. Preserve `Origin:`
and the existing `Revised-by:` prefix on every update. Every removed claim's proof marker goes with
it, or the marker check fails on an unknown claim ID; confirm the closed set with
`grep -rn --include='*.go' '<claim-id>' .` per removed claim. Two of this batch's removals carry
markers in production files rather than tests: `scaffold-core-only` at `internal/project/scaffold.go`
and `curated-init-skill-refs-clean`.

### Task 2.5: Set the house-standard record to Implementing
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render"]

Append two events to the house-standard record's Status history, in order: an `Implementing` status
event carrying its current content digest, then one `Applied` event naming exactly the operations
Task 2.4 applied. The remaining operations stay visible as pending progress for Phase 3.

Run `./x render` to regenerate the decision index and every doc whose content the render change
moves.

Perform a focused reading of the regenerated `AGENTS.md` and `docs/` outputs: with the full catalog
rendering, the document map and the skill listing gain entries. Confirm the added entries read as
intended prose rather than stubs presented as finished guidance, and that the new `debugging` doc
and `roadmap-graduation` skill outputs carry their stub advisory rather than silently empty bodies.

### Task 2.6: Carry full-catalog wording with the render change
Kind: batch
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render"]
Paths: ["glob:.awf/skills/parts/**/*.md", "glob:.awf/docs/parts/**/*.md", "glob:.awf/parts/**/*.md", "glob:.awf/domains/parts/**/*.md", "glob:templates/**/*.tmpl"]
Representative: `.awf/parts/workflow/chain.md` says "Use any enabled native skill when its purpose fits"; replace "any enabled native skill" with "any native skill", leaving the rest byte-identical. `templates/agents-doc/AGENTS.md.tmpl` carries the same construction and takes the same edit.
Edge: Where a template says "the selectable cross-target lifecycle owner", selectability is gone rather than merely unqualified: rewrite it as "the cross-target lifecycle owner". Do not touch claim bodies under `.awf/topics/parts/**/current-state.md`: the transition check refuses a claim edit that no ADR operation declares. Residual selection wording there remains successor-record debt.
Post-check: `grep -rn "enabled target\|enabled skill\|enabled native skill\|enabled set" .awf/skills/parts/ .awf/docs/parts/ .awf/parts/ .awf/domains/parts/ templates/` returns no output, and `./x render && ./x check` reports zero errors. `selectable` is deliberately excluded because effort resident slugs use it truthfully; inspect selection-context uses by reading.

### Phase close

Stage the complete transaction and create its one closing commit after `awf check staged` and
`./x gate` pass.

```commit
feat(rendering): render the full catalog for every target
```

## Phase 3: Selection retirement and CLI grammar

**Execution mode: inline.**

Completes: ["selection-retired", "cli-grammar", "records-applied", "docs-current"]

### Task 3.1: Give the frozen migrations a historical view
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:selection-key-migration"]

This task must land before Task 3.2, because without it Task 3.2 does not compile. Six registered
migration steps read the retired shape from the typed `config.Config`:
`internal/migrate/closeenabledset.go` (`cfg.Skills`, `cfg.Agents`, `cfg.Docs`, the
`cfg.Sidecar(kind, name)` lookup, and `cat.Skills[s].RequiresDoc`),
`internal/migrate/orientingbackfill.go` and `internal/migrate/groundingskillbackfill.go`
(`cfg.Skills` and the same sidecar lookup), and `internal/migrate/retirementtokens.go`,
`internal/migrate/supersessionkeys.go` and `internal/migrate/adrnumberprovenance.go` (`cfg.DocsDir`).

`internal/migrate/dropreplacewith.go` and `internal/migrate/treelayout.go` need no change: each
already decodes its own migration-local sidecar type carrying its own `Local` field, which is
exactly the precedent this task follows.

Record A item 6 commits those steps to keep operating on their own generation's tree shape, so they
may not become no-ops. Give `internal/migrate` its own historical view instead: a migration-local
struct carrying `Skills`, `Agents`, `Docs` and `DocsDir`, decoded from raw YAML, plus a sidecar
lookup method taking kind and name over the historical tree root. The lookup is required rather than
optional: `closeenabledset.go` and `groundingskillbackfill.go` call `cfg.Sidecar(kind, name)`, a
`config.Config` method that resolves and reads a sidecar file, so a struct field alone does not let
them compile. Do not carry `Targets`: no frozen step reads it, and a field with no consumer is
speculative capability. Rebind each of the six listed steps to the view. The live `config.Config`
keeps no field the current schema does not declare.

Extend `loadForMigration` in `internal/migrate/configedit.go` to strip every key generations 38 and
39 retire, using `config.RemoveKey` and `config.RemoveMappingKey` exactly as it already strips
`invariants` and `currentState.maxClaimsPerTopic`. Its existing comment states the reason: an
intervening migration parses the live tree at its historical shape and hard-fails strict decode on a
key the current schema no longer declares. Add a test upgrading a tree from a pre-38 generation
through to current, which is the only thing that proves this path.

### Task 3.2: Remove the selection fields from the config schema
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:repo-facts-only", "house-standard-configuration-expresses-repo-facts-only:retire-selection-keys", "house-standard-configuration-expresses-repo-facts-only:fixed-targets-and-docs-root"]

In `internal/config/config.go`, delete the `Skills`, `Agents`, `Docs`, `Targets` and `DocsDir`
fields and every validation rule that referenced them, including the duplicate-target rule and the
local skill, agent and doc name rules. Keep `Domains`, `Prefix`, `IntegrationBranch`, `Bootstrap`,
`Vars`, `Tags`, `ContextIgnore`, `CurrentState`, `Audit` and `CommitPolicy`.

Delete the sidecar `local` field and every rule that read it, keeping `data`, `dataDefaults`,
`sections` and the domain-only `paths`. The strict-decoder rule rejecting a root-level `data` or
`sections` key survives and is carried by its own claim; keep the rejection and narrow it by
dropping `local` from the rejected set.

In `internal/configspec/spec.go`, delete the descriptors for all five keys and for the sidecar
`local` field.

### Task 3.3: Delete the selection machinery
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render", "house-standard-configuration-expresses-repo-facts-only:retire-local-artifacts"]

Delete the requirement *closure*: `internal/catalog/graph.go`'s `RequiresOf`, `Closure` and `Node`,
and the `requiresDoc` suppression consumer Phase 2 already stopped calling. Do **not** delete the
`RequiresSkills`, `RequiresAgents` or `RequiresDoc` catalog declarations themselves. The
house-standard record declares `update rendering/catalog-and-targets:requires-skills-exact`, not
`remove`, and that claim is a statement about those declarations; deleting the fields would make the
surviving updated claim unstatable and its marker unbackable. `internal/migrate/closeenabledset.go`
also reads `RequiresDoc`, and Task 3.1 freezes it. Keep the `Mandatory` flag for the two roles Task
2.1 preserved. Delete `internal/project/local.go` and `internal/project/resolve.go`
together with the exported `ResolveEnable`, `ResolveDisable` and `PlanOp` surface and the
`PlanDocument` presentation in `internal/project/list_presentation.go` that only they feed. Delete
`internal/initspec`'s curated `core` selection.

Two different checks cover two different failures, and neither proves the whole deletion. The
compiler catches an unremoved *consumer* of deleted code. The dead-code gate catches a function the
deletion newly *orphans*, each of which is a further deletion to make. Neither sees the non-code
residue: configspec descriptors, claim prose, proof markers, test-only helpers and the sidecar
`Local` readers Task 3.1 rebinds. The completeness check for those is a closed grep for each deleted
symbol across `internal/` and `cmd/`, returning no output before the phase closes.

### Task 3.4: Retire the selection commands and rehome domains
Latitude: exact
Applying: ["cli-grammar-expresses-creation-and-inventory:retire-selection-commands", "cli-grammar-expresses-creation-and-inventory:domain-lifecycle-under-new", "cli-grammar-expresses-creation-and-inventory:new-scaffolds-authored-artifacts", "cli-grammar-expresses-creation-and-inventory:list-is-inventory", "cli-grammar-expresses-creation-and-inventory:no-deprecation-window-for-a-retired-key"]

In `internal/clispec/clispec.go` and `cmd/awf`, delete what remains of the `enable` and `disable`
commands: the three catalog-backed arms, the domain arm, the target arm and the bootstrap singleton
arm, together with the commands themselves. Task 1.2 already removed the hooks and runner arms with
the fields they wrote, so six of the eight kinds retire here and the command declarations go with
the last of them. Delete the target list command too. Delete `awf new skill`, `awf new agent` and
`awf new doc` with `newLocal`, `seedScaffoldVars` and `project.ScaffoldVarRefs`; keep
`config.SeedVarKey`, which its migration callers still use.

The bootstrap arm is the one retiring for a different reason: `bootstrap.enabled` survives, so this
arm writes a key the loader still accepts. It goes because a boolean setup fact is read from
configuration rather than selected, which is the CLI record's item 1, not its strict-parsing
argument.

Add `awf new domain <name>` and `awf remove domain <name>`, the latter introducing `awf remove` as a
new top-level verb carrying exactly one kind. Creation validates the name through the config
path-safety rule before writing, then writes the `domains` entry and scaffolds the domain's
convention part without clobbering an existing one. Removal deletes the entry and re-renders so the
domain's rendered output is pruned, and reports a surviving sidecar or convention part as orphaned
rather than deleting authored files.

Both domain arms must dispatch through the exported kind accessors rather than comparing a literal
kind name, so `rendering/project-output-plan:kind-dispatch-single-table` stays true; follow the
existing `project.IsFreeformDomainKind` routing.

Narrow `awf list` to inventory: delete the `enabled`, `available` and `local` states, keep `tuned`,
keep the target listing as a fixed inventory of `claude` and `pi` with no state token, and drop the
bootstrap, hooks and runner categories.

`bootstrap.enabled` keeps no CLI setter. `awf init` continues to scaffold it enabled.

### Task 3.5: Add schema generation 39 and its forward-port entries
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:selection-key-migration", "house-standard-configuration-expresses-repo-facts-only:historical-config-forward-ported"]

Add one migration step registered as generation 39, in the same raw-YAML shape as Task 1.4. It
removes the top-level `skills`, `agents`, `docs`, `targets` and `docsDir` keys, announcing each
removal actually performed and leaving every surviving key, value, comment and key order
byte-intact.

Add a second step in the same generation that strips the `local` field from every sidecar under
`.awf/`. It preflights every match before any write and refuses with an actionable repair naming the
file where a `local: true` sidecar means an artifact the repository owns by hand would now be
overwritten. Announce each changed file and preserve all other bytes and modes. This repository has
no such sidecar, so the refusal path is proven by test rather than by this tree.

Do not edit any step below generation 39: `closeenabledset`, `groundingskillbackfill`,
`orientingbackfill` and `singletonstandarddocs` all keep mutating enable arrays on their own
generation's tree shape, and their `SetArrayMember` calls are what keep that editor reachable.

Add the five removed top-level keys to `retiredKeyRemovals`.

### Task 3.6: Apply the remaining claim operations for both records
Kind: batch
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:retire-selection-keys", "house-standard-configuration-expresses-repo-facts-only:retire-local-artifacts", "cli-grammar-expresses-creation-and-inventory:cli-creation-and-inventory"]
Paths: [".awf/topics/parts/config/configuration/current-state.md", ".awf/topics/parts/config/validation/current-state.md", ".awf/topics/parts/config/configspec-and-reference/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/rendering/local-artifacts/current-state.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/tooling/init-and-enablement/current-state.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/topics/metadata/rendering/local-artifacts.yaml", "internal/project/confighash.go", "internal/project/drift_test.go", "internal/project/scaffold.go", "internal/config/config.go", "internal/configspec/spec.go"]
Representative: For the remove of `config/configuration:enable-arrays`, delete its entire claim block including `Origin:` and `Backing:` lines and the blank line separating it from the next claim, leaving no residual heading.
Edge: The `rendering/local-artifacts` topic loses all ten of its claims, so retire the topic itself: delete `.awf/topics/metadata/rendering/local-artifacts.yaml` and the `.awf/topics/parts/rendering/local-artifacts/` directory. Confirm the paths it selected remain covered by a surviving rendering topic before deleting, and record the covering topic in Notes.
Post-check: `./x render && ./x check` reports zero errors, `grep -rn "local: true\|enable array\|docsDir\|targets array" .awf/topics/parts/` returns no output, and `ls docs/topics/rendering/local-artifacts.md` reports no such file.

Author each added claim with slug-form `Origin:` naming its record and, for an invariant,
`Backing: test` plus its proof marker in this same commit; every removed claim's marker goes with
it, confirmed by `grep -rn --include='*.go' '<claim-id>' .` per removed claim. One exception:
`config/configuration:config-expresses-repo-facts-only` is authored as a `rule:` carrying a
`Verify:` note and no proof marker, because the house-standard record's Consequences states it has
no mechanically checkable content.

Apply every operation both records have not yet applied. For the house-standard record that is
everything outside Task 2.4's batch; for the CLI record it is its whole set, including the
`tooling/init-and-enablement:init-bootstrap-default-on` add that pins init's scaffolded default now
that no command writes the key.

### Task 3.7: Test the retirement and the migration
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:retire-selection-keys", "house-standard-configuration-expresses-repo-facts-only:selection-key-migration", "house-standard-configuration-expresses-repo-facts-only:historical-config-forward-ported", "cli-grammar-expresses-creation-and-inventory:domain-lifecycle-under-new"]

Land the proof for every claim this phase adds, alongside its marker.

Assert that a tree carrying `skills`, `agents`, `docs`, `targets`, `docsDir` or a sidecar `local` is
refused by strict parsing rather than honoured, and that a root-level `data` or `sections` key is
still refused while `local` is no longer part of that rejected set. Assert generation 39 removes
each key it declares, that the sidecar step refuses with an actionable repair when a `local: true`
sidecar would otherwise be overwritten, and that a config carrying every retired key still parses
through `ConfigForCurrentSchema`. Assert the pre-38-to-current upgrade path from Task 3.1 still
passes with the selection keys present.

For the CLI, assert over `internal/clispec` that no command named `enable`, `disable` or `target` is
declared and that `awf new` exposes exactly `adr`, `plan`, `topic` and `domain`, which is what makes
`cli-creation-and-inventory` falsifiable rather than an `awf help` observation. Assert
`awf new domain` refuses a name the loader would reject before writing anything,
that it does not clobber an existing convention part, that `awf remove domain` prunes the rendered
output and reports a surviving part as orphaned rather than deleting it, and that `awf init`
scaffolds `bootstrap.enabled` true, which is what backs `init-bootstrap-default-on`.

### Task 3.8: Apply the final batches and update this repository's config
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:selection-key-migration", "cli-grammar-expresses-creation-and-inventory:cli-creation-and-inventory"]

Append to the house-standard record's Status history one `Applied` event naming exactly the
operations Task 3.6 applied for it. Append to the CLI record an `Implementing` status event carrying
its content digest followed by one `Applied` event naming its full operation set. Neither record
gets `Implemented` here.

Remove `skills`, `agents`, `docs` and `targets` from this repository's `.awf/config.yaml`. `docsDir`
is already absent here, running at its `docs` default, so there is nothing to remove; the generation
39 step still removes it from an adopter tree that sets it. Leave `prefix`, `integrationBranch`, `bootstrap`, `vars`, `domains`, `tags`, `audit.allowedScopes`,
`contextIgnore`, `currentState.sources`, `currentState.testGlobs`, `commitPolicy` and both exemption
lists.

Run `./x render` and confirm the rendered tree is unchanged apart from the config reference, since
Phase 2 already made the render set catalog-derived. A rendered-output diff beyond
`docs/config-reference.md` means a consumer was still reading a removed field; investigate rather
than re-render over it.

### Task 3.9: Carry selection and CLI documentation with the retirement
Latitude: exact
Paths: [".awf/topics/metadata/tooling/init-and-enablement.yaml", ".awf/domains/parts/rendering/current-state.md", ".awf/domains/parts/config/current-state.md", ".awf/parts/working-with-awf/commands.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/parts/working-with-awf/overview.md", ".awf/agents-doc.yaml", ".awf/docs/glossary.yaml", "templates/docs/working-with-awf.md.tmpl", "templates/docs/workflow.md.tmpl", "templates/skills/orienting/SKILL.md.tmpl", "templates/agents-doc/AGENTS.md.tmpl", "templates/docs/doc-standard.md.tmpl", "README.md"]
Applying: ["house-standard-configuration-expresses-repo-facts-only:retire-local-artifacts", "cli-grammar-expresses-creation-and-inventory:cli-creation-and-inventory"]

Rewrite the `tooling/init-and-enablement` topic title and description, which name `add` and
`remove`, so the topic describes init alone. Rewrite the rendering and config domain index prose
that describes retired subjects.

Update the authored sources behind the generated adopter docs, not their rendered outputs. Drop
`awf enable`, `awf disable` and the target commands; document `awf new domain` and
`awf remove domain`; remove `docsDir` as a live key; and remove artifact-selection and local-artifact
guidance. Update the agent guide's Commands section through `.awf/agents-doc.yaml` if needed. Rewrite
the three `.awf/docs/glossary.yaml` rows for the doc-gated-skill state and the two retired resolver
plan operations. Update `README.md` wherever it still documents enable/disable, selectable
artifacts, target mutation, the old domain command or other retired configuration and command
surface. Do not add a pitfalls entry here; retrospective owns that decision.

Perform a focused reading of every regenerated adopter-facing doc and `README.md` for contradictory
fragments. A sentence describing how to enable a skill beside one saying the full catalog always
renders is the failure mode to find.

### Phase close

Stage the complete transaction and create its one closing commit after `awf check staged` and
`./x gate` pass.

```commit
feat(config): retire artifact selection and its command surface
```

## Definition of done

- `dod: gates-unconditional` `.awf/config.yaml` carries no `hooks`, `runner`, `proseGate.enabled`,
  `memoryCite.enabled`, audit tuning key or `currentState.maxTopicsPerPath`, and `awf check repo`
  scans prose and memory citations with no enablement knob and no disabled-child note.
- `dod: full-catalog-render` A full render emits every catalog skill, agent and doc for both
  targets, including `roadmap-graduation` and the `debugging` doc, with no config-derived selection.
- `dod: selection-retired` `.awf/config.yaml` carries no `skills`, `agents`, `docs`, `targets` or
  `docsDir`, no sidecar carries `local`, and a tree carrying any of them is refused by strict
  parsing while `awf audit` over a pre-retirement range still succeeds.
- `dod: cli-grammar` `awf enable`, `awf disable` and the target commands are absent from `awf help`;
  `awf new domain` and `awf remove domain` exist; `awf list` reports no enablement state.
- `dod: docs-current` `./x check` reports zero findings, and no tracked prose describes a retired
  key, command or mechanism as live.
- `dod: records-applied` The gates, house-standard and CLI records each carry an `Implementing`
  status event and Applied events covering their full declared operation sets, with no operation
  Remaining and no `Implemented` event, which effort finalisation appends after review settles.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated
owners may report rather than edit; the parent supplies the report to phase review and reconciles it
with findings in one focused post-review settlement commit before checkpointing or later execution.
Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- The bootstrap record is Abandoned, not implemented. `bootstrap.enabled`, `.awf/bootstrap.sh`,
  `.awf/upgrade.sh` and every bootstrap claim are untouched by this plan. A task that appears to
  require removing one is a misreading.
- After Phase 3 there is no channel for an artifact awf does not ship. Convention parts reshape a
  catalog artifact; they cannot introduce one, and `awf new skill|agent|doc` is gone with the local
  channel. An executor who reaches for a project-local skill mid-implementation should land it in
  the catalog instead, or stop and raise it.
- Owed to a successor record: the residual "enabled target" and "selectable" wording inside claim
  bodies under `.awf/topics/parts/**/current-state.md`. Those claims stay substantively true, but
  the transition check refuses a claim edit no ADR operation declares, so correcting the wording
  needs a record carrying `update` operations for them. Task 2.6 deliberately stops at authored
  prose.
- Phase 2 leaves an intentionally odd intermediate: the enable arrays are parsed and editable but no
  longer affect the render set. That is the price of a green checkpoint before Phase 3's demolition,
  and it lasts one phase.
