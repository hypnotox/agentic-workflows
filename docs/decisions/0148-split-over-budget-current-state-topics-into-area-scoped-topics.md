---
format: current-state-v2
status: Implementing
date: 2026-07-22
---
# ADR-0148: Split over-budget current-state topics into area-scoped topics

## Context

`currentState.maxClaimsPerTopic` is 20, and five topics exceed it (working-tree counts at
the time of this decision): `rendering/project-output-plan` 76, `rendering/templates` 50,
`tooling/cli` 49, `config/configuration` 30, and `rendering/catalog-and-targets` 23. Every
other topic holds 18 or fewer. The consequences are concrete: `awf check` prints the
non-failing topic-claim-budget advisory on every working-tree run, `awf topic <id>`
drilldowns on the offenders render an unordered wall of claims, and every
`awf context --full` packet touching an offender carries the whole topic.

Each offender mixes separable areas: project-output-plan mixes plan composition, sync and
drift mechanics, in-place editing and placeholders, local artifacts, singletons, and
generated docs; templates mixes template hygiene, Pi workflow contracts, companion scripts,
guide and doc templates, and workflow-skill templates; cli mixes orientation queries, init
and enablement, audit surfaces, and core command dispatch; configuration mixes core schema
semantics, the config-spec model, and validation rules; catalog-and-targets mixes the
catalog and target seam with Pi runtime claims.

Mechanics that constrain the split, verified against the checker:

- Claim identity is `<domain>/<topic>:<slug>`, so moving a claim changes its id. A split is
  one remove plus several adds, and a removed id is never reused.
- An added claim's `Origin:` must be the adding ADR (`internal/currentstate/check.go`
  rejects any other value), and every `Revised-by:` entry must match an applied update
  operation on that exact id. A moved claim therefore lands with `Origin:` naming this ADR
  and an empty `Revised-by:`, regardless of its prior provenance. The prior trail stays
  reachable through `awf topic <old-id> --history` on the removed id. Sixteen moved claims
  currently carry `Revised-by` entries (templates 5, cli 7, catalog-and-targets 4); their
  revision provenance survives only in history.
- Precondition at freeze time, verified: zero `References:` lines exist in the claim
  corpus, so no reference edges need repointing and none may be authored against old ids
  while this ADR is in flight.
- Non-proof markers (`state:`, `touches-state:`) must sit inside the claim's effective
  topic scope; each batch rewrites its marker sites in the same staged transaction, and
  the destination selectors already cover every moved claim because the 15 shells land
  with this proposal.

## Decision

1. Split the five over-budget topics along semantic area seams into fifteen new topics,
   each offender keeping its existing topic as the core seam. No claim prose changes: a
   moved claim keeps its slug and body verbatim; only the topic segment of its id changes.
2. The complete mapping is the State changes list below: operations appear as adjacent
   remove/add pairs per moved claim, grouped by source topic in application-batch order.
   Destination membership after the split (claim counts in parentheses):
   - `rendering/project-output-plan` keeps plan composition, scaffolding, sidecar
     semantics, and target capabilities (15); new `rendering/sync-and-drift` takes drift
     detection, config-hash inputs, attribution, backups, residue, pruning, and uninstall
     (17); new `rendering/inplace-and-placeholders` takes in-place sections, authoring
     comments, placeholders, and var/data hygiene (13); new `rendering/local-artifacts`
     takes local skill, agent, and doc declarations (10); new
     `rendering/singletons-and-payloads` takes singleton outputs, bootstrap and hook
     payload rendering, telemetry outputs, and file modes (9); new `rendering/doc-outputs`
     takes domain and topic docs, layout, pitfalls, stubs, and skill-reference hygiene (12).
   - `rendering/templates` keeps template hygiene: frontmatter, goldens, residue,
     publication safety, and fallback guards (9); new `rendering/pi-workflows` takes the Pi
     workflow contracts, including `pi-session-handoff-lifecycle` from
     catalog-and-targets (13); new `rendering/companion-scripts` takes bootstrap, upgrade,
     runner, and hook-payload script contracts (12); new
     `rendering/guide-and-doc-templates` takes guide and doc template contracts (9); new
     `rendering/workflow-skill-templates` takes workflow-chain and task-skill template
     contracts (8).
   - `rendering/catalog-and-targets` keeps the catalog and target seam (16); new
     `rendering/pi-runtime` takes the Pi runtime floor and boundaries (6).
   - `tooling/cli` keeps command dispatch, version gating, and cross-command contracts,
     including the topic-claim-budget advisory, which describes `awf check` output rather
     than an orientation surface and stays under its stable id for the planned severity
     follow-up (16); new `tooling/context-and-topic` takes the read-only orientation
     surfaces (13); new `tooling/init-and-enablement` takes init, add, remove, and new
     (13); new `tooling/audit-commands` takes audit, repoaudit, and mutants (7). The audit
     claims stay out of `tooling/audit-and-snapshots`, which already holds 14 claims.
   - `config/configuration` keeps core schema semantics (15); new
     `config/configspec-and-reference` takes the config-spec model and the generated
     reference (7); new `config/validation` takes name, glob, target, and tag validation
     rules (8).
