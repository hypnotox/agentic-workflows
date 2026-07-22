The templates tree holds the embedded skill, agent, doc, and adapter template source. The claims below capture the current template-content contracts.

## Claims

### `invariant: agents-doc-section-parity`

The agents-doc template's awf:section marker names match its catalog-declared section list exactly and in order, so a guide section added to one but not the other fails rather than half-landing with a broken override path.
Origin: ADR-0069
Backing: test

### `invariant: agentsdoc-parts`

The agent-guide's you-and-this-project and identity section bodies can be overridden by convention parts placed under parts/agents-doc/, and with no override and empty invariants and doc-map data the guide still renders complete adopter-neutral prose with no <no value> token.
Origin: ADR-0009
Backing: test

### `invariant: bootstrap-checksum`

The rendered `awf-bootstrap.sh` performs a SHA-256 checksum verification of the downloaded archive before it installs the binary, so the download is always integrity-checked ahead of use.
Origin: ADR-0040
Backing: test

### `invariant: bootstrap-env-override`

The rendered bootstrap script's version assignment is the default-expansion form AWF_VERSION set to the pattern that prefers a pre-set AWF_VERSION and otherwise expands to the rendering binary's version, so an environment override wins and, absent one, the script resolves exactly the version of the binary that rendered it.
Origin: ADR-0085
Backing: test

### `invariant: bootstrap-local-first`

The rendered bootstrap installer probes for an awf binary already on PATH before downloading anything. When a local binary reports exactly the pinned target version, the script uses it and exits before reaching any download step.
Origin: ADR-0049
Backing: test

### `invariant: bootstrap-stdout-path-only`

The rendered bootstrap installer writes only the resolved binary path to standard output. Every diagnostic line is a comment or is redirected to standard error, so nothing but the binary path reaches standard output.
Origin: ADR-0049
Backing: test

### `invariant: bounded-exploration-reporting`

The rendered exploration guidance and Pi's fixed prompt define adaptive breadth and grounded reporting, keep refinement sequential, permit independent information needs to run concurrently, and make Pi queue above ten active children in FIFO and abort-aware order.
Origin: ADR-0132
Revised-by: ADR-0141
Backing: test

### `invariant: catalog-template-sweep`

A catalog-derived loop renders every standard skill and agent template under empty data - iterating the catalog itself rather than a hand-maintained list - and fails on any leak residue or any skill cross-reference in the output that the artifact has not declared.
Origin: ADR-0080
Backing: test

### `invariant: commit-scope-single-storage`

No file under the embedded templates references `.vars.commitScope`, and the catalog `vars:` block carries no commitScope descriptor; every rendered commit-scope mention derives from `audit.allowedScopes` through the commitScopes render-context key.
Origin: ADR-0051
Backing: test

### `invariant: conditional-fallback-case-guard`

Every standard skill or agent template whose post-include-expansion source contains a conditional action - if, with, or range - must have a hand-authored unset-data case in the fallback case list, and the guard names any template missing one.
Origin: ADR-0080
Backing: test

### `invariant: cross-runtime-exploration-dispatch`

The core exploring skill renders for every target with one semantic breadth-and-detail protocol; the Pi target uses its awf-owned subagent_explore tool while non-Pi targets are directed to a generic target-native fresh-context exploration subagent, with no Pi tool name leaking into their output.
Origin: ADR-0132
Backing: test

### `invariant: docs-section-parity`

For every non-mandatory catalog doc, the section names declared in the catalog exactly match the set of awf:section marker blocks in that doc's template, and each doc renders from its template defaults with no leaked no-value token.
Origin: ADR-0011
Backing: test

### `invariant: document-map-lists-mandatory-docs`

The document-map section of the rendered `AGENTS.md` always cites every mandatory document-map doc (including the workflow guide, the documentation standard, and the agent-guide authoring standard) with its full title, link, and catalog description, regardless of the project's `docs:` array contents.
Origin: ADR-0043
Backing: test

### `invariant: empty-init-coherent-render`

A non-interactive `awf init` with no answers renders artifacts that contain no empty inline code spans, no tables lacking body rows, and no list-introduction sentence followed by nothing, so every artifact degrades to coherent prose.
Origin: ADR-0045
Backing: test

### `invariant: glossary-table-forced`

No convention part can replace the rendered glossary terms table; the only part-override surfaces on the glossary doc are the prepend and append sections.
Origin: ADR-0089
Backing: test

### `invariant: glossary-terms-sorted`

The rendered glossary table orders its rows case-insensitively by term regardless of the authored map order, and two sidecars carrying the same entries in different order render byte-identically.
Origin: ADR-0089
Backing: test

### `invariant: glossary-terms-validated`

