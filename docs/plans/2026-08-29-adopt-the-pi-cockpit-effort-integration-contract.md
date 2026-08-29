---
format: plan-v2
date: 2026-08-29
adrs: [adopt-the-pi-cockpit-effort-integration-contract]
status: Proposed
---
# Plan: Adopt the Pi Cockpit effort integration contract

## Goal

Replace awf's active generated Pi effort-integration identity with the Pi Cockpit event namespace and structural type terminology, preserve every non-brand behavior established by ADR-0231, and prepare an authorized awf v0.42.0 release that the blocked Pi Cockpit adopter can consume. Do not add compatibility aliases, change payloads or lifecycle semantics, or govern the adopter repository's own implementation and release policy.

## Architecture summary

The fixed Pi target remains the sole owner of the dependency-free effort integration. One atomic rendering transaction changes its template, dogfood output, structural publisher assertions, runtime behavior tests, current-state provenance, and adopter-facing changelog from `RemotePi*` and `remote-pi:*` to `PiCockpit*` and `pi-cockpit:*`; no parallel legacy path remains. Focused tests prove that only identity changes while metadata replacement, capability validation, suffix publication and replay, explicit clearing, advisory status, and graceful degradation retain their existing behavior.

After Phase 1 review and terminal exhaustive verification, `effort-workflow` integrates the implementation into the governed primary checkout. Phase 2 then prepares v0.42.0 on clean `main` under the release runbook, after which deferred ADR and plan closure, a final release-range audit, exact-revision push and CI, tag publication, release verification, and the separately governed downstream upgrade occur in that order. The downstream transaction remains owned by the adopter's existing `rename-pi-cockpit` effort rather than by this plan.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. The parent may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds, and may batch independent cheap operations only when they safely share context. Batching never transfers transaction authority or weakens path confinement. An implementation child remains commit-disabled and confined to its assigned paths. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Cut over the generated effort integration

**Execution mode: inline.**

Completes: ["pi-cockpit-contract", "preserved-effort-semantics", "current-contract-authority"]

### Task 1.1: Rename the template contract and its durable behavior oracles
Kind: batch
Applying: ["adopt-the-pi-cockpit-effort-integration-contract:pi-cockpit-event-identity", "adopt-the-pi-cockpit-effort-integration-contract:hard-contract-cutover"]
Paths: ["templates/pi/awf-effort/index.ts.tmpl", ".pi/extensions/awf-effort/index.ts", "tools/pi-extension-test/tests/using-effort.test.ts", "internal/publisher/target_test.go"]
Representative: "The generated extension exports `PiCockpitMetadata`, emits `pi-cockpit:metadata:set`, requests and consumes `pi-cockpit:capabilities`, and publishes or replays `pi-cockpit:display-suffix:*` without importing the provider package."
Edge: "Factory-time capability replies, unsupported or malformed capability snapshots, detach, restart, ownership loss, shutdown, and replay continue to publish or clear the same payloads at the same lifecycle boundaries; no `RemotePi` type or `remote-pi:` listener, emitter, or test helper remains on the active generated surface."
Post-check: "`go test ./internal/publisher` and `./x pi-test run` exit zero. A checked `rg -n 'RemotePi|remote-pi:' templates/pi/awf-effort/index.ts.tmpl .pi/extensions/awf-effort/index.ts tools/pi-extension-test/tests/using-effort.test.ts internal/publisher/target_test.go` reaches the expected terminal set of no findings, while the focused assertions still exercise metadata payload equality, exact capability validation, suffix replay, explicit clearing, and missing-event-bus degradation."

Update the focused Go and TypeScript expectations to the new identity before changing the template, and observe them fail for the old rendered contract. Rename only awf-owned structural types and event literals, then render the dogfood output from its template. Retain every payload shape, state transition, optional event-bus guard, capability rule, and error-isolation boundary byte-for-byte apart from the settled identity tokens. Do not add aliases, dual publication, fallback listeners, configuration, provider imports, or package-version coupling.

### Task 1.2: Apply current-state authority and publish the breaking change
Kind: batch
Applying: ["adopt-the-pi-cockpit-effort-integration-contract:pi-cockpit-event-identity", "adopt-the-pi-cockpit-effort-integration-contract:hard-contract-cutover"]
Paths: [".awf/topics/parts/rendering/pi-workflows/current-state.md", ".awf/topics/parts/rendering/pi-runtime/current-state.md", "changelog/CHANGELOG.md", "docs/decisions/adopt-the-pi-cockpit-effort-integration-contract.md", "docs/decisions/INDEX.md", "glob:docs/topics/rendering/**", "glob:docs/domains/**", ".awf/awf.lock"]
Representative: "The three declared claims describe Pi Cockpit translation and advisory events while preserving the former authorization, lifecycle, routing, payload, capability, replay, clearing, runtime-floor, and failure guarantees; the Unreleased Breaking changes section tells adopters to upgrade awf and Pi Cockpit together."
Edge: "Historical ADRs, implemented plans, and released changelog entries retain their original Remote Pi wording. The pending ADR records all three State changes as Applied in this transaction and remains Implementing until terminal assurance settles."
Post-check: "At the post-render Phase 1 snapshot, `./x check` and `git diff --check` exit zero and `./awf context docs/decisions/adopt-the-pi-cockpit-effort-integration-contract.md` reports each declared State change Applied with none remaining. Run a checked `rg -n 'RemotePi|remote-pi:'` over exactly `templates/pi/awf-effort/index.ts.tmpl`, `.pi/extensions/awf-effort/index.ts`, `tools/pi-extension-test/tests/using-effort.test.ts`, `internal/publisher/target_test.go`, `.awf/topics/parts/rendering/pi-workflows/current-state.md`, and `.awf/topics/parts/rendering/pi-runtime/current-state.md`; require ripgrep's no-match status, fail on any other status, then print a success sentinel. Apply the same checked no-match procedure to only the Unreleased section extracted from `changelog/CHANGELOG.md`. Frozen ADRs, implemented plans, and released changelog sections are explicitly excluded historical records; the terminal active set is empty."

