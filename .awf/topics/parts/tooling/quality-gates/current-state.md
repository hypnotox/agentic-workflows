These packages and the command runner enforce the deterministic quality gates: coverage, prose punctuation, working-memory citations, and the gate tiers. The claims below capture the current gate contracts.

The Pi host lane enforces 100% statement, line, function, and branch coverage for every generated Pi extension without reachable-branch ignores.

## Claims

### `invariant: gates-always-run`

The prose and memory-citation repository checks always scan, retain their configured exemption lists, and expose no disabled state or enablement key.
Origin: ADR-0253
Backing: test

### `invariant: coverage-raw-identity-ratchet`

The full gate deterministically partitions every top-level Go proving unit into isolated, contention-qualified slices, collects whole-`coverpkg` set profiles, and feeds their canonical OR-merged union to `coverage-baseline.json`. The merger rejects malformed headers, mixed modes, ambiguous paths, conflicting statement counts, and empty shards; policy then requires the complete reviewed universe digest. Every raw uncovered block is identified by module-relative file, exact span, and statement count, and any identity absent from the repository baseline or an applicable one of the six exact critical selectors blocks. Covered identities disappear on regeneration, while additions and moved spans require stored reasons, so an unrelated removal or aggregate-count swap cannot authorize a regression. Raw and filtered percentages remain reports only.
Origin: ADR-0302
Revised-by: ADR-0313, ADR-0314
Backing: test

### `invariant: coverage-ignore-admission`

The canonical baseline inventories production and test `coverage-ignore` directives separately. Every retained production directive has explicit evidence and exactly one admitted class: directly tested process-exit seam, revalidated impossible state, safely uninducible deterministic fault, or platform-only branch. Its static unmeasured ledger contains exactly the two Darwin publication rollback directives and never claims that the Linux profile measured them.
Origin: ADR-0302
Revised-by: ADR-0313
Backing: test

### `invariant: coverage-executed-ignore-errors`

Coverage policy evaluation canonically OR-merges duplicate exact profile blocks, matches directives to guarded-body entry positions, and exits nonzero if any measured ignored guarded body has a positive execution count.
Origin: ADR-0302
Backing: test

### `invariant: covercheck-mutation-regression`

The full gate selects an exact `cmd/covercheck` owned-path change from the preserved local staged candidate or a validated explicit range union and runs the pinned, hermetic whole-target mutation blocker; missing, malformed, or unavailable selection evidence runs rather than skips it. The blocker requires the whole-repository preflight, package-test and dependency censuses, dry-to-actual exact identity equality, complete timeout-free trusted reports, and only killed or independently reviewed equivalent survivors. Mutation remains advisory outside that owned path.
Origin: ADR-0302
Revised-by: ADR-0308, ADR-0313
Backing: test

### `invariant: coverage-ignore-reason`

A `// coverage-ignore` marker carrying no non-empty reason makes the coverage checker fail rather than silently dropping the marked block; the reason text is mandatory.
Origin: ADR-0012
Backing: test

### `invariant: covered-profile-honors-ignores`

The filtered coverprofile emitted by internal/coverage contains a block if and only if that block is not coverage-ignore-d under the same directive interpretation used by the raw-identity policy, so filtered Codecov reporting and policy diagnostics never diverge on what ignored means. The filtered percentage is informational rather than a blocking threshold.
Origin: ADR-0065
Revised-by: ADR-0302
Backing: test

### `invariant: deadcode-gate`

The full gate runs deadcode without the -test flag over ./... and fails on any reported unreachable function outside internal/testsupport/; cmd/deadcodecheck ignores exactly that path prefix and exits non-zero on every other finding.
Origin: ADR-0063
Revised-by: ADR-0313
Backing: test

### `invariant: memory-citation-gate`

The check repo memory command reports every concrete `.awf/efforts/<slug>/memory.md` reference in scannable staged decision and plan text and exits non-zero on any finding outside memoryCite.exemptions; check staged commit applies the same slash-or-backslash detector to the git-cleaned message body without exemptions. Prose, links, code spans, and normalized relative spellings are detected without reading resident files, while the bare `.awf/efforts/` directory and an angle-bracket slug placeholder pass.
Origin: ADR-0158
Revised-by: ADR-0159, ADR-0175, ADR-0210, ADR-0253
Backing: test

### `invariant: mutants-timeout-untrusted`

The mutation-report checker exits non-zero when any mutation in its input JSON has status TIMED OUT, signalling an untrustworthy run; otherwise it reports exactly the surviving (LIVED) mutants, dropping NOT COVERED and every other status, and treats a missing or empty input file as an empty run with no survivors.
Origin: ADR-0066
Backing: test

### `invariant: gate-tier-cadence`

`./x gate` is the static-analysis-only fast commit tier: version validation, one native build, blocking lint including its vet analysis, and workflow pin validation. `./x test-affected` is separate fail-closed behavioral feedback for iteration. `./x gate full` remains terminal assurance and adds complete native Go and Pi behavior, coverage policy, standalone vet, advisory analysis, dead-code analysis, four Linux/Darwin release cross-builds, and exact-universe-selected covercheck mutation. Full verification uses the staged candidate locally or explicit range union remotely; uncertainty runs mutation conservatively.
Origin: ADR-0313
Revised-by: ADR-0314
Backing: test

### `invariant: affected-package-feedback`

