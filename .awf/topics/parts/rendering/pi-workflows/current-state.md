Pi workflow contracts: awf profile policy, effort integration, native skills, and workflow guidance.

## Claims

### `invariant: pi-dedicated-grounding-dispatch`

The generated Pi adapter and workflow skills dispatch reusable independent repository-premise checks through the dedicated grounding profile whenever the grounding support trigger fires, while general exploration and coupling audits use exploration. No non-Pi target's rendered output contains either Pi subagent tool name.
Origin: ADR-0148
Revised-by: ADR-0243, ADR-0279
Backing: test

### `invariant: pi-extension-editor-quiet-strip`

Every governed retained Pi extension file carries the ts-nocheck directive on the line immediately after the provenance banner, and the host harness deterministically strips that exact directive from every copied extension TypeScript file after source copy and before strict compilation.
Origin: ADR-0148
Revised-by: ADR-0279, ADR-0281
Backing: test

### `invariant: pi-implementation-batch-exclusivity`

The implementation profile declares one active call and fail-closed parent-batch exclusivity, so it cannot share a parent tool batch with siblings. Pi-tools owns enforcement of that declaration, cancellation, queueing, and execution confinement; awf retains the profile declaration and commit-policy hooks.
Origin: ADR-0148
Revised-by: ADR-0279
Backing: test

### `invariant: pi-native-workflow-skills`

Pi renders every catalog skill at `.pi/skills/<prefix>-<name>/SKILL.md`; no router or hidden workflow-body output remains. The catalog `effort-workflow` entry additionally derives the Pi-target-owned `using-effort` skill at the same native skill path without making it a second catalog entry.
Origin: ADR-0167
Revised-by: ADR-0218, ADR-0251
Backing: test

### `invariant: pi-effort-session-association`

When the catalog `effort-workflow` entry and the fixed Pi target render the generated `using-effort` skill and `using_effort` tool, they explicitly associate one running process with at most one effort through direct non-terminating attach or detach at the repository root. A switch conservatively detaches the prior owner before attaching the next effort; immutable process-local snapshots, owner-checked heartbeat, restart-detached behavior, and ownership-loss cleanup keep advisory state bounded. Successful attachment activates only the three associated memory tools alongside unrelated active tools; detach, restart, missing activity, or ownership loss removes only those names. Every attached model call receives the fixed relative memory path and, when cached directory presence confirms it, the fixed managed-worktree path; failures silently omit unverified advisory facts. The runtime never infers an effort, resolves a checkout, changes CWD, queues a command, replaces or creates a conversation, writes residents directly, or uses local TUI presentation. Complete `awf` metadata remains independent. Optional presentation publishes only `{value:<canonical slug>}` or `{value:null}` through the capability-gated display-suffix event family; complete capability snapshots authoritatively withdraw support, replay is synchronous, and detach, restart, ownership loss, and shutdown clear explicitly. awf never reads or composes a name and display suffix is never routing authority.
Origin: ADR-0218
Revised-by: ADR-0225, ADR-0231, ADR-0239, ADR-0251
Backing: test

### `invariant: using-effort-skill`

The Pi target alone derives the target-owned `using-effort` skill and `awf-effort` extension from the catalog `effort-workflow` entry; neither artifact is independently selectable, and no non-Pi target renders or refers to either one. It projects the Pi-runtime-owned session-replacement protocol without becoming a second protocol authority. The skill documents exactly direct `{effort:"<canonical-slug>"}` attachment and `{detach:true}` detachment from the repository root, use of the supplied fixed relative memory and optional managed-worktree paths, explicit-only association, restart-detached state, and the display-only suffix that is never routing input; activity is neither authority nor a lock. While associated it directs agents to prefer pathless memory reads, separate exact body edits from `phase` or `next` metadata updates, and rely on automatic timestamps, without forbidding generic file tools or direct commands.
Origin: ADR-0218
Revised-by: ADR-0225, ADR-0231, ADR-0239, ADR-0251, ADR-0293
Backing: test

### `invariant: pi-effort-memory-tools`

