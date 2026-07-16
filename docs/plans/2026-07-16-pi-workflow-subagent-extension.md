---
date: 2026-07-16
adrs: [123]
status: Proposed
---
# Plan: Pi Workflow Subagent Extension

## Goal

Implement [ADR-0123](../decisions/0123-pi-workflow-subagent-extension.md): every adopter enabling
the Pi target receives three explicit, fresh-context workflow delegation tools, with generated-file
drift ownership and a fast persistent Docker test lane. Non-goals are arbitrary user-defined agent
orchestration, OS-level sandboxing, worktree-per-child execution, and native subagent support in Pi.

## Architecture summary

First add the containerized TypeScript gate as an independently useful tooling slice. Then extend the
target descriptor and render pipeline with Pi-owned TypeScript outputs, ship the three tools over one
subprocess runner, and behavior-test the dogfooded generated files. Next bind Pi workflow skill copies
to those tools while preserving every other target's wording. Finish by syncing both dogfood trees,
updating adopter and maintainer documentation, removing the completed roadmap entry, and freezing the
plan and ADR together.

The extension uses Pi 0.80.9 APIs and the current-process child invocation pattern from Pi's official
subagent example, but does not copy its general agent discovery, parallel mode, chain mode, unbounded
message retention, or `proc.killed` cancellation test. Reviewer bodies remain owned by awf's existing
`.pi/skills/{adr-reviewer,plan-reviewer,code-reviewer}.md` outputs.

## File structure

- **Created:**
  - `tools/pi-extension-test/Dockerfile`, `package.json`, `package-lock.json`, `tsconfig.json`,
    `container.sh`: digest-pinned, dependency-fingerprinted persistent test environment.
  - `tools/pi-extension-test/fixture/smoke.ts` and `smoke.test.ts`: Phase 1 proof that the container
    type-check/coverage lane is live; deleted when real extension tests replace it.
  - `tools/pi-extension-test/tests/index.test.ts`, `runner.test.ts`, and
    `fixtures/{fake-pi.mjs,term-resistant-pi.mjs}`: deterministic extension tests and child fixtures.
  - `templates/pi/awf-subagents/index.ts.tmpl` and `runner.ts.tmpl`: the shipped extension source.
  - `.pi/extensions/awf-subagents/{index.ts,runner.ts}` and
    `examples/sundial/.pi/extensions/awf-subagents/{index.ts,runner.ts}`: dogfooded generated output.
- **Modified:**
  - `x`: mandatory Pi-extension gate plus `pi-test stop|reset` lifecycle commands.
  - `templates/embed.go`: embed the new `pi` template tree.
  - `internal/render/render.go`, `internal/render/render_test.go`: TypeScript line-comment style.
  - `internal/project/{target.go,render.go,banner.go,confighash.go}` and focused tests in
    `banner_test.go`, `target_test.go`, `drift_test.go`, `project_test.go`, and
    `tool_agnostic_test.go`: target-owned extension outputs, target-sensitive hashes, provenance,
    drift, and cleanup.
  - Workflow templates under `templates/skills/` for `brainstorming`, `refactor-coupling-audit`,
    `subagent-driven-development`, `reviewing-adr`, `reviewing-plan`,
    `reviewing-plan-resync`, and `reviewing-impl`, plus goldens in
    `internal/project/spine_test.go` and cross-artifact assertions in `internal/evals`.
  - `.awf/parts/agents-doc/identity.md`, `.awf/docs/parts/development/{setup,command-runner,dependencies}.md`,
    `.awf/docs/parts/testing/{gate,layout,tiers}.md`, `.awf/docs/parts/roadmap/ideas.md`,
    `.awf/domains/parts/rendering/current-state.md`, and
    `.awf/domains/parts/tooling/current-state.md`.
  - Generated `AGENTS.md`, `docs/{architecture,development,testing,roadmap}.md`,
    `docs/domains/{rendering,tooling}.md`, and corresponding Sundial outputs.
  - `README.md`, `changelog/CHANGELOG.md`, `.awf/awf.lock`,
    `examples/sundial/.awf/awf.lock`, ADR-0123, and this plan.
- **Deleted:** `tools/pi-extension-test/fixture/{smoke.ts,smoke.test.ts}` after the real tests land.