Move the reviewed ADR to Implementing and apply its three declared claim updates in the same authored transaction. Append this ADR to each claim's provenance without disturbing prior Origins or Revised-by history. Add one concise adopter-facing breaking-change entry under Unreleased. Render generated topics, domains, decision index, dogfood extension, and permanent lock from their authorities rather than editing generated prose directly.

### Phase close

Stage the template, tests, authored authority, ADR application, changelog, generated dogfood and documentation, decision index, and lock explicitly. The focused evidence above, staged check, fast gate, and terminal `GOTOOLCHAIN=go1.26.0 ./x gate full` must all exit zero before integration. Create one commit:

```commit
feat(rendering): adopt Pi Cockpit effort contract (applies ADR batch)
```

## Phase 2: Prepare the authorized awf release

**Execution mode: inline.**

Completes: ["release-ready-main"]

### Task 2.1: Promote the integrated contract as v0.42.0
Kind: batch
Paths: ["internal/project/VERSION", "changelog/CHANGELOG.md", ".awf/awf.lock"]
Representative: "`internal/project/VERSION` contains newline-terminated `0.42.0`; a new empty Unreleased section precedes the dated 0.42.0 section containing the complete accumulated adopter-facing changes."
Edge: "Release preparation changes only canonical version, changelog promotion, and the rendered permanent lock. It neither edits the already-assured contract nor publishes before exact-revision CI succeeds."
Post-check: "On clean primary `main`, `./x render`, `go run ./cmd/releasecheck`, `./x check`, and `git diff --check` exit zero; the release checker confirms version and newest changelog agreement with an empty Unreleased section."

Begin only after Phase 1 is independently assured, terminal-full green, and integrated into the governed primary checkout. Follow `docs/releasing.md`: before mutation audit the candidate range with `./awf audit v0.41.0..HEAD`, set the canonical version to 0.42.0, promote the complete Unreleased changelog to a dated 0.42.0 section, recreate the empty standing Unreleased section, and render the lock. Do not absorb unrelated work from another managed effort or weaken the exact-revision release controls.

### Phase close

Stage exactly the canonical version, changelog, and lock release-preparation transaction and rely on the wired hook's canonical test-free local release lane. Create one commit:

```commit
chore(awf): bump version to v0.42.0
```

## Definition of done

- `dod: pi-cockpit-contract` Every active generated effort-integration event uses `pi-cockpit:*`, every awf-owned provider-facing structural type uses `PiCockpit*`, and no active compatibility alias or old listener remains.
- `dod: preserved-effort-semantics` Focused publisher and Pi runtime tests prove unchanged metadata payloads, capability validation, suffix publication and replay, explicit clearing, advisory behavior, and graceful degradation across the renamed contract.
- `dod: current-contract-authority` All three ADR State changes are Applied; authored and rendered current-state claims and the Unreleased changelog accurately describe the hard cutover while frozen history remains intact.
- `dod: release-ready-main` Clean primary `main` carries the integrated, terminally verified contract plus canonical v0.42.0 version, promoted changelog, and matching lock, and releasecheck is clean; the final exact-HEAD release-range audit remains the pre-push continuation gate.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution.

After Phase 2 assurance, `effort-workflow` performs the deferred terminal reconciliation and closes this plan and ADR on primary `main`. Re-run `./awf audit v0.41.0..HEAD`, push that exact clean `main`, and wait for both required `CI / gate` and `CI / release-config` conclusions to succeed on the pushed SHA. Only then create and push tag `v0.42.0` exactly as prescribed by the release runbook; verify the Release workflow succeeds and the GitHub release exposes the expected archives, checksums, and curated 0.42.0 notes before treating the release as consumable.

Once v0.42.0 is verified, resume the separate `rename-pi-cockpit` effort in `/home/hypno/Projects/remote_pi` without modifying its preserved staged Phase 3 transaction by hand. Use that adopter's governed `.awf/upgrade.sh` to upgrade from the published release, then run its recorded residual identity sweep, focused extension checks, full repository gate, implementation assurance, integration, deployment or installation review, retrospective, and effort finish under the adopter repository's own plan and lifecycle authority.

- 2026-08-29 plan-review settlement: bounded the old-identity census to named active surfaces and the Unreleased changelog snapshot, with checked no-match status and an explicit historical-record exclusion. This makes the hard-cutover proof reproducible without rewriting frozen history.
- 2026-08-29 plan-review settlement: removed the unsupported signed-tag commitment and retained the repository release runbook as the sole tag-procedure authority. User authorization covers creating the release, not adding a new signing policy.
