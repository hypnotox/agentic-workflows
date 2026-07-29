---
date: 2026-07-29
adrs: [173]
status: Proposed
---
# Plan: Deliberate subagent model selection

## Goal

Implement [ADR-0173](../decisions/0173-deliberate-subagent-model-selection.md) in its declared
three-batch order: publish cross-runtime deliberate-selection guidance, extend Pi's model
preferences and wizard with semantic tiers and strict model-reference handling, then add the
per-run routing card and bounded model-routing module with real-runtime proof. Each governed
dispatch will choose the smallest model expected to complete reliably while preserving Pi's useful
role defaults.

Non-goals: no price or registry catalog in durable guidance or the routing card, no compatibility
sidecar or schema migration, no new cross-runtime capability detector, no concurrent implementation
mutators, and no change to ADR-0166's phase transaction ownership.

## Architecture summary

The implementation follows ADR-0173's operation order as three independently green transactions.
Phase 1 changes the generic workflow contract and every final governed dispatch occurrence after
ADR-0166, adds the backed cross-runtime claim, and regenerates the agent guide and all target skill
outputs. Phase 2 evolves the existing Pi preference schema in place, makes merged role and tier
state complete or visibly incomplete, enforces bounded exact references and post-queue refresh,
and extends the atomic wizard; it applies the three Pi-workflow claim updates together. Phase 3
extracts preference parsing, merging, validation-state representation, and card construction into a
pure sibling module, wires `before_agent_start`, extends the Pi output descriptor, and proves the
card through the pinned Pi runtime before applying the two Pi-runtime updates and freezing the ADR
and plan.

The generated entrypoint retains tool registration, queueing, child process lifecycle, renderers,
and Pi integration. `model-routing.ts` owns bounded pure routing policy; `runner.ts` remains the
child-process boundary. Authored templates, current-state parts, tests, ADR lifecycle events, and
rendered outputs travel in the same batch. Generated files are never edited directly.

Every repository path below is relative to `/home/hypno/Projects/agentic-workflows/`. Each
subagent-driven phase starts only from the declared clean and green baseline and is owned by one
commit-capable phase implementer. If grounding reveals that an already-declared behavior is owned
by an omitted authoritative source, the parent may amend the plan with that exact source and its
deterministic rendered or lock consequences without another design decision; this source-closure
rule may not add behavior, alter an ownership boundary, or broaden the phase beyond those direct
consequences.

## File structure

- **Created:** `templates/pi/awf-subagents/model-routing.ts.tmpl`,
  `tools/pi-extension-test/tests/index.test.ts`, `tools/pi-extension-test/tests/runtime.test.ts`,
  and `internal/project/subagent_model_selection_test.go`.
- **Modified authored workflow sources:** `templates/agents-doc/AGENTS.md.tmpl`,
  `templates/docs/working-with-awf.md.tmpl`,
  `.awf/parts/working-with-awf/config-and-overrides.md`, and
  `templates/skills/{brainstorming,executing-plans,exploring,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,subagent-driven-development}/SKILL.md.tmpl`.
- **Modified Pi implementation and test plumbing:** `templates/pi/awf-subagents/index.ts.tmpl`,
  `templates/partials/pi-minimum-runtime.md`, `internal/project/target.go`,
  `internal/project/target_test.go`,
  `internal/project/output_plan_test.go`, `internal/project/project_test.go`,
  `internal/evals/chain_test.go` and `tools/pi-extension-test/container.sh`.
