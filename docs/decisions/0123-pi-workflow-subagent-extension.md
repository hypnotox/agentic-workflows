---
status: Implemented
date: 2026-07-16
tags: [multi-target, target-seam, subagent-dispatch, review-agents, command-runner]
related: [37, 38, 74, 122, 125, 126]
domains: [rendering, tooling]
---
# ADR-0123: Pi Workflow Subagent Extension

## Context

ADR-0122 added Pi as a rendering target, emitted the three governed reviewer
agents as top-level Pi skills, and deliberately deferred orchestration. Its Pi
workflow copies use generic delegation prose because Pi has no built-in
subagent facility. A Pi adopter therefore receives the workflow policy and role
definitions but not the fresh-context execution mechanism that brainstorming,
review, and subagent-driven development require.

Pi extensions can register model-callable tools, and Pi's maintained subagent
example demonstrates isolated child execution by spawning `pi` in JSON mode.
The example is general-purpose: it discovers arbitrary agents and offers
single, parallel, and chained modes. awf needs a narrower system whose public
contracts encode its workflow roles, preserve report-only review, serialize
same-checkout implementation, and consume the reviewer bodies awf already
renders.

The extension is executable project-controlled code. Pi loads project
extensions only after project trust is established, but awf must still bound
child capabilities, avoid recursive delegation, handle cancellation and output
limits, and fail clearly when a reduced adopter configuration omits a selected
reviewer. Implementation children share the checkout because awf's
subagent-driven workflow is sequential and commit-oriented; automatic
worktrees or rollback would conflict with the orchestrator's ownership of task
and commit policy.

Shipping TypeScript also creates a production surface outside Go. String-only
render tests would not adequately verify subprocess, cancellation, and stream
handling, while requiring host npm installation would degrade awf's Go-first
local setup. The extension therefore needs a deterministic containerized test
lane with a low steady-state cost.

## Decision

1. Extend a target descriptor with optional project-extension outputs, each
   carrying its template, path, and source comment style. The Pi target renders
   `.pi/extensions/awf-subagents/index.ts` and `runner.ts` automatically; other
   targets render no such output. TypeScript uses `//` provenance comments.
   Extension descriptor data participates explicitly in config hashes, and the
   files join the normal planned-output, manifest drift, stale cleanup, sync,
   and uninstall ownership model. Every new extension and target-specific skill
   template preserves awf's `missingkey=zero` publication-safety contract: empty
   variables render coherent generic output and no unresolved-value token.
   This replaces the orchestration deferral in
   ADR-0122. `refines: ADR-0122#4`

2. Register exactly three workflow-focused public tools backed by one private
   runner. Every `task` is a required, non-empty string:
   - `subagent_explore {task: string}` for fresh-context investigation;
   - `subagent_review {kind: "adr"|"plan"|"code", task: string}` for governed
     report-only review, with `kind` required;
   - `subagent_implement {task: string, allowCommits: boolean}` for fresh-context
     implementation, with `allowCommits` required.
   The review kind is a stable closed enum mapped to the rendered
   `adr-reviewer`, `plan-reviewer`, and `code-reviewer` Pi skill bodies.
   Arbitrary project-local skills or agents are not inferred as reviewers. If
   the mapped reviewer is absent from a reduced agent set, invocation fails
   before spawning with an actionable error naming the missing file and
   directing the adopter to enable the matching agent and run `awf sync`; public
   contract tests cover all three missing-reviewer cases.

3. Adapt Pi's subprocess example rather than embedding in-process SDK sessions
   or depending on a separately installed package. Each call starts the current
   Pi executable in JSON mode with no persisted session, the parent session's
   provider/model and thinking level, an explicit role tool allowlist, and a
   mode-0600 temporary role prompt. Final output is capped at 50 KiB or 2,000
   lines, whichever comes first; stderr retains its last 50 KiB; progress and
   details retain the last 20 event summaries at no more than 2 KiB each. These
   limits are fixed rather than public configuration. Every truncation marker
   reports omitted bytes, lines, or events, and no full raw transcript is
   retained elsewhere. The runner propagates child diagnostics and usage,
   removes temporary state on every exit path, and escalates cancellation from
   TERM to KILL based on observed process exit rather than Node's signal-sent
   flag.
   Pi 0.80.9 is the initial minimum supported runtime; an older incompatible
   runtime receives one actionable startup error instead of partially
   registered tools.

