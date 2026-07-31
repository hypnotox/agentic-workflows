Pi workflow contracts: governed subagent tools, session handoff, native skills, and structured exploration dispatch.

## Claims

### `invariant: pi-session-handoff-lifecycle`

Pi handoff retains its single-use queue, countdown, cancellation, parent link, kickoff, child cleanup, editor fallback, and post-countdown revalidation. When supplied, memory must be the stable, current-owned, singly-linked, bounded valid-UTF-8 regular file at `.awf/efforts/<slug>/memory.md` with matching `Effort: <slug>` identity and confined repository identity. Handoff never parses state, selects or assigns an effort, invokes awf, or mutates memory.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0167, ADR-0175
Backing: test

### `invariant: pi-dedicated-grounding-dispatch`

In the generated Pi extension and skills, brainstorming's grounding check dispatches through the dedicated grounding tool while general exploration and coupling audits use the exploration tool, and no non-Pi target's rendered output contains either Pi subagent tool name.
Origin: ADR-0148
Backing: test

### `invariant: pi-extension-editor-quiet-strip`

Every governed Pi extension file carries the ts-nocheck directive on the line immediately after the provenance banner, and the container test harness deterministically strips that exact directive from every extension TypeScript file in its ephemeral copy after source copy and before running the TypeScript compiler.
Origin: ADR-0148
Backing: test

### `invariant: pi-implementation-batch-exclusivity`

Pi correlates each tool preflight with the current leaf assistant tool-call id, blocks every member of a reconstructable batch that mixes implementation with siblings, and blocks only implementation when trustworthy batch context is unavailable.
Origin: ADR-0148
Backing: test

### `invariant: pi-session-handoff-public-contract`

Pi handoff accepts an optional exact repository-relative `.awf/efforts/<slug>/memory.md` path, or an absolute spelling that normalizes to it, plus a bounded kickoff; absent memory remains valid. Containment resolves against the primary control root: a current-owned regular-file `.git` marker whose `gitdir:` pointer has the `.git/worktrees/<name>` shape is dereferenced to the primary root, any other well-formed pointer or a symlinked, unowned, or absent marker keeps the rendered root, and a marker without a `gitdir:` line is rejected, so validation accepts the effort memory from any managed worktree. It validates slug grammar, exact basename, lexical and no-follow containment, ownership, one hard link, 1 MiB size, fatal UTF-8 decoding, stable identity, effort header, and repository identity without selecting or assigning an effort, invoking awf, adopting checkpoints, or fabricating history.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0162, ADR-0164, ADR-0167, ADR-0175, ADR-0189
Backing: test

### `invariant: pi-session-handoff-workflow`

Pi checkpoint guidance invokes handoff alone after persistence at a settled phase boundary, carrying the same effort slug and exact owned memory path for non-minimal work. It never creates standalone memory, requires selection or telemetry lifecycle state, adopts a checkpoint, or treats checkbox tasks and helper returns as routine handoff boundaries; repository authority remains primary and report-only children do not edit memory.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0164, ADR-0167, ADR-0166, ADR-0175
Backing: test

### `invariant: pi-native-workflow-skills`

Pi renders every enabled standard and local skill at `.pi/skills/<prefix>-<name>/SKILL.md`; disabled skills or Pi disablement prune those paths, and no router or hidden workflow-body output remains.
Origin: ADR-0167
Backing: test

### `invariant: pi-structured-exploration-contract`

The generated Pi extension exposes exactly four closed-schema roles, each with optional exact model routing; exploration retains required task, breadth, and detail and runs through the ten-active FIFO limiter without changing the other process boundaries.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-failure-details`

In the generated Pi extension, expected failures that occur after a child process has started return a marked error result that preserves bounded progress and diagnostics through a tool_result middleware hook instead of throwing, while retaining cancellation, cleanup, and implementation-commit-policy behavior.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-model-preferences`

The generated Pi extension merges user-global and gitignored project-local preferences per field for the shared default, every grounding, exploration, review, and implementation role, and the small, standard, and large tiers. Completeness requires every field explicitly after merging; missing fields remain valid and visible, while any malformed, overlong, unregistered, unauthenticated, unavailable, or unreadable configured field blocks all implicit routing and leaves valid explicit calls usable. Preference and registry state reloads at preflight and again immediately before child startup.
Origin: ADR-0151
Revised-by: ADR-0173
Backing: test

### `invariant: pi-subagent-model-routing`

Every Pi subagent role accepts only omission or an exact registry-valid provider/model-id of at most 256 printable-ASCII characters, excluding space and DEL. The tool schemas and preference parsing derive that form from one shared pattern constant, so the two layers cannot diverge, and within the permitted charset the bound is the same count whether measured in code points, UTF-16 units, or UTF-8 bytes. Omission alone requests configured role routing and parent fallback, and is displayed with a label the shared form check rejects, so a displayed value can never be copied back as a usable argument. Default, auto, inherit parent, and other sentinel values reject with an omit-the-field repair and are never normalized, and an overlong reference reports overlong before any form rejection. Queue acquisition is followed by preference and registry revalidation immediately before child startup, failures never fall through, thinking remains inherited for child clamping, and diagnostics report requested, resolved, and actual models with routing source.
Origin: ADR-0148
Revised-by: ADR-0151, ADR-0173, ADR-0176
Backing: test

### `invariant: pi-subagent-model-wizard`

The /awf-subagent-models command is a TUI-only atomic wizard for the shared default, all four explicit role defaults, and the small, standard, and large tiers. It preserves scope and error display, complete cancellation without writes, live registry-gated Luna/Terra/Sol preset selection, informed manual selectors, project-local gitignore enforcement, owner-only sibling-temp replacement, stale-writer detection, cleanup, and in-memory refresh, and it writes roles and tiers together as one preference transaction.
Origin: ADR-0151
Revised-by: ADR-0173
Backing: test

### `invariant: pi-implement-role-artifact`

The generated Pi extension builds the implementation child's role prompt from the rendered implementer agent at its `.pi/agents/` path, prepending the commit-authority role line for the call's mode. The before-and-after git snapshot fails a commit-capable implementation call whose HEAD is unchanged, naming the required stopped inventory, and retains the existing commit-forbidden violation, its message, cancellation, cleanup, and bounded-diagnostic reporting.
Origin: ADR-0177
Revised-by: ADR-0179
Backing: test

### `invariant: pi-subagent-progress-bounds`

The generated Pi extension retains at most 20 display events of at most 2 KiB each, reports cumulative omitted-event counts and truncation explicitly, and never keeps a second raw child-transcript store.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-progress-context-isolation`

The generated Pi extension carries intermediate child activity only in bounded tool details, never appending it to parent model-visible content or custom session messages, and a subagent tool's final content contains only the child report or a bounded failure summary.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-progress-rendering`

In the generated Pi extension, every public subagent tool's collapsed view renders status, recent bounded activity, omission state, and available usage, and its expanded view additionally renders the task, retained activity, the final report, present diagnostics, and available usage from the same structured details without changing execution.
Origin: ADR-0148
Backing: test

### `invariant: pi-role-contract-loader`

The generated Pi extension loads every dispatched role's contract from its rendered agent artifact through one shared loader that reads the file, strips frontmatter, prepends the role's per-call authority line, and fails with an actionable enable-and-render repair naming that role on a missing file or an empty instruction body. No dispatched role's prose remains inline in the extension.
Origin: ADR-0179
Backing: test
