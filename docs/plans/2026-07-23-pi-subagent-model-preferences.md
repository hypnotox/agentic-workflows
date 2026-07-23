---
date: 2026-07-23
adrs: [151]
status: Implemented
---
# Plan: Pi subagent model preferences

## Goal

Implement [ADR-0151](../decisions/0151-local-per-role-pi-subagent-model-preferences.md): extension-owned
per-role child model preferences for the four governed Pi subagent tools, with strict startup
validation, preference-aware resolution, routing-source diagnostics, an atomic TUI setup wizard
with an embedded registry-gated recommended preset, and the accompanying guidance updates.
Non-goals: no change to which work routes to a child (ADR-0149's territory), no noninteractive
wizard mode, and no awf-side validation of the preference files.

## Architecture summary

All runtime behavior lands in the existing `templates/pi/awf-subagents/index.ts.tmpl` (the
five-file Pi extension surface is unchanged). Phase 1 adds the preference store (two JSON sources,
strict shape validation, session-start registry validation, blocked-state semantics), rewires
`resolveChildModel` to the ADR's precedence chain with per-call registry revalidation, extends the
routing diagnostics, documents the preference behavior in working-with-awf, and applies the first
ADR batch (operations 1-3, ADR flips to Implementing). Phase 2 updates the rendered dispatch
guidance (agent guide, two execution skills, working-with-awf) with no claim operations. Phase 3 adds the `/awf-subagent-models` wizard command with atomic
persistence and the recommended preset, applies the final batch (operation 4), and flips the ADR
to Implemented and this plan to Implemented. Each phase's closing commit is one staged transaction
validated by `awf check --staged` and `./x gate`.

## File structure

- **Created:** `docs/plans/2026-07-23-pi-subagent-model-preferences.md` (this file).
- **Modified:**
  - `templates/pi/awf-subagents/index.ts.tmpl` (phases 1 and 3)
  - `.gitignore` (phase 1)
  - `internal/project/target_test.go` (phases 1 and 3)
  - `tools/pi-extension-test/tests/index.test.ts` (phases 1 and 3)
  - `.awf/topics/parts/rendering/pi-workflows/current-state.md` (phases 1 and 3)
  - `.awf/topics/parts/rendering/pi-runtime/current-state.md` (phase 1)
  - `docs/decisions/0151-local-per-role-pi-subagent-model-preferences.md` (status history, phases 1 and 3)
  - `templates/agents-doc/AGENTS.md.tmpl` (phase 2)
  - `templates/skills/subagent-driven-development/SKILL.md.tmpl` (phase 2)
  - `templates/skills/executing-plans/SKILL.md.tmpl` (phase 2)
  - `templates/docs/working-with-awf.md.tmpl` (phases 1, 2, and 3)
  - Rendered outputs of all of the above via `./x sync` (`.pi/extensions/awf-subagents/index.ts`,
    `AGENTS.md`, `docs/working-with-awf.md`, per-target skill renders, `docs/decisions/INDEX.md`,
    domain/topic docs, `.awf/awf.lock`); stage whatever `git status` shows changed after sync.
- **Deleted:** none.

## Phase 1: preference store, resolution, and first ADR batch

Phase 1 is one staged transaction with a single closing commit: the ADR's first Applied batch must
travel with exactly its claim mutations and the behavior/tests that back them, so the tasks below
stage together and commit once. This is the sanctioned coupled-transaction exception; the batch
pairing rule is why it cannot be sliced.

- [ ] **Task 1.1: Add the preference model and store to `templates/pi/awf-subagents/index.ts.tmpl`.**
  Add, near the existing exported constants:

  ```ts
  export const PREFERENCE_ROLES = ["grounding", "exploration", "review", "implementation"] as const;
  export type PreferenceRole = (typeof PREFERENCE_ROLES)[number];
  export const ROLE_PREFERENCE_KEYS: Record<RunRequest["role"], PreferenceRole> = {
    grounding: "grounding", explore: "exploration", review: "review", implement: "implementation",
  };
  export const GLOBAL_PREFERENCES_FILE = "awf-subagents.json";
  export const LOCAL_PREFERENCES_FILE = "awf-subagents.local.json";
  export interface SubagentModelPreferences {
    default?: string; grounding?: string; exploration?: string; review?: string; implementation?: string;
  }
  ```

  Extend `ExtensionDependencies` with two required fields: `agentDir: string` and
  `configDirName: string`. Production wiring in the default export passes `getAgentDir()` and
  `CONFIG_DIR_NAME`, both added to the existing `@earendil-works/pi-coding-agent` import list.

  Add `createPreferenceStore(deps: ExtensionDependencies)` returning a store with:
  - `paths`: global `join(deps.agentDir, GLOBAL_PREFERENCES_FILE)`; project
    `join(projectRoot(deps.extensionFile), deps.configDirName, LOCAL_PREFERENCES_FILE)`.
  - `reload(): Promise<void>`: reads both paths via `deps.readFile`. A read failure whose error
    carries `code === "ENOENT"` means the file is absent, which is valid (no preferences from that
    scope). Any other read failure is a validation error for that scope. For a present file,
    validate: JSON parses; the value is a plain non-array object; every key is one of `default`
    plus `PREFERENCE_ROLES` (an unknown key is an error naming the key); every value is a string
    in exact `provider/model-id` shape (a slash neither at position 0 nor last; anything else is
    an error naming the key and value). Shape errors accumulate per scope; a scope with errors
    contributes no values.
  - `state()`: returns per-scope `{ path, values, errors }` plus `blocked: boolean` and a flat
    `errors: string[]`, where `blocked` is true whenever either scope has any error (shape errors
    from `reload` or registry errors recorded by `validateAgainstRegistry` below).
  - `validateAgainstRegistry(ctx): void`: for every configured value in both scopes, resolve
    `provider/id` against `ctx.modelRegistry.find`; a missing model records an error naming the
    scope, key, and value as unregistered; a found model failing
    `ctx.modelRegistry.hasConfiguredAuth` records an unauthenticated error. Errors feed `blocked`.

  In `registerSubagentTools`, construct the store once, call `reload()` at registration (fire the
  promise and record it; every later use awaits it settling), and register a `session_start`
  handler mirroring the `guardMinimumRuntime` pattern (single-notice symbol
  `Symbol.for("awf.pi.subagent-preferences-notified")`): after reload settles, run
  `validateAgainstRegistry(ctx)`; if `blocked`, call `ctx.ui.notify(<message>, "error")` where the
  message names every error with its file path and ends with the literal repair pointer
  `Run /awf-subagent-models to repair. Explicit per-call model arguments remain available.`
  Constraint: absent files and empty objects (`{}`) are valid and produce no notice. Forbidden:
  partially honoring a file that has any error.

- [ ] **Task 1.2: Rewire `resolveChildModel` to the precedence chain with routing-source
  diagnostics.** In the same template, change `resolveChildModel` to
  `resolveChildModel(ctx, role, requested, store)` returning
  `{ model: { provider, id }, requested, source }`:
  - `requested` defined: behavior unchanged from today (exact-shape parse, registry find,
    configured-auth check, no fallback), `source: "requested"`.
  - `requested` undefined and `store.state().blocked`: throw an `Error` whose message starts with
    the literal `Subagent model preferences are invalid; implicit routing is blocked.`, includes
    the flat error list, and ends with the same repair pointer as the startup notice. Parent
    inheritance is part of the blocked implicit chain; there is no fallback past this error.
  - otherwise resolve the first configured value in order: project role, global role, project
    default, global default (role key via `ROLE_PREFERENCE_KEYS[role]`), with `source`
    respectively `"project-role" | "global-role" | "project-default" | "global-default"`; the
    winning candidate is revalidated against the live registry at this call (find plus configured
    auth); a failure throws naming the scope, key, and model, with no fall-through to later
    candidates or the parent.
  - nothing configured: inherit the parent as today (`ctx.model` required), `source: "inherited"`.

  Update `ExecutionMetadata.modelSource` to the union
  `"inherited" | "requested" | "project-role" | "global-role" | "project-default" | "global-default"`
  and `executionMetadata` to carry the returned `source`. Update the four tool `description` and
  `promptGuidelines` strings from "omission inherits the parent" to "omission resolves configured
  preferences, then inherits the parent". All four `execute` functions pass their role and the
  store into `resolveChildModel` and await the initial reload settling first.

- [ ] **Task 1.3: Add the project-local ignore rule to the root `.gitignore`.** Append, directly
  after the existing `!.pi/` line:

  ```
  .pi/awf-subagents.local.json
  ```

- [ ] **Task 1.4: Extend the Go render tests in `internal/project/target_test.go`.** Extend the
  want-list of `TestPiSubagentModelRouting` (proof of
  `rendering/pi-workflows:pi-subagent-model-routing`) with tokens pinning the new contract, at
  minimum: `"project-role"`, `"global-role"`, `"project-default"`, `"global-default"`,
  `Subagent model preferences are invalid; implicit routing is blocked.`, and
  `ROLE_PREFERENCE_KEYS`. Keep the existing no-silent-fallback assertions. Add a new test
  `TestPiSubagentModelPreferences` carrying the proof marker comment
  `// invariant: rendering/pi-workflows:pi-subagent-model-preferences`, asserting the rendered
  `awf-subagents/index.ts` contains the store contract tokens, at minimum:
  `GLOBAL_PREFERENCES_FILE = "awf-subagents.json"`,
  `LOCAL_PREFERENCES_FILE = "awf-subagents.local.json"`, `createPreferenceStore`,
  `validateAgainstRegistry`, `ENOENT`, `Run /awf-subagent-models to repair.`, and
  `session_start`. In `TestPiSubagentToolBoundaries` (proof of
  `rendering/pi-runtime:pi-child-tool-boundaries`), keep the allowlist assertions unchanged and
  assert the new four-argument `resolveChildModel` call shape appears for all four tools while
  the old two-argument shape `resolveChildModel(ctx, params.model)` no longer appears. Beyond
  extending, both tests must have every existing exact-string assertion the task-1.1/1.2 rewrite
  invalidates updated to the new shapes - at minimum the pinned per-role call shape
  `const selected = resolveChildModel(ctx, params.model)`, the pinned return shape
  `return { model: { provider: ctx.model.provider, id: ctx.model.id }, requested: undefined }`,
  and any pinned contract strings that quote the old signature or return object; sweep both tests
  for stale strings rather than assuming this list is exhaustive.
  Verification: `go test ./internal/project/ -run 'TestPiSubagent' -count=1` passes.

- [ ] **Task 1.5: Add Pi extension tests in `tools/pi-extension-test/tests/index.test.ts`.**
  Update the existing fake-dependency helpers for the two new required `ExtensionDependencies`
  fields (`agentDir`, `configDirName`) and make the fake `readFile` scriptable per path. New tests
  (names indicative; coverage mandatory - the container run enforces 100% on the extension files):
  - every precedence step resolves and reports its source: explicit beats project role beats
    global role beats project default beats global default beats parent, asserted through
    `details.modelSource` and the resolved model on the governed tools;
  - a malformed file (parse error), an unknown key, a malformed value shape, an unregistered
    reference, and an unauthenticated reference each: produce the session-start error notice once
    (single-notice symbol), block an omitted-model call with the literal blocked message, and
    leave an explicit-model call working;
  - absent files and `{}` files are valid, produce no notice, and fall through to parent
    inheritance;
  - a configured model that disappears from the registry after startup is rejected at queue time
    (revalidation) with no fall-through.
  Verification: `./x gate` (which runs the container extension tests) passes with 100% extension
  coverage.

- [ ] **Task 1.6: Document the preference files in the working-with-awf Pi section.** In
  `templates/docs/working-with-awf.md.tmpl`, in the `### Pi workflow subagents` section's
  model-routing paragraph, first revise the existing omission sentence (the prose stating that
  omission inherits the active parent model) to state that omission resolves configured
  preferences, then inherits the parent (mirroring the tool-description update in task 1.2).
  Then append a paragraph stating, in qualifying form: the extension also reads per-role model
  preferences from a user-global `awf-subagents.json` in Pi's agent directory and a gitignored
  project-local `awf-subagents.local.json` in the project's Pi config directory under the project
  root; resolution order is explicit argument, project role, global role, project default, global
  default, then parent inheritance; both files are strictly validated at session start and any
  error blocks implicit routing visibly until repaired while explicit per-call models keep
  working; and configured choices are revalidated against the live registry before every queued
  child.

- [ ] **Task 1.7: Apply the first ADR batch and its claim mutations.** In
  `.awf/topics/parts/rendering/pi-workflows/current-state.md`, replace the
  `pi-subagent-model-routing` claim body and provenance with exactly:

  ```
  ### `invariant: pi-subagent-model-routing`

  Every Pi subagent role accepts an optional exact provider/model-id; an omitted model resolves configured preferences in explicit, project-role, global-role, project-default, global-default, parent order; unknown, unauthenticated, or unregistered explicit or configured choices reject visibly before queueing without fallback; thinking is inherited for child clamping; and diagnostics report requested, resolved, and actual models with the routing source.
  Origin: ADR-0148
  Revised-by: ADR-0151
  Backing: test
  ```

  In the same file, insert alphabetically (between `pi-subagent-failure-details` and
  `pi-subagent-model-routing`) the new claim exactly:

  ```
  ### `invariant: pi-subagent-model-preferences`

  The generated Pi extension loads a user-global and a gitignored project-local JSON preference file at session start, strictly validates keys, role names, and canonical registry-authenticated model references, blocks all implicit routing including parent inheritance while either file is invalid, keeps explicit per-call models usable throughout, revalidates current registry availability immediately before every queued child, and names each child's routing source in diagnostics.
  Origin: ADR-0151
  Backing: test
  ```

  In `.awf/topics/parts/rendering/pi-runtime/current-state.md`, replace the
  `pi-child-tool-boundaries` claim body and provenance with exactly:

  ```
  ### `invariant: pi-child-tool-boundaries`

  Pi subagent children use an explicitly selected validated model, a validated configured preference, or inherit the parent; inherit the parent's thinking level; receive fixed role allowlists excluding extension tools; and enforce fixed retained-output limits with explicit truncation diagnostics.
  Origin: ADR-0148
  Revised-by: ADR-0151
  Backing: test
  ```

  In `docs/decisions/0151-local-per-role-pi-subagent-model-preferences.md`, append to
  `## Status history` an Implementing event and an Applied event of the shape:

  ```
  - <date>: Implementing; content-sha256: <frozen digest>
  - <date>: Applied; state-sequence: <next>; operations: update `rendering/pi-workflows:pi-subagent-model-routing`, update `rendering/pi-runtime:pi-child-tool-boundaries`, add `rendering/pi-workflows:pi-subagent-model-preferences`
  ```

  where `<date>` is the commit date, `<frozen digest>` is written bare (64 lowercase hex, no code
  span, matching every existing Status history line), and `<next>` is the next value in the
  repository-global contiguous state-sequence namespace. Never pre-compute either value: run
  `awf check --staged`; on mismatch it names the expected digest and the expected next sequence;
  use exactly the values it names (Implementing ADR-0149 may interleave batches, so the sequence
  is only knowable at commit time). Update the file's `status:` frontmatter to `Implementing`.

- [ ] **Task 1.8: Sync, validate, and commit phase 1.** Run `./x sync` (regenerates the rendered
  extension, INDEX.md, topic docs, and the lock). Stage every changed path from tasks 1.1-1.7
  plus the sync output (`git add` the exact paths `git status --short` shows; no `git add -A`).
  Run `awf check --staged` (clean), then `./x gate` (exit 0). Commit:

  ```commit
  feat(rendering): add Pi subagent model preferences (applies 0151 batch)
  ```

## Phase 2: rendered guidance updates

- [ ] **Task 2.1: Extend the agent-guide dispatch guidance.** In
  `templates/agents-doc/AGENTS.md.tmpl`, after the sentence ending `shared-checkout implementation
  stays alone.` (currently line 69), append exactly:

  ```
  Long implementations should favor sequential implementation subagents so the orchestrating parent stays lean: length and parent-context pressure are explicit reasons to prefer subagent implementation, coupling may still justify inline execution, and the orchestrator decides case by case.
  ```

  Constraint: target-generic wording only; no Pi command or tool name.

- [ ] **Task 2.2: Extend the two execution-skill templates.** In
  `templates/skills/subagent-driven-development/SKILL.md.tmpl`, in the positioning prose near the
  "Companion to `{{ .prefix }}-executing-plans`" sentence at the top, append one sentence stating
  that long implementations and parent-context pressure favor this dispatch shape, while tight
  coupling may still justify the inline companion, with the orchestrator deciding case by case.
  In `templates/skills/executing-plans/SKILL.md.tmpl`, append the mirror sentence to its
  companion positioning: inline execution suits tightly coupled tasks, but length and
  parent-context pressure are explicit reasons to hand long plans to the subagent-dispatch
  companion where the runtime supports it. Both additions are non-contractual prose (qualifying
  form); both must stay target-generic with no Pi command or tool name (the existing
  `skill-prose-tool-agnostic` render test fails otherwise).

- [ ] **Task 2.3: Append the dispatch steer to working-with-awf.** In
  `templates/docs/working-with-awf.md.tmpl`, append to the workflow-oriented prose of the
  `### Pi workflow subagents` section (after the paragraph task 1.6 added) one qualifying-form
  sentence carrying the same steer as tasks 2.1 and 2.2: long implementations should favor
  sequential implementation subagents so the orchestrating parent stays lean, with length and
  parent-context pressure as explicit reasons and coupling as the residual reason to stay inline.
  Target-generic wording for the steer itself (no new tool names beyond what the section already
  documents).

- [ ] **Task 2.4: Sync, validate, and commit phase 2.** Run `./x sync`; stage the template edits
  plus rendered outputs; run `awf check --staged` (clean) and `./x gate` (exit 0). Commit:

  ```commit
  docs(rendering): steer long implementations toward subagent dispatch
  ```

## Phase 3: wizard command, preset, and final ADR batch

Phase 3 is one staged transaction with a single closing commit, for the same batch-pairing reason
as phase 1.

- [ ] **Task 3.1: Add the recommended preset and wizard command to
  `templates/pi/awf-subagents/index.ts.tmpl`.** Add the exact constant:

  ```ts
  export const RECOMMENDED_PRESET: Required<SubagentModelPreferences> = {
    default: "openai-codex/gpt-5.6-terra",
    grounding: "openai-codex/gpt-5.6-sol",
    exploration: "openai-codex/gpt-5.6-luna",
    review: "openai-codex/gpt-5.6-sol",
    implementation: "openai-codex/gpt-5.6-terra",
  };
  ```

  Extend `ExtensionDependencies` with required write seams `writeFile(path: string, data: string,
  options: { mode: number }): Promise<void>`, `mkdir(path: string, options: { recursive: true }):
  Promise<unknown>`, `rename(from: string, to: string): Promise<void>`, and
  `unlink(path: string): Promise<void>`; production wiring uses the `node:fs/promises` functions.
  Add `"registerCommand"` to the `guardMinimumRuntime` required-API list in
  `registerSubagentTools`.

  Register `pi.registerCommand("awf-subagent-models", { description: "Configure per-role subagent
  model preferences.", handler })`. Handler behavior (qualifying pseudocode; every branch below is
  required):
  - TUI gate: if `typeof ctx.ui?.custom !== "function"`, call
    `ctx.ui?.notify?.("awf-subagent-models requires an interactive TUI session.", "error")` and
    return; no other mode exists.
  - Await the store's current reload, then `validateAgainstRegistry(ctx)`.
  - Step 1, scope: a select between user-global and project-local, each labeled with its absolute
    path. Cancel (the `tui.select.cancel` keybinding, as in the handoff extension) at this or any
    later step notifies "Subagent model preferences unchanged." and returns without writing.
  - Step 2, current state: display the selected scope's parsed values or its validation errors,
    plus the effective routing per role under the current two-scope state. Capture the target
    file's current raw content (or its absence) now; this snapshot drives stale-writer detection.
  - Step 3, preset: eligible only when every `RECOMMENDED_PRESET` value resolves through
    `ctx.modelRegistry.find` with `hasConfiguredAuth` true. When eligible, offer
    "Apply recommended GPT-5.6 preset" as a choice beside "Configure each slot"; when not
    eligible, do not offer it and show one line naming which referenced models are missing or
    unauthenticated. Applying the preset sets all five keys and skips to step 5.
  - Step 4, slots: configure sequentially in the order default, grounding, exploration, review,
    implementation. Each selector lists every registry model plus a "leave unset (use fallback
    chain)" option, and each entry shows: `provider/id`, display name, base input/output cost per
    Mtok, tier input/output cost with its threshold when the model declares cost tiers (base and
    tier visually distinct), context window, max output tokens, reasoning and image support, and
    markers for the current parent model, the value currently configured in this scope, and the
    preset recommendation for the slot. Each slot shows one role-guidance line (default and
    implementation: mid-tier coding model; grounding and review: strongest model; exploration:
    cheapest capable model) and the wizard states that it configures child routing only - the
    parent session remains the mastermind for brainstorming and plan authoring.
  - Step 5, summary and confirm: show the resulting file content and the effective routing per
    role after the save; confirm or cancel.
  - Step 6, gitignore enforcement (project scope only): run `git rev-parse --is-inside-work-tree`
    via `pi.exec` in the project root. Not a work tree: proceed with the visible notice "Not a git
    work tree; skipping ignore check." In a work tree: `git check-ignore -q <relative path>`;
    when not ignored, offer to append the exact line `<configDirName>/awf-subagents.local.json`
    to the project root `.gitignore` (read-modify-write via the dependency seams, appending with
    a trailing newline); declining aborts the save with "Save canceled: the project-local
    preference file must be gitignored."; accepting appends, then proceeds.
  - Step 7, atomic write: `mkdir` the parent directory recursively; re-read the target file and
    compare with the step-2 snapshot (absent equals absent); a mismatch aborts with an error
    notice naming a concurrent modification and writes nothing; otherwise `writeFile` a sibling
    temporary file named `<target>.<uuid>.tmp` with mode `0o600`, then `rename` it onto the
    target. Any failure after temp creation attempts `unlink` of the temp (best effort) and
    notifies the error; the target file is never left partially written. On success, await
    `store.reload()`, re-run `validateAgainstRegistry(ctx)`, and notify
    "Subagent model preferences saved."
  Forbidden: any write on cancellation; any silent preset application; any save that leaves a
  temp file as the target.

