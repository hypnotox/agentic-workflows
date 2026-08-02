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

awf check fails on present-but-malformed plan frontmatter. Exact `format: plan-v1` selects structured parsing; marker absence selects legacy parsing; and an empty, unknown, duplicate, or malformed format is a frontmatter error rather than a legacy plan. Both paths retain the Proposed and Implemented status enum.
Origin: ADR-0098
Revised-by: ADR-0213
Backing: test

### `invariant: plan-new-unnumbered`

awf new plan scaffolds docs/plans/YYYY-MM-DD-<slug>.md from the plans template with today's date and no sequential number, and refuses to overwrite an existing file.
Origin: ADR-0098
Backing: test

### `invariant: plans-template-taxonomy`

The rendered plans template emits `format: plan-v1`, date, adrs, and status frontmatter; `# Plan:`; nonempty Goal and Architecture summary; sequential heading-identified phases and tasks with one execution mode and one final Phase close per phase; required Definition of done plain bullets; and optional Notes. File structure, Verification, and task checkboxes are not plan-v1 sections or task declarations. Marker-absent historical plans remain on the legacy taxonomy.
Origin: ADR-0098
Revised-by: ADR-0211, ADR-0213
Backing: test

### `invariant: plan-v1-structure-validated`

A `plan-v1` document has nonempty Goal and Architecture summary sections, one or more sequential `## Phase P:` headings, one exact execution-mode declaration per phase, one or more sequential `### Task P.T:` headings followed by exactly one final `### Phase close`, a required Definition of done with one or more plain bullets, and optional Notes. Task fields are contiguous recognized `<Field>: <value>` lines directly below the heading; duplicate or unknown fields fail. Spike and batch relationships, JSON-array Paths entries, literal/glob/pathspec confinement, post-check triggers, phase-close placement, and its single commit fence are mechanically enforced. Ambiguous scope, contract-bearing exactness, baseline substance, and post-check execution remain reviewer or executor judgments. Marker-absent plans skip these structural rules.
Origin: ADR-0213
Backing: test

### `invariant: plan-executable-projection`

internal/plan owns exact filename-or-stem resolution, canonical positive `P` and `P.T` selectors, and projection rendering from the typed plan model. A phase projection contains every task in that phase; a task projection contains only that task; and both contain frontmatter, title, Goal, Architecture summary, owning phase and execution mode, Phase close, Definition of done, and Notes when present in source order. Errors list available exact names or selectors as appropriate. Projection includes no other phase, reparses no Markdown outside the model owner, and never mutates source bytes.
Origin: ADR-0213
Backing: test
