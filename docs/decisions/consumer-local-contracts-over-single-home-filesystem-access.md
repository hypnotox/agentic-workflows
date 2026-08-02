---
format: current-state-v3
slug: consumer-local-contracts-over-single-home-filesystem-access
status: Proposed
date: 2026-08-02
---
# ADR-consumer-local-contracts-over-single-home-filesystem-access: Consumer-local contracts over single-home filesystem access

## Context

The paused filesystem/global-seam effort originally covered `internal/project`, every historical
migration, and upgrade transaction recovery. Since that brainstorm, ADR-0178 established explicit
dependency composition, ADR-0180 removed invocation-derived fields from `project.Project`, ADR-0193
made `internal/git` the one semantic Git seam, ADR-0195 split context querying and resident policy out
of `internal/project`, and ADR-0201 prohibited new mutable global test seams. The original conversion
boundary is now both stale and contrary to `code-design/dependency-composition:concrete-first-consumer`:
a mechanism-wide sweep has no one policy consumer.

Current code leaves one bounded, concrete need. `internal/upgrade/digest.go` recomputes the sealed
attestation by walking authored trees, pruning nested projects, reading selected bytes, and recording
permission modes. It calls `filepath.WalkDir`, `os.ReadFile`, and `os.Stat` directly and holds mutable
package global `lstat = os.Lstat` solely so a test can force one nested-boundary failure. Permission,
entry-info, traversal, read, and stat failures remain behind `coverage-ignore` comments because
kernel permissions and concurrent removal are not deterministic test controls. The digest is a
read-only safety boundary immediately before a journaled cutover, so those outcomes merit controlled
tests without replacing real filesystem semantics with an in-memory model.

The repository already has the composition shape to follow. `internal/git` owns a concrete production
handle while each consumer declares only the structural contract it needs and expresses business
meaning in its own helpers and values. This reconciles two live rules: one shared implementation owns
the mechanism, while the consumer owns its narrow contract. A provider-owned universal interface or
a consumer dependency struct full of closures that merely rename provider methods would weaken that
shape rather than clarify it.

Filesystem code elsewhere does not create an automatic conversion scope. In particular,
`internal/snapshot/filesystem.go` has a private snapshot-specific operation bundle for capturing
regular files, executability, and symlink targets from an ordinary directory. It is neither an
exported production adapter nor a root-confined repository capability. Its source and contract remain
distinct: snapshot capture intentionally represents symlinks, while this decision uses Go 1.26
`os.Root` to refuse any path or symlink target outside a selected root. Other direct OS calls remain
the bounded candidates that ADR-0178 and ADR-0181 already permit until a concrete consumer brings
them into scope.

Go 1.26 supplies the production primitive this boundary needs. `os.Root` anchors operations beneath
one opened directory, rejects absolute and parent-escaping names, and follows symlinks only when their
target remains inside that root. The root itself can be a repository or a kernel-backed temporary
directory. This gives tests real filesystem behavior and gives production a stronger boundary than
`filepath.Join` plus lexical validation.

A shared fault source has a separate test-only constraint. `tooling/test-infrastructure:test-support-leaf-boundary`
forbids `internal/testsupport` from importing another repository package, so a shared fixture cannot
wrap the production handle. ADR-test-support-exports-earn-test-consumers establishes the required
symmetry: a dedicated test-support export and composition capability earn a real outside-package test
first consumer, production code never imports test support, and the test implementation is a distinct
kernel-backed controlled-fault source rather than a second production adapter. That prerequisite's
claim operations must apply before the fixture export lands.

## Decision

1. Add `tooling/filesystem-access` with selectors `internal/filesystem/**` and
   `internal/testsupport/fsfixture/**`, and add `internal/filesystem/**` to the tooling domain path
   map. The topic owns the production root-confined handle and the one controlled test fault source;
   `tooling/test-infrastructure` continues to overlap the fixture path and own its leaf dependency
   boundary.

2. Add `internal/filesystem` as the single production home for deliberately composed,
   root-confined filesystem access. Its package comment states that ownership in one sentence. The
   package exports a concrete `Handle`, its constructor, and only the methods required by the first
   consumer; it exports no provider-owned filesystem interface, dependency bag, policy callback, or
   test configuration.

