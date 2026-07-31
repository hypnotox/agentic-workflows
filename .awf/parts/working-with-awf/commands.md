A minimal simple fix uses no effort. For a concrete non-minimal outcome, run `awf effort new "<outcome>"`; the immutable slug identifies `.awf/efforts/<slug>/state.json`, its always-owned `.awf/efforts/<slug>/memory.md`, `.awf/worktrees/<slug>/`, and the `awf/<slug>` branch. Creation makes the managed worktree by default (`--no-worktree` opts out; `--base <ref>` selects the base); `awf effort worktree add <slug>` remains the standalone operation for efforts created without one. Git topology, not effort state, owns integration and removal facts; finish is restartable deletion and refuses until every managed path, registration, and branch is absent.

Pi's `handoff_session` accepts only the exact repository-relative `.awf/efforts/<slug>/memory.md` path or an absolute spelling that normalizes to it. It validates the slug, confinement, ownership, bounded UTF-8 regular-file identity, and repository identity without selecting an effort or mutating lifecycle state.

For `awf context`, bare directories provide tier-0 census, compact grouping, provenance, topic counts, and bounded pending orientation; bare exact, staged, and range-selected files additionally provide tier-1 `State`, `Touches`, and `Proofs` relationships from actual markers. The eight named facets expand directory relationships, non-direct authority, evidence, selectors, references, pending operations, or artifacts; only `artifacts` refines groups, and `--full` is their union. Output above 8,192 bytes retains secure caller-owned spill delivery.

### Context spill notices

When `awf context` output would exceed 8,192 bytes, the report securely spills outside the
repository and the command returns exactly a two-line `AWF_CONTEXT_SPILL_V1` notice. On that
exact notice, read the file named on its second line and verify that its byte length equals
the `bytes=<decimal>` descriptor before treating its contents as the context packet.
Best-effort delete the named file after packet use, whether packet use succeeds or fails.
Treat any other output as the context packet itself; do not interpret a near-match as a
spill notice. This subsection is the contract's single rendered home; skills and agent
bodies point here.
