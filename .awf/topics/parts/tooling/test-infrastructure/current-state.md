This topic records observable safety for reusable test fixtures.

## Claims

### `invariant: immutable-fixture-seeds`

Expensive reusable test fixtures are captured as immutable representations at their narrowest package owner, and every mutating consumer receives a distinct clone. Cloning preserves file modes, symbolic links, and Git state without relying on filesystem-specific copy correctness; tests never share a live mutable root.
Backing: test