3. Provenance convention: every moved claim lands with `Origin:` naming this ADR and an
   empty `Revised-by:`; the pre-move decision trail is reachable via
   `awf topic <old-id> --history`. This mapping (per-destination rosters above plus the
   remove/add pairs below) is the durable old-to-new record.
4. Selectors: each new topic mirrors its source topic's `paths:` selectors, which keeps
   every existing scope-bound marker site inside effective scope for any partition of the
   source's claims. The two Pi topics deviate: they claim `.pi/extensions/**` plus
   `internal/catalog/**` (and `templates/**` for pi-workflows) but not
   `internal/project/target.go` or `internal/project/target_test.go`, because proof
   markers bind through `currentState.testGlobs` rather than topic scope and no Pi claim
   has a scope-bound marker site in those files; keeping them would push their per-path
   fan-in to 9, over the warning threshold of 8. Resulting fan-in stays at or below 7.
   Narrowing selectors per area is deliberate follow-up refinement, not part of this
   decision.
5. Application: V2 incremental batches, one per source topic, smallest first:
   catalog-and-targets, configuration, cli, templates, project-output-plan. Each batch is
   one staged transaction containing exactly its claim moves, every marker-site rewrite for
   the moved claims (proof comments, `touches-state:`, `state:`), and the rendered fallout:
   each batch runs `./x sync` in the same transaction, so rendered topic and domain docs,
   and `docs/decisions/INDEX.md` on status-carrying batches, regenerate with it.
   The Implementing status event travels with the first Applied batch; the Implemented
   status event lands in the same transaction as the final Applied batch.
6. Out of scope: in-part claim grouping (a part-format change), and promoting the
   topic-claim-budget advisory to a failing severity. The promotion is a follow-up ADR
   once the corpus is clean, expected to add a configurable severity (error, warn, off)
   for adopters.

## State changes

