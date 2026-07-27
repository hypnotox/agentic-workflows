---
date: 2026-07-27
adrs: [165]
status: Proposed
---
# Plan: Request-oriented compact context projection

## Goal

Implement [ADR-0165](../decisions/0165-request-oriented-compact-context-projection.md): replace the expanded path census with a request-oriented impact report, add bounded detail facets and secure oversize delivery, make spill events observable through the repository runner, and update every managed context caller. Non-goals are a structured context format, query IDs, a `--paths` mode, path examples for large groups, or changes to `awf topic --coverage`.

## Architecture summary

`internal/project` continues to own snapshot-consistent context semantics. It will replace the flat request/path/topic projection with ordered request reports, directory censuses, equivalence groups, and globally deduplicated authority categories. Grouping is computed from the complete semantic identity required by ADR-0165 and is independent of selected facets; facets only reveal fields from that stable result. `internal/topic` gains optional claim `Summary:` metadata and the project projection derives a bounded fallback when metadata is absent.

`cmd/awf` remains responsible for selection and human rendering. Normal, uncovered, and static results render fully to bytes and pass through a new `internal/contextdelivery` boundary that either writes at most 8,192 bytes unchanged or securely spills the exact bytes outside the repository and emits the versioned two-line notice. JSON context output is removed.

The hand-written `x` runner captures `./awf context` output byte-for-byte, replays it unchanged, and asks a small repository-local Go helper to recognize and securely log only valid spill notices. Managed skill templates use purpose-specific facet sets and a shared spill-consumption partial. ADR-0165 operations apply in three declaration-ordered batches: operations 1-13 with the semantic/CLI transaction, operation 14 with wrapper observability, and operation 15 with managed callers. The first batch opens Implementing; the final batch immediately precedes Implemented.

## File structure

- **Created:** `internal/contextdelivery/delivery.go`, `internal/contextdelivery/delivery_test.go`; `internal/contextspill/log.go`, `internal/contextspill/log_test.go`; `cmd/contextspilllog/main.go`, `cmd/contextspilllog/main_test.go`; `internal/project/context_wrapper_test.go`; `templates/partials/context-spill.md`; this plan.
- **Modified:** `internal/topic/topic.go`, `internal/topic/topic_test.go`; `internal/project/context.go`, `context_paths.go`, `context_projection.go`, `context_artifacts.go`, `context_adr.go` and their same-basename `_test.go` files; `cmd/awf/context.go`, `context_test.go`, `dispatch.go`; `internal/clispec/clispec.go`, `clispec_test.go`; `internal/project/spine_test.go`, `example_wiring_test.go`; `x`, `.gitignore`; `README.md`, `changelog/CHANGELOG.md`; `.awf/parts/working-with-awf/commands.md`, `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/glossary/prepend.md`, `.awf/docs/parts/roadmap/ideas.md`, `.awf/docs/parts/testing/layout.md`, `.awf/domains/parts/tooling/current-state.md`; `.awf/topics/parts/tooling/context-and-topic/current-state.md`, `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`; `templates/docs/working-with-awf.md.tmpl`; the context-calling skill templates named in Phase 3; ADR-0165 and this plan; generated root and Sundial docs, skills, workflows, decision index, and lock files selected by `./x render`.
- **Deleted:** none. Obsolete JSON structs, tags, render branches, and JSON-focused tests are removed from the modified Go files rather than deleting a whole file.

## Phase 1: Replace the context result and cap delivery

This phase is one coupled commit. The project model, text renderer, CLI grammar, current-state claims, and JSON removal cannot be split into independently green public states: the old tests and claims require JSON and a path census, while the new command cannot truthfully land before the new semantic result and delivery boundary are reachable.

