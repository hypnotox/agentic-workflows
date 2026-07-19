---
date: 2026-07-19
adrs: [133, 134, 135, 136]
status: Proposed
---
# Plan: Project Cutover and Current-State Release

## Goal

Curate and atomically migrate awf and Sundial to current-state topics, consume both bridge
attestations with the Plan 3 runtime, close all four plans and authority ADRs, commit the permanent
cutover, and tag the breaking `v0.19.0` release. No automatic prose extraction, compatibility mode,
or post-release legacy authority remains.

## Architecture summary

Prepare all digest-covered content and runtime source at one clean Git commit while using a pinned
0.18.0 bridge binary for readiness/sync/check. Produce root and Sundial attestations in separate clean
worktrees at that same HEAD, combine their disjoint patches without moving HEAD, then use the 0.19.0
current runtime to consume both seals, generate INDEX/topic outputs, and write permanent locks. The
final commit is followed by a separately gated annotated release boundary through tag `v0.19.0`.

All paths are rooted at `/home/hypno/Projects/agentic-workflows`. The exact topic set below is the
minimum prepared corpus; claim prose must state current behavior concretely and the bridge inventory
must prove every surviving legacy invariant maps once with its backing class unchanged.

## File structure

- **Created:** root `.awf/topics/{metadata,parts}/**` for the listed topics; Sundial topic inputs for
  `almanac/model` and `cli/interface`; rendered `docs/topics/**` and both
  `docs/decisions/INDEX.md`; Plan 3 runtime files; temporary binaries/patches only under
  `/tmp/awf-current-state-close`.
- **Modified:** root and Sundial configs/domain ownership/markers/ADRs/locks; Plan 3 runtime and tests;
  every authored template/workflow/agent/ADR/domain/architecture/config-reference/glossary/pitfalls/
  release source named by Plan 3; README/changelog; Plans 1-4 statuses/Notes.
- **Deleted:** both ACTIVE.md files, domain ADR indexes, legacy authority production code, bridge-only
  package, obsolete invariant markers/config, and stale rendered outputs selected by final output plans.

## Phase 1: Curate both projects under the pinned bridge

- [ ] **Task 1.1: Pin the bridge artifact outside all worktrees.** At the final Plan 2 HEAD run:

  ```sh
  set -euo pipefail
  repo=/home/hypno/Projects/agentic-workflows
  art=/tmp/awf-current-state-close
  rm -rf "$art" && mkdir -p "$art"
  cd "$repo"
  bridge_head="$(git rev-parse HEAD)"
  printf '%s\n' "$bridge_head" > "$art/bridge-head"
  go build -trimpath -o "$art/awf-bridge" ./cmd/awf
  sha256sum "$art/awf-bridge" | tee "$art/awf-bridge.sha256"
  "$art/awf-bridge" version
  ```

  Expected: version 0.18.0 and one SHA-256 line; the artifact remains outside every worktree.

- [ ] **Task 1.2: Extend domain ownership and convert root config.** Add
  `internal/currentstate/**` and `internal/topic/**` to invariants ownership; add
  `internal/snapshot/**`, `internal/upgrade/**`, `internal/prosegate/**`, and `tools/**` to tooling.
  Replace root `invariants` with `currentState` preserving Go and authored-Markdown source entries,
  `testGlobs: ['**/*_test.go']`, coverage error, fanout warn, maximum 8. Apply the same mechanical
  conversion to Sundial and add testGlobs. No disabled or unqualified legacy marker remains.

- [ ] **Task 1.3: Author the exact root topic set.** Create paired metadata/current-state inputs:
  `adr-system/{adr-lifecycle,plan-artifacts,frontmatter}`;
  `config/{configuration,migrations-and-locks}`;
  `invariants/{current-state-authority,topics-and-markers}`;
  `rendering/{render-engine,catalog-and-targets,project-output-plan,templates}`;
  `tooling/{cli,audit-and-snapshots,quality-gates,changelog-and-release,evaluations,upgrade-runtime}`.
  Use the path sets recorded in Plan 4 exploration and keep every path-scoped topic within its owner.
  Claims consolidate current contracts rather than mirroring ADR count. Run bridge inventory readiness
  after each domain until every surviving invariant maps exactly once and coverage reaches zero.