3. Construct a handle by opening an `os.Root` for the selected root. The handle accepts only `.` or
   valid slash-relative names, returns slash-relative walk paths, and rejects empty, absolute,
   parent-escaping, and symlink-escaping access. Opening the root may use any OS path, including a
   test temporary directory. The composition boundary owns handle lifetime and defers `Close`; a
   read-only close failure has no durable state implication and is not promoted into upgrade policy.

4. Give the initial handle exactly four consumer capabilities: walk a subtree while supplying each
   slash-relative path and resolved `fs.FileInfo` to a callback whose boolean result controls descent;
   read one file's bytes; inspect metadata while following a final symlink; and inspect metadata
   without following the final symlink. Naming may use `Walk`, `Read`, `Info`, and `LinkInfo`, but the
   contract is behavioral: traversal resolves entry metadata inside the adapter, reports traversal
   and entry-info failures, never exposes absolute paths or `filepath.SkipDir`, and does not follow a
   directory symlink during the walk. Pure path comparison and attestation selection stay outside the
   handle.

5. Update `code-design/dependency-composition:consumer-owned-contracts` to formalize the general
   pattern. When substitution is needed around a shared concrete implementation, the consumer
   declares the smallest cohesive structural interface locally. The provider exports the concrete
   implementation and neutral values its mechanism yields, not a universal consumer interface.
   Consumer-local helpers and values may translate that imported capability into readable business
   policy, but they do not reimplement the shared concern. A consumer that needs no substitution may
   keep a direct concrete dependency; the rule does not require interfaces as ceremony.

6. Make `internal/upgrade` attestation verification the one named first consumer. It declares a
   private structural interface over the four handle capabilities and gives attestation meaning to
   them through ordinary helpers such as subtree collection, nested-project detection, and digest
   record reading. It does not add a function-field bundle whose closures merely rename handle
   methods. The concrete `filesystem.Handle` satisfies the local interface without importing
   upgrade.

7. Select the production handle at the outer `Verify` boundary. `Verify` opens it for the supplied
   repository root, defers closure, and passes the required interface to an unexported verification
   path, `treeDigest`, and the collection helpers. No inner helper silently constructs a production
   dependency. Delete the mutable package-global `lstat` and its test swap in the same green
   transaction.

8. Keep all attestation policy in `internal/upgrade`: configuration-derived universe selection, ADR
   identity filtering, marker glob matching, nested Git and awf project pruning, optional-missing
   treatment, regular-file filtering, sorting, record encoding, and digest comparison. The
   filesystem package knows none of `config`, `adr`, `pathglob`, bridge attestations, or restoration
   guidance.