`./x test-affected` reads complete staged, working-tree, or explicit-range change evidence and reports deterministic selected targets and reasons before executing them without coverage. It runs changed package owners, every production reverse dependent, test-only importing packages, and declared exact-name meta-suites through bounded isolated workers; a suite runs separately only when its complete owner package is not already selected, and otherwise the full package run must emit execution evidence for every declared proving unit. Shared generators, templates, configuration, tooling, generated or build-tagged Go, deleted ownership, malformed evidence, unavailable packages, and unknown paths widen to the full Go universe or refuse explicitly. The fast gate and terminal full gate remain unchanged.
Origin: ADR-0314
Backing: test


### `rule: verification-performance-contract`

`test-performance.json` is the canonical versioned qualification record for fast, common-local, ordinary-full, exceptional mutation, and hosted critical-path workloads. It declares complete reference environments, cache and sample preparation, landed baselines, unchanged minimums and stronger targets, component evidence, and achieved observations. `./x test-performance validate` refuses malformed records and unlike environment evidence without rewriting the record; `./x test-performance report` renders human or machine evidence from the same observations. Wall-clock samples are qualification evidence rather than flaky correctness assertions, while schema, identity, workload, and component-regression failures block qualification.
Origin: ADR-0314

### `invariant: pi-extension-container-gate`

The full gate wires a complete pinned-host-Node Pi-extension lane. NVM selects exact v24.19.0 locally without downloading, while an explicit CI control accepts the same exact setup-node runtime after exact-version validation. An atomically attributable checkout-local manager and worker-group lock serializes dependency preparation and the complete lane, recovering only when both recorded owners are gone; `npm ci --ignore-scripts` publishes a reusable tree only after success under a labeled, length-framed fingerprint of the pin, manifests, exact Node/npm versions, OS, and architecture. Every run uses a narrow temporary copy with only Pi extensions, agents, skills, harness inputs, and minimal metadata, links that tree, and leaves operator-local Pi state untouched. The commit gate does not run this behavioral lane, and the full gate never derives its execution from staged-path classification.
Origin: ADR-0123
Revised-by: ADR-0198, ADR-0275, ADR-0276, ADR-0281, ADR-0313
Backing: test

### `invariant: prose-gate-refuses-without-git`

In an adopted tree that is not a git repository, the check repo prose command always scans and refuses with an error about being unable to read staged files.
Origin: ADR-0119
Revised-by: ADR-0159, ADR-0210, ADR-0253
Backing: test

### `invariant: prose-gate-tracked-file-scan`

The prose scanner examines every tracked text file without language-specific comment detection, reports a Warning for every en dash and each blank-line-delimited paragraph containing three or more em dashes, and exits zero for findings. It permits ellipses and curly quotes, silently skips files that are not valid UTF-8, and orders findings by path, codepoint, and paragraph. A configured path-and-codepoint exemption, with an optional exact whole-file count, suppresses its guarded character before paragraph evaluation; exemptions for formerly guarded ellipses and curly quotes remain accepted as inert compatibility input. Inability to read or scan the declared corpus remains an Error with nonzero exit because verification is unavailable.
Origin: ADR-0119
Revised-by: ADR-0285, ADR-0290, ADR-0295
Backing: test

### `invariant: gate-severity-by-protected-property`

The repository gate exits nonzero for version or schema incompatibility, required test loss, raw-identity or critical-selector coverage regression, false or unsupported ignore evidence, full-tier-selected `cmd/covercheck` mutation regression, build failure, concrete defect lint, unreachable production code, workflow pin failure, and any checker execution or configuration failure because they protect correctness, safety, authority, or reproducibility. Its separate advisory lint lane reports style, wording, formatting, preferred idiom, speculative performance, possible cohesion, and heuristic maintainability findings as visible Warning output with successful exit. Every enabled lint rule belongs to exactly one classified lane before it runs.
Origin: ADR-0295
Revised-by: ADR-0302, ADR-0313
Backing: test

### `invariant: testsupport-zero-internal-deps`

No non-test Go file under internal/testsupport, including internal/testsupport/gitfixture, imports any internal awf package; only the Go standard library, plus go-git within gitfixture, are permitted, enforced by a static scan of the package's own import graph.
Origin: ADR-0044
Backing: test

### `invariant: workflow-actions-sha-pinned`

The pincheck gate exits non-zero when any remote uses: reference under .github/workflows is not pinned to a full 40-hex commit SHA (repo-local ./ refs exempt, docker:// refs digest-pinned) or when a goreleaser-action version input is not an exact semver version.
Origin: ADR-0079
Backing: test

### `invariant: exact-revision-repository-acceptance`

Repository wiring keeps stable `CI / gate` and `CI / release-config` jobs for one exact revision. The gate conclusion requires native Linux coverage shards and canonical policy, native macOS shards, Pi behavior, analysis and cross-builds, and separately selected mutation. Release configuration constructs one snapshot and validates those same bytes. The release workflow verifies both stable conclusions for the exact release SHA before the credential-bearing publish job.
Origin: ADR-0308
Revised-by: ADR-0313, ADR-0314
Backing: test

### `rule: hosted-main-acceptance-settings`

The live GitHub `main` ruleset requires the GitHub Actions checks `CI / gate` and `CI / release-config` for the exact candidate revision, permits no bypass actor, and retains deletion, non-fast-forward, and signed-commit protections.
Origin: ADR-0308