- [ ] **Task 1.4: Author Sundial topics and truthful proof.** Add `almanac/model` for
  `internal/almanac/**` with rules for cosine model, longitude shift, polar collapse, accuracy ceiling,
  standard-library-only implementation, and test-backed invariant `almanac-clamped-latitude`, Origin
  ADR-0001. Add `cli/interface` for `cmd/**` with exactly two decimal-degree positionals, usage exit 2,
  no alternate coordinate formats, table output, and no model logic, Origin ADR-0002. Remove the proof
  marker from production and add
  `// invariant: almanac/model:almanac-clamped-latitude` to `TestClampLatitude`.

- [ ] **Task 1.5: Resolve every in-flight ADR before attestation.** Set root ADR-0001 and ADR-0133,
  ADR-0134, ADR-0135, ADR-0136 to Implemented after their resulting claims/runtime are present. Set
  Sundial ADR-0003 to Abandoned with rationale that the speculative cache/schedule seam was never
  implemented. Let bridge normalization change every Superseded legacy ADR to Implemented and append
  required Migration history; do not fabricate State changes on legacy ADRs.

- [ ] **Task 1.6: Apply all sealed Plan 3 and documentation changes.** Apply Plan 3 Phases 2-5 runtime,
  tests, deletion, templates, skills, agents, AGENTS parts, workflow/working-with-awf, architecture,
  all domain parts, config-reference sources, glossary, pitfalls, README, releasing source, and
  changelog before attestation. Set `project.Version` to 0.19.0 but keep the Plan 2 release sentinel
  behavior usable through the pinned bridge binary. Do not change any configured marker-source or
  other sealed path afterward.

- [ ] **Task 1.7: Create the clean preparation commit.** In a detached preparation worktree at the
  bridge HEAD, apply the complete runtime/adopter patch, then run only the pinned bridge for project
  commands:

  ```sh
  "$art/awf-bridge" upgrade --check
  (cd examples/sundial && "$art/awf-bridge" upgrade --check)
  "$art/awf-bridge" sync
  (cd examples/sundial && "$art/awf-bridge" sync)
  "$art/awf-bridge" check
  (cd examples/sundial && "$art/awf-bridge" check)
  go test ./...
  (cd examples/sundial && go test ./...)
  ./x gate
  git diff --check
  ```

  Expected: both readiness reports have zero findings; checks/tests/gate are clean. Stage the exact
  authored/generated output-plan set and commit:

  ```commit
  feat(awf): prepare current-state authority cutover
  ```

  Record `prep_head`, require clean status, and do not amend this commit.

## Phase 2: Produce and combine two clean attestations

- [ ] **Task 2.1: Attest root in its own worktree.** Create `/tmp/awf-cs-root` detached at
  `prep_head`; run bridge `upgrade --check`, `--attest-current-state`, then `--check`. Export a binary
  full-index patch excluding `examples/sundial/**` and a sorted path list.

- [ ] **Task 2.2: Attest Sundial independently.** Create `/tmp/awf-cs-sundial` detached at the same
  HEAD; from `examples/sundial` run the same three bridge commands. Export a patch limited to
  `examples/sundial/**` and its sorted path list. Assert `comm -12` over the two lists is empty.

- [ ] **Task 2.3: Prove patches mutate no sealed authored input.** Permit only journaled normalized
  ADR/config/marker bytes already predicted by readiness, generated-output deletions/replacements, and
  lock changes. Fail if either patch changes topic parts/metadata, domain metadata/parts, unplanned
  source markers, or any other digest input. Both attestations must name exactly `prep_head`.

- [ ] **Task 2.4: Combine without moving HEAD.** Create `/tmp/awf-cs-integration` detached at
  `prep_head`; `git apply --check` then `git apply --binary` root followed by Sundial patches. Assert
  `git rev-parse HEAD` still equals `prep_head`. Do not commit the attestation patches.