An empty term, an empty, null, or non-string meaning, an interior newline in a term or meaning, a non-string map key, or a case-insensitive duplicate term in the glossary sidecar fails the render, naming the sidecar path and the offending key.
Origin: ADR-0089
Backing: test

### `invariant: golden-test-completeness`

Every standard catalog skill has a per-artifact Test<Skill>Template function and every catalog agent a Test<Agent>Agent function in the project package's test source, verified by a source scan.
Origin: ADR-0080
Backing: test

### `invariant: guide-scopes-derived`

The agent-guide template renders its commit-scope mention from the root commit-scopes render key rather than any hand-written scope list in the agents-doc data, and the mention degrades to generic Conventional Commits prose when no scopes are configured.
Origin: ADR-0055
Backing: test

### `invariant: hook-payloads-fallback-safe`

With checkCmd, gateCmd, gateCmdFull, and commitGateCmd all unset, every rendered hook payload is a runnable script whose commands degrade to the generic awf forms, carrying no unresolved-value token.
Origin: ADR-0048
Backing: test

### `invariant: local-base-publication-safe`

The base skill and agent templates render leak-free, with no <no value> and no marker or leak residue, under empty data and no content part.
Origin: ADR-0068
Backing: test

### `invariant: local-doc-base-publication-safe`

The base doc template renders leak-free under empty data and no content part, producing generic prose with no unresolved-value token, section marker, or leak residue.
Origin: ADR-0091
Backing: test

### `invariant: memory-checkpoint-chain-coverage`

Every non-terminal chain-node skill plus the bugfix and debugging task skills complete the working-memory update before displaying the completed phase, immediate next action, and exact memory path as an intervention point. The two implementation skills also checkpoint independently resumable tasks, and retrospective alone carries the memory-file deletion step.
Origin: ADR-0069
Revised-by: ADR-0145
Backing: test

### `invariant: no-doc-path-vars`

No template under templates/ references any of the removed doc-path or project-specific vars (workflowDoc, debuggingDoc, pitfallsDoc, roadmapDoc, stateDocsPath, oracleStateDoc, autonomousAdrRef, hostGitAdrRef, keyInvariantAdrRef, noDivingAdrRef, perTaskReviewAdrRef); doc references are supplied through the layout instead.
Origin: ADR-0013
Backing: test

### `invariant: plan-task-detail-modes`

The rendered plan-authoring skill, plan reviewer, implementation-plans README, and agent guide accept exact content/diffs or implementation-ready pseudocode with a closed application contract, require exact form for machine-consumed and other contract-bearing representations, preserve the specialized batch task and no-placeholder boundary, and render coherently with empty variables.
Origin: ADR-0142
Backing: test

### `invariant: pi-dedicated-grounding-dispatch`

In the generated Pi extension and skills, brainstorming's grounding check dispatches through the dedicated grounding tool while general exploration and coupling audits use the exploration tool, and no non-Pi target's rendered output contains either Pi subagent tool name.
Origin: ADR-0125
Backing: test

### `invariant: pi-extension-editor-quiet-strip`

Every governed Pi extension file carries the ts-nocheck directive on the line immediately after the provenance banner, and the container test harness deterministically strips that exact directive from every extension TypeScript file in its ephemeral copy after source copy and before running the TypeScript compiler.
Origin: ADR-0126
Revised-by: ADR-0145
Backing: test

### `invariant: pi-session-handoff-public-contract`

The generated Pi handoff extension exposes exactly the closed memoryPath and bounded kickoff schema, confines canonical no-symlink paths to regular files below .awf/memory, requires a persisted TUI and an exclusive trustworthy tool batch, keeps one correlated pending request, queues its private command, and terminates the calling model turn.
Origin: ADR-0145
Backing: test

### `invariant: pi-session-handoff-workflow`

Pi-rendered checkpoint guidance automatically invokes handoff_session alone after the durable visible summary at phase and intermediate implementation checkpoints, while non-Pi targets retain the checkpoint and continue without naming the unsupported tool.
Origin: ADR-0145
Backing: test

### `invariant: pi-structured-exploration-contract`

The generated Pi extension exposes exactly four closed-schema roles, each with optional exact model routing; exploration retains required task, breadth, and detail and runs through the ten-active FIFO limiter without changing the other process boundaries.
Origin: ADR-0132
Revised-by: ADR-0141
Backing: test

### `invariant: pi-subagent-model-routing`

Every Pi subagent role accepts an optional exact provider/model-id, inherits the parent on omission, rejects unknown or unauthenticated explicit choices without fallback before queueing, inherits thinking for child clamping, and reports requested and actual models.
Origin: ADR-0141
Backing: test

### `invariant: pi-implementation-batch-exclusivity`

Pi correlates each tool preflight with the current leaf assistant tool-call id, blocks every member of a reconstructable batch that mixes implementation with siblings, and blocks only implementation when trustworthy batch context is unavailable.
Origin: ADR-0141
Backing: test

