The awf binary owns lightweight repository-local effort records and their optional resident resources. These records coordinate local work without replacing tracked project authority.

## Claims

### `invariant: effort-record-authority`

The awf binary alone allocates lowercase UUIDv4 effort IDs and owns schema-1 repository-local effort records and optional memory and managed-worktree state. Records contain no Pi-session assignment; creation, rename, lifecycle, and repair retain their existing resident-state authority without replacing Git-tracked project truth.
Origin: ADR-0164
Revised-by: ADR-0167
Backing: test

### `invariant: managed-worktree-lifecycle`

Managed effort worktrees use the fixed `.awf/worktrees/<effort-id>/` path and `awf/<effort-id>` branch, resolve an explicit base or caller HEAD through native Git, and retain the schema-1 integration state machine through integration and until explicit removal. Native-Git operations validate registration, repository identity, branch, operation state, cleanliness, and ancestry before mutation; confinement, symlink, ownership, repository-identity, merge-conflict, and destructive-topology safety refusals are never forceable, while only recoverable cleanliness or non-destructive topology risk accepts paired `--force --reason <nonblank>`. Integration is explicit as fast-forward, merge, or manually recorded only after its required ancestry proof, and completion never removes a managed worktree.
Origin: ADR-0164
Backing: test
