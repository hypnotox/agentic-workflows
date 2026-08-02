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

The rendered brainstorming and ADR-review skills close with the mandatory approval protocol: persist memory, present the completed summary, explicitly request approval, and stop. Continuation and handoff begin only after explicit approval is persisted; no other chain skill renders an approval stop.
Origin: ADR-0152
Revised-by: ADR-0160, ADR-0167
Backing: test

### `invariant: memory-checkpoint-chain-coverage`

Checkpoint guidance renders the four-step digest: it creates no effort for a minimal simple fix or merely because a boundary was reached, and once the outcome is concrete and non-minimal it validates exactly one immutable slug and `.awf/efforts/<slug>/memory.md`, confirms `Effort: <slug>`, carries continuation in the effort's managed worktree when one exists with the owned path spelled primary-root-relative, and updates phase, next action, time, any unrecorded settled decision, and any observation in one writer-owned batch. It appends a handoff-log entry only after a fresh-session boundary actually exists. Routine implementation checkpoints remain after the phase-closing commit and settled report-only review, never after heading-identified tasks or helper returns; an executable plan projection does not create a checkpoint boundary; an additional checkpoint is permitted at any safe point whose next action is independently resumable, and every checkpoint points at the workflow doc's working-memory section for authority precedence, the one-writer contract, the skeleton, and the full protocol.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0160, ADR-0164, ADR-0166, ADR-0167, ADR-0175, ADR-0186, ADR-0189, ADR-0197, ADR-0209, ADR-parsed-plan-artifacts-and-executable-projections
Backing: test

### `invariant: unified-effort-workflow-coverage`

Catalog-derived tests classify every applicable brainstorming, ADR, planning, implementation, review, checkpoint, retrospective, debugging, bugfix, TDD, coupling-audit, exploration, orientation, and roadmap skill and render every enabled target. Each path carries the minimal-fix exception where applicable, mandatory concrete non-minimal slugged effort with always-owned memory, and exact slug/path continuity, and keeps repository-authority precedence, the standalone-memory ban, and the one-writer/report-only-child contract in force through its procedure preamble and the workflow doc's working-memory section. Terminal review conditionally integrates and removes managed topology, renews review after divergence, and retrospective finishes last.
Origin: ADR-0175
Revised-by: ADR-0187, ADR-0197
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

A rendered plan phase is one independently green coherent implementation transaction with an explicit per-phase inline or subagent-driven owner; heading-identified tasks are ordered steps rather than completion state or default dispatch, review, checkpoint, or commit boundaries. A fresh phase or task owner may consume `awf read plan`'s executable closure without changing ownership. One commit-capable implementer owns a complete subagent-driven phase from a known green baseline through staged check, gate, and Phase close commit, while the parent owns inline integration, sequential commit-disabled batch helpers, report-only review settlement, phase checkpointing, and explicit dirty-state recovery without blind task-level succession.
Origin: ADR-0166
Revised-by: ADR-parsed-plan-artifacts-and-executable-projections
Backing: test

### `invariant: plan-task-detail-modes`

The rendered plan-authoring skill, plan reviewer, implementation-plans README, and plan template use qualifying implementation-ready instructions as the default task-content form; require `Latitude: exact` for machine-consumed configuration and manifests, contract-bearing declarations, fixtures, golden output, commands, mechanical replacements, required literal prose, and batch representative and edge transformations; and permit that marker voluntarily elsewhere. They define contiguous task fields for exactness, spikes, batches, affected paths, and deterministic post-checks; require `Paths:` whenever scope is ambiguous, always including a batch; require `Post-check:` for every batch and every glob or pathspec scope; preserve the no-placeholder boundary for implementation tasks; forbid conditional and optional tasks; require one coherent green transaction and an inline or subagent-driven owner per phase; and keep any helper partition exhaustive, path-disjoint, shared-file-safe, and command-confined. A spike is question-only, records its answer in Notes, cannot own a phase, and sequences dependent work into a later phase. Every surface renders coherently with empty variables.
Origin: ADR-0148
Revised-by: ADR-0157, ADR-0166, ADR-0211
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

Brainstorming, ADR proposal, coupling audit, plan writing, test-driven development, inline plan execution, direct execution, subagent-driven development, and bug fixing each render a concise stage-specific obligation pointing to the mandatory maintainable-code guide: designs settle models and boundaries, plans make them executable, and implementation preserves or explicitly reassesses them instead of bolting correctness onto an unsuitable abstraction.
Origin: ADR-0168
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

The rendered implementer agent body states its two authority modes, that the dispatched task is the complete scope, that the agent guide's invariants and commands bind while its skill catalog and chain routing do not, that reaching green is the job and no assertion or golden may be weakened to hide a failure, that no interactive channel exists so escalation is a returned inventory, the commit-capable owner's explicit-stage, staged-check, gate, single-commit procedure, and a closed two-outcome return whose stopped outcome requires working-tree status, work completed, work remaining, the named failing check with its actual output, and what was already tried. The subagent-driven-development and executing-plans skills name that agent in every dispatch branch, and their parent-facing imperatives for raising concerns, preserving the plan's settled design, running the context command, and inventorying batch returns each carry an explicit subject. The body renders coherently with unset data and carries no runtime tool-name token.
Origin: ADR-0177
Revised-by: ADR-0179
Backing: test

### `invariant: maintainable-code-review-lenses`

Plan review checks that structural choices and necessary enabling refactors are explicit, ordered, bounded, approved or durably dispositioned when larger, and verifiable; code review checks cohesion, coupling, dependency direction, representation leakage, duplicated policy, testability, needless indirection, and settled-design conformance; ADR review applies the same structural lens only when a decision changes a semantic model, representation, ownership, module, or package boundary, dependency direction, or comparable structural contract. All reviewer agents remain report-only.
Origin: ADR-0168
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
