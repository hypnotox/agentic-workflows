---
date: 2026-07-19
adrs: [132]
status: Proposed
---
# Plan: Structured Cross-Runtime Exploration Workflow

## Goal

Implement [ADR-0132](../decisions/0132-structured-cross-runtime-exploration-workflow.md): ship a core cross-runtime exploring skill, migrate adopted configs into its dependency closure, and give Pi a required structured exploration schema and fixed bounded-reporting prompt.

Non-goals: exact non-Pi runtime API syntax, filesystem sandboxing, recursive child orchestration, retained search sessions, and changes to the Pi runner/process machinery.

## Architecture summary

One coupled behavior phase changes the catalog, config generation, migration registry, workflow templates, and Pi extension together because no smaller committed slice can keep both the skill dependency closure and Pi's required call schema valid. Tests land first within the phase but red state is not committed. The phase adds schema generation 13 by rerunning the existing close-enabled-set migration against the revised catalog, renders one semantic exploration protocol to all six targets, and maps Pi's branch to explicit `task`, `breadth`, and `detail` tool arguments. Authored docs and generated adopter copies travel in that same behavior commit. A final lifecycle phase records deviations and freezes the ADR and plan.

## File structure

- **Created:**
  - `templates/skills/exploring/SKILL.md.tmpl`
  - generated `awf-exploring` / `sundial-exploring` skill copies under every enabled target directory
- **Modified:**
  - catalog, migration, compatibility, render, scaffold, eval, and Pi-extension tests under `internal/catalog/`, `internal/migrate/`, `internal/project/`, `internal/evals/`, `cmd/awf/`, and `tools/pi-extension-test/tests/`
  - `internal/catalog/standard.go`, `internal/migrate/migrate.go`, `internal/project/project.go`
  - `templates/pi/awf-subagents/index.ts.tmpl`
  - `templates/skills/brainstorming/SKILL.md.tmpl`, `templates/skills/debugging/SKILL.md.tmpl`, and `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`
  - `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/doc-standard.md.tmpl`, and authored documentation/config sources named by ADR-0132 Decision 9
  - `.awf/config.yaml`, `examples/sundial/.awf/config.yaml`, both lock files, generated docs, generated skills, and generated Pi extension copies
  - ADR-0132, its predecessor indexes, and this plan in the lifecycle commit
- **Deleted:** none

## Phase 1: Ship the structured exploration workflow

This is one coupled, gateable behavior commit. The new core skill, its consumer requirements, generation-13 migration, and Pi schema cannot be split across commits without either producing an invalid enabled-skill closure or rendering Pi calls that disagree with the registered tool schema.

- [ ] **Task 1.1: Write failing catalog, scaffold, migration, and cross-runtime render tests.** Before production edits, make these exact test changes:
  - In `internal/project/scaffold_test.go`, put `// invariant: exploration-skill-closure` on the core-scaffold test and assert that the scaffolded skills include `exploring`; derive expected core membership from the catalog where the test already does so rather than pinning a new literal count.
  - In `internal/catalog/graph_test.go`, update the brainstorming closure expectation to include `exploring` and assert that exploring has no reverse dependency on brainstorming, debugging, or refactor-coupling-audit.
  - In `internal/project/resolve_test.go`, update closure expectations so enabling any of brainstorming, debugging, or refactor-coupling-audit brings in exploring; preserve operation ordering assertions by deriving the expected plan from the sorted resolver output, not a hard-coded corpus count.
  - In `internal/migrate/migrate_test.go`, append `exploring-skill-closure` to both ordered migration-name expectations and rename the current-generation test to generation 13 with `Current == 13`.
  - In `internal/migrate/closeenabledset_test.go`, add a generation-13-oriented fixture catalog containing an `exploring` skill plus the three consumer edges. Assert that `applyCloseEnabledSet` adds exploring once, reports the existing `close-enabled-set: enabled skill "exploring" (required by ...)` diagnostic, and is idempotent on the second run.
  - In `cmd/awf/run_test.go`, add an upgrade fixture with schema-12 lock state and an enabled consumer lacking exploring. Run upgrade and assert schema 13, the exploration addition diagnostic, and a successful post-upgrade project open/check.
  - In `internal/project/target_test.go`, add `// invariant: cross-runtime-exploration-dispatch` to a test that renders all targets and asserts all six exploring skill paths exist; Pi names `subagent_explore` with `task`, `breadth`, and `detail`, while the other five require a generic target-native fresh-context exploration child and contain no `subagent_explore` token.
  - In `internal/project/target_test.go`, add `// invariant: bounded-exploration-reporting` to a test asserting the rendered skill contains all breadth/detail values, adaptive-maximum wording, the project search universe, explicit not-found/inconclusive/unverified outcomes, one information need, and parent-driven sequential refinement.
  - In `internal/evals/chain_test.go`, add only the composed seam: a full-catalog render proves each consumer invokes the exploring skill and the Pi exploring rendering names the registered extension tool. Do not duplicate single-prompt assertions here.
  - In `internal/project/spine_test.go`, add the mandatory `TestExploringTemplate` golden and the empty-data fallback case required by the conditional Pi/non-Pi branch.

  Run:

  ```bash
  go test ./internal/catalog ./internal/migrate ./internal/project ./internal/evals ./cmd/awf
  ```

  Expected: non-zero failures only for the absent catalog skill/template, generation-13 migration, revised closures, and structured Pi contract. Do not commit red state.

