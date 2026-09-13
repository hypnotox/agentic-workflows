# AWF: skills and author-owned agent guidance

Implementation plan · 13 September 2026 · Approved for implementation

Based on [AWF at 8b00a0f](https://github.com/hypnotox/agentic-workflows/tree/8b00a0f68f89312aa3368e912e242268f42a7def) and the agreed direction in this conversation. Apply the [working](https://github.com/hypnotox/agentic-doctrine/blob/db812439b8231971d77a010b9dfe7d2b3ae9e9e0/working-principles.md), [planning](https://github.com/hypnotox/agentic-doctrine/blob/db812439b8231971d77a010b9dfe7d2b3ae9e9e0/planning-principles.md), [coding](https://github.com/hypnotox/agentic-doctrine/blob/db812439b8231971d77a010b9dfe7d2b3ae9e9e0/coding-principles.md), [instruction](https://github.com/hypnotox/agentic-doctrine/blob/db812439b8231971d77a010b9dfe7d2b3ae9e9e0/instruction-principles.md), [artifact](https://github.com/hypnotox/agentic-doctrine/blob/db812439b8231971d77a010b9dfe7d2b3ae9e9e0/artifact-principles.md), and [review](https://github.com/hypnotox/agentic-doctrine/blob/db812439b8231971d77a010b9dfe7d2b3ae9e9e0/review-principles.md) principles. Adjust implementation mechanics as evidence changes while preserving the agreed outcome.

AWF should deliver its workflows through task-specific skills, keep repository agent instructions author-owned, and generate only fixed skills and supporting infrastructure. Remove `.awf/project.md` without replacing it with configuration. Its current verbatim projection is intentional; the change removes the need for that projection.

The implementation was approved after goal alignment. Apply the current principles in `../agentic-doctrine` during this work: clean up the instruction set as a coherent whole rather than merely redistributing prose. Doctrine remains an authoring basis, not an adopter runtime dependency. All projected content, including skill frontmatter, launch scripts, ignore rules, and version metadata, must come from actual embedded template/source files, not Go string literals. Go owns embedding, template execution, runtime values, and publishing/checking; canonical workflow Markdown remains shared with CLI pages.

**Agreed ownership**

| Content | Owner and behavior |
| --- | --- |
| `AGENTS.md` | Adopter-authored guidance; AWF neither generates it nor requires a particular Markdown structure. |
| `CLAUDE.md` | Optional adopter-authored file. `awf docs agents` explains the `@AGENTS.md` import for Claude support. |
| Four AWF skills | Generated at the existing Pi and Claude skill locations from canonical workflow content. |
| CLI guides | Canonical Markdown under `internal/docs/`, embedded in the binary. Workflow pages and skill bodies share their instruction source. |
| `awf`, `.awf/bootstrap.sh`, `.awf/.gitignore` | Fixed generated infrastructure. The bootstrap continues to select the pinned binary. |
| `.awf/VERSION` | Generated record of the renderer's version, checked for drift. It supplies no configuration or command prerequisites. |
| Topics and optional artifacts | Topics remain author-owned under `docs/topics/`. Existing `new` commands create author-owned files once; effort contents remain opaque local state. |

**Implementation route**

1. **Give each workflow one instruction source and a focused entrypoint.**

   Split the existing workflow guidance by when it is needed:

   | Skill / CLI page | Responsibility |
   | --- | --- |
   | `awf-topics` / `docs topics` | Discover, read, and maintain applicable project knowledge. |
   | `awf-effort` / `docs effort` | Continuity, memory, notes, worktree coordination, and handoffs. |
   | `awf-changes` / `docs changes` | Define substantial changes and use intent, specification, plan, and ADR documents where useful. |
   | `awf-completion` / `docs completion` | Compare results with agreements, commit coherent work, review retrospective findings, and perform applicable integration and cleanup. |

   Generate substantive skill bodies from the same Markdown served by the CLI. Keep applicability clear in skill descriptions; keep detailed conditional material within its owning workflow. Completing a small effort-free change must not require loading the full document or ADR lifecycle. Use the repository's documented AWF runner consistently.

   Preserve the established behavior: continuity and implementation worktrees use a coordinating effort; small self-contained work can remain effort-free; documents remain optional with distinct roles and authored relationships; memory owns the current checkpoint; ADR agreement and lifecycle semantics remain intact. Preserve commit defaults, including coherent intermediate commits, and completion retrospectives with or without an effort. CLI access remains available without native or external skills.

2. **Restore concise authoring guidance for adopters.**

   Add `awf docs agents`, backed by `internal/docs/agents.md`, and link it from integration guidance and CLI discovery. Adapt the [recovered authoring standard](https://github.com/hypnotox/agentic-workflows/blob/198139455fd72f992b382ed5c01e6c37911cd3aa/templates/docs/agents-md-standard.md.tmpl) to direct editing of `AGENTS.md`.

   Explain brief orientation, genuinely global requirements, essential commands, and useful canonical pointers. Keep procedures in skills and subsystem knowledge in topics. Include only necessary discovery cues, including continuity and completion; native skill descriptions own the skill catalog. Show a small illustrative example with optional headings and explain optional Claude support.

   Remove the obsolete project starter and generated guide template once their useful guidance has a home. Restore no fixed layout, byte budgets, or retired override machinery.

3. **Separate fixed generation from repository knowledge.**

   Remove project-descriptor loading, format metadata, and project-body composition. Construct fixed outputs from embedded AWF content. Topic resolution and coverage read topic sources directly; retain their existing selector and matching semantics. `check` continues to validate topics as well as generated output.

   Keep `init` as a thin first-install entrypoint to the same generation operation as `render`; it creates no agent guide or descriptor. Repeated installation follows ordinary render ownership rules. Remove AGENTS and Claude import ownership from the publisher.

   Add `.awf/VERSION` using the running binary's embedded version. Reserve that exact path as AWF-owned metadata so a later render can replace an older recorded value. Keep existing regular-file checks and generated ownership markers for the other outputs. Include the record in drift checks; neither its presence nor its value controls topic resolution, documentation, artifact creation, or binary selection.

   Reuse the current publisher and documentation mechanisms. Add no replacement schema, configurable output registry, or lifecycle engine.

4. **Make adoption and the ownership transition explicit.**

   Update integration guidance for both new adopters and existing installations. For existing repositories, reconcile `.awf/project.md` with `AGENTS.md`, including any unrendered source edits. Preserve useful adopter instructions, remove superseded shared workflow prose and the generated ownership marker, then retire the descriptor. Preserve useful Claude instructions; retained Claude files become adopter-owned.

   Cover the known old topic-location transition before retired format checks disappear. Recognized legacy descriptors or topic layouts should produce an actionable migration diagnostic rather than silently lose guidance. Keep conversion manual and bounded to known layouts.

   Inspect the new reserved version destination during adoption. Render and check with the target binary after the ownership transition, preserving tracked documents, effort memory, and worktrees.

   Apply this transition to AWF's own checkout, retaining its `./x` development runner and genuine project constraints. Update affected help, README, topics, migration guidance, and native release fixtures with their owning behavior. Regenerate the fixed outputs together.

5. **Verify the agreed behavior and review instruction delivery.**

   Adapt existing tests around observable contracts:

   - Fresh generation works without a descriptor or Git, preserves existing author-owned agent files, and does not create missing AGENTS or Claude files.
   - Both harnesses receive the four skills from the canonical workflows; CLI guides remain usable before installation. Check real content sharing and usable references.
   - Topic resolution, coverage, and invalid-selector reporting retain their behavior without project metadata.
   - Rendering is repeatable, version records update across renderer versions, and checks detect missing or stale outputs. Unrelated commands work without a current version record.
   - The documented ownership transition preserves authored and unrendered guidance. Known legacy topic layouts are not silently overlooked.

   Replace tests tied to retired projection contracts. Avoid exact prose, heading-count, and wording-only assertions. Review the resulting instructions against representative tasks: a small change, a resumed effort, change-document work, and integration. Check that each receives the necessary guidance without restoring a universal runbook.

   Run the existing repository gate, check, and regeneration-drift checks using `./x`; exercise the updated installation behavior through the existing native release fixture. Reuse valid evidence and report any unavailable verification.

Completion means the agreed ownership model works, the four workflows remain discoverable and proportionate, and existing adopters have a clear transition. Keep implementation, documentation, and verified commits coherent; report material retrospective findings and unresolved follow-ups.

