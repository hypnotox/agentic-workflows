---
format: plan-v2
date: 2026-08-16
adrs:
  - 0283
status: Proposed
---
# Plan: Migrate Pi Runtime and Extension Recorder

## Goal

Align awf's retained Pi integration and strict extension-test lane on Pi 0.84.2, directly adopt the pi-tools v0.3.0 source-only recorder for generic Pi test seams, and keep runtime behavior and awf-specific policy unchanged; do not test or pin the adopter pi-tools runtime.

## Architecture summary

Use one checksummed Pi 0.84.2 SDK graph: pi-ai and pi-tui 0.84.2, the maintained `fork-v0.84.2.2` coding-agent asset, the existing Node v24.19.0 runtime, and test-only pi-tools v0.3.0. Replace generic hand-built Pi API, event, context, UI, model-registry, command, tool, active-tool, and execution recordings with direct `createExtensionRecorder` composition. Keep filesystem, Git, preference, routing, effort, UUID, and transport fixtures local to awf, create no recorder copy or adapter, and retain the real Pi SDK smoke rather than treating the recorder as a runtime. Update authored runtime and testing authority, render generated outputs, repair the six-profile current-state statement, and apply ADR-0283's claim batch only when the complete migration is green.

## Phase 1: Align and migrate the complete Pi test boundary

**Execution mode: subagent-driven.**

Completes: ["pi-runtime-aligned", "shared-recorder-adopted", "integration-semantics-preserved", "pi-authority-settled"]

### Task 1.1: Pin the aligned Pi and pi-tools test graph
Applying: ["0283:pi-0-84-2-floor", "0283:aligned-pi-verification", "0283:source-only-recorder-boundary", "0283:preserved-integration-semantics"]
Paths: ["tools/pi-extension-test/package.json", "tools/pi-extension-test/package-lock.json"]

Begin the phase only from a clean checkout after ADR-0283 is review-settled and Accepted. Treat the dependency graph, recorder consumers, runtime declaration, generated outputs, and current-state application as one green transaction because the source-only recorder is compiled against the pinned Pi graph and none of those surfaces can independently claim the final boundary.

Set the direct Pi development dependencies to pi-ai 0.84.2, pi-tui 0.84.2, and the `https://github.com/hypnotox/pi/releases/download/fork-v0.84.2.2/pi-coding-agent-fork-v0.84.2.2.tgz` coding-agent asset. Set the test-only pi-tools source dependency to the tagged v0.3.0 archive. Regenerate the lock through npm without running dependency scripts. Inspect the complete changed lock graph and require the coding-agent asset, its npm integrity, pi-tools v0.3.0 archive and integrity, embedded coding-agent package version 0.84.2, compatible peer resolution, and unchanged Node v24.19.0 project pin. Do not add a runtime pi-tools import or adopter package pin.

### Task 1.2: Replace supported generic Pi fixtures with the recorder
Kind: batch
Applying: ["0283:source-only-recorder-boundary", "0283:preserved-integration-semantics"]
Paths: ["tools/pi-extension-test/tests/index.test.ts", "tools/pi-extension-test/tests/profile-adapter.test.ts", "tools/pi-extension-test/tests/using-effort.test.ts"]
Representative: ["Compose factory-time profile negotiation from the recorder's synchronous event bus, recorded hooks and commands, mutable model registry, recording UI, active tools, and injected exec behavior.", "Use recorder omissions or typed additions when an effort test deliberately proves a missing API or the package-exported file-mutation queue boundary."]
Edge: ["Retain local file maps, scripted filesystem faults, Git checkout behavior, model-preference policy, effort replies, UUIDs, memory transport, and Remote Pi scenarios rather than moving them into a generic recorder wrapper.", "Use direct raw invocation semantics deliberately: do not claim Pi listener-error handling, command suffix routing, middleware, schema validation, scheduling, or error envelopes.", "Leave tools/pi-extension-test/tests/runtime.test.ts on the real SDK and leave the small pure-routing registry fixture in model-routing.test.ts outside this conversion."]
Post-check: With the three exact test files as the input population and runtime.test.ts plus model-routing.test.ts as explicit exclusions, inspect every remaining Pi-shaped fixture member and confirm it is awf-specific or deliberately models an omitted/additional capability; then `./x pi-test run` exits zero with strict TypeScript and all configured statement, line, function, and branch coverage thresholds satisfied.