- [ ] **Task 1.2: Write failing Pi schema and prompt tests.** In `tools/pi-extension-test/tests/index.test.ts`:
  - Rename the exact public-contract test to describe structured exploration and validate all four tools.
  - Compile/check the exploration parameter schema. Accept exactly a non-empty task plus one breadth in `targeted | bounded | broad` and one detail in `paths | summary | analysis`. Reject each missing field, empty task, unknown enum value, and every additional property.
  - Execute all breadth values and all detail values through representative combinations, including `broad + paths` and `targeted + analysis`, and assert the runner request's exploration system prompt carries the selected values.
  - Assert the prompt defines breadth as an adaptive maximum, defines all three levels, names the tracked plus non-ignored-untracked project universe and its exclusions, requires the exact `Not found within <breadth> boundary:` prefix, distinguishes inconclusive and unverified, grounds material claims, prohibits mutation/commit/recursive delegation as policy, and allows parent-driven sequential refinement only through a new call.
  - Keep the existing assertions for repository-root cwd, inherited model/thinking level, `EXPLORE_TOOLS`, partial-details isolation, final-only model content, failure behavior, and no public subagent in the child allowlist.

  Run:

  ```bash
  ./x pi-test run
  ```

  Expected: non-zero only because exploration still has the one-field schema and fixed one-line prompt. Do not commit red state.

- [ ] **Task 1.3: Add the catalog skill, workflow dependencies, and generation-13 migration.** Apply these production changes:
  - In `internal/catalog/standard.go`, add `exploring` with `Core: true`, no chain flag and no requirements, and sections exactly `when-to-invoke`, `breadth`, `detail`, `dispatch`, `results`, `boundaries`, `notes`. Add `exploring` once to each `RequiresSkills` list for brainstorming, debugging, and refactor-coupling-audit; do not add a reciprocal edge.
  - Create `templates/skills/exploring/SKILL.md.tmpl` with frontmatter name `{{ .prefix }}-exploring` and a description that triggers on unknown repository locations whose inline search would pollute the parent context. Its sections must encode ADR-0132 Decisions 2-6 exactly:
    - `when-to-invoke`: delegate unknown-location, non-trivial discovery; keep known-file/trivial reads inline; one information need per call.
    - `breadth`: define `targeted < bounded < broad`, adaptive maximum semantics, and the tracked plus non-ignored-untracked project universe with the ignored/`.git`/nested-repository/external-dependency exclusions.
    - `detail`: define `paths < summary < analysis` independently from breadth and give the exact grounding expectation for each.
    - `dispatch`: require a self-contained task. In the `.targetSubagentTools` branch, call `subagent_explore` once with required `task`, `breadth`, and `detail`; otherwise dispatch one target-native fresh-context exploration subagent with those values and contracts written into its brief.
    - `results`: require the exact not-found prefix, distinguish inconclusive/unverified, consume only relevant final findings, and allow a new sequential refinement call.
    - `boundaries`: report-only policy, no edits/commits/recursive delegation, no widening beyond maximum, no unrelated question bundle, and no retained child state.
    - `notes`: Pi is the deeply integrated awf-owned implementation; non-Pi targets receive semantic parity through native delegation, not identical orchestration.
  - In the three consumer templates, replace local search-orchestration prose with unconditional invocation of `{{ .prefix }}-exploring` where location is unknown or transcript noise is expected. Brainstorming keeps its later `subagent_grounding` step unchanged. Debugging keeps hypothesis/oracle/test-first order unchanged. Refactor coupling audit keeps the six-category output contract but delegates large/noisy discovery through exploring rather than naming `subagent_explore` directly.
  - In `internal/migrate/migrate.go`, append `{To: 13, Name: "exploring-skill-closure", Apply: applyCloseEnabledSet}`. Do not duplicate the closure algorithm.
  - In `internal/project/project.go`, map schema generation 13 to `0.17.0` in `minVersionBySchema`; keep `Version` unchanged unless the implementation discovers a release-policy requirement, and record any such deviation in Notes before proceeding.

  Run:

  ```bash
  go test ./internal/catalog ./internal/migrate ./internal/project ./internal/evals ./cmd/awf
  ```

  Expected: catalog/template/closure/migration tests pass; Pi contract tests may remain red until Task 1.4.

