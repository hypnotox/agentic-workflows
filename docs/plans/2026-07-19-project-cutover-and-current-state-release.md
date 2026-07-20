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
must prove every surviving legacy invariant maps once with its backing class unchanged and no exception path.

## File structure

- **Created:** root `.awf/topics/{metadata,parts}/**` for the listed topics; Sundial topic inputs for
  `almanac/model` and `cli/interface`; root and Sundial `.awf/current-state-migration.yaml` approval
  files during preparation; rendered `docs/topics/**` and both `docs/decisions/INDEX.md`; Plan 3
  runtime files; temporary binaries/patches only under `/tmp/awf-current-state-close`.
- **Modified:** root and Sundial configs/domain ownership/markers/ADRs/locks; Plan 3 runtime and tests;
  every authored template/workflow/agent/ADR/domain/architecture/config-reference/glossary/pitfalls/
  release source named by Plan 3; README/changelog; Plans 3-4 statuses/Notes. Plans 1-2 are verified
  Implemented prerequisites.
- **Deleted:** both `.awf/current-state-migration.yaml` files during final upgrade, both ACTIVE.md
  files, domain ADR indexes, legacy authority production code, bridge-only package, obsolete invariant
  markers/config, and stale rendered outputs selected by final output plans.

## Phase 0: Verify the published bridge prerequisite

- [ ] **Task 0.1: Publish or verify bridge release v0.18.0.** Final plan resync adds the bridge
  publication task to Plan 2: promote its changelog, run gate/check/audit/releasecheck and pinned
  GoReleaser snapshot, merge its clean release commit to main, create annotated `v0.18.0`, and verify
  the release workflow. Before this plan continues run:

  ```sh
  git fetch origin main --tags
  bridge_release="$(git rev-parse 'v0.18.0^{commit}')"
  prep_base="$(git rev-parse origin/main)"
  # v0.18.0 is the published bridge release and an ancestor of the preparation base: Plan 3 Phase 1
  # (the standalone snapshot seam) landed on main after it, so main advanced past the release.
  git merge-base --is-ancestor "$bridge_release" "$prep_base"
  # the bridge source is byte-identical between the release and the preparation base, so a bridge
  # binary built from prep_base is the published 0.18.0 bridge.
  test -z "$(git diff "$bridge_release" "$prep_base" -- internal/bridge)"
  test "$(git show "$prep_base:internal/project/project.go" | grep 'const Version =')" = 'const Version = "0.18.0"'
  ```

  Expected: no output. If v0.18.0 is absent, is not an ancestor of origin/main, the bridge source
  changed between the release and the preparation base, or the version constant is not 0.18.0, stop.

## Phase 1: Curate both projects under the pinned bridge

- [ ] **Task 1.1: Pin the bridge artifact outside all worktrees.** At the preparation base
  (`prep_base`, the post-Phase-1 main HEAD from Task 0.1) run:

  ```sh
  set -euo pipefail
  repo=/home/hypno/Projects/agentic-workflows
  art=/tmp/awf-current-state-close
  rm -rf "$art" && mkdir -p "$art"
  cd "$repo"
  test -z "$(git status --porcelain=v1)"
  # prep_base already contains Plan 3 Phase 1 and is still a 0.18.0-version source; the pinned
  # bridge built from it is the published 0.18.0 bridge (Task 0.1 verified internal/bridge unchanged).
  prep_base="$(git rev-parse origin/main)"
  test "$(git rev-parse HEAD)" = "$prep_base"
  printf '%s\n' "$prep_base" > "$art/prep-base"
  go build -trimpath -o "$art/awf-bridge" ./cmd/awf
  sha256sum "$art/awf-bridge" | tee "$art/awf-bridge.sha256"
  "$art/awf-bridge" version
  ```

  Expected: version 0.18.0 and one SHA-256 line; the artifact remains outside every worktree. Run
  `sha256sum -c "$art/awf-bridge.sha256"` before every later bridge invocation; any mismatch stops.

