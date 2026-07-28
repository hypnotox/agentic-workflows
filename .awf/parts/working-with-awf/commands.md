Use `awf effort new "<outcome>"` for optional durable coordination. `awf effort` creates memory by default and manages opt-in worktrees. Efforts do not carry Pi-session state.

Pi's `handoff_session` accepts a canonical repository-relative memory path or an absolute path that resolves to a regular file beneath this repository's `.awf/memory/` directory. Accepted paths are normalized to repository-relative slash form before handoff.