4. Exploration and review children receive exactly `read`, `grep`, `find`,
   `ls`, and `bash`; their no-mutation contract is prompt policy rather than an
   OS-level filesystem sandbox. Implementation children receive exactly `read`,
   `bash`, `edit`, `write`, `grep`, `find`, and `ls`. Passing those closed
   built-in allowlists through `--tools` excludes all three subagent tools and
   every other extension tool from children, preventing recursive delegation.
   Implementation children run in the parent's checkout and serialize against
   other implementation calls. Pi workflow guidance requires an implementation
   call to be alone in its parent tool batch because the extension cannot
   serialize it against a sibling parent `edit`, `write`, unrestricted `bash`,
   or delegation call. The runner reports starting and ending HEAD plus
   dirty-state summaries. The orchestrator selects whether commits are allowed;
   a changed HEAD when `allowCommits` is false is reported as a policy violation
   and never auto-reverted. Non-git execution remains available with commit
   verification marked unavailable.

5. Pi-target workflow copies explicitly bind grounding checks and large coupling
   audits to `subagent_explore`, governed review and verify passes to
   `subagent_review`, per-task implementation to `subagent_implement`, and
   subagent-driven per-task code review to `subagent_review`. Other targets keep
   their existing native-subagent wording. This replaces ADR-0122's temporary
   generic Pi review dispatch. `refines: ADR-0122#3`
   `supersedes-invariant: ADR-0122#pi-generic-review-dispatch`

6. Add a mandatory Pi-extension lane to `./x gate`. Exact npm dependencies and
   TypeScript tooling are lockfile-pinned inside a digest-pinned minimal Docker
   image; no host Node, npm, or `node_modules` is required. A repository- and
   dependency-fingerprint-keyed named container remains running with the
   checkout bind-mounted and container-owned dependencies shadowing the bind,
   so normal edits need only `docker exec`. The manager recreates stale
   environments automatically, exposes stop/reset cleanup commands, prints
   timing, and runs identically from a clean state in CI.

7. Test the actual dogfooded generated extension through TypeScript type checks,
   dependency-injected unit tests, and a fake Pi JSON child executable, with no
   live model or credentials in the gate. Go tests cover target rendering,
   hashes, drift/sync repair, cleanup, public contracts, process boundaries,
   compatibility behavior, gate wiring, and cross-target workflow wording; they
   carry this ADR's proof markers because this repository scopes invariant
   backing to `**/*_test.go`. The awf repository and Sundial example render the
   extension. A real Pi child run is a documented manual smoke check, not a
   blocking test.

8. Implementation updates the generated AGENTS.md and its source identity part,
   architecture, development, testing, working-with-awf and target guidance,
   README, `CHANGELOG.md`, rendering/tooling current-state
   docs, and the completed roadmap entry in the same commits as behavior. The
   final ADR status change runs `./x sync` and commits regenerated `ACTIVE.md`
   and domain indexes. No `docs/decisions/README.md` index row is owed: it is a
   how-to guide and `ACTIVE.md` is the generated index, following ADR-0005.

## Invariants

- `invariant: pi-extension-target-render`: enabling Pi renders exactly the two
  governed extension files with valid TypeScript provenance and target-sensitive
  hashes; targets without Pi render neither file; ordinary check/sync and
  manifest cleanup semantics own both files.
- `invariant: pi-explicit-workflow-dispatch`: Pi workflow copies name the
  appropriate exploration, review, and implementation tools at every governed
  dispatch site while non-Pi copies retain their target-native wording.
