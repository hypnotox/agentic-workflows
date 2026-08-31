The currentstate package loads current-state topic authority from repository snapshots and reports claim-backing, coverage, and ownership obligations independently of historical decisions.

## Claims

### `invariant: current-state-adr-independent`

The live topic corpus and focused topic queries load claims, references, backing, proof sites, selectors, coverage, and fan-out without loading a decision corpus.
Backing: test

### `invariant: current-state-sole-active-authority`

Focused authority reads and invariant enforcement consume current-state topic claims rather than historical decision prose.
Backing: unbacked
Verify: In a fixture containing both a topic claim and unrelated historical decision prose, `awf read topic` emits only the topic authority and `awf check repo state` evaluates only its backing and coverage obligations.

### `invariant: invariants-zero-slugs-clean`

When a project declares no invariant claims, loading the claim corpus succeeds and `awf check repo state` reports no backing findings or error.
Backing: test

### `invariant: uncovered-lists-unowned`

The current-state coverage report lists present working-tree paths, tracked or untracked, that match no configured domain glob and are not recorded as managed outputs in the lock. It collapses each result to the topmost ancestor directory with no owned descendant in scope; owned and lock-listed paths never appear, and no configurable exclusion can hide a census result.
Backing: test

### `invariant: domain-owned-coverage-no-ignore`

Every ordinary eligible domain-owned path participates in topic coverage without a configurable exclusion escape hatch; generated outputs and nested adopters retain their independent exclusions.
Backing: test
