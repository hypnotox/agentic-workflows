---
format: plan-v2
date: 2026-08-29
adrs: [semantic-artifact-authoring-commands]
status: Proposed
---
# Plan: Implement Semantic Artifact Authoring Commands

## Goal

Provide retryable non-interactive commands that edit or reset one semantically identified artifact part or sidecar leaf, validate the complete candidate project before source publication, and finish successful operations through ordinary rendering and lock publication. Editor invocation, path-based targets, batch mutation, arbitrary mapping replacement, user-visible concurrency tokens, and crash-atomicity claims remain out of scope.

## Architecture summary

The command specification remains the sole owner of the `edit` and `reset` grammar. Thin CLI handlers translate exact-one direct or structured input flags into typed requests for a dedicated authoring operation. Project and kind authorities resolve `<kind> <name>` plus a declared part or capability-valid sidecar leaf; configuration remains the only owner of part and sidecar paths and YAML-node serialization. Free-form sidecar data does not gain a speculative complete value-schema registry: structural operation compatibility is resolved before mutation, and the existing complete-project validation and render path decide remaining semantic validity.

The authoring operation acquires the complete project lease before mutable authority reads, observes the exact selected source, and constructs one candidate overlay consumed through both configuration-tree and project-tree reader contracts. It prepares that candidate without mutation, publishes only the observed source through the confined filesystem boundary, reloads committed authority, and calls ordinary leased synchronization. Pre-source failures preserve every byte. Post-source failures retain typed source, setup, and publisher effects with executable recovery actions. Convention parts remain raw source files; local documents expose only `body`, whose exact marker and framing semantics have one reusable owner shared with publisher readback.

Execution first extracts that reusable in-place boundary policy without changing behavior. It then lands a complete vertical slice for ordinary parts and local bodies, including the common transaction and its first current-state applications. A final vertical slice adds YAML-node sidecar mutation, the `edit sidecar` and `reset sidecar` grammar, remaining claim applications, and complete user documentation. Every phase is independently green, and no production definition lands without a real consumer.

## Phase 1: Establish one reusable in-place body boundary

**Execution mode: subagent-driven.**

Advances: ["safe-authoring-transaction"]

### Task 1.1: Extract and prove the shared local-body boundary model
Applying: ["semantic-artifact-authoring-commands:local-document-body"]
Paths: ["internal/publisher/render.go", "internal/publisher/inplace.go", "internal/publisher/inplace_test.go", "internal/publisher/render_test.go"]

Starting from the reviewed Proposed ADR and the existing publisher in-place readback tests on the clean effort branch, move the exact registered-pointer, expected-heading, body-framing, next-boundary, and final-section rules from the publisher readback path into one publisher-owned semantic boundary model that the existing render path immediately consumes. Preserve byte-for-byte body interiors, regenerated structural framing, missing-pointer fallback, pointer-shaped adopter lines, headingless and headed sections, and the current local-document `body` default. Add focused boundary tests that exercise extraction and reconstruction inputs without introducing a second marker parser, exporting only the smallest operation needed by the later authoring consumer. Run the affected publisher package tests and require existing in-place sync/check behavior to remain green.

### Phase close

```commit
refactor(rendering): share in-place body boundaries
```

## Phase 2: Author convention parts and local-document bodies

**Execution mode: subagent-driven.**

Completes: ["semantic-part-authoring", "safe-authoring-transaction"]

### Task 2.1: Establish part-authoring command and transaction oracles
Applying: ["semantic-artifact-authoring-commands:explicit-semantic-targets", "semantic-artifact-authoring-commands:direct-part-input", "semantic-artifact-authoring-commands:capability-aware-resolution", "semantic-artifact-authoring-commands:local-document-body", "semantic-artifact-authoring-commands:validated-publication-transaction", "semantic-artifact-authoring-commands:friction-boundary"]
Paths: ["internal/authoringop/authoring_test.go", "internal/authoringop/resolver_test.go", "internal/publisher/inplace_test.go", "internal/clispec/clispec_test.go", "cmd/awf/authoring_test.go", "cmd/awf/help_test.go", "cmd/awf/global_help_test.go", "cmd/awf/testdata/help/global.txt"]

