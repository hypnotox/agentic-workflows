This topic records the current ownership contract for shared internal test-support infrastructure.

## Claims

### `invariant: test-support-leaf-boundary`

Non-test Go files under `internal/testsupport/**` may import the standard library and their own subpackages, with go-git additionally permitted only within `gitfixture`, but may not import another repository internal package.
Origin: ADR-0144
Backing: test
