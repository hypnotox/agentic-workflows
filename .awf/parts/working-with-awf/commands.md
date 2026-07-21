{{=awf:sectionDefault}}

`awf context <path>...` is concise orientation: it attributes each effective path,
shows directly marked claims, and names omitted topic-wide claims with drilldowns.
Use `awf context --full <path>...` whenever every applicable authority claim is
required. `--json` serializes the same selected projection; concise JSON omits the
full block. Explicit ADR paths show lifecycle-derived operation progress without
treating ADR prose as current authority.

`awf context --uncovered [<scan-root>...]` reports eligible paths that are
unowned or covered by no scoped topic. Add `--staged` to evaluate the immutable
index universe with the same eligibility and coverage model as staged check.
`--full --uncovered` is refused because a coverage-gap report is not an authority
projection.