## Phase 1: Fast containerized TypeScript gate

- [ ] **Task 1.1: Add the pinned test image and dependency manifest.** Create
  `tools/pi-extension-test/Dockerfile` with this shape:

  ```dockerfile
  FROM node:22.22.0-alpine@sha256:e4bf2a82ad0a4037d28035ae71529873c069b13eb0455466ae0bc13363826e34
  WORKDIR /opt/awf-pi-test
  COPY package.json package-lock.json ./
  RUN npm ci --ignore-scripts
  COPY docker-entrypoint.sh ./
  ENTRYPOINT ["/opt/awf-pi-test/docker-entrypoint.sh"]
  ```

  Add `docker-entrypoint.sh`: initialize the mounted `/workspace/node_modules` volume from
  `/opt/awf-pi-test/node_modules` only when its fingerprint marker is absent, then `exec tail -f
  /dev/null`. Use plain POSIX `sh`; do not install an OS package. Add exact dev dependencies in
  `package.json`: `@earendil-works/pi-coding-agent` `0.80.9`, `typebox` `1.1.38`, `typescript`
  `5.9.3`, `tsx` `4.21.0`, and `@types/node` `22.20.1`. Generate `package-lock.json` inside the
  pinned image, never on the host:

  ```bash
  docker run --rm -v "$PWD/tools/pi-extension-test:/work" -w /work \
    node:22.22.0-alpine@sha256:e4bf2a82ad0a4037d28035ae71529873c069b13eb0455466ae0bc13363826e34 \
    npm install --package-lock-only --ignore-scripts
  ```

  Expected: `tools/pi-extension-test/package-lock.json` is created, `git status --short` contains no
  root or host `node_modules`, and `grep -n '0.80.9' tools/pi-extension-test/package-lock.json`
  matches the coding-agent package.

- [ ] **Task 1.2: Implement the persistent container manager.** Create executable
  `tools/pi-extension-test/container.sh` with `set -euo pipefail` and commands `run`, `stop`, and
  `reset`. It must:
  - derive the absolute repository root through `git rev-parse --show-toplevel`;
  - hash `Dockerfile`, `docker-entrypoint.sh`, `package.json`, and `package-lock.json` for the
    dependency fingerprint, and hash the root path for repository isolation;
  - label image, container, and named dependency volume with both hashes;
  - build from the narrow `tools/pi-extension-test` context only when the fingerprinted image is
    absent;
  - remove stale containers carrying the same repository label before creating the desired one;
  - mount the checkout read-only at `/workspace` and the named volume at
    `/workspace/node_modules`, then keep the container alive;
  - on `run`, start a stopped matching container and use one timed `docker exec` to run the exact
    type-check/test command from Task 1.3;
  - on `stop`, stop matching repository containers without deleting their dependency volume;
  - on `reset`, remove matching containers, volumes, and locally labeled images;
  - when `CI` is non-empty, trap exit and remove the current container and volume after the run;
  - print separate setup/start and test elapsed times.

  Every Docker lookup must use labels rather than a globally shared fixed container name. Quote the
  bind source so paths containing spaces work. A missing Docker daemon exits with one actionable
  `pi-extension-test: Docker is required by ./x gate` error.

- [ ] **Task 1.3: Prove type-checking and coverage before wiring the gate.** Add
  `tools/pi-extension-test/tsconfig.json` with `strict: true`, `noEmit: true`, NodeNext module and
  resolution, target ES2022, `types: ["node"]`, and an initial include for `fixture/**/*.ts`.
  Add `fixture/smoke.ts` exporting `boundedAdd(a, b, maximum)` and a Node test covering both the
  capped and uncapped branches. The manager's test command is:

  ```bash
  tsc -p tools/pi-extension-test/tsconfig.json && \
  node --import tsx --test --experimental-test-coverage \
    --test-coverage-lines=100 --test-coverage-functions=100 --test-coverage-branches=100 \
    tools/pi-extension-test/fixture/*.test.ts
  ```

  Run `tools/pi-extension-test/container.sh run` twice. Expected first run: image/container setup,
  one passing test, and 100% line/function/branch coverage. Expected second run: no build and no
  container creation, only one `docker exec` test run. Run
  `find . -maxdepth 2 -name node_modules -print`; expected no output outside Docker mounts as seen
  from the host.