- [ ] **Task 1.4: Implement Pi's exact structured exploration contract.** In `templates/pi/awf-subagents/index.ts.tmpl`:
  - Introduce closed TypeScript breadth/detail types or constants derived from the exact enums without widening the public API.
  - Change only exploration's schema to required `task`, `breadth: StringEnum(["targeted", "bounded", "broad"] as const)`, and `detail: StringEnum(["paths", "summary", "analysis"] as const)`, retaining `additionalProperties: false`.
  - Change the exploration prompt constructor to accept breadth and detail and append the selected values plus the complete adaptive-boundary, search-universe, outcome, grounding, one-need, and detail-specific report instructions from ADR-0132. Keep grounding and implementation prompt behavior unchanged.
  - Pass `params.breadth` and `params.detail` only into exploration prompt construction. Do not change `RunRequest`, `runner.ts`, model/thinking inheritance, cwd, tool allowlists, event retention, renderers, cancellation, output bounds, review loading, or implementation serialization.
  - Replace the old Go proof marker in `internal/project/target_test.go` with `// invariant: pi-structured-exploration-contract`; assert exactly four registrations, unchanged grounding/review/implementation schemas, and the new exact exploration schema and prompt forwarding.

  Run:

  ```bash
  ./x pi-test run
  go test ./internal/project ./internal/evals
  ```

  Expected: both commands pass with no failures and TypeScript coverage remains 100% for statements, branches, functions, and lines.

- [ ] **Task 1.5: Update authored guidance, migration inputs, and release notes.** Modify the exact authored surfaces below; generated copies are handled in Task 1.6:
  - `templates/agents-doc/AGENTS.md.tmpl`: list exploring among task skills and add the general unknown-location/context-pollution trigger without making every exact read a delegation.
  - `templates/docs/doc-standard.md.tmpl`: keep action-first tool-agnostic prose as the default and add the narrow capability-selected exception for an awf-owned runtime integration such as Pi's `subagent_explore`; do not permit arbitrary native runtime tool names.
  - `README.md`: describe required Pi exploration arguments, the cross-runtime exploring skill, and twelve core skills.
  - `templates/docs/working-with-awf.md.tmpl`: replace `{task}` exploration prose with the exact required schema and summarize breadth/detail semantics and sequential refinement.
  - `.awf/docs/parts/architecture/components.md`: add the core skill, generation-13 migration reuse, cross-runtime dispatch, and dynamic Pi prompt while retaining the runner boundary.
  - `.awf/docs/parts/releasing/content.md`: extend the existing real-Pi smoke with one successful structured lookup and one bounded not-found followed by a wider/corrected call; name all three arguments.
  - `.awf/docs/parts/testing/layout.md`: locate schema/prompt behavior in the TypeScript lane and cross-target/catalog/migration proofs in Go; state that deterministic tests prove instructions, not arbitrary model compliance.
  - `.awf/domains/parts/rendering/current-state.md`: record the core exploring skill, six-target semantic rendering, and Pi structured branch.
  - `.awf/domains/parts/tooling/current-state.md`: change the core from eleven to twelve and record generation 13's closure migration and revised exact Pi contract test.
  - `examples/sundial/README.md`: add exploring to the full-surface example description.
  - `changelog/CHANGELOG.md`: under Unreleased, add the adopter-facing core skill and Pi schema change, explicitly warning that hand-authored Pi calls now owe breadth and detail; mention automatic schema-13 config closure.

  Do not manually add exploring to either `.awf/config.yaml`. Task 1.6 must prove the migration adds it to the repository and Sundial configs.

  Run:

  ```bash
  rg -n 'eleven core|subagent_explore.*required `task`|subagent_explore \{task' README.md templates .awf examples/sundial/README.md changelog/CHANGELOG.md
  ```

  Expected: no stale behavioral or core-count statement; historical ADR/plan text is allowed and should be excluded from remediation.