- [ ] **Task 1.2: Extend domain ownership and convert root config.** Add
  `internal/currentstate/**` and `internal/topic/**` to invariants ownership; add
  `internal/snapshot/**`, `internal/upgrade/**`, `internal/prosegate/**`, and `tools/**` to tooling.
  Replace root `invariants` with `currentState` preserving Go and authored-Markdown source entries,
  `testGlobs: ['**/*_test.go']`, coverage error, fanout warn, maximum 8. Apply the same mechanical
  conversion to Sundial and add testGlobs. No disabled or unqualified legacy marker remains.

- [ ] **Task 1.3: Author the exact root topic set and scopes.** Create paired metadata/current-state
  inputs with these exhaustive scopes: `adr-system/adr-lifecycle` -> `internal/adr/**`;
  `adr-system/plan-artifacts` -> `internal/plan/**`; `adr-system/frontmatter` ->
  `internal/frontmatter/**`; `config/configuration` -> `internal/config/**`,
  `internal/configspec/**`, `internal/pathglob/**`; `config/migrations-and-locks` ->
  `internal/migrate/**`, `internal/manifest/**`; `invariants/current-state-authority` ->
  `internal/currentstate/**`, `internal/invariants/**`; `invariants/topics-and-markers` ->
  `internal/topic/**`; `rendering/render-engine` -> `internal/render/**`, `internal/refs/**`;
  `rendering/catalog-and-targets` -> `internal/catalog/**`; `rendering/project-output-plan` ->
  `internal/project/**`; `rendering/templates` -> `templates/**`; `tooling/cli` -> `cmd/**`,
  `internal/clispec/**`, `internal/initspec/**`; `tooling/audit-and-snapshots` ->
  `internal/audit/**`, `internal/git/**`, `internal/snapshot/**`; `tooling/quality-gates` ->
  `internal/coverage/**`, `internal/prosegate/**`, `tools/**`, `x`;
  `tooling/changelog-and-release` -> `internal/changelog/**`; `tooling/evaluations` ->
  `internal/evals/**`; `tooling/upgrade-runtime` -> `internal/upgrade/**`.

  The user approved semantic claim curation during implementation rather than implementation-sized
  prose in this plan. Author root `.awf/current-state-migration.yaml` with exactly `version: 1` and
  `invariantApprovals`; use the exact empty sequence `invariantApprovals: []` if a project has zero live mappings. After the bridge independently derives each live inventory mapping, add exactly
  one entry containing only nonempty string scalar `key: ADR-NNNN#slug` and
  `destination: domain/topic:slug` for that mapping. Repository review and commit review establish
  approval provenance; do not add reviewer, timestamp, signature, or authored `approved` fields.
  Retired inventory entries receive no approval entries. Final resync adds `upgrade --check --json` to
  Plan 2 with schema `{ready,findings:[{code,path,detail}],invariantAdjudications:[{key,disposition,
  destination,origin,backing,approved}],plannedMutations:[{path,beforePresent,beforeMode,beforeSHA256,
  afterPresent,afterMode,afterSHA256}]}` and deterministic sorted arrays. Export
  `invariantAdjudications` as the exhaustive affected-site table; require every live key's computed
  `approved` to be true from an exact matching entry and every retired key's computed `approved` to be
  true only from valid encoded history evidence or valid migration rationale. Claims consolidate
  current contracts rather than mirroring ADR count. No inventory key may be missing, duplicated,
  retired-and-mapped, retired-and-approved, class-changed, or destination-mismatched. A genuine class change must instead carry valid reviewed migration retirement rationale and produce a distinct current-state claim that is not mapped as the same obligation. Incremental
  readiness may still report in-flight ADRs until Task 1.5; require zero findings and all adjudications
  approved only afterward.

