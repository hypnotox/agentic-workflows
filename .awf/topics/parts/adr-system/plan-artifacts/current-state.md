The plan package parses and validates implementation plan documents. The claims below capture the current plan-artifact contracts.

## Claims

### `invariant: plan-adr-link-resolved`

awf check fails when a frontmatter-bearing plan's adrs: entry resolves to no record: a numeric entry against a docs/decisions/NNNN-*.md file, a slug entry against a pending record's file or a numbered record's retained slug key. Numbering never rewrites a plan. A plan with no frontmatter is skipped.
Origin: ADR-0098
Revised-by: ADR-0202
Backing: test

### `invariant: plan-commit-subject-length-checked`

awf check fails a plan under docs/plans/ that carries a validated commit fence whose first non-empty line exceeds the resolved audit.SubjectMaxLength, reporting the offending length and the limit.
Origin: ADR-0111
Backing: test

### `invariant: plan-commit-subject-marker-scoped`

A fenced block is read as a planned commit subject only when the first token of its info string is commit; a bare fence, any other first token, an empty or whitespace-only commit block, and template.md or README.md are never validated.
Origin: ADR-0111
Backing: test

### `invariant: plan-commit-subject-optout-honored`

A commit fence whose info string also carries the awf-ignore token is never validated, producing no drift and no note, so a display-only commit example is suppressed while keeping its first-token commit highlighting.
Origin: ADR-0111
Backing: test

### `invariant: plan-commit-subject-scope-advisory`

A validated commit fence subject naming a scope outside a non-empty audit.allowedScopes yields a non-failing awf check note rather than drift; the commit-message gate still treats an unknown scope as a hard finding.
Origin: ADR-0111
Backing: test

### `invariant: plan-commit-subject-shape-checked`

awf check fails a plan under docs/plans/ whose validated commit fence first non-empty line is malformed (not of the form type(scope)?: subject) or names a type outside a non-empty audit.allowedTypes.
Origin: ADR-0111
Backing: test

### `invariant: plan-frontmatter-validated`

awf check fails on present-but-malformed plan frontmatter. Exact `format: plan-v1` or `format: plan-v2` selects its structured parser; marker absence selects legacy parsing; and an empty, unknown, duplicate, or malformed format is a frontmatter error rather than a legacy plan. Both paths retain the Proposed and Implemented status enum.
Origin: ADR-0098
Revised-by: ADR-0213, ADR-task-scoped-plan-decision-context-and-phase-outcomes
Backing: test

### `invariant: plan-new-unnumbered`

awf new plan scaffolds docs/plans/YYYY-MM-DD-<slug>.md from the plans template with today's date and no sequential number, and refuses to overwrite an existing file.
Origin: ADR-0098
Backing: test

### `invariant: plans-template-taxonomy`

The rendered plans template emits `format: plan-v2`, date, adrs, and status frontmatter; `# Plan:`; nonempty Goal and Architecture summary; sequential heading-identified phases and tasks with one execution mode, optional ordered Advances then Completes arrays, and one final Phase close per phase; required unique slugged Definition-of-done bullets; and optional Notes. File structure, Verification, and task checkboxes are not plan-v2 sections or task declarations. Plan-v1 and marker-absent historical plans retain their compatibility taxonomies.
Origin: ADR-0098
Revised-by: ADR-0211, ADR-0213, ADR-task-scoped-plan-decision-context-and-phase-outcomes
Backing: test

### `invariant: plan-v1-structure-validated`

A `plan-v1` document has nonempty Goal and Architecture summary sections, one or more sequential `## Phase P:` headings, one exact execution-mode declaration per phase, one or more sequential `### Task P.T:` headings followed by exactly one final `### Phase close`, a required Definition of done with one or more plain bullets, and optional Notes. Task fields are contiguous recognized `<Field>: <value>` lines directly below the heading; duplicate or unknown fields fail. Spike and batch relationships, JSON-array Paths entries, literal/glob/pathspec confinement, post-check triggers, phase-close placement, and its single commit fence are mechanically enforced. Ambiguous scope, contract-bearing exactness, baseline substance, and post-check execution remain reviewer or executor judgments. Marker-absent plans skip these structural rules.
Origin: ADR-0213
Backing: test

### `invariant: plan-v2-decision-references`

Plan-v2 parses nonempty unique Applying and Context JSON references with a four-digit-or-retained-slug ADR identity, lowercase-kebab V4 selector, or canonical positive frozen pre-V4 `#N` selector; malformed Applying or Context field lookalikes, including a nonexact colon separator, block before task prose. The project resolves every plan-level ADR link through the corpus identity map, compares Applying membership by resolved record identity, permits Applying to amendable V4 ADRs, requires Context to be frozen, and retains frozen pre-V4 `#N` compatibility.
Origin: ADR-task-scoped-plan-decision-context-and-phase-outcomes
Backing: test

### `invariant: plan-v2-phase-outcomes`

Plan-v2 uses a fence-aware source-range parser: every top-level plain Definition-of-done bullet has a unique lowercase-kebab `dod:` marker, and each retained item preserves its complete authored multiline, nested, or fenced block byte-for-byte through the next item or section boundary. Optional nonempty Advances then Completes arrays are ordered. Unknown outcomes, duplicate Completes owners, and same-phase advance plus complete fail.
Origin: ADR-task-scoped-plan-decision-context-and-phase-outcomes
Backing: test

### `invariant: plan-v2-assignment-advisories`

Proposed plan-v2 records emit sorted non-failing assignment notes from their selected working or staged universe; Applying assignment is scoped independently per plan, so one plan cannot silence another plan's missing-Decision note. Implemented records emit none.
Origin: ADR-task-scoped-plan-decision-context-and-phase-outcomes
Backing: test

### `invariant: plan-executable-projection`

internal/plan owns exact filename-or-stem resolution, canonical positive `P` and `P.T` selectors, and projection rendering from typed plan-owned inputs; plan-v2 validates its selector before the project loads the ADR corpus, preserving its typed error and exact available selectors. Plan-v2 orders frontmatter/title, Goal, Architecture summary, Applying then Context Decisions, owning phase/execution mode, selected work, Phase close, its phase-owner-context advanced then completed outcomes, and Notes; it excludes whole-plan Definition of done, deduplicates resolved keys in first-authored order, and promotes Context to Applying. Plan-v1 bytes, including Definition of done, remain unchanged. Errors retain exact selector identities and available values. Projection includes no other phase, reparses no Markdown outside the model owner, and never mutates source bytes.
Origin: ADR-0213
Revised-by: ADR-task-scoped-plan-decision-context-and-phase-outcomes
Backing: test
