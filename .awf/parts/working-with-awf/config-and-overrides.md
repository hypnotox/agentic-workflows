Rendering recognizes three repository-wide resident roots: `.awf/efforts`, `.awf/worktrees`, and the local `.awf/effort-archive`. The state protocol keeps active efforts and managed worktrees under their respective roots; one optional active-effort `scratch/` directory has an owned real boundary but opaque descendants. Creation never scaffolds or manages scratch. Finish preserves the complete resident at `.awf/effort-archive/<uuid>-<slug>`, where it releases the slug; list, show, and selection remain active-only. Rendering governs only each root's self-ignoring `.gitignore`; dynamic descendants are local state that render, drift checks, sweep, and uninstall preserve without recursing into. Archive descendants are ignored, opaque, unmanaged, non-authoritative, manually disposable, and still subject to backups or local disclosure. There is no archive inventory, restore, prune, analysis, or retention surface. Schema generation 42 introduces the archive root, so older projects must run `./awf upgrade` before effort commands proceed; upgrade's ordinary render publishes the marker and current lock. There is no standalone memory root. Schema generation 21 removes obsolete metrics and assignment residents during upgrade, and generation 22 resets protocol-1 effort records and standalone `.awf/memory/` content rather than migrating them. That reset is journaled with the lock replacement as its commit point, so it refuses beforehand, and changes nothing, while any legacy managed worktree path, registration, or branch remains.

Discovery creates no effort. When durable continuity materially helps, `effort-workflow` autonomously selects a faithful outcome, title, and canonical short slug, creates exactly one immutable slugged effort, reports its identity, and continues in its managed worktree; work without that need remains effort-free. An existing effort resumes under its fixed identity only inside its outcome, and a newly discovered outcome cannot silently reuse, rename, replace, or create beside it. A deliberate switch checkpoints a kept effort, or transfers necessary context and inspects safe cleanup or explicit intentional discard before ordinary archival finish of a discontinued effort. Its memory is `.awf/efforts/<slug>/memory.md`, with one user-managed writer. Repository authority outranks the checkpoint.

Full-replacement workflow, guide, checkpoint, or affected skill parts must re-derive autonomous creation and deliberate switching; default-template projection tests cannot inspect replacement prose.

New plan scaffolds carry `format: plan-v2`, sequential phase and task headings, one final Phase close per phase, task-scoped decisions, and required Definition of done outcomes. Marker-absent historical plans retain legacy checks and are not projected. `./awf read plan <plan> <P[.T]>` accepts an exact filename or stem and canonical positive numeric selector, then prints the source-ordered executable closure.

Plan execution selects `inline` or `subagent-driven` ownership independently per phase. One
commit-capable owner takes a complete subagent-driven phase from a clean green baseline through its
staged check, gate, and closing commit; the parent owns inline phases, integration, report-only
review settlement, and the settled-phase checkpoint. Optional batch helpers are sequential and
commit-disabled, receive path-disjoint subsets, and never own shared files or the closing commit. A
dirty stop is inventoried before the parent completes inline, restores and restarts the complete
phase, or transfers the complete revised phase with completed and remaining work plus recovery
verification. Heading-identified tasks, executable projections, and helper returns are not transaction or checkpoint boundaries, and a
blind task-level successor is forbidden. See this document's Model selection section for the full model-tier definitions. In Pi, omission uses the configured role default and an exact tier reference is supplied only for a deliberate override.

Core `effort-workflow` renders for both built-in targets and directs native persistent checkout or context tooling to the exact existing `.awf/worktrees/<slug>` checkout. Pi additionally derives `using-effort` and the `awf-effort` extension; non-Pi targets never receive or invoke them, claim activity, or create a parallel harness-owned worktree.

Pi association stays at repository root. `using_effort` directly attaches with an effort slug or detaches with `{detach:true}` and never changes CWD or transfers a conversation. Attached model calls receive the relative owned memory path and, when the fixed directory exists, the managed-worktree path. While associated, prefer the pathless memory tools for complete-document reads, exact Markdown-body edits, and separate `phase` or `next` updates with automatic timestamps; they are a convenience, not workflow authority, so direct file access and ordinary awf commands remain available. Activity and complete Remote Pi metadata are advisory only, never authority or a lock. Optional display suffix publication carries only the effort slug or null, never a routing input or composed name; restart begins detached and local lifecycle degradation is silent.

A convention part replaces only its section body. A declared Markdown structural heading is awf-owned, excluded from part replacement and in-place read-back, and disappears only when the complete section is dropped.

**Topic metadata.** A topic may declare nonempty anchored `paths:`, `applies: global`, or both. A global topic remains applicable repository-wide for context and markers; when it also declares paths, those selectors separately state bounded ownership within its owning domain. Path-only metadata retains its scoped applicability and ownership meaning, while global-only metadata retains global authority without path ownership. Schema generation 41 activates the combined form; upgrade advances its lock generation without rewriting valid path-only or global-only metadata.

**Domain paths.** A domain sidecar's `paths:` selectors bound current-state topic ownership within that domain; context and coverage use the same anchored glob dialect. Working-tree and staged loading reject empty, duplicate, or malformed selectors. Historical audit projection intentionally omits domain sidecars, and audit does not infer documentation freshness from changed domain paths.

**Reading generated source guidance.** The generated-by banner says that awf owns the rendered
file; it is not an editing instruction. An `awf:edit` pointer identifies the convention part that
owns one rendered section. An informational `awf:source` comment, when present immediately after
the banner, identifies the compact reader-facing authority for an otherwise opaque generated
document. It is neither an `awf:edit` pointer nor a read-back boundary, and it is not an exhaustive
dependency list. When `render.templateSourceRoot` is configured, maintainer-facing
`awf:template-source` comments follow the banner and any `awf:source` marker to identify the
checked-in Markdown root template, included partials, and structural sections. They do not replace
reader guidance or edit ownership; frontmatter, native-format outputs, template-less producers, and
in-place editable bodies are not annotated. The optional value is a normalized repository-relative
directory and every emitted source must resolve in the selected working or staged tree; ordinary
adopters omit it. For a topic page, edit both `.awf/topics/metadata/<domain>/<topic>.yaml` and
`.awf/topics/parts/<domain>/<topic>/current-state.md`; a topic index instead names the family globs
`.awf/topics/metadata/<domain>/*.yaml` and `.awf/topics/parts/<domain>/*/current-state.md`. The
output plan remains the authority for machine render dependencies; `.awf/awf.lock` remains the
drift authority. **Pitfalls.** Run `./awf new pitfall "<Title>"` to create one canonical authored
source under `.awf/docs/pitfalls/` and report its repository-relative path without rendering. Edit
that source, then run `./awf render`. The compact generated `docs/pitfalls.md` index points to generated
`docs/pitfalls/<slug>.md` leaves; read them but never edit them. Deleting an authored source retires
its entry through ordinary render pruning. **Source map.** Section-overridable catalog docs and
`AGENTS.md` use `awf:edit`. Topic pages name their metadata-and-claim-part pair, while indexes and
domain navigation name the family globs. Glossary names its sidecar and
`derived:awf-standard-vocabulary`; the pitfall index names `.awf/docs/pitfalls/*.md`, while each
pitfall leaf names its exact authored source. The ADR index names `derived:authored-adr-corpus`; config
reference names `derived:configspec` and `derived:project-configuration`; target bridges name
`AGENTS.md`. Authored ADRs and plans are banner-free. Edit the authority, run `./awf render`, run
`./awf check`, and commit regenerated outputs and lock together.
