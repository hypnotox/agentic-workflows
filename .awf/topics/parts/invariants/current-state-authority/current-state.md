The currentstate package is the active authority engine: it loads topic claims and ADRs from a tree, checks their static and transition consistency, and reports invariant obligations. The claims below capture the current authority contracts.

## Claims

### `invariant: abandoned-remove-pair-attributed`

An applied V2 remove continues to attribute claim absence after its ADR becomes Abandoned, while a remaining operation canceled by abandonment never attributes absence; snapshot-pair validation proves the Applied event and actual removal together.
Origin: ADR-0139
Revised-by: ADR-0143
Backing: test

### `invariant: accepted-authority-is-pending-only`

An Accepted ADR appears as a pending implementation instruction and never replaces a current claim before its state-impact transaction completes.
Origin: ADR-0133
Backing: unbacked
Verify: In a fixture with a conflicting current claim and an Accepted ADR, awf context keeps the claim under current authority and places the ADR only under pending changes.

### `invariant: accepted-does-not-override-current`

Accepted operations appear only as bounded pending instructions and do not replace current claim output.
Origin: ADR-0135
Backing: unbacked
Verify: A fixture with an Accepted update conflicting with its current claim keeps both clearly separated with the claim remaining current.

### `invariant: merge-transition-ordered-aggregate`

A merge transition is validated as an ordered aggregate rather than one authoring step: several application batches are legal in ascending ADR-number and intra-ADR history order, a claim's operations across the pair must form a legal ordered chain of at most one leading add, any number of updates, at most one remove, and after the remove any number of dominated updates, and an appended Status history must preserve the prior history as an exact prefix. A non-merge transition keeps the stricter per-step contract of one new batch per ADR, one operation per claim, and the fixed status-event shape.
Origin: ADR-0182
Revised-by: ADR-0191
Backing: test

### `invariant: current-state-sole-active-authority`

Normal context retrieval and invariant enforcement consume current-state topic claims, not Implemented historical ADR prose.
Origin: ADR-0133
Backing: unbacked
Verify: In a fixture with claim provenance, one topic-declared invariant, and one ADR-only legacy invariant, awf context emits the active claim but no historical ADR, the invariant checker treats only the topic declaration as an active obligation, and awf topic <claim-id> --history emits the provenance ADR.

### `invariant: currentstate-handshake-findings-unranked`

A current-state claim-handshake finding carries no rank: every provenance and transition finding the current-state checker produces is blocking, and the check path reports each by message with no severity field. The ranked coverage and fan-out findings the project report also carries are a separate concern and keep their ranks.
Origin: ADR-0183
Backing: test

### `invariant: historical-rationale-is-explicit`

Historical rationale stays reachable from active claim provenance without appearing in normal path-context output.
Origin: ADR-0133
Backing: unbacked
Verify: Over the same fixture, normal context omits Implemented provenance while awf topic <claim-id> --history follows it.

### `invariant: implemented-impact-bidirectional`

Every applied governed state operation has its required current, removed, or dominated-history result, and every active claim Origin or revision has the inverse applied ADR operation; Remaining, Canceled, and dominated operations provide no authority.
Origin: ADR-0135
Revised-by: ADR-0143, ADR-0191
Backing: test

### `invariant: invariants-zero-slugs-clean`

When a project declares no invariant claims, loading the claim corpus succeeds and the invariant report is empty. There is no backing obligation to enforce, so the check passes with no findings and no error.
Origin: ADR-0008
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
Revised-by: ADR-0194
Backing: test

### `invariant: applied-remove-absorbing-tombstone`

An applied remove is an absorbing tombstone: the qualified id is currently absent from the moment the remove applies, a concurrently developed update that integrates after the remove is retained as dominated history with no current effect and an empty required mutation set, and update-then-remove and remove-then-dominated-update integration orders converge to the same attributed absence.
Origin: ADR-0191
Backing: test

### `invariant: uncovered-lists-unowned-unignored`

The current-state coverage report lists as unowned only working-tree paths that are tracked, not generated or lock-listed, not context-ignored, and matched by no configured domain glob, collapsed to the topmost ancestor directory that has no owned descendant in scope. Owned, generated, and ignored paths never appear.
Origin: ADR-0110
Backing: test

### `invariant: update-requires-substance`

An update preserves Origin and prior revision history, adds its ADR once at its canonical ascending position, and changes a canonical claim field other than formatting or provenance alone. Across a merge, where the intermediate claim states exist only in the branch's own commits and never in either compared universe, the substantive-change requirement is evaluated on the net effect of the claim's operation chain; per-step substance is enforced where it is verifiable, at the authored commits themselves.
Origin: ADR-0135
Revised-by: ADR-0182, ADR-0191
Backing: unbacked
Verify: Staged fixtures with Origin edits, revision deletion or reordering, whitespace-only, provenance-only, and substantive prose, reference, or backing changes satisfy an update only in the prefix-preserving substantive cases.