- [ ] **Task 3.2: Extend the Go render tests.** In `internal/project/target_test.go`, add
  `TestPiSubagentModelWizard` with the proof marker comment
  `// invariant: rendering/pi-workflows:pi-subagent-model-wizard`, asserting the rendered index
  contains at minimum: `registerCommand("awf-subagent-models"`, `RECOMMENDED_PRESET`,
  `openai-codex/gpt-5.6-terra`, `openai-codex/gpt-5.6-sol`, `openai-codex/gpt-5.6-luna`,
  `requires an interactive TUI session`, `check-ignore`, `.tmp`, `mode: 0o600`, and `rename`; and
  does not contain a `writeFileSync` token (no synchronous write path). Verification:
  `go test ./internal/project/ -run 'TestPiSubagent' -count=1` passes.

- [ ] **Task 3.3: Extend the Pi extension tests.** In
  `tools/pi-extension-test/tests/index.test.ts`, extend the fake dependencies with scriptable
  `writeFile`/`mkdir`/`rename`/`unlink` recorders and a fake `ctx.ui.custom`/`ctx.ui.notify`
  driver (following the existing driver pattern in
  `tools/pi-extension-test/tests/handoff.test.ts`). New tests covering every wizard branch (100%
  coverage enforced): non-TUI refusal; cancel at each step writes nothing; preset offered only
  when all five referenced models are registered and authenticated, and applying it writes all
  five keys; slot flow writes the selected values and honors "leave unset"; project-scope save in
  a work tree with an unignored target offers the rule, a decline aborts, an accept appends the
  exact line; non-work-tree save proceeds with the notice; stale-writer mismatch aborts without
  writing; write failure unlinks the temp and leaves the target; success renames the temp,
  reloads the store, and a subsequent omitted-model call routes by the new preferences.
  Verification: `./x gate` passes with 100% extension coverage.