- **Modified current state and lifecycle records:**
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`,
  `.awf/topics/parts/rendering/pi-workflows/current-state.md`,
  `.awf/topics/parts/rendering/pi-runtime/current-state.md`,
  `docs/decisions/0173-deliberate-subagent-model-selection.md`, this plan,
  `docs/decisions/INDEX.md`, `changelog/CHANGELOG.md`, and `.awf/awf.lock`.
- **Generated root outputs:** `AGENTS.md`, `docs/working-with-awf.md`,
  `docs/topics/rendering/{workflow-skill-templates,pi-workflows,pi-runtime}.md`,
  `docs/domains/rendering.md`, `.pi/extensions/awf-subagents/{index,model-routing}.ts`,
  `.pi/extensions/awf-handoff/index.ts`, and
  enabled target copies of the eight changed skills under `.agents`, `.claude`, `.cursor`,
  `.gemini`, `.github`, and `.pi`.
- **Generated Sundial outputs:** `examples/sundial/AGENTS.md`,
  `examples/sundial/docs/working-with-awf.md`,
  `examples/sundial/.pi/extensions/awf-subagents/{index,model-routing}.ts`,
  `examples/sundial/.pi/extensions/awf-handoff/index.ts`, the enabled
  Sundial target copies of the eight changed skills, and `examples/sundial/.awf/awf.lock`.
- **Deleted:** none.

If `./x render` produces an additional path solely from an authored input listed above, add that
exact path to this Proposed plan before staging. Do not replace the inventory with a catch-all.

## Phase 1: publish the cross-runtime dispatch contract

**Execution mode: subagent-driven.** The parent runs `git status --short` and requires no output,
then runs `./x gate` and requires success. One commit-capable implementer owns this complete batch,
including the operation, generated outputs, staged check, gate, and closing commit.

- [ ] **Task 1.1: Add the failing cross-runtime proof in
  `internal/project/subagent_model_selection_test.go`.** Create
  `TestDeliberateSubagentModelSelectionAcrossGovernedDispatches` with the marker
  `// invariant: rendering/workflow-skill-templates:deliberate-subagent-model-selection` immediately
  above it. Render the complete enabled catalog for every `KnownTargets()` target and render the
  affected templates once more with empty vars/data under `missingkey=zero`. Enumerate these
  authored dispatch sites rather than searching by a stale count:

  - grounding: `brainstorming`;
  - exploration: `exploring`;
  - inline optional batch helper: `executing-plans`;
  - complete implementation phase and its report-only phase review:
    `subagent-driven-development`;
  - primary and verify review: `reviewing-adr`, `reviewing-plan`,
    `reviewing-plan-resync`, and `reviewing-impl`.

  At each primary and verify/helper occurrence, assert the text selects the smallest reliable tier,
  defines `small`, `standard`, and `large`, and requires reconsideration after uncertainty, failed
  reasoning, or widened scope. Pi branches must say to omit `model` for configured role routing and
  to override only with the exact `provider/model-id` mapped to the selected tier; they must state
  that `model` is absent rather than showing `model: "default"`, `model: "auto"`, or
  `model: "inherit parent"`. Non-Pi branches must require target-native explicit selection where
  supported and a visible note that selection is unavailable when the harness cannot select.
  Assert generic outputs and `AGENTS.md` contain no `subagent_grounding`, `subagent_explore`,
  `subagent_review`, `subagent_implement`, Luna/Terra/Sol reference, price, context limit, or Pi
  registry catalog. Empty renders must contain none of `<no value>`, unresolved-value tokens, empty
  inline code spans, or dangling selection sentences. Run
  `go test ./internal/project -run TestDeliberateSubagentModelSelectionAcrossGovernedDispatches`;
  before Task 1.2 it must fail on missing deliberate-selection clauses.

- [ ] **Task 1.2: Update the authored agent-guide and documentation convention.** In the workflow
  section of `templates/agents-doc/AGENTS.md.tmpl`, add one provider-neutral paragraph after the
  enabled-skill listing and before Conventional Commits. It must say: every governed subagent
  dispatch chooses the smallest model expected to complete reliably; small is narrow, mechanical,
  and low ambiguity; standard is substantive but bounded; large is broad, intricate, cross-cutting,
  or high consequence; uncertainty, failed reasoning, or widened scope triggers reconsideration and
  possible escalation. It must also state that a runtime with model selection chooses explicitly,
  while an unsupported runtime uses its harness default and notes that explicit selection is
  unavailable. Do not name a Pi tool or provider model.

  Add the same target-neutral convention to the workflow-subagents section of
  `templates/docs/working-with-awf.md.tmpl`, followed by a Pi-only sentence saying omission uses the
  configured role default and an exact tier reference is supplied only for a deliberate override.
  The root adopter replaces that template section, so add the same convention and Pi-only sentence
  to `.awf/parts/working-with-awf/config-and-overrides.md` after its phase-ownership paragraph; this
  part is the authored source for `docs/working-with-awf.md`, while the template remains the authored
  source for Sundial and generic adopters. Preserve existing phase ownership and preference-file
  documentation.

- [ ] **Task 1.3: Update every final governed dispatch occurrence in the eight skill templates.** Use
  one shared qualifying wording shape, specialized only for the local role and occurrence. For a Pi
  branch, state `Omit the model field entirely to use configured role routing; when the selected
  complexity warrants an override, pass the tier's exact provider/model-id. Never pass default,
  auto, or inherit parent as a model value.` For a non-Pi branch, state `Select the smallest reliable
  target-native model explicitly; if this harness cannot select a model, use its default and note in
  the dispatch brief that explicit selection is unavailable.` Place this rule on each actual call or
  dispatch sentence, including the four verify-pass sentences, the optional commit-disabled helper
  sentence in `executing-plans`, and the report-only phase-review dispatch in
  `subagent-driven-development`; a rule elsewhere in the file does not satisfy an occurrence.

  Preserve the exact role-specific arguments and ownership: grounding is one no-mutation call;
  exploration retains task/breadth/detail; the phase owner retains `allowCommits: true` and solo
  tool batching; batch helpers retain sequential `allowCommits: false`, path confinement, and no
  phase commit; the phase review and dedicated review skills retain `kind: adr|plan|code`,
  report-only behavior, and exactly one verify pass. Generic branches must not acquire Pi syntax.

- [ ] **Task 1.4: Update existing render and eval assertions.** In
  `internal/project/target_test.go`, replace the old `provider/model-id` plus `inherits the parent`
  sweep in `TestCrossRuntimeExplorationDispatch` with assertions delegated to the new exhaustive
  proof while retaining exploration's Pi/non-Pi capability and leakage checks. In
  `internal/evals/chain_test.go`, make the reviewer dispatch table inspect both the primary and
  verify occurrence for each review skill and require deliberate selection at both without changing
  invocation-graph semantics. Keep ADR-0166's phase and helper assertions green; do not add a
  task-level dispatch or checkpoint.

