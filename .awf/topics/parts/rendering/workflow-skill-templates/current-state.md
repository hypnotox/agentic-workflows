Workflow-chain and task-skill template contracts: chain ordering, exploration and review behaviour, and memory checkpoints.

## Claims

### `invariant: bounded-exploration-reporting`

The rendered exploration guidance and the rendered explorer agent define adaptive breadth and grounded reporting, keep refinement sequential, and permit independent information needs to run concurrently, while Pi's per-call suffix supplies the selected breadth and report detail and makes Pi queue above ten active children in FIFO and abort-aware order.
Origin: ADR-0148
Revised-by: ADR-0179
Backing: test

### `invariant: cross-runtime-exploration-dispatch`

The core exploring skill renders for every target with one semantic breadth-and-detail protocol; the Pi target uses its awf-owned subagent_explore tool while non-Pi targets are directed to the named explorer agent as a generic target-native fresh-context exploration subagent, with no Pi tool name leaking into their output.
Origin: ADR-0148
Revised-by: ADR-0179
Backing: test

### `invariant: implementer-context-grounding`

Every managed context-calling skill and the grounding-checker agent body carry the one-sentence spill pointer naming the exact `AWF_CONTEXT_SPILL_V1` notice and the working-with-awf doc's Context spill notices subsection, the contract's single rendered home for byte-length verification and best-effort packet deletion. Brainstorming, orientation, implementation, planning, debugging, test-first, and refactor-orientation calls start with bare `awf context`; plan and implementation review request `invariants`, `all-rules`, `evidence`, and `pending`; plan/ADR resync requests `invariants`, `all-rules`, and `pending`; and ADR lifecycle requests `pending` where lifecycle detail is needed. No managed skill prescribes `--full` or `--json`, and the projection-pinning spine test classifies every context-calling skill template and expands the shared spill pointer.
Origin: ADR-0155
Revised-by: ADR-0165, ADR-0174, ADR-0187, ADR-0197
Backing: test

### `invariant: mandatory-approval-boundaries`

The rendered brainstorming skill carries mandatory first-creation confirmation before detailed design: it presents labeled `Outcome:`, `Effort title:`, and `Effort slug:` fields, asks the user to confirm all three, ends the turn without mutation, and permits the required explicit-slug creation command only after a clear response in a later turn. Brainstorming also closes with final grounded-design approval, and ADR review closes with settled-ADR approval; each final approval persists memory, presents the completed summary, explicitly requests approval, and stops. Continuation and handoff begin only after the applicable later response is persisted when an effort exists. Brainstorming also settles a proportionate simplicity contract covering scope and exclusions, structural approach and dependencies, patterns or abstractions, and checks and testing strategy. A deviation affecting behavior, scope, structure, dependencies, patterns, checks, or testing strategy stops before further mutation and returns to the user with the changed fact, why the approved approach no longer fits, affected categories, and simplest viable options; equivalent mechanical choices remain autonomous. No other chain skill renders a final approval stop, and no checkpoint creates missing ownership.
Origin: ADR-0152
Revised-by: ADR-0160, ADR-0167, ADR-0222, ADR-0226, ADR-0232
Backing: test

### `invariant: memory-checkpoint-chain-coverage`

Checkpoint guidance renders the four-step digest: it creates no effort for a minimal simple fix, merely because a boundary was reached, or because work was classified non-minimal. For non-minimal work it validates exactly one already-confirmed immutable slug and `.awf/efforts/<slug>/memory.md`; missing ownership routes back to mandatory first-creation confirmation. It confirms canonical YAML `effort: <slug>` or deprecated legacy `Effort: <slug>` identity (legacy remains only until active efforts finish), carries continuation in the effort's managed worktree when one exists with the owned path spelled primary-root-relative, and runs exactly one structured `awf effort memory update` as the sole writer of phase, next action, and time while separately appending unrecorded settled decisions and observations. It appends a handoff-log entry only after a fresh-session boundary actually exists. Routine implementation checkpoints remain after the phase-closing commit and settled report-only review, never after heading-identified tasks or helper returns; an executable `awf read plan` projection does not create a checkpoint or handoff boundary; an additional checkpoint is permitted at any safe point whose next action is independently resumable, and every checkpoint points at the workflow doc's working-memory section for authority precedence, the one-writer contract, the skeleton, and the full protocol.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0160, ADR-0164, ADR-0166, ADR-0167, ADR-0175, ADR-0186, ADR-0189, ADR-0197, ADR-0209, ADR-0213, ADR-0218, ADR-0219, ADR-0222
Backing: test

