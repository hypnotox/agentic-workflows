The three repository-wide resident roots are `.awf/efforts`, `.awf/memory`, and `.awf/worktrees`. Rendering governs only each root's self-ignoring `.gitignore`; dynamic descendants are local state preserved by render, drift checks, and uninstall. Schema generation 21 removes obsolete metrics and assignment residents during upgrade.

Plan execution selects `inline` or `subagent-driven` ownership independently per phase. One
commit-capable owner takes a complete subagent-driven phase from a clean green baseline through its
staged check, gate, and closing commit; the parent owns inline phases, integration, report-only
review settlement, and the settled-phase checkpoint. Optional batch helpers are sequential and
commit-disabled, receive path-disjoint subsets, and never own shared files or the closing commit. A
dirty stop is inventoried before the parent completes inline, restores and restarts the complete
phase, or transfers the complete revised phase with completed and remaining work plus recovery
verification. Checkbox tasks and helper returns are not transaction or checkpoint boundaries, and a
blind task-level successor is forbidden. Every governed subagent dispatch chooses the smallest model expected to complete reliably: `small` is for narrow, mechanical, low-ambiguity work; `standard` is for substantive but bounded work; and `large` is for broad, intricate, cross-cutting, or high-consequence work. Uncertainty, failed reasoning, or widened scope requires reconsideration and possible escalation. A runtime with model selection chooses explicitly; an unsupported runtime uses its harness default and notes that explicit selection is unavailable. In Pi, omission uses the configured role default and an exact tier reference is supplied only for a deliberate override.
