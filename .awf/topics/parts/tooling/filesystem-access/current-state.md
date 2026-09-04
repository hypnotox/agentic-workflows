This topic records root-confined path behavior and project mutation lease safety.

## Claims

### `invariant: root-confined-paths`

Root-confined operations accept only valid slash-relative paths beneath the selected root, refuse absolute and parent access, do not follow symlinks in any path component, and refuse a symlink at the final mutation path even when it resolves in-root. Walks return slash-relative paths without following directory symlinks, and wrapped error identity is preserved. A read-with-mode result returns bytes and permissions from one observed file generation even while the path is replaced. Expected file replacement validates the observed identity and optional exact bytes and mode, publishes a complete same-directory temporary with rename, and makes no per-file fsync promise. Expected removal revalidates identity and exact content where supplied, directly unlinks files, and removes directories only when empty; no recursive expected-tree retirement remains.
Backing: test

### `invariant: root-scoped-project-mutation-leases`

Project mutation leases canonicalize existing roots including symlink aliases, retain restrictive user-cache lock files, order complete scope-and-root identities even when roots match, wait with context cancellation, and release explicitly or on process exit. Distinct tracked and resident scopes let linked checkouts remain independently mutable while a shared resident root serializes cross-checkout mutation.
Backing: test

### `invariant: focused-project-mutations-visible`

Focused operations acquire their selected tracked-only or complete project lease before mutable-authority reads, use confined authority access, validate before mutation, and stop at the first failure. Operations that render reload one fresh committed Session before at most one Publisher synchronization attempt. They leave earlier successful paths visible, report written or removed files, created or corrected directories, and the final lock among the affected paths for inspection and rerun, and do not roll back. Tracked-only topic scaffolding never synchronizes.
Backing: test
