- `internal/project`: configuration, rendering, output planning, and repository checks.
- `internal/adr`, `internal/currentstate`, and `internal/plan`: decision, active-authority, and plan models.
- `internal/effort` and `internal/worktree`: local residents and Git-backed topology.
- `internal/git`: the sole semantic Git seam.
- `internal/snapshot` and `internal/filesystem`: immutable input and confined filesystem boundaries.
- `cmd/awf`: CLI composition root.
- Generated Pi extensions: context observation, handoff, and subagent dispatch.


### Commands

| Command | Role |
|---|---|
| `cmd/awf` | CLI composition root. |
| `cmd/contextspilllog` | Context spill logging. |
| `cmd/covercheck` | Coverage checking. |
| `cmd/deadcodecheck` | Dead-code checking. |
| `cmd/mutants` | Mutation-test reporting. |
| `cmd/pincheck` | Dependency pin checking. |
| `cmd/releasecheck` | Release validation. |
| `cmd/repoaudit` | Repository audit support. |
