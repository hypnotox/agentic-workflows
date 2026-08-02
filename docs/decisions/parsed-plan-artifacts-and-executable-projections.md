---
format: current-state-v3
slug: parsed-plan-artifacts-and-executable-projections
status: Proposed
date: 2026-08-02
---
# ADR-parsed-plan-artifacts-and-executable-projections: Parsed plan artifacts and executable projections

## Context

Implementation plans are authored as structured Markdown but are not parsed as structured
artifacts. `internal/plan` currently reads frontmatter, ADR links, and commit fences. The
project check validates status, link resolution, and commit subjects, while phase shape,
task identity, execution mode, task fields, phase close, and end-state requirements remain
reviewer judgment re-derived from prose on every plan.

The plans are large enough that this matters. A fresh phase owner either reads hundreds of
lines or searches for a heading and risks omitting plan-scoped constraints. A bare task is
not an executable unit: it depends on the plan's Goal and non-goals, Architecture summary,
its phase execution mode and transaction fence, the phase close, the whole-plan completion
requirements, and Notes recording deviations discovered in earlier work.

ADR-0209 sanctions task fields for exactness, spikes, batches, affected paths, and
deterministic post-checks, and retires the authored `## File structure` snapshot. That
vocabulary gives a parser a stable structural target. It does not itself select which
historical plans use the target. The corpus contains both frontmatter-less plans and
frontmatter-bearing plans with several historical shapes. A date or filename cutoff would
make parser selection depend on an external sequence, repeating the routing model ADR-0206
retired for ADRs.

Checkbox tasks have also stopped representing state. The three plans dated 2026-08-01 carry
68 unchecked tasks and no checked task. The workflow already says task boundaries are not
transaction, review, dispatch, commit, or checkpoint boundaries; the phase-closing commit,
Notes, review, and final plan status are the actual completion record. A checkbox therefore
suggests tracking the workflow does not perform, while a Markdown heading gives a parser a
real task identity.

The existing package boundary is suitable. `internal/plan` owns plan parsing and plan
scaffolding. `internal/project` orchestrates that parser and maps validation results to
findings. Under the model-owner-renders rule, the plan package should also render a plan
projection; `cmd/awf` should retain only command argument handling, output routing, and exit
mapping. No second Markdown parser belongs in the command or project packages.

## Decision

1. Plans gain an intrinsic authored format selected by frontmatter. A structured plan
   declares exactly `format: plan-v1`. Marker absence selects the legacy path. An unknown,
   empty, duplicate, or malformed format is a plan-frontmatter error rather than a legacy
   plan. `awf new plan` and the rendered plan template emit `plan-v1`; existing plans are
   not retrofitted. Legacy plans retain the checks they receive today and skip every new
   phase/task structural rule below.

2. `internal/plan` owns one typed plan model and one parse of each plan. The model retains
   frontmatter, title, Goal, Architecture summary, ordered phases, ordered tasks, phase
   closes, Definition of done, optional Notes, ADR links, and commit fences. Structural
   validation and focused projection consume that model. During one `awf check` operation,
   the plan directory is parsed once and the resulting typed set is threaded to both
   blocking plan findings and advisory commit-scope notes, with no long-lived cache.
   `internal/project` does not reparse headings or fields; it converts typed validation
   results into stable findings.

3. A `plan-v1` document has this top-level order: frontmatter; `# Plan: <title>`; `## Goal`;
   `## Architecture summary`; one or more phases; `## Definition of done`; and optional
   `## Notes`. Goal and Architecture summary are nonempty opaque Markdown sections. Goal
   continues to state the outcome and its non-goals. `## File structure` and
   `## Verification` are not `plan-v1` sections. Definition of done is required and carries
   one or more plain bullet requirements that state concrete, observable whole-plan checks
   and end conditions. Per-phase commands remain in the phase close rather than being
   duplicated there.