- [ ] **Task 1.1: Specify claim summaries and the request-oriented result before implementation.** In `internal/topic/topic_test.go`, add parser cases for absent and valid `Summary:`, duplicate or out-of-order metadata, blank or multiline values, exactly 160 Unicode code points, 161 code points, and non-ASCII code-point counting. Pin the canonical claim metadata order as `Summary:` before `Origin:`. In `internal/project/context_paths_test.go`, replace flat-census expectations with fixtures asserting:
  - every nonblank positional token retains its original display spelling and input order, including repeated equal tokens; normalization is lookup-only;
  - overlapping file/directory requests remain independent blocks and share globally deduplicated authority;
  - exact files and sorted Git selections remain individual entries, including a staged deletion classified `not-found` rather than fabricated into the staged census;
  - working directories census tracked-present and nonignored-untracked leaves, staged directories census stage-0 present/addition entries, generated/context-ignored leaves count as exclusions, and symlink/nested-adopter boundaries count once without descent;
  - empty or fully excluded directories remain visible with included and per-classification excluded counts;
  - included descendants group by the full, facet-independent semantic signature: classification, compact and expanded artifact provenance, domains, topics, direct rule IDs, applicable invariant/proof IDs, and warnings;
  - every group reports its member count, groups at or below three list all sorted members, and larger groups disclose none.

  In `internal/project/context_test.go`, add working/index divergence, no-planned-output, request-order, mixed-directory-over-20-files, static fallback, and existing classification-preservation fixtures. Tests must initially fail against the old model. Run `go test ./internal/topic ./internal/project -run 'Summary|ContextRequest|ContextGroup|ContextCensus|MixedDirectory|StaticContext'`; failure must be confined to the new expectations.

- [ ] **Task 1.2: Implement Summary parsing, census, and stable grouping.** In `internal/topic/topic.go`, add `Summary string` to `Claim`; recognize `Summary:` as reserved metadata; require it before `Origin:`, nonblank, single-line, and at most 160 Unicode code points. In `internal/project/context_paths.go`, replace `ContextProjection`, flat `ContextRequest.EffectivePaths`, `ContextPath`, and JSON-shaped structs with a human-contract-neutral model containing:
  - a closed `ContextFacet` enum in canonical order `all-rules`, `evidence`, `selectors`, `references`, `pending`, `artifacts`, plus validation and normalized-set helpers;
  - a `ContextOptions` carrying normalized facets and selection origin;
  - ordered request reports retaining raw display and normalized lookup values;
  - exact/Git path entries, directory census counts, and directory groups with optional disclosed members;
  - semantic path attribution and warnings sufficient to build a canonical comparison key without rendered text.

  Refactor `buildContextRequests`, `assembleContextUniverse`, and `classifyContextPath` across `internal/project/context.go` and `context_paths.go` so one selected snapshot supplies both included and excluded census data. Stop traversal at nested adopter and symlink boundaries; never infer absent generated files. Preserve classification precedence and safe glob-literal behavior. Build the grouping key from complete semantic identity before projection so `--show` and `--full` cannot change grouping. Preserve public `ContextFor`, staged-root, Git-selection, artifact, uncovered, and static entry points through options-based internal helpers rather than parallel concise/full implementations. Run `gofmt -w internal/topic internal/project && go test ./internal/topic ./internal/project`; all Task 1.1 tests and preserved snapshot/classification tests must pass.

- [ ] **Task 1.3: Specify authority categories and every facet.** Rewrite `internal/project/context_projection_test.go` around the closest-category order: directly related claims, applicable non-direct invariants, additional topic rules, referenced context, then pending changes. Assert a directly marker-selected invariant stays direct, a claim occupies only its closest category, and authority shared by request blocks renders once. Cover summary behavior exactly:
  - prefer valid `Claim.Summary`;
  - otherwise take the first prose paragraph, whitespace-fold it, and return it unchanged at 160 code points or fewer;
  - above the limit reserve three code points for ASCII `...`, cut at the last word boundary at or before 157, and hard-cut at 157 only when no boundary exists.

  Add independent and composed facet fixtures: `all-rules` adds summaries but not prose; `evidence` discloses backing/Verify and each state, touches, or proof kind completely only at three sites or fewer, otherwise only that kind's count; `selectors` adds domain/topic selector declarations and the both-must-match rule without paths; `references` expands incoming/outgoing edges from claims visible after non-reference facets by one level, retains direction on the origin, and deduplicates targets already closer; `pending` gives a default per-topic operation summary with at most three ADR IDs and expands remaining operations when selected; `artifacts` expands deterministic attribution without exposing a large group's members. Assert duplicate/reordered facets normalize canonically and `--full` equals the six-facet union byte-for-byte.

  In `internal/project/context_adr_test.go`, pin explicit ADR behavior: lifecycle summary always remains; `pending` adds every declaration and canonical progress; `evidence` adds only operation-linked current/removal history and bounded marker evidence; `artifacts` adds ADR attribution; `all-rules`, `selectors`, and `references` are no-ops. In `context_artifacts_test.go`, retain authority-derived matching, safe-match guards, compact provenance, expanded roles/edges/navigation, and grouping participation. Remove proof markers belonging to the two retired JSON parity claims. Run `go test ./internal/project -run 'ContextProjection|ContextFacet|ContextADR|ContextArtifact|ClaimSummary|Reference|Pending'`; the new tests must initially fail.

