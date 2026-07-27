{{=awf:sectionDefault}}

`awf context internal/project/context.go` is a representative concise orientation query: it renders each applicable topic once (claim-ID roster, directly marked claim detail, matched-path count with a coverage drilldown) and attributes each classified effective path. Use `awf context --full internal/project/context.go` whenever every applicable authority claim is required; the full detail still renders once per topic. `awf topic tooling/cli --coverage` drills into the owning-domain and topic selectors, their both-must-match rule, current matched paths, and marker sites. `--json` serializes the same selected projection for machine consumption (prefer the text form when reading); concise JSON omits the full block. Explicit ADR paths show lifecycle-derived operation progress without treating ADR prose as current authority. `./awf` forwards every awf verb verbatim; `./x` carries the repo-local project verbs; run `./x` without a command to list them.

In Pi, the local telemetry extension has no dashboard command, overlay, canonical refresh, process read, query tool, or maintenance control. Its muted below-editor line uses `[awf:init]`, `[awf:<phase>]`, `[awf:done]`, or `[awf:abandoned]` and updates only after a successful explicit lifecycle or association action. Totals sum each unique active-branch assistant message's public usage once; public context usage supplies only the current percentage and window without recharging history. A fresh session retains at most 256 provisional observations and 1 MiB in memory until `awf_workflow`, `/awf-resume-effort <effort-id>`, or external adoption settles identity. Use `/awf-resume-effort` only for a resident discovery or active effort. For a normalized external checkpoint with no resident effort, the agent invokes `awf_adopt_effort` alone with matching explicit memory path, effort, route, phase, and workflow fields; successful atomic adoption persists the association before returning the governed body.

Lightweight efforts are repository-local resident coordination state. `awf effort new <title>` creates an active schema-1 record and normalized memory by default; pass `--no-memory` for work that needs no checkpoint. Use `awf effort list`, `show <id>`, `rename <id> <title>`, `memory <id>`, `complete <id>`, `abandon <id>`, `reopen <id>`, and `repair <id>` to manage the available Phase 1 lifecycle. List, show, and repair accept `--json` where documented. `--worktree` is reserved and rejected until managed worktree safety ships; Phase 1 never creates one. The records are repository-local resident orchestration state, not durable project truth. Phase 1 does not yet manage ignore rules for the effort-record directory.

`awf context --uncovered [<scan-root>...]` reports eligible paths that are
unowned or covered by no scoped topic. Add `--staged` to evaluate the immutable
index universe with the same eligibility and coverage model as staged check.
`--full --uncovered` is refused because a coverage-gap report is not an authority
projection.

`awf check --staged` runs the same index-snapshot coverage and the HEAD-to-index
claim-transition handshake; the rendered pre-commit hook runs it. Selected workflow reports require an explicit resident effort: use `awf metrics
--effort <id>` for a footer-like selected-effort summary and `awf metrics doctor
--effort <id>` for severity, rule, and integrity counters; session, phase, and
time selectors combine with AND inside that effort and `--json` preserves its
canonical projection. The metrics summary includes current-path and all-work
usage/counters plus at most 10 deterministic per-phase turn, token, and cost
lines. Human doctor output never lists findings or raw evidence. `awf metrics list`
is the bounded unscoped discovery surface: it is newest-first, defaults to 10
rows, accepts at most 100, and continues only through its opaque cursor. An
incompatible resident effort is listed without projection details but cannot be
selected. Applying retention or confirmed
`awf metrics purge` is explicit maintenance, never an agent query action.
