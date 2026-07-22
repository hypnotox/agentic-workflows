{{=awf:sectionDefault}}

`awf context internal/project/context.go` is a representative concise orientation query: it renders each applicable topic once (claim-ID roster, directly marked claim detail, matched-path count with a coverage drilldown) and attributes each classified effective path. Use `awf context --full internal/project/context.go` whenever every applicable authority claim is required; the full detail still renders once per topic. `awf topic tooling/cli --coverage` drills into the owning-domain and topic selectors, their both-must-match rule, current matched paths, and marker sites. `--json` serializes the same selected projection for machine consumption (prefer the text form when reading); concise JSON omits the full block. Explicit ADR paths show lifecycle-derived operation progress without treating ADR prose as current authority. Run `./x` with no command to print the metadata-derived forwarded awf verbs alongside the project-owned runner verbs.

`awf context --uncovered [<scan-root>...]` reports eligible paths that are
unowned or covered by no scoped topic. Add `--staged` to evaluate the immutable
index universe with the same eligibility and coverage model as staged check.
`--full --uncovered` is refused because a coverage-gap report is not an authority
projection.