- [ ] **Task 1.6: Rehearse upgrade, regenerate every target, verify, and commit.** Run the source-built upgrade in both adopted trees before sync:

  ```bash
  ./x upgrade
  bindir="$(mktemp -d)"
  go build -o "$bindir/awf" ./cmd/awf
  (cd examples/sundial && "$bindir/awf" upgrade)
  rm -rf "$bindir"
  ./x sync
  ./x check
  ```

  Expected: each upgrade reaches schema generation 13 and adds exploring through the closure migration; sync creates the exploring skill under every enabled target, updates the three consumer skills, Pi extension, AGENTS.md, config reference, docs, locks, and all Sundial copies; check is clean with no notes.

  Inspect fanout with:

  ```bash
  find .claude .pi examples/sundial/.claude examples/sundial/.agents \
    examples/sundial/.github examples/sundial/.cursor examples/sundial/.gemini \
    examples/sundial/.pi -path '*-exploring/SKILL.md' -print | sort
  rg -n 'subagent_explore' .claude/skills examples/sundial/.agents/skills \
    examples/sundial/.github/skills examples/sundial/.cursor/skills \
    examples/sundial/.gemini/skills
  ```

  Expected: the first command lists the enabled main and six Sundial target copies; the second prints no matches. Then run:

  ```bash
  ./x gate
  git diff --check
  ```

  Expected: the full gate passes at 100% Go statement coverage and 100% Pi-extension statement/branch/function/line coverage; no whitespace errors.

  Stage only the files enumerated by `git status --short` for ADR-0132 behavior, including authored sources and generated copies. Review `git diff --cached --stat` and `git diff --cached --name-only` to ensure no unrelated file is staged, then commit:

  ```commit
  feat(rendering): add structured exploration workflow
  ```

## Phase 2: Freeze ADR-0132 and the plan

- [ ] **Task 2.1: Record implementation deviations.** Add a concrete `Implementation deviations:` entry to this plan's Notes. Name every changed path or test seam that differed from Phase 1; if none differed, write `Implementation deviations: none.` Keep the plan Proposed during this edit.

- [ ] **Task 2.2: Flip lifecycle state and regenerate indexes.** Change `status: Proposed` to `status: Implemented` in ADR-0132 and this plan. Do not change ADR-0038 or ADR-0125 status. Run:

  ```bash
  ./x sync
  ./x check
  ./x gate
  git diff --check
  ```

  Expected: ACTIVE.md and the rendering/tooling domain indexes show ADR-0132 as Implemented; all checks pass and no generated drift remains.

  Stage ADR-0132, this plan, `.awf/awf.lock`, `docs/decisions/ACTIVE.md`, `docs/domains/rendering.md`, and `docs/domains/tooling.md` only when changed. Review the staged path list, then commit:

  ```commit
  docs(adr): implement 0132 structured exploration
  ```

## Verification

From a clean tree after Phase 2:

```bash
./x check
./x gate
git status --short
rg -n 'status: Implemented' \
  docs/decisions/0132-structured-cross-runtime-exploration-workflow.md \
  docs/plans/2026-07-19-structured-cross-runtime-exploration-workflow.md
rg -n 'name: "subagent_explore"|targeted|bounded|broad|paths|summary|analysis' \
  .pi/extensions/awf-subagents/index.ts
```

Expected: check and gate pass; git status is empty; both lifecycle files are Implemented; the generated Pi extension contains the structured exploration registration and enum values.

Manual release smoke, following `docs/releasing.md`, remains non-gated: on real Pi 0.80.9 or newer, run one successful `targeted + paths` lookup and one bounded not-found followed by a corrected or broad call. Confirm intermediate child activity appears only in tool details and only the final report enters model-visible content.

## Notes

- The behavior phase is intentionally one commit because the core skill's dependency closure, generation-13 repair, consumer calls, and Pi's required schema must agree at every committed gate.
- Keep Pi's runner and all process/output boundaries unchanged; an implementation need to modify `runner.ts` is an ADR resync trigger, not an incidental plan deviation.
- Generic non-Pi action wording is settled by ADR-0132. Exact target-native API syntax is out of scope.
- Implementation deviations: pending.
