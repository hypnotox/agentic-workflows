## Test layout

Package unit tests are Go `_test.go` files in `internal/<pkg>`, in that package's own test
package (`package <pkg>` or the black-box `package <pkg>_test` where a test needs no access to
unexported identifiers). Template golden tests — render assertions against the embedded catalog
— live in `internal/project/spine_test.go`. CLI integration tests drive the `awf` binary's
command functions directly (not a subprocess) against a temp directory built with `t.TempDir()`,
in `cmd/awf/*_test.go`.

Shared test-fixture building — project-config scaffolding, ADR frontmatter fixtures,
file-writing primitives, the seam-swap idiom, and git-repo fixtures — goes through
`internal/testsupport` (and its `gitfixture` subpackage), a leaf package with no dependency on
any other `internal/*` awf package (ADR-0044). New test code needing one of these idioms calls
into `internal/testsupport` rather than hand-rolling a local copy.