The Pi target alone derives three closed-schema pathless associated-memory tools from the catalog `effort-workflow` entry: paginated complete-document read, exact Markdown-body edit, and mutable `phase` or `next` update. They are registered but inactive while detached, activate only with a successful association, preserve unrelated active tools, carry active-only preference guidance, and clear on detached lifecycle boundaries or advisory ownership loss. The generated client uses bounded invocation and strict protocol decoding, while the index serializes complete operations and puts mutations on Pi's shared real-path file queue; these conveniences neither prohibit direct file access nor make activity an authority or lock.
Both mutation tools additionally run one owner-scoped binary preview before execution. The client selects and strictly decodes the operation-specific closed `previewed` envelope, requiring `replacementCount` for edit preview, forbidding it for update preview, forbidding any memory fact in either, and refusing a preview envelope for a normal invocation or a normal success envelope for a preview one. Preview inserts only `--preview` into otherwise unchanged owner-scoped argv and leaves normal mutation validation, publication, and durability semantics untouched. Any preview refusal or preview transport failure becomes the thrown tool error and no mutation runs. One shared self-rendered mutation surface built from Pi's public `renderDiff` and TUI exports serves both tools: it keys one preview per tool call, immutable association, and exact arguments, redraws only the still-current key, and always replaces preview state with the authoritative result, refusal, or error, so no refusal reads as success and a bounded diff shows its truncation warning. Model-visible mutation content stays compact while structured details keep the protocol reply. Preview joins association serialization only; the file mutation queue stays reserved for the mutations themselves.
Origin: ADR-0239
Revised-by: ADR-0244, ADR-0251
Backing: test

### `invariant: pi-structured-exploration-contract`

The generated Full adapter atomically registers exactly six closed-schema profiles and Core registers exactly four, each with optional exact model routing. Every reviewer included by the selected governance footprint has a dedicated profile and tool with no kind argument: Full includes ADR, plan, and code review, while Core includes code review only. One shared review-profile factory owns their common preparation and policy, the shared `review` model-preference role governs every reviewer, and each review profile independently declares ten active calls. Implementation alone accepts optional `verificationCheckout`; preparation validates and caches one canonical accessible descendant checkout as both the child CWD and commit-policy identity, while omission retains root/root. This does not confine deliberately targeted paths or move the parent session. Exploration retains required task, breadth, and detail and declares ten active calls; grounding also declares ten, while implementation declares one and parent-batch exclusivity.
Origin: ADR-0148
Revised-by: ADR-0260, ADR-0279, ADR-0280, ADR-0292, ADR-0309
Backing: test

### `invariant: pi-subagent-model-preferences`

The generated adapter merges user-global and gitignored project-local preferences per field for the shared default, every grounding, exploration, review, and implementation role, and the small, standard, and large tiers. Completeness requires every field explicitly after merging; missing fields remain valid and visible, while malformed, overlong, unregistered, unauthenticated, unavailable, or unreadable configured fields block all implicit routing and leave valid explicit calls usable. Preference and registry state reload once for every profile invocation before pi-tools queue acquisition; no post-queue awf reload remains.
Origin: ADR-0151
Revised-by: ADR-0173, ADR-0279
Backing: test

### `invariant: pi-subagent-model-routing`

Every profile accepts only omission or an exact registry-valid provider/model-id of at most 256 printable-ASCII characters, excluding space and DEL. The schemas and preference parser derive that form from one shared pattern constant. Omission alone requests configured role routing and parent fallback; sentinel values reject with an omit-the-field repair and are never normalized, and overlong references report overlong before form rejection. Async selection reloads preferences against the session-owned live registry, reports requested and resolved routing facts, and returns a concrete model for pi-tools to validate again before execution.
Origin: ADR-0148
Revised-by: ADR-0151, ADR-0173, ADR-0176, ADR-0279
Backing: test

### `invariant: pi-subagent-model-wizard`

The /awf-subagent-models command is a TUI-only atomic wizard for the shared default, all four explicit role defaults, and the small, standard, and large tiers. It preserves scope and error display, complete cancellation without writes, live registry-gated Luna/Terra/Sol preset selection, informed manual selectors, project-local gitignore enforcement, owner-only sibling-temp replacement, stale-writer detection, cleanup, and in-memory refresh, and it writes roles and tiers together as one preference transaction.
Origin: ADR-0151
Revised-by: ADR-0173
Backing: test

### `invariant: pi-implement-role-artifact`

The generated adapter builds the implementation profile prompt from the rendered implementer agent at its `.pi/agents/` path through the shared loader, strips frontmatter, and prepends the call's commit-authority line. Preparation resolves and caches the validated checkout before dispatch, returns it as child CWD, and before and after Git snapshots use that same identity. A commit-capable unchanged HEAD names the checkout, required stopped inventory, and explicit `verificationCheckout` retry repair; a no-commit changed HEAD retains its violation without auto-reverting. The generic implementer role contract carries no Pi-only checkout-routing duty.
Origin: ADR-0177
Revised-by: ADR-0179, ADR-0260, ADR-0279, ADR-0309
Backing: test

### `invariant: pi-role-contract-loader`

Every dispatched profile loads its role contract from the rendered agent artifact through one shared loader that reads the file, strips frontmatter, prepends role-specific per-call authority, and fails with an actionable enable-and-render repair naming the role when the file is missing or its instruction body is empty. No dispatched role prose remains inline in the adapter.
Origin: ADR-0179
Revised-by: ADR-0279
Backing: test
