The awf binary owns lightweight repository-local effort records and their optional resident resources. These records coordinate local work without replacing tracked project authority.

## Claims

### `invariant: effort-record-authority`

The awf binary derives an immutable 1-63 byte ASCII slug from each outcome, allocates an internal lowercase UUIDv4, and durably publishes schema-2 `.awf/efforts/<slug>/state.json` only after its always-owned `memory.md`. Directory presence is the active-effort fact; listing ignores unpublished incomplete directories and preserves malformed or foreign residents, while restartable finish renames to a slug-and-UUID-matched tombstone and deletes only proven bytes after managed topology is absent. Efforts have no lifecycle ledger, coordination lock, Pi-session assignment, standalone memory, or stored worktree state, and Git-tracked project truth remains authoritative.
Origin: ADR-0164
Revised-by: ADR-0167, ADR-0175
Backing: test

### `invariant: managed-worktree-lifecycle`

Managed effort worktrees are stateless native-Git utilities at `.awf/worktrees/<slug>/` on `awf/<slug>`. Add is separate from effort creation and leaves the complete effort unchanged on failure. Integration revalidates clean target, registration, repository identity, branch, operation state, and ancestry immediately before mutation; it reports already-contained history, fast-forwards, or starts a divergent `--no-commit` merge, never tests, reviews, commits, pushes, resolves, removes, records disposition, or finishes. Remove independently inspects path, registration, and branch on every retry, requires cleanliness and target ancestry, and uses no awf force-discard path; intentional discard stays explicit native Git.
Origin: ADR-0164
Revised-by: ADR-0175
Backing: test
