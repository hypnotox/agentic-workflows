---
format: plan-v2
date: 2026-08-15
adrs:
  - depend-on-pi-tools-for-general-pi-tooling
status: Proposed
---
# Plan: Transition Pi tooling to pi-tools

## Goal

Make `pi-tools` the independently installed provider of general Pi context, handoff, and subagent execution while awf renders only its workflow-specific profile adapter and effort integration; do not pin or vendor the external package or move awf policy into it.

## Architecture summary

Replace the generated awf subprocess implementation with one protocol-v2 consumer extension that registers grounding, exploration, review, and implementation as an atomic `pi-tools` profile batch. The adapter owns schemas, prompt guidance, rendered role loading, async model preferences, tool policies, and implementation Git audits; `pi-tools` owns scheduling, child execution, confinement, execution facts, and rendering. Remove awf's context-usage, handoff, and runner outputs in the same green transaction as the adapter cutover, retain the effort extension, and make missing or rejected handshake capability an actionable no-fallback prerequisite failure. Contract-compatible test doubles prove the handshake without coupling the awf gate to a pinned `pi-tools` checkout.

## Phase 1: Cut over the complete Pi tooling boundary

**Execution mode: subagent-driven.**

Completes: ["awf-profile-adapter-operational", "awf-profile-policy-preserved", "pi-tools-boundary-established", "generated-general-tooling-removed", "pi-tools-prerequisite-documented", "current-authority-settled"]

### Task 1.1: Implement the protocol-v2 profile adapter
Applying: ["depend-on-pi-tools-for-general-pi-tooling:handshake-profile-integration", "depend-on-pi-tools-for-general-pi-tooling:retained-awf-profile-policy", "depend-on-pi-tools-for-general-pi-tooling:compatibility-not-version-pinning"]
Paths: ["templates/pi/awf-subagents/index.ts.tmpl", "templates/pi/awf-subagents/model-routing.ts.tmpl", "templates/pi/awf-subagents/runner.ts.tmpl", "tools/pi-extension-test/tests/index.test.ts", "tools/pi-extension-test/tests/runner.test.ts", "tools/pi-extension-test/fixtures/pi-tools-subagent-profile.d.ts", "tools/pi-extension-test/tsconfig.json"]

Begin only after ADR-0279 is review-settled and Accepted. Treat the phase as one atomic cutover because the adapter rewrite, obsolete-output pruning, runtime compatibility claims, container inputs, and target-render authority cannot remain independently truthful at an intermediate commit. Rewrite the generated subagent entrypoint as a runtime-import-free protocol-v2 consumer. Import the canonical `pi-tools/subagent-profile` surface as types only, with a test-only declaration and compiler mapping that disappears from emitted runtime code. Subscribe to capability and registration-result events before emitting a uniquely correlated request; use one stable awf registration id, accept a matching reply or availability announcement idempotently, register all four profiles atomically with `suppressDefault: true`, and issue one actionable installation or compatibility notice for missing, incompatible, late, or rejected registration. A failed batch activates no awf fallback and does not claim ownership of the external generic `subagent` tool.

Define grounding, exploration, review, and implementation profiles with their existing closed argument schemas, descriptions, prompt snippets, and prompt guidelines. Preserve rendered agent-contract loading and frontmatter stripping, role-specific tool allowlists, exploration, grounding, and review concurrency of ten, implementation concurrency of one, and implementation parent-batch exclusivity. Use async model selection to reload global and project-local preferences for every invocation, validate explicit and configured exact references against the live session model registry, preserve the preference wizard and routing diagnostics in bounded profile data, and remove only the second historical reload after queue acquisition.

Use profile preparation for root-CWD role prompts and tool policy. Use lifecycle state and bounded profile data for requested/resolved routing facts, role options, and implementation audit facts. Implementation `beforeRun` resolves the caller-selected verification checkout and records the before snapshot; `afterRun` records the after snapshot and returns a terminal policy failure, with both snapshots retained, when commits were forbidden but HEAD changed or commits were allowed but HEAD did not change. Preserve cancellation authority and the existing repair wording. Delete the awf runner implementation and its execution-mechanics tests; replace them with adapter contract tests covering both load orders, idempotent replay, each terminal registration outcome, once-only missing-capability reporting, profile metadata and schemas, async routing, role preparation, tool confinement, concurrency declarations, and successful and failed implementation audits.

