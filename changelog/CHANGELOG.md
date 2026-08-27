# Changelog

All notable changes to `awf` are documented here, newest first. Entries are grouped per release
into up to four categories (Breaking changes, Features, Bug fixes, Others) chosen by actual
adopter-facing effect (does it change rendered template output, CLI behavior, or config/lock
schema), not by mirroring a commit's Conventional Commits type. Run `awf changelog --help` to
query a single version or a range.

## [Unreleased]

### Features

- Pi implementation children now use an explicitly supplied, validated managed checkout as both their base CWD and commit-policy snapshot identity. Root omission remains root/root; parent path targeting remains explicit and is not filesystem confinement.

### Bug fixes

- Linux release tarballs now use portable root ownership and expected executable and regular-file modes, so restricted rootless extraction does not depend on the release builder's account.

- Terminal plan closure now freezes Implemented plan history and validates a parsed complete reconciliation against a selected repository implementation range. Proposed plans remain amendable; closure does not require original plan choreography.

- Pre-commit verification now runs behavioral gates against the materialized staged candidate, and release publication requires exact-SHA CI evidence before its credential-bearing job.

- Mutable project authority now fails closed: config and sidecars require one complete YAML document, live locks require closed nonempty permanent inventory, and upgrade recovery binds a complete journal to its terminal lock replacement before mutation.

- Project rendering now serializes tracked and shared resident mutations by canonical physical root, prepares inside the lease, commits through confined identity-aware operations, and reports stable partial effects with recovery guidance.

- Domain, local-document, and topic creation now acquire checkout transaction leases before authority loading, retain the observed config identity for replacement, and exclusively publish authored sources. These protections serialize live operations but do not claim crash atomicity.

- Plan and ADR scaffolding, ADR numbering, upgrades, uninstall, effort lifecycle, and managed-worktree lifecycle now hold every applicable tracked or shared-resident lease from mutable authority loading through complete outcome presentation. Confined mutation paths reject parent relocation, and partial failures retain exact committed paths, topology, upgrade recovery, and Publisher effects with safe recovery guidance.

## [0.40.0] - 2026-08-25

### Breaking changes

- Live project authority now starts at schema 46. Below-floor, retired-layout, and partial authority refuses before decoding or mutation with recovery guidance; audit alone reads represented managed history from schemas 3 through 46. Live manifests reject retired pre-31 ADR routing keys at every admitted schema, and ordinary output pruning no longer gives the retired co-owned runner a special backup path. Bridge and cutover support, schema-1 effort retirement, legacy four-line effort memory, and plan-v2 ordinal Decision selectors are removed. Generic journal recovery, schema-2 effort residents, represented ADR and plan formats, and frozen pre-V4 ordinal navigation remain supported.

- The approval boundary is an unresolved material decision rather than any hand-authored production-code change. A routine change whose outcome, design, scope, safety, and verification are already settled now proceeds without an approval stop, whatever kind of file it touches, and the per-artifact carve-out for documentation, test, and generated-output work is removed because the general rule already covers it.

### Features

- Maintainable-design guidance now requires new workflow concepts, artifact fields, lifecycle states, hard checks, and glossary terms to consolidate existing semantics unless a demonstrated correctness or safety invariant cannot fit the existing model. This remains a judgment rather than a target count or checker, and the glossary no longer preserves the retired split-effort term.

- Operative workflow guidance now separates rules, flexible implementation details, bounded stop conditions, and required evidence across planning, execution, and review without changing their authority or verification boundaries.

- Check output now separates blocking Errors, zero-exit Warnings, and unranked Information by protected property. Prose style and heuristic lint no longer block valid work, while invalid inputs, serious drift, authority violations, unsafe behavior, and unavailable required verification remain blocking.

- The daily-use guide now covers initialization, generated ownership, rendering, checking, and upgrading without rare protocol detail. Workflow, debugging, and configuration retain advanced lifecycle, recovery, and override facts; the new Pi Runtime Reference retains portable Pi runtime and subagent protocol.

- Pi session-replacement protocol now has one Pi-runtime-owned executable projection, while shared continuity guidance remains capability-neutral and pi-tools remains independently installed.

- Core and Full are documented as governance footprints with one shared correctness, autonomy, maintainability, and review-quality bar. Core includes the operational workflow; Full adds ADR, plan, current-state, context, and audit capabilities. The `profile` config key and existing migration behavior are unchanged.

- Bug fixes now require the strongest practical durable oracle. Automated red-then-green regression evidence remains the default; when it is impractical, a concrete reason and the strongest safely reproducible alternative are accepted without weakening expected behaviour or verification strength.

- Prose checks now enforce punctuation restraint instead of a seven-codepoint ban: every en dash and any paragraph with more than two em dashes produces a zero-exit Warning, while ellipses, curly quotes, and restrained em-dash use are permitted. Existing exemptions for formerly guarded ellipses and curly quotes remain accepted as inert compatibility input.

- Implementation and plan review now admit maintainability findings only when they identify semantic ownership, location, concrete risk, smallest clean remediation, and existing classification. Risk-free aesthetic preferences are rejected without a new severity or disposition; ADR review and severity policy are unchanged.

- Clean integration is operative across design, planning, implementation, and review: one proportional rule requires semantic ownership, bounded enabling refactoring, practical obsolete-path retirement, moving verification surfaces, and explicit residual debt while preserving YAGNI.

- Plan execution routes are mutable while the protected contract stays fixed; plan owners may revise non-load-bearing choreography and stop for reapproval only when the protected contract would change.

- The rendered workflow document states one protected-contract doctrine: the workflow governs a change's protected contract, while phase and task shape, ordering, inventories, execution mode, and commit decomposition are a route its owner may revise. Precedence is decided per constraint, so one project rule can be binding in its protected clauses and subordinate in its route clauses.

### Bug fixes

- Current tag vocabulary validation and health advisories now evaluate authored pitfalls only, while parsed legacy ADR tags remain accepted append-only history.

- Shipped glossary fallback now reflects that effort finish archives the complete resident and that brainstorming outline approval occurs only when a material decision is unresolved.

## [0.39.2] - 2026-08-18

### Bug fixes

- Prose checks silently skip non-UTF-8 files instead of reporting one warning per binary path.

## [0.39.1] - 2026-08-17

### Others

- Local release preparation now stores the version in a dedicated data file, so its version, changelog, and generated-lock commit skips Go and Pi test suites while retaining version validation, static gates, and the complete tag-time gate.

## [0.39.0] - 2026-08-16

### Breaking changes

- The awf-owned Pi effort extension now requires Pi 0.84.2 or a later compatible build; adopters on the former 0.81.1 floor must upgrade.

- The Pi extension test lane now requires exact host Node v24.19.0 and npm. `./x pi-test run` uses NVM when available, a checkout-local lockfile dependency tree, and a narrow temporary workspace; Docker and `pi-test reset` are removed.

- Pi review dispatch now gives every enabled reviewer a dedicated task-only profile: Full exposes `subagent_review_adr`, `subagent_review_plan`, and `subagent_review_code`, while Core exposes only `subagent_review_code`. The generic `subagent_review` tool and its required `kind` argument are removed without an alias.

### Bug fixes

- Shipped workflow skills and configuration suggestions no longer name commands from awf's repository-only `./x` runner.

## [0.38.0] - 2026-08-15

### Features

- Pi adopters now install the unpinned `pi-tools` prerequisite independently. awf no longer renders its general context-usage, handoff, or subprocess-runner extensions; it registers only its four workflow-specific grounding, exploration, review, and implementation profiles through the protocol-v2 handshake, with no fallback when capability or final registration is unavailable.
- New projects now default to a closed Core workflow profile with brainstorming, implementation, testing, review, efforts, and managed worktrees; Full adds ADR, plan, current-state, context, and workflow-audit governance, and existing repositories migrate explicitly to Full.
- Repository and staged drift checks now require every generated output and `.awf/awf.lock` in Git's index, report staged deletion, ignored untracked replacement, and absent-lock failures as `untracked`, retain tracked files that later match ignore rules, exclude nested-adopter resident outputs, and report tracking as unavailable without disabling filesystem drift checks outside Git.

### Bug fixes

- Pi profile registration now waits through the complete startup-handler turn before reporting a missing `pi-tools` capability, avoiding a false incompatibility notice when final registration arrives later in the same startup dispatch.

## [0.37.0] - 2026-08-13

### Features

- `./x gate` now independently selects test lanes from staged paths: exact documentation-only changes skip both, Pi-only changes run Pi smoke only, Go-only changes run Go tests and coverage only, and overlapping or uncertain changes run both.

### Others

- Current guides and native workflow prose are shorter while preserving their triggers, authority boundaries, and verification obligations.

## [0.36.2] - 2026-08-13

### Bug fixes

- Native skill guidance now selects bodies for the next concrete action instead of preloading likely later workflow owners, with explicit timing boundaries for orientation, generated-tree maintenance, and documentation authoring.

## [0.36.1] - 2026-08-13

### Bug fixes

- Effort-backed Pi handoffs now identify only the effort, leaving association, resume procedure, handoff logging, and autonomous continuation to the rendered skills and effort authority instead of embedding phase-scoped stopping cues in kickoff prose.

## [0.36.0] - 2026-08-12

### Features

- Repositories can declare additive `localDocs` whose sole inline-editable Markdown body remains managed by ordinary render and check workflows, appears in the `AGENTS.md` document map, and receives ordinary Markdown-link and skill-reference checks. `./awf new doc <name> <description> [--title <title>]` scaffolds the declaration and output, deriving the title from the final kebab-case name segment unless an explicit title is supplied. Removing a declaration or uninstalling awf preserves the complete document at a sibling `.awf-bak` recovery file (with a numbered suffix when needed).

### Changed

- Standardized rendered repository-local awf invocations on the unconditional `./awf` wrapper and moved this repository's source execution body into a runner convention part.

### Bug fixes

- Default guidance no longer advertises retired project-local skills, agents, docs, or the `local` sidecar field, and historical configuration projection now strips every retired key before strict current-schema decoding.

## [0.35.1] - 2026-08-12

### Bug fixes

- Render now confines output, backup, retired-output removal, empty-ancestor cleanup, and lock publication to the selected tracked or resident repository root, replaces final symlinks without following them, and safely refuses foreign escaping or broken symlinks instead of reading, writing, or deleting outside the root.

- Repository-maintainer context-spill observations now use an ignored checkout cache outside `.awf`, so the next `./x check` reaches its non-failing advisory instead of rejecting the log as closed config-tree drift.

## [0.35.0] - 2026-08-11

### Features

- Effort-workflow guidance now creates explicit-slug efforts autonomously when durable continuity materially helps, reports their identity, and directs deliberate active-effort switching through checkpointing or safe discontinuation and ordinary finish.

## [0.34.0] - 2026-08-11

### Breaking changes

- Schema generation 43 moves `data.pitfalls` into strict per-entry Markdown sources under `.awf/docs/pitfalls/`. Upgrade preflights every destination and refuses relative Markdown links with the entry and target named; replace those links and retry. Byte-identical leaves from an interrupted attempt are accepted, all leaves are created before old authority is retired, and a sidecar containing independent section configuration survives with only those sections.

### Features

- Agent guides now treat native-skill descriptions as routing metadata and direct agents to load skill bodies only when their owned work begins. Likely follow-ups stay unloaded until transition, while multiple bodies are reserved for skills that concurrently govern the current step.

- Hand-authored production-code changes now require a proportionate, explicitly approved implementation outline before mutation, including mechanical production refactors and preparatory tests. Documentation-only, test-only maintenance, generated-output-only, and non-code mechanical work retain independent autonomy, while conversation, effort decisions, and explicitly requested named-plan Architecture summaries can carry approval evidence into direct and delegated execution.

- The approved production-code outline is now the sole routine user checkpoint for its boundary. Settled ADR review continues autonomously to linked-plan handling or implementation, while new material decisions or boundary changes return through brainstorming.

- ADR authoring guidance now requires explicit user acceptance before a commitment enters a decision record, keeps unaccepted suggestions outside the artifact, and defaults to the narrowest durable semantics while routing implementation detail to a plan or direct execution. ADR review now separates consent adherence from scope, consumes effort-memory transcript evidence or the effort-free approved summary, removes unauthorized surplus commitments as reasoned corrections, and discloses removals and refinements at approval.

- Configured repositories can declare `render.templateSourceRoot` to add maintainer-facing `awf:template-source` provenance to generated Markdown without changing ordinary adopter output.

- The adopter-facing documentation standard now advises count-free prose for changing sets and requires essential exact counts to name their query and be reverified when the source population changes. The agent-guide standard now keeps literal placeholder syntax in reference documentation unless its brace guard deliberately recognizes it. Subagent return contracts now put bounded decision-bearing evidence before optional exhaustive inventories so runtime truncation does not hide outcomes, deviations, verification, or blockers.

- Pitfalls now publish as a compact title-sorted metadata index with domain and Unassigned navigation plus one exact-source generated leaf per entry. The complete family participates in working and staged output plans, lock, drift, backup, and deletion pruning.

- `awf new pitfall "<Title>"` now creates one canonical authored `.awf/docs/pitfalls/<slug>.md` source exclusively, reports its repository-relative path, and never renders or mutates another registry. Duplicate titles and a selected-path race refuse; a later retry reloads the corpus and chooses the first free deterministic suffix. Deleting the authored source retires its generated index row and leaf through ordinary render pruning.

### Bug fixes

- Current-state coverage findings now give actionable recovery: they name same-domain claim-bearing global topics for adopter judgment, recommend extending a bounded selector only when that topic naturally governs the path, and otherwise direct the adopter to create or use an appropriate scoped claim-bearing topic.

- Pi implementation commit policy now accepts an explicit same-repository verification checkout, so commits in an awf-managed worktree are verified there without changing parent or child Pi CWD. Invalid explicit identities refuse before dispatch, forbidden commits use the same selected checkout, and unchanged-HEAD failures name the checkout and retry field.

## [0.33.0] - 2026-08-10

### Breaking changes

- Schema generation 42 adds the governed self-ignored `.awf/effort-archive` resident root. Older projects must run `awf upgrade` before effort commands; upgrade publishes the marker and current lock, and ordinary render repairs that marker without managing archive descendants. `awf effort finish` now preserves the complete unchanged resident at `.awf/effort-archive/<uuid>-<slug>` instead of deleting it, releases the slug after the no-replace move, and reports typed recovery actions across partial durability outcomes. Active residents may also contain one optional owned `scratch/` directory whose descendants remain opaque and unmanaged. Failed default-worktree creation still deletes only its identity-matched just-created resident when topology is proven absent; it never archives that unsuccessful creation, and rollback parent-sync failures identify whether the reservation or deletion completed. The archive has no inventory, restore, prune, analysis, or retention command and remains manually disposable local data that may still appear in backups or local disclosure.

- Removed the `domain-doc-staleness`, `domain-code-staleness`, and `undocumented-domain` audit warnings. Exact ADR operation replay and structural topic coverage, ownership, and selector validation remain the enforceable current-state checks.

- Schema generation 41 permits global topics to declare domain-bounded path ownership. Topic coverage output replaces `matchedPaths` and `matched-paths` with separate `applicablePaths`/`applicable-paths` and `ownedPaths`/`owned-paths` witnesses; no compatibility alias remains. Human context selector records now expose separate global-declaration and ownership-selectors columns while remaining free of matched-path witnesses.

### Bug fixes

- ADR and plan scaffolding now publish complete files without clobbering an existing artifact, and concurrent `awf new adr` calls serialize numbered allocation for one decisions directory. Render and runner-prune backups preserve complete rescue copies and retry the next suffix when another process wins a backup name.

- Parent plan executors now continue autonomously to the next unfinished phase after a settled checkpoint, while delegated children remain phase-scoped and terminal assurance still waits for every phase.

## [0.32.0] - 2026-08-07

### Breaking changes

- Rendering unconditionally emits the complete standard catalog for fixed Claude Code and Pi targets beneath the fixed `docs` root. Schema generation 39 retires the `skills`, `agents`, `docs`, `targets`, and `docsDir` selection keys and the artifact-sidecar `local` field; `bootstrap.enabled` remains conditional.

- The complete `awf enable`, `awf disable`, and target command surfaces are removed. Domains now use `awf new domain` and `awf remove domain`; `awf list` is inventory-only.

- The owner-free `awf effort memory edit` and `awf effort memory update` commands now report the same bounded contextual diff the owner-scoped protocol carries: numbered removed, added, and surrounding context rows with omission markers, instead of the previous `before:`/`after:` whole-body pair. The reported first changed line and truncation flag are unchanged in meaning. Adopters reading that field expecting complete before-and-after bodies must read the resident itself.

### Features

- Added thin `using-awf` and `writing-docs` support skills for generated-tree maintenance and documentation authoring, rendered for both Claude Code and Pi with detailed rules delegated to their authoritative docs.

- Plans now focus on change-specific direction while generic execution protocol stays in workflow owners. The separate plan reconciliation skill is retired; schema generation 39 removes the historical selection surface for older trees, and generation 40 completes that retirement for independently stamped generation-39 trees. Ordinary full plan review renews every typed linked Proposed plan after ADR-first corrections plus affected landed-phase assurance when implementation has started.

- Added compact `awf:source` markers for opaque generated documents and the canonical source-editing map.

- Commit-capable implementation phase owners may now add necessary paths omitted from their dispatch when the approved scope and authority remain unchanged, reporting each addition as a reasoned deviation; commit-disabled helpers remain strictly confined to their assigned path partition.

- Plan, ADR, and implementation review now apply authority-preserving corrections autonomously, including after the single verify pass, and ask the user only when a correction would deviate from settled user-approved design or an active current-state claim.

- Added independent judgment-based workflow escalation, reusable grounding support, risk-based effort-independent implementation review, a single-home effort lifecycle, and guarded grounding backfill for compatible adopters.

- README commands, embedded templates, and config-reference live state now derive completeness from their source authorities, replacing three manual pitfall reminders.

- Owner-scoped Pi effort-memory edits and metadata updates now expose binary-authored read-only previews and authoritative bounded contextual diffs. Preview failures stop mutation, while normal validation and publication semantics remain unchanged.

- The Pi `effort_memory_edit` and `effort_memory_update` tools now render like Pi's own edit tool: one stable row shows the previewed diff while arguments settle and then retains the authoritative colored diff, a truncated diff carries a visible warning, and model-visible output stays a compact summary with the protocol reply kept in structured details.

