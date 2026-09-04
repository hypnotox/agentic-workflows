| Area | Test location and shape |
|---|---|
| Go behavior | Focused package tests cover effort/worktree safety, migration, session validation, Publisher definition completeness and exactly-once rendering, complete collision and destructive preflight, first-failure affected-path reporting, deterministic joins, resident roots, and real-Git topology. |
| Harness publication | Catalog, render, and integration tests prove exactly four fixed `awf-*` skills for Claude and Pi, no AWF roles or adapter, upgrade refusal for unowned content, unrelated-content preservation, and external-package coexistence. |
| Git boundary | Behavioral tests exercise range grammar, repository status, revision and worktree reads, environment isolation, cancellation deadlines, and error identity through package and application boundaries. Native fixtures create real repositories without making backend topology part of the contract. |
| Test homes | Five `TestMain` suites use canonical `home-<decimal>` homes, retain Go's default `GOPATH`, and sweep only homes older than 24 hours. Failure to remove the current home fails the suite. |

### Parallelism

Run tests concurrently only when mutable state is independent. CI may execute independent Go and platform lanes in parallel; package-global seam families remain serial within each process. `t.Setenv` blocks its calling test, not the package, and `internal/worktree` stays in-process serial because package-level filesystem-ownership swaps race.

### Test shape

- Use a table when cases share one act-and-assert shape and differ only in data.
- Keep divergent setup or assertions flat; a one-row table adds scaffolding.
- Use `t.Fatal` when later checks depend on the failed step; use `t.Error` for independent evidence.
- Test one observable behavior. A name requiring "and" usually names two tests.

The isolated-TMPDIR real-hook regression proves the handwritten stub removes its staged slice before rendered-payload handoff.