- [ ] **Task 1.4: Implement summary projection, relevance deduplication, facets, artifacts, and ADRs.** Rewrite `internal/project/context_projection.go` so topic impact starts with one topic summary, classifies every current claim once, derives deterministic one-line summaries, and projects only selected facet fields. Use marker kind, not one merged site slice, for bounded evidence. Expand reference edges only from claims visible after default plus non-reference facets, one hop, and never recursively process newly referenced claims. Keep pending operations topic-scoped and declaration/progress ordered. Adapt `context_artifacts.go` and `context_adr.go` to compact/default and expanded/facet forms without path-lookalike attribution or prose authority. Keep `awf topic --coverage` as the unbounded drilldown. Add `// invariant: tooling/context-and-topic:context-summary-projection` to the focused summary projection test. Run `gofmt -w internal/project && go test ./internal/project`; all semantic, artifact, ADR, existing read-only, and uncovered tests must pass.

- [ ] **Task 1.5: Specify exact capped delivery and the human-only CLI.** Create `internal/contextdelivery/delivery_test.go` against an injectable filesystem/writer environment. Assert unchanged direct output at exactly 8,192 bytes; spill at 8,193; exact mode 0600; complete spilled bytes; canonical, absolute, newline-free temporary paths outside the canonical repository; segment-aware rejection when temp equals or lies below the repository; and the exact notice bytes:

  ```text
  AWF_CONTEXT_SPILL_V1 bytes=<decimal> format=text
  <absolute canonical path>
  ```

  Cover create, permission, write, close, first-line, second-line, short-write, and final-newline failures. Every post-create failure must best-effort remove the spill and return the original failure even when removal also fails. Add `// invariant: tooling/context-and-topic:context-terminal-output-cap` to this suite.

  Rewrite `cmd/awf/context_test.go` and `internal/clispec/clispec_test.go` to assert `--json` is an ordinary unknown flag in normal and uncovered modes; repeatable `--show` accepts exactly the closed facets; unknown facets fail; `--show` and `--full` are rejected with `--uncovered`; `--full` is the normalized union; exact and Git-selection headers identify their origins; normal, uncovered, and static output all use the shared cap. Replace JSON parity tests rather than retaining hidden serialization fixtures. Run `go test ./internal/contextdelivery ./internal/clispec ./cmd/awf`; only the new behavior may fail before implementation.

- [ ] **Task 1.6: Implement delivery, CLI grammar, and human rendering.** Create `internal/contextdelivery/delivery.go` with `Deliver(rendered []byte, repositoryRoot string, stdout io.Writer) error` and test-only injectable seams. Canonicalize `os.TempDir()` and repository root before `os.CreateTemp`; reject unsafe containment/newlines; verify owner-only permissions; write/close the exact complete rendering before emitting the two notice lines; use complete-write loops; and clean up on every failed delivery path. Successful callers own deletion.

  In `internal/clispec/clispec.go`, remove `--json`, add repeatable `--show`, and document facets, grouping, the 8 KiB boundary, notice grammar, and caller deletion. In `cmd/awf/dispatch.go`, normalize repeated facet values and pass options. Rewrite `cmd/awf/context.go` so normal, uncovered, and static text render into `bytes.Buffer` through human-only renderers and then call `contextdelivery.Deliver`; remove JSON encoding, concise/full branches, and direct streaming. Render each request as `[n] <original argument>`, show directory census/groups and exact/Git entries under the request, and render globally deduplicated authority categories after requests. Do not emit examples, hidden paths, full claim prose, or an alternate structured form. Run `gofmt -w internal/contextdelivery internal/clispec cmd/awf && go test ./internal/contextdelivery ./internal/topic ./internal/project ./internal/clispec ./cmd/awf`; every focused test passes.