- remove `rendering/catalog-and-targets:pi-child-process-safety`
- add `rendering/pi-runtime:pi-child-process-safety`
- remove `rendering/catalog-and-targets:pi-child-tool-boundaries`
- add `rendering/pi-runtime:pi-child-tool-boundaries`
- remove `rendering/catalog-and-targets:pi-extension-target-render`
- add `rendering/pi-runtime:pi-extension-target-render`
- remove `rendering/catalog-and-targets:pi-implementation-state-boundary`
- add `rendering/pi-runtime:pi-implementation-state-boundary`
- remove `rendering/catalog-and-targets:pi-minimum-runtime`
- add `rendering/pi-runtime:pi-minimum-runtime`
- remove `rendering/catalog-and-targets:pi-real-runtime-smoke`
- add `rendering/pi-runtime:pi-real-runtime-smoke`
- remove `rendering/catalog-and-targets:pi-session-handoff-lifecycle`
- add `rendering/pi-workflows:pi-session-handoff-lifecycle`
- remove `config/configuration:config-reference-data-rejected`
- add `config/configspec-and-reference:config-reference-data-rejected`
- remove `config/configuration:config-reference-no-bare-vars`
- add `config/configspec-and-reference:config-reference-no-bare-vars`
- remove `config/configuration:config-reference-regen-drift`
- add `config/configspec-and-reference:config-reference-regen-drift`
- remove `config/configuration:configspec-data-parity`
- add `config/configspec-and-reference:configspec-data-parity`
- remove `config/configuration:configspec-description-residue`
- add `config/configspec-and-reference:configspec-description-residue`
- remove `config/configuration:configspec-key-parity`
- add `config/configspec-and-reference:configspec-key-parity`
- remove `config/configuration:configspec-var-derivation`
- add `config/configspec-and-reference:configspec-var-derivation`
- remove `config/configuration:domain-name-validated`
- add `config/validation:domain-name-validated`
- remove `config/configuration:duplicate-target-rejected`
- add `config/validation:duplicate-target-rejected`
- remove `config/configuration:glob-migration-anchored`
- add `config/validation:glob-migration-anchored`
- remove `config/configuration:local-doc-name-path-validated`
- add `config/validation:local-doc-name-path-validated`
- remove `config/configuration:local-name-validated`
- add `config/validation:local-name-validated`
- remove `config/configuration:pathglob-anchored`
- add `config/validation:pathglob-anchored`
- remove `config/configuration:tag-not-domain-name`
- add `config/validation:tag-not-domain-name`
- remove `config/configuration:testglobs-anchored-validated`
- add `config/validation:testglobs-anchored-validated`
- remove `tooling/cli:context-adr-operation-projection`
- add `tooling/context-and-topic:context-adr-operation-projection`
- remove `tooling/cli:context-applicability-navigation`
- add `tooling/context-and-topic:context-applicability-navigation`
- remove `tooling/cli:context-default-excludes-history`
- add `tooling/context-and-topic:context-default-excludes-history`
- remove `tooling/cli:context-full-authority-packet`
- add `tooling/context-and-topic:context-full-authority-packet`
- remove `tooling/cli:context-known-artifact-navigation`
- add `tooling/context-and-topic:context-known-artifact-navigation`
- remove `tooling/cli:context-output-parity`
- add `tooling/context-and-topic:context-output-parity`
- remove `tooling/cli:context-path-attribution`
- add `tooling/context-and-topic:context-path-attribution`
- remove `tooling/cli:context-path-classification`
- add `tooling/context-and-topic:context-path-classification`
- remove `tooling/cli:context-read-only`
- add `tooling/context-and-topic:context-read-only`
- remove `tooling/cli:context-static-fallback`
- add `tooling/context-and-topic:context-static-fallback`
- remove `tooling/cli:describe-read-only`
- add `tooling/context-and-topic:describe-read-only`
- remove `tooling/cli:uncovered-collapses-directories`
- add `tooling/context-and-topic:uncovered-collapses-directories`
- remove `tooling/cli:uncovered-output-parity`
- add `tooling/context-and-topic:uncovered-output-parity`
- remove `tooling/cli:add-applies-closure-plan`
- add `tooling/init-and-enablement:add-applies-closure-plan`
- remove `tooling/cli:add-skill-pairs-agent`
- add `tooling/init-and-enablement:add-skill-pairs-agent`
- remove `tooling/cli:explicit-answers-win`
- add `tooling/init-and-enablement:explicit-answers-win`
- remove `tooling/cli:init-collision-guard`
- add `tooling/init-and-enablement:init-collision-guard`
- remove `tooling/cli:init-force-backs-up`
- add `tooling/init-and-enablement:init-force-backs-up`
- remove `tooling/cli:init-hooks-default-on`
- add `tooling/init-and-enablement:init-hooks-default-on`
- remove `tooling/cli:init-noninteractive-default`
- add `tooling/init-and-enablement:init-noninteractive-default`
- remove `tooling/cli:init-prompts-enabled-vars`
- add `tooling/init-and-enablement:init-prompts-enabled-vars`
- remove `tooling/cli:init-set-closed`
- add `tooling/init-and-enablement:init-set-closed`
- remove `tooling/cli:init-unborn-head-supported`
- add `tooling/init-and-enablement:init-unborn-head-supported`
- remove `tooling/cli:new-seeds-scaffold-vars`
- add `tooling/init-and-enablement:new-seeds-scaffold-vars`
- remove `tooling/cli:remove-agent-pairing-guard`
- add `tooling/init-and-enablement:remove-agent-pairing-guard`
- remove `tooling/cli:remove-refuses-dependents`
- add `tooling/init-and-enablement:remove-refuses-dependents`
- remove `tooling/cli:audit-empty-range-announced`
- add `tooling/audit-commands:audit-empty-range-announced`
- remove `tooling/cli:audit-reports-evaluated-scope`
- add `tooling/audit-commands:audit-reports-evaluated-scope`
- remove `tooling/cli:audit-requires-explicit-range`
- add `tooling/audit-commands:audit-requires-explicit-range`
- remove `tooling/cli:audit-scopes-descriptor-routed`
- add `tooling/audit-commands:audit-scopes-descriptor-routed`
- remove `tooling/cli:audit-warn-exit-zero`
- add `tooling/audit-commands:audit-warn-exit-zero`
- remove `tooling/cli:mutants-missing-report-errors`
- add `tooling/audit-commands:mutants-missing-report-errors`
- remove `tooling/cli:repoaudit-requires-explicit-range`
- add `tooling/audit-commands:repoaudit-requires-explicit-range`
- remove `rendering/templates:pi-dedicated-grounding-dispatch`
- add `rendering/pi-workflows:pi-dedicated-grounding-dispatch`
- remove `rendering/templates:pi-extension-editor-quiet-strip`
- add `rendering/pi-workflows:pi-extension-editor-quiet-strip`
- remove `rendering/templates:pi-implementation-batch-exclusivity`
- add `rendering/pi-workflows:pi-implementation-batch-exclusivity`
- remove `rendering/templates:pi-session-handoff-public-contract`
- add `rendering/pi-workflows:pi-session-handoff-public-contract`
- remove `rendering/templates:pi-session-handoff-workflow`
- add `rendering/pi-workflows:pi-session-handoff-workflow`
- remove `rendering/templates:pi-structured-exploration-contract`
- add `rendering/pi-workflows:pi-structured-exploration-contract`
- remove `rendering/templates:pi-subagent-failure-details`
- add `rendering/pi-workflows:pi-subagent-failure-details`
- remove `rendering/templates:pi-subagent-model-routing`
- add `rendering/pi-workflows:pi-subagent-model-routing`
- remove `rendering/templates:pi-subagent-progress-bounds`
- add `rendering/pi-workflows:pi-subagent-progress-bounds`
- remove `rendering/templates:pi-subagent-progress-context-isolation`
- add `rendering/pi-workflows:pi-subagent-progress-context-isolation`
- remove `rendering/templates:pi-subagent-progress-rendering`
- add `rendering/pi-workflows:pi-subagent-progress-rendering`
- remove `rendering/templates:pi-workflow-dashboard-public-contract`
- add `rendering/pi-workflows:pi-workflow-dashboard-public-contract`
- remove `rendering/templates:bootstrap-checksum`
- add `rendering/companion-scripts:bootstrap-checksum`
- remove `rendering/templates:bootstrap-env-override`
- add `rendering/companion-scripts:bootstrap-env-override`
- remove `rendering/templates:bootstrap-local-first`
- add `rendering/companion-scripts:bootstrap-local-first`
- remove `rendering/templates:bootstrap-stdout-path-only`
- add `rendering/companion-scripts:bootstrap-stdout-path-only`
- remove `rendering/templates:hook-payloads-fallback-safe`
- add `rendering/companion-scripts:hook-payloads-fallback-safe`
- remove `rendering/templates:runner-awf-verbs-owned`
- add `rendering/companion-scripts:runner-awf-verbs-owned`
- remove `rendering/templates:runner-example-adopted`
- add `rendering/companion-scripts:runner-example-adopted`
- remove `rendering/templates:runner-project-verbs-in-place`
- add `rendering/companion-scripts:runner-project-verbs-in-place`
- remove `rendering/templates:runner-render-publication-safe`
- add `rendering/companion-scripts:runner-render-publication-safe`
- remove `rendering/templates:runner-singleton-toggle`
- add `rendering/companion-scripts:runner-singleton-toggle`
- remove `rendering/templates:upgrade-delegates-fetch`
- add `rendering/companion-scripts:upgrade-delegates-fetch`
- remove `rendering/templates:upgrade-exec-final`
- add `rendering/companion-scripts:upgrade-exec-final`
- remove `rendering/templates:agents-doc-section-parity`
- add `rendering/guide-and-doc-templates:agents-doc-section-parity`
- remove `rendering/templates:agentsdoc-parts`
- add `rendering/guide-and-doc-templates:agentsdoc-parts`
- remove `rendering/templates:docs-section-parity`
- add `rendering/guide-and-doc-templates:docs-section-parity`
- remove `rendering/templates:document-map-lists-mandatory-docs`
- add `rendering/guide-and-doc-templates:document-map-lists-mandatory-docs`
- remove `rendering/templates:glossary-table-forced`
- add `rendering/guide-and-doc-templates:glossary-table-forced`
- remove `rendering/templates:glossary-terms-sorted`
- add `rendering/guide-and-doc-templates:glossary-terms-sorted`
- remove `rendering/templates:glossary-terms-validated`
- add `rendering/guide-and-doc-templates:glossary-terms-validated`
- remove `rendering/templates:guide-scopes-derived`
- add `rendering/guide-and-doc-templates:guide-scopes-derived`
- remove `rendering/templates:no-doc-path-vars`
- add `rendering/guide-and-doc-templates:no-doc-path-vars`
- remove `rendering/templates:bounded-exploration-reporting`
- add `rendering/workflow-skill-templates:bounded-exploration-reporting`
- remove `rendering/templates:cross-runtime-exploration-dispatch`
- add `rendering/workflow-skill-templates:cross-runtime-exploration-dispatch`
- remove `rendering/templates:memory-checkpoint-chain-coverage`
- add `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- remove `rendering/templates:plan-task-detail-modes`
- add `rendering/workflow-skill-templates:plan-task-detail-modes`
- remove `rendering/templates:reviewers-report-only`
- add `rendering/workflow-skill-templates:reviewers-report-only`
- remove `rendering/templates:skill-prose-tool-agnostic`
- add `rendering/workflow-skill-templates:skill-prose-tool-agnostic`
- remove `rendering/templates:workflow-chain-adr-before-plan`
- add `rendering/workflow-skill-templates:workflow-chain-adr-before-plan`
- remove `rendering/templates:workflow-chain-surfaces-resync`
- add `rendering/workflow-skill-templates:workflow-chain-surfaces-resync`
- remove `rendering/project-output-plan:awf-bak-flagged`
- add `rendering/sync-and-drift:awf-bak-flagged`
- remove `rendering/project-output-plan:catalog-data-in-confighash`
- add `rendering/sync-and-drift:catalog-data-in-confighash`
- remove `rendering/project-output-plan:check-active-md-stale`
- add `rendering/sync-and-drift:check-active-md-stale`
- remove `rendering/project-output-plan:check-invalid-frontmatter`
- add `rendering/sync-and-drift:check-invalid-frontmatter`
- remove `rendering/project-output-plan:closed-config-tree`
- add `rendering/sync-and-drift:closed-config-tree`
- remove `rendering/project-output-plan:drift-source-set`
- add `rendering/sync-and-drift:drift-source-set`
- remove `rendering/project-output-plan:managed-output-attribution`
- add `rendering/sync-and-drift:managed-output-attribution`
- remove `rendering/project-output-plan:part-scopes-in-confighash`
- add `rendering/sync-and-drift:part-scopes-in-confighash`
- remove `rendering/project-output-plan:provenance-banner`
- add `rendering/sync-and-drift:provenance-banner`
- remove `rendering/project-output-plan:regeneration-checked-attribute`
- add `rendering/sync-and-drift:regeneration-checked-attribute`
- remove `rendering/project-output-plan:residue-exemptions-pinned-three`
- add `rendering/sync-and-drift:residue-exemptions-pinned-three`
- remove `rendering/project-output-plan:scopes-in-confighash`
- add `rendering/sync-and-drift:scopes-in-confighash`
- remove `rendering/project-output-plan:skills-set-in-confighash`
- add `rendering/sync-and-drift:skills-set-in-confighash`
- remove `rendering/project-output-plan:sync-always-writes-active-md`
- add `rendering/sync-and-drift:sync-always-writes-active-md`
- remove `rendering/project-output-plan:sync-backs-up-foreign`
- add `rendering/sync-and-drift:sync-backs-up-foreign`
- remove `rendering/project-output-plan:target-prune-ancestors`
- add `rendering/sync-and-drift:target-prune-ancestors`
- remove `rendering/project-output-plan:uninstall-removes-lock-entries`
- add `rendering/sync-and-drift:uninstall-removes-lock-entries`
- remove `rendering/project-output-plan:absent-var-acknowledged`
- add `rendering/inplace-and-placeholders:absent-var-acknowledged`
- remove `rendering/project-output-plan:authoring-comment-inplace-inert`
- add `rendering/inplace-and-placeholders:authoring-comment-inplace-inert`
- remove `rendering/project-output-plan:authoring-comment-stripped`
- add `rendering/inplace-and-placeholders:authoring-comment-stripped`
- remove `rendering/project-output-plan:escaped-placeholder-literal`
- add `rendering/inplace-and-placeholders:escaped-placeholder-literal`
- remove `rendering/project-output-plan:in-place-readback`
- add `rendering/inplace-and-placeholders:in-place-readback`
- remove `rendering/project-output-plan:in-place-spacing-owned`
- add `rendering/inplace-and-placeholders:in-place-spacing-owned`
- remove `rendering/project-output-plan:in-place-tamper-drift`
- add `rendering/inplace-and-placeholders:in-place-tamper-drift`
- remove `rendering/project-output-plan:part-placeholder-sandboxed`
- add `rendering/inplace-and-placeholders:part-placeholder-sandboxed`
- remove `rendering/project-output-plan:placeholder-value-token-free`
- add `rendering/inplace-and-placeholders:placeholder-value-token-free`
- remove `rendering/project-output-plan:section-orphan-flagged`
- add `rendering/inplace-and-placeholders:section-orphan-flagged`
- remove `rendering/project-output-plan:section-source-exclusive`
- add `rendering/inplace-and-placeholders:section-source-exclusive`
- remove `rendering/project-output-plan:unused-data-drift`
- add `rendering/inplace-and-placeholders:unused-data-drift`
- remove `rendering/project-output-plan:unused-var-drift`
- add `rendering/inplace-and-placeholders:unused-var-drift`
- remove `rendering/project-output-plan:local-catalog-clone`
- add `rendering/local-artifacts:local-catalog-clone`
- remove `rendering/project-output-plan:local-doc-catalog-clone`
- add `rendering/local-artifacts:local-doc-catalog-clone`
- remove `rendering/project-output-plan:local-doc-map-fields`
- add `rendering/local-artifacts:local-doc-map-fields`
- remove `rendering/project-output-plan:local-doc-no-shadow`
- add `rendering/local-artifacts:local-doc-no-shadow`
- remove `rendering/project-output-plan:local-doc-renders-from-base`
- add `rendering/local-artifacts:local-doc-renders-from-base`
- remove `rendering/project-output-plan:local-doc-requires-declaration`
- add `rendering/local-artifacts:local-doc-requires-declaration`
- remove `rendering/project-output-plan:local-frontmatter`
- add `rendering/local-artifacts:local-frontmatter`
- remove `rendering/project-output-plan:local-no-shadow`
- add `rendering/local-artifacts:local-no-shadow`
- remove `rendering/project-output-plan:local-renders-from-base`
- add `rendering/local-artifacts:local-renders-from-base`
- remove `rendering/project-output-plan:local-requires-declaration`
- add `rendering/local-artifacts:local-requires-declaration`
- remove `rendering/project-output-plan:adr-system-singletons-rendered`
- add `rendering/singletons-and-payloads:adr-system-singletons-rendered`
- remove `rendering/project-output-plan:bootstrap-config-tree-path`
- add `rendering/singletons-and-payloads:bootstrap-config-tree-path`
- remove `rendering/project-output-plan:bootstrap-two-files`
- add `rendering/singletons-and-payloads:bootstrap-two-files`
- remove `rendering/project-output-plan:hook-payloads-rendered`
- add `rendering/singletons-and-payloads:hook-payloads-rendered`
- remove `rendering/project-output-plan:memory-gitignore-always-on`
- add `rendering/singletons-and-payloads:memory-gitignore-always-on`
- remove `rendering/project-output-plan:plain-singleton-via-renderkind`
- add `rendering/singletons-and-payloads:plain-singleton-via-renderkind`
- remove `rendering/project-output-plan:shebang-rendered-executable`
- add `rendering/singletons-and-payloads:shebang-rendered-executable`
- remove `rendering/project-output-plan:singleton-kinds-complete`
- add `rendering/singletons-and-payloads:singleton-kinds-complete`
- remove `rendering/project-output-plan:workflow-telemetry-governed-outputs-and-resident-data`
- add `rendering/singletons-and-payloads:workflow-telemetry-governed-outputs-and-resident-data`
- remove `rendering/project-output-plan:domain-doc-regenerated`
- add `rendering/doc-outputs:domain-doc-regenerated`
- remove `rendering/project-output-plan:domains-dir-given`
- add `rendering/doc-outputs:domains-dir-given`
- remove `rendering/project-output-plan:layout-derivation`
- add `rendering/doc-outputs:layout-derivation`
- remove `rendering/project-output-plan:layout-docs-enabled-only`
- add `rendering/doc-outputs:layout-docs-enabled-only`
- remove `rendering/project-output-plan:pitfall-adr-link-resolved`
- add `rendering/doc-outputs:pitfall-adr-link-resolved`
- remove `rendering/project-output-plan:pitfall-data-validated`
- add `rendering/doc-outputs:pitfall-data-validated`
- remove `rendering/project-output-plan:pitfall-domains-resolved`
- add `rendering/doc-outputs:pitfall-domains-resolved`
- remove `rendering/project-output-plan:skill-ref-dead-fails`
- add `rendering/doc-outputs:skill-ref-dead-fails`
- remove `rendering/project-output-plan:skill-ref-unknown-ignored`
- add `rendering/doc-outputs:skill-ref-unknown-ignored`
- remove `rendering/project-output-plan:stub-notes-path-keyed`
- add `rendering/doc-outputs:stub-notes-path-keyed`
- remove `rendering/project-output-plan:topic-output-complete`
- add `rendering/doc-outputs:topic-output-complete`
- remove `rendering/project-output-plan:working-with-awf-mandatory`
- add `rendering/doc-outputs:working-with-awf-mandatory`

## Consequences

- The budget advisory goes quiet only after the final batch; intermediate states keep it
  firing for not-yet-split offenders, which is acceptable because it is non-failing.
- Provenance flattening is accepted: 157 claims will show this ADR as `Origin:`, and 16 of
  them lose active `Revised-by` entries. The original rationale is one drilldown away via
  history on the removed id, and this ADR records the full mapping.
- Mirrored selectors leave `awf context --full` packet volume for broad-selector paths
  unchanged: an `internal/project/**` path still matches all six successor topics, so the
  same claims apply, only better grouped. Drilldowns, topic docs, and the budget advisory
  improve now; packet shrinkage arrives with the follow-up selector narrowing.
- Every move is one-way: removed ids are never reused, so a claim cannot return to its old
  id. Old-id history reachability depends on the source topics staying alive; all five
  survive as core seams, and retiring one later would orphan its removed-id history.
- Rendered fallout per batch: new `docs/topics/*` documents appear, moved claims leave
  their old topic documents, and domain docs regenerate. This lands with each batch.
- Marker rewrites touch a handful of adopter-facing files (comment-only changes), so the
  repoaudit changelog advisory will warn; accepted as benign, no changelog entry is owed
  because no template, CLI, or schema behavior changes.
- The 15 new topic shells (metadata plus empty-claim parts) land with this proposal so the
  ADR can be accepted; claims land only as batches apply.
- Post-split, every topic touched by this split holds at most 17 claims (sync-and-drift),
  and the corpus-wide maximum becomes 18, under the budget of 20, unblocking the follow-up
  severity-promotion ADR.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| One ADR per offender (5 ADRs) | Five review cycles restating the same criteria and provenance convention; the Pi regrouping spans two offenders and would entangle two ADRs anyway. |
| Single big-bang transaction | A 157-claim, 300-plus-site diff in one commit is unreviewable and unresumable. |
| Provenance-preserving move operation in the checker | A schema and lifecycle change to the current-state format for a one-off corpus event; history stays reachable via the removed id, so the tooling investment is not warranted. |
| In-part claim grouping instead of splitting | The part parser forbids non-claim headings inside the Claims region; grouping is an adopter-facing format change and would not silence the advisory or shrink context packets. |
| Merge audit claims into `tooling/audit-and-snapshots` | 14 existing plus 7 incoming claims lands at 21, immediately over budget. |
| One merged Pi topic | 20 claims sits exactly at budget with zero headroom; the runtime floor and the workflow contracts are separable areas. |
| Raise `maxClaimsPerTopic` | Legalizes the smell instead of fixing it; drilldowns and context packets stay oversized. |

## Status history

- 2026-07-22: Proposed
- 2026-07-22: Accepted; content-sha256: 1ec4a8afee213941a5a600a79737e8b331e9f61e07a22c014573e3fef5b05819
- 2026-07-22: Implementing; content-sha256: 1ec4a8afee213941a5a600a79737e8b331e9f61e07a22c014573e3fef5b05819
- 2026-07-22: Applied; state-sequence: 21; operations: remove `rendering/catalog-and-targets:pi-child-process-safety`, add `rendering/pi-runtime:pi-child-process-safety`, remove `rendering/catalog-and-targets:pi-child-tool-boundaries`, add `rendering/pi-runtime:pi-child-tool-boundaries`, remove `rendering/catalog-and-targets:pi-extension-target-render`, add `rendering/pi-runtime:pi-extension-target-render`, remove `rendering/catalog-and-targets:pi-implementation-state-boundary`, add `rendering/pi-runtime:pi-implementation-state-boundary`, remove `rendering/catalog-and-targets:pi-minimum-runtime`, add `rendering/pi-runtime:pi-minimum-runtime`, remove `rendering/catalog-and-targets:pi-real-runtime-smoke`, add `rendering/pi-runtime:pi-real-runtime-smoke`, remove `rendering/catalog-and-targets:pi-session-handoff-lifecycle`, add `rendering/pi-workflows:pi-session-handoff-lifecycle`
- 2026-07-22: Applied; state-sequence: 22; operations: remove `config/configuration:config-reference-data-rejected`, add `config/configspec-and-reference:config-reference-data-rejected`, remove `config/configuration:config-reference-no-bare-vars`, add `config/configspec-and-reference:config-reference-no-bare-vars`, remove `config/configuration:config-reference-regen-drift`, add `config/configspec-and-reference:config-reference-regen-drift`, remove `config/configuration:configspec-data-parity`, add `config/configspec-and-reference:configspec-data-parity`, remove `config/configuration:configspec-description-residue`, add `config/configspec-and-reference:configspec-description-residue`, remove `config/configuration:configspec-key-parity`, add `config/configspec-and-reference:configspec-key-parity`, remove `config/configuration:configspec-var-derivation`, add `config/configspec-and-reference:configspec-var-derivation`, remove `config/configuration:domain-name-validated`, add `config/validation:domain-name-validated`, remove `config/configuration:duplicate-target-rejected`, add `config/validation:duplicate-target-rejected`, remove `config/configuration:glob-migration-anchored`, add `config/validation:glob-migration-anchored`, remove `config/configuration:local-doc-name-path-validated`, add `config/validation:local-doc-name-path-validated`, remove `config/configuration:local-name-validated`, add `config/validation:local-name-validated`, remove `config/configuration:pathglob-anchored`, add `config/validation:pathglob-anchored`, remove `config/configuration:tag-not-domain-name`, add `config/validation:tag-not-domain-name`, remove `config/configuration:testglobs-anchored-validated`, add `config/validation:testglobs-anchored-validated`
- 2026-07-22: Applied; state-sequence: 23; operations: remove `tooling/cli:context-adr-operation-projection`, add `tooling/context-and-topic:context-adr-operation-projection`, remove `tooling/cli:context-applicability-navigation`, add `tooling/context-and-topic:context-applicability-navigation`, remove `tooling/cli:context-default-excludes-history`, add `tooling/context-and-topic:context-default-excludes-history`, remove `tooling/cli:context-full-authority-packet`, add `tooling/context-and-topic:context-full-authority-packet`, remove `tooling/cli:context-known-artifact-navigation`, add `tooling/context-and-topic:context-known-artifact-navigation`, remove `tooling/cli:context-output-parity`, add `tooling/context-and-topic:context-output-parity`, remove `tooling/cli:context-path-attribution`, add `tooling/context-and-topic:context-path-attribution`, remove `tooling/cli:context-path-classification`, add `tooling/context-and-topic:context-path-classification`, remove `tooling/cli:context-read-only`, add `tooling/context-and-topic:context-read-only`, remove `tooling/cli:context-static-fallback`, add `tooling/context-and-topic:context-static-fallback`, remove `tooling/cli:describe-read-only`, add `tooling/context-and-topic:describe-read-only`, remove `tooling/cli:uncovered-collapses-directories`, add `tooling/context-and-topic:uncovered-collapses-directories`, remove `tooling/cli:uncovered-output-parity`, add `tooling/context-and-topic:uncovered-output-parity`, remove `tooling/cli:add-applies-closure-plan`, add `tooling/init-and-enablement:add-applies-closure-plan`, remove `tooling/cli:add-skill-pairs-agent`, add `tooling/init-and-enablement:add-skill-pairs-agent`, remove `tooling/cli:explicit-answers-win`, add `tooling/init-and-enablement:explicit-answers-win`, remove `tooling/cli:init-collision-guard`, add `tooling/init-and-enablement:init-collision-guard`, remove `tooling/cli:init-force-backs-up`, add `tooling/init-and-enablement:init-force-backs-up`, remove `tooling/cli:init-hooks-default-on`, add `tooling/init-and-enablement:init-hooks-default-on`, remove `tooling/cli:init-noninteractive-default`, add `tooling/init-and-enablement:init-noninteractive-default`, remove `tooling/cli:init-prompts-enabled-vars`, add `tooling/init-and-enablement:init-prompts-enabled-vars`, remove `tooling/cli:init-set-closed`, add `tooling/init-and-enablement:init-set-closed`, remove `tooling/cli:init-unborn-head-supported`, add `tooling/init-and-enablement:init-unborn-head-supported`, remove `tooling/cli:new-seeds-scaffold-vars`, add `tooling/init-and-enablement:new-seeds-scaffold-vars`, remove `tooling/cli:remove-agent-pairing-guard`, add `tooling/init-and-enablement:remove-agent-pairing-guard`, remove `tooling/cli:remove-refuses-dependents`, add `tooling/init-and-enablement:remove-refuses-dependents`, remove `tooling/cli:audit-empty-range-announced`, add `tooling/audit-commands:audit-empty-range-announced`, remove `tooling/cli:audit-reports-evaluated-scope`, add `tooling/audit-commands:audit-reports-evaluated-scope`, remove `tooling/cli:audit-requires-explicit-range`, add `tooling/audit-commands:audit-requires-explicit-range`, remove `tooling/cli:audit-scopes-descriptor-routed`, add `tooling/audit-commands:audit-scopes-descriptor-routed`, remove `tooling/cli:audit-warn-exit-zero`, add `tooling/audit-commands:audit-warn-exit-zero`, remove `tooling/cli:mutants-missing-report-errors`, add `tooling/audit-commands:mutants-missing-report-errors`, remove `tooling/cli:repoaudit-requires-explicit-range`, add `tooling/audit-commands:repoaudit-requires-explicit-range`
