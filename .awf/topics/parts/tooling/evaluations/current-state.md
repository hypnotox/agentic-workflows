The evals package holds the golden-task evaluation suite for the workflow artifacts. The claims below capture the current evaluation contracts.

## Claims

### `invariant: evals-full-catalog-coverage`

The golden-task fixture's enabled skill and agent set is derived from loading the catalog over the embedded template filesystem and includes every catalog skill and agent, so a test fails if any catalog skill or agent is absent from the fixture's enabled set.
Origin: ADR-0053
Backing: test
