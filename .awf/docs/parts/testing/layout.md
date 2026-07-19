## Test layout

Package unit tests are Go `_test.go` files in `internal/<pkg>`, in that package's own test
package (`package <pkg>` or the black-box `package <pkg>_test` where a test needs no access to
unexported identifiers). Template golden tests (render assertions against the embedded catalog)
live in `internal/project/spine_test.go`. CLI integration tests drive the `awf` binary's
command functions directly (not a subprocess) against a temp directory built with `t.TempDir()`,
in `cmd/awf/*_test.go`.

Workflow-chain golden-task evals live in `internal/evals`, a test-only package (only `_test.go`
files, no production source). Each scenario runs a full `Project.SyncReport` over a fixture config derived
from the embedded catalog (every skill, agent, and doc enabled) and asserts *cross-artifact* seams a
single-template test cannot: that a skill names its handoff successor on an *invocation-verb line* (a
real handoff instruction, not an incidental mention), that a reviewing skill dispatches a reviewer agent
carrying the shared review-spine partial, and that the forward-chain handoff graph is connected: no
orphaned node, every node reachable from `brainstorming` (ADR-0053, ADR-0054). The fixture's enabled set
is catalog-derived so it cannot silently stop covering a newly-added chain artifact. A companion
section-parity guard in `internal/project` (`TestSkillAndAgentSectionParity`) asserts every skill/agent
template's `awf:section` markers match its catalog-declared sections, so a section-slug rename
cannot half-land with a blank-path provenance pointer.
Three further catalog-derived guards live in `internal/project/catalog_sweep_test.go`
(ADR-0080): an empty-data sweep rendering every catalog template and holding its
`requiresSkills` coupling declaration exact, a conditional-fallback case guard requiring
a hand-authored `unsetFallbackCases` entry per conditional template, and a
golden-completeness guard machine-enforcing the one-golden-per-artifact convention in
`spine_test.go`.

Pi-extension tests live under `tools/pi-extension-test/`. A digest-pinned container keeps locked
dependencies in a named volume, snapshots the read-only checkout inside the container for each run,
and executes strict TypeScript and coverage checks without host npm state. The rendered extension
files carry a `// @ts-nocheck` directive (ADR-0126) that keeps adopter IDEs quiet without a
resolvable `@types/node`; the container strips that line from its snapshot before `tsc` runs, so the
type-check still covers the real extension code. Runner tests cover
structured event ordering and bounds, cumulative omissions, setup cleanup, and cancellation. An
in-memory Pi 0.80.9 `AgentSession` proves that partial details and result-middleware error patches
survive the real runtime seam without entering model-visible content. Grounding schema/prompt tests
and shared-renderer tests cover every role and state at narrow and normal widths, including
omissions, diagnostics, usage, malformed details, and configurable expansion hints. TypeScript
owns Pi's exact exploration schema and fixed-prompt behavior; Go owns catalog closure, migration,
and cross-target render behavior. These tests prove schemas and instruction contracts, not
arbitrary model compliance.
`./x pi-test stop|reset` controls the container lifecycle.

Shared test-fixture building (project-config scaffolding, ADR frontmatter fixtures,
file-writing primitives, the seam-swap idiom, and git-repo fixtures) goes through
`internal/testsupport` (and its `gitfixture` subpackage), a leaf package with no dependency on
any other `internal/*` awf package (ADR-0044). New test code needing one of these idioms calls
into `internal/testsupport` rather than hand-rolling a local copy.