- [ ] **Task 1.5: Add and apply the first current-state operation.** In
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, add this claim with its
  proof marker already present:

  ```text
  ### `invariant: deliberate-subagent-model-selection`

  Every final governed subagent dispatch chooses the smallest model expected to complete reliably from the semantic small, standard, and large tiers and reconsiders escalation after uncertainty, failed reasoning, or widened scope. Pi uses configured role routing only by omitting the model field and overrides deliberately with an exact tier reference; other targets select a target-native model explicitly where supported and otherwise use the harness default with a visible unsupported-selection note. Generic rendered guidance contains no Pi tool name, provider-specific model reference, price, context limit, or registry catalog, and every affected template renders coherently with empty variables.
  Origin: ADR-0173
  Backing: test
  ```

  Change ADR-0173 to `Implementing`, append its checker-derived `Implementing; content-sha256:`
  event, and append one Applied event with the next checker-reported global state sequence and
  exactly `add rendering/workflow-skill-templates:deliberate-subagent-model-selection`. Later
  operations remain Remaining. Never precompute the digest or sequence: stage, run
  `./awf check --staged`, and use the values it reports. Run `./x render` and inspect every generated
  change for provider/tool leakage and unrelated drift.

- [ ] **Phase-close: verify, stage, gate, and commit.** Run `gofmt -w
  internal/project/subagent_model_selection_test.go internal/project/target_test.go
  internal/evals/chain_test.go`, `go test ./internal/project ./internal/evals`, `./x render`,
  `./x check`, and `git diff --check`; all must pass and the final command prints no output. From
  the verified Phase 1 status inventory, run this exact staging command (brace expansion names only
  individual files, never a directory):

  ```sh
  git add -- templates/agents-doc/AGENTS.md.tmpl templates/docs/working-with-awf.md.tmpl .awf/parts/working-with-awf/config-and-overrides.md templates/skills/{brainstorming,executing-plans,exploring,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,subagent-driven-development}/SKILL.md.tmpl internal/project/subagent_model_selection_test.go internal/project/target_test.go internal/evals/chain_test.go .awf/topics/parts/rendering/workflow-skill-templates/current-state.md docs/decisions/0173-deliberate-subagent-model-selection.md .awf/awf.lock AGENTS.md docs/working-with-awf.md docs/topics/rendering/workflow-skill-templates.md docs/domains/rendering.md docs/decisions/INDEX.md .{claude,pi}/skills/awf-{brainstorming,executing-plans,exploring,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,subagent-driven-development}/SKILL.md examples/sundial/AGENTS.md examples/sundial/docs/working-with-awf.md examples/sundial/.awf/awf.lock examples/sundial/.{agents,claude,cursor,gemini,github,pi}/skills/sundial-{brainstorming,executing-plans,exploring,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,subagent-driven-development}/SKILL.md
  ```

  Require `git diff --cached --name-only` to equal the Phase 1 status inventory. Run `./awf check
  --staged` and require its clean terminal state, then run `./x gate` and require success. Commit:

```commit
feat(rendering): govern subagent model selection (applies 0173)
```

## Phase 2: evolve Pi preferences, routing, and wizard atomically

**Execution mode: subagent-driven.** From the clean Phase 1 tree, the parent runs `git status
--short` and requires no output, then runs `./x gate` and requires success. One commit-capable
implementer owns the preference, routing, wizard, claims, generated outputs, and one closing commit;
the three Pi-workflow operations may not split.

- [ ] **Task 2.1: Reintroduce the Pi entrypoint behavior harness before changing production.** Create
  `tools/pi-extension-test/tests/index.test.ts` using the current `handoff.test.ts` map-based
  tool/command/hook harness and the historical awf-subagents harness shape only as non-authoritative
  reference. Import the rendered `registerSubagentTools`, model schema, store, resolver, and preset.
  Provide scriptable preference-file reads/writes, registry registration/auth changes, queue release,
  active tools, session manager identity, notices, tool schemas, and runner calls.

  Add failing named tests for: absent, partial, complete, malformed, unauthenticated, unregistered,
  and later-unavailable preferences; project-over-global merge and tier precedence; shared default
  not satisfying an absent explicit role; one invalid tier blocking every omitted-model role while a
  valid explicit call remains usable; exact references at 256 characters accepted and 257 rejected;
  omission accepted; `default`, `auto`, and `inherit parent` rejected by all named tool schemas and
  by execute-time validation with an error ending in `Omit the model field to use configured or
  inherited routing.`; refresh after queue acquisition before runner start; and one incomplete-state
  notice per session-manager identity. Run `./x pi-test run`; it must fail on the unimplemented tier
  and strict-reference cases while the TypeScript compilation remains clean.

