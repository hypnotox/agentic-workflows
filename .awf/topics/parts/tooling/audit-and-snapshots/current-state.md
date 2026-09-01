These packages read git history, build immutable tree snapshots, and audit retained repository conventions over commit ranges. The claims below capture the current audit and snapshot contracts.

## Claims

### `invariant: audit-advisories-always-run`

The plain-punctuation and uncommitted-changes rules always evaluate; plain-punctuation emits Warning findings only for rising punctuation-restraint violations, and uncommitted-changes emits an Error.
Backing: test

### `invariant: audit-thresholds-fixed`

Audit fixes the Conventional Commits subject limit at 72 and the accepted type set at build, chore, ci, docs, feat, fix, perf, refactor, revert, style, and test.
Backing: test

### `invariant: audit-history-operation-owned`

One `awf audit` invocation walks its requested commit range exactly once, feeds each commit through audit-owned rule accumulators, and retains only findings after the visitor returns. No cache survives the invocation or lives on Project.
Backing: unbacked
Verify: Inspect `internal/audit` range evaluation to confirm one invocation owns its accumulators and stores no state on Project or package globals.

### `invariant: audit-cancellation-propagates`

When range collection or the live cleanliness read returns context cancellation or deadline expiry, `awf audit` aborts and preserves that error identity. It never converts context termination into a finding or continues to later audit work.
Backing: unbacked
Verify: Inject cancellation at the range and live-cleanliness seams and confirm the returned error matches the injected context error and later work does not run.

### `invariant: audit-conventional-commits`

awf audit raises an Error finding for a range commit whose subject is not a well-formed Conventional Commit, carries a type outside the fixed Conventional Commits set or a scope outside the fixed historical scope vocabulary, or exceeds the fixed 72-character subject limit; a conforming commit raises none. Live `audit.allowedScopes` policy does not alter historical evaluation.
Backing: test

### `invariant: audit-empty-range-clean`

awf audit over a branch with no commits beyond its base yields zero findings.
Backing: test

### `invariant: audit-plain-punctuation`

awf audit compares old and new text for each non-generated Markdown file under the documentation root and emits a Warning when the en-dash count or total em-dash excess rises. Total em-dash excess sums each blank-line-delimited paragraph's count beyond two. The finding names the file and risen measures in sorted order; permitted ellipses, curly quotes, and restrained em dashes are silent, as are unchanged or falling measures, generated paths, and paths outside the documentation root.
Backing: test

### `invariant: audit-uncommitted-changes`

When the working tree is dirty, audit always emits a single Error finding whose detail tallies the tracked-change and untracked-file counts.
Backing: test

### `invariant: commit-gate-shared-rule`

The Conventional Commits subject check is defined once in the commit-message package. Audit and the command-local commit hook call that shared policy rather than reimplementing its grammar.
Backing: test

### `invariant: repo-audit-error-exit`

The repo-local audit tool exits non-zero only when it reports at least one error finding. An infrastructure failure such as a failing merge-base lookup produces an error finding and exit code 1, while warning-only and clean runs exit zero.
Backing: test
