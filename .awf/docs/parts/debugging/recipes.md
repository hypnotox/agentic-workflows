### Rendered drift

Run `./awf check repo drift` and follow its repair hint. Edit the owning `.awf/` source, run `./awf render`, and recheck; never repair a generated output directly.

### Current-state refusal

Run `./awf check repo state`, then query the affected path with `./awf resolve topic <affected-path>`. Use the reported qualified topic with `./awf read topic <domain>/<topic>`; edit the owning topic source and reconcile its evidence with repository reality.

### Binary-version refusal

Use the repository `./awf` wrapper. If it still refuses, update the pinned awf binary through the documented upgrade flow; do not bypass the compatibility gate.

### Red gate

Run `./x test` for a Go failure, the narrowest relevant command while iterating. Fix the first failing stage or revert the change; do not weaken the check.
### Upgrade inspection and retry

With bootstrap enabled, `bash .awf/upgrade.sh` upgrades to the newest release and `bash .awf/upgrade.sh <version>` selects an exact version. The script checksum-verifies its bootstrap handoff, runs `./awf upgrade`, and re-pins bootstrap. To trial a release without repinning, run `AWF_VERSION=<version> bash .awf/bootstrap.sh` and use the printed binary.

Live authority starts at schema 50, and the temporary schema-50-through-53 bridge remains until external managed adopters complete rollout. A below-floor or retired layout is refused before decoding or mutation; first use AWF 0.44 to reach schema 50, then adopt the supported `.awf/` control pair. A failed upgrade stops at its first failed mutation and leaves earlier successful effects visible. Inspect the reported paths with `git status --short` and `git diff`, correct the blocking condition, and rerun `./awf upgrade`.

For an ordinary upgrade, inspect renderer provenance before checking and committing. `(template)` is upstream template churn, `(config)` is project configuration, `(template+config)` is both, `(internal)` and `(regenerated)` come from non-hashed inputs, and `added` is a newly shipped surface. Sweep project-owned references, stale convention-part prose, and newly rendered integration surfaces. Query adopter-facing changes with `awf changelog --since <previous version>`. A first sync and a byte-identical render print no provenance lines.