| Area | Test location and shape |
|---|---|
| Go behavior | Focused package tests cover effort/worktree safety, migration, session protocol validation, deterministic joins, resident roots, and real-Git topology. Legacy protocol residents are read-only fixtures. |
| Pi extension | Container tests cover generated protocol-v2 profile negotiation through a contract double, adapter schemas and policy, model routing, native awf skill delivery, Git audits, and retained `using_effort` integration. External `pi-tools` mechanics are not reproduced. |
| Git seam | `internal/git/entrypoints_test.go` derives entrypoints and requires a backend-agnostic contract suite for each. Repo walkers keep Git libraries and subprocesses within `internal/git` and `internal/testsupport/gitfixture`. |
| Test homes | Five `TestMain` suites use canonical `home-<decimal>` homes, retain Go's default `GOPATH`, and sweep only homes older than 24 hours. Failure to remove the current home fails the suite. |

`TestPiRealRuntimeSmoke` runs only when the gate enables it and without caching. The Pi association tests cover root attach/detach, fixed relative paths, owner-checked recovery, heartbeat, detached restart cleanup, advisory Remote Pi metadata, capability-gated suffix negotiation, replay, lifecycle clears, ownership loss, and optional-emission degradation. Adapter tests prove successful and failed protocol-v2 negotiation, no-fallback behavior, profile routing delivery, and generated role discovery without testing external context, handoff, scheduling, execution, or presentation internals.

### Parallelism

Run packages in parallel only when their mutable state is independent. `internal/snapshot`, `internal/project`, `internal/effort`, and fixtures do; `cmd/awf`, `internal/git`, `internal/migrate`, `internal/audit`, and `internal/worktree` do not. `t.Setenv` blocks its calling test, not the package; `internal/worktree` remains serial because package-level filesystem-ownership swaps race.

### Test shape

- Use a table when cases share one act-and-assert shape and differ only in data.
- Keep divergent setup or assertions flat; a one-row table adds scaffolding.
- Use `t.Fatal` when later checks depend on the failed step; use `t.Error` for independent evidence.
- Test one observable behavior. A name requiring "and" usually names two tests.

The isolated-TMPDIR real-hook regression proves the handwritten stub removes its staged slice before rendered-payload handoff.
