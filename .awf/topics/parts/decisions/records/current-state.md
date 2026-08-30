Decision records preserve load-bearing choices that should outlive implementation. The claims below define the live repository boundary without making decision history current authority.

## Claims

### `rule: plain-append-only-decisions`

Existing numbered files are append-only historical Markdown and remain byte-stable. New records use an accepted `YYYY-MM-DD-<slug>.md` filename and contain Context, Decision, and Consequences sections. awf does not parse, index, scaffold, number, review, or lifecycle-manage decision records.

### `rule: current-state-independent`

A current-state mutation does not require a decision record. Current-state topics directly own implemented rules and invariants; decision records preserve rationale without determining what is currently true.