Starting from Phase 1's publisher-owned boundary model and its green existing-consumer proof, before production implementation add focused tests and observe the intended failures for `awf edit <kind> <name> <part>` with exactly one present `--content` value or `--stdin`, and `awf reset <kind> <name> <part>` with no content mode. Cover every closed kind, catalog names, configured local-document names, singleton and plural part layouts, undeclared parts, capability-valid generated documents, local `body`, rejected local part names, empty authored content, malformed arity, conflicting input modes, and noninteractive stdin. Prove that invalid identity, content, candidate validation, or concurrency leaves source, output, and lock bytes unchanged; that successful create, replace, and reset leave source, generated output, and lock coherent; and that a post-source publication fault reports the committed source or setup axes separately from publisher effects with residue-first recovery. Before implementation, add a distinct lease-release-fault case through the narrowest existing or directly injected operation dependency, retaining source and publisher effects, matching the cause by identity, and presenting residue-first executable recovery without a global test seam. Update the exact gated-command oracle for the new top-level `edit` and `reset` members.

### Task 2.2: Compose semantic resolution, candidate validation, and source-to-sync publication
Applying: ["semantic-artifact-authoring-commands:explicit-semantic-targets", "semantic-artifact-authoring-commands:direct-part-input", "semantic-artifact-authoring-commands:capability-aware-resolution", "semantic-artifact-authoring-commands:local-document-body", "semantic-artifact-authoring-commands:validated-publication-transaction", "semantic-artifact-authoring-commands:friction-boundary"]
Paths: ["internal/authoringop/authoring.go", "internal/authoringop/resolver.go", "internal/authoringop/outcome.go", "internal/project/kind.go", "internal/project/project.go", "internal/config/config.go", "internal/publisher/inplace.go", "internal/publisher/sync.go", "internal/filesystem/handle.go", "internal/clispec/clispec.go", "cmd/awf/authoring.go", "cmd/awf/dispatch.go", "cmd/awf/main.go"]

Add the dedicated operation boundary and thin CLI composition. Resolve singular kinds through the existing kind descriptor and selected catalog view, derive standard part paths only through configuration APIs, and map configured local documents only to synthetic `body`. Keep generated status independent from part authorability. Distinguish flag presence from content value so an explicit empty part remains valid, and consume stdin only through the runner-owned input seam.

Acquire the complete lease before loading mutable authority. Observe the selected convention part or local output identity, overlay the candidate bytes through both reader interfaces, open and prepare the complete candidate project without publishing, then create, replace, remove, or reconstruct only the selected source through confined expected-identity operations. Reload the committed tree and invoke ordinary `SyncLeased`; never publish the previously prepared candidate plan. Model source replacement or removal, created parent residue, local-body replacement, publisher effects, release faults, and their recovery steps in an authoring-owned typed outcome while preserving the publisher's own semantic result. Reuse existing filesystem and presentation mechanisms rather than adding direct writes, error-string branching, global test seams, rollback, or another publication path. Run the authoring-operation, publisher, project, filesystem, and CLI packages after the focused tests turn green.