- [ ] **Task 1.4: Make the lane mandatory and document its lifecycle.** In `x`, call
  `tools/pi-extension-test/container.sh run` after Go tests/coverage and before vet, and add a
  `pi-test)` case accepting only `run`, `stop`, or `reset`; update the usage line. Update authored
  development and testing parts so setup requires Go plus Docker (not host Node/npm), the gate table
  names TypeScript type/coverage tests, and `./x pi-test stop|reset` behavior is exact. Run
  `./x sync && ./x check`, then `./x gate` twice. Expected: both gates pass; the second reports no
  image rebuild/container creation. Test `./x pi-test stop`, rerun the gate, and expect only a
  container start; test `./x pi-test reset`, rerun, and expect one cached or clean image setup.

- [ ] **Task 1.5: Commit the container test lane.** Run `./x gate` and `./x check`. Stage only `x`,
  `tools/pi-extension-test/**`, the authored development/testing parts, and their generated docs and
  lock changes. Commit:

  ```commit
  feat(tooling): add containerized Pi extension tests
  ```

## Phase 2: Target-owned extension and subprocess engine

- [ ] **Task 2.1: Add TypeScript provenance and target output descriptors test-first.** In
  `internal/render/render.go`, add `SlashComment` to `CommentStyle`; make `wrap` and `open` emit
  `// ` for it. In `internal/project/banner.go`, make `injectBanner` use `// ` when the supplied
  style is `SlashComment`. Add failing tests first:
  - `internal/render/render_test.go`: `SlashComment.wrap`, pointer prefixes, and assembly produce
    `// awf:edit ...` rather than HTML or hash comments;
  - `internal/project/banner_test.go`: a plain TypeScript body with `SlashComment` begins
    `// GENERATED by awf...`;
  - `internal/project/target_test.go` with proof marker
    `// invariant: pi-extension-target-render`: Pi plans exactly the two extension paths, every
    other target plans none, and rendered content parses the expected `//` banner;
  - `internal/project/drift_test.go`: changing an extension descriptor path/style changes its
    config hash, deleting/modifying either output is drift, and sync restores it;
  - `internal/project/project_test.go`: disabling Pi makes both prior extension outputs stale and
    sync prunes them.

  In `internal/project/target.go`, add:

  ```go
  type TargetOutput struct {
      Path         string
      TemplateID   string
      CommentStyle render.CommentStyle
  }
  ```

  Add `Outputs []TargetOutput` and `SubagentTools bool` to `Target`; Pi carries the two outputs and
  sets `SubagentTools: true`. Do not hard-code `t.Name == "pi"` in `RenderAll`.

- [ ] **Task 2.2: Make every adapter artifact target-hash-aware.** Replace `renderEncoding` with
  `renderOutputOptions` containing optional `encode`, `bannerStyle`, and a `target *Target`.
  `renderKind` constructs options for every target-scoped skill and agent, not only encoded agents.
  `renderTarget` applies `encode` only when non-nil, applies an explicit banner style when present,
  and passes the pointed target to `artifactConfigHash`. This fixes the existing hole where a
  target's review style changes skill output but does not enter the skill config hash. Preserve
  byte-identical non-Pi output. Add a focused drift assertion that changing only
  `Target.SubagentTools` changes a Pi skill config hash.

- [ ] **Task 2.3: Render descriptor-owned outputs.** Embed `all:pi` in `templates/embed.go`. In the
  target loop in `RenderAll`, after skills/agents, iterate `t.Outputs` in descriptor order and call
  `renderTarget` with kind `target-output`, no sidecar/sections, the normal data map, explicit style,
  and the target pointer. Add each returned file exactly once. The template source must render with
  empty vars under `missingkey=zero` and contain no `<no value>`; add empty-config coverage. Extend
  `TestAllTargetPathsAndBridges`, `PlannedOutputs`, sync/check, uninstall/planned cleanup, and target
  dialect tests rather than adding a second ownership path.