- [ ] **Task 3.4: Document the wizard in working-with-awf.** In
  `templates/docs/working-with-awf.md.tmpl`, append to the paragraph added in task 1.6 (qualifying
  form): the `/awf-subagent-models` TUI wizard is the setup and repair path; it selects scope,
  shows current state and errors, offers an embedded recommended preset only when every referenced
  model is currently registered and authenticated, presents per-model pricing with request-wide
  tiers distinguished from base rates, enforces the project-local gitignore rule at save, persists
  atomically, and leaves the file unchanged on cancellation.

- [ ] **Task 3.5: Apply the final ADR batch, flip statuses.** In
  `.awf/topics/parts/rendering/pi-workflows/current-state.md`, insert alphabetically (after
  `pi-subagent-model-routing`, before `pi-subagent-progress-bounds`) the new claim exactly:

  ```
  ### `invariant: pi-subagent-model-wizard`

  The /awf-subagent-models command is a TUI-only atomic wizard that selects scope, surfaces current preferences and validation errors, offers the embedded recommended preset only when every referenced model is currently registered and authenticated, presents informed model selectors distinguishing base from request-wide tier pricing, enforces the project-local gitignore rule at save with a visible-notice degradation outside a git work tree, persists with owner-only permissions and stale-writer detection via sibling-temp rename, refreshes in-memory preferences after success, and leaves the existing file unchanged on cancellation or failure.
  Origin: ADR-0151
  Backing: test
  ```

  In `docs/decisions/0151-local-per-role-pi-subagent-model-preferences.md`, append to
  `## Status history` a final Applied event and the terminal status event of the shape:

  ```
  - <date>: Applied; state-sequence: <next>; operations: add `rendering/pi-workflows:pi-subagent-model-wizard`
  - <date>: Implemented; content-sha256: <frozen digest>
  ```

  with `<frozen digest>` the same bare 64-hex value as the phase-1 Implementing event and
  `<next>` the next global state-sequence exactly as `awf check --staged` names it (as in task
  1.7, never pre-computed). Set the frontmatter `status:` to `Implemented`. In this plan file, set the frontmatter
  `status:` to `Implemented` and record any implementation findings under `## Notes`.