4. A phase heading is `## Phase P: <title>`, where positive integer `P` is sequential from
   1. Directly beneath it, `**Execution mode: inline.**` or
   `**Execution mode: subagent-driven.**` declares the phase's transaction owner. A
   subagent-driven phase still owes the exact commands and expected terminal states for its
   clean and green starting baseline; the reviewer judges their substance while the parser
   validates the execution-mode declaration. Each phase contains one or more tasks followed
   by exactly one phase close.

5. A task heading is `### Task P.T: <title>`, where `P` equals the owning phase and positive
   integer `T` is sequential from 1 within that phase. Task checkboxes are not part of
   `plan-v1`. Fields are contiguous `<Field>: <value>` lines immediately below the heading
   and before prose. The recognized fields are `Kind`, `Latitude`, `Question`, `Paths`,
   `Representative`, `Edge`, and `Post-check`; an unknown or duplicate field is a structural
   error. Omitted `Kind` means an implementation task and omitted `Latitude` means
   qualifying form. The only authored values are `Kind: spike`, `Kind: batch`, and
   `Latitude: exact`.

6. The parser enforces the field relationships it can determine mechanically. A spike
   requires `Question`, has no prose body after its fields, forbids the batch-only fields,
   and makes Notes required; its Question is its complete task content. A batch requires
   `Paths`, `Representative`, `Edge`, and `Post-check`.
   `Paths` is also required whenever scope is ambiguous, but ambiguity and whether content
   belongs to ADR-0209's closed exactness categories remain reviewer judgments. A `Paths`
   value carrying wildcard or pathspec syntax requires `Post-check`. The parser validates
   those structural implications; it does not attempt to infer contract-bearing prose or
   execute a post-check during plan validation.

7. A phase ends with `### Phase close`, never a checkbox. It is the last child of the phase
   and carries the phase's staged-check, gate, and single closing-commit instructions. It
   contains exactly one non-ignored `commit` fence. Existing commit-fence validation keeps
   owning subject shape, type, scope, and length. The structural parser owns only the
   presence, uniqueness, and placement of the phase-close fence.

8. Conditional and optional tasks remain forbidden. A spike may precede other work in its
   phase but cannot constitute a phase alone, and work depending on its answer begins in a
   later phase. The answer is appended to Notes. Task headings are ordered instructions,
   not completion state; commits, Notes, review settlement, and the plan's Implemented
   status remain the durable execution record.

9. A new gated command has grammar `awf read plan <plan> <P[.T]>`. `read` is a command group
   and `plan` its child. `<plan>` is an exact filename or exact filename stem under the
   configured plans directory; traversal, a path outside that directory, fuzzy title
   matching, and partial-name matching are refused. `P` selects a whole phase and `P.T`
   selects one task. Selectors use the canonical positive integers from the headings. An
   absent or invalid plan or selector fails and lists the available exact plan or selector
   values as appropriate.

10. A read result is an executable closure, not the selected source lines. Every result is
    rendered in source order: frontmatter, the plan title, Goal and non-goals, Architecture
    summary, the owning phase heading and execution mode, the selected content, that phase's
    phase close, Definition of done, and Notes when present. A `P` selection includes every
    task in the phase. A `P.T` selection includes only that task before the phase close. The
    output does not include other phases or mutate the source plan.

11. `internal/plan` renders the projection from its typed model. The command layer parses
    the two arguments, asks the project service to resolve the plans directory and selected
    file, calls the plan projection API, writes the returned bytes, and maps typed errors to
    the CLI outcome. Neither `cmd/awf` nor `internal/project` owns Markdown rendering or a
    second representation of plan structure.

12. The plan-authoring and review surfaces move together: the plan template, plans README,
    writing-plans skill, and plan reviewer use the `plan-v1` marker, heading tasks, heading
    phase close, field vocabulary, and required Definition of done. Generated target
    renders and the sundial example follow from those sources. The authored `.awf/` sources
    behind the repository README command table and the working-with-awf guide document
    `awf read plan`, the legacy boundary, and executable-closure semantics in the same
    implementation. Every changed template remains publication-safe under missingkey=zero:
    empty project variables render coherent generic prose and no no-value token.

