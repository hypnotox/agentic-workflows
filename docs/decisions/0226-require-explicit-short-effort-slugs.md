---
format: current-state-v4
slug: require-explicit-short-effort-slugs
status: Implemented
date: 2026-08-03
---
# ADR-0226: Require Explicit Short Effort Slugs


## Context

ADR-0175 replaced opaque UUID command identities with meaningful slugs derived from outcome titles.
The slug became the immutable public identity, resident-directory name, managed-worktree path, and
branch suffix. Its 63-byte limit keeps those representations safe, but derivation also makes a title
serve two different purposes: describing the outcome and choosing a compact operational handle.
Users must shorten an otherwise useful title merely to shorten every path and branch, while a title
near the accepted limit creates a needlessly long identity agents must repeat throughout the
workflow.

ADR-0222 later required the user to confirm `Outcome:` and `Effort title:` before first creation
because allocation fixes that identity. The confirmation still does not expose the derived slug it
implicitly approves. A caller cannot deliberately choose a short handle, and the command contract
cannot distinguish title validation from slug validation or give either value independent recovery
guidance.

Both records are terminal history. This decision changes the current-state claims they established
rather than editing them: creation will accept a required explicit slug, and first-creation
confirmation will expose that third immutable value. Existing schema-2 residents can legitimately
carry slugs through 63 bytes, including active efforts created before this change, so a shorter
creation policy must not make them unreadable or unfinishable.

The change crosses CLI grammar, effort creation, default managed-worktree orchestration, error
recovery, generated workflow guidance, Pi attachment validation, current architecture prose, and
every active signature reference. The persisted record and Git topology model do not need a new
schema: they already store and use a validated slug independently from the normalized title. The
current interspersed command parser already permits a declared value flag anywhere after the
selected command. A typed creation input is the bounded enabling change needed to carry two
semantically distinct strings without positional transposition or duplicated policy.

## Decision

1. `decision: require-explicit-slug` Change creation to
   `awf effort new --slug <slug> <outcome-title> [--json] [--no-worktree] [--base <ref>]`.
   `--slug` is a required, nonrepeatable value flag and may appear anywhere after `new`, including
   before or after the single outcome-title positional. Missing, duplicate, valueless, or unknown
   flags fail grammar validation before project composition or mutation. Remove implicit
   title-to-slug derivation and every creation path that omits the explicit slug.

2. `decision: separate-creation-input` Introduce an `internal/effort` creation input with named
   `Slug` and `Title` fields. The command boundary constructs it, default managed-worktree
   orchestration carries it, and the effort service validates and publishes it. Slug policy remains
   owned by `internal/effort`; neither the CLI nor the worktree layer duplicates it. Retain enough of
   the input or created record through rollback to render reconstructive retry commands with safely
   quoted independent slug and title arguments.

3. `decision: bound-new-slugs` Require newly created explicit slugs to contain 1 through 32 bytes,
   match `[a-z0-9]+(?:-[a-z0-9]+)*`, remain one confined path segment, and make
   `refs/heads/awf/<slug>` pass Git ref-format validation. Validate the title independently as a
   normalized nonblank valid UTF-8 value through 160 bytes. Refusals identify whether slug, title,
   or Git-ref validation failed, report no changed bytes, and give an explicit corrective action
   without title-derivation guidance.

4. `decision: preserve-resident-compatibility` Keep the existing 1-through-63-byte canonical slug
   validation for persisted residents and command selection. Listing, showing, activity attachment,
   memory updates, managed-worktree operations, and finishing therefore continue to accept valid
   preexisting slugs longer than the new-creation limit. Align Pi's independent effort-attachment
   validation to this 63-byte resident boundary rather than the 32-byte creation boundary. Add no
   migration, rename, truncation, suffixing, or lifecycle exception.

5. `decision: retain-record-and-topology` Keep schema-2 record and JSON shapes, UUID allocation,
   memory identity, resident paths, publication ordering, collision and tombstone behavior,
   managed-worktree paths and branches, default base selection, rollback classification, and finish
   semantics unchanged. The explicit slug becomes the same immutable value those mechanisms already
   consume; `--no-worktree`, `--base`, and JSON behavior retain their existing combinations and
   outcomes.

6. `decision: confirm-three-fields` Expand mandatory first-creation confirmation to present
   `Outcome:`, `Effort title:`, and `Effort slug:` and wait for a clear later user response before
   mutation. A requested change to any field produces a revised three-field proposal; ambiguity
   receives focused clarification. Once confirmed, generated guidance invokes
   `awf effort new --slug <confirmed-slug> "<confirmed-title>"`. Existing efforts resume under their
   fixed identity without reconfirmation only inside the already-confirmed outcome, and all other
   discovery, failure-retry, context-loss, minimal-fix, ownership, checkpoint, and handoff rules from
   ADR-0222 remain unchanged.