- [ ] **Task 3.6: Sync, validate, and commit phase 3.** Run `./x sync`; stage every changed path;
  run `awf check --staged` (clean) and `./x gate` (exit 0). Commit:

  ```commit
  feat(rendering): add Pi subagent model wizard (implements 0151)
  ```

## Verification

- `./x gate full` exits 0 on the final tree.
- `awf check` is clean; `awf topic rendering/pi-workflows` lists `pi-subagent-model-preferences`
  and `pi-subagent-model-wizard` with `Origin: ADR-0151`, and `pi-subagent-model-routing` with
  `Revised-by: ADR-0151`; `awf topic rendering/pi-runtime` shows `pi-child-tool-boundaries` with
  `Revised-by: ADR-0151`.
- `docs/decisions/INDEX.md` lists ADR-0151 under History as Implemented (regenerated, never
  hand-edited).
- `git check-ignore .pi/awf-subagents.local.json` exits 0 in this repository.
- The rendered `.pi/extensions/awf-subagents/index.ts` contains no unresolved template token
  (`awf check` enforces this).

## Notes

- A plan-resync against Implementing ADR-0149 precedes implementation (`awf-reviewing-plan-resync`
  runs after plan review since an ADR exists); ADR-0149's operations touch none of this plan's
  claims, so drift is expected only if 0149's guidance edits collide textually with phase 2's
  template sentences - reconcile wording, never the operation sets.
