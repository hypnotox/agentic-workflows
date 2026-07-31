Rendering recognizes exactly two repository-wide resident roots, `.awf/efforts` and `.awf/worktrees`, matching the only state protocol 2 keeps: `.awf/efforts/<slug>/` and `.awf/worktrees/<slug>/`. There is no standalone memory root; an effort's memory lives inside the effort that owns it. Rendering governs only each root's self-ignoring `.gitignore`; dynamic descendants are local state that render, drift checks, sweep, and uninstall preserve without recursing into. Schema generation 21 removes obsolete metrics and assignment residents during upgrade, and generation 22 resets protocol-1 effort records and standalone `.awf/memory/` content rather than migrating them. That reset is journaled with the lock replacement as its commit point, so it refuses beforehand, and changes nothing, while any legacy managed worktree path, registration, or branch remains.

A minimal simple fix uses no effort. A concrete non-minimal outcome uses exactly one immutable slugged effort whose memory is `.awf/efforts/<slug>/memory.md`, with one user-managed writer. Repository authority outranks the checkpoint. Worktree-backed efforts integrate after terminal review, renew review after a divergent merge, remove all managed topology, run retrospective, and finish last.

Plan execution selects `inline` or `subagent-driven` ownership independently per phase. One
commit-capable owner takes a complete subagent-driven phase from a clean green baseline through its
staged check, gate, and closing commit; the parent owns inline phases, integration, report-only
review settlement, and the settled-phase checkpoint. Optional batch helpers are sequential and
commit-disabled, receive path-disjoint subsets, and never own shared files or the closing commit. A
dirty stop is inventoried before the parent completes inline, restores and restarts the complete
phase, or transfers the complete revised phase with completed and remaining work plus recovery
verification. Checkbox tasks and helper returns are not transaction or checkpoint boundaries, and a
blind task-level successor is forbidden. Every governed subagent dispatch chooses the smallest reliable tier - `small` (narrow, mechanical), `standard` (substantive but bounded), or `large` (broad, intricate, cross-cutting, or high-consequence) - escalating after uncertainty, failed reasoning, or widened scope; the full tier definitions live in the agent guide's workflow section. In Pi, omission uses the configured role default and an exact tier reference is supplied only for a deliberate override.
