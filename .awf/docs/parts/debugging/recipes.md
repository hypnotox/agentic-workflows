### Rendered drift

Run `./awf check repo drift` and follow its repair hint. Edit the owning `.awf/` source, run `./awf render`, and recheck; never repair a generated output directly.

### Current-state refusal

Run `./awf check repo state`, then query the affected path with `./awf context <affected-path>`. Use the reported qualified topic with `./awf topic <domain>/<topic>`; change active claims only through their ADR lifecycle.

### Binary-version refusal

Use the repository `./awf` wrapper. If it still refuses, update the pinned awf binary through the documented upgrade flow; do not bypass the compatibility gate.

### Red gate

Run `./x test` for the Go failure and `./x gate` for the complete transaction. Fix the first failing stage or revert the change; do not weaken the check.