- [ ] **Task 1.4: Author Sundial topics and truthful proof.** Add `almanac/model` for
  `internal/almanac/**` with Origin ADR-0001 rules `cosine-day-length-model`,
  `longitude-shifts-solar-noon`, `polar-results-collapse`, `minutes-level-accuracy`, and
  `standard-library-only`, each stating the named current contract, plus test-backed invariant
  `almanac-clamped-latitude`. Add `cli/interface` for `cmd/**` with Origin ADR-0002 rules
  `two-decimal-degree-positionals`, `usage-errors-exit-two`, and
  `no-alternate-coordinate-formats`, each stating exactly its ADR-backed contract. Describe table
  output and the CLI/almanac package boundary only as explanatory prose, not normative claims. Remove the proof
  marker from production and add
  `// invariant: almanac/model:almanac-clamped-latitude` to `TestClampLatitude`. Author Sundial
  `.awf/current-state-migration.yaml` under the same strict schema with exactly one matching approval
  entry for each independently derived live mapping and no entries for retired inventory keys.

- [ ] **Task 1.5: Resolve every in-flight ADR before attestation.** Set root ADR-0001 and ADR-0133,
  ADR-0134, ADR-0135, ADR-0136 to Implemented after their resulting claims/runtime are present. Set
  Sundial ADR-0003 to Abandoned with rationale that the speculative cache/schedule seam was never
  implemented. Let bridge normalization change every Superseded legacy ADR to Implemented and append
  required Migration history; do not fabricate State changes on legacy ADRs. Assert ADR-0120's body remains byte-for-byte unchanged while its `related:` frontmatter includes ADR-0136, limiting the relation regression scope to frontmatter and generated indexes.

- [ ] **Task 1.6: Apply all sealed Plan 3 and documentation changes.** Apply Plan 3 Phases 2-5 runtime,
  tests, deletion, templates, skills, agents, AGENTS parts, workflow/working-with-awf, architecture,
  all domain parts, config-reference sources, glossary, pitfalls, README, releasing source, and
  changelog before attestation. Plan 3 Phase 1 (the immutable snapshot seam) is already present in
  `prep_base` and is not part of this patch. Set `project.Version` to 0.19.0. The pinned bridge embeds its own
  0.18.0 templates, so its preparation sync/check validates only bridge-rendered outputs; current-
  template INDEX/domain/runtime fan-out is deliberately deferred to final upgrade and final source
  sync/check. Do not change any configured marker-source or other sealed path afterward.

  Add a tested preparation-only branch to `.githooks/pre-commit`: when both `AWF_PREP_BRIDGE` and
  `AWF_PREP_BRIDGE_SHA256` are set, retain the staged-slice `go build`, verify the external binary hash,
  and use that binary for root/Sundial staged drift checks; otherwise retain the normal source-binary
  path exactly. This is the sanctioned commit boundary, not `--no-verify`, and is removed or made inert
  after cutover.

- [ ] **Task 1.7: Create the clean preparation commit.** Materialize the reviewed runtime/adopter diff
  as `$art/current-state-preparation.patch`, then create and verify the detached worktree:

  ```sh
  git worktree add --detach /tmp/awf-cs-prep "$prep_base"
  cd /tmp/awf-cs-prep
  test "$(git rev-parse HEAD)" = "$prep_base"
  git apply --check "$art/current-state-preparation.patch"
  git apply --binary "$art/current-state-preparation.patch"
  ```

  Run only the pinned bridge for readiness/sync/check project commands:

  ```sh
  sha256sum -c "$art/awf-bridge.sha256"
  "$art/awf-bridge" upgrade --check
  (cd examples/sundial && "$art/awf-bridge" upgrade --check)
  "$art/awf-bridge" sync
  (cd examples/sundial && "$art/awf-bridge" sync)
  "$art/awf-bridge" check
  (cd examples/sundial && "$art/awf-bridge" check)
  go test ./...
  (cd examples/sundial && go test ./...)
  ./x gate
  (cd examples/sundial && ./x gate)
  git diff --check
  ```

  Expected: both readiness reports have zero findings and every adjudication has computed
  `approved:true` after Task 1.5; bridge checks and both projects' source tests and full gates are clean. `./x gate` is allowed
  because it runs source tests and authority-independent static
  tooling, not source context/invariants/check. Stage the exact authored and bridge-rendered set, then
  commit through the sanctioned hook:

  ```sh
  AWF_PREP_BRIDGE="$art/awf-bridge" \
  AWF_PREP_BRIDGE_SHA256="$(cut -d' ' -f1 "$art/awf-bridge.sha256")" \
    git commit -m "feat(awf): prepare current-state authority cutover"
  ```

  Record `prep_head`, require clean status, and do not amend this commit. Before any attestation, build the matching current-state binary from this exact commit, record and verify its SHA-256 and version, and keep it available for immediate seal consumption:

  ```sh
  go build -trimpath -o "$art/awf-current" ./cmd/awf
  sha256sum "$art/awf-current" | tee "$art/awf-current.sha256"
  sha256sum -c "$art/awf-current.sha256"
  "$art/awf-current" version
  ```