- [ ] **Task 2.4: Implement the shared runner template.** Create
  `templates/pi/awf-subagents/runner.ts.tmpl` as TypeScript with Node built-ins only. Export constants
  for the ADR limits and these test seams:

  ```typescript
  export type Role = "explore" | "review" | "implement";
  export interface RunnerDependencies { spawn; fs; makeTempDir; removeTempDir; gitSnapshot; now; }
  export interface RunRequest { role; task; cwd; model; thinkingLevel; tools; systemPrompt; signal; onUpdate; }
  export function createRunner(deps: RunnerDependencies): { run(request: RunRequest): Promise<RunResult> };
  export function resolvePiInvocation(argv: string[], execPath: string): { command: string; args: string[] };
  ```

  Follow Pi's official current-script/Bun/Node fallback logic. Spawn with `shell: false`, JSON mode,
  print mode, `--no-session`, `--model provider/id`, `--thinking`, `--tools`, and
  `--append-system-prompt`. Write the prompt under a mode-0700 temp directory as a mode-0600 file.
  Parse newline-delimited JSON incrementally; retain final assistant text and usage, compact tool/text
  event summaries rather than raw result messages, and enforce exactly the ADR byte/line/event caps.
  Truncation is UTF-8-safe and reports omitted counts. Track `closed` separately from signal delivery;
  TERM starts one five-second timer, KILL fires only when still open, and ordinary close removes timer
  and abort listener. Always remove the temporary directory. A non-zero exit, stop reason `error` or
  `aborted`, spawn error, malformed terminal stream, or compatibility failure throws one bounded
  diagnostic error.

- [ ] **Task 2.5: Implement the three-tool entry template.** Create
  `templates/pi/awf-subagents/index.ts.tmpl`. Import only Pi peer APIs and TypeBox plus the local
  runner. Export role allowlist constants and `registerSubagentTools(pi, dependencies?)`; the default
  export calls it with production dependencies. At registration:
  - read Pi's package version through `getPackageDir()` and require semver at least `0.80.9` using a
    local strict numeric parser; unsupported or unparseable versions register no partial tool set and
    notify one actionable minimum-version error;
  - register exactly `subagent_explore`, `subagent_review`, and `subagent_implement`, each with the
    ADR-required strict schema, prompt snippet, and tool-named guidelines;
  - reject empty/whitespace tasks before child creation;
  - derive project root by ascending from `import.meta.url`, never from caller input;
  - map reviewer kind to the exact three `.pi/skills/*.md` paths, parse with Pi's frontmatter helper,
    and fail before spawn with the ADR-required enable-agent plus `awf sync` message;
  - append fixed role policy to the dynamic task, and for review append the rendered reviewer body;
  - use `ctx.model.provider/id` and `pi.getThinkingLevel()`;
  - explore/review use exactly `read,grep,find,ls,bash`; implement uses exactly
    `read,bash,edit,write,grep,find,ls`; no child custom tool is named;
  - serialize implementation calls through one promise queue and check abort before a queued call
    starts;
  - use `pi.exec("git", ...)` for before/after HEAD and porcelain status snapshots; non-git is a
    structured unavailable snapshot; compare HEAD when `allowCommits` is false and return a policy
    violation without rollback;
  - state in `subagent_implement` metadata that it must be the only tool call in its parent batch.

- [ ] **Task 2.6: Replace the smoke fixture with full deterministic tests.** Delete the Phase 1
  fixture. Expand `tsconfig.json` to include the dogfooded `.pi/extensions/awf-subagents/*.ts` and
  `tests/**/*.ts`. Add `index.test.ts` and `runner.test.ts` covering every branch named by ADR-0123:
  three registrations and strict schemas; unsupported/unparseable version; all reviewer mappings and
  missing files; model/thinking inheritance; exact allowlists and recursive-tool absence; task
  validation; role prompts; queue ordering and queued abort; git/non-git snapshots; allowed,
  forbidden, and violating commit states; JSON fragmentation; malformed events; usage; every cap and
  truncation marker; spawn/exit/stop failures; temp permissions and cleanup; ordinary completion,
  pre-abort, TERM completion, and TERM-resistant KILL with listener/timer cleanup. Fake Pi scripts
  emit recorded JSON and never call a provider. Put proof markers for
  `pi-subagent-public-contract`, `pi-child-tool-boundaries`, `pi-child-process-safety`,
  `pi-implementation-state-boundary`, and `pi-minimum-runtime` on the Go tests that own these
  generated/runtime contracts; TypeScript tests remain behavioral coverage, not invariant markers.

