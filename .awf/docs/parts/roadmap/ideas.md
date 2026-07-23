## Ideas

- Add a session dashboard for active and queued governed child work.
- Add phase-sensitive tool activation so each workflow phase exposes only its relevant tools.
- Promote the topic-claim-budget advisory to a configurable severity (`error`, `warn`,
  `off`) now that ADR-0148 brought every topic under budget; needs its own small ADR
  revising `tooling/cli:topic-claim-budget-advisory` and an adopter-facing config key.
- Fix the unsatisfiable tag advisory for governed ADRs: `awf check` prints "carries no
  tags: add a narrow topic tag" for `current-state-v2` ADRs, but the governed frontmatter
  (internal/adr/format.go, `KnownFields(true)`) rejects a `tags:` key, so the advisory can
  never be satisfied; either accept tags in governed frontmatter or exclude governed ADRs
  from the tag-coverage scan.
- Narrow the ADR-0148 successor topics' mirrored path selectors per area so broad-path
  `awf context --full` packets shrink (deferred by ADR-0148 Decision item 4).
- Enforce the plan freeze mechanically: `awf check --staged` could refuse a diff that edits a
  `docs/plans/` file whose HEAD `status:` is `Implemented`. The recorded "record implementation
  deviations before the terminal artifact transaction" pitfall did not prevent the ADR-0151
  session from appending Notes to a frozen plan at review's direction; a prose rule that failed
  twice is the promotion signal for a deterministic check (needs an ADR: it changes check
  behavior and the plan lifecycle contract).
