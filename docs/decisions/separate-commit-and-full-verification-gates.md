---
format: current-state-v4
slug: separate-commit-and-full-verification-gates
status: Implementing
date: 2026-08-27
---
# ADR-separate-commit-and-full-verification-gates: Separate commit and full verification gates


## Context

The repository has one `./x gate` command for both commit-time feedback and exhaustive assurance.
Depending on the staged paths, it can run the whole-module profiled Go suite and coverage policy and
the Pi runtime suite. Every invocation also runs version validation, vet, six release-platform
cross-builds, blocking and advisory lint, dead-code analysis, workflow pin validation, and selected
mutation verification. This makes ordinary commits and iterative fixes pay much of the same cost as
pre-push, CI, and release verification.

ADR-0275 and ADR-0276 made suite selection a staged-path optimization while keeping every non-test
check unconditional. ADR-0284 retained that shape for test-free release preparation. The resulting
policy still conflates two different feedback boundaries: a quick candidate check at commit time and
a complete verification of an implementation. Agent guidance compounds the cost when it manually
runs a gate immediately before `git commit` or `git push` triggers a wired hook that runs the same
gate again.

Measured in a clean worktree, version validation took about 0.8 seconds, a host build 1.3 seconds,
vet 1.2 seconds cold and 0.2 seconds warm, and workflow pin validation 0.1 seconds. Blocking lint took
about 13.5 seconds cold and 1.3 seconds with cached analysis; it already enables `govet`. These checks
fit a useful commit boundary better than the behavioral, coverage, Pi, cross-platform, advisory, and
mutation suites.

The six current platform stages are cross-compilations, not native test executions. Ordinary tests
run only on the Linux/amd64 host. The repository nevertheless owns Darwin-specific filesystem
publication and durability implementations whose runtime behavior cannot be proved by a Linux
cross-build. Release configuration also still publishes Windows binaries even though Windows is no
longer an intended supported platform.

## Decision

1. `decision: two-gate-cadence` Keep a fast commit gate and a distinct exhaustive full gate. Focused
tests, reproductions, builds, or lint checks are the iteration loop. The commit gate runs at the
commit boundary; the full gate runs at the end of an implementation, and additionally at pre-push,
CI, and release assurance boundaries. A full run inside an implementation is exceptional and must
be justified by the affected risk rather than by task or phase cadence.

2. `decision: hook-aware-execution` Gate guidance must account for active Git hooks. An agent does
not manually run a gate immediately before a commit or push whose wired hook will run that same gate.
It runs the gate manually when the corresponding hook is absent, when the operation will not trigger
the hook, or when a distinct terminal verification boundary requires independent evidence.

3. `decision: fast-commit-gate` The commit gate retains canonical version validation, one native host
build, blocking lint including vet analysis, and workflow pin validation. Behavioral test suites,
coverage policy, Pi runtime verification, advisory lint, standalone vet, dead-code analysis,
release-target cross-builds, and mutation qualification are not commit-time work.

4. `decision: exhaustive-full-gate` The full gate includes every commit-tier check plus the complete
Go and Pi behavioral suites, whole-module coverage policy, advisory static analysis, dead-code
analysis, release-target compilation, and mutation verification when the exact local staged
candidate or supplied range union selects the mutation-owned path. A local full run executes tests only on its native
platform; it does not emulate another operating system or architecture.

5. `decision: native-platform-ci` Required CI executes the exhaustive full gate on native
Linux/amd64 and runs the complete native Go behavioral suite separately on macOS/arm64. The stable
required `CI / gate` conclusion depends on both lanes for the exact revision. The macOS lane does not
repeat coverage, lint, Pi, mutation, or other platform-independent suites. Each lane validates its
actual operating system and architecture before testing.

6. `decision: supported-release-platforms` Publish and cross-compile binaries for Linux/amd64,
Linux/arm64, macOS/amd64, and macOS/arm64. Windows is unsupported: do not publish or gate Windows
binaries, and remove Windows-only production and test implementations rather than retaining an
unverified dormant platform path.

