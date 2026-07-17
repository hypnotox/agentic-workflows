---
date: 2026-07-17
adrs: [124]
status: Implemented
---
# Plan: Deterministic Output Plans and Target Capabilities

## Goal

Implement ADR-0124's one internal, deterministic output-plan authority for target, neutral,
singleton, generated, and local-reservation paths, with explicit output policy and closed target
capabilities. It does not add adopter-configurable targets, plugins, or a config schema.

## Architecture summary

`internal/project` will compile configured producers into a topologically ordered, path-keyed
plan. A plan node declares either a managed write or a local reservation, its recipe, policy,
dependencies, and target declarers. Rendering returns planned rendered nodes; sync, manifest,
prune, check, and planned-output reporting consume those nodes. A config-reference node depends
on preceding ordinary/domain output metadata but excludes itself. Target descriptors contribute
typed placements, output declarations, encoder/provenance data, and a closed capability set;
recipe comparison excludes target identity while hashes retain sorted declarers.

## File structure

- **Created:** `/home/hypno/Projects/agentic-workflows/internal/project/output_plan.go`,
  `/home/hypno/Projects/agentic-workflows/internal/project/output_plan_test.go`.
- **Modified:** `/home/hypno/Projects/agentic-workflows/internal/project/{target,render,check,project,confighash,agent}.go`,
  `/home/hypno/Projects/agentic-workflows/internal/project/{target,drift,project,coverage}_test.go`,
  `/home/hypno/Projects/agentic-workflows/internal/config/{config,config_test}.go`,
  `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/overview.md`,
  `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/{rendering,tooling}/current-state.md`,
  and their sync-generated outputs `/home/hypno/Projects/agentic-workflows/{docs/architecture.md,docs/domains/rendering.md,docs/domains/tooling.md,.awf/awf.lock}`.
- **Deleted:** obsolete target-output path, `isManagedMarkdown`, suffix-based policy, and
  local-path helper branches once their behavior is owned by output-plan nodes.

## Phase 1: Compile, render, and consume the output plan

- [ ] **Task 1.1: Define strict target and output-plan descriptors.** In
  `internal/project/target.go`, replace `SkillDir`, `AgentDir`, `AgentSuffix`,
  `AgentDialect`, `BridgeFile`, `BridgeTemplate`, `SubagentTools`, and loose `TargetOutput`
  handling with typed placement, encoder, provenance, output-contribution, and closed
  `Capability` declarations. Provide the one named Pi capability and a deterministic template
  projection. Add descriptor precondition validation for known encoders/policies, complete
  bridge pairs, non-empty path-safe target outputs, and provenance-policy compatibility. Keep
  the six current registry definitions semantically identical.

- [ ] **Task 1.2: Add the compiler and use it for every producer.** Create
  `internal/project/output_plan.go` with exported-only-where-needed `OutputPlan`, `OutputNode`,
  `OutputRecipe`, `OutputPolicy`, and local-reservation types. Compile, sort, dependency-order,
  and path-index nodes for catalog docs/skills/agents, bridges, target extension outputs,
  mandatory/config-tree singletons, memory, generated ACTIVE/domain/config-reference outputs,
  and every enabled local skill/agent reservation. Coalesce equal normalized recipes at one path,
  retain sorted declarers for diagnostics and hash input, and reject differing recipes before any
  render/write. Make configuration-reference depend on regular/domain nodes and exclude itself.
  Refactor `internal/project/render.go` so `RenderAll` renders plan write nodes through existing
  section assembly and agent encoding, while generated-node recipes invoke their current
  generators through the same plan. In the same coupled change, refactor
  `internal/project/project.go` so SyncReport, manifest ownership, prune protection, and
  PlannedOutputs consume plan write/reservation nodes; refactor `internal/project/check.go` so
  frontmatter, link, skill-reference, local, and regeneration checks consume OutputPolicy; then
  delete `isManagedMarkdown`, the `.toml` exception, and separate local/output reconstruction.
  Refactor `internal/project/confighash.go` to hash the normalized recipe plus sorted declarers
  rather than a single raw `Target`.

- [ ] **Task 1.3: Preserve strict target selection.** In `internal/config/config.go`, reject a
  duplicate entry in `Config.Targets` during `Validate`, before `resolveTargets`. Keep unknown
  target validation in `internal/project` to avoid the existing import boundary.

- [ ] **Task 1.4: Test compiler, rendering, capabilities, and hash behavior.** Add
  `internal/project/output_plan_test.go` fixtures that assert deterministic node order and the
  complete class set, a single write/manifest node for equivalent shared recipes, pre-render
  failure for a differing template/context/encoder/provenance/policy, generated config-reference
  dependency ordering without a self-edge, and local reservation nodes. Add proof comments for
  `output-plan-complete`, `shared-output-coalesced`, and `target-capabilities-closed`. Extend
  `target_test.go` to assert the exact closed capability projection, invalid descriptor failure,
  empty-variable publication safety, all built-in target paths/encodings, and descriptor-hash
  selectivity. Extend `project_test.go` to prove PlannedOutputs contains write nodes, excludes
  local reservations, and preserves init collision input. Extend `config_test.go` with
  `// invariant: duplicate-target-rejected`. Put `// invariant: output-plan-complete`,
  `// invariant: shared-output-coalesced`, and `// invariant: target-capabilities-closed` in
  `output_plan_test.go`, and `// invariant: output-policy-explicit` in the named policy-routing
  test in `drift_test.go`. Run:

  ```sh
  go test ./internal/config ./internal/project
  ```

  Expected: `ok` for both packages.