- [ ] **Task 1.7: Apply ADR operations 1-13, update active documentation, render, and commit.** In `.awf/topics/parts/tooling/context-and-topic/current-state.md`, mutate exactly the first thirteen ADR-0165 operations in declaration order: update `context-adr-operation-projection`, `context-applicability-navigation`, `context-default-excludes-history`, `context-concise-projection`, `context-full-authority-packet`, and `context-known-artifact-navigation`; remove `context-output-parity`; update `context-path-attribution`, `context-read-only`, and `context-static-fallback`; remove `uncovered-output-parity`; add test-backed `context-summary-projection` and `context-terminal-output-cap`. Preserve each update's `Origin`, existing `Revised-by` prefix, and backing mode while appending ADR-0165. New claims use `Origin: ADR-0165`, `Backing: test`, and the proof markers from Tasks 1.4-1.5.

  Update `.awf/domains/parts/tooling/current-state.md`, `README.md`, `.awf/parts/working-with-awf/commands.md`, `templates/docs/working-with-awf.md.tmpl`, `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/glossary/prepend.md`, `.awf/docs/parts/roadmap/ideas.md`, and `.awf/docs/parts/testing/layout.md` for the request/group/facet model, Summary metadata, JSON removal, direct-command spill ownership, and new delivery package. Do not describe the `./x context` advisory until Phase 2. Add an Unreleased breaking entry for JSON removal and feature entries for compact reports and capped delivery in `changelog/CHANGELOG.md`.

  In ADR-0165, change status to Implementing and append the digest-bearing Implementing event immediately followed by one Applied event containing operations 1-13 exactly in declaration order and the next state sequence reported by `./awf check --staged`; never hard-code the authoring-time candidate. Run `./x render && ./x check`, stage exact authored/code/test/generated paths, run `./awf check --staged`, then `./x gate`; every command exits zero. Commit:

  ```commit
  feat(tooling): compact context impact reports (applies 0165)
  ```

## Phase 2: Observe spills through the repository runner

- [ ] **Task 2.1: Specify secure recognition, logging, and runner behavior.** Add `internal/contextspill/log_test.go` for an exact two-line parser that accepts only the reserved prefix, decimal bytes, closed `format=text`, one absolute canonical newline-free path, and one final newline. It must reject near misses without logging. For a valid notice, test UTC RFC3339Nano timestamp, declared byte count, and shell-escaped `./x context` argv, with no spill path in the record. Test `.awf/local` mode 0700, log mode 0600, no-follow component checks, current-owner checks, `flock`-serialized append, non-interleaved concurrent records, and fsync/close/error preservation. Add `// invariant: tooling/context-and-topic:context-spill-observability` to this focused suite.

  Add `cmd/contextspilllog/main_test.go` for the private helper grammar `contextspilllog --root <root> --notice-file <capture> -- <invocation...>`: valid notices log, non-notices exit zero silently, and security/logging errors exit nonzero with no stdout. Add `internal/project/context_wrapper_test.go` with a copied `x` and fake sibling `awf`; assert normal and spill stdout are byte-identical, including final newlines; the child status is preserved; a valid spill logs without its path; near misses do not log; logging failure warns on stderr but does not change a successful context result; concurrent calls do not interleave; and `./x check` emits a stderr advisory only while a nonempty log exists. Extend `internal/project/example_wiring_test.go` to pin `context` in the runner usage/delegation and the non-failing check advisory.