## Coupled Phases 2-3: Attest and immediately consume both seals

No commit is legal between these phases: an attested bridge lock intentionally prunes legacy indexes and refuses ordinary commands until the prebuilt, verified current runtime consumes it. If that binary is unavailable or fails identity verification, do not attest; escape from the window is Git restoration plus bridge reinstallation.

- [ ] **Task 2.1: Produce exact disjoint attestation patches.** Run:

  ```sh
  prep_head="$(git rev-parse HEAD)"
  sha256sum -c "$art/awf-current.sha256"
  git worktree add --detach /tmp/awf-cs-root "$prep_head"
  git worktree add --detach /tmp/awf-cs-sundial "$prep_head"
  (cd /tmp/awf-cs-root && sha256sum -c "$art/awf-bridge.sha256" &&
    "$art/awf-bridge" upgrade --check --json > "$art/root-readiness.json" &&
    "$art/awf-bridge" upgrade --attest-current-state)
  git -C /tmp/awf-cs-root diff --binary --full-index HEAD -- . \
    ':(exclude)examples/sundial/**' > "$art/root-attestation.patch"
  git -C /tmp/awf-cs-root diff --name-only -z HEAD -- . \
    ':(exclude)examples/sundial/**' | sort -zu > "$art/root-attestation.paths0"
  (cd /tmp/awf-cs-sundial/examples/sundial && sha256sum -c "$art/awf-bridge.sha256" &&
    "$art/awf-bridge" upgrade --check --json > "$art/sundial-readiness.json" &&
    "$art/awf-bridge" upgrade --attest-current-state)
  git -C /tmp/awf-cs-sundial diff --binary --full-index HEAD -- examples/sundial \
    > "$art/sundial-attestation.patch"
  git -C /tmp/awf-cs-sundial diff --name-only -z HEAD -- examples/sundial | sort -zu \
    > "$art/sundial-attestation.paths0"
  test -z "$(comm -z -12 "$art/root-attestation.paths0" "$art/sundial-attestation.paths0")"
  ```

  Expected: both reports are ready with every adjudication approved, patches are nonempty, both
  approval files remain byte-for-byte and mode-for-mode equal to `prep_head`, neither appears in its
  attestation patch, and the disjoint assertion emits no output.

- [ ] **Task 2.2: Verify sealed-input exclusion and combine.** Final resync makes Plan 2's JSON
  `plannedMutations` the exact path/before/after-SHA allowlist and tests it against the journal plan.
  Save root and Sundial JSON check reports before attestation; use a Python assertion to require exact
  set equality between patch paths and planned mutation paths; verify every before hash/mode/presence
  against `prep_head`, every after hash/mode/presence in its attestation worktree, equality with journal
  operations, and each lock's PreparedHead equals `prep_head`. Explicitly assert both approval files
  are digest members but absent from `plannedMutations` and journal operations, and remain unchanged in
  the attestation worktrees. Any mismatch fails. Then:

  ```sh
  git worktree add --detach /tmp/awf-cs-integration "$prep_head"
  cd /tmp/awf-cs-integration
  git apply --check "$art/root-attestation.patch"
  git apply --binary "$art/root-attestation.patch"
  git apply --check "$art/sundial-attestation.patch"
  git apply --binary "$art/sundial-attestation.patch"
  test "$(git rev-parse HEAD)" = "$prep_head"
  ```

  Expected: all checks emit no output; do not commit.