### Task 2.3: Apply part and transaction authority with operational documentation
Kind: batch
Applying: ["semantic-artifact-authoring-commands:direct-part-input", "semantic-artifact-authoring-commands:local-document-body", "semantic-artifact-authoring-commands:validated-publication-transaction", "semantic-artifact-authoring-commands:current-state-assurance"]
Paths: [".awf/domains/tooling.yaml", ".awf/topics/metadata/tooling/cli.yaml", ".awf/docs/parts/architecture/components.md", ".awf/parts/working-with-awf/config-and-overrides.md", ".awf/parts/working-with-awf/commands.md", ".awf/topics/parts/rendering/inplace-and-placeholders/current-state.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", "README.md", "docs/architecture.md", "docs/working-with-awf.md", "glob:docs/topics/tooling/*.md", "docs/topics/rendering/inplace-and-placeholders.md", "docs/topics/rendering/sync-and-drift.md", "docs/decisions/semantic-artifact-authoring-commands.md", "docs/decisions/INDEX.md", "changelog/CHANGELOG.md", ".awf/awf.lock", "x"]
Representative: ["replace a standard convention part from stdin and observe the rendered section plus lock update", "reset a configured local document body to its empty inherited default without changing its declaration or shell"]
Edge: ["empty convention-part content remains an authored override", "a local-document reset changes only the body", "candidate refusal precedes every source effect", "post-source faults disclose committed source and publisher axes without claiming rollback"]
Post-check: "For the lifecycle snapshot transitioning ADR-semantic-artifact-authoring-commands from Proposed to Implementing, apply exactly the update of `rendering/inplace-and-placeholders:local-doc-body-inline` and the add of `rendering/sync-and-drift:authoring-sync-transaction` with matching test-backed claim mutations. Render from `.awf/`, inspect the README command projection, architecture inventory, command examples, and both generated topic claims for coherent part, local-body, candidate-validation, and partial-effect semantics. Run `./awf resolve topic internal/authoringop/authoring.go internal/authoringop/resolver.go internal/authoringop/outcome.go` and require the new package to resolve to the tooling domain and `tooling/cli`. Inspect the complete changed `docs/topics/tooling/*.md` population projected from the domain selector, require every such generated diff to be declared by this batch and no generated diff outside the declared Paths, then require the focused clispec, CLI, and authoring suites plus `./x check` to finish cleanly with no remaining operation mismatch."

Assign `internal/authoringop/**` to the tooling domain and `tooling/cli` topic, and add the focused operation package to the authored architecture component inventory. Document the part commands, exact input modes, semantic identities, inherited reset behavior, automatic render, and partial-outcome recovery. Transition the ADR to Implementing and apply only the two rendering operations whose complete behavior now exists. Add their `invariant:` proof markers with tests from Tasks 2.1 and 2.2, regenerate the root README and every managed output, and record the user-visible capability in the changelog without documenting the not-yet-landed sidecar child.

### Phase close

```commit
feat(tooling): author semantic document parts
```

## Phase 3: Author typed sidecar leaves and complete the surface

**Execution mode: subagent-driven.**

Completes: ["semantic-sidecar-authoring", "complete-authoring-authority"]

### Task 3.1: Establish YAML and sidecar command oracles
Applying: ["semantic-artifact-authoring-commands:explicit-semantic-targets", "semantic-artifact-authoring-commands:leaf-sidecar-mutation", "semantic-artifact-authoring-commands:capability-aware-resolution", "semantic-artifact-authoring-commands:validated-publication-transaction", "semantic-artifact-authoring-commands:friction-boundary"]
Paths: ["internal/config/edit_test.go", "internal/config/tree_reader_test.go", "internal/authoringop/sidecar_test.go", "internal/project/validate_test.go", "cmd/awf/authoring_test.go", "cmd/awf/help_test.go", "cmd/awf/global_help_test.go"]

Starting from Phase 2's live `edit` and `reset` families, semantic resolver, candidate overlay, typed outcome, and ordinary publication transaction, before production implementation add focused failures for `awf edit sidecar <kind> <name> <field>` accepting exactly one of `--value`, `--json-value`, `--add`, `--add-json`, `--remove`, or `--remove-json`, and for `awf reset sidecar <kind> <name> <field>`. Cover string scalars, JSON scalars and mappings, string lists, structured mapping lists, strict single-value JSON with no trailing document, duplicate structural values, absent removal, authored-list order, whole replacement, missing-sidecar creation, absent reset, empty-parent pruning, and final sidecar removal. Prove comment and unrelated-key ordering preservation across block and flow YAML, capability rejection for intermediate mappings and inert kind fields, arbitrary valid `data.<key>` leaves without a fabricated global value schema, domain `paths`, `dataDefaults` booleans, section `drop` booleans, catalog-default versus authored-list separation, complete structured removal, invalid candidate no-mutation, confined path behavior, and source/publisher partial outcomes.