- [ ] **Task 2.2: Implement the private logger and byte-preserving runner arm.** Create `internal/contextspill/log.go` with exact notice parsing and a Linux/Unix owner-only no-follow locked append modeled on the repository's existing secure resident-file primitives. Create `cmd/contextspilllog/main.go` as the only production caller. It reads the capture file, returns success for non-notices, and logs recognized notices without deleting the spill.

  In `x`, add `context)` and its usage token. Use `mktemp` plus a trap, redirect `./awf context "$@"` into the capture without command substitution, save its status, replay the capture with `cat`, and invoke `go run ./cmd/contextspilllog --root "$PWD" --notice-file "$capture" -- ./x context "$@"` only after a successful child result. A logger failure prints a warning to stderr and leaves the replayed stdout/status unchanged. In `check)`, emit a non-failing stderr advisory when `.awf/local/context-spills.log` is a nonempty safe regular file; tell the operator to resolve or promote the issue and remove the log. Add `.awf/local/` to `.gitignore`. Run `gofmt -w internal/contextspill cmd/contextspilllog internal/project && go test ./internal/contextspill ./cmd/contextspilllog ./internal/project`; all security, concurrency, and wiring tests pass.

- [ ] **Task 2.3: Apply operation 14, document local observability, render, and commit.** Add `tooling/context-and-topic:context-spill-observability` to `.awf/topics/parts/tooling/context-and-topic/current-state.md` with the exact wrapper/log/advisory contract, `Origin: ADR-0165`, `Backing: test`, and the Task 2.1 proof marker. Append one ADR Applied event containing exactly `add tooling/context-and-topic:context-spill-observability` with the checker-reported next sequence. Update `README.md`, `.awf/parts/working-with-awf/commands.md`, `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/testing/layout.md`, `.awf/domains/parts/tooling/current-state.md`, and `changelog/CHANGELOG.md` for `./x context`, the ignored advisory log, warning-only degradation, operator-owned removal, and the private helper. Run `./x render && ./x check`, stage explicitly, run `./awf check --staged` and `./x gate`; every command exits zero. Commit:

  ```commit
  feat(tooling): observe repository context spills (applies 0165)
  ```

## Phase 3: Move managed callers to bounded facets

- [ ] **Task 3.1: Pin caller policy and shared spill consumption.** Add `templates/partials/context-spill.md` with runtime-neutral instructions that detect only the exact two-line notice, read the named file, verify its byte length against the descriptor before treating it as the packet, and best-effort delete it after both successful and failed packet use. The partial must not prescribe a runtime-specific file tool.

  Rewrite `internal/project/spine_test.go`'s managed-context projection test to expand includes and classify every template containing `awf context`. Pin these exact policies:
  - `brainstorming`, `bugfix`, `debugging`, `executing-plans`, `subagent-driven-development`, `tdd`, `refactor-coupling-audit`, and `writing-plans` use bare context;
  - `reviewing-plan` and `reviewing-impl` use `--show all-rules --show evidence --show pending`;
  - `reviewing-plan-resync` uses `--show all-rules --show pending`;
  - `adr-lifecycle` uses `--show pending` where lifecycle detail is needed;
  - no managed template prescribes `--full` or `--json`;
  - every context-calling template expands the exact spill-consumption and cleanup contract.

  Run `go test ./internal/project -run ManagedContextCallers`; it must fail until Task 3.2 changes the templates.

- [ ] **Task 3.2: Batch-update authored skill templates and generated targets.** Apply the policy from Task 3.1 to this exhaustive affected-site set: `templates/skills/brainstorming/SKILL.md.tmpl`, `bugfix/SKILL.md.tmpl`, `debugging/SKILL.md.tmpl`, `executing-plans/SKILL.md.tmpl`, `refactor-coupling-audit/SKILL.md.tmpl`, `subagent-driven-development/SKILL.md.tmpl`, `tdd/SKILL.md.tmpl`, `writing-plans/SKILL.md.tmpl`, `reviewing-impl/SKILL.md.tmpl`, `reviewing-plan/SKILL.md.tmpl`, `reviewing-plan-resync/SKILL.md.tmpl`, and `adr-lifecycle/SKILL.md.tmpl`. Include `context-spill` at each call site and retain dispatch ownership of resolved arguments. Do not add context grounding to skills that do not already have a resolved path set.

  Update `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md` claim `implementer-context-grounding` to cover bare implementation/orientation calls, review facet sets, ADR pending detail, the `--full` ban, and spill consumption. Preserve `Origin: ADR-0155`, append `Revised-by: ADR-0165`, retain `Backing: test`, and keep its proof in `internal/project/spine_test.go`. Run `./x render`; generated root `.claude/skills/awf-*/SKILL.md` and `.pi/awf-workflows/*.md`, Sundial `.agents/.claude/.cursor/.gemini/.github` skill copies and `.pi/awf-workflows/*.md`, rendered topic/docs outputs, and both locks must change only through rendering. The affected generated set is exactly the output of `git diff --name-only` after starting from a clean Phase 2 tree and running this task's authored edits plus `./x render`; inspect every path and reject unrelated drift.