## Phase 3: Consume both seals and close the repository

- [ ] **Task 3.1: Build and run the current runtime.** In integration, build
  `/tmp/awf-current-state-close/awf-current`; run plain `upgrade` at root and Sundial. Each verifies
  attestation version 1, PreparedHead, exact digest, and permanent predicates; journals the final plan;
  replaces lock last; clears attestation; promotes cutoff/gaps.

- [ ] **Task 3.2: Assert permanent lock and output facts.** Root lock must have
  `adrFormatV1From: 137`, no gaps, no bridgeAttestation; Sundial lock cutoff 4, no gaps, no attestation.
  Both INDEX files exist with In flight and History; both ACTIVE files are absent; no domain decision
  index remains; topic docs/indexes and topic-only domain navigation match the output plan.

- [ ] **Task 3.3: Run the final runtime gate.** Run:

  ```sh
  ./x sync
  ./x check
  ./x gate
  git diff --check
  go test ./internal/currentstate -run '^TestLegacyAuthorityAbsent$' -count=1
  go tool deadcode -json ./... | go run ./cmd/deadcodecheck
  ```

  Expected: sync self-settles; check/gate are clean; legacy-absence package reports `ok`; deadcode
  reports no production dead code.

- [ ] **Task 3.4: Freeze plans and verify ADR history.** Set Plans 1-4 to Implemented and record actual
  Notes. Confirm ADR-0133-0136 and root ADR-0001 are Implemented, Sundial ADR-0003 Abandoned, and INDEX
  sections reflect them. Run final sync/check/gate again.

- [ ] **Task 3.5: Commit the permanent cutover.** Stage exactly the two attestation patches, final
  upgrade/output-plan changes, locks, plan statuses, and regenerated fan-out; commit:

  ```commit
  feat(awf): cut over to current-state authority
  ```

  Working tree must be clean after the commit.

## Phase 4: Tag and publish the breaking current-state release

- [ ] **Task 4.1: Pin release version and changelog.** Promote complete breaking notes from Unreleased
  to `[0.19.0] - <release-date>`, restore an empty Unreleased section, and ensure
  `project.Version == "0.19.0"` plus both locks use 0.19.0. The preparation/cutover commits must already
  contain these bytes; this task verifies rather than creates an extra release commit.

- [ ] **Task 4.2: Run release preflight.** From clean main after the cutover commit:

  ```sh
  ./x gate
  ./x check
  ./x audit v0.18.0..HEAD
  go run ./cmd/releasecheck
  go run github.com/goreleaser/goreleaser/v2@v2.17.0 check
  go run github.com/goreleaser/goreleaser/v2@v2.17.0 release --snapshot --clean
  ```

  Expected: gate/check clean; audit has no blocking conformance finding; releasecheck passes;
  GoReleaser config and snapshot succeed outside tracked outputs.

- [ ] **Task 4.3: Push main and tag `v0.19.0`.** Verify the cutover commit is on main and remote is
  correct, then run:

  ```sh
  git push origin main
  git tag -a v0.19.0 -m "v0.19.0"
  git push origin v0.19.0
  ```

  Expected: the release workflow verifies tag/version equality, main ancestry, changelog pin, gate,
  drift, and pinned GoReleaser before publishing six platform archives, checksums, and curated notes.
  If any preflight fails, do not create or push the tag.

## Verification

- Bridge readiness reports zero missing invariant mappings, markers, topic coverage, or in-flight ADRs.
- Both attestations share the preparation HEAD, have disjoint patches, and preserve sealed inputs.
- Permanent locks have exact cutoffs/gaps and no attestation; INDEX replaces ACTIVE everywhere.
- Context/invariants/check consume topic claims only; denylist/import/deadcode checks find no legacy
  engine.
- Plans 1-4 and ADR-0133-0136 close in the cutover commit.
- `v0.19.0` is tagged only after the clean, gated cutover commit and release preflight.

## Notes

- Rollback before final lock commit uses journal recovery. After successful final upgrade, rollback is
  Git restoration plus reinstalling the 0.18.0 bridge; no mixed-mode downgrade exists.
