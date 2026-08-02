These packages read git history, build immutable tree snapshots, and audit workflow conformance over commit ranges. From schema generation 31, audit replays the shared cleaned-message authorization parser and exact incoming-parent qualification for committed merges, while pre-epoch merges and non-merges remain outside that replay. The claims below capture the current audit and snapshot contracts.

## Claims

### `invariant: audit-history-operation-owned`

One `awf audit` invocation collects its requested commit range exactly once and owns one immutable historical operation for that range. Transition replay and stale-merge replay share the operation's revision states and cached load errors, each required revision is derived at most once, and no cache survives the invocation or lives on Project.
Origin: ADR-revision-aware-historical-audit-pipeline
Backing: test

### `invariant: audit-history-policy-projection`

Historical transition and stale-merge replay derive only committed configuration and schema boundaries, ADR records and source bytes, and topic definitions and claims. Marker indexes, test-glob backing, coverage paths, and domain ownership sidecars stay owned by repository and staged checks; malformed bytes exclusive to those omitted projections do not create historical findings or failures. An in-range revision proven not to change this authority reuses its first-parent state, with merge relevance derived separately from first-parent paths and ambiguous evidence forcing a reload.
Origin: ADR-revision-aware-historical-audit-pipeline
Backing: test

### `invariant: audit-cancellation-propagates`

When range collection, committed evidence, revision derivation, transition replay, stale-merge replay, or the live cleanliness read returns context cancellation or deadline expiry, `awf audit` aborts and preserves that error identity. It never converts context termination into a finding, retries the canceled derivation, or continues to later audit work; non-context transition load failures remain advisory.
Origin: ADR-revision-aware-historical-audit-pipeline
Backing: test

### `invariant: audit-adr-status-cochange`

awf audit raises an Error finding when a range commit adds a current-state-v1 ADR or changes its status without also changing `docs/decisions/INDEX.md`, and raises none when the same change co-changes the index. Legacy-format ADR transitions are outside this rule.
Origin: ADR-0017
Revised-by: ADR-0137
Backing: test

### `invariant: audit-conventional-commits`

awf audit raises an Error finding for a range commit whose subject is not a well-formed Conventional Commit, carries a type or scope outside the configured allow lists, or exceeds the configured subject length limit; a conforming commit raises none.
Origin: ADR-0017
Backing: test

### `invariant: audit-dependency-warn`

awf audit raises a Warning finding, never an Error, when a dependency-manifest file changed somewhere on the branch but no ADR file changed on the branch.
Origin: ADR-0017
Backing: test

### `invariant: audit-domain-code-staleness`

The domain-code-staleness rule emits a Warning for a domain exactly when it is configured with non-empty sidecar paths, an in-range commit changed a non-generated file matching those patterns, and no in-range commit changed that domain's current-state part; it is silent when the part is co-changed, when only generated paths matched, when the domain declares no paths, and when the rule is disabled.
Origin: ADR-0077
Backing: test

### `invariant: audit-domain-doc-staleness`

The domain-doc-staleness audit rule emits one branch-level warning for a configured domain when an in-range commit brings an ADR tagging that domain to Implemented status without any in-range commit changing that domain's `.awf/domains/parts/<domain>/current-state.md`; it stays silent when the narrative is co-changed, when the status only reaches Accepted or Proposed, when the domain is unconfigured, and when the ADR carries no domains.
Origin: ADR-0019
Backing: test

### `invariant: audit-empty-range-clean`

awf audit over a branch with no commits beyond its base yields zero findings.
Origin: ADR-0017
Backing: test

### `invariant: audit-plain-punctuation`

With audit.plainPunctuation enabled, awf audit emits a Warning for each commit in which a non-generated markdown file under docsDir has a rising banned-codepoint count, naming the file and the risen codepoints in sorted order, and emits nothing when the count is unchanged or falls, when the path is generated, when the file lies outside docsDir, or when the knob is false.
Origin: ADR-0117
Backing: test

### `invariant: audit-plan-threshold-warn`

awf audit raises a Warning finding when the branch-aggregate count of non-generated changed lines exceeds the configured threshold but no file under the plans directory was touched.
Origin: ADR-0017
Backing: test

### `invariant: audit-uncommitted-changes`

When the uncommitted-changes rule is enabled and the working tree is dirty, audit emits a single Error finding whose detail tallies the tracked-change and untracked-file counts; when the rule is disabled it emits no finding even on a dirty tree.
Origin: ADR-0025
Backing: test

### `invariant: audit-undocumented-domain`

The undocumented-domain audit rule emits one branch-level warning for a domain when an in-range commit adds or changes an ADR whose domains list names a domain absent from the configured domain set; it stays silent for configured domains and for ADRs carrying no domains.
Origin: ADR-0019
Backing: test

### `invariant: commit-gate-shared-rule`

The Conventional Commits subject check is defined once as a single shared function. Both the audit range loop and the check staged commit command call that function, so neither re-implements the subject regex, the allowed type and scope lists, or the subject-length limit, and a subject the audit rejects is rejected identically by the commit gate.
Origin: ADR-0036
Revised-by: ADR-0159, ADR-0210
Backing: test

### `invariant: repo-audit-error-exit`

The repo-local audit tool exits non-zero only when it reports at least one error finding. An infrastructure failure such as a failing merge-base lookup produces an error finding and exit code 1, while warning-only and clean runs exit zero.
Origin: ADR-0073
Backing: test

### `invariant: stale-merge-trailer-replay`

For a merge whose result tree is at or after the intrinsic-format schema generation, `awf audit` derives the merge-time current authoring format from the shared activation registry and replays the shared cleaned-message parser and incoming-parent qualification against the result, first parent, and every incoming parent. It reports an Error for malformed reserved trailers or an older-format import lacking its complete authorization pair, while pre-epoch merges, historical non-merges, valid or redundant pairs, and true fast-forwards produce no stale-merge authorization finding.
Origin: ADR-0206
Backing: test
