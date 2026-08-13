- Design concurrent same-checkout batch helpers only after scope enforcement, incidental-write and failure attribution, and deterministic integration are specified. Worktree-isolated and patch-producing workers remain out of scope for the current workflow contract.

- Add phase-sensitive tool activation so each workflow phase exposes only its relevant tools.

- Decide whether staged drift has semantics for directory entries, untracked files, config-tree hygiene, and dead-reference probing. A snapshot tree has neither directory entries nor untracked files; define these filesystem-only checks before making them blocking.

- Pin what `internal/config`'s editors guarantee about an adopter's `config.yaml`. ADR-0026 chose `yaml.Node` round-trip preservation of keys, values, and comments while canonicalizing layout; observed changes include four-space to two-space indentation, re-indented sequence items, and dropped blank lines. ADR-0185 had to narrow an unsupported claim. Decide in a small ADR whether this package property is claimed once or per migration.

- Add an advisory `awf audit` rule flagging code-scoped (`fix`, `feat`, `test`, `refactor`) commits that mutate an ADR body. The shared-index sweep pitfall recurred four times (2026-07-10 twice, 2026-07-19, 2026-07-23), with three ADR bodies folded into code commits; prose prevention failed. Needs an ADR because it changes shipped audit behavior.

- Scope the `config/configuration:tag-coverage-note` claim to tag-capable legacy ADRs ("each legacy ADR and each pitfall"). Governed ADRs reject `tags:`, so the current "each ADR" claim drifts with every new governed ADR. Needs a config-domain ADR.

- Design a structured context result only when a demonstrated consumer can define its contract; ADR-0165 removed speculative JSON rather than retain a hidden path census.

- Enforce the plan freeze mechanically. `awf check staged` could refuse edits to a HEAD-Implemented `docs/plans/` file, but that alone would not prevent incomplete frozen inventories. The prose rule failed twice; ADR-0158 exposed three unrecorded deviations, and phase-transaction-ownership found two missing touched paths and one untouched listed path. Prefer a pre-flip deviation sweep in execution skills, where omissions occur; it is a shipped-template change requiring its own ADR. The rejected post-freeze Notes append in ADR-0151 and ADR-0158's declined request show the remediation half worked.

- A mechanical check for over-broad current-state claim prose. `claim-prose-no-broader-than-reality` caught all four claims in the 2026-07-30 severity session but prevented none. Consider requiring explicit justification for absolute quantifiers (`every`, `always`, `never`, `no`, `byte-identical`) and flagging edits to a claim sentence while its Origin ADR is frozen; two corrections inherited false wording without rereading its source. Needs an ADR because it changes `awf check`; false positives on legitimate absolutes are costly.

- Broaden the task-skill set: PR title/body generation from effort commits, third-party PR review, security review, refactor execution, and dependency upgrades. The 2026-07-10 manual upgrade model supplies cooldown, govulncheck reachability triage, SHA-pin bumps, and changelog entry.

- Publish the standard as an artifact: there is no versioned spec, discoverability beyond the GitHub README, or examples gallery.

- Audit this repo's own overrides for dogfooding. The 2026-07-26 principle is to override only what is needed and use template defaults in overrides. That survey found 7 full replacements under `.awf/parts/`, 2 under `.awf/skills/parts/` (notably retrospective/procedure and debugging/debugging-surfaces), and 14 under `.awf/docs/parts/`.

- An advisory for hand-curated prose counts that drift when their source count changes; recorded twice (agent-guide invariant list and glossary exemption count).

- A static-state inventory command for bounded ratchet-scoped code-design candidates: shallow `os.Is*` predicates, message-text identity assertions, unmatched exported error identities, undocumented exported declarations, and the global test-seam census. It should replace prose backlogs with a mechanical census; do not call it "awf doctor" without disambiguation because ADR-0162 retired that name. Needs an ADR because it adds a command.

- Two Verify-line tightenings deferred from ADR-0199/0200 review: `code-design/outcome-modeling:actionable-outcome-protocol` should verify an axis records what moved and the remedy renders under `next action:`; `code-design/package-composition:package-owns-one-sentence` should verify a single-sentence ownership statement while allowing detail sentences. Review reverted both because an applied claim requires an update operation and same-ADR add+update is illegal; take them with the next ADR updating these claims.

- Align error-message prefixes across `cmd/awf`, `internal/adr`, and changelog tooling. Cosmetic; decide the winning convention before a sweep.

- A plan-reviewer docCurrencyItem for an adopter-facing plan missing a changelog task, if a second plan ships without one. First occurrence: 2026-07-12; the repo-local audit already catches it later.

- The init collision probe over-refuses on artifacts a `--set` trim would deselect. Accepted as conservative design; revisit only on adopter report.

