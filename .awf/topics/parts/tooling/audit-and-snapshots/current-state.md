These packages read git history, build immutable tree snapshots, and audit retained repository conventions over commit ranges. The claims below capture the current audit and snapshot contracts.

## Claims

### `invariant: audit-advisories-always-run`

The plain-punctuation and uncommitted-changes rules always evaluate; plain-punctuation emits Warning findings only for rising punctuation-restraint violations, and uncommitted-changes emits an Error.
Backing: test

### `invariant: audit-thresholds-fixed`

Audit fixes the Conventional Commits subject limit at 72, the plan diff threshold at 400, the accepted type set at build, chore, ci, docs, feat, fix, perf, refactor, revert, style, and test, and the dependency-manifest set at the nineteen built-in language-agnostic globs.
Backing: test

### `invariant: audit-history-operation-owned`

One `awf audit` invocation walks its requested commit range exactly once, feeds each commit through audit-owned rule accumulators, and retains only findings and compact graph metadata after the visitor returns. No cache survives the invocation or lives on Project.
Backing: unbacked
Verify: Inspect `internal/audit` operation construction and range walking to confirm one invocation owns all accumulators and caches and that no state is stored on Project or package globals.

### `invariant: audit-history-policy-projection`

Historical audit decoding reads only committed configuration, schema boundaries, decision records, and source bytes needed by retained audit rules. Marker indexes, test-glob backing, coverage paths, and domain ownership sidecars stay owned by repository and staged checks.
Backing: test

### `invariant: audit-cancellation-propagates`

When range collection, committed evidence, historical decoding, or the live cleanliness read returns context cancellation or deadline expiry, `awf audit` aborts and preserves that error identity. It never converts context termination into a finding or continues to later audit work.
Backing: unbacked
Verify: Inject cancellation at each audit-owned range, historical-read, and live-cleanliness seam and confirm the returned error matches the injected context error and later seams are not called.

### `invariant: sparse-snapshot-explicit-selection`

Historical audit enumerates committed path and mode metadata without reading blob contents, then reads only its exact authority selection into an immutable snapshot Selection that is type-distinct from a complete snapshot Tree. Selected reads fail on an unsafe, duplicate, missing, outside-project, or unsupported requested path; full-tree consumers cannot treat an unselected path as repository absence, and current and staged checks retain complete snapshots.
Backing: test

### `invariant: audit-adr-status-cochange`

awf audit raises an Error finding when a range commit adds a current-state-v1 ADR or changes its status without also changing `docs/decisions/INDEX.md`, and raises none when the same change co-changes the index. Legacy-format ADR transitions are outside this rule.
Backing: test

### `invariant: audit-conventional-commits`

awf audit raises an Error finding for a range commit whose subject is not a well-formed Conventional Commit, carries a type outside the fixed Conventional Commits set or a scope outside the configured scope list, or exceeds the fixed 72-character subject limit; a conforming commit raises none.
Backing: test

### `invariant: audit-dependency-warn`

awf audit raises a Warning finding, never an Error, when a dependency-manifest file changed somewhere on the branch but no ADR file changed on the branch.
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

### `rule: managed-history-decode-horizon`

Audit owns read-only decoding of managed schemas 3 through its explicit horizon 47, including represented pre-31 lock routing fields. A pre-.awf revision is empty; malformed, partial, or out-of-horizon authority refuses with recovery direction and is never promoted to live authority.
