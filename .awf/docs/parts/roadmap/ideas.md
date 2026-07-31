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
  happens to be closest. ADR-0183 hit this concretely and had to extend an unrelated
  topic's selectors to keep a new package covered. Needs its own ADR: the coverage
  semantics, whether a path-owning global topic satisfies scoped coverage or only
  ownership, and what it does to the fan-out budget, which deliberately excludes global
  topics today.
- Pin what `internal/config`'s editors guarantee about an adopter's `config.yaml`. The `yaml.Node`
  round-trip ADR-0026 chose preserves every key, value, and comment but canonicalizes layout: a
  four-space block returns two-space, a sequence item under a surviving key re-indents, and blank
  lines are dropped. No claim states this, so every migration that edits config inherits an unstated
  contract and ADR-0185 had to narrow one claim that had guessed at it. Needs its own small ADR
  covering whether the property is claimed once for the package or restated per migration.
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
- A mechanical check for over-broad current-state claim prose. The `claim-prose-no-broader-than-reality`
  reviewer focus item exists and works: it caught all four over-broad claims in the 2026-07-30 severity
  session. It prevented none of them, which is the signal to climb from a judgment rung to a
  deterministic one. Two shapes look mechanizable without natural-language understanding: an absolute
  quantifier in a claim sentence ("every", "always", "never", "no", "byte-identical") could require an
  explicit justification marker, and a claim whose sentence is edited while its Origin ADR is frozen
  could be flagged, since that is the exact case needing a successor `update` operation. Twice in that
  session the false wording was INHERITED from an earlier record that the correcting pass never
  re-read, so the check would earn its keep on amendments rather than on new claims. Needs its own ADR:
  it changes what `awf check` rejects, and a false positive on legitimate absolute prose would be
  expensive.
- Broaden the task-skill set. Nothing produces a PR title and body from the commits of an
  effort; there is no skill for reviewing an incoming third-party PR, no security-review
  lens, no refactor-execution skill (`refactor-coupling-audit` only scopes the decision),
  and no dependency-upgrade skill. For the last, a concrete by-hand model exists from
  2026-07-10: cooldown window, govulncheck reachability triage, SHA-pin bumps, changelog
  entry.
- Publish the standard as an artifact. No versioned spec exists (the standard is implied
  by the renderer and its templates), there is no discoverability surface beyond the
  GitHub README, and no examples gallery beyond `examples/sundial`.
- Audit this repo's own overrides for dogfooding. The principle (user, 2026-07-26): only
  overwrite what is really needed, otherwise dogfood the shipped defaults, and use
  template defaults inside overrides so they keep rendering. A survey that day found 7
  full-replacement parts under `.awf/parts/`, 2 under `.awf/skills/parts/`
  (retrospective/procedure and debugging/debugging-surfaces are the worrying pair: awf
  never renders those shipped defaults at all), and 14 under `.awf/docs/parts/`.
- An advisory for hand-curated prose counts, which drift when the source-of-truth count
  changes; two recorded occurrences (the agent-guide invariant list and the glossary
  exemption count).
- Align error-message prefixes across `cmd/awf`, `internal/adr`, and the changelog
  tooling. Cosmetic, and blocked on deciding which convention wins before any sweep.
- A plan-reviewer docCurrencyItem for the missing changelog task of an adopter-facing
  plan, to be added if a second plan ships without one (first occurrence 2026-07-12; the
  repo-local audit rule already catches the omission, just later than plan review would).
- The init collision probe over-refuses on artifacts a `--set` trim would deselect.
  Accepted as conservative design; revisit only if an adopter reports hitting it.
