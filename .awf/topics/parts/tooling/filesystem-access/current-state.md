This topic records root-confined path behavior and project mutation lease safety.

## Claims

### `invariant: root-confined-paths`

Root-confined operations accept only valid slash-relative paths beneath the selected root, refuse absolute, parent, and escaping-symlink access, return slash-relative walk paths without following directory symlinks, and preserve wrapped error identity. A read-with-mode result returns bytes and permissions from one observed file generation even while the path is atomically replaced.
Backing: test

### `invariant: root-scoped-project-mutation-leases`

Project mutation leases canonicalize existing roots including symlink aliases, retain restrictive user-cache lock files, order complete scope-and-root identities even when roots match, wait with context cancellation, and release explicitly or on process exit. Distinct tracked and resident scopes let linked checkouts remain independently mutable while a shared resident root serializes cross-checkout mutation.
Backing: test