- [ ] **Task 1.5: Document, regenerate, verify, and commit the coupled migration.** In the
  authored architecture and rendering/tooling current-state sources listed above, replace the
  old Target-field/render-loop narrative with output-plan authority, policies, reservations,
  capabilities, coalescing, and config-reference ordering. Run `./x sync`, `./x check`, and
  `./x gate`; expected final output includes `awf invariants: clean`, `coverage: 100.0%`,
  `deadcodecheck: no production dead code`, and `prose-gate: clean`. Stage exactly the absolute
  Phase 1 source/test/doc paths and generated outputs listed in File structure with `git add --`
  (never `git add -A`). Do not commit yet: Phase 2 is the explicitly coupled continuation because
  policy routing and lifecycle migration cannot be separately live while every producer is moving.

  ```commit
  refactor(rendering): compile deterministic output plans
  ```

## Phase 2: Drive lifecycle and checks from node policy

- [ ] **Task 2.1: Replace lifecycle reconstruction with plan consumption.** In
  `internal/project/project.go`, make `SyncReport` write manifest entries, derive `want`, and
  protect local paths from output-plan nodes instead of appending generated files and separately
  calling `localTargetPaths`. Make `PlannedOutputs` return planned write paths. Remove the
  superseded append/reconstruction helpers. Preserve foreign-file backups, executable shebang
  handling, regeneration behavior, and target-drop ancestor pruning.

- [ ] **Task 2.2: Replace template-ID and suffix drift inference.** In
  `internal/project/check.go`, make lock comparison, local-frontmatter validation,
  dead-reference scanning, and dead-skill-reference scanning select nodes by `OutputPolicy`.
  Delete `isManagedMarkdown` and the `.toml` exclusion. Ensure generated and in-place nodes use
  their declared regeneration policy and local reservations use their declared validation policy.
  Add proof comments for `output-policy-explicit` and `duplicate-target-rejected`.

- [ ] **Task 2.3: Test lifecycle and policy routing.** Extend `internal/project/drift_test.go`
  and `project_test.go` with Markdown, TOML, TypeScript, generated, in-place, and local fixtures
  whose policy behavior is asserted independently of their template IDs and filename suffixes;
  assert local reservations survive Sync pruning, shared nodes create one lock entry, removal of a
  target prunes only now-unwanted files, and Check reports missing/stale/hand-edited plan nodes.
  Extend `coverage_test.go` for every new precondition, collision, dependency, and policy branch.
  Run:

  ```sh
  go test ./internal/project
  ```

  Expected: `ok github.com/hypnotox/agentic-workflows/internal/project`.

- [ ] **Task 2.4: Verify and commit the coupled migration.** Run `./x gate`, then stage exactly
  `/home/hypno/Projects/agentic-workflows/internal/project/{output_plan.go,output_plan_test.go,target.go,render.go,check.go,project.go,confighash.go,agent.go,target_test.go,drift_test.go,project_test.go,coverage_test.go}`, `/home/hypno/Projects/agentic-workflows/internal/config/{config.go,config_test.go}`, `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/overview.md`, `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/{rendering,tooling}/current-state.md`, and sync-generated `/home/hypno/Projects/agentic-workflows/{docs/architecture.md,docs/domains/rendering.md,docs/domains/tooling.md,.awf/awf.lock}` with `git add -- <each path>`; commit:

  ```commit
  refactor(rendering): drive drift checks from output plans
  ```

## Phase 3: Document, dogfood, and close the decision

- [ ] **Task 3.1: Confirm documentation is current.** Verify the authored sources changed in the
  coupled migration, `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/architecture/overview.md`
  and `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/{rendering,tooling}/current-state.md`,
  describe planner authority, strict capabilities, coalescing, policies, reservations, and
  config-reference ordering; do not describe an adopter-configurable graph.

- [ ] **Task 3.2: Regenerate and verify shipped surfaces.** Run `./x sync` so rendered
  architecture/domain outputs, `docs/decisions/ACTIVE.md`, the lock, and the example adopter
  settle. Run `./x check`; expected final line is `awf invariants: clean` with no drift. Run
  `./x gate`; expected final checks include `coverage: 100.0%`, `deadcodecheck: no production dead
  code`, and `prose-gate: clean`.

- [ ] **Task 3.3: Freeze plan and ADR.** Change `status: Proposed` to `status: Implemented` in
  `docs/plans/2026-07-17-deterministic-output-plans-and-target-capabilities.md` and
  `docs/decisions/0124-deterministic-output-plans-and-target-capabilities.md`. Run `./x sync`,
  add `None beyond the recorded review findings.` (or actual findings) under Notes, then stage
  exactly `/home/hypno/Projects/agentic-workflows/docs/plans/2026-07-17-deterministic-output-plans-and-target-capabilities.md`, `/home/hypno/Projects/agentic-workflows/docs/decisions/0124-deterministic-output-plans-and-target-capabilities.md`, `/home/hypno/Projects/agentic-workflows/docs/decisions/ACTIVE.md`, `/home/hypno/Projects/agentic-workflows/docs/domains/rendering.md`, `/home/hypno/Projects/agentic-workflows/docs/domains/tooling.md`, and `/home/hypno/Projects/agentic-workflows/.awf/awf.lock` with `git add -- <each path>`, then run `./x gate` and commit:

  ```commit
  docs(rendering): implement deterministic output plans
  ```

## Verification

- `./x check` is clean after every sync and the Sundial example remains clean.
- `./x gate` passes at every phase boundary.
- The final `git diff HEAD~3..HEAD -- internal/project` contains no template-ID or filename-suffix
  policy classifier and no separate rendered-output reconstruction path.

## Notes

- None beyond the recorded review findings.
- A dedicated grounding-check agent is out of scope; the current workflow uses one
  `subagent_explore` grounding dispatch.