### `invariant: unified-effort-workflow-coverage`

Catalog-derived tests classify every applicable brainstorming, ADR, planning, implementation, review, checkpoint, retrospective, debugging, bugfix, TDD, coupling-audit, exploration, orientation, roadmap, and effort-workflow skill into closed semantic roles and render every enabled target. Brainstorming, debugging, and roadmap graduation own first-creation discovery: each presents labeled `Outcome:`, `Effort title:`, and `Effort slug:` fields through the shared confirmation and permits `awf effort new --slug <confirmed-slug> "<confirmed-title>"` only after a clear later response and before mutation. Every downstream or support role contains no creation command; downstream work requires already-confirmed ownership and routes absence back to first-creation confirmation, report support remains read-only toward memory, and exploration and orientation never create an effort. Existing efforts resume under fixed identity without title reconfirmation only inside the confirmed outcome. Each path carries the minimal-fix exception where applicable, always-owned memory and exact slug/path continuity, repository-authority precedence, the standalone-memory ban, and the one-writer/report-only-child contract through its operative pre-mutation contract and the workflow doc's working-memory section. Terminal review conditionally integrates and removes managed topology, renews review after divergence, and retrospective finishes last. The core `effort-workflow` entry guide composes the workflow document and shared checkpoint partials as the single lifecycle-policy home, requires an existing confirmed effort, directs runtimes without supplied effort paths into the exact awf-managed worktree, permits explicit-path runtimes to remain at repository root and target that worktree by path, names no runtime-specific tool, and adds no second policy copy.
Origin: ADR-0175
Revised-by: ADR-0187, ADR-0197, ADR-0218, ADR-0222, ADR-0225, ADR-0226
Backing: test

### `invariant: effort-workflow`

Core `effort-workflow` is the single selectable cross-target entry guide for a non-minimal awf effort: new scaffolds select it by default, existing adopter selections change only through explicit enablement, and every enabled target renders it. It composes the workflow document and shared checkpoint policy, uses ordinary awf effort commands, directs runtimes without supplied effort paths to enter the exact existing `.awf/worktrees/<slug>` managed worktree through native persistent checkout or context tooling, permits explicit-path runtimes to remain at the repository root and target that worktree by path, and preserves structured checkpoint, review, integration, removal, retrospective, and finish ordering. It never creates standalone memory or a parallel harness-owned worktree, infers topology, treats activity as authority, names a runtime, or exposes a runtime-specific effort tool.
Origin: ADR-0218
Revised-by: ADR-0225
Backing: test

### `invariant: memory-log-consumer-coverage`

The shared review spine carries the consensus-adherence check: with pasted user-provenance consensus entries in the brief, a deviation from a user entry is a `user-decision` finding citing the deviating passage and carrying the "we decided X; during <phase> we found Z; recommend Y, approve?" escalation, and an entry-free brief leaves the check idle. The reviewing-adr, reviewing-plan, and reviewing-impl dispatch briefs paste user entries verbatim including whatever `Record:` blocks exist while resync stays narrowed, and the retrospective reads the observation and decision logs as primary input with recurrence extended across the effort's sessions.
Origin: ADR-0186
Revised-by: ADR-0197
Backing: test

### `invariant: workflow-transitions-advisory`

Rendered workflow skills describe catalog relationships only as recommendations. Any enabled skill may be used when its purpose fits, while controls within a selected skill remain mandatory.
Origin: ADR-0167
Backing: test

### `invariant: phase-transaction-ownership`

A rendered plan phase is one independently green coherent implementation transaction with an explicit per-phase inline or subagent-driven owner; heading-identified tasks are ordered steps rather than completion state or default dispatch, review, checkpoint, or commit boundaries. A fresh phase or task owner may consume bounded plan-v2 closure, but its generated scope notice, Phase close, Advances, and Completes remain phase-owner context and never transfer commit, review, checkpoint, handoff, helper, or outcome authority. One commit-capable implementer owns a complete subagent-driven phase from a known green baseline through staged check, gate, and Phase close commit, while the parent owns inline integration, sequential commit-disabled batch helpers, report-only review settlement, phase checkpointing, and explicit dirty-state recovery without blind task-level succession.
Origin: ADR-0166
Revised-by: ADR-0213, ADR-0217
Backing: test

