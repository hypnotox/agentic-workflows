`./x gate` is the static-analysis-only fast commit tier: version validation, one native build, blocking lint including govet, and workflow pin validation. Use focused tests, builds, or lint while editing, then run the separate `./x test-affected` command for fail-closed behavioral feedback. It reports selected changed owners, affected callers, and declared meta-suites with reasons before bounded execution without coverage; shared or uncertain inputs widen or refuse visibly. `./x gate full` is terminal exhaustive verification: it runs every top-level Go proving unit once in isolated deterministic slices, canonically merges whole-module set profiles, applies coverage policy, and adds complete Pi behavior, standalone vet, advisory lint, dead-code analysis, four Linux/Darwin release cross-builds, and range-qualified `cmd/covercheck` mutation. The Pi runtime smoke runs without analysis contention; independent analysis and platform stages replay buffered output in stable order after every started stage terminates. A local full run selects mutation from the exact staged candidate; pre-push, CI, and release callers provide one or more exact ranges. Missing or malformed evidence runs mutation conservatively. The Pi lane retains strict type checks and 100% statement, branch, function, and line coverage with descriptor parity.

| Gate class | Protected property and exit behavior |
|---|---|
| Error | Version/schema compatibility, tests, coverage, vet, builds, defect lint, production reachability, and workflow pins protect correctness, safety, authority, or reproducibility and exit nonzero. |
| Warning | Style, wording, formatting, preferred idiom, speculative performance, possible cohesion, and heuristic maintainability lint remains visible and exits zero. |
| Information | Skipped-lane and successful operation notes exit zero. |

### Check severity

`awf check` uses the same three visible categories. Error and Warning are the two fixed finding ranks; Information is an unranked note. A complete report exits nonzero exactly when it contains an Error.

| Category | Current checks and protected property |
|---|---|
| Error | Invalid config, locks, sidecars, ADRs, plans, topics, frontmatter, and declared references protect correctness and authority. Generated and staged drift, tracking membership, residue, binary/schema compatibility, current-state transitions, memory citations, commit policy, and unavailable required verification protect reproducibility, safety, or authority. |
| Warning | Prose punctuation, glossary length, tag health, plan assignment/detail, current-state fan-out, and guide size are style, readability, cohesion, or review heuristics. They remain visible and exit zero. |
| Information | Unused or unset render vocabulary, stub content, marker suggestions, tracking or staged-universe availability, non-blocking compatibility, context suggestions, and successful operation notes are optional guidance and exit zero. |

Direct checks retain the same classification as their aggregate. Operational inability to load or scan a declared universe remains an Error even when that universe's produced findings are Warning or Information.

| Command | Use |
|---|---|
| `./x gate` | Fast commit verification. |
| `./x gate timings` | Fast commit verification with elapsed stage timings. |
| `./x gate full` | Terminal exhaustive verification. |
| `./x gate full timings` | Terminal verification with elapsed stage timings. |
| `./x test` | Go suite without the host Pi smoke. |
| `./x test-affected` | Report and run working-tree affected-package feedback. Use `--staged` or `--range <base>..<head>` for reproducible evidence. |
| `./x pi-test run` | Pi lane alone. |
| `./x test-performance validate` | Validate the canonical qualification record without rewriting it. |
| `./x test-performance report` | Render qualification evidence for people; add `--machine` for JSON. |

### Performance qualification

`test-performance.json` distinguishes the fast gate, common affected-package feedback, ordinary local full verification, exceptional selected mutation, and the hosted critical path. It binds each observation to the complete declared environment and refuses a mismatched identity. Prepare warm samples by running the workload once and retaining the declared Go caches; run `GOTOOLCHAIN=go1.26.4 go clean -testcache` before each cold sample. Capture stage timings with `GOTOOLCHAIN=go1.26.4 ./x gate timings` or `GOTOOLCHAIN=go1.26.4 ./x gate full timings`, record the complete environment and component observations, then run `./x test-performance validate` and `./x test-performance report --machine`. For a package split across concurrent proving-unit slices, `package-total` records its slowest slice. A wall-clock sample is evidence, not a per-run correctness assertion. The ordinary-full component limits pin the landed serial profiled lane, dominant package totals, coverage policy, and Pi smoke; a qualified sample missing one or exceeding its median limit blocks qualification. Unchanged wall targets remain visible evidence.

The gate enables `TestPiRealRuntimeSmoke` once with test caching disabled; `./x test` and verbose direct Go tests explain its omission. The deterministic Pi lane uses a protocol-v2 contract double to prove generated adapter negotiation, native awf skill discovery and routing delivery, and retained effort integration. Its test-only pi-tools v0.3.0 source pin supplies generic recorder seams and one narrow `createSubagentToolkit` lifecycle composition that proves prepared-CWD transport, completed and failed callback traversal, and checkout-isolated invocation state. It neither installs nor pins an adopter pi-tools runtime. Plain-punctuation (`awf check repo prose`) and effort-memory (`awf check repo memory`) scans are hook and CI checks, not gate steps. Commit-provenance tests use native Git, SSH-signed commits, disposable refs and remotes, both `core.hooksPath` forms, and linked worktrees. A red gate blocks the commit: fix the cause or revert.

### Coverage

`./x gate full` is the sole hard coverage gate. `covercheck --merge` rejects malformed, mixed-mode, ambiguous, conflicting, or empty shard inputs and emits one sorted set profile whose execution counts are OR-merged. Policy compares that complete union's exact statement-block identities with `coverage-baseline.json`; covered misses improve the baseline automatically, while additions and moved spans require explicit reviewed reasons. Raw and filtered statement percentages are reports only. Codecov reports line coverage, so its figures cannot equal `go tool cover`'s statement figures (ADR-0065).

| Codecov flag | Meaning |
|---|---|
| `raw` | Whole-tree line coverage. |
| `covered` | Line coverage after `covercheck --emit-filtered` removes `// coverage-ignore` blocks. |

Codecov is informational; exact coverage policy is enforced by `./x gate full`, including the hosted `CI / gate` check required for protected `main` and release tags.

### Mutation testing

Coverage proves execution, not useful assertions. Change a condition, comparison, or constant and confirm a test fails; otherwise add the missing assertion.

`./x mutants` (ADR-0066) runs deterministic `gremlins` mutation testing against the production diff from `main`; pass a package, such as `./x mutants ./internal/refs`, for a focused run. It remains advisory. The sole blocking exception is an exact `cmd/covercheck` owned-path change: the full gate selects from its local staged candidate or explicit remote ranges, uncertainty runs the blocker, and the pinned whole-target recipe accepts only killed or independently reviewed exact equivalent mutants from complete timeout-free reports. A timed-out mutant is untrusted. Triage every survivor as a missing assertion or an equivalent mutant.

The strict host lane covers awf's rendered profile contract, routing and Git policy, handshake outcomes, generated-output boundary, retained effort behavior, and the narrow selected-checkout lifecycle composition. General context usage, handoff, subprocess supervision, and progress rendering remain assured by `pi-tools`, not duplicated in awf tests.
