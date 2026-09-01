| Area | Test location and shape |
|---|---|
| Go behavior | Focused package tests cover effort/worktree safety, migration, session protocol validation, Publisher definition completeness and exactly-once rendering, collision-before-render ordering, deterministic joins, resident roots, and real-Git topology. Legacy protocol residents are read-only fixtures. |
| Pi extension | Host tests cover generated protocol-v2 profile negotiation through a contract double, adapter schemas and policy, model routing, native awf skill delivery, Git audits, the absence of effort association, and narrow selected-checkout implementation composition. The test-only pi-tools v0.3.0 source pin supplies generic Pi recordings and `createSubagentToolkit` composition for prepared-CWD transport, callback traversal, and invocation isolation; confinement, subprocess supervision, and general presentation mechanics are not reproduced. |
| Git boundary | Behavioral tests exercise range grammar, repository status, revision and worktree reads, environment isolation, cancellation deadlines, and error identity through package and application boundaries. Native fixtures create real repositories without making backend or source topology part of the contract. |
| Test homes | Five `TestMain` suites use canonical `home-<decimal>` homes, retain Go's default `GOPATH`, and sweep only homes older than 24 hours. Failure to remove the current home fails the suite. |

The CI Pi lane and explicit `./x pi-test run` execute the host suite directly. The Go `TestPiRealRuntimeSmoke` wrapper runs only when `AWF_PI_RUNTIME_SMOKE=1` and otherwise skips to avoid duplicating that suite. It requires the named selected-checkout lifecycle proof to pass. Adapter tests prove successful and failed protocol-v2 negotiation, no-fallback behavior, profile routing delivery, generated role discovery, absent retired effort-association outputs, and the narrow toolkit lifecycle above without testing external context, handoff, subprocess supervision, or presentation internals.

### Parallelism

Run tests concurrently only when their mutable state is independent. CI may execute independent Go, Pi, and platform lanes in parallel; package-global seam families remain serial within each process. `t.Setenv` blocks its calling test, not the package, and `internal/worktree` stays in-process serial because package-level filesystem-ownership swaps race.

### Test shape

- Use a table when cases share one act-and-assert shape and differ only in data.
- Keep divergent setup or assertions flat; a one-row table adds scaffolding.
- Use `t.Fatal` when later checks depend on the failed step; use `t.Error` for independent evidence.
- Test one observable behavior. A name requiring "and" usually names two tests.

The isolated-TMPDIR real-hook regression proves the handwritten stub removes its staged slice before rendered-payload handoff.