- [ ] **Task 2.2: Extend the in-place preference model and deterministic validation state in
  `templates/pi/awf-subagents/index.ts.tmpl`.** Add `PREFERENCE_TIERS = ["small", "standard",
  "large"] as const`, add those optional fields to `SubagentModelPreferences`, and make the legal key
  order exactly `default`, grounding, exploration, review, implementation, small, standard, large.
  Keep project-over-global precedence per field. Build an effective-state projection that records
  for every field its merged reference and source, `missing` in the same fixed order, and this exact
  bounded invalid union:

  ```ts
  type SourceScope = "global" | "project";
  type SourceReason = "read-error" | "malformed-json" | "non-object" | "unknown-key";
  type FieldReason = "malformed" | "overlong" | "unregistered" | "unauthenticated" | "unavailable";
  type InvalidState =
    | { kind: "source"; scope: SourceScope; reason: SourceReason }
    | { kind: "field"; scope: SourceScope; field: PreferenceField; reason: FieldReason };
  ```

  A source with any parse-shape failure contributes no values. Record at most one entry for each
  source reason, without the raw JSON error or unknown key. Sort invalid state by global then project
  scope; within a scope, source reasons use the declaration order above, followed by field failures
  in preference-field order. A role falling back to shared default remains routable but its role
  field remains missing for completeness.

  Replace `exactModelReference` with one parser used by preference files and explicit calls. It
  rejects non-strings, strings longer than 256 characters, a slash at either end, and the literal
  case-sensitive sentinels `default`, `auto`, and `inherit parent`; registry lookup remains the
  authority for the exact provider and remainder. Validation precedence is parse shape, overlong,
  registry `find` (unregistered), `hasConfiguredAuth` (unauthenticated), then membership in
  `getAvailable()` by provider and id (unavailable). Never truncate a reference. Missing entries are
  valid and non-blocking; any configured invalid field or source blocks all implicit routing,
  including when only a tier is invalid, while valid explicit calls remain usable.

- [ ] **Task 2.3: Make queue-time resolution a preflight and revalidate immediately before child
  startup.** Preserve the existing precedence for an omitted role request and the existing execution
  metadata sources. At execute entry, validate task, snapshot thinking, reload both preference files,
  validate the current registry, and resolve once for actionable preflight. For exploration, acquire
  the FIFO limiter; for implementation, await the serialized implementation tail. After acquisition
  and immediately before `runner.run`, reload and revalidate again and resolve from the original
  explicit-or-omitted request. Grounding and review perform the same final reload directly before
  runner startup. Publish queued metadata from preflight, but replace running/final metadata with the
  final resolution so diagnostics name the model actually started. A preference, auth, registration,
  or availability change while queued must fail before the runner and must not fall through.

  Replace the process-global `PREFERENCES_NOTICE` Symbol with a `WeakSet<object>` keyed by the active
  `ctx.sessionManager` object. Incomplete otherwise-valid state emits one concise notice per key;
  invalid state emits the existing strict error per key. Explicit valid calls remain usable. Do not
  suppress a notice in a later session within the same process.

- [ ] **Task 2.4: Enforce the same public contract in all four tool schemas and guidance.** Export
  and reuse this exact TypeBox field in grounding, exploration, review, and implementation:

  ```ts
  export const MODEL_REFERENCE_SCHEMA = Type.String({
    minLength: 3,
    maxLength: 256,
    pattern: "^[^/\\s]+/[^\\s]+$",
    description: "Exact provider/model-id. Omit the model field to use configured or inherited routing; default, auto, and inherit parent are invalid.",
  });
  ```

  Update each description and `promptGuidelines` to say omission is the only default form, the exact
  tier mapping is used for deliberate override, and the repair is `Omit the model field to use
  configured or inherited routing.` The runtime parser must repeat the check because a `tool_call`
  handler may mutate already-validated input.

- [ ] **Task 2.5: Extend `/awf-subagent-models` to one eight-field atomic transaction.** Keep the
  existing scope choice, current/error display, cancellation behavior, gitignore enforcement,
  owner-only preference write, sibling temporary rename, stale-writer check, cleanup, refresh, and
  live validation. Iterate slots in the fixed field order from Task 2.2. The effective preview must
  separately list exact role defaults, exact tier mappings, missing fields, and invalid fields.

  Extend `RECOMMENDED_PRESET` with exact entries `small:
  "openai-codex/gpt-5.6-luna"`, `standard: "openai-codex/gpt-5.6-terra"`, and `large:
  "openai-codex/gpt-5.6-sol"`; preserve the established five role/default mappings unchanged. Offer
  the preset only when every reference is registered and authenticated. Slot guidance defines the
  three semantic tier meanings without publishing them into generic target output. Cancellation at
  scope, state confirmation, mode, every slot, final confirmation, and ignore confirmation writes no
  preference file. The summary writes roles and tiers together; no partial schema transaction exists.