### Task 3.2: Extend the authoring transaction with YAML-node leaf mutation
Applying: ["semantic-artifact-authoring-commands:explicit-semantic-targets", "semantic-artifact-authoring-commands:leaf-sidecar-mutation", "semantic-artifact-authoring-commands:capability-aware-resolution", "semantic-artifact-authoring-commands:validated-publication-transaction", "semantic-artifact-authoring-commands:friction-boundary"]
Paths: ["internal/config/edit.go", "internal/config/config.go", "internal/configspec/spec.go", "internal/project/kind.go", "internal/project/loader_validation.go", "internal/authoringop/authoring.go", "internal/authoringop/resolver.go", "internal/authoringop/sidecar.go", "internal/authoringop/outcome.go", "internal/clispec/clispec.go", "cmd/awf/authoring.go", "cmd/awf/dispatch.go"]

Add configuration-owned YAML-node operations that target one resolved leaf, encode scalar modes as YAML strings, decode JSON modes as exactly one YAML-compatible structured value, compare list entries structurally, and preserve untouched nodes and comments through the existing two-space encoder. Add and remove operate only on the authored leaf, never the effective catalog-default merge; absent add creates the authored list, absent remove is unchanged, and reset prunes empty ancestors and the empty sidecar. Do not introduce a complete machine-readable type registry for free-form `data`: resolve known structural capabilities and operation shape, then depend on the overlaid strict project load and ordinary render validation for remaining semantics.

Extend resolution and CLI dispatch with the `sidecar` child beneath both families, retaining parent part invocation and exact help derivation from clispec. Reuse Phase 2's lease, observation, overlay, source publication, committed-authority reload, synchronization, outcome, and presentation path for sidecar create, replace, unchanged, and removal. Ensure unchanged idempotent operations neither rewrite YAML nor trigger a misleading changed-source result, while still returning a complete successful command outcome. Run the focused config, configspec, project, authoring, CLI, publisher, and filesystem feedback after the red cases turn green.

### Task 3.3: Apply sidecar and complete CLI authority
Kind: batch
Applying: ["semantic-artifact-authoring-commands:explicit-semantic-targets", "semantic-artifact-authoring-commands:leaf-sidecar-mutation", "semantic-artifact-authoring-commands:capability-aware-resolution", "semantic-artifact-authoring-commands:current-state-assurance"]
Paths: [".awf/parts/working-with-awf/config-and-overrides.md", ".awf/parts/working-with-awf/commands.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/config/configuration/current-state.md", "README.md", "docs/working-with-awf.md", "docs/topics/tooling/cli.md", "docs/topics/config/configuration.md", "docs/config-reference.md", "docs/decisions/semantic-artifact-authoring-commands.md", "docs/decisions/INDEX.md", "changelog/CHANGELOG.md", ".awf/awf.lock"]
Representative: ["add a structured mapping to a skill sidecar list with `--add-json` and safely retry it", "reset the final authored leaf and observe the sidecar file disappear before coherent rendering"]
Edge: ["scalar flags always produce strings", "JSON flags preserve structured equality", "catalog defaults are not removed by authored-list removal", "generated documents remain editable only for supported fields", "no top-level `set` command or intermediate mapping mutation exists"]
Post-check: "For the Implementing ADR snapshot, apply exactly the adds of `tooling/cli:semantic-artifact-authoring` and `config/configuration:sidecar-authoring-roundtrip` with their test-backed claim sources, leaving every declared operation Applied and the ADR nonterminal. Render from `.awf/`, inspect root help, working guidance, configuration reference, and generated CLI/configuration topics for exact grammar, scalar-versus-JSON meaning, idempotence, cleanup, and recovery with no contradictory or premature text. Require focused config, project, authoring, CLI, publisher, and filesystem feedback, `./x check`, and the ADR operation projection to terminate cleanly with no Remaining operation."