Import `createExtensionRecorder` and its first-class fixtures directly from `pi-tools/testing`. Install synchronous or asynchronous factories through the recorder and await their installation before inspecting registrations or invoking handlers. Preserve every existing observable assertion for negotiation order and correlation, terminal reporting, schemas, profile callbacks, routing, Git verification, wizard interaction, effort association, memory tools, capability degradation, and default factory composition. A suite-local harness may compose recorder state with awf policy fixtures, but it must not reimplement generic recorder methods, event storage, context construction, UI recording, model-registry queries, command lookup, tool lookup, or active-tool mutation.

### Task 1.3: Publish the 0.84.2 floor and source-only testing boundary
Kind: batch
Applying: ["0283:pi-0-84-2-floor", "0283:aligned-pi-verification", "0283:source-only-recorder-boundary", "0283:preserved-integration-semantics"]
Paths: ["templates/partials/pi-minimum-runtime.md", "internal/project/target_test.go", "README.md", ".awf/docs/parts/development/dependencies.md", ".awf/docs/parts/testing/gate.md", ".awf/docs/parts/testing/tiers.md", ".awf/docs/parts/testing/layout.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", ".pi/extensions/awf-effort/index.ts", "docs/development.md", "docs/testing.md", "docs/topics/rendering/pi-runtime.md", "docs/domains/rendering.md", ".awf/awf.lock"]
Representative: ["Change the retained effort extension's numeric minimum and actionable compatibility text from 0.81.1 to 0.84.2 while preserving its capability checks.", "Describe pi-tools/testing v0.3.0 as pinned source-only test support while keeping the adopter pi-tools runtime independently installed and protocol-v2 compatible."]
Edge: ["Update the README from fork-v0.81.1-awf.3 to fork-v0.84.2.2 without implying that the fork or pi-tools test pin is an adopter runtime dependency.", "Correct the integration-boundary claim from four to the current six atomic profiles and preserve all profile, routing, effort, protocol, Node, output-plan, and real-SDK-smoke semantics.", "Do not hand-edit rendered docs, generated extensions, topic/domain outputs, the decision index, or lock outputs."]
Post-check: After `./x render`, read the root effort entrypoint, dependency and testing documents, Pi runtime topic and rendering domain, README prerequisite text, and changed lock entries; each must consistently distinguish the 0.84.2 effort floor, six-profile protocol boundary, source-only recorder pin, and unpinned adopter runtime. Record those inspected boundaries and whether any contradictory fragment or meaning drift was found in Notes as semantic-rendering evidence. `./x check` exits zero, and a repository search across README.md, templates/partials, internal/project, tools/pi-extension-test, .awf/docs/parts, and .awf/topics/parts finds no live 0.81.1 or fork-v0.81.1-awf.3 claim.

Update authored sources first and render their outputs. The strict lane must state that the recorder replaces generic local test seams but does not behavior-test external pi-tools scheduling, child execution, confinement, or presentation. Preserve the real SDK smoke as the proof of generated extension loading, profile negotiation, model-routing delivery, native skill discovery, and effort registration on the pinned fork.

