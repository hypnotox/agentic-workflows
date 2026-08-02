---
format: current-state-v3
slug: test-support-exports-earn-test-consumers
status: Implemented
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

The same overbreadth appears in
`code-design/dependency-composition:concrete-first-consumer`: its verification requires production
callers for every shared composition symbol. A controlled dependency exported by dedicated test
support has no legitimate production caller, but it still needs the claim's same-transaction,
non-speculative first-consumer discipline. Export eligibility and composition capability therefore
need the same production-versus-test symmetry.

## Decision

1. Update `code-design/package-composition:export-earns-consumer` so its outside-package
   production-consumer requirement applies to an export declared by a production package. A
   black-box `_test` package remains unable to justify that production export, and `export_test.go`
   remains the legal in-package test seam it is today.

2. For an export declared by a dedicated shared test-support package under
   `internal/testsupport/**`, require at least one outside-package test consumer in the same green
   transaction. The test consumer must use the exported capability for its actual fixture,
   controlled-dependency, or assertion setup; a compile-only reference does not earn the export.

3. Update `code-design/dependency-composition:concrete-first-consumer` with the same symmetry. A
   production composition capability requires one named concrete production first consumer, while
   a composition capability exported by dedicated `internal/testsupport/**` requires one named
   outside-package test first consumer. In both cases the capability and consumer land in the same
   green transaction, the consumer uses the whole introduced capability, and anticipated reuse
   earns no API.

4. Keep test support one-way through a separate
   `tooling/test-infrastructure:production-never-imports-test-support` claim. Production Go files
   outside `internal/testsupport/**` must not import a test-support package. The existing
   test-support leaf boundary continues to govern what shared test support itself may import; this
   decision creates no production dependency on test code and no exception to that boundary.

5. Back the new one-way claim with a repository import scan that fails when a non-test Go file
   outside `internal/testsupport/**` imports the root test-support package or any of its subpackages.
   Apply the claim addition and both production-versus-test updates as one checked current-state
   transaction. Verification classifies each declaring package and caller before applying the
   corresponding production or dedicated-test-support requirement.

## State changes

- update `code-design/package-composition:export-earns-consumer`
- update `code-design/dependency-composition:concrete-first-consumer`
- add `tooling/test-infrastructure:production-never-imports-test-support`

## Consequences

Shared test infrastructure can expose the minimum API its real test consumers need without inventing
a production caller or copying the fixture into each consumer package. Concrete-first pressure
remains explicit and symmetric: the outside-package test consumer lands with the export, uses the
whole capability, and prevents test-support APIs from being added for anticipated reuse.

Production API discipline does not weaken. A production package still cannot export a symbol merely
because an external test wants it, and a black-box test remains irrelevant to the production
consumer check. The distinction is determined by the package that owns the export, not by a broad
"tests may consume exports" exemption.

Review must classify the declaring package correctly. `internal/testsupport/**` is a deliberate
boundary, so an arbitrary package used mostly by tests does not qualify. The new repository import
scan prevents production code from turning the shared fixture home into a runtime dependency.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Require a production consumer for every exported symbol without exception | Makes a shared test-support package unable to add any usable API, contradicting its sole purpose. |
| Let any outside-package test justify any export | Allows tests to enlarge production APIs and removes the production-consumer discipline the claim was created to enforce. |
| Keep this fault fixture local to upgrade tests | Hides the kernel-backed fault-source contract inside its first consumer instead of placing the real outside-package test consumer against the repository's designated shared test-support home. |
| Put configurable test fakes in production packages | Ships test-only capability in the runtime surface and still lacks a production consumer for that capability. |

## Status history

- 2026-08-02: Proposed
- 2026-08-02: Accepted; content-sha256: 498dc41c72649dd547e4ed9398153e2a75c7094f0966a2e533cc61765d381700
- 2026-08-02: Implemented; content-sha256: 498dc41c72649dd547e4ed9398153e2a75c7094f0966a2e533cc61765d381700