- [ ] **Task 2.7: Dogfood, document the new output, and commit.** Update architecture and
  rendering/tooling current-state authored parts for `Target.Outputs`, TypeScript provenance,
  config-hash ownership, subprocess shape, trust boundary, Pi 0.80.9 minimum, role permissions,
  same-checkout behavior, and test container. Update README's target paragraph: Pi now receives the
  three generated delegation tools rather than generic-only delegation. Run `./x sync`; inspect both
  repository and Sundial extension files and lock entries. Run `./x check`, `./x gate`, and
  `./x pi-test stop && ./x gate` to cover restart. Stage the Go/render changes, templates, tests,
  dogfood files/locks, README, and authored/generated docs. Commit:

  ```commit
  feat(rendering): ship Pi workflow subagent tools
  ```

## Phase 3: Explicit Pi workflow dispatch

- [ ] **Task 3.1: Add failing cross-target dispatch tests.** Replace
  `TestPiReviewDispatchUsesGenericRuntimeWording` with proof marker
  `// invariant: pi-explicit-workflow-dispatch` and table assertions over all affected skills. Pi
  copies must contain the exact tool names and role arguments below; Claude, Codex, Copilot, Cursor,
  and Gemini copies must contain their existing native-subagent instruction and none of the three Pi
  tool names. Extend `internal/evals` so the Pi reviewing-skill-to-reviewer edge remains connected and
  the tool wording occurs on the actual invocation step, not incidental prose.

- [ ] **Task 3.2: Bind exploration dispatch sites as one batch transformation.** In
  `templates/skills/brainstorming/SKILL.md.tmpl`, the representative site is step 6: when
  `.targetSubagentTools`, say `Call subagent_explore once with the self-contained grounding brief in
  task`; preserve the existing neutral/native sentence in the else branch. The edge site is
  `refactor-coupling-audit`: its large-scope branch says to call `subagent_explore` once with the six
  audit categories and required output in `task`. Affected-site set is exactly:

  ```bash
  rg -l 'fresh-context (exploration )?subagent' templates/skills/{brainstorming,refactor-coupling-audit}/SKILL.md.tmpl
  ```

  Expected two paths. Post-check: the Phase 3 table test passes and empty-var render sweeps remain
  coherent.

- [ ] **Task 3.3: Bind governed review sites as one batch transformation.** In each reviewing
  template's initial dispatch and verify-pass instruction, branch on `.targetSubagentTools` and call
  `subagent_review` with `kind: "adr"`, `"plan"`, or `"code"` plus the existing self-contained brief
  as `task`. Plan resync uses kind `plan` and keeps resync-mode constraints in the task. The exhaustive
  set is the four reviewing templates listed in File structure; `rg -l 'Dispatch.*reviewer|verify
  pass'` over that set must print exactly four paths. Preserve report-only, one-verify-pass, and
  classification routing text.

- [ ] **Task 3.4: Bind subagent-driven development.** In step 4 of
  `subagent-driven-development/SKILL.md.tmpl`, Pi calls `subagent_implement` with the assembled task
  and an explicit `allowCommits` chosen from the plan task; it issues that call alone in the parent
  tool batch and stays sequential. Step 6 calls `subagent_review` with `kind: "code"` and the task
  requirements plus just-created SHA(s). The final-task wording keeps commit/status-flip ownership
  unchanged. Add golden assertions for `allowCommits`, alone-in-batch, sequential-only, and per-task
  code review.

- [ ] **Task 3.5: Remove the temporary generic fallback and update workflow docs.** Pi's review
  style no longer renders `available reviewer or delegation mechanism`. Keep the descriptor field
  only if another target still consumes generic style; otherwise remove `GenericReviewDispatch` and
  simplify the conditional without changing native target output. Update AGENTS identity source to
  state that Pi receives awf's extension-backed subprocess orchestration rather than having no
  orchestration. Update working/target guidance and README examples with the three tool contracts,
  trust boundary, minimum version, and `allowCommits` ownership. Update the roadmap ideas part to
  `No other roadmap ideas are recorded.` if the Pi orchestrator was its only entry; preserve the
  separate Pi/shared-skills collision deferred entry because ADR-0123 does not solve it.