## [0.31.0] - 2026-08-06

### Features

- Native harness discovery now replaces the generated skill roster in managed agent guides, while canonical documentation carries the full procedure. Fixed default and self-hosted guide proofs prevent base-guide regrowth, and aggregate `awf check` warns when a managed guide exceeds its advisory size bound.

- Implementation paths now resolve authority-preserving reasoned deviations and review findings autonomously; inline owners amend mutable plans, delegated owners report deviations for post-review parent reconciliation, and material authority, outcome, scope, or persistent safety and verification boundaries still escalate.

- `awf effort memory read`, `edit`, and `update` now provide owner-free readable commands plus owner-scoped protocol forms. Reads paginate on complete lines with handled refusals, exact body-only edit batches update timestamps automatically, and advisory owner loss is reported without treating activity as authorization.

- Schema generation 36 makes structural Markdown headings awf-owned while convention parts and in-place editing own only section bodies. Dropping a section removes its complete structure, and the exact frozen convention-part migration removes only copied legacy headings or refuses ambiguous content before changing files.

- Catalog-backed sidecar lists now preserve standard defaults before project entries unless `dataDefaults.<key>: false` explicitly suppresses them. Null and non-list project values are rejected, glossary term layering remains specialized, and the schema-35 upgrade records suppression for existing replacements without changing their rendered meaning.

- Planning and implementation review now carry focused human checks for contradictory generated prose, concept-preserving paraphrase, and intentional literal placeholder syntax at each affected output boundary. The guidance requires concrete examples and expected readings without introducing semantic inference or a universal output validator.

- Render completeness now derives conditional config-tree units and live template identities from their existing catalog, kind, target, and singleton declarations. Config-reference live values use one exhaustive typed classification for generated documentation and CLI presentation, and singleton template conditionals are checked against their artifact-specific live contexts.

### Bug fixes

- Historical audit now streams range evidence once and releases consumed historical authority state after its final consumer, preventing long authority-heavy audits from exhausting memory.

## [0.30.0] - 2026-08-05

### Features

- Ordinary CLI mutations, details, collections, refusals, and commit-policy reports now use the deterministic readable-text presentation contract. Convenience JSON remains removed from effort and topic commands; authored plan and changelog payloads plus effort activity, init descriptor, and context spill protocols retain their exact bytes.

- Checks and audits now collect complete structured reports before rendering, grouping errors before warnings while preserving source order within each category and separating produced failures from operational diagnostics. Upgrade mutations now identify every applied migration and report proven changed axes with recovery steps if terminal sync fails.

- Render, effort, and managed-worktree commands now use readable typed output. Convenience JSON for `awf effort new`, `list`, and `show` is retired; activity JSON remains the byte-exact protocol.

- CLI help now derives from structured command specifications, command diagnostics use the typed output boundary, commit authorization uses actionable steps, and init prompts validate completely before their single flushed prompt tail.

- `awf context` and `awf topic` now present structured readable text through the shared presentation grammar; context authority categories retain fixed-schema records, and `awf topic --json` is removed.

- Authored current-state transactions may now append several distinct-target Applied or Reapplied batches and any exact-prefix legal Status progression, while repeated same-claim occurrences remain separately observable and merge-only folding stays provenance-gated. Historical audit applies the same rule to retained non-merge commits.

- Plan authoring and review now scope material verification to its actual lifecycle snapshot, require demonstrated negative cases for mechanical checks, and distinguish authority, state, and choreography constraints so durable properties are preserved without enforcing unnecessary process shape.

- Execution and integration guidance now verifies checkout identity after commit-capable work, reconciles plan deviations before freeze, revalidates workflow prose after divergent integration, keeps one writer per checkout, preserves gate status while logging, and stages each render manifest atomically with its outputs.

- Rendered workflow guidance now makes the simplest sufficient solution the default: brainstorming approves a proportionate scope and design boundary before implementation, material deviations stop for user approval, planning is used only when it materially helps, and reviewers reject speculative additions.

- Pi effort association now keeps complete metadata independent while capability-gated display suffix publication sends only the canonical slug or null. It never overrides, reads, or composes routing identity, and degrades to metadata-only behavior.

- Pi fresh-session handoff now persists accepted kickoff as one visible default-rendered `agent-handoff` custom message with an explicit agent-authored envelope and replacement-bound turn trigger. The public bounded `{kickoff}` input remains unchanged; Pi's current provider adapter still receives the custom content as a user-role message.

- Release verification now pins the root project to the canonical AGPL-3.0-only license bytes, matching README references, and license inclusion in every GoReleaser archive, without recategorizing dependency metadata or retained third-party notices as project-license inputs.

- `awf check commit-policy <revision-or-range>...` previews configured exact author, committer, and optional SSH-signature provenance for explicit commits after a full baseline. Ranges use the shared exact two-sided grammar, so three-dot, empty-sided, and multi-range forms are refused rather than treated as clean history. Disabled policy succeeds with one note; complete violations and operational refusals are actionable. The hooks singleton now renders a fifth inert payload, `reference-transaction.sh`, which rejects nonconforming introduced branch commits before refs move; pre-push expands branch and recursively peeled tag targets through the same verifier before its gate. Adopter-owned stubs resolve the invoking worktree, and awf never activates hooks or replaces remote branch policy.

- New efforts now require a caller-selected canonical slug through 32 bytes via `awf effort new --slug <slug> "<outcome>"`; workflow guidance confirms outcome, title, and slug before creation, while schema-2 state, managed topology, and existing resident support through 63 bytes remain unchanged.

- ADR guidance now distinguishes durable commitments from implementation directives with post-implementation and counterfactual tests, routes execution detail to plans and unsettled context to effort memory, and reviews ambiguous mechanisms semantically. ADR scaffolding remains the single source for the current authoring format.

- Repository checks now use capability-planned execution: direct and aggregate checks share prepared config, project reports, and enabled scanner index inputs while preserving successful output order. Any readiness failure occurs before step output.

- Discovery now remains effort-free until the agent presents a labeled concrete outcome and proposed effort title, stops without mutation, and receives clear confirmation in a later user response; existing efforts resume under their fixed identity without reconfirmation only inside their confirmed outcome, a new outcome cannot silently reuse or replace an active effort, and no CLI or schema migration occurs. Adopters with full-replacement workflow, guide, checkpoint, or affected skill parts must re-derive this first-creation boundary because default-template projection tests cannot inspect replacement prose.

- `awf audit` now enumerates committed metadata without eager blob reads, then loads only its exact configuration, ADR, and topic authority into a type-distinct immutable selection. Repository and staged checks retain complete snapshots and the full marker, coverage, and domain-sidecar validation boundary, while revisions outside historical authority can reuse their first-parent state. Context cancellation and deadline expiry abort immediately with preserved error identity instead of becoming transition warnings.

- `awf effort memory update` now maintains canonical memory frontmatter while migrating exact legacy metadata. Advisory Pi activity is now protocol 2: JSON-only attach, heartbeat, and detach carry owner/timestamp facts, and explicit attach safely replaces a bounded old v1 resident. The removed checkout and CWD fields and operations have no compatibility execution path. Pi directly associates at repository root, supplies fixed relative effort paths in transient context, and retains complete advisory Remote Pi metadata plus capability-gated display-only suffix publication without local TUI presentation or routing-name replacement. Core `effort-workflow` is selected by new untrimmed scaffolds, while existing adopters opt in explicitly; every target receives its cross-runtime existing-worktree guide, and Pi alone derives the companion skill and extension. Activity remains advisory and non-locking.

- Schema generation 33 activates V4 ADR scaffolds with stable per-Decision slugs while preserving every historical ADR and ordinary authored file byte-for-byte during upgrade.

- New plans use `plan-v2`: task-scoped Applying and Context Decision references, slugged Definition-of-done outcomes, and phase Advances and Completes assignments are validated in working and staged universes. `awf read plan` orders only selected resolved Decisions and owning-phase outcomes, excludes whole-plan Definition of done, and adds task scope safety; plan-v1 remains byte-compatible.

- The repository gate now offers opt-in per-stage timings, keeps ordinary Go tests Docker-free with actionable Pi-lane guidance, and runs the uncached Pi runtime smoke exactly once. The awf and Sundial runners also remove their no-op `gate full` aliases while generic extended-tier support remains available to adopters that have a real second tier.

- Root-confined upgrade attestation traversal now preserves established digest and error behavior.

- New plans are parsed `plan-v1` artifacts with mechanically validated phase, task, field, path, phase-close, and Definition of done structure. `awf read plan <plan> <P[.T]>` resolves exact filenames or stems and prints a source-ordered executable phase or task closure, while marker-absent historical plans retain legacy checks.

- V2, V3, and V4 ADR State changes now form an unordered exact-once completion set: explicit batches may exhaust Remaining while status stays Implementing, `Reapplied` corrections remain available throughout Implementing, and settled review later appends only the terminal status event. Direct implicit completion remains atomic with all matching claim mutations.

- Plan authoring now treats qualifying implementation-ready instructions as the default, marks contract-bearing tasks with `Latitude: exact`, supports explicit spike and batch fields with affected-path and post-check contracts, and retires the duplicated whole-plan File structure section.

- `awf check` now reports a non-failing advisory for glossary meanings longer than the terseness guideline, naming the term and its length. It evaluates the merged set, so shipped and project-authored vocabulary follow the same rule.

- The glossary now renders one sorted table from two layers: awf's shipped standard vocabulary and project-authored `data.terms`. A project record overrides a shipped term with the same case-insensitive name, including to reword or retire it locally.

- Pi now renders a standalone context-usage extension that injects neutral transient session context facts before every model call, without persistence, warnings, telemetry, or automatic pressure action.

- Mandatory workflow checkpoints now persist durable effort memory independently of discretionary Pi session replacement. Eligible Pi boundaries may continue in-session or replace the session, while only a replacement session records the actual handoff boundary.

- `awf check staged` now includes a rendered-output drift check, also available directly as `awf check staged drift`. It renders from the staged config and compares against the staged output tree, reporting only stale and hand-edited output; repository-only drift kinds remain outside its scope.

- `awf audit` now replays stale-ADR merge authorization for committed schema-31-and-later merges, using the same cleaned-message trailers and exact incoming-parent qualification as `awf check commit`. It reports malformed reserved trailers and unauthorized older-format imports while leaving pre-epoch merges, non-merges, and fast-forwards outside the rule.

- `awf check commit` now definitively authorizes exact incoming-parent older-format ADRs in real merges through adjacent `AWF-Allow-Version` and nonempty `AWF-Allow-Reason` trailers. Malformed syntax or an unqualified import refuses without changing the staged index or merge state, so an agent can correct the message and finish the existing merge.

- Schema generation 31 removes permanent ADR format cutoff and gap fields from new locks. Older schema-30 lock snapshots remain readable during upgrade, while version-1 bridge attestations retain their frozen payload solely until final cutover verifies and discards it.

- ADR parsing now follows each record's authored format marker rather than its number. Markerless
  numbered records remain legacy, governed V1, V2, and V3 records select their matching frozen
  parser at any number, and unknown or malformed markers are refused. New numbered and pending
  ADRs derive the binary's current authoring format from one activation registry.

- The shipped Maintainable Code Design guide gains a Readability section between semantic
  modeling and boundaries (ADR-0200): language-agnostic decision-framework guidance on naming
  for meaning, straight-line common paths, boring-over-clever constructs, comments stating
  what code cannot say, and restructuring until a passage argues its own correctness. The
  section name is a permanent override surface; adopters re-render `docs/maintainable-code-design.md`
  on upgrade.

### Bug fixes

- Pi handoff kickoffs now scope managed-worktree-only execution to the pre-integration phase and preserve the governed switch to the target checkout for integration, deferred lifecycle closure, worktree removal, and retrospective.

- ADR-authoring guidance now requires `awf new adr` before any ADR-file mutation, followed by capturing and reading the generated scaffold and editing it in place rather than creating or replacing the record through another mechanism.

### Breaking changes

- The built-in runtime target set is now exactly `claude` and `pi`. The `codex`, `copilot`, `cursor`, and `gemini` target values, their renderers, and their generated outputs are removed immediately without a migration or compatibility alias; remove those names from `targets` before opening an adopted project with this release.

- `data.terms` in `.awf/docs/glossary.yaml` is now an ordered list of `{term, meaning, domains}` records rather than a `term: meaning` map. No migration converts it. Convert each pair by hand; an unconverted tree fails render with `data.terms: must be a list of {term, meaning} records`.

- Pi `handoff_session` removes `memoryPath` and now exposes the exact closed `{kickoff}` schema for bounded replacement prose.

- Verification commands now live under repository and staged universes: `awf check repo` owns
  drift, state, prose, and memory checks, while `awf check staged` owns transition-state and
  rendered-output drift checks and `awf check staged commit` remains direct-only. Bare `awf check`
  runs both aggregates. A disabled prose or memory child emits a non-failing note naming its
  enablement knob. The `--staged` flag and the non-verdict `awf check invariants` report are removed.
  Schema generation 32 retargets surviving var values to the new paths, clears values that invoke
  the removed report, and deletes the retired `proseGateCmd` and `memoryGateCmd` keys.

- Every `invariant:` proof marker must now name the unit that proves it,
  `<marker> invariant: <domain>/<topic>:<slug> (<name>)`, and that text must occur verbatim on a
  line of the marker's own file that does not itself open with that family's marker token,
  unflanked by a letter, digit, or underscore (ADR-0205). A
  marker whose test was deleted, renamed, or moved now fails `awf check` instead of satisfying
  `Backing: test` while proving nothing. The name is free text, not an identifier, so an adopter
  whose tests are string literals rather than named functions can name the literal. No schema bump
  or `awf upgrade` step signals this break, because marker syntax is not part of the config schema;
  the first `awf check` after upgrading reports each unnamed marker with its file and line.
  Adopters migrate their own markers: deriving a name from a test file needs language knowledge
  awf does not have, and the shipped check deliberately has none. One knock-on: `awf audit`'s
  `current-state-transition` rule reads each commit's tree through the same loader, so over a range
  reaching back before the migration it reports a warning per pre-migration commit instead of
  evaluating that rule. The rule was already bounded this way by any tree the current binary cannot
  load; exempting the loader instead would have disabled the name requirement inside
  `awf check --staged`, which the pre-commit hook runs.

- Remove the `currentState.maxClaimsPerTopic` config key and the non-failing topic claim-count
  note `awf check` emitted from it. Schema generation 28 removes the key from an existing tree;
  because `config.yaml` is strict-parsed, a tree that still carries the key fails to load on
  this binary until `awf upgrade` runs. Topic cohesion is now an authoring and review concern:
  see the `One subject per topic` rule in the documentation standard.

- Compress the rendered checkpoint blocks to a four-step digest and replace the inlined
  context-spill recovery paragraph with a one-line pointer at every context-calling skill and
  the grounding-checker agent body; the full spill contract moves to a new `Context spill
  notices` subsection of `docs/working-with-awf.md`, its single rendered home (ADR-0197).
  Authority precedence and the one-writer contract leave the checkpoint blocks for the
  workflow doc's working-memory section. An adopter overriding the working-with-awf doc's
  `commands` part must carry the spill contract in the override.

- The four reviewing skills dispatch their verify pass conditionally (ADR-0197): a fix round
  whose applied fixes are all mechanical skips it and records the skip; any reasoned or
  user-decision fix keeps it. The `Record:` evidence block narrows to material decisions
  (scope, design, authority, or previously-approved output); reviewer briefs paste whatever
  blocks exist. The reviewing skills' restated commit-scope list and the coverage-regression
  reminder are deleted where deterministic gates already enforce them; an adopter without
  such gates loses a reminder, not a control.

- Reword the staged-authority instruction across the agent guide, the adr-lifecycle,
  executing-plans, subagent-driven-development, and writing-plans skills, the implementer
  contract, and the plans README and template: every commit still requires the staged check and
  the gate to pass, but a wired pre-commit hook is named as the enforcing layer and a manual run
  before committing is instructed only for a clone without wired hooks. The unconditional
  "run both commands manually, the hook repeats the staged check as defense in depth" model no
  longer renders (ADR-0196).

- A zero-padded `adrs:` entry in a plan is now always read as a decimal ADR number. The YAML
  decoder previously read one whose digits are all octal-valid as octal, so `adrs: [0153]`
  silently resolved against ADR-0107 and `adrs: [0012]` against ADR-0010, while a spelling
  containing an 8 or a 9 (`adrs: [0186]`) was already decimal. Such an entry now names the record
  its digits spell. Two plans in this repository were pointing at the wrong record and now point
  at the right one; if an adopter's zero-padded entry named a record that does not exist, a
  previously-clean `awf check` reports `plan-adr-link` drift. An entry outside 1 to 9999, and an
  empty entry, are now refused outright instead of decoding to an unusable number.

- Add the `current-state-v3` ADR format and its `adrFormatV3From` lock cutoff, sealed by schema
  generation 29. V3 is `current-state-v2` plus a mandatory `slug:` frontmatter key that is
  retained forever, and a record carrying no number routes into the corpus by that format marker
  instead of by a cutoff. Two adopter-visible consequences land with it. A file under
  `docs/decisions/` that is neither a reserved basename (`README.md`, `INDEX.md`, `template.md`)
  nor a parseable record is now a corpus error, where it was silently ignored; move any such file
  out of the decisions directory before upgrading. A duplicate ADR number, or a duplicate slug
  across pending and numbered records, is now a hard error from one place rather than a silent
  last-wins parse. Run `awf upgrade` to seal the cutoff; a fresh adoption seals V3 alongside V2.

- Add the required `integrationBranch` config key, written visibly into `config.yaml` by schema
  generation 30. It is the first key awf requires explicitly with no in-code default: it decides
  whether `awf new adr` writes a numbered or a pending record, and a silently defaulted branch
  name would silently change which. `awf upgrade` seeds `integrationBranch: main`; a config that
  reaches validation without the key is refused. A pending ADR checked out on that branch now
  fails `awf check` with `pending-adr-on-integration-branch`, which is what forces numbering to
  happen at integration; a detached HEAD passes, since the block fires only on positive branch
  identification.