### `invariant: pi-subagent-failure-details`

In the generated Pi extension, expected failures that occur after a child process has started return a marked error result that preserves bounded progress and diagnostics through a tool_result middleware hook instead of throwing, while retaining cancellation, cleanup, and implementation-commit-policy behavior.
Origin: ADR-0125
Backing: test

### `invariant: pi-subagent-progress-bounds`

The generated Pi extension retains at most 20 display events of at most 2 KiB each, reports cumulative omitted-event counts and truncation explicitly, and never keeps a second raw child-transcript store.
Origin: ADR-0125
Backing: test

### `invariant: pi-subagent-progress-context-isolation`

The generated Pi extension carries intermediate child activity only in bounded tool details, never appending it to parent model-visible content or custom session messages, and a subagent tool's final content contains only the child report or a bounded failure summary.
Origin: ADR-0125
Backing: test

### `invariant: pi-subagent-progress-rendering`

In the generated Pi extension, every public subagent tool's collapsed view renders status, recent bounded activity, omission state, and available usage, and its expanded view additionally renders the task, retained activity, the final report, present diagnostics, and available usage from the same structured details without changing execution.
Origin: ADR-0125
Backing: test

### `invariant: reviewers-report-only`

The rendered reviewer agent templates and the shared review spine instruct the reviewer only to read, run its lenses, and report findings; none contains a directive to apply fixes, commit, or loop a re-review.
Origin: ADR-0074
Backing: test

### `invariant: runner-awf-verbs-owned`

In the rendered command runner, every clispec command declared runner-forwarded has an awf-owned arm outside editable in-place sections and delegates to the bootstrap-resolved pinned binary; the usage tail derives from the same ordered metadata, so command additions cannot drift.
Origin: ADR-0101
Revised-by: ADR-0144
Backing: test

### `invariant: runner-example-adopted`

The bundled sundial example enables the runner singleton, and its rendered `x` is drift-free, invariant-clean, and free of advisory notes, carrying the awf-owned dispatch including the context verb.
Origin: ADR-0101
Backing: test

### `invariant: runner-project-verbs-in-place`

The rendered runner's project-verb region and its setup-and-helpers region are editable in-place sections, so an adopter's edits there survive re-sync while the awf-owned verb arms and file structure are regenerated.
Origin: ADR-0101
Backing: test

### `invariant: runner-render-publication-safe`

The runner template renders leak-free under empty data, producing no unresolved token and no stray section or marker residue, like every other awf template.
Origin: ADR-0101
Backing: test

### `invariant: runner-singleton-toggle`

With the runner singleton enabled, `awf sync` renders exactly one runner file at the repo-root path `x`; with it disabled or absent, it renders none.
Origin: ADR-0101
Backing: test

### `invariant: skill-prose-tool-agnostic`

Every rendered skill and agent body is free of runtime tool-name tokens; a case-insensitive, word-anchored scan rejects subagent_type, the phrase subagent type, agent tool, skill tool, AskUserQuestion, the backticked-agent prompt phrasing, and the backticked or phrased file-operation tool names for write, edit, and read, while plain action verbs and the shell grep stay allowed.
Origin: ADR-0038
Backing: test

### `invariant: template-source-residue`

Every file in the embedded templates tree is free of concrete ADR citations (the token ADR- followed by four digits) and free of the repo-identity literals hypnotox and agentic-workflows, except in an explicit exemption list whose each entry fails when its named file no longer carries the literal.
Origin: ADR-0082
Backing: test

### `invariant: templates-valid-frontmatter`

Every catalog skill and agent template, rendered with representative data, produces leading frontmatter that parses as YAML with a non-empty name and a non-empty description.
Origin: ADR-0006
Backing: test

### `invariant: upgrade-delegates-fetch`

The rendered `.awf/upgrade.sh` obtains the binary only by invoking `.awf/bootstrap.sh` with AWF_VERSION set; it performs no release-asset download and no checksum of its own, and its single direct network call is the latest-tag redirect probe against releases/latest.
Origin: ADR-0085
Backing: test

### `invariant: upgrade-exec-final`

The rendered `.awf/upgrade.sh` hands off with exec of the fetched binary running upgrade as its final statement, so the shell process is replaced before `awf upgrade` re-renders the script in place.
Origin: ADR-0085
Backing: test

### `invariant: workflow-chain-adr-before-plan`

The rendered AGENTS.md and workflow.md workflow-chain string presents the ADR step before the plan step.
Origin: ADR-0028
Backing: test

### `invariant: workflow-chain-surfaces-resync`

The rendered workflow chain names the plan-to-ADR resync step explicitly rather than hiding it from the high-level chain.
Origin: ADR-0028
Backing: test
