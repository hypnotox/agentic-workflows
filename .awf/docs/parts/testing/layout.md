| Area | Test location and shape |
|---|---|
| Go behavior | Focused package tests cover effort/worktree safety, migration, session protocol validation, deterministic joins, resident roots, and real-Git topology. Legacy protocol residents are read-only fixtures. |
| Pi extension | Container tests cover generated protocol-v2 profile negotiation through a contract double, adapter schemas and policy, model routing, native awf skill delivery, Git audits, retained `using_effort` integration, and narrow selected-checkout lifecycle composition. The test-only pi-tools v0.3.0 source pin supplies generic Pi recordings and `createSubagentToolkit` composition for prepared-CWD transport, callback traversal, and invocation isolation; confinement, subprocess supervision, and general presentation mechanics are not reproduced. |
| Git seam | `internal/git/entrypoints_test.go` derives entrypoints and requires a backend-agnostic contract suite for each. Repo walkers keep Git libraries and subprocesses within `internal/git` and `internal/testsupport/gitfixture`. |
| Test homes | Five `TestMain` suites use canonical `home-<decimal>` homes, retain Go's default `GOPATH`, and sweep only homes older than 24 hours. Failure to remove the current home fails the suite. |

`TestPiRealRuntimeSmoke` runs only when the gate enables it and without caching. It requires the named selected-checkout lifecycle proof to pass. The Pi association tests cover root attach/detach, fixed relative paths, owner-checked recovery, heartbeat, detached restart cleanup, advisory Remote Pi metadata, capability-gated suffix negotiation, replay, lifecycle clears, ownership loss, and optional-emission degradation. Adapter tests prove successful and failed protocol-v2 negotiation, no-fallback behavior, profile routing delivery, generated role discovery, and the narrow toolkit lifecycle above without testing external context, handoff, subprocess supervision, or presentation internals.

### Parallelism

Run tests concurrently in one process only when their mutable state is independent. The terminal gate may execute deterministic proving-unit slices in separate isolated process homes; package-global seam families remain serial within each process. `t.Setenv` blocks its calling test, not the package, and `internal/worktree` stays in-process serial because package-level filesystem-ownership swaps race.

### Test shape

- Use a table when cases share one act-and-assert shape and differ only in data.
- Keep divergent setup or assertions flat; a one-row table adds scaffolding.
- Use `t.Fatal` when later checks depend on the failed step; use `t.Error` for independent evidence.
- Test one observable behavior. A name requiring "and" usually names two tests.

The isolated-TMPDIR real-hook regression proves the handwritten stub removes its staged slice before rendered-payload handoff.