7. `decision: synchronize-active-signatures` Update the CLI specification, production diagnostics,
   current architecture and workflow documentation, authoring convention parts, templates, skills,
   agent guides, generated runtime outputs, and example adopter wherever they state or invoke the
   active creation signature. Render generated files from their owning sources. Preserve terminal
   ADRs, historical plans, and changelog history even where they quote the former command because
   those records describe past behavior rather than current instructions.

8. `decision: gate-signature-drift` Add deterministic coverage over a closed path policy to reject
   the former live signature, title-derived creation guidance, or two-field first-creation
   confirmation. Scan the authoring roots `cmd/`, `internal/`, `.awf/parts/`, `.awf/docs/`,
   `.awf/skills/`, `.awf/topics/`, and `templates/`; scan the rendered or current surfaces
   `AGENTS.md`, `README.md`, `docs/`, `.pi/`, `.claude/`, and `examples/`. Exclude only the historical
   roots `docs/decisions/`, `docs/plans/`, and `changelog/` from that policy instead of using a raw
   repository-wide textual ban; update current-behavior fixtures where applicable. Existing catalog
   projection and missing-key-zero tests continue to prove every enabled runtime and the example
   adopter.

9. `decision: prove-boundaries` Test 1-, 32-, and 33-byte new slugs, canonical grammar failures,
   independent Unicode titles, Git-ref validation, collision and publication behavior, and
   actionable recovery. Test missing, duplicate, before-title, and after-title `--slug` together
   with JSON, `--no-worktree`, and `--base`. Construct valid 33-through-63-byte resident fixtures and
   prove selection, listing, showing, Pi attachment, managed topology, and finishing remain usable.
   Prove supplied-slug worktree path and branch behavior, rollback reconstruction, three-field
   workflow confirmation, active-signature synchronization, rendering, drift checks, and the full
   project gate.

10. `decision: render-lifecycle-index` Run `./x render` for every transition of this record to
    Accepted, Implementing, Implemented, or Abandoned, and stage the regenerated
    `docs/decisions/INDEX.md` and lock output in the same commit.

## State changes

- update `tooling/effort-management:effort-record-authority`
- update `tooling/effort-management:default-worktree-creation`
- update `tooling/cli:effort-command-contract`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`

## Consequences

Outcome titles can describe work without becoming unwieldy path and branch names, while callers and
users deliberately choose and confirm the short immutable handle they will carry. Validation errors
become specific to the supplied title or slug. Named creation input keeps the distinction visible
through the CLI, persistence, worktree, and rollback path without changing the stored record or
introducing a second identity.

Creation becomes intentionally incompatible for old callers: every new effort command must add a
canonical slug of at most 32 bytes, and workflow interactions gain a third proposed field to review.
The explicit active-surface gate and generated projection tests make stale instructions visible.
Historical records remain textually accurate to their own time, so repository searches can still
find the former signature outside the gated active set.

Existing residents require two validation limits: 32 bytes when minting a new identity and 63 bytes
when reading or selecting an identity that may predate this decision. That distinction is deliberate
compatibility policy and must stay explicit in names, tests, diagnostics, and Pi attachment. It adds
a small amount of validation structure but avoids stranding active memory and Git topology or
requiring an unsafe rename migration.

The user can choose a slug unrelated in wording to the title. The confirmed three-field boundary,
canonical grammar, collision refusal, immutable storage, and one-effort ownership rule make that
choice visible rather than silently reconciling it. Awf does not enforce semantic similarity between
title and slug because such a rule would recreate derivation policy and require subjective language
judgment.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep deriving the slug from the title | It keeps descriptive prose coupled to operational path and branch length and does not expose the immutable value at confirmation. |
| Make `--slug` an optional override | Two creation modes preserve hidden derivation, complicate guidance and errors, and let callers omit the value this decision requires users to confirm. |
| Use a required second positional argument | Two adjacent free-form values are easier to transpose and make the exceptional immutable identity less visible than a named flag. |
| Pass slug and title as adjacent string parameters internally | The compiler cannot distinguish them, and rollback plus worktree orchestration would obscure which representation each argument owns. |
| Reduce all slug validation to 32 bytes | Existing valid residents and their memory or managed Git topology could become inaccessible before safe integration and cleanup. |
| Truncate, hash, or suffix a derived slug automatically | Silent transformation weakens explicit identity approval and introduces collision or reconstruction policy that a required short value avoids. |
| Require semantic similarity between slug and title | Language-dependent judgment is not deterministic and would recreate the coupling explicit identity is intended to remove. |

## Status history

- 2026-08-03: Proposed
- 2026-08-03: Implementing; content-sha256: 25528f6461e42c4879107bc8ab887e0ff84ec9e3c7b077166f69c7a961e6382a
- 2026-08-03: Applied; operations: update `tooling/effort-management:effort-record-authority`, update `tooling/effort-management:default-worktree-creation`, update `tooling/cli:effort-command-contract`, update `rendering/guide-and-doc-templates:working-memory-single-home`, update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- 2026-08-03: Applied; operations: update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- 2026-08-03: Implemented; content-sha256: 25528f6461e42c4879107bc8ab887e0ff84ec9e3c7b077166f69c7a961e6382a