### `invariant: plan-task-detail-modes`

The rendered plan-authoring skill, plan reviewer, implementation-plans README, and plan template use qualifying implementation-ready instructions as the default task-content form; require `Latitude: exact` for machine-consumed configuration and manifests, contract-bearing declarations, fixtures, golden output, commands, mechanical replacements, required literal prose, and batch representative and edge transformations; and permit that marker voluntarily elsewhere. Their plan-v2 vocabulary defines contiguous `Applying:` and `Context:` task fields as nonempty JSON string arrays that are omitted rather than written as `[]`, uses retained pending ADR slugs with V4 stable Decision slugs or frozen pre-V4 `#N` selectors, requires slugged `dod:` Definition-of-done bullets, and assigns outcome contribution and final ownership through nonempty phase `Advances:` and `Completes:` arrays. Review checks substantive Applying coverage, Context misuse, and final Completes ownership while treating Proposed-plan coverage findings as advisory and historical Decision prose as distinct from current-state authority. Task projections keep their scope notice, Phase close, Advances, and Completes as phase-owner context without transferring task-helper authority. The surfaces also define contiguous task fields for exactness, spikes, batches, affected paths, and deterministic post-checks; require `Paths:` whenever scope is ambiguous, always including a batch; require `Post-check:` for every batch and every glob or pathspec scope; preserve the no-placeholder boundary for implementation tasks; forbid conditional and optional tasks; require one coherent green transaction and an inline or subagent-driven owner per phase; and keep any helper partition exhaustive, path-disjoint, shared-file-safe, and command-confined. A spike is question-only, records its answer in Notes, cannot own a phase, and sequences dependent work into a later phase. Every surface renders coherently with empty variables.
Origin: ADR-0148
Revised-by: ADR-0157, ADR-0166, ADR-0211, ADR-0217
Backing: test

### `invariant: reviewers-report-only`

The rendered reviewer agent templates and the shared review spine instruct the reviewer only to read, run its lenses, and report findings; none contains a directive to apply fixes, commit, or loop a re-review.
Origin: ADR-0148
Backing: test

### `invariant: skill-prose-tool-agnostic`

Every rendered skill and agent body is free of runtime tool-name tokens; a case-insensitive, word-anchored scan rejects subagent_type, the phrase subagent type, agent tool, skill tool, AskUserQuestion, the backticked-agent prompt phrasing, and the backticked or phrased file-operation tool names for write, edit, and read, while plain action verbs and the shell grep stay allowed.
Origin: ADR-0148
Backing: test

### `invariant: workflow-chain-adr-before-plan`

The rendered workflow.md workflow-chain string presents the ADR step before the plan step.
Origin: ADR-0148
Revised-by: ADR-0157
Backing: test

### `invariant: workflow-chain-surfaces-resync`

The rendered workflow chain names the plan-to-ADR resync step explicitly rather than hiding it from the high-level chain.
Origin: ADR-0148
Backing: test

### `invariant: maintainable-code-stage-coverage`

Brainstorming, ADR proposal, coupling audit, plan writing, test-driven development, inline plan execution, direct execution, subagent-driven development, and bug fixing each render a concise stage-specific obligation pointing to the mandatory maintainable-code guide: designs settle models and boundaries, plans make them executable, and implementation preserves or explicitly reassesses them instead of bolting correctness onto an unsuitable abstraction. Stage-local simplest-sufficient obligations cover brainstorming, plan writing, TDD, direct execution, inline plan execution, subagent-driven execution, and bug fixing; plans record approved choices instead of expanding them.
Origin: ADR-0168
Revised-by: ADR-0232
Backing: test

### `invariant: maintainable-code-subagent-contract`

Every scoped implementation brief carries only the task-relevant semantic boundaries and ownership, representations and translation points, dependency direction, preparatory-refactor decision, prohibited bolt-on shortcuts, and validation expectations; the implementer preserves those choices or reports invalidating source facts without becoming a second planner, broadening scope, or performing unrelated cleanup, and inline plan execution extracts the same context for its current task.
Origin: ADR-0168
Backing: test

### `invariant: deliberate-subagent-model-selection`

