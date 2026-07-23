{{=awf:sectionDefault}}

`awf metrics --json` prints the canonical repository-wide metrics projection. Narrow it with
`--effort`, `--session`, `--phase`, `--since`, and `--until`; export the same projection with
`awf metrics export --format json`, or validated normalized ledger events with `--format jsonl`.
`awf doctor --json` applies exact rules plus the configured heuristics to the same selector and is
read-only and advisory: effort-owned findings do not change its exit status. `awf metrics retain --dry-run --json`
previews deterministic terminal-effort retention; applying retention or confirmed `awf metrics purge`
is explicit maintenance, never an agent query action. Lifecycle, repair, and waiver writes use the closed
`awf metrics lifecycle --request <FILE|-> --json` contract and fail unless durably appended. Protocol
2 normal chain edges use one `transition-phase` request to close the named start and enter its
successor, optionally with a route effect. Repair and waiver requests must use the selected finding's
owning effort and current nonempty causal frontier; stale, cross-effort, mismatched, or ineligible
input is rejected without append.

Before every commit, stage the complete transaction, run `awf check --staged`, then run `./x gate`.
Commit only after both commands pass. The pre-commit hook repeats the staged check as defense in depth;
it does not replace the agent's explicit validation.

`awf new topic <domain> "<title>"` scaffolds paired current-state metadata and an empty-claim authored
part without syncing. Replace the anchored path placeholder and generic prose, then author and review
claims manually. The command prints both repository-relative input paths and does not mutate config,
the lock, or rendered docs. `awf topic <domain>/<topic>[:<claim>]` queries active state read-only;
`--history`, `--references`, and `--coverage` independently add direct details, while `--json` changes
presentation only. Start with `awf context internal/project/context.go` for topic-grouped orientation: each applicable topic renders once with its claim-ID roster, directly marked claim detail, and matched-path count, while effective paths carry classification and attribution; use `awf context --full internal/project/context.go` for the complete applicable authority packet, once per topic, without recursively expanding references or history. Use `awf topic tooling/cli --coverage` to inspect separate domain/topic selectors, current matched paths, and marker sites. `--json` serializes the selected projection for machine consumption (agents should read the text form), and concise JSON has no full block. `awf context --uncovered` reports unowned paths and per-domain uncovered coverage and refuses `--full`; `--staged` evaluates the Git index instead of the working tree. Explicit ADR paths report lifecycle-derived operation progress while treating ADR prose only as pending intent or decision history. Run `./x` without a command to discover the metadata-derived forwarded awf verbs and project-owned verbs available through the runner. `awf check --staged` runs the same index-snapshot
coverage and the HEAD-to-index claim-transition handshake; the rendered pre-commit hook runs it.

`awf upgrade` migrates the `.awf/` config tree to the current schema and syncs. When the lock carries
a bridge attestation, plain `awf upgrade` instead performs the final current-state cutover: it verifies
only the sealed facts (the prepared HEAD and tree digest), journals the deletion of the migration
approval file and the permanent lock, and promotes the sealed format cutoff and gaps. Attestation and
readiness reporting live only in the preceding bridge release; this binary consumes seals, it never
produces them. `awf upgrade --recover` replays the current-state upgrade journal recovery table,
rolling an interrupted cutover back or cleaning up a committed one, and is the only mode a committed
journal permits.