- Rename the agent-guide render key `taskSkillRows` to `skillRows` (the row set always covered
  every enabled skill, not only task skills). A local override of
  `templates/agents-doc/AGENTS.md.tmpl` that still references `taskSkillRows` renders an empty
  skills list rather than erroring; update the reference when upgrading.

- `awf effort new` now creates the managed `.awf/worktrees/<slug>` checkout on `awf/<slug>` by
  default and directs execution there; `--no-worktree` keeps the invoking checkout, `--base <ref>`
  selects the branch base, and creation inherits the standalone add refusal surface (for example
  an in-progress rebase or merge in the invoking checkout). Effort commands report
  primary-root-qualified absolute memory paths outside the primary checkout, and Pi handoff
  resolves the primary control root so it validates the effort memory from any managed worktree
  (submodule and separate-git-dir layouts keep their rendered root).

- Add the `orienting` support skill: the single home of the orientation procedure. Its grounding
  ladder is current-state first (agent guide, document-map docs, domain docs), and consults recent
  path history only when current state leaves the situation unexplained; it carries the managed
  `awf context` discipline, report-only exploration dispatch, and effort-resume revalidation that
  reads the working-memory file whole, decision log included, and resolves any discrepancy in
  favour of the repository. The ladder is shared with the grounding-checker contract through a
  template partial, so the two stop drifting apart. Brainstorming's first step now invokes the
  skill instead of carrying its own copy, and `proposing-adr` and `writing-plans` gain an advisory
  pointer for stale grounding. Schema generation 26 enables it in any config that has
  `brainstorming` enabled, since the shrunk brainstorming template now invokes it by name; configs
  without `brainstorming` are untouched. Disabling `orienting` afterwards while `brainstorming`
  stays enabled fails `awf check` with dead-skill-reference drift until you re-enable it or
  override the three consumer sections.

- Remove the repository-global `state-sequence` from ADR status history. Applied events use
  `- <date>: Applied; operations: ...`; per-claim provenance is ordered by ascending final ADR
  number; an applied remove is an absorbing tombstone. `awf topic` drops its `[state-sequence: N]`
  suffix, `awf context` its per-operation sequence annotations, and the `stateSequence` field
  leaves the `awf topic --json` contract. Schema generation 27 strips the segments from every
  governed ADR and canonicalizes every `Revised-by` list to ascending ADR number; run
  `awf upgrade`. (ADR-0191)

- Remove the `currentState.topicCoverage` and `currentState.topicFanout` severity settings and
  the `off` value with them. Every adopted tree now always evaluates both topic coverage and
  fan-out, at ranks fixed in code: coverage reports at error and fan-out at warn. Whether your
  config declares a `currentState` block no longer affects it. If your tree has no such block it
  previously received neither finding and now receives both, so an error-rank coverage finding can
  newly fail a gate that was passing; every tree `awf init` scaffolds already declares the block and
  is unaffected. Where the two keys were a present block's only children the migration seeds and
  announces `maxTopicsPerPath: 8` rather than letting the emptied `currentState` block be dropped.
  That seeding was protecting the block-presence gate this release also removes, so it is now inert
  but harmless and its announcement still says it keeps the checks evaluating; the migration is
  frozen and neither is changed. That is the one case where `awf upgrade` adds a line to
  your `config.yaml` instead of only removing one; it materializes the budget that was already in
  force by default, and the regenerated `docs/config-reference.md` prints that row as `8` rather than
  `8 (default)`. A caller that does not want a finding
  class no longer suppresses it with a rank, it does not request the check. Every finding rank
  awf reports is now one shared two-member rank spelled `error` or `warn`; the audit surfaces
  that previously printed `warning` for a rank print `warn`, while the `%d warning(s)` verdict
  summaries are unchanged. Schema generation 25 removes both keys from a config tree, announcing
  each removal; run `awf upgrade`. `config.yaml` is strict-parsed, so a surviving key hard-fails
  on the new binary rather than warning.

- Replace effort JSON protocol 1 and mutable UUID lifecycle commands with protocol 2 immutable slug residents. `awf effort` now exposes only new/list/show/finish, separate worktree add/remove, and stateless integrate; standalone memory, rename, complete, abandon, reopen, repair, combined creation, manual integration, and every awf force-discard flag are removed. New/show return `{schemaVersion:2,effort:{id,slug,title,createdAt,memoryPath}}`, list sorts the same objects by slug, and JSON failures write no stdout.

- Schema generation 22 resets protocol-1 effort residents and standalone memory rather than
  migrating them. `awf upgrade` discards every `.awf/efforts/<uuid>.json` record, the efforts
  `.lock`, each `.<uuid>.<worktree|integration|removal>.partial` evidence file, and the whole
  `.awf/memory/` root, which stops being a resident root: rendering now governs only
  `.awf/efforts/.gitignore` and `.awf/worktrees/.gitignore`, and no render recreates the memory
  one. Nothing is ported, because a UUID record carries no slug and inventing one would fabricate
  identity; recreate the efforts you still need with `awf effort new "<outcome>"`, and copy
  anything you want to keep out of `.awf/memory/` before upgrading.
  The reset is one journaled transaction whose final lock replacement is the commit point, so it
  either completes or leaves the old generation and every resident exactly as they were. Before
  the journal exists, a complete read-only preflight refuses, changing no bytes and naming the
  next action, while any legacy managed worktree path, Git registration, or `awf/<uuid>` branch
  remains: settle or remove those with the pre-upgrade release first. It refuses the same way for
  an unknown, malformed, symlinked, hard-linked, non-directory, or foreign-owned resident, which
  is preserved for you to inspect by hand. An interrupted upgrade is resumed with
  `awf upgrade --recover`; every other command refuses while its journal exists.
  Older binaries refuse a tree that has advanced to generation 22.

- Remove `awf context --json` from normal and uncovered modes. Context now has one human-text contract; a future structured form will require a demonstrated consumer.

- Schema generation 21 destructively removes obsolete `.awf/metrics` and `.awf/assignments` residents during upgrade. Remove metrics and assignment commands, `/awf-effort`, `awf_workflow`, the Pi telemetry extension, and hidden workflow bodies. Enabled workflows now render as native Pi skills.

- The verification commands are regrouped under `awf check` and `awf sync` is renamed to
  `awf render` (ADR-0159). There are no aliases, on the ADR-0093 precedent: `awf sync` becomes
  `awf render`, `awf invariants` becomes `awf check invariants`, `awf prose-gate` becomes
  `awf check prose`, `awf memory-gate` becomes `awf check memory`, and `awf commit-gate [FILE]`
  becomes `awf check commit [FILE]`. Two new subcommands split what bare `awf check` already did:
  `awf check drift` reports stale or hand-edited rendered output (carrying the config-tree hygiene
  sweep) and `awf check state` reports current-state authority findings. Bare `awf check` is
  unchanged in behaviour and exit status, and `--staged` stays a flag on the bare form: a
  subcommand rejects it with a usage message. `awf help` now lists a group's children, so the
  regrouped commands stay discoverable.
  Upgrade note: this ships schema generation 19, so `awf upgrade` is required before `awf render`
  or `awf check` will run, including for a project whose vars name none of the renamed commands.
  The `rename-retired-commands` migration rewrites a var value whose *leading* token is an awf
  invocation (`awf`, `./awf`, or a path ending in `/awf`) followed by a retired subcommand,
  preserving trailing arguments. It deliberately does not touch a value naming another runner
  (`./x check`, `make gate`), one that mentions a retired name anywhere but that anchored second
  position, or any call site outside `.awf/config.yaml`: your own scripts, CI workflows, and git
  hooks that invoke a retired command by name need updating by hand.
  Expect a whole-tree diff on the first `awf render` after upgrading. The provenance banner stamped
  into the first line of every rendered file names the command to re-run, so the rename rewrites
  that line everywhere. The churn is content-free.

- Narrow Pi subagent model references to printable ASCII, the range `0x21` through `0x7E`, so the
  256 bound is the same count in code points, UTF-16 units, and UTF-8 bytes. The tool schemas and
  preference parsing now derive that form from one shared pattern constant instead of maintaining
  two independent checks, and an overlong reference still reports overlong before any form
  rejection. A reference containing accented letters, emoji, or control bytes is now rejected as
  malformed with the existing omit-the-field repair; every registry model id in use is already
  ASCII, including the whole recommended preset. The TUI label for an omitted `model` argument
  changes from `inherit parent`, which was itself a rejected sentinel value, to
  `(configured or inherited)`, which the shared form check rejects, so a displayed value can no
  longer be copied back as an argument that fails later as an unregistered model.

- Every Git operation awf performs now runs isolated and under a two-minute deadline, which
  changes three observable behaviours. A Git invocation that previously hung indefinitely on a
  stale `index.lock`, a credential prompt, or an unreachable remote now fails with the command,
  its exit status, and Git's own stderr, including inside the pre-commit hook; the deadline is a
  ceiling for a pathological case, not a budget any normal operation approaches. Effort
  operations, which previously inherited the ambient Git environment, no longer see an
  inherited `GIT_DIR`, `GIT_WORK_TREE`, or credential helper, so they act on the repository
  named on the command line rather than on whatever the environment selected; managed-worktree
  operations were already isolated and are unchanged in that respect. And a failure that used
  to surface as a bare exit status now carries the invocation and stderr.

- The worktree cleanliness refusal now honours your global gitignore. `awf effort integrate`
  and `awf effort worktree remove` previously ran their cleanliness read under a fully
  isolated environment, which also hid `core.excludesFile`, so a checkout dirty only with
  globally-ignored files (editor scratch files, OS metadata) was refused as dirty; it now
  passes. `awf audit`'s uncommitted-changes rule already honoured the global ignore and is
  unchanged in that respect, though its read is now isolated in every other way, so global
  or system configuration other than the ignore file no longer reaches it. Both directions
  are deliberate: the oracle answers what Git answers about ignoring, and nothing else the
  environment happens to say.

- `awf effort integrate` and `awf effort worktree remove` no longer tolerate untracked files
  under `.awf/efforts/` and `.awf/worktrees/` when judging whether the invoking checkout is
  clean. In an adopted project this changes nothing, because awf renders the `.gitignore` files
  that keep owned resident state out of Git's view entirely. It does change the answer in a
  checkout where those rendered files exist but have never been committed: the operation now
  refuses as dirty, and committing them resolves it. The previous allowance was broader than
  intended, tolerating any untracked file under those two directories, a developer's own
  uncommitted work included.

### Features

- Add `awf adr number [<slug>...]`, which assigns numbers to pending ADRs at integration.
  `awf new adr` is now branch-aware: on the `integrationBranch` it writes `NNNN-<slug>.md` as
  before, and on any other branch (so in every managed worktree) it writes a pending
  `<slug>.md` headed `# ADR-<slug>: Title`. Run the command in the worktree after merging the
  integration branch in and before merging back: it renames the file, rewrites the heading,
  substitutes `ADR-<slug>` in authored `Origin:` and `Revised-by:` lines, canonicalizes each
  touched list, re-renders, and prints one `<slug> -> NNNN` line for the integration commit
  message. It touches no status-history event, no already-numbered record, and no plan. Bare
  invocation numbers a single pending record; several require an explicit list naming every one
  in an order that numbers a record before any record revising what it adds. A number once
  assigned never changes: a numbering that raced another integration is unmade by
  `git reset --hard HEAD~1`, re-merged, and remade, and the command refuses a duplicate-number
  corpus with that recipe rather than guessing. It deliberately does not precondition on a green
  check, so an unrelated finding cannot deadlock it.
- Staged transition validation now pairs a record predating the slug format across a rename, by
  its canonical content digest. Such a record is paired on its number, so when the integration
  branch has taken that number meanwhile, renaming the local one used to pair it with the
  stranger that took it and fail with a cascade of unrelated findings. The pairing key is now
  resolved in three steps, retained slug, then content digest, then number, and the digest step
  applies only to a record carrying no slug, only on a digest carried by exactly one such record
  on each side, and only where the two ends hold different numbers. An unchanged or amended
  record therefore pairs on its number exactly as before. Such a pair admits the number,
  filename, and heading change and nothing else: status and Status history must be byte
  identical, no application batch may be appended or dropped, and the old number substitutes
  into `Origin:` and `Revised-by:` under the numbering substitution's rules. Because the digest
  is the key, a rename and a content amendment cannot share a commit.

- Render a fourth git-hook payload, `.awf/hooks/pre-merge-commit.sh`, running the staged
  current-state check. Git runs no `pre-commit` hook for a true merge commit, so a conflict-free
  automerge could otherwise land two records on one ADR identity, or a transition neither branch
  authored, on the integration branch unchecked. Like the other three the payload is inert: awf
  never activates hooks, so wire it yourself with a `.git/hooks/pre-merge-commit` stub that execs
  it. Adopters with the hooks singleton enabled will see the new file as drift until they render.
- A plan's `adrs:` frontmatter entry may now name a decision record by slug as well as by number.
  A slug entry resolves against a pending record's file or a numbered record's retained `slug:`
  key, so a plan written beside a record that has no number yet keeps a valid link once
  integration numbers it, and numbering never rewrites plan files. A numeric entry parses as
  before, except for the zero-padded octal case recorded under breaking changes.

- Tighten and correct the rendered skill and agent prose corpus (the 2026-07-30 audit fixes):
  the writing-plans scaffold command resolves the awf binary instead of the skill prefix,
  reviewer-lens enumerations are count-free, the resync skill names both invokers and carries
  plan-path identification on every target, the Pi-only `allowCommits` literal no longer leaks
  into non-Pi renders, catalog relationship rows and the support-skill vocabulary match the
  bodies, refactor-coupling-audit uncounts its categories and gives each one its own report
  line so sidecar-dropped categories drop cleanly, restated working-memory and notes prose is
  trimmed, and shared spine prose moves into new `templates/partials/` files
  (context-orientation, escalation-menu, exploration-breadth,
  exploration-detail). Rendered output changes for every target.

- Keep current-state-v2 ADR content amendable until Implemented. An `Amended` history event records
  each post-Accepted content digest, status events repeat the latest stamp, and terminal review now
  owns the final Implemented flip after findings settle. Existing records remain valid unchanged;
  this requires no migration or schema-generation bump.
- `awf check --staged` and `awf audit` now validate a merge as an ordered aggregate rather than
  refusing it. A merge is one Git commit but the aggregate of a branch's commits, so an ADR may
  contribute several application batches, a claim's operations across the pair may form an ordered
  chain of at most one leading add, any number of updates and at most one trailing remove, and an
  appended Status history need only preserve the prior history as an exact prefix. Every authored
  commit keeps the stricter per-step contract. This makes `awf effort integrate`'s divergent path
  usable for an incrementally-applied ADR and for a branch advancing an ADR the target already
  carries. Two things deliberately do not change: the global state-sequence namespace must still be
  contiguous, so a branch numbered before the target advanced still has to renumber before
  integrating, and a merge is recognized by its recorded provenance (`MERGE_HEAD`), so
  `git merge --squash`, which records none, keeps the authored-commit contract.
- Establish `code-design/state-ownership`, the second code-design authority, with four reasoned claims
  (construction-immutable state, operation-owned derivation, no remembered invalidation, and a single
  derivation producer) plus one test-backed claim over `internal/project`. The `code-design` commit
  scope now means "code-design authority and cross-package code structure" rather than dependency
  composition alone, so the rendered scope tables and the reviewer sidecars change with it. ADR-0180

- Change the rendered Pi subagent extension's preference API: `model-routing.ts` no longer exports
  `createPreferenceStore`, exports `async loadPreferenceState(deps, registry)` returning an immutable
  `EffectivePreferenceState` instead, and `resolveChildModel`'s last parameter is that derived state
  rather than the store. Adopters regenerating `.pi/extensions/awf-subagents/` see the store's
  `ready`/`reload`/`validateAgainstRegistry`/`state` protocol replaced by one load call; behaviour,
  including the ENOENT and read-error paths and registry validation, is unchanged. ADR-0180

- Add the `explorer` and `grounding-checker` agents: the child-facing contracts for dispatched
  exploration and grounding-check work, rendered per runtime like the review and implementer agents.
  The explorer body carries the one-information-need rule, the ordered breadth and report-detail
  scales with their project search universe, `file:line` grounding, and the not-found, inconclusive,
  and unverified outcome distinction; the grounding-checker body carries its verification obligations
  and its closed finding schema. Both are report-only. Schema generation 24
  (`explorer-grounding-closure`) pairs `exploring` with `explorer` and `brainstorming` with
  `grounding-checker`, so `awf upgrade` enables the paired agent for any tree that already enables
  either skill; a tree that enables one of those skills without its agent fails at project open.
  In Pi, `subagent_explore` and `subagent_grounding` now load their contracts from
  `.pi/agents/explorer.md` and `.pi/agents/grounding-checker.md` and fail closed with an
  enable-and-render repair when the file is missing or bodyless, the same way `subagent_implement`
  already did. Pi's selected breadth and report detail and its ten-child FIFO limiter remain per-call
  text the extension appends, so the rendered bodies stay runtime-neutral.
- Add repository-wide dependency-composition authority and a `code-design` commit scope, preserve reviewer defaults while teaching reviewers to reject speculative capabilities, and introduce the three-dependency `project.Loader` foundation composed by the `runSync` command family. Existing callers retain `project.Open` as a compatibility wrapper; this does not convert other filesystem, process, or package-global seams.
- Add the `implementer` agent: the child-facing contract for dispatched implementation work, rendered
  per runtime like the review agents. It states the two authority modes (commit-capable phase owner and
  commit-disabled path-confined helper), that the dispatched task is the complete scope, that the agent
  guide's invariants bind while its skill catalog and chain routing do not, the green obligation and its
  ban on weakening a check to hide a failure, that no interactive channel exists so escalation is a
  returned inventory, the owner transaction, and a closed two-outcome return whose `stopped` outcome
  requires working-tree status, work completed, work remaining, the named failing check with its actual
  output, and what was already tried. `executing-plans` and `subagent-driven-development` now require
  the agent, and schema generation 23 enables it in trees that already enable either skill. Pi's
  implementation subagent now loads that rendered contract from disk and so fails closed without it: a
  Pi target that enables neither dispatching skill is not reached by generation 23's closure and must
  run `awf enable agent implementer` (then `awf render`), or `subagent_implement` errors with that same
  repair hint instead of running on a built-in instruction. A commit-capable implementation call that
  did not itself fail and leaves HEAD unchanged now fails too, carrying the child's own report
  alongside the demand for the required stopped inventory.