13. The new contracts are mechanically backed. Parser tests cover legacy absence, every
    valid structural form, order and numbering failures, field relationships, phase-close
    placement, and required Definition of done. Project tests prove structural failures
    become stable findings without changing legacy results. Command and plan-package tests
    prove exact plan resolution, selector errors, phase projection, task projection, and
    every member of the executable closure. Template parity tests pin the native authoring
    surfaces to the same grammar and exercise empty variables to prove missingkey=zero
    produces coherent prose with no no-value token.

## State changes

- update `adr-system/plan-artifacts:plan-frontmatter-validated`
- update `adr-system/plan-artifacts:plans-template-taxonomy`
- add `adr-system/plan-artifacts:plan-v1-structure-validated`
- add `adr-system/plan-artifacts:plan-executable-projection`
- add `tooling/cli:plan-read-command`
- update `rendering/workflow-skill-templates:phase-transaction-ownership`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/pi-workflows:pi-session-handoff-workflow`

## Consequences

A plan becomes one artifact with two consumers rather than prose that validation and readers
interpret independently. The parser earns its ceremony twice: `awf check` can reject broken
shape, and a phase owner can request a bounded projection without losing transaction and
plan-scoped context.

New plans gain stricter syntax. Heading levels, numbering, field placement, phase close, and
Definition of done are no longer stylistic choices. That cost is accepted because each
constraint supplies either stable identity, safe projection, or a mechanically checkable
contract. Narrative content inside Goal, Architecture summary, task bodies, Definition of
done bullets, and Notes remains Markdown rather than a new data language.

The marker creates two plan populations permanently. This is preferable to retrofitting more
than a hundred historical plans or routing by a date that later migrations would have to
preserve. The legacy parser remains intentionally narrow; new structure is available only
where the author opts into the intrinsic format, and scaffolding makes that opt-in the
default for future work.

Removing checkboxes means the rendered plan no longer looks like an interactive checklist.
That is deliberate. Current workflow state is phase commits and review settlement, not
mutable task marks, and the task heading is a stronger navigation and parsing anchor. The
Definition of done uses plain bullets for the same reason: its requirements are acceptance
contracts, not a second resident progress tracker.

Every task projection repeats plan-scoped material. Output is longer than the task alone,
but bounded and safe: Goal, Architecture summary, Definition of done, phase ownership,
phase close, and Notes are exactly the information whose omission could make an isolated
task execute incorrectly. Notes can grow, but silently omitting earlier deviations is the
more dangerous failure.

The reader accepts only repository-owned exact names and numeric selectors. This gives up
convenient fuzzy lookup and descriptive selectors, but keeps scripts deterministic and
makes an error teach the caller the accepted identities. A later output format can be added
as a separate decision; this command emits the authored Markdown closure only.

`plans-template-taxonomy` receives a second update after ADR-0209. ADR-0209 removes File
structure and makes Notes conditional for spikes; this record adds the intrinsic marker,
heading grammar, required Definition of done, and parsed legacy boundary. The operations
are separate because the records decide separate concerns, and their final provenance order
records that progression.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep checkbox tasks and parse their list items | Preserves a false completion affordance and gives tasks weaker structural identity than headings |
| Keep optional Verification | Its name describes an activity rather than the concrete end state, and optionality permits a parsed plan with no whole-plan acceptance contract |
| Route structured plans by date or filename cutoff | Repeats the brittle external parser-selection model retired for ADRs and makes historical routing depend on chronology |
| Retrofit every historical plan to plan-v1 | Large mechanical churn would imply stronger historical structure than authors actually supplied and would create review noise with no execution benefit |
| Project only the selected task lines | Omits plan scope, architecture, transaction ownership, closing verification, completion requirements, and recorded deviations |
| Put parsing or projection rendering in cmd/awf | Duplicates the plan model outside its owner and couples semantic behavior to one presentation adapter |
| Add separate validate and read parsers | Allows the two consumers to disagree on task identity and valid structure, defeating the reason to parse plans |
| Use descriptive phase and task selectors | Titles change and can collide; numeric heading identities are explicit, ordered, and cheap to report on failure |

## Status history

- 2026-08-02: Proposed