- [ ] **Task 3.3: Apply operation 15, close ADR-0165, and commit.** Append one Applied event containing exactly `update rendering/workflow-skill-templates:implementer-context-grounding` with the checker-reported next sequence, then append the digest-bearing Implemented event immediately after it and set ADR-0165 status to Implemented. Keep this plan Proposed through implementation review. Add the managed-caller policy to the Unreleased changelog. Run `go test ./internal/project ./internal/evals`, `./x render && ./x check`, and searches that return no active managed-template matches for `awf context --full` or `awf context .*--json`. Stage exact authored/generated paths, run `./awf check --staged` and `./x gate`; every command exits zero. Commit:

  ```commit
  feat(rendering): bound managed context packets (implements 0165)
  ```

## Phase 4: Review and freeze the implementation record

- [ ] **Task 4.1: Audit and review the concrete implementation range.** From a clean tree, resolve `<implementation-base>` as the parent of the Phase 1 commit and `<implementation-head>` as `HEAD`. Run `./awf audit <implementation-base>..<implementation-head>` and `./x audit-local <implementation-base>..<implementation-head>`; both finish without Error findings. Run `./x context --show all-rules --show evidence --show pending $(git diff --name-only <implementation-base>..<implementation-head>)`; if it spills, read and byte-verify the packet and remove it after use. Invoke `subagent_review` with kind `code`, the concrete SHA range, ADR-0165, this plan, and the packet. Require review of snapshot isolation, census boundaries, stable grouping, relevance precedence, Unicode summary limits, bounded evidence/references/pending, artifact and ADR semantics, JSON removal, spill security/cleanup, runner byte preservation and log races, generated caller parity, state-operation order, and documentation.

- [ ] **Task 4.2: Resolve findings and freeze the accepted plan.** Apply mechanical and authority-determined fixes in new focused commits; stop for user approval if a finding changes ADR-0165. Repeat the fresh code review once after fixes and require zero findings. Record concrete implementation commit IDs and final audit/review results under Notes, set this plan's frontmatter to `status: Implemented`, and run `./x render && ./x check`. Stage the plan and changed generated records explicitly, run `./awf check --staged` and `./x gate`; every command exits zero. Commit:

  ```commit
  docs(plans): implement compact context projection
  ```

## Verification

- [ ] Exact files and Git selections remain path-specific; directory output scales with distinct semantic groups, never includes a hidden member census, and discloses members only through the three-file boundary.
- [ ] Default authority contains topic, invariant, and direct-rule summaries only; facets compose deterministically without changing grouping or relevance, and `--full` equals their union.
- [ ] Normal, uncovered, and static output write exact bytes through 8,192 and securely spill above it with the exact notice; every failed delivery removes partial output best-effort and preserves the primary error.
- [ ] `./x context` preserves stdout/status, logs only exact spill notices without paths under owner-only locking, and `./x check` reports a non-failing advisory for a nonempty log.
- [ ] Managed callers use only the approved bare/facet forms, consume and remove spills, and generated root/Sundial copies match authored templates.
- [ ] ADR-0165 operations apply in exact declaration order with matching claim transactions; removed JSON parity claims and proof markers are absent.
- [ ] `go test ./...`, `./x render`, `./x check`, staged check, `./x gate`, both audits, and final code review finish cleanly.

## Notes

- ADR-0165 deliberately stays Proposed during plan authoring and review. Its first implementation transaction opens Implementing; the final operation transaction closes it as Implemented.
- Empty-directory reporting is snapshot-observable: a directory prefix with only excluded descendants remains visible, while an untracked empty filesystem directory absent from the selected snapshot cannot enter the census.
- The private `cmd/contextspilllog` helper exists to obtain no-follow, ownership, flock, and exact-byte guarantees that Bash redirection cannot provide without a symlink race. It is repository-runner infrastructure, not a public `awf` subcommand.
