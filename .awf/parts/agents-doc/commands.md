{{=awf:sectionDefault}}

`awf new topic <domain> "<title>"` scaffolds paired current-state metadata and an empty-claim authored
part without syncing. Replace the anchored path placeholder and generic prose, then author and review
claims manually. The command prints both repository-relative input paths and does not mutate config,
the lock, or rendered docs. `awf topic <domain>/<topic>[:<claim>]` queries active state read-only;
`--history`, `--references`, and `--coverage` independently add direct details, while `--json` changes
presentation only. `awf upgrade --check` evaluates bridge readiness without writing the worktree,
index, config, lock, or generated output; add `--json` for the stable exhaustive report. Readiness
requires the authored `.awf/current-state-migration.yaml` approval inventory, including
`version: 1` and `invariantApprovals: []` when no live legacy invariant maps. The check independently
derives each exact Origin/backing-preserving mapping before matching approval evidence.

`awf upgrade --attest-current-state` seals a ready, clean-HEAD prepared tree: it records the clean
HEAD, a post-normalization digest, and the ADR cutoff and gaps in an optional `bridgeAttestation` lock
block, journals every normalization, marker, status, and terminal legacy-index deletion at
`.awf/current-state-upgrade.journal`, and commits the attested lock last. Obtain and verify the
matching current-state binary before attesting. The four upgrade modes are mutually exclusive, and a
committed journal or attestation makes ordinary commands non-operational: only `awf upgrade --recover`
runs while a journal exists, only `awf upgrade --check` runs against an attested lock, a malformed
journal refuses every mode with Git-restoration guidance, and `awf version`/`awf changelog`/`awf help`
always bypass the transaction state. `awf upgrade --recover` replays the journal recovery table
idempotently, rolling an interrupted attestation back or cleaning up a committed one.
