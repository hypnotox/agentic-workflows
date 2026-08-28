This topic records the current ownership contract and both directions of the dependency boundary for shared internal test-support infrastructure.

## Claims

### `invariant: test-support-leaf-boundary`

Non-test Go files under `internal/testsupport/**` may import the standard library and their own subpackages, with go-git additionally permitted only within `gitfixture`, but may not import another repository internal package.
Origin: ADR-0144
Backing: test

### `invariant: immutable-fixture-seeds`

Expensive reusable test fixtures are captured as immutable representations at their narrowest package owner, and every mutating consumer receives a distinct clone. Cloning preserves file modes, symbolic links, and Git state without relying on filesystem-specific copy correctness; tests never share a live mutable root.
Origin: ADR-0314
Backing: test

### `invariant: production-never-imports-test-support`

Non-test Go files outside `internal/testsupport/**` never import the root test-support package or any of its subpackages; shared test fixtures remain a test-only dependency in the direction from outside-package tests into test support.
Origin: ADR-0215
Backing: test
