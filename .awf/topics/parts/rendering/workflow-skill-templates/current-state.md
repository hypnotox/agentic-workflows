Workflow-chain and task-skill template contracts: chain ordering, exploration and review behaviour, and memory checkpoints.

## Claims

### `invariant: independent-workflow-escalation`

Workflow intake independently evaluates brainstorming for material choices, continuity for durable coordination or resumability, grounding for broad or uncertain repository premises, ADRs for load-bearing decisions or changed active claims, plans for useful sequencing, and implementation review for independent assurance value. Material fact changes re-evaluate only affected triggers before further mutation without invalidating prior valid work. Effort-free and effort-backed work may each skip or receive review under the risk trigger; universal authority, documentation, verification, and commit obligations remain. No classifier, checklist, router, or new runtime mechanism is introduced.
Origin: ADR-0243
Backing: test


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
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0155
Revised-by: ADR-0165, ADR-0174, ADR-0187, ADR-0197, ADR-0243
Backing: test

### `invariant: mandatory-approval-boundaries`

The rendered effort-workflow owns first-creation confirmation when continuity fires: it presents labeled `Outcome:`, `Effort title:`, and `Effort slug:` fields, asks the user to confirm all three, ends the turn without mutation, and permits explicit-slug creation only after a clear later response. Brainstorming closes with explicit final grounded-design approval whenever brainstorming fires, and ADR review closes with settled-ADR approval; each presents its completed summary, requests approval, and stops, persisting only when an effort exists. Brainstorming settles a proportionate simplicity contract covering scope and exclusions, structure and dependencies, abstractions, and verification. No checkpoint creates ownership, and effort-free approval omits memory rather than fabricating it.
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0152
Revised-by: ADR-0160, ADR-0167, ADR-0222, ADR-0226, ADR-0232, ADR-0240, ADR-0243
Backing: test

### `invariant: authority-guided-implementation-autonomy`

One variable-free shared prose partial is directly included once by every named implementation consumer. It requires authority-preserving reasoned correction and diagnosis before escalation, preserves the approved outcome, material scope, settled durable boundaries, and required verification, and forbids weakened oracles and unrelated cleanup. Inline owners amend stale mutable instructions and Notes; delegated owners report deviations for report-only review and focused parent reconciliation before checkpointing or later execution. Implementation review routes authority-determined remedies by classification and keeps one verify pass. Empty-data rendering remains coherent.
Origin: ADR-0240
Backing: test

### `invariant: memory-checkpoint-chain-coverage`

Checkpoint guidance never creates an effort. Effort-backed checkpoints validate one immutable slug and primary-root-relative `.awf/efforts/<slug>/memory.md`, accept canonical YAML or deprecated legacy identity, continue in the managed worktree when present, and run exactly one structured memory update as the sole writer while separately appending decisions and observations. Effort-free work omits persistence. Handoff logging follows only an actual fresh-session boundary; task headings and projections never create checkpoint authority. Every checkpoint points to the workflow document for repository precedence, the one-writer contract, and the full protocol.
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0152, ADR-0160, ADR-0164, ADR-0166, ADR-0167, ADR-0175, ADR-0186, ADR-0189, ADR-0197, ADR-0209, ADR-0213, ADR-0218, ADR-0219, ADR-0222, ADR-0243
Backing: test

### `invariant: unified-effort-workflow-coverage`

Catalog-derived tests render every applicable workflow for every enabled target and prove continuity is independent of brainstorming, artifacts, implementation, and review. Only effort-workflow contains the creation command and owns later-turn confirmation, resume, checkpoints, integration, divergence handling, deferred artifact transitions, topology removal, retrospective routing, and finish. Other workflows may run effort-free, carry validated read-only effort context only when continuity fired, and never create or finalize topology. Reviewing-impl owns assurance only. All four effort/review completion routes exist, and divergent integration activates review before removal. Repository authority, the standalone-memory ban, and the one-writer/report-only-child contract remain universal.
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0175
Revised-by: ADR-0187, ADR-0197, ADR-0218, ADR-0222, ADR-0225, ADR-0226, ADR-0243
Backing: test

### `invariant: effort-workflow`

Core `effort-workflow` is the selectable cross-target lifecycle owner used only when durable continuity materially helps or an existing effort resumes or finishes. It owns later-turn three-field confirmation, creation, validation, managed-worktree context, checkpoints, integration, divergence-triggered review, deferred artifact closure, topology removal, retrospective routing, and finish. It directs runtimes without supplied paths into the exact managed worktree and permits explicit-path runtimes to target it from the root. It never creates standalone memory or parallel topology, infers state, treats activity as authority, or names a runtime-specific tool.
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0218
Revised-by: ADR-0225, ADR-0243
Backing: test

### `invariant: memory-log-consumer-coverage`

The shared review spine carries the consensus-adherence check when user-provenance entries exist. Artifact and implementation review briefs paste available user entries verbatim, including `Record:` blocks; effort-free briefs omit memory evidence without fabricating consensus and still carry outcome, constraints, summary, range, and verification. Resync remains narrowed, and retrospective consumes effort observations and decisions only for effort-backed work.
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0186
Revised-by: ADR-0197, ADR-0243
Backing: test

### `invariant: workflow-transitions-advisory`

Rendered workflow skills describe catalog relationships only as recommendations. Any catalog skill may be used when its purpose fits, while controls within a selected skill remain mandatory.
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0167
Revised-by: ADR-0243, ADR-house-standard-configuration-expresses-repo-facts-only
Backing: test

### `invariant: phase-transaction-ownership`

