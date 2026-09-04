The evals package holds the golden-task evaluation suite for the workflow artifacts. The claims below capture the current evaluation contracts.

## Claims

### `invariant: evals-full-catalog-coverage`

The golden-task fixture's AWF skill set is derived from loading the catalog over the embedded template filesystem and includes every catalog skill, so a test fails if any catalog skill is absent from the fixture.
Backing: test