## Phase 3: Consume both seals and close the repository

- [ ] **Task 3.1: Build and run the current runtime.** In integration run:

  ```sh
  sha256sum -c "$art/awf-current.sha256"
  "$art/awf-current" upgrade
  (cd examples/sundial && "$art/awf-current" upgrade)
  ```

  Each invocation verifies seal version 1, PreparedHead, and exactly the sorted slash-relative path/mode/content digest records over the post-normalization proposed result for config, domains, ADRs, topics, configured marker sources, and the required approval file; journals the complete final plan including deletion of that file; replaces
  lock last; clears attestation; promotes cutoff/gaps; and removes the bridge approval parser/runtime
  claim.

- [ ] **Task 3.2: Assert permanent lock and output facts.** Run:

  ```sh
  python3 - <<'PY'
  import json
  for path, cutoff in ((".awf/awf.lock", 137),
                       ("examples/sundial/.awf/awf.lock", 4)):
      lock = json.load(open(path, encoding="utf-8"))
      assert lock["awfVersion"] == "0.19.0", path
      assert lock["adrFormatV1From"] == cutoff, path
      assert lock["legacyAdrGaps"] == [], path
      assert "bridgeAttestation" not in lock, path
  PY
  for f in docs/decisions/INDEX.md examples/sundial/docs/decisions/INDEX.md; do
    grep -q '^## In flight' "$f" && grep -q '^## History' "$f"
  done
  test ! -e .awf/current-state-migration.yaml
  test ! -e examples/sundial/.awf/current-state-migration.yaml
  test ! -e docs/decisions/ACTIVE.md
  test ! -e examples/sundial/docs/decisions/ACTIVE.md
  test -z "$(find docs/domains examples/sundial/docs/domains -type f -path '*/decisions/*' -print)"
  go test ./internal/project -run '^TestCurrentStateOutputPlanMatchesTree$' -count=1
  ```

  Expected: Python and filesystem assertions emit no output; both approval files are absent; the
  output-plan/tree/topic coverage test reports `ok`, owns every configured topic document and domain
  index, and confirms permanent sweep/runtime nonconsumption of the deleted migration path.

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

- [ ] **Task 3.4: Freeze Plan 3 and verify ADR history.** Require Plans 1-2 already Implemented at
  prep_base. Set only Plan 3 to Implemented and record its Notes; keep this Plan 4 Proposed through
  the release phase. Confirm ADR-0133-0136 and
  root ADR-0001 are Implemented, Sundial ADR-0003 Abandoned, and INDEX
  sections reflect them. Run final sync/check/gate again.

- [ ] **Task 3.5: Commit the permanent cutover.** Stage exactly the two attestation patches, both
  journaled `.awf/current-state-migration.yaml` deletions, final upgrade/output-plan changes, locks,
  plan statuses, and regenerated fan-out; commit:

  ```commit
  feat(awf): cut over to current-state authority
  ```

  Record `cutover_head="$(git rev-parse HEAD)"`; working tree must be clean. Return to the clean primary
  checkout, fetch origin, require local main and origin/main equal `prep_base`, then
  `git merge --ff-only "$cutover_head"`; assert clean main contains the cutover commit before release.

## Phase 4: Tag and publish the breaking current-state release

- [ ] **Task 4.1: Create the dated release commit on tag day.** Keep Unreleased and this plan Proposed
  during preparation and cutover. On the UTC tag date, record final Notes, set this Plan 4 to
  Implemented, promote breaking notes to `[0.19.0] - YYYY-MM-DD`, create a fresh
  empty Unreleased, verify `project.Version == "0.19.0"` and both locks use 0.19.0, then run
  sync/check/gate/releasecheck and commit:

  ```commit
  docs(awf): release 0.19.0
  ```

  The release commit and annotated tag must be created on that same recorded UTC date.

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

