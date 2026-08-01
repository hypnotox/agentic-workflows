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
- A static-state inventory command enumerating the outstanding bounded candidates of
  ratchet-scoped code-design claims (today: the shallow `os.Is*` predicate sites,
  message-text identity assertions, exported error identities no caller matches,
  undocumented exported declarations, and the global test-seam census),
  so each ratchet topic can point its existing-violation backlog at a mechanical census
  instead of prose. The name must not be "awf doctor" without a disambiguating note:
  ADR-0162 retired that name with a different meaning. Needs its own ADR: it adds a
  command surface.
- Two Verify-line tightenings deferred from the ADR-0199/0200 terminal review, for the
  next ADR that updates these claims: `code-design/outcome-modeling:actionable-outcome-protocol`
  should verify that a changed axis records what actually moved rather than what was
  intended and that the remedy renders under the `next action:` prefix, and
  `code-design/package-composition:package-owns-one-sentence` should verify the ownership
  statement reads off a single sentence with further detail sentences legal. Both were
  reverted at review because an applied claim's Verify: line cannot change without an
  update operation and a same-ADR add+update pair is illegal.
- Align error-message prefixes across `cmd/awf`, `internal/adr`, and the changelog
  tooling. Cosmetic, and blocked on deciding which convention wins before any sweep.
- A plan-reviewer docCurrencyItem for the missing changelog task of an adopter-facing
  plan, to be added if a second plan ships without one (first occurrence 2026-07-12; the
  repo-local audit rule already catches the omission, just later than plan review would).
- The init collision probe over-refuses on artifacts a `--set` trim would deselect.
  Accepted as conservative design; revisit only if an adopter reports hitting it.

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

## A required config key reds the before-side of every transition check

`loadTreeCurrentState` validates the HEAD-side config after porting it forward, so a
key that is required with no in-code default fails the whole before-side load until
every committed config in range carries it. ADR-0202's `integrationBranch` hit this and
paid for it with a per-key seeding case in `ConfigForCurrentSchema` (`internal/migrate/
migrate.go`), the first case in that function to materialize a key rather than remove
one - which the function's own `retiredKeyRemovals` rationale argues against, since that
table exists precisely to keep the port-forward a pure parse fix.

The shipped fix is correct and mutation-proven, but it makes the function a growing list
of per-required-key special cases: the next required key repeats the discovery, and until
it does, the before-side of every transition check is red for a release cycle. The
alternative worth settling is that the before-side needs a historical config to PARSE,
not to satisfy today's validation rules, since coverage is never evaluated from a
before-side config. That would retire the seeding cases entirely rather than adding one
per key. Raised by the Phase 3 review of ADR-0202 on 2026-07-31 as a policy question
rather than a defect.

## A claim can carry at most one operation per ADR

`internal/adr/application.go` refuses to apply an operation more than once, and duplicate
declarations collapse to a single map key, so one ADR can never declare two updates on
the same claim. That is right for the usual case, but it means a record that has already
applied an update to a claim cannot correct that claim again, however small the
correction - the fix has to wait for a different decision record.

The live instance: `config/configuration:config-serialization-owned` asserts a CLOSED
enumeration of the editors that mutate config.yaml, and it omits `RemoveKey` and
`RemoveMappingKey`, both of which live migrations use. ADR-0202 already spent its one
update on that claim (adding `SetString`), so the enumeration stays false until another
ADR updates it. Worth folding into whatever decision next touches the config editors, and
worth asking there whether a "closed enumeration" claim should be backed by a
source-scanning test rather than by prose nobody can mechanically falsify.

## A slug colliding with the number namespace is only refused at the scaffold

`Corpus.ByIdentity` routes a four-digit key to the number index and everything else to the
slug index, so a record whose slug is exactly four digits can only ever be found as a
number - which it is not. `plan.ADRLink` reads any digits-only `adrs:` entry as a number
too, so an all-digit slug of any width is unlinkable from a plan.

The scaffold now refuses an all-digit title slug outright (`allDigitSlugRe`,
`internal/adr/adr.go`), which closes the only path that mints one through awf. A
hand-authored `2026.md` carrying `slug: 2026` still parses into the corpus and is still
unreachable by identity. The complete guard belongs on `corpus-single-identity-key`, whose
text already governs what is a corpus error - but ADR-0202 spent its one operation on that
claim in batch 1, and a claim carries at most one operation per ADR, so widening it needs a
later decision record. Worth folding into whatever decision next touches corpus identity,
and worth asking there whether the number-shaped-slug check belongs beside the existing
duplicate-key error rather than as a separate rule. Surfaced by the Phase 4 review of
ADR-0202 on 2026-07-31; the scaffold half landed the same day.


## Two pending records tie in the appended-batch rank

`adr.IdentityOrder` gives every slug the same rank, above every number, so a pending
record's newly applied batch sorts after every numbered one. That is the guarantee the
provenance order needs and it is proven. Two pending records, though, tie with each other
and the stable sort falls back to corpus order, which is directory order.

The consequence is a narrow false finding. Two pending records in one worktree, whose
batches are applied in the same commit, where the alphabetically earlier slug revises a
claim the later one adds, report `an add must be the first operation` even though the
numbering command's add-before-revise refusal guarantees the adder takes the lower number.
Verified 2026-08-01: corpus order `[alpha zebra]` reports it, `[zebra alpha]` is clean. The
author's workaround is to apply the two batches in separate commits.

Imposing a topological order instead would contradict
`invariants/current-state-authority:provenance-ordered-by-adr-number`, which says slug
entries compare in authored list order among themselves - and ADR-0202 spent its one
operation on that claim, so restating it needs a later decision record. Worth folding into
whatever decision next touches pending provenance order, and worth deciding there whether
the authored order should instead be something the tree records explicitly, since nothing
today declares the intended numbering order until the command is invoked. Surfaced by the
Phase 5 review of ADR-0202 on 2026-08-01.
