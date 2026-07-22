The advisory audit surfaces: awf audit, repoaudit, and mutants reporting.

## Claims

### `invariant: audit-empty-range-announced`

When the resolved audit range contains zero commits, `awf audit` prints a distinct notice naming the range and stating that no history rule was evaluated, never the bare clean line, and still exits zero with zero findings.
Origin: ADR-0148
Backing: test

### `invariant: audit-reports-evaluated-scope`

Every `awf audit` run prints the resolved range and its commit count on its verdict, whether clean, warning, or error, so no verdict is emitted without the scope that produced it.
Origin: ADR-0148
Backing: test

### `invariant: audit-requires-explicit-range`

`awf audit` invoked with no positional range argument exits non-zero without evaluating any rule, and its refusal message names both the bare-base and the a..b accepted forms.
Origin: ADR-0148
Backing: test

### `invariant: audit-scopes-descriptor-routed`

A non-empty commit-scopes init answer is written to `audit.allowedScopes` (comma-split and trimmed), while an empty answer writes no audit block at all, leaving scopes accept-any.
Origin: ADR-0148
Backing: test

### `invariant: audit-warn-exit-zero`

The awf audit command returns success with a zero exit status when all of its findings are Warning severity, and returns a non-zero exit status when any single finding is Error severity.
Origin: ADR-0148
Backing: test

### `invariant: mutants-missing-report-errors`

The mutants command exits non-zero for a nonexistent report path and never prints no survived mutants for one, while a present-but-empty report file reports no survivors with exit 0.
Origin: ADR-0148
Backing: test

### `invariant: repoaudit-requires-explicit-range`

The repoaudit command invoked with no range argument exits non-zero with its usage line and evaluates no rule; a supplied bare base is also rejected because repoaudit does not opt into the parser's bare-base form.
Origin: ADR-0148
Backing: test
