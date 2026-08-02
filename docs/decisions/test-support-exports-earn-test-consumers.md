---
format: current-state-v3
slug: test-support-exports-earn-test-consumers
status: Proposed
date: 2026-08-02
---
# ADR-test-support-exports-earn-test-consumers: Test-support exports earn test consumers

## Context

`code-design/package-composition:export-earns-consumer` currently says every new or deliberately
converted exported symbol ships with an outside-package production consumer. It keeps
`export_test.go` legal but says a black-box `_test` package does not satisfy the consumer
requirement. That is correct for an export from a production package: a test must not enlarge the
production API merely to reach an implementation detail.

The wording also reaches dedicated shared test-support packages under `internal/testsupport`, where
it produces the opposite result. Those packages exist only to supply test fixtures and controlled
test dependencies across package boundaries. Their legitimate consumers are outside-package test
files, while production code is prohibited from importing them by the test-support dependency
boundary. Requiring a production consumer for such an export makes every new shared test fixture
impossible by construction.

This is not a request for test code to justify production-package surface. It is a correction that
makes the export test follow the purpose of the package declaring the symbol. The production/test
boundary remains strict: production packages earn exports through production consumers, and
dedicated test-support packages earn exports through test consumers. The pending filesystem seam
needs this correction before it can place one fault-injectable implementation in a shared
`internal/testsupport` home rather than duplicating local test fakes.

## Decision

1. Update `code-design/package-composition:export-earns-consumer` so its outside-package
   production-consumer requirement applies to an export declared by a production package. A
   black-box `_test` package remains unable to justify that production export, and `export_test.go`
   remains the legal in-package test seam it is today.

2. For an export declared by a dedicated shared test-support package under
   `internal/testsupport/**`, require at least one outside-package test consumer in the same green
   transaction. The test consumer must use the exported capability for its actual fixture,
   controlled-dependency, or assertion setup; a compile-only reference does not earn the export.

3. Keep test support one-way. Production Go files outside `internal/testsupport/**` must not import a
   test-support package. The existing test-support leaf boundary continues to govern what shared
   test support itself may import; this decision creates no production dependency on test code and
   no exception to that boundary.

4. Apply the claim update as one checked current-state transaction. Its verification inspects each
   new or deliberately converted export according to the declaring package: find an
   outside-package production consumer for a production package, or an outside-package test
   consumer for a dedicated shared test-support package, and reject any production import of test
   support.

## State changes

- update `code-design/package-composition:export-earns-consumer`

## Consequences

Shared test infrastructure can expose the minimum API its real test consumers need without inventing
a production caller or copying the fixture into each consumer package. The same concrete-first
pressure remains: the outside-package test consumer lands with the export, so test-support APIs are
not added for anticipated reuse.

Production API discipline does not weaken. A production package still cannot export a symbol merely
because an external test wants it, and a black-box test remains irrelevant to the production
consumer check. The distinction is determined by the package that owns the export, not by a broad
"tests may consume exports" exemption.

Review must classify the declaring package correctly. `internal/testsupport/**` is a deliberate
boundary, so an arbitrary package used mostly by tests does not qualify. Existing dependency checks
continue to prevent production code from turning the shared fixture home into a runtime dependency.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Require a production consumer for every exported symbol without exception | Makes a shared test-support package unable to add any usable API, contradicting its sole purpose. |
| Let any outside-package test justify any export | Allows tests to enlarge production APIs and removes the production-consumer discipline the claim was created to enforce. |
| Keep every fixture local to its consuming package | Duplicates shared test concerns and conflicts with the single-home rule once more than one package needs the fixture. |
| Put configurable test fakes in production packages | Ships test-only capability in the runtime surface and still lacks a production consumer for that capability. |

## Status history

- 2026-08-02: Proposed
