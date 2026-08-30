### Rendered drift

Run `./awf check repo drift` and follow its repair hint. Edit the owning `.awf/` source, run `./awf render`, and recheck; never repair a generated output directly.

### Current-state refusal

Run `./awf check repo state`, then query the affected path with `./awf resolve topic <affected-path>`. Use the reported qualified topic with `./awf read topic <domain>/<topic>`; edit the owning topic source and reconcile its evidence with repository reality.

### Binary-version refusal

Use the repository `./awf` wrapper. If it still refuses, update the pinned awf binary through the documented upgrade flow; do not bypass the compatibility gate.

### Red gate

Run `./x test` for a Go failure, the narrowest relevant command while iterating. Fix the first failing stage or revert the change; do not weaken the check.
### Upgrade recovery and triage

With bootstrap enabled, `bash .awf/upgrade.sh` upgrades to the newest release and `bash .awf/upgrade.sh <version>` selects an exact version. The script checksum-verifies its bootstrap handoff, runs `./awf upgrade`, and re-pins bootstrap. To trial a release without repinning, run `AWF_VERSION=<version> bash .awf/bootstrap.sh` and use the printed binary.

Live authority starts at schema 46. A below-floor or retired layout is refused before decoding or mutation; use a release that supports that source, then restore or adopt the supported `.awf/` control pair. If `.awf/current-state-upgrade.journal` is present, run only `./awf upgrade --recover`; precommit recovery rolls back and postcommit recovery cleans residue without reverting authority.

For an ordinary upgrade, inspect renderer provenance before checking and committing. `(template)` is upstream template churn, `(config)` is project configuration, `(template+config)` is both, `(internal)` and `(regenerated)` come from non-hashed inputs, and `added` is a newly shipped surface. Sweep project-owned references, stale convention-part prose, and newly rendered integration surfaces. Query adopter-facing changes with `awf changelog --since <previous version>`. A first sync and a byte-identical render print no provenance lines.