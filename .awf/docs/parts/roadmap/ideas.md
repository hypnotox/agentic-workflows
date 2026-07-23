## Ideas

- Add a session dashboard for active and queued governed child work.
- Add phase-sensitive tool activation so each workflow phase exposes only its relevant tools.
- Promote the topic-claim-budget advisory to a configurable severity (`error`, `warn`,
  `off`) now that ADR-0148 brought every topic under budget; needs its own small ADR
  revising `tooling/cli:topic-claim-budget-advisory` and an adopter-facing config key.
- Add an advisory `awf audit` rule flagging a code-scoped commit (`fix`, `feat`, `test`,
  `refactor` types) that also mutates a `docs/decisions/` ADR body: the shared-index sweep
  pitfall has now recurred four times (2026-07-10 twice, 2026-07-19, 2026-07-23) and three
  of the four occurrences folded ADR content into a code commit, which this rule would have
  flagged deterministically; prose prevention has demonstrably failed (needs an ADR: it
  changes shipped audit behavior).
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
- A conditional-key consumption check: extend the ADR-0086 consumption union so a template
  conditional keyed on a render key that no render path for that artifact sets fails loudly.
  The 0157 effort found every `targetSessionHandoff` branch in the singleton templates had
  been dead prose since authoring; the fix plumbed the key, but nothing today prevents the
  next dead conditional (recorded as a rendering pitfall, 2026-07-23).