7. `decision: range-qualified-mutation` Keep the `cmd/covercheck` mutation blocker qualified by an
exact change universe. Local end-of-implementation verification may use the materialized staged
candidate; pre-push, CI, and release callers supply every relevant exact push, review, or release
range. A full-gate name alone does not make broad or unconditional mutation testing acceptable.

## State changes

- remove `tooling/quality-gates:staged-test-selection`
- add `tooling/quality-gates:gate-tier-cadence`
- update `tooling/quality-gates:coverage-raw-identity-ratchet`
- update `tooling/quality-gates:coverage-ignore-admission`
- update `tooling/quality-gates:covercheck-mutation-regression`
- update `tooling/quality-gates:deadcode-gate`
- update `tooling/quality-gates:pi-extension-container-gate`
- update `tooling/quality-gates:gate-severity-by-protected-property`
- update `tooling/quality-gates:exact-revision-repository-acceptance`
- update `tooling/changelog-and-release:release-gate-on-tag`
- add `tooling/changelog-and-release:release-platforms`

## Consequences

Commit-time feedback remains blocking and useful without repeatedly paying for whole-repository
behavioral and platform assurance. Hook-aware instructions avoid duplicate manual and automatic
runs. Developers must choose focused iteration commands deliberately, while the terminal full gate
becomes a meaningful, explicit assurance event rather than background ceremony.

A local full run no longer claims cross-platform runtime assurance. Linux/amd64 and macOS/arm64 CI
provide the two native execution environments, while four release cross-builds prove the supported
artifact set compiles. The macOS job adds hosted runner cost, but it is focused on native Go behavior
and does not duplicate platform-independent suites. The stable required gate conclusion remains the
hosted acceptance boundary and succeeds only after both native lanes pass.

The new gate-tier cadence and release-platform claims describe repository-controlled behavior and
land as test-backed invariants with durable proof annotations.

Removing staged behavioral-suite selection simplifies the commit gate and makes the full gate
unambiguously exhaustive on its host. Mutation remains separately range-qualified because its
measured cost and narrow ownership contract do not support unconditional execution.

Windows users receive no binary or support claim. Removing the dormant Windows implementation also
removes its platform-only coverage evidence and reduces future maintenance, at the cost of an
intentional compatibility break in this pre-1.0 project.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep one staged-selecting gate | Continues to conflate commit feedback with terminal assurance and encourages repeated expensive runs. |
| Remove all static analysis from commit time | Loses high-value feedback even though the blocking checks are fast, cached, and deterministic. |
| Manually run gates before every Git operation | Immediately duplicates work when the corresponding hook is wired. |
| Run the full gate after every phase or review fix | Recreates the excessive cadence under a new command name. |
| Cross-compile tests for macOS without executing them | Proves compilation but not Darwin filesystem runtime behavior. |
| Repeat the complete full gate on macOS | Duplicates platform-independent assurance instead of adding only the missing native behavior evidence. |
| Stop Windows publication but retain its implementation and compile compatibility | Preserves an unverified maintenance surface after ending the support and artifact claim. |
| Continue publishing Windows binaries | Retains an unsupported platform surface with no native runtime assurance. |
| Run targeted mutation unconditionally in every full gate | Violates the measured narrow-change qualification and can add up to fifteen minutes without a relevant change. |

## Status history

- 2026-08-27: Proposed
- 2026-08-27: Implementing; content-sha256: b5ffcfb22ac1d81c82e3aac424f0b82a116083682834bf1c16bb4b74caf000b4
- 2026-08-27: Applied; operations: remove `tooling/quality-gates:staged-test-selection`, add `tooling/quality-gates:gate-tier-cadence`, update `tooling/quality-gates:coverage-raw-identity-ratchet`, update `tooling/quality-gates:coverage-ignore-admission`, update `tooling/quality-gates:covercheck-mutation-regression`, update `tooling/quality-gates:deadcode-gate`, update `tooling/quality-gates:pi-extension-container-gate`, update `tooling/quality-gates:gate-severity-by-protected-property`, update `tooling/quality-gates:exact-revision-repository-acceptance`, update `tooling/changelog-and-release:release-gate-on-tag`, add `tooling/changelog-and-release:release-platforms`
