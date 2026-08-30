The command runner, hosted workflows, and repository checks provide focused local feedback and one aggregate CI verdict.

## Claims

### `invariant: gates-always-run`

The prose and memory-citation repository checks always scan, retain their configured exemption lists, and expose no disabled state or enablement key.
Backing: test

### `invariant: memory-citation-gate`

The check repo memory command reports every concrete `.awf/efforts/<slug>/memory.md` reference in scannable staged decision text and exits nonzero on any finding outside memoryCite.exemptions; check staged commit applies the same slash-or-backslash detector to the git-cleaned message body without exemptions. Prose, links, code spans, and normalized relative spellings are detected without reading resident files, while the bare `.awf/efforts/` directory and an angle-bracket slug placeholder pass.
Backing: test

### `invariant: gate-tier-cadence`

`./x gate` is the fast commit tier: version validation, one native build, blocking lint including its vet analysis, and workflow pin validation. `./x test-affected` is separate fail-closed behavioral feedback. Complete Go and Pi behavior runs in CI and at terminal implementation verification rather than through a second local gate tier.
Backing: test

### `invariant: affected-package-feedback`

`./x test-affected` reads complete staged, working-tree, or explicit-range change evidence and reports deterministic selected targets and reasons before executing them without coverage. It runs changed package owners, production reverse dependents, test-only importing packages, and a small declared package smoke set through bounded isolated workers. Shared generators, templates, configuration, tooling, generated or build-tagged Go, deleted ownership, malformed evidence, unavailable packages, and unknown paths widen to the full Go universe or refuse explicitly.
Backing: test

### `invariant: pi-extension-container-gate`

The pinned-host-Node Pi-extension lane uses the declared Node version and a checkout-local dependency tree, runs against a narrow temporary copy, and leaves operator-local Pi state untouched. The commit gate does not run this behavioral lane; CI and explicit `./x pi-test run` do.
Backing: test

### `invariant: prose-gate-refuses-without-git`

In an adopted tree that is not a git repository, the check repo prose command always scans and refuses with an error about being unable to read staged files.
Backing: test

### `invariant: prose-gate-tracked-file-scan`

The prose scanner examines every tracked text file without language-specific comment detection, reports a Warning for every en dash and each blank-line-delimited paragraph containing three or more em dashes, and exits zero for findings. It permits ellipses and curly quotes, silently skips files that are not valid UTF-8, and orders findings by path, codepoint, and paragraph. A configured path-and-codepoint exemption, with an optional exact whole-file count, suppresses its guarded character before paragraph evaluation. Inability to read or scan the declared corpus remains an Error with nonzero exit.
Backing: test

### `invariant: gate-severity-by-protected-property`

The repository gate exits nonzero for version or schema incompatibility, build failure, concrete defect lint, workflow pin failure, and checker execution or configuration failure. Advisory lint reports style, wording, formatting, preferred idiom, speculative performance, possible cohesion, and heuristic maintainability findings as visible Warning output with successful exit.
Backing: unbacked
Verify: Inspect `x`, `.golangci.yml`, and `.golangci-advisory.yml`; blocking commands must propagate failure and advisory lint must use a zero issues exit code without hiding execution failures.

### `invariant: testsupport-zero-internal-deps`

No non-test Go file under internal/testsupport, including internal/testsupport/gitfixture, imports any internal awf package; only the Go standard library, plus go-git within gitfixture, are permitted, enforced by a static scan of the package's own import graph.
Backing: test

### `invariant: workflow-actions-sha-pinned`

The pincheck gate exits non-zero when any remote uses: reference under .github/workflows is not pinned to a full 40-hex commit SHA or when a goreleaser-action version input is not an exact semver version.
Backing: test

### `invariant: exact-revision-repository-acceptance`

Repository wiring exposes one stable `CI / gate` conclusion for the exact revision. It aggregates exhaustive Linux behavior, build, lint, state and drift checks, strict Pi behavior, and targeted macOS safety. The release workflow verifies that exact conclusion before constructing and validating snapshot archives and before its credential-bearing publish job.
Backing: test

### `rule: hosted-main-acceptance-settings`

The live GitHub rulesets still require `CI / gate` and the retired `CI / release-config` status. Repository owners must update those remote rules to require only `CI / gate`; repository files do not claim that remote change has occurred.
