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

Four inline phases, ordered by what each one makes possible.

Phase 1 lands the gates record whole: it removes only keys whose consumers are local to the gate,
payload and audit paths, so it needs nothing from the other records. Phases 2 and 3 split the
selection retirement, which cannot be split by compilation: removing a `Config` array field forces
every consumer to change in the same commit. Phase 2 therefore changes the render path to emit the
whole catalog while the fields still exist and still parse, proving the new render behaviour under
test; Phase 3 then deletes the fields, the commands, the resolver, the local-artifact channel and
the requirement graph in one transaction. Phase 4 carries the prose the earlier phases invalidate.

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

### Task 1.3: Remove the gate and audit config surface
Latitude: exact
Applying: ["unconditional-gates-and-audit-rules:toggle-key-migration", "unconditional-gates-and-audit-rules:fan-out-budget-fixed"]

In `internal/config/config.go`, delete the `Hooks` and `Runner` fields and their
`HooksConfig`/`RunnerConfig` types, delete `Enabled` from `ProseGateConfig` and `MemoryCiteConfig`
(both keep their exemption lists), delete `MaxTopicsPerPath` from `CurrentStateConfig`, and delete
every `AuditConfig` field except `AllowedScopes`. Leave `Bootstrap` and `BootstrapConfig` untouched.

Fix the path-scoped topic fan-out budget at `8` at its use site in `internal/topic/coverage.go`.

In `internal/configspec/spec.go`, delete the descriptor entries for every key removed above,
including the `currentState.maxTopicsPerPath` entry. The config-reference regeneration in Task 1.6
proves the table matches.

