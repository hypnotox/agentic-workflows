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