- Unify every concrete non-minimal workflow around one immutable slugged effort with always-owned `.awf/efforts/<slug>/memory.md`, one user-managed writer, repository-authority precedence, conditional post-review worktree integration/removal, renewed review after divergent merge, retrospective, and restartable finish last. Minimal simple fixes remain effort-free. Pi handoff and durable-memory citation checks now enforce the owned path without selecting or mutating effort state.
- Add a generated bounded Pi subagent model-routing module and inject a current per-run routing card only when an awf subagent tool is active. Preference and registry state refresh before injection, the card stays out of session history, and pinned in-process runtime coverage proves delivery without an external model call.
- Make each plan phase an independently green implementation transaction with per-phase inline or subagent-driven ownership. A complete subagent-driven phase has one commit-capable owner; the parent retains review settlement and dirty recovery, while optional batch helpers remain sequential, commit-disabled, explicitly partitioned, and excluded from shared files and phase checkpoints.
- Add the mandatory Maintainable Code Design guide and its document-map link to rendered adopter documentation. Brainstorming, ADR proposal, coupling audit, plan writing, TDD, inline plan execution, direct execution, subagent-driven development, and bugfix workflows now carry stage-specific model, boundary, dependency, validation, and scope obligations, including bounded enabling-refactor assessment and materially larger-work escalation.
- Add first-class local efforts, managed resident roots, binary session assignment, and independent schema-1 Pi session telemetry.
- Add atomic repository-wide Pi session assignment for lightweight efforts. `awf effort assign`, `unassign`, and `assignments` maintain one current session-to-effort authority from primary or linked worktrees; explicit reassignment, including to terminal efforts, never changes lifecycle state.
- Add safe managed effort worktrees. `awf effort new --worktree` and `awf effort worktree add` create fixed manager-owned paths and branches; `awf effort integrate` integrates them, `awf effort integrated` records manual integration, and `awf effort worktree remove` removes them. Native-Git topology checks, explicit integration dispositions, and paired force/reason recovery protect recoverable risks.
- Add binary-owned lightweight effort records as repository-local resident state. `awf effort` supports memory-by-default creation, deterministic list/show output, rename, explicit memory creation, complete, abandon, reopen, and confined repair from primary or linked worktrees.
- Managed context callers now start with bare context or request only the facets required by their active lens, never prescribe `--full`, and share verified spill-packet consumption and cleanup guidance across rendered targets.
- Add request-sensitive context tiers: bare directories provide compact tier-0 orientation, exact and Git-selected files add actual marker-kind relationships, and explicit facets expand non-direct authority without changing request ownership.
- The awf source repository now provides `./x context`, which preserves context output and status while recording path-free spill observations in an ignored owner-only locked local log. Logging failures warn without hiding a delivered spill, and `./x check` gives a non-failing advisory until the operator resolves or promotes the issue and removes the log.
- `awf context` now preserves request blocks, groups equivalent directory descendants without disclosing large member sets, deduplicates authority globally, and offers eight bounded repeatable detail facets with `--full` as their canonical union.
- Context output now writes unchanged through 8,192 bytes and securely delivers larger complete renderings through a caller-owned mode-0600 temporary file and versioned two-line notice.
- New `awf check memory` command scans the staged decisions and plans directories for a citation of
  a specific working-memory file and exits non-zero on any finding, and `awf check commit` applies
  the same detector to the git-cleaned commit-message body (ADR-0158). A mention of the bare
  `.awf/memory/` directory, an angle-bracket placeholder segment, and the directory's own ignore
  file all pass; only a concrete file name is a finding. Both scans are opt-in through the new
  `memoryCite.enabled` key and off by default, and `memoryCite.exemptions` permits a path that
  genuinely needs one (the commit-message scan honours no exemption, since an exemption is keyed by
  path). A new `memoryGateCmd` var and an unconditional `awf check memory` line in the rendered
  pre-commit payload re-render on your next `awf render`. One breaking edge: a project with the hooks
  singleton enabled and the runner singleton explicitly disabled must now set `memoryGateCmd`
  before `awf render` or `awf check` will pass, the same requirement the other three hook-referenced
  awf-verb vars already carry.

### Bug fixes

- Managed context instructions now require explicit paths (or a staged/range selection) and describe the initial query as omitting detail flags, avoiding the ambiguous "start bare" wording that could lead agents to invoke `awf context` without its required selection.

- Pi `handoff_session` now emits Remote Pi's optional continuation disposition after it successfully queues the replacement command. Compatible push integrations no longer report the intermediate parent run as a terminal completion, while listener failures remain isolated from handoff execution.

- `awf check` no longer refuses an integration whose effort branch was forked before schema
  generation 29. Merging an integration branch that has already sealed the ADR v3 cutoff crosses
  that generation in one step, and the record being renumbered may land above the cutoff, which
  forces it to take the v3 encoding. Three transition rules refused that shape: the permanent lock
  now admits a cutoff inherited from the other parent when the generation advances across the seal,
  the renumber digest index admits a numbered record whose slug is new in the transition, and one
  governed-format change is sanctioned, a v2 record renumbered to a v3 one. A pending record is
  still never paired, so a deletion beside an unrelated addition is still refused, and a retained
  slug still cannot change. `docs/roadmap.md` records the decision this class still owes.

- Managed-worktree refusals no longer direct an agent to resolve or discard work that may belong
  to a concurrent effort. A checkout mid-merge is now refused with "finish or abort this merge
  only if you started it; otherwise wait until this checkout is clean, then retry", and the
  refusal names the effort whose tip is being merged when it can prove one from repository truth
  alone (MERGE_HEAD matched against the effort branches registered at their managed worktrees).
  The cleanliness refusal, which cannot attribute unstaged work, now asks the caller to confirm
  the changes are its own before acting. Previously the cleanliness refusal told a blocked agent
  to inspect and discard the changes with native Git and the merge refusal told it to finish or
  abort the operation unconditionally, either of which destroys a peer's staged integration when
  several efforts finish at once. The other four in-progress operations (cherry-pick, revert, and
  the two rebase states) keep the unconditional finish-or-abort guidance. Attribution is
  best-effort and decorates the refusal without deciding it: an unresolvable probe drops the
  effort name and keeps the same instruction.

- Divergent effort-integration guidance now derives the project gate command from `vars.gateCmd` and uses generic project-gate prose when that value is unavailable.
- Pi fresh-session handoff now accepts absolute memory paths confined beneath the repository memory root, normalizes them to canonical repository-relative slash form, requires a regular file, and revalidates the checkpoint after the countdown immediately before replacement.
- Managed effort worktrees now support current-user-owned checkouts beneath system-owned filesystem ancestors while retaining ancestor symlink, resident ownership, and repository-identity protections.
- `awf context` no longer reports an in-flight decision record as frozen. It derived mutability from
  whether the record was still Proposed, which stopped being the amendability rule once a
  current-state-v2 body became amendable through Accepted and Implementing, freezing only at a
  terminal status. Those two statuses now report `mutable`; terminal records, and every
  current-state-v1 record outside Proposed, are unchanged.
- `awf render` and `awf check` now fail when a directory under the project tree cannot be read,
  instead of silently enumerating what they could reach. A truncated enumeration narrowed the set
  of managed outputs the drift oracle was computed over, so an unreadable directory could produce
  a clean verdict and exit 0 over a tree that was never fully inspected.
- Every command now refuses when it cannot determine whether a current-state upgrade journal
  exists, instead of reading the failed check as "no journal". An unreadable `.awf` therefore no
  longer permits the commands an unrecovered upgrade is meant to block.
- `awf upgrade --recover` now restores each journaled file through a temp-file-plus-rename write,
  so an interrupted recovery cannot leave a partially written file at a path the journal records
  as holding a whole image. The restored file keeps its recorded permissions.
- `awf audit`'s uncommitted-changes rule now reads the worktree it was asked about. It shelled out
  to Git with the inherited environment, so an inherited `GIT_DIR` selected a different repository
  and the rule could report a dirty tree as clean.
- `awf context` now classifies an absolute request as outside the repository on Windows. The check
  asked only `filepath.IsAbs`, which answers false there for a slash-rooted path, so such a request
  was reported as merely not found. Unix behaviour is unchanged.

### Others

- The repository no longer carries the committed `examples/sundial` adopter or its fixed-path runner and hook orchestration. Focused temporary fixtures now preserve catalog-derived Claude and Pi rendering and drift checks, representative authored-adoption repair, governed Pi output directives, legacy upgrade-through-check composition, and generic nested-adopter behavior without maintaining a second generated tree.

## [0.22.0] - 2026-07-24

### Features
- New `awfInvokeCmd` var overrides how the rendered `./awf` wrapper invokes awf; unset, it
  resolves the bootstrap-pinned binary and falls back to PATH `awf`.
- The rendered agent guide is now an entry-point router (ADR-0157): the workflow section becomes a
  catalog-derived entry-skill trigger table, the working-memory and awf-setup sections shrink to
  routing minimums, and the working-memory protocol moves to a new canonical working-memory section
  in the workflow doc, with the shared checkpoint partials, the brainstorming skill, and the chain
  section re-pointed there. The neutral guide and singleton-doc render now honors a project-level
  session-handoff signal, so Pi-gated singleton prose renders for session-handoff-capable projects.
  Adopters re-render a much smaller AGENTS.md on their next `awf sync`. If you replaced the workflow
  doc's chain section with a full-replacement part, the checkpoint protocol prose relocated to the
  new working-memory section and your part will not receive it; re-derive your part or adopt the
  new section.
- Workflow skill templates now ground implementers in current-state authority: seven
  implementer-chain skills (executing-plans, subagent-driven-development, writing-plans,
  bugfix, debugging, tdd, refactor-coupling-audit) instruct a concise `awf context` run over
  their touched paths before editing, the reviewer dispatches (reviewing-impl, reviewing-plan,
  and the previously packet-free resync) instruct the reviewer to run `awf context --full`
  itself with parent-resolved arguments instead of pasting packet output into the brief, and
  the ADR-reviewer brief gains an `awf topic` destination-topic hint (ADR-0155). Adopters see
  the skill drift resolved by their next `awf sync`.

### Bug fixes
- `awf audit` now uses native Git status semantics for its uncommitted-change check, preventing ignored managed-worktree residents from appearing as false untracked files.
- Pi session-v1 telemetry now validates native Git directory and gitdir-pointer control roots, refuses unsafe lock cleanup and corruption markers without following replacements, and supports separate-Git-dir primary checkouts.
- Lightweight effort records now use safe native file access, repository locking, and conditional atomic publication on every supported Linux, Darwin, and Windows release target; raced creation and replacement preserve the existing destination.
- Pi fresh-session handoffs now explicitly identify their restored active effort association and prohibit external-checkpoint adoption or structured resume, preventing a successor from re-adopting the already-associated effort.
- `awf check` accepts the historical operations for a topic retired after another ADR removes its final claim, rather than reporting its already-removed topic metadata as missing.
- `awf check` no longer emits the "carries no tags: add a narrow topic tag" advisory for
  governed current-state ADRs (v1 and v2): their closed frontmatter rejects a `tags:` key, so
  the note was impossible to satisfy. The advisory still fires for tag-capable legacy ADRs and
  pitfalls under a non-empty tag vocabulary.
- Pi anchor claims are now owned exclusively by `trajectory_closed` events keyed on the payload
  anchor, declared in the protocol descriptor's required `anchorClaimKinds` vocabulary; the
  envelope `piAnchorId` is observation-location metadata the causal checker never reads, and
  references resolve causally forward only. The checker stops flagging legitimate producer
  co-anchoring as `ambiguous-anchor` (previously accreted on every real session and defeated
  fork resolution), accumulated findings clear retroactively without any ledger rewrite, and
  the unsound anchor-based association invalidation on trajectory resume is removed from both
  the Go projection and the dashboard mirror (ADR-0154). For external protocol writers this is
  a semantics change, not only a fix: events of non-claiming kinds are no longer
  anchor-resolution targets.

### Breaking changes
- The runner singleton now renders a pure awf wrapper `awf` at the repo root instead of the
  co-owned `x`: no per-verb dispatch, no in-place project-verb sections. Project verbs live in
  the adopter's own runner; a pruned co-owned `x` is backed up to `x.awf-bak` for the one-time
  hand-port. `awf upgrade` (schema generation 18) seeds the wrapper enabled unless the config
  carries an explicit `enabled: false`, and hooks with an unset `gateCmd` (or, with the runner
  disabled, an unset `checkCmd`/`commitGateCmd`/`proseGateCmd`) now fail `awf sync`/`awf check`
  instead of degrading silently.
- awf 0.22.0 advances to schema generation 17 with a strict, tracked `workflowTelemetry`
  configuration block for retention, dashboard widget behavior, diagnostics, and heuristic thresholds.
  Existing adopters must run `awf upgrade`; the migration writes the complete defaults.
- awf 0.21.0 advances to schema generation 16 with strict `currentState.maxClaimsPerTopic`
  configuration. Existing adopters must run `awf upgrade`; the migration writes the explicit default
  of 20 while preserving an explicit positive value and the sealed ADR format cutoff.
- Current-state topics are now awf's single active authority, replacing the ADR-derived context,
  supersession, and invariant-authority engines. Active rules and invariants live as individually
  identified claims in domain-owned topic documents under `.awf/topics/`, rendered to `docs/topics/`;
  `awf context`, `awf check`, `awf invariants`, and the new `awf topic` read those claims rather than
  the ADR corpus.
- ADRs use the `current-state-v1` format: closed `format`/`status`/`date` frontmatter, the ordered
  sections Context, Decision, State changes, Consequences, Alternatives Considered, and Status history,
  and the four-state lifecycle Proposed/Accepted/Implemented/Abandoned. The `Superseded` status and all
  anchor-level ADR-to-ADR supersession are removed; a later decision changes the affected current-state
  claims through its own `## State changes` operations (`add`/`update`/`remove` over qualified
  `<domain>/<topic>:<slug>` claim ids). `awf check --staged` verifies each operation against the
  matching claim mutation in one Git transaction, and the rendered pre-commit hook runs it.
- `docs/decisions/ACTIVE.md` and the per-domain ADR indexes are replaced by a generated
  `docs/decisions/INDEX.md` (In flight and compact History); generated domain docs link a compact topic
  list instead of an ADR index.
- The legacy `invariants` config block is replaced by `currentState` (marker `sources`, `testGlobs`,
  `topicCoverage`/`topicFanout` severities, `maxTopicsPerPath`); scanned markers are `state:`,
  `invariant:`, and `touches-state:` over qualified claim ids.
- Crossing an existing project to this release is a one-time cutover: run the preceding bridge release
  to attest the prepared tree, then this binary's plain `awf upgrade` consumes the seal (with
  `awf upgrade --recover` for an interrupted cutover). The bridge's `awf upgrade --check` and
  `--attest-current-state` modes live only in that preceding release; this binary consumes seals and
  never produces them.

### Features
- Rendered chain skills now carry two boundary protocols (ADR-0152): the end of brainstorming and
  the settled ADR review render a mandatory approval check-in that stops for explicit user
  approval, while every other checkpoint boundary persists working memory, classifies whether user
  attention is required, and either raises a check-in or states a continuity notice and continues.
  The implementation skills, including the direct route on memory-backed efforts, embed the
  complete routine protocol at their per-task sections, and a routine checkpoint summary is a
  continuity notice rather than an intervention point.
- The Pi target's governed subagent tools resolve an omitted `model` through extension-owned local
  per-role preferences before inheriting the parent: a user-global `awf-subagents.json` and a
  gitignored project-local `awf-subagents.local.json` set a shared default, four explicit role
  models (grounding, exploration, review, implementation), and small, standard, and large tier
  mappings. Completeness requires all eight fields explicitly after project-over-global merging;
  missing fields remain visible and non-blocking, while invalid configured state blocks implicit
  routing and leaves valid explicit calls usable. Omission is the only default form; sentinel
  values are rejected, and exact references are limited to 256 characters. Preferences and the
  live registry are validated at preflight and again immediately before startup: queued roles
  refresh after acquisition, while direct roles refresh immediately before their child starts, with
  routing-source diagnostics. The `/awf-subagent-models` TUI wizard writes
  roles and tiers atomically, with a registry-gated recommended preset, informed per-model pricing
  selectors, and save-time gitignore enforcement for the project-local file. Rendered guidance now
  steers long implementations toward sequential implementation subagents.
- `awf context --uncovered` annotates each collapsed unowned directory with how many unowned
  files it covers and how many files beneath it are excluded from coverage (generated, ignored,
  or otherwise ineligible), so a mostly-generated directory no longer reads as wholly unowned.
  The JSON `unowned` array becomes structured entries.
- `awf context` output is grouped by topic: each applicable topic renders its authority exactly once
  per invocation (selectors, a matched-path count with an `awf topic <id> --coverage` drilldown, the
  uncapped claim-ID roster, and the deduplicated direct-claim detail with an explicit detail-omission
  line), while effective paths carry classification and attribution and `eligible-unowned` paths gain
  a remediation hint. `--full` renders every applicable claim once per topic instead of once per file.
  The JSON projection serializes the same grouped model (a shape change, pre-1.0, no bridge). Rendered
  topic docs reduce their Applicability paragraph to selectors plus the coverage drilldown, so adding
  a file to a matched package no longer rewrites topic docs; existing adopters see one-time topic-doc
  drift resolved by their next `awf sync`.
- Pi targets now ship a privacy-minimal, trajectory-aware workflow ledger and dashboard on ledger
  protocol 2, with interactive metrics/doctor views, confirmed maintenance, parent-handoff
  association, and resident history preserved by uninstall. One discoverable `awf-workflow` router
  fronts the governed workflow chain: its `awf_workflow` tool settles or resumes effort identity,
  durably applies the catalog-mapped lifecycle effect as a single transactional protocol-2
  transition, and only then returns the fixed pre-rendered skill body, so lifecycle telemetry cannot
  be bypassed. A fresh session shows `[awf:init]` and buffers bounded provisional telemetry in memory
  until the first router call or an explicit `/awf-resume-effort <effort-id>` settles identity; the
  muted below-editor widget mirrors Pi footer accounting from unique active-branch assistant entries,
  nested subagents and restored history included, with `[awf:<phase>]`, `[awf:done]`, and
  `[awf:abandoned]` badges. Canonical findings carry their owning effort, repair and waiver
  re-resolve against the current causal frontier, and canonical refreshes apply in generation order.
  The canonical `awf metrics` and read-only `awf doctor` commands provide selector-scoped human,
  JSON, and export surfaces. Pi does not currently produce shell/gate observations because its tool
  API exposes command text rather than a trusted token vector; the typed protocol shape remains
  reserved.