Delete the now-callerless editors in `internal/config/edit.go`: `SetMappingScalar` lost the
`*.enabled` booleans and `SetMappingInteger` lost `subjectMaxLength`, `diffThreshold` and
`maxTopicsPerPath`. Verify with `grep -rn "SetMappingScalar\|SetMappingInteger" --include="*.go" .`
before deleting; if either retains a caller, keep it and record the caller in Notes.

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
Paths: [".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/invariants/topics-and-markers/current-state.md", ".awf/topics/parts/rendering/companion-scripts/current-state.md", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/config/validation/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/adr-system/plan-artifacts/current-state.md"]
Representative: For the update to `tooling/quality-gates:memory-citation-gate`, delete the opening condition so the claim reads as an unconditional scan and keep every other clause byte-identical, then append `, ADR-<this record's assigned number>` to its existing `Revised-by:` line, creating that line if absent.
Edge: For the add of `rendering/companion-scripts:runner-wrapper-rendered`, author a new claim asserting that a full render emits exactly one wrapper file at the repo-root path `awf`, mirroring the shape of `rendering/singletons-and-payloads:hook-payloads-rendered`, with `Origin: ADR-<this record's assigned number>` and `Backing: test`. It carries the render-existence half of the removed `runner-singleton-toggle`; the toggle half is not carried forward.
Post-check: `./x render && ./x check` reports zero errors, and `grep -rn "proseGate.enabled\|memoryCite.enabled\|maxTopicsPerPath\|domainDocStaleness\|domainCodeStaleness\|undocumentedDomain\|plainPunctuation\|uncommittedChanges\|allowedTypes\|subjectMaxLength\|diffThreshold\|dependencyManifests" .awf/topics/parts/` returns no output.

Apply exactly the record's declared operations: seven adds, four removes and sixteen updates. Author
each added claim's prose with `Origin:` naming this record and, for an invariant, `Backing: test`
plus its proof marker. Preserve `Origin:` and the existing `Revised-by:` prefix on every update.
Three audit claims take no operation and must be left byte-identical: `audit-domain-doc-staleness`,
`audit-undocumented-domain` and `audit-dependency-warn`.

When rewriting `tooling/audit-and-snapshots:audit-plain-punctuation`, also replace its two `docsDir`
references with the documentation root, which the house-standard record fixes; the key is gone by
the end of Phase 3 and this claim is not operated on again.

### Task 1.6: Set the gates record to Implementing and regenerate
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

Render both targets unconditionally and derive every documentation path from a fixed `docs/` root
rather than from `cfg.DocsDir`. Leave `cfg.Targets` and `cfg.DocsDir` parsed and validated; this
task stops reading them, and Phase 3 deletes them.

Local artifacts still synthesize in this phase. `internal/project/local.go` keeps working, and a
`local: true` sidecar still suppresses its catalog entry, so the intermediate stays coherent for a
tree that uses the flag. This repository uses it nowhere.

### Task 2.2: Update the render-set tests to the full catalog
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render"]

Update the tests that assert a selected subset so they assert the whole catalog: the fixture in
`internal/evals` already enables every catalog skill and agent, so its expectations should now be
derived from the catalog rather than from an enable list. Assert a method, not a count: prove the
rendered skill set equals `catalog` membership rather than pinning a number.

Add a test proving `roadmap-graduation` renders with no doc gate, and one proving the `debugging`
doc renders. Each lacks a convention-parts directory, so each raises a non-failing stub advisory;
assert the advisory is non-failing rather than absent.

### Task 2.3: Apply the house-standard record's render-set operations
Kind: batch
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render", "house-standard-configuration-expresses-repo-facts-only:fixed-targets-and-docs-root"]
Paths: [".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/rendering/catalog-and-targets/current-state.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", ".awf/topics/parts/rendering/singletons-and-payloads/current-state.md", ".awf/topics/parts/rendering/adapter-outputs/current-state.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/topics/parts/tooling/evaluations/current-state.md"]
Representative: For the update to `rendering/catalog-and-targets:target-dialect-render`, replace "Each enabled target renders every selected catalog skill and agent exactly once" with the unconditional form naming every target and every catalog skill and agent, keep the rest byte-identical, and append this record to `Revised-by:`.
Edge: For the add of `rendering/project-output-plan:full-catalog-render`, author the claim in `project-output-plan` rather than `catalog-and-targets`, because that topic's selectors (`internal/project/**`) cover where its proof marker lives while `catalog-and-targets` selectors do not. It replaces `catalog-trim-applied`, `scaffold-core-only`, `skills-context-effective-set`, `enabled-set-closed` and `mandatory-doc-pool-exclusion`, all removed in this batch.
Post-check: `./x render && ./x check` reports zero errors, and `grep -rn "enabled set\|doc gate\|requiresDoc\|toggleable doc pool" .awf/topics/parts/rendering/` returns no output.

Apply only the operations this batch owns: the render-set adds, removes and updates from the
house-standard record's State changes, plus `rendering/doc-outputs:docs-root-fixed` and the
`layout-docs-enabled-only` remove with its `layout-docs-full-catalog` add. Leave for Phase 3 every
operation about the config keys themselves, the local-artifact topic, the migrations topic, the
configspec topic and `config/configuration`.

### Task 2.4: Set the house-standard record to Implementing
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render"]

Append two events to the house-standard record's Status history, in order: an `Implementing` status
event carrying its current content digest, then one `Applied` event naming exactly the operations
Task 2.3 applied. The remaining operations stay visible as pending progress for Phase 3.

Run `./x render` to regenerate the decision index and every doc whose content the render change
moves.

Perform a focused reading of the regenerated `AGENTS.md` and `docs/` outputs: with the full catalog
rendering, the document map and the skill listing gain entries. Confirm the added entries read as
intended prose rather than stubs presented as finished guidance, and that the new `debugging` doc
and `roadmap-graduation` skill outputs carry their stub advisory rather than silently empty bodies.

### Phase close

Stage the complete transaction and create its one closing commit after `awf check staged` and
`./x gate` pass.

```commit
feat(rendering): render the full catalog for every target
```

## Phase 3: Selection retirement and CLI grammar

**Execution mode: inline.**

Advances: ["records-applied"]
Completes: ["selection-retired", "cli-grammar"]

### Task 3.1: Remove the selection fields from the config schema
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

### Task 3.2: Delete the selection machinery
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render", "house-standard-configuration-expresses-repo-facts-only:retire-local-artifacts"]

Delete `internal/catalog/graph.go` and the `RequiresSkills`, `RequiresAgents`, `RequiresDoc` and
`Mandatory`-as-pool-partition machinery it serves, keeping the `Mandatory` flag itself for the two
roles Task 2.1 preserved. Delete `internal/project/local.go` and `internal/project/resolve.go`
together with the exported `ResolveEnable`, `ResolveDisable` and `PlanOp` surface and the
`PlanDocument` presentation in `internal/project/list_presentation.go` that only they feed. Delete
`internal/initspec`'s curated `core` selection.

The dead-code gate is the check that this is complete: run `./x gate` and treat any
`deadcodecheck` finding as an unremoved consumer rather than a false positive.

### Task 3.3: Retire the selection commands and rehome domains
Latitude: exact
Applying: ["cli-grammar-expresses-creation-and-inventory:retire-selection-commands", "cli-grammar-expresses-creation-and-inventory:domain-lifecycle-under-new", "cli-grammar-expresses-creation-and-inventory:new-scaffolds-authored-artifacts", "cli-grammar-expresses-creation-and-inventory:list-is-inventory", "cli-grammar-expresses-creation-and-inventory:no-deprecation-window-for-a-retired-key"]

In `internal/clispec/clispec.go` and `cmd/awf`, delete the `enable` and `disable` commands across
all eight kinds, including the bootstrap, hooks and runner singleton arms, and delete the target
add, remove and list commands. Delete `awf new skill`, `awf new agent` and `awf new doc` with
`newLocal`, `seedScaffoldVars` and `project.ScaffoldVarRefs`; keep `config.SeedVarKey`, which its
migration callers still use.

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

### Task 3.4: Add schema generation 39 and its forward-port entries
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

### Task 3.5: Apply the remaining claim operations for both records
Kind: batch
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:retire-selection-keys", "house-standard-configuration-expresses-repo-facts-only:retire-local-artifacts", "cli-grammar-expresses-creation-and-inventory:cli-creation-and-inventory"]
Paths: [".awf/topics/parts/config/configuration/current-state.md", ".awf/topics/parts/config/validation/current-state.md", ".awf/topics/parts/config/configspec-and-reference/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/rendering/local-artifacts/current-state.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/rendering/doc-outputs/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/tooling/init-and-enablement/current-state.md", ".awf/topics/metadata/rendering/local-artifacts.yaml"]
Representative: For the remove of `config/configuration:enable-arrays`, delete its entire claim block including `Origin:` and `Backing:` lines and the blank line separating it from the next claim, leaving no residual heading.
Edge: The `rendering/local-artifacts` topic loses all ten of its claims, so retire the topic itself: delete `.awf/topics/metadata/rendering/local-artifacts.yaml` and the `.awf/topics/parts/rendering/local-artifacts/` directory. Confirm the paths it selected remain covered by a surviving rendering topic before deleting, and record the covering topic in Notes.
Post-check: `./x render && ./x check` reports zero errors, `grep -rn "local: true\|enable array\|docsDir\|targets array" .awf/topics/parts/` returns no output, and `ls docs/topics/rendering/local-artifacts.md` reports no such file.

Apply every operation both records have not yet applied. For the house-standard record that is
everything outside Task 2.3's batch; for the CLI record it is its whole set, including the
`tooling/init-and-enablement:init-bootstrap-default-on` add that pins init's scaffolded default now
that no command writes the key.

### Task 3.6: Apply the final batches and update this repository's config
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:selection-key-migration", "cli-grammar-expresses-creation-and-inventory:cli-creation-and-inventory"]

Append to the house-standard record's Status history one `Applied` event naming exactly the
operations Task 3.5 applied for it. Append to the CLI record an `Implementing` status event carrying
its content digest followed by one `Applied` event naming its full operation set. Neither record
gets `Implemented` here.

Remove `skills`, `agents`, `docs`, `targets` and `docsDir` from this repository's `.awf/config.yaml`,
leaving `prefix`, `integrationBranch`, `bootstrap`, `vars`, `domains`, `tags`, `audit.allowedScopes`,
`contextIgnore`, `currentState.sources`, `currentState.testGlobs`, `commitPolicy` and both exemption
lists.

Run `./x render` and confirm the rendered tree is unchanged apart from the config reference, since
Phase 2 already made the render set catalog-derived. A rendered-output diff beyond
`docs/config-reference.md` means a consumer was still reading a removed field; investigate rather
than re-render over it.

### Phase close

Stage the complete transaction and create its one closing commit after `awf check staged` and
`./x gate` pass.

```commit
feat(config): retire artifact selection and its command surface
```

## Phase 4: Documentation surfaces and residual wording

**Execution mode: inline.**

Completes: ["docs-current", "records-applied"]

### Task 4.1: Rewrite the topic narratives the retirements invalidate
Latitude: exact
Applying: ["unconditional-gates-and-audit-rules:conditional-units-narrow-to-bootstrap", "house-standard-configuration-expresses-repo-facts-only:retire-local-artifacts"]

In `.awf/topics/parts/rendering/companion-scripts/`, rewrite the narrative opening "When hooks are
enabled, awf renders five inert payloads", which is false once the payloads are unconditional. In
`.awf/topics/parts/rendering/singletons-and-payloads/`, keep the always-on and toggleable partition,
which stays accurate because bootstrap survives as the toggleable one, and move the hook payloads to
the always-on side.

Rewrite the `tooling/init-and-enablement` topic title and description, which name `add` and
`remove`: the topic is reduced to init alone. Rewrite the `rendering` and `config` domain index
prose that describes retired subjects.

### Task 4.2: Sweep the residual enablement wording
Kind: batch
Latitude: exact
Applying: ["house-standard-configuration-expresses-repo-facts-only:full-catalog-render"]
Paths: ["glob:.awf/topics/parts/**/current-state.md", "glob:.awf/skills/parts/**/*.md", "glob:.awf/docs/parts/**/*.md", "glob:.awf/parts/**/*.md"]
Representative: In `rendering/render-engine:sidecar-optional`, replace "an enabled target" with "a target", leaving the rest of the claim byte-identical. The claim stays true either way; this removes a condition that no longer exists.
Edge: In `rendering/workflow-skill-templates:effort-workflow`, "the selectable cross-target lifecycle owner" loses its selectability, not just a qualifier: rewrite as "the cross-target lifecycle owner" rather than substituting a synonym for "selectable".
Post-check: `grep -rn "enabled target\|enabled skill\|selectable\|enabled set" .awf/topics/parts/ .awf/skills/parts/ .awf/docs/parts/ .awf/parts/` returns no output, and `./x render && ./x check` reports zero errors.

These claims stay substantively true because every target and every skill is now always present, so
this is wording, not a claim operation. Do not add State-changes operations for them; the
house-standard record records this sweep as deliberately deferred.

### Task 4.3: Update the guide and adopter documentation
Latitude: exact
Applying: ["cli-grammar-expresses-creation-and-inventory:cli-creation-and-inventory"]

Update the working-with-awf doc's command reference and override guidance to drop `awf enable`,
`awf disable` and the target commands, and to document `awf new domain` and `awf remove domain`.
Update the agent guide's Commands section through `.awf/agents-doc.yaml` if it names a retired
command. Update the glossary for any term that names a retired mechanism, and add or amend a
pitfalls entry only if the retirements create a trap worth recording.

Perform a focused reading of every regenerated adopter-facing doc for contradictory fragments: a
sentence describing how to enable a skill, next to one saying the full catalog always renders, is
the failure mode to find.

### Phase close

Stage the complete transaction and create its one closing commit after `awf check staged` and
`./x gate` pass.

```commit
docs(rendering): carry prose through the configuration collapse
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
- Phase 2 leaves an intentionally odd intermediate: the enable arrays are parsed and editable but no
  longer affect the render set. That is the price of a green checkpoint before Phase 3's demolition,
  and it lasts one phase.