- [ ] **Task 2.6: Complete TypeScript and render coverage.** In `index.test.ts`, cover every branch
  from Tasks 2.1-2.5, including wizard preset eligibility, eight-slot manual flow, leave-unset,
  cancellation, project ignore decline/append/outside-worktree behavior, stale writer, mkdir/read/
  write/rename failures, temp cleanup, live post-save refresh, and the Luna/Terra/Sol mapping. Add
  named regression cases for malformed JSON, non-object JSON, unknown keys, read errors, and a source
  containing valid fields plus an invalid field. Each case asserts the bounded source/field reason,
  deterministic ordering, no raw error/key leakage, implicit-routing block, continued valid explicit
  call, and one notice per session identity. In `internal/project/target_test.go`, update the
  substantive Pi model preference, routing, and wizard render assertions to pin the tier fields,
  256-character boundary, omission repair, post-queue reload, session-scoped notice key, and exact
  preset mappings. Place each existing invariant marker immediately above a substantive test rather
  than leaving marker-only placeholders. The pinned SDK imports compile under the existing
  `tools/pi-extension-test/tsconfig.json`; do not modify that file, add a new rendered extension
  output, or change `internal/project/target.go` in this phase.

- [ ] **Task 2.7: Apply the three Pi-workflow operations together.** Replace the three claims in
  `.awf/topics/parts/rendering/pi-workflows/current-state.md` with these contract bodies, preserving
  each Origin and appending `Revised-by: ADR-0173`:

  ```text
  ### `invariant: pi-subagent-model-preferences`

  The generated Pi extension merges user-global and gitignored project-local preferences per field for the shared default, every grounding, exploration, review, and implementation role, and the small, standard, and large tiers. Completeness requires every field explicitly after merging; missing fields remain valid and visible, while any malformed, overlong, unregistered, unauthenticated, unavailable, or unreadable configured field blocks all implicit routing and leaves valid explicit calls usable. Preference and registry state reloads at preflight and again immediately before child startup.
  Origin: ADR-0151
  Revised-by: ADR-0173
  Backing: test

  ### `invariant: pi-subagent-model-routing`

  Every Pi subagent role accepts only omission or an exact registry-valid provider/model-id of at most 256 characters. Omission alone requests configured role routing and parent fallback; default, auto, inherit parent, and other sentinel values reject with an omit-the-field repair and are never normalized. Queue acquisition is followed by preference and registry revalidation immediately before child startup, failures never fall through, thinking remains inherited for child clamping, and diagnostics report requested, resolved, and actual models with routing source.
  Origin: ADR-0148
  Revised-by: ADR-0151, ADR-0173
  Backing: test

  ### `invariant: pi-subagent-model-wizard`

  The /awf-subagent-models command is a TUI-only atomic wizard for the shared default, all four explicit role defaults, and the small, standard, and large tiers. It preserves scope and error display, complete cancellation without writes, live registry-gated Luna/Terra/Sol preset selection, informed manual selectors, project-local gitignore enforcement, owner-only sibling-temp replacement, stale-writer detection, cleanup, and in-memory refresh, and it writes roles and tiers together as one preference transaction.
  Origin: ADR-0151
  Revised-by: ADR-0173
  Backing: test
  ```

  Append one ADR-0173 Applied event using the next checker-reported sequence and exactly these
  declaration-order operations: `update rendering/pi-workflows:pi-subagent-model-preferences`,
  `update rendering/pi-workflows:pi-subagent-model-routing`, `update
  rendering/pi-workflows:pi-subagent-model-wizard`. Keep the ADR Implementing and the two runtime
  operations Remaining. Run `./x render`.

- [ ] **Phase-close: verify, stage, gate, and commit.** Run
  `go test ./internal/project ./internal/evals`, `./x pi-test run`, `./x render`, `./x check`, and
  `git diff --check`; require passing tests, 100% configured Pi-extension coverage, clean drift, and
  no diff-check output. Run this exact staging command:

  ```sh
  git add -- templates/pi/awf-subagents/index.ts.tmpl tools/pi-extension-test/tests/index.test.ts internal/project/target_test.go .awf/topics/parts/rendering/pi-workflows/current-state.md docs/decisions/0173-deliberate-subagent-model-selection.md .awf/awf.lock .pi/extensions/awf-subagents/index.ts docs/topics/rendering/pi-workflows.md docs/domains/rendering.md docs/decisions/INDEX.md examples/sundial/.awf/awf.lock examples/sundial/.pi/extensions/awf-subagents/index.ts
  ```

  Require `git diff --cached --name-only` to equal the Phase 2 status inventory, then run `./awf
  check --staged` and `./x gate`; both must pass. Commit:

```commit
feat(rendering): add Pi subagent complexity tiers (applies 0173 batch)
```

### Phase 2 documentation settlement prerequisite

Phase 2 and its ADR operations are already committed and Applied, so this correction is recorded
forward rather than moved into or represented as part of that retained transaction. Before Phase 3
starts, update the adopter-facing preference paragraph in
`templates/docs/working-with-awf.md.tmpl` to reflect the already-applied Phase 2 contract: both
preference files accept the shared default, four explicit roles, and three semantic tiers;
completeness requires every field after project-over-global merging; references are exact
`provider/model-id` values of at most 256 characters; missing fields remain valid and visible;
malformed, overlong, unregistered, unauthenticated, unavailable, or unreadable configured state
blocks implicit routing while valid explicit calls remain usable; and preference plus registry state
reloads at preflight and immediately before child startup. Keep the existing wizard behavior and
generic dispatch guidance coherent. Run `./x render`, inspect deterministic consequences, and stage
only `templates/docs/working-with-awf.md.tmpl`, `.awf/awf.lock`,
`examples/sundial/docs/working-with-awf.md`, and `examples/sundial/.awf/awf.lock`; these are the
complete deterministic render consequences. Run `./awf check
--staged` and `./x gate`, then commit `docs(rendering): refresh Pi model preference guidance`.