### Task 1.2: Prune general extension outputs and reshape their assurance
Applying: ["depend-on-pi-tools-for-general-pi-tooling:external-general-pi-tooling", "depend-on-pi-tools-for-general-pi-tooling:separated-assurance-ownership"]
Paths: ["templates/pi/awf-context-usage/index.ts.tmpl", "templates/pi/awf-handoff/index.ts.tmpl", "templates/partials/pi-minimum-runtime.md", "internal/project/target.go", "internal/project/target_test.go", "internal/project/agent_test.go", "internal/project/output_plan_test.go", "internal/project/repository_wiring_test.go", "internal/project/source_marker_test.go", "internal/project/template_source_marker_test.go", "internal/project/gate_runner_test.go", "internal/contextq/adapter_outputs_test.go", "tools/pi-extension-test/container.sh", "tools/pi-extension-test/tests/context-usage.test.ts", "tools/pi-extension-test/tests/handoff.test.ts", "tools/pi-extension-test/tests/runtime.test.ts", ".pi/extensions/awf-context-usage/index.ts", ".pi/extensions/awf-handoff/index.ts", ".pi/extensions/awf-subagents/index.ts", ".pi/extensions/awf-subagents/model-routing.ts", ".pi/extensions/awf-subagents/runner.ts", ".pi/extensions/awf-effort/index.ts", ".awf/awf.lock", "x"]

Remove context usage, handoff, and the runner from the Pi target descriptors, template source, generated output set, and lock while retaining the profile adapter index, model-routing module, and conditional effort index/client. Replace deleted representatives in target and adapter-output tests with retained awf Pi outputs. Update output-plan, source-marker, proof-marker, editor-quiet, harness-wiring, and gate-routing assertions so they prove the smaller governed set and no longer attribute external mechanics to awf.

Delete the private context, handoff, runner, scheduling, process-supervision, and common-rendering behavior suites rather than reproducing `pi-tools` implementation tests. Reshape the container and runtime smoke so awf proves generated profile negotiation through a protocol-v2 contract double, native awf skill discovery, routing delivery, and the retained effort extension without cloning, pinning, importing, or requiring a sibling `pi-tools` checkout. Preserve the strict TypeScript and coverage lane for the remaining awf-owned adapter and effort code. Render and verify that no active target, generated output, lock member, test fixture, or command-runner dependency treats context usage, handoff, or the runner as awf implementation source.

### Task 1.3: Publish the independently installed prerequisite
Applying: ["depend-on-pi-tools-for-general-pi-tooling:compatibility-not-version-pinning", "depend-on-pi-tools-for-general-pi-tooling:separated-assurance-ownership"]
Paths: ["README.md", "templates/docs/architecture.md.tmpl", ".awf/docs/parts/architecture/components.md", "templates/docs/working-with-awf.md.tmpl", ".awf/docs/parts/development/setup.md", ".awf/docs/parts/development/dependencies.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/tiers.md", ".awf/docs/parts/testing/layout.md", "docs/architecture.md", "docs/working-with-awf.md", "docs/development.md", "docs/testing.md", ".awf/awf.lock"]

State that Pi adopters install `hypnotox/pi-tools` independently and may patch or update it without an awf release or revision pin. Define successful protocol-v2 capability and final profile registration as the compatibility test, describe the actionable no-fallback failure behavior, distinguish the external general extensions from awf's rendered profile adapter and effort integration, and retain the adopter-supplied Pi runtime requirement only for awf-owned outputs. Update both the project-specific architecture override and the default template. Assign general extension mechanics to `pi-tools` and the deterministic awf lane to profile-contract, rendering, policy, effort, and handshake evidence.

Render the documentation and inspect the supported-agent, architecture component, working-with-awf prerequisite and model-routing sections plus testing lane tables. They must not say awf renders or tests its own context usage, handoff, runner, progress renderer, or subprocess supervisor, and must not imply a pinned `pi-tools` version.