A rendered plan phase is one independently green coherent implementation transaction with an explicit per-phase inline or subagent-driven owner; heading-identified tasks are ordered steps rather than completion state or default dispatch, review, checkpoint, or commit boundaries. A fresh phase or task owner may consume bounded plan-v2 closure, but its generated scope notice, Phase close, Advances, and Completes remain phase-owner context and never transfer commit, review, checkpoint, handoff, helper, or outcome authority. One commit-capable implementer owns a complete subagent-driven phase from a known green baseline through staged check, gate, and Phase close commit, while the parent owns inline integration, sequential commit-disabled batch helpers, report-only review settlement, phase checkpointing, and explicit dirty-state recovery without blind task-level succession.
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0166
Revised-by: ADR-0213, ADR-0217, ADR-0243
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

Every scoped implementation brief carries only the task-relevant semantic boundaries and ownership, representations and translation points, dependency direction, preparatory-refactor decision, prohibited bolt-on shortcuts, and validation expectations; the implementer makes authority-preserving reasoned detail deviations with a structured completed report without becoming a second planner, broadening scope, or performing unrelated cleanup, and inline plan execution extracts the same context for its current task.
Origin: ADR-0168
Revised-by: ADR-0240
Backing: test

### `invariant: deliberate-subagent-model-selection`

Every final governed subagent dispatch retains its operative smallest-reliable-tier and escalation rule: Pi omits the model field for configured role routing and overrides deliberately with an exact tier reference; other targets select a target-native model where supported and otherwise visibly use the harness default. Generic rendered guidance contains no Pi tool name, provider-specific model reference, price, context limit, or registry catalog, and every affected template renders coherently with empty variables. The full semantic small, standard, and large tier definition occurs exactly once in docs/working-with-awf.md, while AGENTS.md does not duplicate it.
Origin: ADR-0173
Revised-by: ADR-0190, ADR-0241
Backing: test

### `invariant: implementer-role-contract`

The rendered implementer agent body states its two authority modes, that the dispatched task is the complete scope, that the agent guide's invariants and commands bind while its skill catalog and chain routing do not, that reaching green is the job and no assertion or golden may be weakened to hide a failure, that no interactive channel exists so escalation is a returned inventory, the commit-capable owner's explicit-stage, staged-check, gate, single-commit procedure, and a closed two-outcome return whose stopped outcome requires working-tree status, work completed, work remaining, the named failing check with its actual output, and what was already tried. The completed outcome inventories deviations or `none`; stopped reserves persistent verification failure or a narrow authority, outcome, scope, ambiguity, or safety boundary while retaining the shared inventory and closed two-outcome contract. The subagent-driven-development and executing-plans skills name that agent in every dispatch branch, and their parent-facing imperatives for raising concerns, preserving the plan's settled design, running the context command, and inventorying batch returns each carry an explicit subject. The body renders coherently with unset data and carries no runtime tool-name token.
Origin: ADR-0177
Revised-by: ADR-0179, ADR-0232, ADR-0240
Backing: test

### `invariant: maintainable-code-review-lenses`

Plan review checks that structural choices and necessary enabling refactors are explicit, ordered, bounded, approved or durably dispositioned when larger, and verifiable; code review checks cohesion, coupling, dependency direction, representation leakage, duplicated policy, testability, needless indirection, and settled-design conformance; ADR review applies the same structural lens only when a decision changes a semantic model, representation, ownership, module, or package boundary, dependency direction, or comparable structural contract. All reviewer agents remain report-only. Plan and code review flag unapproved or unjustified machinery and do not demand additions merely because more abstraction, cleanup, testing, or validation is imaginable.
Origin: ADR-0168
Revised-by: ADR-0232
Backing: test

### `invariant: explorer-and-grounding-role-contracts`

The explorer remains the report-only owner of one bounded repository information need. The unchanged grounding-checker remains report-only and verifies factual premises, unstated assumptions, altitude, convention fit, and confidence from its brief. The reusable grounding support skill owns guide-first managed context, self-contained brief construction, target-native checker dispatch, and mechanical/reasoned/user-decision finding classification from any invoking workflow; it is advisory, single-pass, effort-noncreating, and never a chain prerequisite. Brainstorming refers to grounding conditionally but does not own or directly dispatch the checker. Generic rendered bodies contain no runtime-specific tool name.
This claim reflects independent trigger judgment and the single-home effort lifecycle.
Origin: ADR-0179
Revised-by: ADR-0187, ADR-0243
Backing: test

### `invariant: orienting-single-home`

The orienting support skill is the single home of the orientation procedure: its rendered body defines the four invocation moments, the guide-first grounding ladder shared as a partial with the grounding-checker contract, multi-child report-only exploration dispatch with one information need per child, the managed context discipline, and effort-resume revalidation that reads the memory file whole and resolves discrepancies in favor of the repository; the skill is single-pass, never a chain gate, never creates an effort, and never commits. Brainstorming's first step invokes it, and proposing-adr and writing-plans carry advisory pointers.
Origin: ADR-0187
Backing: test

### `invariant: semantic-rendering-review`

Every enabled target that renders the planning skill or plan/code reviewer receives instructions that schedule or inspect a focused human check of affected generated prose for contradictory fragments, concept-preserving paraphrase, and intentional literal placeholder syntax. The focused target-render and empty-data tests prove those instructions render without an unresolved no-value token while missingkey=zero and coherent generic empty fallbacks remain unchanged; the instructions add no synonym, contradiction, or placeholder-intent inference and no universal output-language validator.
Origin: ADR-0235
Backing: test
