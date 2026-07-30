## Ideas

- Design concurrent same-checkout batch helpers only after scope enforcement, incidental-write
  attribution, failure attribution, and deterministic integration are specified; worktree-isolated
  and patch-producing parallel workers remain out of scope for the current workflow contract.

- Add phase-sensitive tool activation so each workflow phase exposes only its relevant tools.
- Let a global topic carry path selectors, so it can own specific paths as well as supply
  global authority. Today `applies: global` is skipped outright by both `coveredByDomain`
  and `matchingScopedTopics` in `internal/topic/coverage.go`, so a global topic can never
  own a path and can never satisfy scoped coverage. That leaves a shared-pattern holder
  with nowhere natural to live: a small package existing only to carry a cross-cutting
  type belongs to the global topic describing that pattern, not to whichever scoped topic
  happens to be closest. ADR-0179 hit this concretely and had to extend an unrelated
  topic's selectors to keep a new package covered. Needs its own ADR: the coverage
  semantics, whether a path-owning global topic satisfies scoped coverage or only
  ownership, and what it does to the fan-out budget, which deliberately excludes global
  topics today.
- Promote the topic-claim-budget advisory from a non-failing note to a fixed blocking
  rank now that ADR-0148 brought every topic under budget; needs its own small ADR
  revising `tooling/cli:topic-claim-budget-advisory`. ADR-0179 forecloses the
  configurable-severity and adopter-facing-config-key half of this idea: awf exposes no
  severity setting, so the promotion is to a rank fixed in code or not at all.
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
- Design a structured context result only when a demonstrated consumer can define its contract;
  ADR-0165 deliberately removed speculative JSON rather than preserving a hidden path census.
- Enforce the plan freeze mechanically: `awf check --staged` could refuse a diff that edits a
  `docs/plans/` file whose HEAD `status:` is `Implemented`. The recorded "record implementation
  deviations before the terminal artifact transaction" pitfall did not prevent the ADR-0151
  session from appending Notes to a frozen plan at review's direction; a prose rule that failed
  twice is the promotion signal for a deterministic check (needs an ADR: it changes check
  behavior and the plan lifecycle contract). The ADR-0158 effort is the third occurrence and
  splits the target in two: the remediation half of the pitfall worked (terminal review directed
  a post-freeze Notes append and the executing session declined it, citing the pitfall), while
  the prevention half failed again (three deviations reached the freeze commit unrecorded). The
  phase-transaction-ownership effort is the fourth occurrence: terminal review found two touched
  paths missing from the frozen inventory and one listed path that was not touched. A refusal to
  edit a frozen plan would not have prevented either miss. The complementary and cheaper lever is
  a pre-flip deviation sweep in the execution skills' final-commit step, which
  is where the omission actually happens; that is a shipped-template change, so it needs its own
  ADR rather than a local override, on pain of awf diverging from the standard it publishes.
- A conditional-key consumption check: extend the ADR-0086 consumption union so a template
  conditional keyed on a render key that no render path for that artifact sets fails loudly.
  The 0157 effort found every `targetSessionHandoff` branch in the singleton templates had
  been dead prose since authoring; the fix plumbed the key, but nothing today prevents the
  next dead conditional (recorded as a rendering pitfall, 2026-07-23).
