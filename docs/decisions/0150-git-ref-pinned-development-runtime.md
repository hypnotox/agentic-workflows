---
format: current-state-v2
status: Proposed
date: 2026-07-23
---
# ADR-0150: Git Ref Pinned Development Runtime


## Context

ADR-0146 requires the Pi dashboard to invoke the canonical Go metrics and doctor engine rather than duplicate it in TypeScript. Its binary resolution supports an enabled `.awf/bootstrap.sh` or an `awf` binary on `PATH` and explicitly rejects the repository runner. The awf repository disables bootstrap because it develops from source, and its normal shell does not require awf on `PATH`. The generated dashboard therefore remains permanently degraded in the repository where it is developed and reviewed.

A simple `go run` fallback would make the dashboard usable, but would rebuild or re-evaluate the mutable checkout on repeated queries. A long-lived Pi session could then switch schema, protocol, or projection semantics in the middle of an effort as the developer edits files. Building from the working tree would also let unreviewed changes become the resident telemetry runtime before the change passes review.

The development runtime needs a stable identity independent of the mutable branch, an immutable cached executable, an explicit reviewed advance operation, and a policy snapshot matching the executable. Existing sessions must not switch implementations after capture. At the same time, read-only metrics and doctor queries need to remain useful while unrelated live project configuration advances, without weakening normal version gates for mutation, retention, purge, repair, waiver, upgrade, or other maintenance.

This is a repository-development fallback, not a replacement distribution channel for adopters with bootstrap or PATH resolution.

## Decision

1. The awf repository uses the local Git ref `refs/awf/dashboard-runtime` as the sole development-runtime pointer for the Pi dashboard fallback. The ref names a commit object, is not pushed as project history, and never follows the checked-out branch automatically.

2. `./x dashboard-awf-path` resolves the ref and prints exactly one launcher path on standard output. If the ref is absent, the command initializes it atomically to the current `HEAD`, emits a diagnostic on standard error, and continues. Concurrent initializers either establish the same expected value or fail visibly; they never overwrite an existing different ref.

3. The path command requires the pinned object to be a commit and builds from that exact committed tree, not the index or working tree. Dirty files are allowed because they cannot affect the build. The command never invokes `go run` for a dashboard query and never rebuilds merely because the current checkout changed.

4. The build key includes repository identity, pinned commit ID, target OS and architecture, Go toolchain identity, and the runtime artifact format version. The command publishes the executable and its metadata into an immutable XDG cache entry using an interprocess lock, a private staging directory, file and directory flushes, and atomic rename. A valid existing entry is reused. A collision, incomplete entry, unsafe path, failed build, or metadata mismatch fails without replacing a good entry.

5. The cache entry also contains an immutable derived `workflowTelemetry` policy snapshot from the pinned commit. The snapshot includes only the config leaves required by protocol handshake, canonical read-only metrics, export, and doctor interpretation. It is produced and validated by pinned code during cache publication and is covered by the same cache identity and atomicity as the executable.

6. The returned launcher binds the executable to its pinned policy snapshot and to read-only dashboard mode. It permits protocol handshake, metrics query and export, and doctor query against a live compatible resident ledger. It does not grant lifecycle writes, repair, waiver, retention application, purge, upgrade, sync, check, or any other mutation or maintenance command through the relaxed development path.

7. The pinned read-only runtime may tolerate unrelated live project schema or lock advancement because it does not load live tracked policy as authority. It must still validate ledger confinement, protocol descriptor compatibility, and policy-snapshot integrity. A telemetry protocol-major mismatch fails with a restart-required diagnostic; it is never interpreted through compatibility guessing.

8. The generated Pi dashboard resolves in this order: enabled bootstrap, `awf` on `PATH`, then the controlled repository runner fallback when the project runner advertises `dashboard-awf-path`. It captures one resolved launcher and handshake per Pi session. Existing sessions retain that launcher even after the ref advances; only a new session resolves the new pointer.

9. `./x dashboard-awf-advance [commit]` is the only supported pointer advance. Omitted `commit` means current `HEAD`. The command requires a clean commit object, captures the current ref value, builds and validates the candidate cache entry first, and updates the ref with compare-and-swap against that captured value. A concurrent change fails without overwriting it. The command prints the old and new commit IDs and the new launcher path.

10. Advancing is an explicit post-review action. Implementation and release documentation instruct maintainers to run it only after the candidate implementation has passed staged checks, the full gate required by the change, and implementation review. The command enforces object and cache validity but does not pretend to infer whether human review occurred.

11. Cache entries are immutable and content-addressed. Failed and expired staging directories may be recovered under the cache lock, but a published entry is never rewritten in place. Cache garbage collection, if later added, is a separate explicit maintenance decision and must not delete the entry captured by a running session.

12. The generic rendered runner keeps its existing bootstrap-owned forwarding contract. The two dashboard development commands live in the awf repository's editable project-verb region and degrade to clear unsupported diagnostics in adopters that do not define them. Generated dashboard code depends only on the advertised command contract, not on awf repository source paths.

13. Tests cover absent-ref initialization, concurrent initialization, explicit-commit selection, dirty-checkout isolation, cache reuse, lock contention, failed and interrupted publication, unsafe cache paths, metadata collision, policy-snapshot derivation and tamper rejection, command allowlisting, live schema advancement for reads, protocol-major refusal, resolution precedence, one-capture-per-session behavior, compare-and-swap advance races, and publication-safe behavior when a project runner lacks the fallback.

## State changes

- add `rendering/companion-scripts:dashboard-development-runtime-commands`
- add `rendering/pi-runtime:pi-pinned-development-runtime`
- update `rendering/adapter-outputs:pi-workflow-dashboard-runtime`

## Consequences

The dashboard works in the from-source awf repository without requiring a globally installed binary and without recompiling on every refresh. A running session has stable executable, protocol, and policy semantics even while development changes the checkout. Reviewed advancement is explicit and does not surprise existing sessions.

The repository gains a local ref, an XDG cache protocol, locking and recovery code, a launcher, and two project runner commands. Developers must advance the pointer after reviewed changes when they want new sessions to use them. The first resolution of a new commit pays one build cost.

Read-only canonical queries can survive unrelated live schema advancement, but all mutations remain subject to normal project and binary version gates. Protocol-major changes require a new session after the pointer advances. This intentionally favors safety over hot-swapping.

The local ref and cache are operational state, not tracked project outputs. Cloning the repository starts with an absent ref that initializes to that clone's current HEAD. Cache cleanup is deliberately left out of scope until retention requirements are known.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Require developers to install awf on `PATH` | It makes the repository's generated runtime depend on untracked workstation setup and does not bind the session to reviewed source. |
| Run `go run ./cmd/awf` for every query | It repeatedly builds and lets a long-lived session change semantics with the mutable checkout. |
| Build once from the working tree at Pi startup | Dirty or unreviewed changes could become the telemetry authority and the build would not have a durable source identity. |
| Resolve current `HEAD` for every new session | Branch movement would silently advance the runtime without the explicit reviewed promotion the pointer is meant to provide. |
| Hot-swap existing sessions after ref advance | In-flight sessions could mix protocol and projection semantics within one effort. |
| Relax the normal version gate for all pinned commands | A stale runtime must not mutate a newer project or perform maintenance under an old schema interpretation. |
| Store the policy snapshot in the live metrics tree | It would mix tracked-policy derivation with resident effort data and allow a session's interpretation to change independently of its executable. |
| Use a globally named mutable cache path | Concurrent builds and advances could replace the executable beneath running sessions. |

## Status history

- 2026-07-23: Proposed
