---
format: current-state-v4
slug: bound-native-git-fixtures
status: Proposed
date: 2026-08-10
---
# ADR-0261: Bound Native Git Fixtures


## Context

The native Git fixture lane constructs repository states that go-git cannot express. Its
subprocesses run under an isolated environment, but both its ordinary and byte-oriented Git
runners use `exec.Command` without a deadline. A blocked fixture therefore hangs until the
Go test binary's timeout, obscuring the responsible invocation and delaying every affected
gate run.

ADR-0193 accepted that divergence from the production Git seam because a fixture serves no
caller that could supply a bounded context. The fixture boundary can instead own a fixed
hang-prevention ceiling without changing its exported API or threading contexts through
all fixture helpers. The test-support leaf boundary prevents importing the production
seam's timeout constant, just as it prevents sharing the environment-isolation policy.

## Decision

1. `decision: bound-every-fixture-git-process` Every native Git subprocess used to construct or inspect fixture state has a fixed hang-prevention deadline owned by the fixture boundary. Both string-oriented and byte-oriented invocations use the same deadlined execution path, so an exported helper cannot accidentally select an unbounded lane.
2. `decision: match-the-native-git-ceiling` The fixture deadline matches the production native Git command ceiling. The value remains a documented duplicate because the test-support leaf boundary forbids importing the production seam; a focused test proves that a blocked fixture process is terminated by the deadline.

## State changes

- update `tooling/git-access:fixture-isolation-parity`

## Consequences

A stalled fixture fails at its own invocation rather than consuming the test binary's full
timeout. Existing fixture callers and their backend-neutral API remain unchanged.

A healthy Git operation that exceeds the shared ceiling is terminated, but the ceiling is
intentionally far above observed fixture work. The timeout value remains duplicated across
the production seam and test support and must be kept equal by hand under the existing leaf
boundary.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep fixture Git unbounded | It preserves the hanging-test hazard and leaves diagnosis to the much later test-binary timeout. |
| Thread caller contexts through every fixture helper | The fixture boundary can own one uniform ceiling without widening every helper signature or requiring callers to make inconsistent timeout choices. |
| Bound only the ordinary string runner | Byte-oriented fixture operations would retain the same hang and create an API-dependent safety gap. |

## Status history

- 2026-08-10: Proposed