## Phase 3: add the routing card, bounded module, and real-runtime proof

**Execution mode: subagent-driven.** From the clean Phase 2 tree, the parent runs `git status
--short` and requires no output, then runs `./x gate` and requires success. One commit-capable
implementer owns the extraction, runtime hook, output-plan change, smoke proof, final claim batch,
ADR implementation transition, and closing commit.

- [ ] **Task 3.1: Extract the bounded pure routing module with its consumer.** Create
  `templates/pi/awf-subagents/model-routing.ts.tmpl` with the provenance-compatible ts-nocheck line
  and move, without duplicating policy, the preference constants/types, exact-reference parser,
  source parser, project-over-global merge, completeness projection, bounded reason-code validation,
  preferred-role resolution, and preset data from `index.ts.tmpl`. Task 3.2 adds the new card builder
  directly to this consumed module. Export only the types and functions consumed by `index.ts` or
  direct tests. Keep filesystem access behind the entrypoint's injected dependencies and registry
  access behind a narrow `{find, hasConfiguredAuth, getAvailable}` interface. `index.ts` imports the module and retains registration, wizard UI,
  queueing, process lifecycle, and renderers. No dead copy of a moved function remains.

  In `internal/project/target.go`, add the target output descriptor
  `.pi/extensions/awf-subagents/model-routing.ts` from
  `pi/awf-subagents/model-routing.ts.tmpl`, with slash-comment provenance and the same explicit
  output policy as `index.ts` and `runner.ts`. Update `internal/project/target_test.go`,
  `internal/project/output_plan_test.go`, and `internal/project/project_test.go` so render, output
  attribution/config hash, and Pi-disable prune assertions enumerate the new path and still reject
  unrelated extension files. The resulting awf-subagents directory contains `index.ts`,
  `model-routing.ts`, and `runner.ts`; handoff remains the other Pi extension entrypoint.

- [ ] **Task 3.2: Build the deterministic bounded routing card.** In `model-routing.ts.tmpl`, export
  `buildRoutingCard(state)` using this exact complete representative fixture and separators:

  ```text
  [awf subagent routing]
  default: example/default
  roles: grounding=example/grounding; exploration=example/exploration; review=example/review; implementation=example/implementation
  tiers: small=example/small; standard=example/standard; large=example/large
  missing: none
  invalid: none
  selection: omit model for the role default; otherwise override deliberately with the selected tier's exact provider/model-id.
  ```

  Substitute the state's exact reference for each representative reference. When an explicit role
  mapping is absent but the shared default resolves that role, render the shared-default reference
  in the `roles:` entry while still listing the absent explicit role in `missing:`; use the literal
  `missing` only when no effective reference exists. A missing state replaces the `missing: none`
  line with, for example, exactly `missing: grounding, small`; fields are comma-space-separated in
  preference-field order. Add a fallback-card fixture proving the effective shared-default role
  reference and explicit-role missing marker coexist.
  An invalid state replaces `invalid: none` with semicolon-space-separated entries; exact
  representatives are `global:source:malformed-json`, `project:source:unknown-key`, and
  `project:small:unavailable`, ordered by Task 2.2. A mixed state carries both non-`none` lines. After a non-`none` invalid line, insert exactly
  `repair: Run /awf-subagent-models; omit model only after invalid preferences are repaired.` before
  `selection`. Never include raw parser, filesystem, registry, or auth errors, prices, limits, or a
  catalog; never truncate a reference.

  Measure `Buffer.byteLength(card, "utf8")`. The maximum-length complete normal form must remain at
  or below 4096 bytes. If an unexpected construction exceeds the budget, discard all mappings and
  return exactly:

  ```text
  [awf subagent routing]
  state: unavailable (routing card exceeded 4096 UTF-8 bytes)
  repair: Run /awf-subagent-models and retry; implicit routing remains strict.
  ```

  The caller warning is exactly `awf subagent routing card exceeded 4096 UTF-8 bytes; injected a
  failure card. Run /awf-subagent-models and retry.` Tests cover complete, missing, invalid, mixed,
  maximum-length, and defensive fixtures; they construct every field at the 256-character boundary
  to prove the normal form fits rather than relying only on the defensive branch.