- `awf context` now preserves request-to-effective-path attribution, reports one primary path
  classification and known-artifact navigation, defaults to directly relevant concise claims, and
  adds an untruncated `--full` authority packet plus lifecycle-aware explicit ADR navigation.
  `awf topic --coverage` shares honest domain/topic applicability evidence, and managed runners derive
  their forwarded commands and exclusions from the CLI command table.
- New ADRs use `current-state-v2` after a metadata-only schema-15 cutoff upgrade. The new
  `Implementing` state and append-only `Applied` events let one frozen decision apply claim operations
  in independently checked batches across commits, including interleaved ADRs and partially Abandoned
  execution, while context reports only the remaining operations as pending and preserves V1 history.
- Implementation plans may use implementation-ready pseudocode for logic and non-contractual prose while keeping machine-consumed, contract-bearing, fixture, golden, command, mechanical, and literal-text portions exact; the writing skill, reviewer, scaffold, plan guide, and agent guide now enforce the same boundary.
- Pi's four governed subagent roles now accept strict optional exact model routing, independent
  exploration runs through an abort-aware ten-active FIFO queue, and mixed implementation batches
  are mechanically blocked before any member executes. Active tool rows now show queued/running
  state, resolved and actual models, thinking level, role options, and cumulative per-turn usage with
  Pi-style cache-hit statistics.
- Pi targets now render a guarded `handoff_session` extension that can replace a persisted TUI
  session with a parent-linked fresh session after a cancelable countdown; it requires a working
  memory file carrying the active effort's exact `Effort: <id>` line and restores the association
  before kickoff, while effort creation itself never requires a checkpoint or memory file. A
  post-queue failure that leaves the old session active places the prepared kickoff wrapper in
  that session's editor and raises a visible failure notice; the extension never retries
  automatically and never starts a model turn (ADR-0152). The Pi
  extension fixture pins the checksummed `hypnotox/pi` 0.81.1 awf.3 fork that provides queued
  extension commands.

### Bug fixes
- `awf context` no longer attributes domains and topics to a user-typed glob query that matches
  nothing: the star-containing string previously string-matched domain globs, producing misleading
  half-answers. Such paths now carry `globLiteral` in JSON and a "globs are not expanded" hint in
  text.
- Working-tree path universes now honor the user's global and system gitignore
  (`core.excludesfile` in `~/.gitconfig` and `/etc/gitconfig`), matching `git status`: files real
  git treats as globally ignored no longer surface as unowned in `awf context --uncovered`, no
  longer classify as eligible-unowned, and no longer enter working-tree snapshots. One narrow
  divergence remains: a repository-level `.gitignore` negation cannot re-include a globally-ignored
  file.
- First adoption now records the executing awf version and seals ADR cutoff authority before render:
  cutoff 1 for an empty corpus, or highest-plus-one with explicit gaps for validated brownfield
  history. Sync and forced init preserve that provenance, while unattested older projects are refused
  before destructive upgrade mutation.
- Staged lifecycle validation now rejects changes to frozen ADR content, non-append-only Status
  history, and Accepted or Implemented operations whose destination topic metadata is absent.
  Abandoned removals are attributed from the actual snapshot pair rather than final absence alone.
- `awf topic --history` now resolves removed claim identities, including legacy-baseline removals
  whose origin is not retained, without fabricating active prose or tombstones. `awf context` now
  includes topic summaries, invariant backing, and unbacked `Verify` instructions, and applies
  `contextIgnore` consistently to directory expansion and coverage.
- Topic scaffolding now replaces both date placeholders and validates its generated current-state-v1
  ADR. Agent workflows explicitly stage the complete transaction and run `awf check --staged` before
  the project gate instead of assuming hooks are installed.
- Staged current-state checks and `awf context --staged` now read config, lock, topics, markers, and
  coverage from one index snapshot. Staged transitions reject changes to the permanent ADR format
  cutoff or legacy gaps, and `awf context --uncovered --staged` reports index-only coverage.
- Context directory queries, including repository root `.`, now expand only eligible descendants and
  return no paths for an existing directory with none. Topic rendering also normalizes trailing part
  newlines so generated topic documents end with exactly one newline.
- The permanent pre-commit path no longer accepts the preparation-only bridge bypass, and a
  reintroduced `.awf/current-state-migration.yaml` is reported as unclaimed drift after cutover.

### Others
- Purpose-partition the effort memory skeleton into consumer-named sections: `## Brief` with durable-artifact pointers, an append-only ordinal `## Decision log` (the effort-spanning consensus record with verbatim `Record:` evidence blocks on user entries), a new at-occurrence `## Observations` log, and `## Handoff log`. Checkpoint guidance gains a backstop append for unrecorded decisions and observations, the full-review dispatch briefs paste user-provenance entries verbatim, the shared review spine gains a consensus-adherence check that routes any deviation from a user entry as a user-decision finding, and the retrospective reads both logs as primary input with recurrence tracked across an effort's sessions. Pre-existing memory files migrate on first write by appending the missing headings.
- Split three overloaded invariant claims (version-compat gate, metrics/doctor command contract,
  context authority packet) into six focused single-obligation claims (ADR-0153); no behavior
  change.
- The concise `awf context` text rendering prints each domain's selector block once per domain
  group (as a named `Domain <name> paths:` header) instead of repeating it verbatim under every
  topic of that domain; JSON is unchanged.

## [0.18.0] - 2026-07-20

### Breaking changes
- Version 0.18.0 introduces schema generation 14, `current-state-topic-substrate`, and the optional
  strict `currentState` bridge-preparation config beside unchanged legacy `invariants`. The new keys
  describe marker sources, proof test globs, topic coverage and fan-out severities, and a positive
  per-path topic budget (default 8), but they do not switch normal context or invariant authority.
  0.18.0 ships the current-state bridge (the topic substrate and the migration tooling) as one
  tranche; the authority switch and adopter cutover follow in a later current-state release.
- Pi's `subagent_explore` now requires `{task, breadth, detail}` (ADR-0132). Breadth is
  `targeted`, `bounded`, or `broad`; detail is `paths`, `summary`, or `analysis`. Hand-authored
  calls that pass only `task` must add both fields.
- The new core `exploring` skill gives every target one bounded exploration and reporting
  protocol. schema-13 `exploring-skill-closure` automatically adds it to adopted configs that
  enable brainstorming, debugging, or refactor-coupling-audit; run `awf upgrade`.
- The `supersedes:` and `superseded_by:` ADR frontmatter keys are removed, and full ADR
  supersession is now derived from anchor coverage rather than declared (ADR-0128, ADR-0129,
  ADR-0130). **Run `awf upgrade`: the generation-12 migration rewrites `docs/decisions/`**,
  stripping both keys, downgrading every pre-existing `` `supersedes: ADR-NNNN#<item>` `` token
  to the new `` `refines:` `` relation, appending a supersedence-bookkeeping Decision item that
  retires each superseded predecessor's anchors, backfilling the predecessors' `related:`
  back-pointers, and rewriting `status: Superseded by ADR-NNNN` to bare `status: Superseded`.
  `awf check` refuses either key with upgrade guidance.

  Supersession now has one encoding and two relations: `` `supersedes:` `` and
  `` `supersedes-invariant:` `` **retire** an anchor (a Decision item or a declared invariant
  slug), while the new `` `refines:` `` **adapts** one and counts toward nothing. An ADR is
  `Superseded` exactly when every one of its anchors carries a retirement from a carrier that
  has shipped (`Implemented` or `Superseded` - superseding an ADR does not un-supersede what
  that ADR superseded, so chains deeper than two generations resolve); the status stays hand-authored and `awf check` refuses drift in both directions, naming
  the required edit. The status is bare because coverage may split across several successors.
  The mechanical migration downgrades every existing item token to `refines:` deliberately - it
  asserts less, and promoting a genuine retirement back is a reviewable edit, whereas a wrong
  retirement silently kills an ADR.

  Two further checks arrive with it: the `related:` back-pointer is now owed for token targets
  of **any** status (previously live targets only), and a token claiming its own carrier's
  anchor, or a retirement cycle among fully covered ADRs, is refused. The advisory for a token
  into an already-superseded target is dropped, since that is now the normal shape of every
  completed supersedence.
