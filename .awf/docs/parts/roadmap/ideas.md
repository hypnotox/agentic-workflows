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
- Promote the topic-claim-budget advisory from a non-failing note to a fixed blocking
  rank now that ADR-0148 brought every topic under budget; needs its own small ADR
  revising `tooling/cli:topic-claim-budget-advisory`. ADR-0183 forecloses the
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
- Make `awf effort integrate` fast-forward-only. Keep the already-contained and fast-forward
  arms and refuse when the target is not an ancestor of the effort tip, naming the recovery:
  merge the target in the managed worktree, run `awf check --staged`, run the gate, commit,
  renew terminal review, retry. The motivation is concurrency, not correctness: the divergent
  path leaves a staged uncommitted merge in the shared receiving checkout across a full gate
  and a renewed terminal review, blocking every other finishing effort for that whole window,
  while moving the work into the effort worktree collapses the shared critical section to an
  atomic fast-forward. The predicate already exists, `Ancestor(target, tip)` at
  `internal/worktree/manager.go:315`; turning that branch point into a precondition is the
  whole behavioural change. Compare against the receiving checkout's HEAD as `Integrate`
  already resolves it, or against `integrationBranch` once that key ships. `MergeNoCommit`
  has exactly one production consumer (`manager.go:333`), so removing the divergent arm makes
  it dead down through the Runner contract (`manager.go:30`) and `internal/git/lifecycle.go:139`
  and the dead-code gate forces the deletion. Needs its own ADR: it falsifies the test-backed
  `tooling/effort-management:managed-worktree-lifecycle` claim, whose text still reads that
  integration "reports already-contained history, fast-forwards, or starts a divergent
  `--no-commit` merge", so it carries an update operation. SEQUENCING: this must land after
  the pending-ADR-numbering decision's final phase, not merely after that decision lands. Both
  changes rewrite the same lines of `templates/skills/reviewing-impl/SKILL.md.tmpl` step 8
  (`:75` carries the divergent-merge routing that becomes wrong here; the numbering plan's
  Task 6.2 rewrites `:72-77`), so authoring them concurrently reproduces exactly the
  integration collision both are trying to reduce. Deliberately NOT absorbed into the numbering
  decision: that record was already `Implementing` with its first batch applied when this was
  raised, its prescription that numbering "runs in the effort worktree after merging the
  integration branch in" is a usage prescription rather than an assertion about integrate's
  behaviour, and its integration-branch block plus the branch-independent duplicate-identity
  check already force numbering without this. Worth doing as the first pending slug ADR once
  numbering ships, which dogfoods the mechanism on its first real use.