### Task 1.4: Apply ADR-0283's runtime-boundary claim batch
Kind: batch
Latitude: exact
Applying: ["0283:pi-0-84-2-floor", "0283:aligned-pi-verification", "0283:source-only-recorder-boundary", "0283:preserved-integration-semantics"]
Paths: ["docs/decisions/0283-advance-pi-runtime-floor-to-0-84-2.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", "docs/topics/rendering/pi-runtime.md", "docs/domains/rendering.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]
Representative: ["Update pi-minimum-runtime to the landed 0.84.2 effort floor and aligned proof graph.", "Update pi-real-runtime-smoke and pi-tools-integration-boundary to distinguish source-only recorder assurance from the external runtime and to name six atomic profiles."]
Edge: ["Retain each claim's Origin and append ADR-0283 in Revised-by order with its existing deterministic Backing.", "The ADR remains Implementing after the complete Applied batch; terminal status waits for implementation assurance."]
Post-check: After rendering, `./awf check repo state` exits zero and `./awf context --show pending docs/decisions/0283-advance-pi-runtime-floor-to-0-84-2.md` reports update `rendering/pi-runtime:pi-minimum-runtime`, update `rendering/pi-runtime:pi-real-runtime-smoke`, and update `rendering/pi-runtime:pi-tools-integration-boundary` as Applied with no Remaining or Canceled operation.

Under the ADR lifecycle handshake, transition the reviewed ADR from Accepted to Implementing and apply all three declared updates in the same transaction as their matching current-state mutations. The resulting topic must describe the landed runtime floor, the direct source-only recorder boundary, the real SDK proof, and the unchanged protocol and effort ownership without attributing external runtime mechanics to awf.

### Phase close

Inspect the dependency lock, direct recorder compositions, generated effort output, testing documentation, and complete Pi runtime topic. Run `./x pi-test run`, `env AWF_PI_RUNTIME_SMOKE=1 go test -json ./internal/project -run '^TestPi(EffortMemoryToolContract|RealRuntimeSmoke)$' -count=1`, `./x check`, `./awf check staged`, and the full project gate before closing the ADR application transaction.

```commit
feat(rendering): align Pi runtime and recorder (applies 0283 batch)
```

## Definition of done

- `dod: pi-runtime-aligned` The checked Pi graph resolves pi-ai and pi-tui 0.84.2, the checksummed fork-v0.84.2.2 coding-agent asset with embedded version 0.84.2, pi-tools v0.3.0, and unchanged Node v24.19.0; generated effort compatibility text requires Pi 0.84.2.
- `dod: shared-recorder-adopted` The three converted suites directly consume `pi-tools/testing` for supported generic Pi recordings, retain only awf-specific or deliberate omission/addition fixtures locally, and keep real runtime verification outside the recorder.
- `dod: integration-semantics-preserved` Strict tests and the pinned real SDK smoke prove unchanged six-profile negotiation, routing, Git policy, effort association, memory tools, and capability degradation with full configured extension coverage.
- `dod: pi-authority-settled` All ADR-0283 operations are Applied while the ADR remains Implementing, authored and generated dependency/testing/runtime authority describe one coherent boundary, `./x check` is clean, and the project gate passes.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, findings, and follow-ups surfaced during implementation.

After implementation assurance settles, the effort-free execution parent owns the terminal artifact transaction: reconcile final deviations and review settlement here, append only ADR-0283's Implemented status event, change this plan to `status: Implemented`, regenerate the decision index and lock, and commit those lifecycle-only changes together.

Phase 1 implementation evidence: semantic rendering review inspected `.pi/extensions/awf-effort/index.ts`, `README.md`, `docs/development.md`, `docs/testing.md`, `docs/topics/rendering/pi-runtime.md`, and `docs/domains/rendering.md`. The 0.84.2 effort floor, six-profile protocol boundary, source-only pi-tools/testing v0.3.0 recorder, independently unpinned adopter runtime, and real-SDK smoke remain consistent; no contradictory fragment or meaning drift was found. Deviation: `tools/pi-extension-test/tests/runtime.test.ts` was updated from 0.81.1 to 0.84.2 only to remove the obsolete live local fixture claim required by Task 1.3; ADR-0283 requires one aligned proof graph, and `./x pi-test run` verified the update.

Post-review settlement: each main recorder harness installs its extension factory through `recorder.install` and awaits `recorder.ready` before returning state for inspection or invocation. Each harness has a direct assertion that its recorder has exactly one installation. A mutation demonstration replacing each harness installation with its former direct factory invocation made its corresponding assertion fail with `0 !== 1`; the mutations were restored. The synchronous factory-time negotiation assertion remains in place. The existing `runtime.test.ts` deviation remains unchanged.