- Make `./awf effort integrate` fast-forward-only. Retain already-contained and fast-forward cases; otherwise refuse and direct recovery: merge target in the managed worktree, run `./awf check staged` and the gate, commit, renew terminal review, retry. Divergent `--no-commit` merges leave a shared staged checkout across gate and review; moving them to the effort worktree makes integration an atomic fast-forward. The predicate is `Ancestor(target, tip)` at `internal/worktree/manager.go:315`; compare the receiving HEAD (or `integrationBranch` once shipped). Removing the sole `MergeNoCommit` consumer (`manager.go:333`) makes its Runner and lifecycle path dead, requiring deletion under the dead-code gate. Needs an ADR and update to `tooling/effort-management:managed-worktree-lifecycle`, which currently promises divergent merge. Sequence after the pending-ADR-numbering decision's final phase: both rewrite `templates/skills/reviewing-impl/SKILL.md.tmpl` step 8 (`:75`; numbering Task 6.2 rewrites `:72-77`). Keep separate: numbering was Implementing when raised, its prescription is usage rather than integrate behavior, and its integration-branch and duplicate-identity checks already force numbering. Use the first pending-slug ADR after numbering ships.

## A required config key reds the before-side of every transition check

`loadTreeCurrentState` validates ported-forward HEAD-side config, so a newly required key without an in-code default makes every before-side config fail until history contains it. ADR-0202 added a per-key `ConfigForCurrentSchema` seed for `integrationBranch`, the first materializing case despite `retiredKeyRemovals` defining that port-forward as a pure parse fix. The mutation-proven fix works, but each new required key repeats the exception and leaves transition checks red for a release cycle. Decide whether before-side config need only parse rather than satisfy current validation; coverage is never evaluated there. Raised as a policy question in ADR-0202 Phase 3 review on 2026-07-31.

## A claim can carry at most one operation per ADR

`internal/adr/application.go` permits one operation per claim and duplicate declarations collapse to one map key. `config/configuration:config-serialization-owned` falsely gives a closed editor enumeration that omits migration-used `RemoveKey` and `RemoveMappingKey`; ADR-0202 already spent its update adding `SetString`. Fold correction into the next config-editor ADR, and decide whether a closed enumeration needs a source-scanning proof rather than unfalsifiable prose.

## A slug colliding with the number namespace is only refused at the scaffold

`Corpus.ByIdentity` routes four-digit keys to numbers; `plan.ADRLink` routes every digits-only `adrs:` entry to a number. Thus a hand-authored all-digit slug parses but is unreachable. The scaffold now rejects all-digit title slugs (`allDigitSlugRe`, `internal/adr/adr.go`), but a `2026.md` with `slug: 2026` remains live. Widen `corpus-single-identity-key` in a later ADR because ADR-0202 spent its one operation; decide whether to place the check beside duplicate-key handling. Surfaced in ADR-0202 Phase 4 review on 2026-07-31.

## Two pending records tie in the appended-batch rank

`adr.IdentityOrder` ranks all slugs together above numbers; among pending records stable sort falls back to directory order. Two same-commit pending records can therefore falsely report `an add must be the first operation` when alphabetically earlier `alpha` revises a claim later `zebra` adds, though numbering assigns the adder lower number. Verified 2026-08-01: `[alpha zebra]` fails and `[zebra alpha]` passes; current recovery is separate commits. A topological order conflicts with `invariants/current-state-authority:provenance-ordered-by-adr-number`, which requires authored slug-list order, and ADR-0202 spent that claim's operation. Fold into the next pending-provenance ADR and decide whether the tree must record intended numbering order. Surfaced in ADR-0202 Phase 5 review on 2026-08-01.

## Bind effort mutations to an explicit checkout identity

Effort-associated Pi sessions remain rooted at the primary checkout while receiving a managed-worktree path. During active-pitfalls, the parent ran `./x render` in primary; it happened to be current, but the same error can rewrite outputs and `.awf/awf.lock` into the wrong transaction. The managed-worktree pitfall already records the shape, so prose is insufficient. Design invocation-owned checkout binding or an effort-aware seam that refuses ambiguous pre-integration primary mutation, while preserving primary use for integration, removal, retrospective, and finish. Pi association is runtime metadata, and read-only commands must work from either checkout.

## Preserve configured identity for lifecycle Git mutations

The native Git runner suppresses global and system configuration, but `awf effort integrate`'s `git merge --no-ff --no-commit` still needs committer identity. Global-only owner identity therefore fails as `merge-conflict` with `Committer identity unknown`; a local override works but violates the repository ban. Design a lifecycle path that receives validated effective owner identity without reopening repository selection, credentials, signing, or arbitrary config, and distinguish pre-merge configuration failure from content conflict. Tests must cover global-only identity, hostile inherited config, and real conflicts; Git isolation remains load-bearing.