- The `checkpoint-handoff-semantics` worktree holds an uncommitted ADR numbered 0151; if it lands
  on main before this branch merges, this effort's ADR renumbers (the contiguity check forces it)
  and every `0151` reference in this plan, the topic provenance lines, and the ADR filename moves
  with it.
- Accepted transient window: phase 1 ships the error-message repair pointer naming
  `/awf-subagent-models` while the command itself lands in phase 3. Between the two commits the
  extension names a not-yet-existing command; this is the accepted cost of keeping the batch
  pairing sliceable, and phase 3 closes the window in the same effort.
- Path anchoring: the project-local preference file anchors at the project root derived from the
  extension location (`projectRoot(deps.extensionFile)`), not the process working directory - the
  gitignore enforcement assumes the repository root, and Pi may run from a subdirectory. ADR-0151
  Decision item 2 was amended to match while Proposed (commit 0b8d0db4).
- Dependency verification (done at plan review): the pinned fork tarball
  `pi-coding-agent-fork-v0.81.1-awf.3` exports both `getAgentDir(): string` and
  `CONFIG_DIR_NAME: string` from the package root (`dist/index.d.ts` re-exporting
  `dist/config.d.ts:68,76`), so the task-1.1 production wiring is sound.
- Out of scope, deliberately: noninteractive wizard mode (revisit only with an explicit design),
  awf-side validation of preference files, and any change to inline-vs-subagent execution routing
  (ADR-0149).
