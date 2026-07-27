The awf binary owns lightweight repository-local effort records and their optional resident resources. These records coordinate local work without replacing tracked project authority.

## Claims

### `invariant: effort-record-authority`

The awf binary is the only allocator of lowercase UUIDv4 effort IDs and owns schema-1 current-state records at stable ID-derived paths, replacing each record atomically under a repository-local lock. A valid record has a trimmed nonblank UTF-8 title of at most 160 bytes; active, completed, or abandoned state; immutable UTC creation time and monotonic UTC update time; filesystem-truth memory presence; an exact legal worktree/integration pair; and sorted assigned session IDs joined logically from the separate assignment authority rather than duplicated in the persisted record. Creation defaults to normalized effort-owned memory, rename changes display metadata only, lifecycle changes retain memory and worktree metadata, reopen accepts either terminal state, and repair preserves corrupt input while changing and reporting only facts derivable from confined resident state. This binary-owned state is repository-local orchestration authority only; Git-tracked code, ADRs, plans, and documentation remain durable project truth and are never governed by an effort record.
Origin: ADR-0164
Backing: test

### `invariant: managed-worktree-lifecycle`

Managed effort worktrees use the fixed `.awf/worktrees/<effort-id>/` path and `awf/<effort-id>` branch, resolve an explicit base or caller HEAD through native Git, and retain the schema-1 integration state machine through integration and until explicit removal. Native-Git operations validate registration, repository identity, branch, operation state, cleanliness, and ancestry before mutation; confinement, symlink, ownership, repository-identity, merge-conflict, and destructive-topology safety refusals are never forceable, while only recoverable cleanliness or non-destructive topology risk accepts paired `--force --reason <nonblank>`. Integration is explicit as fast-forward, merge, or manually recorded only after its required ancestry proof, and completion never removes a managed worktree.
Origin: ADR-0164
Backing: test