- [ ] **Task 4.3: Push main and tag `v0.19.0`.** Run the exact safety assertions against the canonical
  origin URL:

  ```sh
  git fetch origin main --tags
  test -z "$(git status --porcelain=v1)"
  test "$(git branch --show-current)" = main
  test "$(git remote get-url origin)" = 'git@github.com:hypnotox/agentic-workflows.git'
  ! git show-ref --verify --quiet refs/tags/v0.19.0
  test -z "$(git ls-remote --tags origin refs/tags/v0.19.0)"
  release_head="$(git rev-parse HEAD)"
  git push origin main:main
  git fetch origin main
  test "$(git rev-parse origin/main)" = "$release_head"
  git tag -a v0.19.0 "$release_head" -m "v0.19.0"
  test "$(git rev-parse 'v0.19.0^{commit}')" = "$release_head"
  git push origin refs/tags/v0.19.0
  ```

  Expected: the release workflow verifies tag/version equality, main ancestry, changelog pin, gate,
  drift, and pinned GoReleaser before publishing six platform archives, checksums, and curated notes.
  If any preflight fails, do not create or push the tag.

## Verification

- Bridge readiness requires each approval file, including exact `invariantApprovals: []` for zero live mappings, reports zero missing invariant mappings, approvals, markers, topic coverage, or
  in-flight ADRs, and every adjudication has computed approval with strict backing-class preservation.
- Both attestations share the preparation HEAD, have disjoint patches, preserve both approval files
  unchanged, and exclude those files from mutations merely due to digest membership.
- Final upgrades journal-delete both approval files; permanent locks have exact cutoffs/gaps and no
  attestation; INDEX replaces ACTIVE everywhere; permanent runtime neither claims nor consumes the
  deleted migration inputs.
- Context/invariants/check consume topic claims only; denylist/import/deadcode checks find no legacy
  engine.
- Plans 1-2 are already Implemented at the bridge tag. Root ADR-0001 and ADR-0133-0136 become
  Implemented, and Sundial ADR-0003 becomes Abandoned, in preparation; Plan 3 closes in the cutover
  commit and Plan 4 closes in the dated v0.19.0 release commit.
- `v0.19.0` is tagged only after the clean, gated cutover commit and release preflight.

## Notes

- Amended 2026-07-20 (execution finding): Plan 3 Phase 1 shipped as a standalone commit on main
  (per Plan 3's design), advancing main past the published v0.18.0 bridge release. The preparation
  base is therefore `prep_base` = post-Phase-1 main HEAD, distinct from the v0.18.0 commit; the
  v0.18.0 tag stays put (it is a published release) and serves only as the release-audit floor
  (`v0.18.0..HEAD`) and the bridge-release ancestor. Tasks 0.1, 1.1, 1.6, 1.7, 3.4, and 3.5 were
  retargeted from `bridge_head` to `prep_base`. The pinned bridge binary is built from `prep_base`
  and remains 0.18.0 because Phase 1 changed neither `internal/bridge` nor the version constant.
  `prep_base` must equal `origin/main`, so the Phase 1 commits are pushed before this plan runs.
- If a journal exists, run the matching binary's `upgrade --recover` in that project before any other
  command; if only one project upgraded, recover/restore it before touching the other. Before the
  cutover commit, discard the integration worktree and recreate it at `prep_head`. After a successful
  unpushed cutover, reset the primary worktree to `prep_base` and reinstall the v0.18.0 bridge binary. After main is
  pushed, rollback requires a new reviewed forward-revert commit, never history rewrite. If an
  erroneous tag/release was pushed, stop publication, delete the GitHub Release, delete remote/local
  tag only with maintainer approval, fix forward, rerun the complete preflight, and issue a new tag.
- Remove `/tmp/awf-cs-{root,sundial,integration,prep}` with `git worktree remove` and prune only after
  the release succeeds or rollback evidence has been preserved.