- [ ] **Task 3.3: Wire one per-run `before_agent_start` injection with current state.** In the
  shared authoritative source `templates/partials/pi-minimum-runtime.md`, add `getActiveTools` to
  `MinimumRuntimeAPI` and its capability map; require it from `index.ts.tmpl`. Because the handoff
  entrypoint consumes the same partial, regenerate both root and Sundial handoff entrypoints and
  their already-inventoried locks without changing handoff behavior. Register one async
  `before_agent_start` handler in `index.ts.tmpl`. Determine the active awf tool set from
  `event.systemPromptOptions.selectedTools` when it is an array; otherwise use
  `pi.getActiveTools()`. If none of `subagent_grounding`, `subagent_explore`, `subagent_review`, or
  `subagent_implement` is active, return `undefined` and perform no notice. Otherwise reload both
  preference files, validate against the current registry, construct the card, and return only
  `{systemPrompt: event.systemPrompt + "\n\n" + card}`. Never return `message`, call
  `sendMessage`, or call `appendEntry`.

  The handler runs once per agent run, so it appends exactly one card to its received chained prompt;
  it does not retain a process-global injected flag. Incomplete valid state adds the missing line and
  uses the Phase 2 per-session notice tracker for one non-blocking notice. Invalid state adds bounded
  invalid fields and reason codes and preserves strict implicit-routing blocking. Defensive
  over-budget state returns the failure card and emits one actionable warning without weakening
  routing. Next-run preference and registry changes must appear without reload.

- [ ] **Task 3.4: Add pure, hook-level, and real Pi runtime tests.** Move pure policy cases from
  `index.test.ts` into direct imports from `model-routing.ts` where that gives a narrower seam; keep
  entrypoint tests for all tool schemas, queue ordering, wizard integration, active-tool fallback,
  notices, and hook registration. Add routing-card assertions for deterministic field order,
  complete/partial/invalid state, bounded reasons, no raw errors, maximum-length normal form,
  defensive failure card, exactly one append per handler call, no injection without an active awf
  tool, `selectedTools` precedence, `getActiveTools` fallback, and preference/registry refresh. Add
  an entrypoint regression test proving that a runtime missing `getActiveTools` registers no
  subagent tools and emits the bounded actionable incompatibility notice, plus a handoff assertion
  proving its required capability set and behavior remain unchanged despite consuming the shared
  partial. Update `tools/pi-extension-test/container.sh` so c8 includes
  `.pi/extensions/awf-subagents/model-routing.ts` at 100% while retaining runner and handoff coverage;
  do not include the integration-heavy ignored entrypoint in the metric.

  Create `tools/pi-extension-test/tests/runtime.test.ts` using the pinned coding-agent SDK, not a
  hand-called imitation of `before_agent_start`: construct `DefaultResourceLoader` with an inline
  extension factory that invokes the rendered `registerSubagentTools` with test dependencies;
  create an in-memory `SessionManager`, in-memory settings, and a registry-authenticated fake model
  provider whose stream captures the outbound model request and returns one terminal assistant
  message. Activate one awf subagent tool, call `session.prompt`, and assert the captured system
  prompt contains one complete routing card. Assert `session.messages` contains the user and
  assistant exchange but no routing-card text and no custom routing message. Repeat with no awf tool
  active and assert the captured request has no card. Dispose the session. This is the real-runtime
  assertion used by `./x pi-test run` and `TestPiRealRuntimeSmoke`.

- [ ] **Task 3.5: Apply the final runtime operations and freeze lifecycle records.** In
  `.awf/topics/parts/rendering/pi-runtime/current-state.md`, replace the two claims as follows,
  preserving Origin and appending `Revised-by: ADR-0173`:

  ```text
  ### `invariant: pi-extension-target-render`

  Enabling Pi renders the handoff entrypoint and the subagent index, bounded model-routing module, and runner with provenance. The model-routing module owns pure preference parsing, merging, validation-state representation, and routing-card construction; the entrypoint retains tool registration, queueing, process lifecycle, and runtime integration. No telemetry or workflow-router output renders, and all files follow normal render and cleanup semantics.
  Origin: ADR-0148
  Revised-by: ADR-0162, ADR-0164, ADR-0167, ADR-0173
  Backing: test

  ### `invariant: pi-real-runtime-smoke`

  Pinned Pi runtime smoke covers generated TypeScript loading, native Pi skill discovery, effort-independent handoff, and before-agent-start routing-card delivery into the model request without a persisted session message, and verifies telemetry, router, and selection surfaces are absent.
  Origin: ADR-0148
  Revised-by: ADR-0149, ADR-0161, ADR-0162, ADR-0164, ADR-0167, ADR-0173
  Backing: unbacked
  Verify: Run `./x pi-test run` to exercise native Pi skill discovery, effort-independent handoff, and routing-card delivery into a real pinned Pi model request without session-message persistence, with no telemetry, router, or selection.
  ```

  Append the final Applied event with the next checker-reported sequence and exactly `update
  rendering/pi-runtime:pi-extension-target-render`, `update
  rendering/pi-runtime:pi-real-runtime-smoke`. Change ADR-0173 to `Implemented`, append its
  `Implemented` event with the unchanged frozen digest, and run `./x render`. If implementation is
  abandoned before this event, append `Abandoned` with a reason and preserve only completed batches:
  before Phase 1, all six operations are Canceled; after Phase 1, its workflow-template add remains
  Applied and the five later operations are Canceled; after Phase 2, the first four operations remain
  Applied and only the two runtime updates are Canceled. An unfinished batch is never partially
  Applied: discard its uncommitted claim/source/proof changes and cancel every operation in that
  batch plus all later operations.

