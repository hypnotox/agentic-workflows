## Test layout

Package unit tests are Go `_test.go` files in `internal/<pkg>`, in that package's own test
package (`package <pkg>` or the black-box `package <pkg>_test` where a test needs no access to
unexported identifiers). Template golden tests — render assertions against the embedded catalog
— live in `internal/project/spine_test.go`. CLI integration tests drive the `awf` binary's
command functions directly (not a subprocess) against a temp directory built with `t.TempDir()`,
in `cmd/awf/*_test.go`.

Workflow-chain golden-task evals live in `internal/evals`, a test-only package (only `_test.go`
files, no production source). Each scenario runs a full `Project.Sync` over a fixture config derived
from the embedded catalog — every skill, agent, and doc enabled — and asserts *cross-artifact* seams a
single-template test cannot: that a skill's terminal handoff names a skill present in the same rendered
set, and that a reviewing skill's dispatched reviewer agent carries the shared review-spine partial. The
fixture's enabled set is catalog-derived so it cannot silently stop covering a newly-added chain
artifact (ADR-0053).

Shared test-fixture building — project-config scaffolding, ADR frontmatter fixtures,
file-writing primitives, the seam-swap idiom, and git-repo fixtures — goes through
`internal/testsupport` (and its `gitfixture` subpackage), a leaf package with no dependency on
any other `internal/*` awf package (ADR-0044). New test code needing one of these idioms calls
into `internal/testsupport` rather than hand-rolling a local copy.