- `awf audit` now requires an explicit commit range and has no default (ADR-0127). Pass a bare
  `<base>` (meaning `<base>..HEAD`) or a two-sided `<a>..<b>`; a no-argument invocation is refused.
  The `--base` flag is removed, superseded by the positional argument. `./x audit-local` (this
  repo's own tooling) loses its `origin/main..HEAD` default on the same grounds.
- The `audit.baseBranch` config key is removed, along with its `main` default. A schema-11
  migration strips it from `.awf/config.yaml` and reports the removal, so run `awf upgrade` after
  upgrading the binary. awf no longer holds an opinion about which branch you integrate into: a
  configured base that already contained HEAD silently emptied the range and made every history
  rule inert while the command still reported clean.
- Pi target adopters now receive executable project extension files under
  `.pi/extensions/awf-subagents/` on sync (ADR-0123). Pi's project-trust boundary applies and Pi
  0.80.9 or newer is required; `awf check` reports extension drift and `awf sync` repairs it. Pi
  workflow skills call the four governed extension tools explicitly, while other targets retain
  their existing dispatch language.
- ADR supersession is structured and machine-checked (ADR-0120), and the `retires_invariants:`
  frontmatter key is removed from the ADR schema. **Run `awf upgrade`: the generation-10
  migration rewrites `docs/decisions/`**, stripping every `retires_invariants:` key and
  appending a retirement-bookkeeping Decision item carrying one
  `` `supersedes-invariant: ADR-NNNN#<slug>` `` token per retired slug (plus the target's
  `related:` back-pointer); a corpus still carrying the raw key fails `awf check` until the
  migration runs. Two token grammars express partial supersession inside a successor's Decision
  section: `` `supersedes: ADR-NNNN#<item>` `` (a Decision item) and
  `` `supersedes-invariant: ADR-NNNN#<slug>` `` (an invariant, which an Implemented carrier
  retires from owed backing). New `awf check` errors, any status: every ADR's Decision section
  must be column-0 numbered items sequential from 1 (item numbers are the stable anchors);
  `supersedes:` frontmatter is finally parsed and its three-way symmetry enforced (claim,
  `Superseded by ADR-NNNN` status flip, scalar `superseded_by:`); every token's ref must
  resolve (existing non-Proposed target, in-range item or declared slug); a token into a live
  target requires the target's `related:` back-pointer; and one successor cannot both fully and
  partially supersede the same target. Two non-failing advisories: a token whose target was
  later fully superseded, and one anchor claimed by two live ADRs. `ACTIVE.md` gains a
  `## Supersedence` section (full chains plus superseded anchors on live ADRs; omitted for a
  supersession-free corpus) and `awf context` annotates surfaced ADRs with their overridden
  anchors.
- Seven typographic punctuation substitutes are banned from the prose awf ships (ADR-0115): the
  em-dash (U+2014), en-dash (U+2013), ellipsis (U+2026), and the four curly quotes (U+2018,
  U+2019, U+201C, U+201D). The generated `docs/decisions/ACTIVE.md` now renders a row's status in
  parentheses (`- [ADR-0001: Title](0001-file.md) (Accepted)`) instead of after an em-dash, so
  **every adopter's committed `ACTIVE.md` drifts until they run `awf sync`**, and `awf check`
  reports it until they do. The shipped templates, awf's own output strings, and this changelog
  are cleaned to match. The rendered documentation standard's plain-punctuation rule is rewritten
  to name all seven codepoints and now covers authored prose (ADRs, plans, and hand-written docs)
  as well as shipped prose (ADR-0117), so `docs/doc-standard.md` re-renders too. Nothing rewrites
  prose you have already written. Notation (arrows, mathematical symbols, accented letters) is
  unaffected.
- Invariant backing is redesigned into enforced test-backing with a two-marker
  vocabulary (ADR-0105, ADR-0106). The ADR Invariants-section declaration token is
  unified from `inv: <slug>` to `invariant: <slug>`: the same token the source
  proof marker uses (**adopters must rewrite `inv:`→`invariant:` in their own
  `docs/decisions/**`; awf cannot auto-migrate user-owned ADR prose**). Source
  markers split into a proof `invariant: <slug>` marker and an advisory
  `touches-invariant: <slug>, <note>` context marker. A new `invariants.testGlobs`
  config scopes the proof marker to test files (backing then means an executed test
  line); when it is empty or absent, backing falls back to source-glob scope, so
  the change is additive for projects that do not set it. Each invariant is declared
  `invariant:` (backed) or `unbacked-invariant:` (unbacked, carrying a `Verify:`
  note), symmetrically enforced: `awf check` fails a backed slug with no proof
  marker, an unbacked slug that has a proof marker, and an unbacked declaration
  missing its `Verify:` note; a marker naming an undeclared slug and a note-less
  `touches-invariant:` are non-failing advisories. `awf context` now labels each
  governing invariant `backed`/`unbacked` and surfaces its `Verify:`/touches site
  notes (the `--json` invariant refs carry per-invariant `class` and notes), reading
  as a risk map: its Tier-1 scan spans both markers across the source and test globs.
- `awf context <paths>` output is now relevance-tiered (ADR-0104). It no longer
  dumps every ADR/pitfall sharing a queried path's domain. The human render gains
  `## Governing ADRs (invariants backed here)` (Tier 1: ADRs whose invariants are
  backed under the queried paths), `## Related ADRs (shared tag)` / `## Related
  pitfalls (shared tag)` (Tier 2: sharing a finer-than-domain precise tag or
  `related:`-linked), and a one-line `## Domain background: N more ADR(s)` (Tier 3,
  collapsed). The `--json` shape changes accordingly: the flat `adrs` array is
  replaced by `governing` + `related` + an integer `background`; each ADR ref drops
  its `invariants` echo; and each pitfall ref carries `tags` instead of `domains`.
  Pitfalls now surface by shared tag, not by domain membership. The
  `context-surfaces-pitfalls` and `context-surfaces-linked-plans` invariants are
  retired for tiered successors. Read-only, output-parity, and static-fallback are
  unchanged.
- Pitfalls become a structured, domain-tagged sidecar-derived doc (ADR-0099).
  `docs/pitfalls.md` is no longer authored as a free-prose `entries` part; its
  entries now live as a `data.pitfalls` list of `{title, domains, related, body}`
  in `.awf/docs/pitfalls.yaml`, rendered by a transform (the ADR-0089 seam the
  glossary uses). The schema-9 `pitfalls-data` migration ports adopters on the
  next `awf upgrade`: it auto-splits an existing `entries.md` on its top-level
  `##` headings (fenced-code `##` lines skipped) into `data.pitfalls` entries with
  empty `domains`, deletes the part, and prints one provenance line per entry plus
  a review instruction. **Review the split and tag each entry's `domains:`**;
  untagged entries render but do not surface in `awf context`. An entry's optional
  `related:` ADR numbers render as plain `ADR-NNNN` text and are link-validated.
  `awf check` now fails on unparseable pitfalls data, a bad entry shape, an unknown
  `domains:`, or a dangling `related:`. Schema bumps to 9 (awf `0.17.0`).

### Features
- `awf upgrade --check` reports exhaustive current-state bridge readiness without writing the
  worktree, index, config, lock, approval input, or generated output. It inventories exact shipped
  legacy invariant declarations and retirements, plans idempotent Migration history/status/config and
  qualified-marker normalization, requires repository-reviewed strict
  `.awf/current-state-migration.yaml` evidence only after independently deriving each unique
  Origin/backing-preserving mapping, checks scoped topic coverage and migration-safe terminal output
  deletion, and emits deterministic human or `--json` findings, computed adjudications, and exact
  before/after path/mode/SHA-256 mutation records. The ephemeral approval input does not bump schema
  14, cannot disambiguate mappings, requires `invariantApprovals: []` for an empty live inventory,
  and is omitted from mutations when unchanged. The authority switch and runtime consumption of the
  attestation are intentionally still absent.
- `awf upgrade --attest-current-state` seals a ready, clean-HEAD prepared tree through a recoverable
  transaction. It reruns readiness, refuses any staged, unstaged, or untracked change, records the
  clean HEAD, a digest over the post-normalization config, domains, ADRs, topics, marker sources, and
  the required approval file, and the ADR cutoff and gaps in an optional `bridgeAttestation` lock
  block (old locks omit it). It then journals every normalization, marker, status, and terminal
  legacy-index deletion at `.awf/current-state-upgrade.journal`, applies them, and commits the
  attested lock last; the unchanged approval file never enters the mutations. Obtain and verify the
  matching current-state binary before attesting. Because the terminal projection prunes
  `docs/decisions/ACTIVE.md` and the domain ADR indexes without generating their replacements, the
  attested project is deliberately index-pruned and refuses every ordinary command until a later
  current-state release consumes the attestation.
- `awf upgrade --recover` replays the journal's recovery table: a precommit journal whose lock still
  differs from the sealed hash rolls every prior image back in reverse; a precommit or lock-committed
  journal already carrying the sealed lock cleans up the residue; a lock-committed journal with a
  different lock refuses rather than rolling committed authority back; a third-party edit halts and
  preserves the journal, naming the path. A committed journal or attestation now makes ordinary
  commands non-operational: with a journal present only `awf upgrade --recover` proceeds, with an
  attested lock only `awf upgrade --check` inspects it, a malformed journal refuses every mode
  (recovery included) with deterministic Git-restoration-and-bridge-reinstallation guidance, and
  `awf version`/`awf changelog`/`awf help` always bypass the transaction state.
- `cmd/releasecheck` carries the `project.BridgeTrancheComplete` release sentinel. Plans 1 and 2 of
  the current-state bridge are one unreleased v0.18.0 tranche, so the check refuses publication from
  any intermediate commit while the const is `false`. The sentinel gates publication until both plans
  land; with the bridge complete it is `true`, and the tranche is released as a single v0.18.0.
- `awf topic <domain>/<topic>[:<claim>]` adds a version-gated, read-only active-state query with one
  deterministic human/JSON model. Defaults show current title/summary, claims, types, prose, and
  backing while hiding provenance and references. Independent `--history`, `--references`, and
  `--coverage` flags add direct ADR details, direct incoming/outgoing claim IDs, and declared/effective
  scope plus marker sites; no option traverses transitively or resolves removed history. Outside an
  adopted tree the command prints a static reference without gating. This is bridge preparation and
  does not switch context or invariant authority.
- `awf new topic <domain> "<title>"` scaffolds exactly the paired current-state metadata and authored
  part without syncing or mutating config, lock, or rendered docs. It allocates a collision-free
  kebab slug, protects the reserved `index` and either orphaned half, rolls back the first file if the
  second write fails, and prints both repository-relative paths. The scaffold contains a valid path
  placeholder, generic prose, and no invented claims; adopters must edit and author it manually. A
  zero-claim shell renders but does not satisfy scoped coverage. This is bridge preparation, not
  runtime authority: it scaffolds topics without switching context or invariant enforcement.
- The current-state topic producer strictly pairs
  `.awf/topics/metadata/<domain>/<topic>.yaml` with
  `.awf/topics/parts/<domain>/<topic>/current-state.md`, parses canonical rule and invariant claims,
  resolves Implemented-ADR provenance and direct references, and validates qualified configured
  state, touches, and proof markers. It renders managed topic pages and sorted per-domain indexes,
  adds compact domain navigation without removing Decisions, and joins ordinary output-plan,
  manifest, brownfield backup, drift, collision, and prune behavior. This is preparation substrate,
  not shadow authority: legacy context and invariant enforcement remain unchanged, and the authority
  switch follows in a later current-state release.
- **`awf check` reports a supersession claim stated in prose and never encoded** (ADR-0131). The
  new `adr-unencoded-claim` finding fires when an override verb occurs in the same Decision item
  as a citation of another ADR's anchor and that item carries no relation token for it, naming the
  carrier, its item, the anchor, and the token shapes that would satisfy it. Item citations are
  recognized in six spellings: `ADR-NNNN Decision item N`, `ADR-NNNN Decision N`,
  `ADR-NNNN item N`, `ADR-NNNN DN`, plus the possessive `ADR-NNNN's ...` and markdown-link
  `[ADR-NNNN](path) ...` wrappers. Exemptions are structural, never a marker: a `Proposed` target,
  a self-citation, a slug the target never declares, anything outside `## Decision`, and an **item**
  citation inside an inline code span, so an ADR can discuss the item-citation grammar without
  tripping it. A slug citation is recognized regardless of its code span: the backticks in
  `` `inv: <slug>` `` are the citation syntax itself rather than a quoting device, so masking them
  would recognize none. To record one as informational, write `cites-invariant:`.
- **Two new relation tokens, `cites: ADR-NNNN#<item>` and `cites-invariant: ADR-NNNN#<slug>`**
  (ADR-0131), for a Decision item that mentions, quotes, or reasons from another ADR's anchor
  without changing it. A citation asserts nothing: it contributes no anchor coverage, so it cannot
  retire an ADR or drop an invariant's backing, and it renders in no `ACTIVE.md` or domain-index
  annotation. It exists so the check above has a truthful answer for an informational citation;
  without it an author reaches for a relation token and records a supersession that never
  happened. It still owes the `related:` back-pointer every relation owes. Judge the key by the
  target's clause set, not the carrier's verb.
- `awf check` reports `adr-related-order` when an ADR's `related:` array does not ascend, naming
  the first descent (ADR-0131). A back-pointer edge has exactly one correct position, so appending
  a low-numbered carrier to an array that already names a higher one is an authoring slip that
  previously went unseen. Resolution and ordering are reported independently, so a descending
  array still has every entry checked against the corpus. Sorting an existing array is a
  meaning-preserving edit: `related:` carries an unordered set.
- `awf audit` reports the range and commit count it evaluated on every run, so a verdict is never
  readable without its scope. A range resolving to zero commits says so explicitly instead of
  printing `clean`, which previously made "examined forty commits" and "examined none"
  indistinguishable.
- Pi now ships dedicated `subagent_grounding` and binds brainstorming to it while retaining
  `subagent_explore` for general investigation and coupling audits. All four subagent roles show
  bounded inline progress from context-isolated details; only final report or failure-summary
  content reaches the parent model (ADR-0125).
- The Pi target ships `subagent_grounding`, `subagent_explore`, `subagent_review`, and
  `subagent_implement`: isolated no-session child processes for grounding, read-oriented
  exploration, the three governed reviewer bodies, and serialized same-checkout implementation
  with explicit commit permission (ADR-0123, ADR-0125).
- Codex, Pi, Gemini, and Copilot are now selectable targets for agent artifacts.
  Codex renders skills under `.agents/skills/` and validated custom-agent profiles
  under `.codex/agents/` as TOML with `name`, `description`, and
  `developer_instructions` fields. Pi renders generic review-dispatch wording;
  Gemini imports `AGENTS.md` through `GEMINI.md`; Copilot uses `.agent.md` agents.
- Whole-line `<!-- awf:comment ... -->` authoring comments in templates and convention parts
  (ADR-0121): stripped at render with their newline, so parts and templates can carry
  internal notes and `touches-invariant:` tags that never reach rendered output. Whole-line
  and exact-literal only (mid-line and whitespace-variant forms still render; a malformed
  whole-line opener is a hard render error naming the source; fenced demos are preserved).
  `invariants.sources` entries gain an optional `close:` token (`-->`, `*/`) stripped from
  marker lines before tag parsing, so block-comment-family markers - the new tagging recipe
  included - yield clean touches notes.
- New `awf prose-gate` command (ADR-0119): a blocking, presence-level scan of every tracked text
  file for the seven banned typographic punctuation substitutes, the counterpart to the advisory
  `plain-punctuation` audit rule. It is opt-in through `proseGate.enabled` (bool, default off,
  because a presence gate would fail an unswept tree on the day it lands) and exits zero without
  scanning when off, so a hook may invoke it unconditionally. Genuine depictions are pinned in
  `proseGate.exemptions` (a list of `path` plus `codepoint`, the codepoint spelled as `U+2014`
  rather than typed, with an optional `count`). Adopters wire it into a pre-commit hook: the
  rendered `.awf/hooks/pre-commit.sh` payload gains an `awf prose-gate` line, so **an adopter who
  upgrades sees their committed payload drift until they run `awf sync`**, even with the knob off.
  The documentation standard's plain-punctuation rule now also lists the bare hyphen as a valid
  em-dash replacement.
- `awf audit` gains an advisory `plain-punctuation` rule (ADR-0117), on by default and switched
  off with `audit.plainPunctuation: false`. It warns, and never errors, when a commit **raises**
  the count of a typographic punctuation substitute in an authored markdown file under `docsDir`.
  Prose already written never warns: only a net increase does, so there is no allowlist, no cutoff
  date, and nothing to migrate. Generated files are skipped.
- awf can now render **co-owned files with in-place-editable sections** (ADR-0100) and ships
  a **managed command-runner `x`** as their first consumer (ADR-0101). A section declared with
  the `inplace` marker has its body read back from the existing rendered output (bounded by its
  `awf:edit-in-place` provenance pointer and awf's next section pointer) and preserved across
  syncs, while awf regenerates every other section and the file structure; such a file is
  drift-checked by regeneration-with-read-back (a first-class `RegenChecked` attribute that
  replaced awf's hardcoded generated-index list). Two shell-script properties are now rendered per
  target off the one `#!`-shebang predicate: the surviving `awf:edit`-family pointers take the
  target's comment syntax (`#` for a shebang script, HTML otherwise), and a rendered `#!` file is
  written executable (`0755`, enforced every sync), so **the bootstrap and hook payloads flip
  from `0644` to `0755` on the next sync** (harmless; still `bash ...`-invoked). Enable the new
  `runner` singleton (`awf enable runner`, or set `runner.enabled` in `.awf/config.yaml`) to render
  `x` at the repo root: awf owns the awf-verb dispatch (`sync check invariants audit context
  commit-gate new`, delegating to the pinned binary via the bootstrap), and the setup and
  project-verb regions are yours to edit in place. awf itself keeps its from-source runner; the
  `examples/sundial` adopter demonstrates the feature.
- `awf check` now validates planned commit subjects in plans (ADR-0111). A plan marks
  a phase's closing-commit subject with a fenced code block tagged `commit`; `awf check`
  reads its first non-empty line and validates it against the project's `audit` settings:
  an over-length subject, a disallowed type, or a malformed shape is drift, while an
  unknown scope is a non-failing advisory note (a plan may be the change that adds the
  scope). Tag a display-only example `commit awf-ignore` to skip that one block. The rule
  is presence-triggered, so bare-fence plans are unaffected; the plans template and the
  writing-plans skill teach the fence in prose.
- `awf context --uncovered` now reports a clean coverage floor (ADR-0110). Every
  code package has a domain home, and the report additionally subtracts awf's own
  generated outputs (`PlannedOutputs`) and a new absent-safe top-level
  `contextIgnore` config key, a list of anchored globs naming genuinely non-domain
  paths (config source, docs, the example adopter, top-level non-code files), so a
  newly-unowned path surfaces as a real signal rather than standing noise. An empty
  or absent `contextIgnore` adds no exclusion.
- Narrow-topic tag taxonomy for precise `awf context` relevance (ADR-0109). Tags are
  redefined as sub-domain topics, never domain-scale buckets: `awf check` now fails
  if any `tags:` vocabulary member equals a configured domain name, and Tier 2 drops
  its domain-name filter (the precise set is the plain union of the Tier-1 tags), so
  a domain-scoped query returns a tight topical cluster instead of a third of the
  corpus. Two advisory, non-failing `awf check` notes flag tag health: a coarsening
  note for any tag on more than 25% of the tag-bearing artifacts, and an
  under-tagging note for any ADR or pitfall with zero tags; both inert under an
  empty vocabulary.
- Governed tag vocabulary and revived ADR/pitfall metadata (ADR-0103). ADR
  `tags:` and `related:` frontmatter (long authored but parsed-then-dropped)
  are now lifted into `adr.ADR`, and pitfall entries gain an optional `tags:`
  field. A new top-level `tags:` config key declares a vocabulary mapping each
  tag to a one-line meaning; when it is non-empty, `awf check` fails on any ADR
  or pitfall tag that is not a declared member and on any member with an empty
  meaning (an empty or absent vocabulary is inert, so tags stay free-form until
  you opt in). `awf check` also now resolves every ADR's `related:` numbers
  against `docs/decisions/`. The key is additive and absent-safe (no schema
  migration) and changes nothing about `awf context` output yet.
- `awf context --uncovered [<scan-root>...]` reports git-tracked-at-HEAD paths
  matched by no configured domain glob: the inverse of the per-path domain
  resolution, and an on-demand signal for where a domain is missing (ADR-0102).
  A fully-uncovered directory collapses to its topmost node; positional args are
  optional scan roots (matched on directory-segment boundaries), while
  `--staged`/`--range` are rejected in this mode. Human and `--json` output derive
  from one result, and the mode reuses `awf context`'s read-only and
  static-fallback contracts.
- `awf context <path>` now surfaces the pitfalls relevant to a queried area
  (ADR-0099): when the toggleable `pitfalls` doc is enabled, it lists each pitfall
  whose own `domains:` owns a queried path (by the entry's tag, like an ADR, not
  transitively like a plan) in both the human and `--json` output, on the same
  read-only `ContextResult`.
- Plans get a machine-readable spine and a uniform authored shape (ADR-0097,
  ADR-0098, ADR-0108). A new `plans-template` singleton renders `docs/plans/template.md`,
  the canonical taxonomy: `date`/`adrs`/`status` frontmatter, a `# Plan:` H1,
  the Goal/Architecture-summary/File-structure header (Goal carries a one-line non-goals
  statement; the template interpolates the project's configured gate command, not a
  hard-coded literal), phases, and
  optional Verification/Notes tails. `awf new plan "<Title>"` scaffolds a
  date-prefixed plan from it (no sequential number). `awf check` now validates
  plan frontmatter (`status` enum; unparseable YAML is a hard error) and
  plan→ADR links (`adrs:` must resolve to real ADRs); the grandfathered
  frontmatter-less corpus is skipped. `awf context` surfaces each plan whose
  `adrs:` links a reported ADR, in both the human and `--json` output. The
  plan convention itself is reframed: task granularity is now "one reviewable,
  logically-coherent change" (not wall-clock minutes), a sanctioned
  coupled-phase escape covers genuinely un-sliceable changes, and plans carry
  a two-state (`Proposed`→`Implemented`) lifecycle that freezes on the plan's
  own `status`, replacing the ad-hoc `# Implementation complete` line. The
  `awf-writing-plans`, `awf-executing-plans`, and
  `awf-subagent-driven-development` skills, the `plan-reviewer` agent, and the
  plans README are reconciled to it. Adopters get it all on their next
  `awf sync`.

### Bug fixes
- Collapsed Pi subagent activity now presents bounded history chronologically: omitted older events,
  hidden retained events, then the visible live event log with the newest event at the bottom.
- Pi child failures now retain bounded progress and diagnostics in tool details while preserving
  error status through Pi's result middleware; intermediate child activity remains outside the
  parent model's visible content (ADR-0125).

### Others
- The generated Pi extension files now carry a `// @ts-nocheck` directive on the line after
  their provenance banner (ADR-0126), so adopter IDEs no longer flag `.pi/extensions/awf-subagents/`
  with errors like `Cannot find name 'Buffer'` when no `@types/node` is resolvable above `.pi/`.
  Every rendered copy stays byte-identical, and the container test lane strips that one directive
  before `tsc` so the static type-check still covers the real extension code. Adopters get the
  reworded output on their next `awf sync`.
- The `adr-lifecycle` skill now states the partial-amendment back-pointer rule, and the
  `adr-reviewer` checks it (ADR-0116). When an ADR overrides a live ADR's Decision item
  without superseding it wholesale, the overridden ADR's `related:` must name the
  overriding ADR in the same commit; previously the skill named only the successor's
  `related:`, so the amended ADR's item read as current guidance with no signal. The
  skill's append-only statements are reworded to match: a live ADR now permits in-place
  edits to `status` **and** cross-reference metadata (`superseded_by:`, `related:`),
  since append-only protects rationale, not bookkeeping. The body stays frozen.
- Invariant backing is documented as a ledger, not a proof (ADR-0114). The marker
  scan is a textual line match with no assertion awareness, so a backed `invariant:`
  slug records that a test is declared to back it, not that the property is proven.
  The ADR-README, the ADR template, and the proposing-adr skill drop the
  "test-proven property" wording; the invariants domain doc gains a ledger-not-proof
  caveat cross-referenced to the coverage doc; a new glossary term defines invariant
  backing; and the `code-reviewer` testing-discipline lens now charges the semantic
  check that a backing test actually asserts the invariant it backs. Adopters get the
  reworded prose and the lens on their next `awf sync`.
- Shipped templates are now gate-checked em-dash-free (ADR-0113). A new gate
  scans awf's embedded templates and fails on the em-dash character (U+2014),
  and the documentation authoring standard gains a plain-punctuation rule.
  The ban is scoped to shipped templates; hand-authored ADRs and plans, and
  adopter-authored parts and sidecar data, are out of scope. Adopters get the
  reworded standard on their next `awf sync`.
- The agent guide's Invariants section is now core-only (ADR-0112). The
  `agents-md-standard.md` authoring guidance gains a decidable retention
  criterion: a rule belongs in the guide only when it is not scoped to a single
  subsystem's files (process, gate, commit-hygiene, the flagship rendering
  guarantee, the toolchain preconditions, and the invariant-backing meta-rule).
  Path- or subsystem-specific invariants stay in their owning ADR and are reached
  on demand via `awf context` and the generated ADR status index; do not mirror
  the ADR ledger into the guide. Adopters get the reworded standard on their next
  `awf sync`; awf's own guide list is trimmed from 84 bullets to 10 to match.
- Em-dashes are removed from the shipped template prose across every skill,
  review agent, doc, the agent guide, and the ADR/plan scaffolds, and from the
  `GENERATED by awf` banner atop every rendered file, in favour of plain
  punctuation (colons, semicolons, commas, parentheses). The `awf:edit` /
  `awf:edit-in-place` provenance-pointer separator likewise changes from an em-dash
  to `: `. The rendered wording is unchanged in meaning; only the punctuation reads
  less machine-set. Adopters get the reworded output on their next `awf sync`.

## [0.16.0] - 2026-07-11

### Features
- The plan convention sanctions a second task form, the **batch task**
  (ADR-0095): for a transformation repeated across many sites, a plan task may
  show one representative diff (plus an edge case, unless the shape is identical
  everywhere), name the affected-site set as an exhaustive list or a reproducing
  command, and a deterministic post-check that fails if any site is missed,
  instead of N near-identical diffs. The `awf-writing-plans` skill, the plans
  README, and the `plan-reviewer` `step-exactness` lens are reconciled so a
  well-formed batch task is not flagged as under-specified. Adopters get it on
  their next `awf sync`.
- Read-only `awf context <path>...` query command (ADR-0092): for a set of
  repo-relative paths it reports their owning domain(s), the invariant slugs
  backed by markers under those paths, and the related ADRs (with each ADR's
  own declared invariants): the deterministic context awf already holds,
  surfaced instead of reconstructed by grep. Human and `--json` output;
  `--staged`/`--range <a>..<b>` resolve the paths from git. Gated and degrading
  to a static notice outside an adopted tree, like `awf config`. The workflow
  skills (`awf-brainstorming`, `awf-reviewing-impl`, `awf-reviewing-plan`) now
  call `awf context` to ground their domain/invariant/ADR context instead of
  reconstructing it by grep.

### Breaking changes
- The config-toggle commands are renamed `awf add`/`awf remove` →
  `awf enable`/`awf disable` (ADR-0093), with no backward-compat alias. The
  verb now matches the operation (toggling an artifact's membership in the
  config enable arrays) instead of implying it creates or deletes something
  (which `awf new` does). Kinds, flags (`--dry-run`, `--with-dependents`), the
  closure/dependent behavior, and `awf list` are unchanged. An adopter's
  rendered `AGENTS.md` and docs switch to the new verbs on their next
  `awf sync`.

### Bug fixes
- `awf <cmd>` now rejects a repeated single-value flag (e.g. `awf audit --base a
  --base b`) with a usage error, instead of silently taking the last value.
  Repeatable flags like `awf init --set` are unaffected.
- `awf enable`/`awf disable` now reject a nameless singleton given a name (e.g.
  `awf enable bootstrap foo`) with a usage error, instead of silently ignoring
  the extra argument.
- `awf enable <kind>`/`awf disable <kind>` with the kind but no name (e.g. `awf
  enable target`) now say "requires a name" instead of the misleading "requires
  a kind" hint that treated the kind as if it were a name.
- `awf help new <kind>` (e.g. `awf help new adr`) now prints the subcommand's
  help; previously only `awf new adr --help` reached it and `awf help new adr`
  printed the group help.

### Others
- CLI dispatch is restructured onto a declarative command table
  (`internal/clispec`) driven by a generic parse-once dispatcher (ADR-0094):
  one path parses arguments, applies the gating classification, and calls the
  handler, replacing the hand-rolled per-command `switch`. `awf new <kind>
  --help` now prints kind-specific help. The resolver's internal
  `Add`/`Remove` vocabulary is renamed to `Enable`/`Disable`, completing
  ADR-0093's deferred rename. The rendered `AGENTS.md` binary-version-gate line
  and the gated-command list in the docs are now generated from the command
  table, so they list every gated command (adding `config`/`context`) and
  cannot drift.

## [0.15.0] - 2026-07-11

### Features
- Project-local custom docs (ADR-0091): `awf new doc <name> "<description>"`
  scaffolds a managed doc (a declaring sidecar plus a `content` convention
  part rendered from awf's base doc template) that joins the AGENTS.md
  document map and the dead-link check like any catalog doc. Names may be
  nested (e.g. `guides/ci`). A new toggleable `releasing` catalog doc
  (`awf add doc releasing`) ships a stub-default release runbook that imposes
  no structure.

### Bug fixes
- `awf sync` now fails loudly when two artifacts resolve to the same output
  path, so a path-aware local doc name colliding with awf's reserved
  `decisions/`, `plans/`, or `domains/` output is caught rather than silently
  overwriting the other file.
- `docs/config-reference.md` (and `awf config`) now document a project-local
  artifact's base data keys when a *synthesized* local skill, agent, or doc is
  enabled: the case where the base template actually renders those keys.
  Previously, for skills and agents the `_base` rows surfaced only for a
  hand-authored `local: true` opt-out (which renders nothing from the base
  template) and never for a `awf new`-created artifact, and for docs they never
  surfaced at all, so a real custom artifact's keys went undocumented.

## [0.14.1] - 2026-07-10

### Bug fixes
- The invariant-backing scan no longer descends into nested checkouts: a
  subdirectory carrying its own `.git` entry (a directory in a primary clone,
  a gitdir-pointer file in a linked worktree or submodule) is another
  repository's working tree, so a marker inside it can no longer silently
  keep this project's invariant "backed": previously a stale session
  worktree under `.claude/worktrees/` could preserve a deleted marker and
  hide an unbacked invariant from `awf check`.

## [0.14.0] - 2026-07-10

### Breaking changes
- The glossary doc is data-driven (ADR-0089): terms live in
  `.awf/docs/glossary.yaml` under `data.terms` as a `term: meaning` YAML map,
  and awf renders the table always sorted (case-insensitive), with `|`
  escaped in cells and content violations (empty terms or meanings, interior
  newlines, non-string values, case-insensitive duplicate terms) failing the
  render with the offending key named. The old `terms` section is gone: an
  authored `.awf/docs/parts/glossary/terms.md` part flags as orphaned drift
  after upgrading: move each table row into `data.terms` and delete the part.
  Framing prose goes in the new empty-by-default `prepend`/`append` sections.
  With no terms configured, the doc renders a placeholder line naming the
  authoring surface.

### Features
- `awf config [<key-or-var>]` (ADR-0088): print the configuration reference
  from the CLI: the full reference or a single entry, with live state inside
  a project (current values, consumers, dormant hints) and a static
  catalog-wide fallback outside one for pre-adoption discovery.
- `docs/config-reference.md`: a generated, always-on configuration reference
  (ADR-0088): every config key, var, sidecar field, and per-artifact data key
  with full descriptions, defaults, availability, and the project's live state
  (which vars are set/empty/absent, what consumes them, what enabling would
  activate). Regeneration-checked like the domain docs; the intro section is
  overridable, the generated tables are not, and `data:` on its sidecar
  refuses at open.
- Deleting a `vars:` key now acknowledges its unset-var note (ADR-0087): the
  advisory fires only for a key that is present with an empty (or null) value
  (the seeded open-to-do state), and an absent key is read as "considered and
  declined", permanently silencing the note for that var. The note text names
  both exits ("set a value, or delete the key to accept the generic prose").
  Deleting a key changes the referenced-var config hash, so expect a one-time
  stale flag until the next `awf sync`; and a var consumed by a part's
  `{{=awf:gateCmd}}`-style placeholder still hard-errors when deleted (the
  placeholder contract is unchanged). Rendering is untouched: absent, null,
  and empty all degrade to the same generic prose as before.

### Bug fixes
- `awf audit` (and every git-reading path) now works from a linked git
  worktree or submodule checkout, where `.git` is a `gitdir:` pointer file
  rather than a directory: the repo open resolves the pointer and routes
  shared state (objects, refs, config) through the worktree's `commondir`.
  Previously it failed with `open repo: ... .git/config: not a directory`.

### Others
- The repository now carries a committed example adopter (`examples/sundial/`):
  a full-surface worked example of an awf adoption, browsable in the repo and
  kept render-synced from awf's source by the repo's own checks (its ADR-0090).
- Dependency refresh: `golang.org/x/crypto` v0.51.0 → v0.53.0 (clears 13
  published SSH-package advisories: none reachable from awf, which only
  reads local git history), plus `x/mod` and `x/tools` to their
  current-minus-cooldown versions.

## [0.13.0] - 2026-07-10

### Breaking changes
- The `.awf/` tree is now closed (ADR-0086): `awf check` fails on any file or
  directory it cannot claim (strays like `.awf/notes.md`, files with the wrong
  extension in kind/parts dirs, parts of a `local: true` artifact) with a
  repair hint per entry, collapsing to the topmost unclaimed directory.
  Sync-written `<path>.awf-bak[.N]` collision backups are flagged as stale
  backups to review and delete (a brownfield adopt is therefore red on its
  first check until the backups are cleared; intended to-do surfacing).
  `.awf/memory/` stays exempt session scratch.
- `awf check` now fails on authored-but-unconsumed configuration (ADR-0086): a
  non-empty `vars:` key no rendered artifact references (`unused-var`), and a
  sidecar `data:` key the artifact's template never reads (`unused-data`): the
  typo that publication-safe degradation used to hide. Empty vars stay legal
  (the init scaffold is unchanged), but note that leftover keys from removed
  catalog vars (e.g. ADR-0084's) are now flagged when non-empty, and disabling
  a render unit (`awf remove hooks`) can strand the var only it consumed;
  delete the key in the same change.
- Inert sidecar fields now refuse at project open (ADR-0086): `paths:` on a
  non-domain sidecar, and anything but `paths:` on a domain sidecar (`data:`,
  `sections:`, `local: true`), fail every gated command with the exact file
  and fix named. These fields were silently ignored before; delete them (or
  move `paths:` to a domain sidecar) and re-run.
- The four prose-knob catalog vars (`docCurrencyTargets`, `adrProposeCommitFmt`,
  `gateDuration`, `modulePrefix`) are removed (ADR-0084): catalog vars now carry
  functional values only (commands, enforced identifiers, structural paths).
  The consuming templates render their former fallback prose unconditionally, so
  a project that set one of these sees the affected skill rewritten to the
  generic wording on its next `awf sync`: no warning is emitted; override the
  section with a convention part to restore concrete wording. Leftover keys in
  `vars:` are inert and can be deleted at leisure, but a saved init answers file
  (or `--set`) carrying a removed key now fails `awf init` on a fresh scaffold
  with an unknown-answer-key error.

### Features
- Interactive `awf init` now asks for the skill/doc selection first and then
  prompts only for the vars that selection's templates (plus the always-on
  singletons and hook payloads) actually reference (ADR-0086); every other
  catalog var is seeded empty as before. `--set`/answers-file values are
  honored for any var either way.
- Single-command upgrades: the bootstrap singleton now renders `.awf/upgrade.sh`
  alongside `.awf/bootstrap.sh` (ADR-0085). `bash .awf/upgrade.sh` resolves the
  newest release (or takes an exact version argument), fetches and verifies it
  through the bootstrap, and hands off to `awf upgrade`, closing the
  chicken-and-egg where every upgrade started with a manual binary fetch. The
  bootstrap itself now honors a pre-set `AWF_VERSION` environment override for
  which release to fetch; without one it resolves its pin exactly as before.
  `docs/working-with-awf.md` gains an "Upgrading awf" section covering the flow.
  (This upgrade is the bridging one: the script only exists in your tree after
  upgrading to a release that ships it; use
  `AWF_VERSION=<new> bash .awf/bootstrap.sh`, then `<printed path> upgrade`.)

### Bug fixes
- `awf upgrade` now always ends in a sync, even when no schema migration
  applies (ADR-0085): a same-schema binary bump re-renders every managed file
  and re-pins the bootstrap. Previously a template-only release left the
  rendered output stale until the next unrelated sync. The no-op message is
  now `config already at schema N`, followed by normal sync output.

### Others
- `awf sync` (and every command ending in a sync) now prints one provenance
  line per file whose rendered output actually changed, classifying the cause
  from the lock's hashes: `changed <path> (template)` for upstream template
  churn, `(config)` when your own vars/sidecars/parts caused it,
  `(template+config)` for both, `(internal)`/`(regenerated)` for non-hashed
  inputs (the pinned binary version; the generated decision indexes), and
  `added <path>` for newly shipped files: the triage signal for reviewing a
  large upgrade diff. A byte-identical re-render stays silent, and a first
  sync into a fresh project reports nothing.
- The rendered `docs/workflow.md` local-hooks section now documents the
  stub-as-override-point pattern: hook payloads are deliberately
  all-or-nothing, and a project-specific deviation (e.g. a docs-only fast
  path) belongs in the stub you own, commented as a deliberate deviation,
  keeping the payload canonical and sync-updated.

## [0.12.0] - 2026-07-09

### Breaking changes
- The catalog `requires*` declarations are now an enforced dependency graph
  (schema 8; run `awf upgrade`). A config enabling an artifact without its
  required skills/agents/docs is refused by every command; the migration
  closes your enabled set (adding missing requirements, printing each) and
  drops dormant doc-gated skills (enabled while their doc was disabled;
  they rendered nothing before, so your output is unchanged). `awf add`
  now enables the full requirement closure in one edit, printing a plan;
  `awf remove` refuses while enabled artifacts still require the target;
  `--with-dependents` removes them together, `--dry-run` previews either
  plan. `awf init` follows the same rule: a trimmed selection is
  closure-completed (missing requirements added, each printed) and the
  scaffolded agent set derives from the trimmed skills instead of always
  enabling every agent. The render-time suppression of doc-gated skills
  is gone: enabled now always means rendered.

### Others
- `awf check` and `awf init` now print a non-failing note when a convention
  part contains a whole line that is (or begins with) a section marker
  (`<!-- awf:section ... -->` / `<!-- awf:end -->`) which is inert inside a
  part and previously rendered into output silently. Inline quoting and
  fenced code examples never trigger the note; fencing is the remedy the
  note itself suggests.
- `awf sync` (and every command that ends in a sync: `upgrade`, `init`,
  `add`, `remove`, `new`) now prints `awf sync: pruned <path>` for each
  file its prune actually removes: a disabled artifact, a dropped
  target's tree, or a path relocated across versions no longer disappears
  silently. A routine re-sync still prints nothing.
- `awf upgrade` migrations now print one provenance line per config
  operation: the schema-6 migration reports each relocated sidecar/parts
  directory and each doc it strips from `docs:`, and the schema-7 migration
  reports each glob it anchors, matching the schema-8 migration's existing
  per-op lines, so an upgrade's config changes are readable from the output
  instead of the diff.
- Shipped templates no longer cite awf's own decision records: the agent
  guide's commit-scope bullet, the working-with-awf command overview, and
  the bootstrap comments drop their `ADR-NNNN` citations, and the
  working-with-awf glob examples switch from awf's repo layout to a neutral
  `src/` project. A source-level scan now bans concrete ADR citations and
  unexempted repo-identity literals in every template, all branches included.
- The bootstrap script's unsupported-OS/arch failure now points at the
  manual-install path (`https://github.com/hypnotox/agentic-workflows#install`),
  so Windows/git-bash users see the way forward instead of a bare error.
- The catalog now declares each skill's and agent's unconditional chain-skill
  coupling (`requiresSkills`), and the standard's test suite enforces the
  declarations both ways (undeclared reference and stale declaration each
  fail). Data only: no CLI or rendering behavior changes.

## [0.11.0] - 2026-07-08
### Breaking changes
- One anchored path-glob dialect everywhere (ADR-0077, schema 7; run `awf upgrade`). Every
  glob (`invariants.sources[].globs`, `audit.dependencyManifests`, and the new domain
  `paths`) now matches a file's full slash-separated repo-relative path: `*.go` means
  top-level `.go` files only, any-depth is written `**/*.go`, and path patterns like `cmd/**`
  or `internal/audit/*.go` are now legal. The migration rewrites existing no-slash patterns
  to `**/<pattern>`, so migrated configs behave exactly as before.
- A present-but-unreadable `.awf/awf.lock` is now a hard error in every command (ADR-0076),
  with one recovery hint: restore the lock from version control, or delete it deliberately
  to re-adopt. Previously an unparseable lock silently skipped the version sub-check
  (ADR-0039 Decision 5, partially superseded), read as schema-current to `awf upgrade`,
  and made `awf sync` treat every rendered file as foreign. A *missing* lock keeps its
  existing semantics everywhere.

### Features
- Domain territories and the `domain-code-staleness` audit rule (ADR-0077): a domain sidecar
  `.awf/domains/<name>.yaml` may declare the domain's file territory as anchored path globs
  under `paths:`; when a branch changes matching files without refreshing
  `.awf/domains/parts/<name>/current-state.md`, `awf audit` raises an advisory Warning,
  closing the ADR-less half of the domain-doc currency gap (ADR-0019 covers the ADR-driven
  half). Opt-in per domain; disable via `audit.domainCodeStaleness: false`.
- Trust-bearing writes are atomic (ADR-0076): `.awf/awf.lock` and migration rewrites of an
  existing `.awf/config.yaml` go through a same-directory temp-file-plus-rename helper, so
  an interrupted process can no longer leave a truncated lock or config.
- The agent guide's working-memory check is now on-demand (ADR-0075). The rendered guidance
  no longer tells the agent to check `.awf/memory/` on *every* start of work; instead it reads
  memory when the request implies earlier work to continue, or as a safety net when a fresh or
  context-compacted session finds `.awf/memory/` non-empty and unaccounted-for, and skips the
  check for a self-contained request. The resume-discipline (match → resume; ambiguous → ask;
  never silently resume a stale effort) is unchanged. Partial-item supersedence of ADR-0069
  Decision item 5; ADR-0069 stays Implemented.
- Review agents are now report-only (ADR-0074): the three reviewer subagents
  (`adr-reviewer`, `plan-reviewer`, `code-reviewer`) emit findings and a digest but no longer
  edit, commit, or re-review. The `<prefix>-reviewing-adr`/`-plan`/`-plan-resync`/`-impl`
  skills now own fix application, routing findings by classification (mechanical directly /
  reasoned with a one-line rationale / user-decision escalated), and run exactly one fresh
  verify-pass dispatch instead of the retired agent-side 3-round soft cap. Restores reviewer
  independence (a judge that never edits what it judged) and makes fix application visible on
  the main thread. Backed by the `reviewers-report-only` invariant.
- Convention parts can re-inject their section's own rendered default via the new
  `{{=awf:sectionDefault}}` sandbox placeholder (ADR-0072). Placing it in a convention part
  splices the overridden section's rendered default at that point, so a part can *extend* a
  shipped default (preamble, appendix, or wrap) instead of copying and forking it (which
  silently rots when awf revises the default). A part still replaces its section body; the
  placeholder just carries the default forward. Re-injecting a `stub` section's default (an
  authoring prompt) is a hard render error. Documented in the working-with-awf overrides
  section and placeholder key table.

### Others
- The rendered working-with-awf doc's command list now covers `awf uninstall` and
  `awf version`, and its `sectionDefault` key description states the stub re-injection
  failure mode precisely (a hard render error, not a silent skip).
- The rendered ADR-README's `supersedes:` example now models a bare int (`[1]`) instead
  of a zero-padded one (`[0001]`), which YAML-1.1 parsers read as octal.
- The two plan-execution skills' terminal-handoff line now attributes finding
  classification to the report-only review agent (ADR-0074): the reviewing skill routes
  findings by the agent's classification rather than "classifies" them itself.
- The plan-resync skill's verify-pass step now states which rule wins when the single
  verify pass surfaces an ADR-implicating residual: the amend-and-re-review return edge
  applies to initial-dispatch findings only, so verify-pass residuals escalate as
  `user-decision` items instead of re-entering the loop.

### Bug fixes
- A corrupt lock can no longer trigger `awf sync`'s backup storm: sync refuses before
  rendering or writing anything, so no spurious `.awf-bak` files are created and pruning is
  never silently skipped (ADR-0076). `awf check` and `awf uninstall` report a corrupt lock
  truthfully instead of "no lock"; `awf init` reports the lock error instead of listing
  every rendered path as a collision.
- `awf upgrade` no longer prints "already current" when the binary is behind the tree's
  schema (it gives the version-gate guidance) or when run outside any project; any
  project-requiring command that finds no `.awf/config.yaml` now says
  "not an awf project (run `awf init`)" instead of a raw file-not-found error (ADR-0076).
- ACTIVE.md and domain-index generation now group every ADR whose status carries the
  lifecycle convention's suffixed form (`Superseded by ADR-NNNN`) under one `Superseded`
  section, ordered by the status ranking. Previously the suffixed status never matched the
  bare `Superseded` ranking entry, so each successor minted its own alphabetical section.
  Entry lines keep the full status, so the successor stays visible per ADR.

## [0.10.0] - 2026-07-07
### Breaking changes
- The canonical workflow chain gains a terminal `retrospective` step, and the `reviewing-impl`
  skill now names `<prefix>-retrospective` unconditionally (ADR-0067). An existing project must
  enable the new Core skill after upgrading (`awf add skill retrospective`) or the next
  `awf check` fails with a dead skill reference from `reviewing-impl`.
### Features
- New `retrospective` chain skill (ADR-0067): a main-thread terminal step after `reviewing-impl`
  that reflects on the finished effort and routes recurring, codifiable findings up a four-rung
  promotion ladder: project invariant, gate test/lint rule, code-reviewer focus item,
  pitfalls entry. First-occurrence observations are noted rather than promoted, and promotion
  is never delegated or auto-applied unverified.
- Project-local skills and agents (ADR-0068): a project may enable skill/agent names outside
  the standard catalog by declaring a sidecar (`.awf/skills/<name>.yaml` /
  `.awf/agents/<name>.yaml`) and authoring a single `content` convention part; awf renders the
  artifact from an awf-owned base template per kind, with `{{=awf:key}}` placeholders available
  and publication-safe degradation for unset values. `awf new skill|agent <name> "<description>"`
  scaffolds the sidecar, starter part, and enable entry in one step; `awf list` shows local
  artifacts alongside their state. Local names may not shadow catalog names, and `local: true`
  keeps its existing meaning (fully hand-authored file, no rendering).
- Working-memory convention for chain session continuity (ADR-0069): `awf sync` now always
  renders a self-ignoring `.awf/memory/.gitignore`; the agent guide gains a working-memory
  section (per-effort `.awf/memory/<effort-slug>.md` files, resume protocol, JIT-retrieval
  guidance); brainstorming checkpoints its design brief continuously; the chain skills plus
  bugfix/debugging checkpoint phase/handoff state; the retrospective deletes the file.
- Must-replace template defaults are now declared with a `stub` attribute on their section
  marker, and `awf new`'s starter parts open with a whole-line `<!-- awf:stub -->` marker.
  `awf check` and `awf init` print a non-failing note per artifact with unauthored stub
  content; a stub section's rendered pointer reads `stub; replace by creating <path>`
  (ADR-0070). Upgrading re-renders every artifact whose template was swept; expect one large
  `awf sync` commit.
- A malformed `awf:section`/`awf:end` marker is now a hard render error instead of leaking
  verbatim into rendered output (ADR-0070).
- Plans must be phase-standalone: the writing-plans skill and the plans README now require every
  phase's closing commit to pass the project's gate on its own (each definition lands in the
  phase that first uses it), and the plan reviewer's executability lens checks the same rule.
### Bug fixes
- `awf check` now reports an enabled artifact whose output file was never synced (the drift scan
  previously iterated only lock entries, so an artifact enabled by hand-editing
  `.awf/config.yaml` was invisible until the next sync), and flags orphaned singleton convention
  parts under `.awf/parts/`: a typo'd section name or unknown kind directory was silently
  ignored.
- The sync prune and `awf uninstall` now skip lock entries that are not local relative paths; a
  corrupted or malicious `.awf/awf.lock` entry could previously delete a file outside the repo
  and then hang walking parent directories.
- `awf upgrade` no longer loops unrecoverably on a lockless pre-relocation (`.claude/awf/`)
  tree: generation detection anchors to the relocation migration instead of drifting upward as
  newer migrations register.
- `awf add`/`awf remove` enforce the binary-version gate before rewriting `.awf/config.yaml`; a
  stale binary previously failed only inside the chained sync, leaving a half-mutated config.
- `awf audit` fixes: a merge commit no longer attributes the merged-in branch's whole diff to
  the branch under audit; unparseable ADR frontmatter is surfaced as an `adr-frontmatter`
  finding instead of silently disabling the status-cochange rule; and the commit-subject length
  limit counts characters, not bytes.
- Dead-link check fixes: a badge-wrapped link (`[![CI](ci.svg)](docs/x.md)`) now has its outer
  destination checked; an angle-bracket target containing spaces (`[spec](<my file.md>)`)
  unwraps before checking; root-relative `/docs/...` targets resolve against the repo root; and a
  target escaping the repo root is dead by definition instead of depending on host contents.
- Invariant backing now requires the marker comment to open its line (after indentation), so a
  marker-shaped string inside a literal (e.g. a test fixture) no longer silently backs a slug;
  the rendered tagging guidance states the own-line contract.
- `awf init` with an existing config no longer walks through interactive prompts it then
  discards (it says it is keeping the config and flags ignored `--set`/`--answers` values),
  and unknown answer keys or out-of-options enum values are rejected instead of silently
  no-op'ing.
- Replayed migrations on a degraded lock no longer strip a modern `hooks:` mapping or overwrite
  an explicit bootstrap opt-out with the upgrade default.
- `awf new` refuses to overwrite an existing local artifact's sidecar or content part; a
  declared-but-disabled name was previously reset to the scaffold stub without warning.
- Unset-var notes now report each base-shared local artifact independently and are labeled by
  artifact path; previously all local artifacts collapsed onto one note keyed by the shared
  base-template id.
- Three skill descriptions no longer render a hardcoded article before the skill prefix
  ("a awf ADR" for vowel-initial prefixes).
### Others
- The agent guide's task-skill sentence now derives from the catalog via a `Chain` flag on the
  ten progression nodes, so enabled non-chain skills (e.g. `refactor-coupling-audit`) appear in
  the rendered guide instead of a hand-enumerated list.

## [0.9.0] - 2026-07-05
### Bug fixes
- `awf check` / `awf sync` now reject an `invariants.sources` entry that carries a comment marker
  but no globs: such a source scans no files, and was previously accepted silently (ADR-0064
  follow-up).
### Others
- ADR-system invariant-tagging guidance (`docs/decisions/README.md`) now derives its comment
  marker from `invariants.sources` instead of a hardcoded `//`: the adr-readme template renders
  the glob→marker mapping (via `.invariantMarkers`, degrading to marker-agnostic prose when no
  sources are set), and editing `invariants.sources` reflags the guidance (ADR-0064). Two new
  override placeholders (`invariantMarkerSentence`, `invariantMarkerTable`) are documented in
  the working-with-awf placeholder table.
- `awf init` no longer prompts for `invariantsMarker` / `invariantsGlobs` or accepts
  `--set invariantsMarker=...`; configure `invariants.sources` in `.awf/config.yaml` directly. The
  out-of-box default is unchanged (both descriptors defaulted empty, seeding no invariants
  config), so only the interactive/`--set` seeding path is removed (ADR-0064).
- Internal: the standard's catalog moves from an embedded `catalog.yaml` parsed at runtime to a
  compile-time Go value (`catalog.Standard`), and the toggleable docs and always-on singletons
  merge into one `DocEntry` collection from which every projection derives, so adding a mandatory
  doc is a single entry instead of ~6 hand-edited sites (ADR-0060, ADR-0061). Rendered output is
  byte-identical; no adopter migration or schema change.
- The `AGENTS.md` document map now renders its mandatory-doc lines from the catalog rather than
  hardcoded template lines, so a new mandatory doc appears with no template edit (ADR-0062). The
  four mandatory lines reorder to alphabetical and drop their trailing periods: the only
  adopter-visible output change.

## [0.8.0] - 2026-07-05
### Features
- Granular, domain-aligned commit scopes: `audit.allowedScopes` expands from `[adr, awf, plans]`
  to eight domain-named scopes, and each entry may carry a `{name, meaning}` mapping so the scope
  taxonomy renders from config (ADR-0055, ADR-0056).
- Convention parts can splice awf-derived values via the `awf:`-namespaced placeholder syntax: a
  dynamic, non-empty-only registry (scope list/table/sentence, prefix, gate commands),
  hard-error guards, and a backslash escape for documenting the syntax (ADR-0057, ADR-0058).
- New mandatory `working-with-awf` usage doc rendered into every project: a post-adoption guide
  to the CLI, overrides, the placeholder registry, and the sync/check loop (ADR-0059).
### Others
- The agent guide's commit-scope prose and the `docs/workflow.md` taxonomy table now derive from
  `audit.allowedScopes`; editing scopes reflags them (ADR-0055, ADR-0057). The guide's
  `awf-setup` section now points at the new usage doc rather than carrying the whole reference.

## [0.7.0] - 2026-07-04
### Breaking changes
- The brainstorming skill's terminal-handoff section is renamed from `terminal-handoff` to
  `terminal-step` for uniform chain-handoff naming (ADR-0054). Its rendered prose is unchanged,
  but any override at `.awf/skills/parts/brainstorming/terminal-handoff.md` must be renamed to
  `terminal-step.md` to keep applying.
### Features
- Add a `Red flags` rationalization-guard section (a "Rationalization | Reality" table) to the
  `tdd`, `debugging`, `executing-plans`, and `subagent-driven-development` skills, each
  overridable via `.awf/skills/parts/<skill>/red-flags.md`.
### Others
- Add a deterministic golden-task eval suite (`internal/evals`) that renders every catalog skill
  and agent through a full `Project.Sync` and asserts cross-artifact workflow-chain seams:
  forward handoffs name their successor on an invocation-verb line, and the chain graph is
  connected and reachable from `brainstorming` (ADR-0053, ADR-0054). Test-only; no change to
  rendered output or CLI behavior.
- Enforce `skill-section-parity`: every catalog skill/agent template's `awf:section` markers must
  match its declared sections, so a section rename can no longer half-land with a blank override
  path (ADR-0054).

## [0.6.2] - 2026-07-03
### Others
- The code-review agent's universal correctness lens is now paradigm-neutral: "race conditions,
  missing locks" broadens to "concurrency hazards (data races, unsynchronised shared state)" and
  the storage-layer concurrency clause is dropped: a project with a storage layer re-adds those
  traps via the reviewer sidecar's project-focus data.
- Add a general `awf:include` template-partials directive (awf-owned embedded partials under
  `templates/partials/`, spliced before section parsing, with the drift hash computed over the
  expanded source so a partial edit still flags dependent artifacts stale) and use it to
  deduplicate the review-discipline spine shared by the three reviewer agents. An awf-internal
  change: rendered template output is byte-for-byte unchanged (ADR-0052).

## [0.6.1] - 2026-07-03
### Bug fixes
- Converting a managed skill or agent to `local: true` no longer deletes its file on the next
  sync. The prune step now preserves every declared local artifact's output path, so a
  managed→local conversion keeps the hand-authored file instead of breaking later syncs with
  "local skill file absent".

## [0.6.0] - 2026-07-03
### Breaking changes
- The three standard docs (`workflow`, `doc-standard`, `agents-md-standard`) are now mandatory
  always-on singletons instead of toggleable catalog docs; config schema migrates to
  generation 6 (ADR-0043). Run `awf upgrade` after updating.
- The rendered bootstrap moves off the repo root into the config tree at `.awf/bootstrap.sh`
  (ADR-0047); update any hook or CI reference to the old `awf-bootstrap.sh` path.
- The `commitScope` var is removed: commit scopes now live only in `audit.allowedScopes`, set
  at init via the comma-separated `commitScopes` answer, quoted by the reviewing skills from
  the same storage `awf commit-gate` enforces, and folded into the drift signal (ADR-0051).
  A leftover var entry is inert; set `audit.allowedScopes` and re-sync to keep the prose.
### Features
- Render three inert git-hook payload scripts (`pre-commit`/`commit-msg`/`pre-push`) under
  `.awf/hooks/` via a `hooks` singleton: enabled by default at init, toggled with
  `awf add/remove hooks`; awf still never touches git config (ADR-0048).
- Add `awf new adr`, scaffolding the next sequential ADR from the rendered template (ADR-0042).
- Add `awf changelog` with `--version`/`--since`/`--range` filters over an embedded changelog
  (ADR-0041).
- `awf add domain` scaffolds the domain's `current-state.md` convention part alongside the
  config edit.
- The rendered workflow doc gains gate-composition and CI-backstop sections.
- Every var/data interpolation degrades to coherent generic prose when unset (an empty
  `awf init` renders publication-safe output), and `awf check`/`awf init` print advisory
  notes for referenced-but-unset vars (ADR-0045).
- `awf check` fails on a rendered reference to a catalog skill outside the enabled set, and
  templates can read the enabled-skill set to conditionalize prose (ADR-0046).
- Reviewing skills and their reviewer agents are pair-validated: `awf add skill` enables the
  missing agent, `awf remove agent` refuses while an enabled skill requires it, and gated
  commands fail on an unpaired config (ADR-0050).
- `awf init` refuses collisions before asking a single prompt, prints unset-var notes and a
  next-steps block after rendering, and falls silent when stdin hits EOF instead of
  streaming the remaining prompts.
- Bare `awf list` shows all seven kinds (including targets, bootstrap, and hooks), and
  `awf help <command>` prints that command's help text.
- The rendered `AGENTS.md` Commands section shows a self-describing placeholder when no
  commands are configured and de-duplicates identical command values.
### Bug fixes
- Single-source the binary version on `project.Version` so the version gate, lock stamp, and
  bootstrap pin cannot disagree; the bootstrap prefers a matching local binary, prints only
  the binary path on stdout, and falls back to `shasum` where `sha256sum` is missing
  (ADR-0049).
- Anchor the rendered skill-reference scanner on a token boundary, so prose like
  `example-bootstrap.sh` no longer trips the dead-skill-reference check.
- Restore the ADR title heading dropped when a project overrides the ADR template's
  sections, and route the generated ACTIVE.md through the canonical generated-by banner.
- The `add`/`remove`/`list` command help now enumerates the `target` kind those commands
  already dispatch, and `awf init` help documents the `--answers` file schema (a flat
  key→value map; multiselect answers comma-joined).
### Others
- Sweep chain-prose seams, tool-specific vocabulary, and repo residue from the rendered
  templates; hook-command descriptor options no longer suggest unpinned `awf` invocations;
  the `domains` frontmatter guidance now scopes itself to projects with configured domains.
- The `adr-lifecycle` skill drops the `Proposed→Deferred`/`Proposed→Declined` commit templates
  (states outside the default 4-state lifecycle) and rewords deferral as a Context amendment on
  a still-Proposed ADR; `refactor-coupling-audit` aligns its scope-shrink rule with that
  amendment form, and `reviewing-plan-resync` gains the ADR-amendment return edge.

## [0.5.1] - 2026-07-01
### Bug fixes
- Fix `awf audit`/`awf check` failing to open a repository with `extensions.worktreeConfig` set (a
  flag `git worktree add` can leave behind) due to an upstream go-git bug; also make the internal
  `.git` path join portable across OSes.

## [0.5.0] - 2026-06-30
### Breaking changes
- Add a self-pinning `awf-bootstrap.sh` installer singleton (toggle with `awf add/remove
  bootstrap`), pinned to the exact rendering binary's version and checksum-verified before
  install; config schema migrates to generation 5 (ADR-0040). Run `awf upgrade` after updating.
### Features
- Add a binary-version compatibility gate: `sync`/`check`/`invariants`/`audit`/`list` now refuse
  to run when the awf binary is behind the project's schema generation or recorded release
  version (ADR-0039).

## [0.4.0] - 2026-06-29
### Breaking changes
- Multi-target rendering goes live: adapter artifacts (skills, agents) now render once per
  enabled adapter via a `targets` config array (default `[claude]`), replacing the implicit
  Claude-only output path (ADR-0037, ADR-0038). Run `awf upgrade` after updating.
### Features
- Add a Cursor adapter (`.cursor/skills`, `.cursor/agents`, no `CLAUDE.md`-style bridge: Cursor
  reads `AGENTS.md` natively); manage adapters via `awf add/remove/list target <name>` (ADR-0037).
- Skill and agent prose is now tool-agnostic (neutral vocabulary instead of Claude Code-specific
  terms), so it reads correctly under any adapter (ADR-0038).

## [0.3.1] - 2026-06-29
### Others
- Sharpen the rendered workflow doc's guidance to explicitly name `awf check` as the pre-commit
  drift guard your own gate must run, rather than vaguely "your check and gate commands".

## [0.3.0] - 2026-06-29
### Features
- Convention-part bodies now render as raw input (never template-interpolated), closing a class
  of accidental-`{{`-breakage bugs (ADR-0034).
- `awf sync` now backs up any foreign file it would otherwise overwrite to a free
  `<path>.awf-bak[.N]` sibling, so adopting awf into an existing repo no longer risks silently
  clobbering unrelated files (ADR-0035).
- Add `awf commit-gate`, the deterministic, blocking counterpart to `awf audit`'s advisory
  Conventional-Commits rule; wire it into your own `commit-msg` hook (ADR-0036).
- Add `--help`/`-h` support to every subcommand.

## [0.2.0] - 2026-06-29
### Breaking changes
- Remove the `hook` artifact kind entirely; config schema migrates to generation 4, and awf no
  longer installs or manages git hooks (ADR-0032). Run `awf upgrade` after updating.
### Features
- Add an invariant-retirement mechanism: a successor ADR can now formally retire a predecessor's
  invariant tags via `retires_invariants` (ADR-0031).
- Add an opt-in local-hooks section to the rendered workflow doc, describing how to wire your own
  hooks now that awf doesn't (ADR-0032).
- `awf audit` gains a rule flagging an ADR status change whose per-domain index wasn't
  regenerated in the same commit (ADR-0033).

## [0.1.0] - 2026-06-28
_Initial public release._
### Features
- `awf init`/`sync`/`check` render a `.awf/`-configured tree of skills, review agents, docs, and
  the `AGENTS.md` agent guide from embedded templates into a project, with drift detection
  against a schema-versioned lock.
- `awf add`/`remove`/`list` manage which skills, agents, docs, and domains are enabled.
- `awf audit` reports advisory workflow-conformance findings (Conventional Commits, ADR/index
  co-change, and more) over a branch's git history.
- `awf upgrade` migrates a project's `.awf/` config tree across schema versions.
- Ships as prebuilt cross-platform binaries (linux/darwin/windows × amd64/arm64), with
  `go install` as a source fallback.
