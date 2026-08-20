### Rendered drift

Run `./awf check repo drift` and follow its repair hint. Edit the owning `.awf/` source, run `./awf render`, and recheck; never repair a generated output directly.

### Current-state refusal

Run `./awf check repo state`, then query the affected path with `./awf context <affected-path>`. Use the reported qualified topic with `./awf topic <domain>/<topic>`; change active claims only through their ADR lifecycle.

### Binary-version refusal

Use the repository `./awf` wrapper. If it still refuses, update the pinned awf binary through the documented upgrade flow; do not bypass the compatibility gate.

### Red gate

Run `./x test` for the Go failure and `./x gate` for the complete transaction. Fix the first failing stage or revert the change; do not weaken the check.
### Context spill recovery

On an exact two-line `AWF_CONTEXT_SPILL_V1` notice, read the file named on the second line and verify its byte length equals the `bytes=<decimal>` descriptor before using it. Best-effort delete that file after use, whether use succeeds or fails. Treat any other output as the context packet itself.

### Upgrade recovery and triage

Use `bash .awf/upgrade.sh <version>` for an exact upgrade. To trial a release without repinning, run `AWF_VERSION=<version> bash .awf/bootstrap.sh` and use the printed binary.

If the lock contains a bridge attestation, plain `awf upgrade` verifies the sealed HEAD and tree digest, journals deletion of the migration approval and replacement of the permanent lock, and discards the attestation's historical routing payload. A project that still needs this cutover cannot cross it without its attestation. If `.awf/current-state-upgrade.journal` is present, run only `./awf upgrade --recover`; precommit recovery rolls back and postcommit recovery cleans residue without reverting authority.

For an ordinary upgrade, inspect renderer provenance before checking and committing. `(template)` is upstream template churn, `(config)` is project configuration, `(template+config)` is both, `(internal)` and `(regenerated)` come from non-hashed inputs, and `added` is a newly shipped surface. Sweep project-owned references, stale convention-part prose, and newly rendered integration surfaces. A byte-identical render prints no provenance lines.