- [ ] **Task 3.6: Sync, verify, and commit explicit dispatch.** Run:

  ```bash
  ./x sync
  ./x check
  ./x gate
  rg -n 'available reviewer or delegation mechanism' .pi/skills templates/skills
  rg -n 'subagent_(explore|review|implement)' .pi/skills
  ```

  Expected: sync/check/gate pass; the generic phrase has zero Pi/template matches; the tool search
  finds only the governed Pi dispatch sections and extension metadata. Inspect non-Pi render fixtures
  for unchanged native wording. Commit all workflow templates, tests, authored parts, generated Pi
  and Sundial skills, docs, and locks:

  ```commit
  feat(rendering): bind Pi skills to subagent tools
  ```

## Phase 4: Release surface and lifecycle freeze

- [ ] **Task 4.1: Complete adopter-facing release documentation.** Add an Unreleased Breaking changes
  entry to `changelog/CHANGELOG.md`: existing Pi adopters get executable project extension files on
  sync, Pi project trust applies, minimum Pi is 0.80.9, and `awf sync` repairs extension drift. Add a
  Features bullet describing the three roles, isolated no-session children, shared checkout, and
  Docker being only an awf-contributor gate dependency rather than an adopter runtime dependency.
  Ensure architecture, development, testing, working guidance, README, AGENTS identity, domain
  current-state docs, and roadmap all state current reality without the ADR-0122 deferral.

- [ ] **Task 4.2: Run the real-runtime smoke check.** With Pi 0.80.9 or newer, invoke each generated
  tool once in a disposable clean test repository: exploration reads one file; review uses one
  rendered reviewer and reports without edits; implementation writes a disposable file with
  `allowCommits: false` and leaves HEAD unchanged. Abort one long-running fake-command child and
  confirm the parent returns promptly with no child process. Record only deviations in this plan's
  Notes; credentials and transcripts are never committed. Expected: all four checks succeed.

- [ ] **Task 4.3: Freeze the design and execution record.** Change ADR-0123 from `Proposed` to
  `Implemented` and this plan from `Proposed` to `Implemented`. Run `./x sync` to regenerate
  `ACTIVE.md`, domain indexes, both dogfood trees, and locks. Run `./x check`, `./x gate`, and
  `./x audit-local main..HEAD`; expected no check/gate errors and no audit-local Error. Verify every
  ADR-0123 backed invariant has one matching marker under `**/*_test.go`, and the
  `pi-real-runtime-smoke` unbacked invariant has no proof marker.

- [ ] **Task 4.4: Commit the final lifecycle and release surface.** Stage only changelog, final docs,
  roadmap source/render, ADR, plan, generated indexes, dogfood outputs, and lock changes. Commit:

  ```commit
  feat(rendering): complete Pi subagent rollout
  ```

## Verification

- `./x check` reports modified/missing extension files and `./x sync` restores both; disabling Pi
  prunes both files while every non-Pi target remains extension-free.
- `./x gate` passes from a clean Docker state and on an already-running matching container; the
  second run performs no image build or container creation and creates no host `node_modules`.
- The containerized lane type-checks actual dogfooded output and reaches 100% line, function, and
  branch coverage over the extension core with fake children only.
- Pi 0.80.9 registers exactly the three public tools; an older/unparseable stub registers none and
  returns one actionable error.
- Child tool allowlists, output caps, cancellation, cleanup, serialization, git reporting, and commit
  policy match ADR-0123, with no recursive delegation tool active.
- Pi workflow skills invoke the exact role tool at every governed dispatch site; all other target
  copies retain native-subagent wording.
- The repository and Sundial dogfood the extension; generated files carry valid `//` provenance and
  ordinary manifest/config hashes.
- README, AGENTS, architecture, development, testing, domain docs, roadmap, and CHANGELOG describe
  the implemented behavior and no longer claim Pi orchestration is deferred.

## Notes

- The current separate roadmap item about Pi discovering both `.pi/skills` and shared
  `.agents/skills` when Pi and Codex targets coexist remains out of scope. The extension reads
  reviewer files by exact Pi path and does not solve duplicate workflow-skill discovery.
- Execute Phases 1-4 inline with `awf-executing-plans`: rendering, generated dogfood, the mandatory
  gate, and lifecycle flips are tightly ordered. Do not dispatch implementation phases in parallel.
