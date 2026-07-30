## Procedure

1. **Validate closure context.** Carry the exact effort slug and `.awf/efforts/<slug>/memory.md`, confirm `Effort: <slug>`, and remain the one user-managed writer. Repository sources and current-state documentation outrank checkpoint prose; standalone memory is forbidden.

2. **Reflect and record worthy observations.** Read the effort memory's `## Observations` and `## Decision log` as primary input alongside this session's implementation and terminal-review findings and friction, and confirm every user-provenance decision either landed in a durable artifact or was explicitly re-decided. Record a first occurrence at the appropriate durable rung; recording observations in memory during the effort is the expected path, and a lesson must land at a durable rung before finish deletes the file.

3. **Promote recurring, codifiable observations** to the strongest justified rung. Verify recurrence before promoting (within this session or across the effort's sessions as recorded in the observation log) and land any pitfall, invariant, deterministic check, or reviewer-focus change with its required render, staged check, gate, and commit.

4. **Update adopter-facing records.** Confirm adopter-visible behavior is recorded under `## [Unreleased]` in `changelog/CHANGELOG.md`, grouped by Breaking changes / Features / Bug fixes / Others, and note where each lesson landed.

5. **Verify managed topology is absent.** Require no `.awf/worktrees/<slug>` path, native Git registration, or `awf/<slug>` branch. Integration and removal belong after terminal review and before retrospective; retrospective never discards Git resources.

6. **Finish last.** Update the final checkpoint, then run `awf effort finish <slug>` and report its changed active-rename and cleanup status. Never delete memory directly. Finish is the last effort mutation and occurs only after every durable lesson or changelog correction is committed.