### Task 1.4: Apply the complete ownership claim batch
Kind: batch
Latitude: exact
Applying: ["depend-on-pi-tools-for-general-pi-tooling:external-general-pi-tooling", "depend-on-pi-tools-for-general-pi-tooling:handshake-profile-integration", "depend-on-pi-tools-for-general-pi-tooling:retained-awf-profile-policy", "depend-on-pi-tools-for-general-pi-tooling:compatibility-not-version-pinning", "depend-on-pi-tools-for-general-pi-tooling:separated-assurance-ownership"]
Paths: ["docs/decisions/0279-depend-on-pi-tools-for-general-pi-tooling.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".awf/topics/parts/rendering/pi-workflows/current-state.md", "docs/topics/rendering/pi-runtime.md", "docs/topics/rendering/pi-workflows.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: ["Replace the awf-owned child-process and progress claims with the protocol-v2 profile integration boundary and retained awf policy claims.", "Remove the context-usage and exact handoff implementation claims while preserving workflow guidance and effort integration."]
Edge: ["Every retained invariant keeps its original Origin, adds ADR-0279 as Revised-by, and names deterministic backing.", "The new integration-boundary invariant has ADR-0279 origin; removed claim ids do not remain in rendered authority or proof markers."]
Post-check: After rendering, `./awf check repo state` exits zero and `./awf context --show pending docs/decisions/0279-depend-on-pi-tools-for-general-pi-tooling.md` reports every declared ADR-0279 operation Applied, with no Remaining or Canceled operation.

Transition the reviewed ADR from Accepted to Implementing under the lifecycle handshake. In the same implementing transaction, apply every declared operation with matching Origin or `Revised-by` provenance and deterministic backing for retained or added invariants:

- remove `rendering/pi-runtime:pi-child-process-safety`
- remove `rendering/pi-runtime:pi-child-tool-boundaries`
- remove `rendering/pi-runtime:pi-context-usage-injection`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-implementation-state-boundary`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-runtime:pi-real-runtime-smoke`
- add `rendering/pi-runtime:pi-tools-integration-boundary`
- remove `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-dedicated-grounding-dispatch`
- update `rendering/pi-workflows:pi-extension-editor-quiet-strip`
- update `rendering/pi-workflows:pi-implementation-batch-exclusivity`
- remove `rendering/pi-workflows:pi-session-handoff-public-contract`
- update `rendering/pi-workflows:pi-structured-exploration-contract`
- remove `rendering/pi-workflows:pi-subagent-failure-details`
- update `rendering/pi-workflows:pi-subagent-model-preferences`
- update `rendering/pi-workflows:pi-subagent-model-routing`
- update `rendering/pi-workflows:pi-implement-role-artifact`
- remove `rendering/pi-workflows:pi-subagent-progress-bounds`
- remove `rendering/pi-workflows:pi-subagent-progress-context-isolation`
- remove `rendering/pi-workflows:pi-subagent-progress-rendering`
- update `rendering/pi-workflows:pi-role-contract-loader`

The resulting topics describe the landed handshake boundary, the two fixed awf subagent modules, retained effort runtime floor, per-invocation rather than post-queue preference loading, profile-owned Git policy, and externally owned general mechanics. Preserve the awf session-handoff workflow guidance and effort session and memory claims because their workflow policy or implementation remains awf-owned. Inspect the complete rendered Pi topics for one coherent ownership boundary, correct provenance, no removed claim ids, and no contradiction between the external prerequisite and retained effort runtime floor.

### Phase close

The phase owner inspects the rendered adapter, generated-output pruning, prerequisite documentation, and current-state topics; runs the strict Pi container lane, focused target, contextq, output-plan, and gate-routing Go tests, protocol-v2 runtime smoke, `./x check`, `./awf check staged`, and the project gate; then closes the atomic cutover and ADR application transaction.

```commit
feat(rendering): require pi-tools (applies 0279 batch)
```

## Definition of done

- `dod: awf-profile-adapter-operational` A compatible protocol-v2 capability registers all four awf tools atomically with their prompt guidance and suppresses the generic default, while every missing, incompatible, late, or rejected path reports one actionable prerequisite failure and activates no awf fallback.
- `dod: awf-profile-policy-preserved` Rendered agent contracts, exact model preference routing, role tool policies, ten-active grounding, exploration, and review profiles, serialized exclusive implementation, and structured verification-checkout commit audits remain awf-owned and covered through profile callbacks.
- `dod: generated-general-tooling-removed` Awf renders no context-usage, handoff, or subprocess-runner implementation or lock entry; the only fixed subagent outputs are the profile adapter and model-routing module, and the conditional effort extension remains intact.
- `dod: pi-tools-boundary-established` General execution, scheduling, confinement, execution facts, rendering, context usage, and handoff are neither implemented nor behavior-tested by awf, while awf's deterministic gate proves the protocol contract without pinning or importing external runtime code.
- `dod: pi-tools-prerequisite-documented` Adopter documentation explains independent `pi-tools` installation, handshake compatibility, no-fallback failure, and the distinct awf-owned adapter and effort surfaces without naming a required package revision.
- `dod: current-authority-settled` Every ADR-0279 claim operation is Applied while the ADR remains Implementing, rendered current-state topics describe the landed boundary, `./x check` is clean, and `./x gate` passes.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- Plan review collapsed the original two-phase sequence into one atomic cutover. The adapter rewrite makes the old runtime, target-render, and process-ownership claims false before a second phase could prune them, so no independently truthful green intermediate transaction existed.
- Plan review moved the `pi-minimum-runtime` update into the same cutover because replacing the adapter changes its direct runtime compatibility boundary immediately; the retained minimum-runtime partial remains scoped to the awf effort extension after pruning.
- The review request to restate generic clean-tree and gate baseline protocol was not added because the subagent-driven execution owner already owns that protocol. The change-specific precondition that ADR-0279 is reviewed and Accepted remains explicit.

After implementation assurance settles, the effort-free execution parent owns the terminal artifact transaction: reconcile final deviations and review settlement here, append only ADR-0279's Implemented status event, change this plan to `status: Implemented`, regenerate the decision index and lock, and commit those lifecycle-only changes together.
