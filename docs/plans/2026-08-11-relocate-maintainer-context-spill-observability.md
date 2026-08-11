---
format: plan-v2
date: 2026-08-11
adrs: [keep-maintainer-context-spill-state-outside-configuration]
status: Proposed
---
# Plan: Relocate Maintainer Context Spill Observability

## Goal

Make the repository's context-spill advisory reachable after a spill by moving its maintainer-owned log outside `.awf`, while preserving secure per-checkout logging and leaving public config-tree and resident-root semantics unchanged.

## Architecture summary

Keep `internal/contextspill` as the single owner of notice recognition and secure logging. Replace its `.awf/local` storage walk with an owned shared-cache parent and a dedicated mode-0700 `awf-context` descendant, retaining descriptor-relative no-follow access, a mode-0600 locked log, path-free records, and warning-only wrapper degradation. Update the private command, runner-facing guidance, current-state claim, and regression spine together. Do not add a resident root, claim a `.awf/local` path, migrate an obsolete log, or change `awf context` spill delivery.

## Phase 1: Relocate and apply context-spill observability

**Execution mode: inline.**

Completes: ["spill-advisory-reachable", "spill-storage-secure", "spill-authority-current"]

### Task 1.1: Pin the outside-configuration behavior before production changes
Latitude: exact
Applying: ["keep-maintainer-context-spill-state-outside-configuration:separate-maintainer-state", "keep-maintainer-context-spill-state-outside-configuration:preserve-advisory-security"]
Paths: ["internal/contextspill/log_test.go", "internal/contextspill/log_fault_test.go", "cmd/contextspilllog/main_test.go", "internal/project/context_wrapper_test.go", "internal/project/sweep_test.go"]

Change the focused tests first and run them against the unchanged implementation to observe failure. Replace expected storage with `.cache/awf-context/context-spills.log`; prove the dedicated descendant is mode 0700, the log is mode 0600, shared-cache and dedicated-directory symlinks or foreign ownership refuse, records remain path-free and serialized, and absence of either cache component is a safe empty state. Preserve the existing fault matrix and descriptor-substitution coverage while adding the creation-failure residue case: if the dedicated directory is created but the log open or append fails, real config-tree drift still reports no new finding because no state entered `.awf`.

In the wrapper contract, keep byte/status preservation, warning-only logging failure, concurrency, and advisory behavior. Add composed evidence using the real project drift path rather than the existing successful `./awf check` stub: a safe nonempty cache log and an empty dedicated cache directory both leave base drift clean, after which the wrapper-focused test proves the nonempty advisory; `.awf/local` remains unclaimed in the closed-tree sweep. Run `go test ./internal/contextspill ./cmd/contextspilllog ./internal/project`; the relocated-path expectations must fail before Task 1.2 changes production code, and retain the failure reason as transient phase evidence.

### Task 1.2: Move the secure helper to the dedicated checkout cache
Latitude: exact
Applying: ["keep-maintainer-context-spill-state-outside-configuration:separate-maintainer-state", "keep-maintainer-context-spill-state-outside-configuration:preserve-advisory-security"]
Paths: ["internal/contextspill/log.go", "cmd/contextspilllog/main.go", "x", ".gitignore"]

Make `Log` and `HasSafeLog` traverse from the verified repository root through an owned `.cache` directory into a dedicated `awf-context` directory. Create a missing shared cache parent without requiring its mode to be private, because other maintainer tooling already shares it; create and require the dedicated descendant at mode 0700. Open every component descriptor-relatively with no-follow semantics, retain current ownership checks, keep the log mode 0600, and preserve locking, complete append, fsync, cleanup, and first-error behavior. Missing cache components remain a safe empty advisory state. Do not inspect or migrate `.awf/local`, and do not change spill notice parsing or the external temporary delivery file.

Update the private command's operator-removal diagnostic to name the new log. Keep `x` ordering unchanged: ordinary `./awf check` completes first, then safe-log inspection emits the non-failing advisory. Remove the obsolete `.awf/local/` ignore entry and broaden the existing `.cache/` comment to cover maintainer caches without changing the ignored boundary. Run `gofmt -w internal/contextspill/log.go internal/contextspill/log_test.go internal/contextspill/log_fault_test.go cmd/contextspilllog/main.go cmd/contextspilllog/main_test.go internal/project/context_wrapper_test.go internal/project/sweep_test.go`, then run `go test ./internal/contextspill ./cmd/contextspilllog ./internal/project`; every Task 1.1 regression must pass.

### Task 1.3: Apply the claim and publish the corrected maintainer guidance
Latitude: exact
Applying: ["keep-maintainer-context-spill-state-outside-configuration:separate-maintainer-state", "keep-maintainer-context-spill-state-outside-configuration:preserve-advisory-security"]
Paths: ["docs/decisions/keep-maintainer-context-spill-state-outside-configuration.md", "docs/decisions/INDEX.md", ".awf/topics/parts/tooling/context-and-topic/current-state.md", "docs/topics/tooling/context-and-topic.md", "README.md", "changelog/CHANGELOG.md", ".awf/awf.lock"]

Use `awf-adr-lifecycle` for the explicit application transaction. Change the pending ADR to `Implementing`, append its `Implementing; content-sha256:` event followed by one Applied event for exactly `update tooling/context-and-topic:context-spill-observability`, and leave the final `Implemented` event deferred until assurance settles. Obtain the content digest mechanically from the staged checker diagnostic rather than guessing it.

Update the claim to describe the ignored dedicated checkout-cache log outside `.awf`, preserve its path-free, owner-only, descriptor-relative no-follow, locked append, warning-only, advisory, and operator-removal semantics, and append `ADR-keep-maintainer-context-spill-state-outside-configuration` to its existing `Revised-by` provenance. Keep its existing invariant identity and proof marker. Update README with the same concise operator-visible path and add an Unreleased bug-fix entry explaining that a spill observation no longer causes closed config-tree drift before the advisory. Do not edit historical ADR-0165, its historical implementation plan, or released changelog entries.

Run `./x render` so the topic output, ADR index, and lock travel with their sources. Read back the ADR lifecycle, source claim, rendered topic, README paragraph, changelog entry, and lock diff. Confirm the new path and outside-configuration meaning agree, the old path remains only in frozen history or tests that deliberately prove it unclaimed, no prose implies a new resident root or public config exception, and no obsolete path appears in live production, current-state, or operator guidance.

### Phase close

Stage the complete application transaction explicitly. Run `awf check staged` and `./x gate`; both must pass, with the gate retaining 100% statement coverage. Create the one closing commit:

```commit
fix(tooling): move context spill log outside config (applies ADR batch)
```

## Definition of done

- `dod: spill-advisory-reachable` A successful spill observation leaves ordinary project drift clean and the next `./x check` emits the intended non-failing advisory until the operator removes the cache log.
- `dod: spill-storage-secure` The relocated log retains descriptor-relative no-follow traversal, ownership and mode enforcement, locked durable path-free appends, concurrency safety, and warning-only degradation across the focused fault tests.
- `dod: spill-authority-current` The pending ADR is Implementing with its sole operation Applied, the active claim and reader guidance name checkout-cache storage outside `.awf`, generated outputs are synchronized, and historical records remain unchanged.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation.
