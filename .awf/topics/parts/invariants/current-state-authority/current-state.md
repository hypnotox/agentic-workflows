The currentstate package is the active authority engine: it loads topic claims and ADRs from a tree, checks their static and transition consistency, and reports invariant obligations. The claims below capture the current authority contracts.

## Claims

### `invariant: production-packages-domain-owned`

Every ordinary production package path is owned by a configured domain and applicable current-state topic, with the focused resolver exposing absence without enforcing it.
Origin: ADR-delegate-relevance-discovery-to-codegraph
Backing: test

### `invariant: abandoned-remove-pair-attributed`

An applied V2 remove continues to attribute claim absence after its ADR becomes Abandoned, while a remaining operation canceled by abandonment never attributes absence; snapshot-pair validation proves the Applied event and actual removal together.
Origin: ADR-0139
Revised-by: ADR-0143
Backing: test

### `invariant: accepted-authority-is-pending-only`

An Accepted ADR appears as a pending implementation instruction and never replaces a current claim before its state-impact transaction completes.
Origin: ADR-0133
Revised-by: ADR-delegate-relevance-discovery-to-codegraph
Backing: unbacked
Verify: In a fixture with a conflicting current claim and an Accepted ADR, focused authority reads keep the claim under current authority and place the ADR only under pending changes.
### `invariant: accepted-does-not-override-current`

Accepted operations appear only as bounded pending instructions in focused authority reads and do not replace current claim output.
Origin: ADR-0135
Revised-by: ADR-delegate-relevance-discovery-to-codegraph
Backing: unbacked
Verify: A fixture with an Accepted update conflicting with its current claim keeps both clearly separated with the claim remaining current.
### `invariant: merge-transition-ordered-aggregate`

Application and re-application batches remain in ascending ADR-identity and intra-ADR history order, while operation positions inside one Applied or Reapplied event are unordered membership and create no chronology. An authored transition preserves the prior Status history as an exact prefix, accepts any appended events that replay as a legal ordered lifecycle, and may append any number of batches across distinct claim IDs, but one claim may be the target of only one operation occurrence so every authored occurrence has an observable before and after. Recorded merge provenance alone permits the pair to fold a claim's operations into a legal ordered chain of at most one leading add, any number of updates, at most one remove, and after the remove any number of dominated updates. Repeated updates from one ADR contribute that updater once and require a material endpoint; repeated adds by their originating ADR fold into the chain's first absent-to-present net add; a canceling update endpoint is refused. Aggregate validation checks the observable ordered net effect without inventing intermediate claim bytes because per-occurrence materiality was proven by the authored transactions. A newly introduced ADR in an older intrinsic format is provisional at the staged boundary that lacks merge-parent and message evidence; every other derivable transition check remains blocking, and definitive admission requires exact incoming-parent qualification at commit-msg.
Origin: ADR-0182
Revised-by: ADR-0191, ADR-0206, ADR-0212, ADR-0229, ADR-0233
Backing: test

### `invariant: current-state-sole-active-authority`

Focused authority reads and invariant enforcement consume current-state topic claims, not Implemented historical ADR prose.
Origin: ADR-0133
Revised-by: ADR-delegate-relevance-discovery-to-codegraph
Backing: unbacked
Verify: In a fixture with claim provenance, one topic-declared invariant, and one ADR-only legacy invariant, awf read topic emits the active claim but no historical ADR, the invariant checker treats only the topic declaration as an active obligation, and awf read topic <claim-id> --history emits the provenance ADR.
### `invariant: currentstate-handshake-findings-unranked`

A current-state claim-handshake finding carries no rank: every provenance and transition finding the current-state checker produces is blocking, and the check path reports each by message with no severity field. The ranked coverage and fan-out findings the project report also carries are a separate concern and keep their ranks.
Origin: ADR-0183
Backing: test

### `invariant: historical-rationale-is-explicit`

Historical rationale stays reachable from active claim provenance without appearing in normal focused authority output.
Origin: ADR-0133
Revised-by: ADR-delegate-relevance-discovery-to-codegraph
Backing: unbacked
Verify: Over the same fixture, normal focused authority output omits Implemented provenance while awf read topic <claim-id> --history follows it.
### `invariant: implemented-impact-bidirectional`

Every applied governed state operation has its required current, removed, or dominated-history result, and every active claim Origin or revision has the inverse applied ADR operation; Remaining, Canceled, and dominated operations provide no authority.
Origin: ADR-0135
Revised-by: ADR-0143, ADR-0191
Backing: test

### `invariant: invariants-zero-slugs-clean`

When a project declares no invariant claims, loading the claim corpus succeeds and `awf check repo state` reports no backing findings or error.
Origin: ADR-0008
Revised-by: ADR-0210
Backing: test

### `invariant: removed-claim-id-not-reused`

Once any applied remove records a qualified claim ID, no later add may reuse it, regardless of whether the removing ADR later ends Implemented or partially Abandoned.
Origin: ADR-0135
Revised-by: ADR-0143
Backing: test

### `invariant: state-impact-transition-atomic`

Every newly appended application batch and exactly its matching claim mutations occur in one HEAD-to-index transaction, where a dominated operation's required mutation set is empty; staged validation refuses an operation record or mutation split across snapshot pairs.
Origin: ADR-0135
Revised-by: ADR-0143, ADR-0191
Backing: test

### `invariant: provenance-ordered-by-adr-number`

A claim's provenance order is ascending final ADR number: the canonical chain is its Origin ADR followed by its Revised-by ADRs sorted ascending and duplicate-free, every Revised-by entry is greater than the Origin's number, claim history output sorts revision records the same way, and no status-history event carries a state sequence. An `ADR-<slug>` entry naming a pending record is legal and resolves only against a pending record: it is placed after every numeric entry, because a pending record takes the corpus's next numbers at integration, and slug entries compare in authored list order among themselves. When the Origin is itself a slug entry, the greater-than-Origin comparison is deferred to numbering, which the numbering command's add-before-revise refusal guarantees.
Origin: ADR-0191
Revised-by: ADR-0202
Backing: test

### `invariant: applied-remove-absorbing-tombstone`

An applied remove is an absorbing tombstone: the qualified id is currently absent from the moment the remove applies, a concurrently developed update that integrates after the remove is retained as dominated history with no current effect and an empty required mutation set, and update-then-remove and remove-then-dominated-update integration orders converge to the same attributed absence.
Origin: ADR-0191
Backing: test

### `invariant: uncovered-lists-unowned`

The current-state coverage report lists present working-tree paths, tracked or untracked, that match no configured domain glob and are not recorded as managed outputs in the lock. It collapses each result to the topmost ancestor directory with no owned descendant in scope; owned and lock-listed paths never appear, and `contextIgnore` does not affect the census.
Origin: ADR-delegate-relevance-discovery-to-codegraph
Backing: test

### `invariant: update-requires-substance`

An update preserves Origin and prior revision history, carries its ADR once at its canonical ascending position, and changes a canonical claim field other than formatting or provenance alone. The once rule is satisfied by adding the ADR for its first application and by preserving that existing entry for a corrective re-application. Across a merge, where intermediate claim states exist only in authored commits and not in either compared universe, substance is evaluated on the net operation-chain endpoint; every authored application and re-application proves its own materiality.
Origin: ADR-0135
Revised-by: ADR-0182, ADR-0191, ADR-0212
Backing: unbacked
Verify: Staged fixtures with Origin edits, revision deletion, duplication, or reordering, whitespace-only, provenance-only, first substantive update, and repeated substantive correction accept only the prefix-preserving materially changed cases with one canonical ADR entry.
