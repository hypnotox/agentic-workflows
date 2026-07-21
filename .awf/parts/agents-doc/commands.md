{{=awf:sectionDefault}}

Before every commit, stage the complete transaction, run `awf check --staged`, then run `./x gate`.
Commit only after both commands pass. The pre-commit hook repeats the staged check as defense in depth;
it does not replace the agent's explicit validation.

`awf new topic <domain> "<title>"` scaffolds paired current-state metadata and an empty-claim authored
part without syncing. Replace the anchored path placeholder and generic prose, then author and review
claims manually. The command prints both repository-relative input paths and does not mutate config,
the lock, or rendered docs. `awf topic <domain>/<topic>[:<claim>]` queries active state read-only;
`--history`, `--references`, and `--coverage` independently add direct details, while `--json` changes
presentation only. `awf context <paths>` answers which current-state claims apply to those paths;
`awf context --uncovered` reports unowned paths and per-domain uncovered coverage, and `--staged`
evaluates the Git index instead of the working tree. `awf check --staged` runs the same index-snapshot
coverage and the HEAD-to-index claim-transition handshake; the rendered pre-commit hook runs it.

`awf upgrade` migrates the `.awf/` config tree to the current schema and syncs. When the lock carries
a bridge attestation, plain `awf upgrade` instead performs the final current-state cutover: it verifies
only the sealed facts (the prepared HEAD and tree digest), journals the deletion of the migration
approval file and the permanent lock, and promotes the sealed format cutoff and gaps. Attestation and
readiness reporting live only in the preceding bridge release; this binary consumes seals, it never
produces them. `awf upgrade --recover` replays the current-state upgrade journal recovery table,
rolling an interrupted cutover back or cleaning up a committed one, and is the only mode a committed
journal permits.