- Implementation findings (recorded at the freeze): `awf sync` refuses a proof marker naming a
  not-yet-existing claim, so within each batch phase the claim mutations must land in the working
  tree before the sync step, not after the tests. The thinking level must be snapshotted
  synchronously before awaiting the preference store, or the queued-children thinking-snapshot
  contract breaks. The preference store captures `deps.readFile` at creation so a test replacing
  the dependency after registration cannot corrupt the in-flight initial load. The task-2.2
  executing-plans sentence says "run a long plan through the subagent-per-task companion" instead
  of the drafted "hand ... to the subagent-dispatch companion": the evals chain checker treats
  "dispatch" on a line naming a skill as an invocation line. The wizard uses Pi's built-in
  `ctx.ui.select`/`ctx.ui.confirm` dialogs (cancel = undefined/false) rather than a `ui.custom`
  component, with the TUI gate on `ctx.mode !== "tui"`; the resolution chain and the wizard's
  routing preview share one `preferredReference` helper so the precedence logic exists once.
- Review-noted deviations: the per-tool four-argument `resolveChildModel` call-shape assertions
  live in `TestPiSubagentModelRouting`'s per-role block loop rather than
  `TestPiSubagentToolBoundaries` (equivalent coverage, different home than task 1.4's text). The
  working-with-awf Pi section documents the wizard command unconditionally, following that doc's
  established pattern of naming Pi tools in its inherently Pi-scoped section; ADR-0151 Decision
  8's Pi-conditional-branch constraint governs the shared skill and agent-guide surfaces. The
  implementation review added a post-save validity notice: a save whose content blocks implicit
  routing reports the error list and repair pointer immediately instead of a plain success
  notice.
