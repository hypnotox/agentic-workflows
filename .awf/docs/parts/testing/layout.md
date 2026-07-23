## Test layout

Package unit tests are Go `_test.go` files in `internal/<pkg>`, in that package's own test
package (`package <pkg>` or the black-box `package <pkg>_test` where a test needs no access to
unexported identifiers). Template golden tests (render assertions against the embedded catalog)
live in `internal/project/spine_test.go`. CLI integration tests drive the `awf` binary's
command functions directly (not a subprocess) against a temp directory built with `t.TempDir()`,
in `cmd/awf/*_test.go`. The private dashboard-read tests prove the exact argv allowlist, validation
ordering, snapshot-backed reads across live schema advancement, and that forbidden mutation shapes
cannot reach a handler.

`internal/dashboardruntime` tests use temporary Git repositories and XDG caches to cover absent and
concurrent ref initialization, commit peeling, dirty-checkout isolation, normalized builds, immutable
reuse, advisory-lock release, interrupted staging recovery, atomic publication, path and collision
rejection, complete canonical policy snapshots, digest tampering, and compare-and-swap advance races.
The cached launcher's closed translation is tested separately under `cmd/awf-dashboard-launcher`.

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
and executes strict TypeScript plus 100% statement, branch, function, and line coverage checks without
host npm state. Every rendered extension file carries a `// @ts-nocheck` directive (ADR-0126) that
keeps adopter IDEs quiet without a resolvable `@types/node`; the container strips that line from its
snapshot before `tsc` runs, so the type-check still covers the real extension code. Descriptor golden and fixture tests compare the complete protocol-2 Go and generated TypeScript
vocabulary, event acceptance, creation and append recovery, transactional phase transitions, and gate
classification. A repository-resident preflight test refuses automatic cleanup when any protocol-1
effort exists and proves it changes no resident evidence. Dashboard tests cover confined owner-only
storage, leases, tombstones, serialized drain, explicit versus passive failure, active-branch
association, bounded provisional creation/discard/overflow/graceful/crash behavior, ordered retry recovery, structured replacement resume and cancellation, trajectory navigation, closed lifecycle schemas, bootstrap authority,
PATH-to-advertised-runner fallback, launcher project-root environment, bounded dual-failure
diagnostics, one launcher capture per session, controlled binary handshake and refresh, atomic
stale/degraded state, widget and overlay rendering, fixed maintenance arrays, confirmation, complete
repository-path privacy exclusion, effort-owned finding re-resolution, stale or empty-frontier and
cross-effort mutation rejection, and no-spawn rendering. Workflow-loader tests cover the closed semantic enum, exclusive trustworthy batches, catalog-mapped route, phase, activity, mode, and terminal effects, one-event chain transitions, causal predecessor rejection, winner/conflict retries, fixed-body identity after durable acknowledgement, and retrospective completion, failure, cancellation, shutdown, and recovery. Runner tests cover
structured event ordering and bounds, cumulative omissions, setup cleanup, and cancellation. An
in-memory pinned-fork Pi 0.81.1 runtime loads all three extension factories and proves that partial details,
result-middleware error patches, producer observations, router loading, structured replacement, memory-identity association copying before kickoff, widget and overlay
registration, lifecycle durability, retrospective settlement, shutdown drain, canonical refresh degradation, and current-leaf
mixed-implementation blocking survive the real runtime seam without entering model-visible content.
Compatibility tests reject an official-0.81.1-shaped API that lacks any required queued-command,
persisted-session, custom-entry, widget, overlay, or shutdown surface. Handoff tests cover the closed schema, path and symlink boundaries, exact memory `Effort:` parsing, independently validated association match, exclusivity, pending token, countdown cleanup and cancellation, pending-window identity revalidation, parent-linked replacement with setup-time association, restore-before-kickoff ordering, teardown failures, kickoff, and editor fallback. The container type-check and c8 coverage globs include every extension TypeScript file. Unit tests cover exact all-role model routing and rejection, inherited
thinking, and the ten-active FIFO limiter's abort and release lifecycle including runner setup
failure. Grounding schema/prompt tests and shared-renderer tests cover every role and state at narrow
and normal widths, including omissions, diagnostics, usage, malformed details, and configurable
expansion hints. TypeScript owns Pi's exact exploration schema and fixed-prompt behavior; Go owns
catalog closure, migration, and cross-target publication safety so Pi-only model and concurrency
syntax never leaks to another target. These tests prove schemas and instruction contracts, not
arbitrary model compliance.
`./x pi-test stop|reset` controls the container lifecycle.

Release readiness also requires the unbacked real-Pi smoke on `hypnotox/pi` `fork-v0.81.1-awf.3` or a later API-verified build. In a new session, load `brainstorming` through `awf_workflow`, load its routed successor, and verify one transactional phase transition. Cancel and retry `/awf-resume-effort <effort-id>` into a replacement; create matching memory carrying `Effort: <id>`, hand off, and verify association is present before kickoff. Interrupt and retry one provisional or transition settlement to verify idempotent recovery, then fork/resume, refresh the dashboard, exercise widget/overlay and a canceled destructive action, complete, shut down, and inspect retained history. Repeat with canonical binary resolution unavailable and confirm visible non-blocking degradation. Record the exact commands and observed states in the release work; this manual smoke has no invariant proof marker.

Shared test-fixture building (project-config scaffolding, ADR frontmatter fixtures,
file-writing primitives, the seam-swap idiom, and git-repo fixtures) goes through
`internal/testsupport` (and its `gitfixture` subpackage), a leaf package with no dependency on
any other `internal/*` awf package (ADR-0044). New test code needing one of these idioms calls
into `internal/testsupport` rather than hand-rolling a local copy.
