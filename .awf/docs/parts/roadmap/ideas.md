## Ideas

- Add a session dashboard for active and queued governed child work.
- Add phase-sensitive tool activation so each workflow phase exposes only its relevant tools.
- Promote the topic-claim-budget advisory to a configurable severity (`error`, `warn`,
  `off`) now that ADR-0148 brought every topic under budget; needs its own small ADR
  revising `tooling/cli:topic-claim-budget-advisory` and an adopter-facing config key.
- Scope the `config/configuration:tag-coverage-note` claim text to tag-capable legacy ADRs
  ("each legacy ADR and each pitfall"): the tag-coverage scan now skips all governed ADRs
  (their closed frontmatter rejects a `tags:` key), so the claim's unqualified "each ADR"
  drifts further from behavior with every new governed ADR; the mutation needs a
  config-domain ADR.
- Narrow the ADR-0148 successor topics' mirrored path selectors per area so broad-path
  `awf context --full` packets shrink (deferred by ADR-0148 Decision item 4).
- Enforce the plan freeze mechanically: `awf check --staged` could refuse a diff that edits a
  `docs/plans/` file whose HEAD `status:` is `Implemented`. The recorded "record implementation
  deviations before the terminal artifact transaction" pitfall did not prevent the ADR-0151
  session from appending Notes to a frozen plan at review's direction; a prose rule that failed
  twice is the promotion signal for a deterministic check (needs an ADR: it changes check
  behavior and the plan lifecycle contract).