Document the complete parallel grammar, dotted leaves, scalar and JSON modes, authored-list behavior, reset cleanup, retry semantics, and examples for string and structured lists. Add the tooling and configuration invariant claims with their deterministic proofs, apply the ADR's remaining operations, regenerate every help and managed documentation projection, and update the changelog. Preserve the ADR's Implementing status for post-implementation assurance and deferred effort closure.

### Phase close

```commit
feat(config): author semantic sidecar fields
```

## Definition of done

- `dod: semantic-part-authoring` `awf edit <kind> <name> <part>` and `awf reset <kind> <name> <part>` resolve every supported artifact semantically, accept the exact direct-input grammar, preserve empty overrides, restore inherited convention parts, and edit or reset only a local document's synthetic body.
- `dod: safe-authoring-transaction` Every operation validates one candidate universe through both reader contracts before source publication, uses confined observed-identity mutation, reloads committed authority for ordinary leased sync, leaves all bytes unchanged on pre-source refusal, and reports every later source, setup, and publisher effect with actionable recovery.
- `dod: semantic-sidecar-authoring` `edit sidecar` and `reset sidecar` implement exact scalar and JSON whole/list modes, structurally idempotent ordered list changes, YAML comment/order preservation, capability-aware leaf validation, authored-default separation, empty-parent pruning, and final sidecar removal without a top-level `set` family.
- `dod: complete-authoring-authority` Help, operational documentation, changelog, current-state claims, generated outputs, lock state, and the linked ADR operation projection describe the implemented surface consistently; every declared operation is Applied while the ADR and plan remain nonterminal until assurance and effort closure.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record implementation findings and any material route deviations here.

- Plan review settlement: a reasoned assurance finding identified lease-release failure as a distinct authoring-owned terminal outcome already named by Task 2.2 but not established red in Task 2.1. The plan now requires an identity-matched pre-implementation release-fault oracle through a direct operation dependency, retaining committed axes and residue-first recovery without adding a global seam. Mechanical corrections also assign the new operation package to tooling and CLI authority, update the architecture inventory, and move the gated-command and README projections into the first phase that changes them.
- Phase 2 route reconciliation: the first delegated pass established the part-authoring model and stopped at the declared confinement boundary when the clispec-derived global-help golden also required mutation. `cmd/awf/testdata/help/global.txt` is now explicitly assigned to Task 2.1 so exact public-help evidence travels with the top-level command registration; this adds no behavior or scope beyond the approved grammar. The resumed pass then exposed `x` as the exhaustive qualified Go-shard inventory for the newly introduced production package; Task 2.3 now owns that required gate integration so the independently green phase can pass the repository's complete package census.
- Phase 2 review settlement: normalize a plain deferred cleanup fault into the typed partial outcome whenever source, setup, residue, or publisher effects committed; strengthen the transaction claim with direct lease-ordering, dual-reader overlay, expected-identity race, publisher-fault, release-fault, and committed-authority synchronization oracles. Configured local-document `body` remains the sole final in-place section: exact-prefix evidence now proves that model, so no nonexistent rendered suffix is reconstructed. The bounded verify pass also assigns `SourceRemoved` when reset commits before cleanup fails, retaining the complete source axis. The immutable reviewed range is `8b66651bac0de6469342d04c6621f86b9e864aa9..703ada33ea569e6394ae4d2628a64654c7bcb8ec`; the earlier review brief's expanded base SHA was a transcription error.
- Phase 3 route reconciliation: parent completion expanded the thin delegated oracle set to cover typed CLI modes, invalid-candidate byte preservation, structural JSON equality, pruning, semantic capability refusal, known boolean controls, domain paths, and mandatory versus catalog-document sidecar layouts. Mandatory catalog docs resolve to their root singleton sidecars while nonmandatory docs retain `.awf/docs/<name>.yaml`; this uses existing catalog ownership and avoids authoring inert parallel files.