9. Read `.awf/config.yaml` through the injected handle and pass its bytes to `config.Parse`; do not
   convert `internal/config` generally. Preserve `config.Load`'s `not an awf project` and `read
   config` context around the underlying error and preserve error identity with `%w`. Converted code
   matches `fs.ErrNotExist` through `errors.Is`, never shallow `os.IsNotExist`. Optional subtrees and
   a selected file absent at its initial read retain their current treatment; other walk, metadata,
   boundary-probe, and read failures propagate.

10. Add `internal/testsupport/fsfixture` as the one kernel-backed controlled filesystem fault source.
    It remains a standard-library-only leaf, opens its own `os.Root`, satisfies consumer structural
    contracts through the same standard-library method signatures, and lets tests select a fault by
    operation and slash-relative path before delegating every unselected operation to the real root.
    It is not an in-memory filesystem and owns no production or attestation policy. Its package and
    implementation comments reference this ADR's distinct-source reasoning, as required by
    `code-design/single-home:single-implementation`.

11. Introduce only fault operations used by upgrade tests in the same transaction. The fixture's
    exported surface earns the outside-package upgrade test consumer under
    ADR-test-support-exports-earn-test-consumers; a compile-only reference does not count. Future
    consumers define their own local contracts and may use this same fixture when its existing
    capability fits. A new method or fault operation still requires its own concrete first consumer;
    anticipated reuse adds nothing.

12. Prove successful digest bytes and path selection remain unchanged, then cover missing optional
    subtrees, traversal failure, entry-info failure, non-regular entries, content-read failure,
    post-read metadata failure, nested Git boundary inspection failure, nested awf boundary
    inspection failure, and propagation through verification. Run the relevant behavior contract
    against both the production handle and fault source where parity matters. Reassess every
    `coverage-ignore` in `internal/upgrade/digest.go`, deleting each exclusion made reachable by the
    seam and retaining only one with an independent unreachable or race-only justification.

13. Keep this conversion narrow. Do not convert project sync, historical migration, upgrade journal
    mutation, snapshot capture, clock, environment, working directory, subprocess, or unrelated
    direct filesystem access. Each later conversion needs its own policy consumer and adopts the
    production handle rather than adding another root-confined implementation when the contract
    fits.

14. Apply ADR-test-support-exports-earn-test-consumers before exporting the fixture, then implement
    this decision through a reviewed multi-package plan. Update the tooling domain map, topic
    metadata and claims, the dependency-composition claim, managed architecture component source,
    rendered documentation, and proof markers in the same checked transactions as their code. The
    implementation uses the `code-design` commit scope for cross-package structure and preserves the
    prerequisite ADR's declared operation order.

## State changes

- update `code-design/dependency-composition:consumer-owned-contracts`
- add `code-design/dependency-composition:upgrade-attestation-filesystem-wiring`
- add `tooling/filesystem-access:single-production-handle`
- add `tooling/filesystem-access:root-confined-paths`
- add `tooling/filesystem-access:single-fault-source`

## Consequences

Upgrade digest failures become deterministic to test without weakening filesystem fidelity. The
mutable `lstat` global disappears, tests can run in parallel, and branches previously justified only
by permissions or concurrent removal can be exercised directly. `os.Root` also prevents an authored
path or symlink from causing attestation reads outside the selected repository, while temporary-root
tests retain the same confinement semantics.

The repository gains one production home and one deliberately distinct test fault source, not one
repository-wide interface. Consumers retain readable business logic because their local helpers name
attestation concepts; the injected interface remains a small structural view of a concrete handle.
Future sessions receive both halves through current-state context: the global consumer-local contract
pattern and the filesystem-specific ownership topic.

The handle introduces resource lifetime and path-contract obligations. The outer boundary must close
it, callers must use slash-relative names, and tests must cover confinement across supported
platforms. Standard-library `fs.FileInfo` crosses the seam because mode, directory, and regular-file
facts are neutral mechanism results the consumer needs; absolute OS paths and traversal sentinels do
not cross.

The fault source necessarily repeats a small amount of standard-library delegation because its leaf
boundary forbids importing production code. That duplication is accepted only for the distinct
kernel-backed controlled-fault contract, recorded here and referenced at the site. Behavior-contract
tests prevent its successful-path semantics from drifting from the production handle. No production
caller or runtime binary can import it.

Existing direct OS calls and snapshot-specific capture remain bounded candidates rather than instant
violations. This limits churn but means the repository is transitional: `internal/filesystem` is the
only home for new deliberately composed root-confined access, not yet the route for every historical
filesystem call.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| A repository-wide `Filesystem` interface mirroring `os` | Gives the provider ownership of consumer contracts, invites speculative methods, and erases policy boundaries. |
| A semantic function-field bundle in upgrade | Adds closure and initialization indirection whose main purpose is renaming concrete methods; the cohesive multi-operation dependency earns a local structural interface. |
| A provider-owned interface implemented by production and tests | Reverses contract ownership and makes every consumer depend on one broad abstraction. |
| Keep the seam and fake local to upgrade | Creates no single production mechanism home and hides the controlled fault source inside its first consumer. |
| Reuse `internal/snapshot` as the production home | Makes upgrade depend on snapshot capture policy; snapshot's symlink-representing ordinary-tree contract is materially different from root-confined repository access. |
| Convert snapshot and every direct filesystem caller now | Violates the concrete-first boundary, couples unrelated policies, and recreates the stale broad effort. |
| Use an in-memory filesystem | Replaces the kernel semantics that attestation safety depends on and becomes a second semantic oracle. |
| Use permissions or concurrent removal in tests | Is nondeterministic across operating systems, identities, and timing, and cannot reliably cover the failure branches. |
| Permit lexical `filepath.Join` confinement | Does not prevent a symlink target from escaping the selected root; `os.Root` provides the required kernel-backed boundary. |

## Status history

- 2026-08-02: Proposed