Every final governed subagent dispatch chooses the smallest model expected to complete reliably from the semantic small, standard, and large tiers and reconsiders escalation after uncertainty, failed reasoning, or widened scope. Pi uses configured role routing only by omitting the model field and overrides deliberately with an exact tier reference; other targets select a target-native model explicitly where supported and otherwise use the harness default with a visible unsupported-selection note. Generic rendered guidance contains no Pi tool name, provider-specific model reference, price, context limit, or registry catalog, and every affected template renders coherently with empty variables. Each governed dispatch section carries the compressed tier-and-escalation rule with its target branch rule, and the full tier definitions render once per target in the agent guide's workflow section, sourced from the shared model-selection partial.
Origin: ADR-0173
Revised-by: ADR-0190
Backing: test

### `invariant: implementer-role-contract`

The rendered implementer agent body states its two authority modes, that the dispatched task is the complete scope, that the agent guide's invariants and commands bind while its skill catalog and chain routing do not, that reaching green is the job and no assertion or golden may be weakened to hide a failure, that no interactive channel exists so escalation is a returned inventory, the commit-capable owner's explicit-stage, staged-check, gate, single-commit procedure, and a closed two-outcome return whose stopped outcome requires working-tree status, work completed, work remaining, the named failing check with its actual output, and what was already tried. The stopped outcome may instead report an approval-requiring invalidating source fact while retaining the shared inventory and closed two-outcome contract. The subagent-driven-development and executing-plans skills name that agent in every dispatch branch, and their parent-facing imperatives for raising concerns, preserving the plan's settled design, running the context command, and inventorying batch returns each carry an explicit subject. The body renders coherently with unset data and carries no runtime tool-name token.
Origin: ADR-0177
Revised-by: ADR-0179, ADR-0232
Backing: test

### `invariant: maintainable-code-review-lenses`

Plan review checks that structural choices and necessary enabling refactors are explicit, ordered, bounded, approved or durably dispositioned when larger, and verifiable; code review checks cohesion, coupling, dependency direction, representation leakage, duplicated policy, testability, needless indirection, and settled-design conformance; ADR review applies the same structural lens only when a decision changes a semantic model, representation, ownership, module, or package boundary, dependency direction, or comparable structural contract. All reviewer agents remain report-only. Plan and code review flag unapproved or unjustified machinery and do not demand additions merely because more abstraction, cleanup, testing, or validation is imaginable.
Origin: ADR-0168
Revised-by: ADR-0232
Backing: test

### `invariant: explorer-and-grounding-role-contracts`

The rendered explorer body defines its report-only identity, one information need with no bundling or recursive delegation, concurrent independent needs with sequential refinement, breadth ordered targeted < bounded < broad as an adaptive maximum with its project search universe, report detail ordered paths < summary < analysis independent of breadth, file:line grounding, the distinction between not-found, inconclusive, and unverified outcomes with the exact not-found opening, final-report-only output, and statelessness across calls. The rendered grounding-checker body defines its report-only identity, that it works only from its brief and never edits the working memory that brief may name, its verification obligations across factual premises, unstated assumptions, altitude, and convention fit, and a closed finding schema whose confidence field distinguishes verified, interpreted, and unverified. The exploring and brainstorming skills each name their dispatched agent in the branch that dispatches a target-native subagent, and neither rendered body carries per-call or runtime-specific text. The grounding-checker body grounds guide-first through the shared orientation ladder partial.
Origin: ADR-0179
Revised-by: ADR-0187
Backing: test

### `invariant: orienting-single-home`

The orienting support skill is the single home of the orientation procedure: its rendered body defines the four invocation moments, the guide-first grounding ladder shared as a partial with the grounding-checker contract, multi-child report-only exploration dispatch with one information need per child, the managed context discipline, and effort-resume revalidation that reads the memory file whole and resolves discrepancies in favor of the repository; the skill is single-pass, never a chain gate, never creates an effort, and never commits. Brainstorming's first step invokes it, and proposing-adr and writing-plans carry advisory pointers.
Origin: ADR-0187
Backing: test

### `invariant: semantic-rendering-review`

Every enabled target that renders the planning skill or plan/code reviewer receives instructions that schedule or inspect a focused human check of affected generated prose for contradictory fragments, concept-preserving paraphrase, and intentional literal placeholder syntax. The focused target-render and empty-data tests prove those instructions render without an unresolved no-value token while missingkey=zero and coherent generic empty fallbacks remain unchanged; the instructions add no synonym, contradiction, or placeholder-intent inference and no universal output-language validator.
Origin: ADR-derive-render-completeness-from-output-authority
Backing: test