- [ ] **Task 3.6: Verify, stage, gate, and create the final implementation commit.** Run `gofmt -w
  internal/project/target.go internal/project/target_test.go internal/project/output_plan_test.go
  internal/project/project_test.go`, `go test ./...`, `./x pi-test run`, `./x render`, `./x check`,
  and `git diff --check`. Require all tests and coverage to pass, drift to be clean, and diff-check to
  print no output. Inspect `git status --short`, reject paths outside File structure, stage the
  complete transaction with this exact command:

  ```sh
  git add -- templates/pi/awf-subagents/{index.ts.tmpl,model-routing.ts.tmpl} templates/partials/pi-minimum-runtime.md tools/pi-extension-test/tests/{index.test.ts,runtime.test.ts} tools/pi-extension-test/container.sh internal/project/{target.go,target_test.go,output_plan_test.go,project_test.go} .awf/topics/parts/rendering/pi-runtime/current-state.md docs/decisions/0173-deliberate-subagent-model-selection.md .awf/awf.lock .pi/extensions/awf-subagents/{index.ts,model-routing.ts} .pi/extensions/awf-handoff/index.ts docs/topics/rendering/pi-runtime.md docs/domains/rendering.md docs/decisions/INDEX.md examples/sundial/.awf/awf.lock examples/sundial/.pi/extensions/awf-subagents/{index.ts,model-routing.ts} examples/sundial/.pi/extensions/awf-handoff/index.ts
  ```

  Require `git diff --cached --name-only` to equal the Phase 3 status inventory, run `./awf check
  --staged`, then `./x gate`; both must pass. Commit:

```commit
feat(rendering): inject Pi subagent routing cards (implements 0173)
```

- [ ] **Task 3.7: Settle phase review and freeze this plan.** After the Phase 3 closing commit,
  perform the required report-only phase review. Resolve findings through focused parent-owned
  commits, each with explicit staging, `./awf check --staged`, and `./x gate`; never amend the phase
  commit. Record settled implementation findings under Notes, check every completed task, change
  this plan to `status: Implemented`, stage exactly with `git add --
  docs/plans/2026-07-29-deliberate-subagent-model-selection.md`, require the cached inventory to
  contain only that path, and commit the plan-only freeze after `./awf check --staged` and `./x gate`:

```commit
docs(plans): freeze deliberate subagent model selection plan
```

## Verification

- `go test ./...`, `./x pi-test run`, and `./x gate full` pass on the final tree.
- `./x render && ./x check` is clean for the root project and Sundial adopter.
- `./awf topic rendering/workflow-skill-templates`, `./awf topic rendering/pi-workflows`, and
  `./awf topic rendering/pi-runtime` show the six ADR-0173 operations Applied in declaration order,
  with backed proof markers for every invariant claim and the exact unbacked smoke Verify command.
- `docs/decisions/INDEX.md` lists ADR-0173 under Implemented history and is render-generated.
- `rg -n 'model[": ]+(default|auto|inherit parent)' templates/skills .awf/parts/agents-doc
  .agents/skills .claude/skills .cursor/skills .gemini/skills .github/skills .pi/skills` returns no
  dispatch example that passes a sentinel; display-only explanation text is reviewed manually.
- `rg -n 'openai-codex|gpt-5.6-(luna|terra|sol)' templates/skills
  templates/agents-doc/AGENTS.md.tmpl` prints no generic template match; provider references remain
  confined to the Pi preset, dynamic local card data, tests, and historical records.
- `git status --short` prints no output.

## Notes

- Phase 2 settlement commits `2014e4df` and `2249e36f` already updated
  `changelog/CHANGELOG.md`; its File structure entry records that settled work and does not imply
  another Phase 3 staging change.
- Review after Phase 2 found that adopter-facing preference documentation had not traveled with the
  Applied operation batch. Retained commits and lifecycle events are not rearranged for a cleaner
  plan narrative; the explicit forward documentation prerequisite corrects current truth before
  Phase 3. The retrospective should record shared-source and adopter-document closure as a planning
  pitfall.
- The current entrypoint displays `inherit parent` as TUI call metadata. That display label may
  remain only if tests prove it is never accepted or emitted as a tool argument; changing the label
  to `configured/default` is preferred to remove ambiguity.
- The routing card uses `before_agent_start` system-prompt replacement. It never uses `message`,
  `sendMessage`, or `appendEntry`, because the card is run-local configuration rather than session
  history.
- Phase 3 deliberately creates and consumes `model-routing.ts` in the same transaction. Creating the
  template or output descriptor earlier would leave an unconsumed definition or apply the runtime
  output claim out of order.
- Phase 3's `getActiveTools` requirement is owned by the shared minimum-runtime partial. Its handoff
  output changes are provenance-only consequences of that shared source; they do not change handoff
  behavior or widen the runtime operation batch.