- `invariant: pi-subagent-public-contract`: the generated extension exposes only
  `subagent_explore`, `subagent_review`, and `subagent_implement`, with the closed
  reviewer-kind mapping and public parameter shapes decided above.
- `invariant: pi-child-tool-boundaries`: children inherit model and thinking
  state, receive exactly their closed role allowlists, cannot recursively
  delegate, and enforce the fixed retained-output limits with explicit
  truncation diagnostics.
- `invariant: pi-child-process-safety`: every child exit path cleans temporary
  prompts and listeners, cancellation escalates against observed process exit,
  and child errors preserve bounded diagnostics.
- `invariant: pi-implementation-state-boundary`: implementation calls serialize,
  enforce the selected commit permission, and report git state without
  automatic rollback, including an explicit unverifiable state outside git.
- `invariant: pi-minimum-runtime`: the generated extension supports Pi 0.80.9 or
  newer and emits one actionable compatibility error on an older runtime.
- `unbacked-invariant: pi-real-runtime-smoke`: the containerized fixtures are the
  deterministic gate, while release readiness also includes one real child run
  on Pi 0.80.9 or newer. **Verify:** perform the documented real-Pi smoke check
  before release and record any compatibility finding in the release work.
- `invariant: pi-extension-container-gate`: the normal gate invokes the
  dependency-fingerprinted persistent Docker test environment without creating
  host Node/npm artifacts, and its explicit cleanup commands remain available.

## Consequences

- A Pi adopter receives a complete fresh-context awf workflow by enabling one
  target; no second awf toggle or separately installed Pi package is required.
- Pi's project-trust prompt now protects executable awf-generated code in
  addition to project instructions. Existing Pi adopters receive that extension
  on their next sync, so release notes and working guidance must call out the
  trust and minimum-version boundary.
- Reviewer policy keeps one authority: convention-part changes flow into the
  rendered reviewer body that the extension reads at invocation time.
- Child process startup costs more than an in-process SDK session but supplies a
  genuinely isolated context and reuses the adopter's Pi authentication and
  provider configuration.
- `bash` means exploration and review are report-only by enforced instructions,
  not by filesystem isolation. True portable sandboxing remains out of scope.
- Same-checkout implementation is simple and matches the workflow, but external
  processes and sibling parent mutations remain possible; explicit no-sibling
  guidance, serialized implementation calls, and before/after git reporting
  make that boundary visible rather than pretending it is transactional.
- Docker becomes a development prerequisite for the full gate. The persistent
  container, narrow build context, cached dependency image, and bind-mounted
  source minimize recurring cost while keeping npm off the host.
- The project gains pinned Node test dependencies and must deliberately update
  them and the minimum Pi version as the extension API evolves.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| One generic subagent tool | A role switch centralizes the API but gives weaker model steering and mixes incompatible permission contracts in one schema. |
| Three public tools with one shared runner | Chosen: explicit schemas and permission intent outweigh maintaining three public schemas and a broader tool API, while shared process machinery avoids implementation duplication. |
| In-process Pi SDK sessions | Lower startup overhead, but weaker process isolation and substantially more resource-loader, extension-recursion, and lifecycle coupling. |
| Depend on an external generic Pi package | Makes the workflow non-self-contained and introduces package/version coordination around awf-owned reviewer paths and semantics. |
| Temporary worktree per implementation child | Stronger checkout isolation, but adds branch, merge, conflict, and cleanup policy that contradicts the existing sequential per-task commit workflow. |
| Reviewer-only extension | Leaves brainstorming, coupling audits, and subagent-driven development without the fresh-context mechanism they require. |
| Host npm test lane | Simpler container plumbing, but imposes Node/npm state on every awf contributor and violates the agreed local-environment constraint. |

## Migration history

- 2026-07-16: retired invariant `ADR-0122#pi-generic-review-dispatch`; basis: encoded
