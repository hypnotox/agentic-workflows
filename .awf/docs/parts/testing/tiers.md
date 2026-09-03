awf uses focused iteration checks, one fast commit gate, and complete terminal or hosted verification. A wired pre-commit hook owns the fast gate; pre-push remains a focused preflight rather than an exhaustive local duplicate.

| Lane | Proves |
|---|---|
| Go | Unit, integration, regression, build, lint, optional dead-code, migration, publication, and pin checks. |
| Render | Exact fixed AWF skill publication, generated documentation, collisions, conservative pruning, and drift. |
| Platform-sensitive | Filesystem, Git, effort, worktree, and release-archive behavior on the supported native targets. |

`agentic-skills` and `pi-tools` are operator-managed packages with their own tests. AWF tests only its output and dependency boundary; it does not embed or behavior-test either external runtime.

The release-only interactive Pi smoke is manual verification, not a deterministic gate lane.
