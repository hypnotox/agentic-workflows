---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0190: Slug-identified pending ADRs numbered at integration


## Context

Managed worktrees are the default effort execution location (ADR-0189), so parallel
efforts routinely author ADRs concurrently. `awf new adr` allocates highest-plus-one, so
two in-flight efforts allocate the same number; whichever merges second collides and must
renumber by hand at merge time. The 0151 -> 0153 renumber (state sequence 34) was done
manually, and a live collision exists today: the in-flight git-seam decision ("git access
through one semantic seam") is numbered 0186 in its worktree while main's 0186 is a
different record.

Hand renumbering is a multi-surface rewrite: filename, heading, topic `Origin:` and
`Revised-by:` lines, the generated INDEX, applied state-sequence values, and any prose
references. Two enforced rules bound the solution space. ADR numbers must be contiguous
from 1 with only lock-pinned legacy gaps (`checkADRContiguity`,
internal/currentstate/load.go), so pre-allocation and reservation schemes are out. State
sequences must be contiguous from 1 with no gaps ever (internal/currentstate/check.go),
so merge-time resequencing of Applied batches is unavoidable no matter what happens to
ADR numbers.

The content digest covers only the five body sections (internal/adr/digest.go):
frontmatter, the `# ADR-NNNN` heading, and Status history are excluded. Renaming a file,
rewriting its heading, and shifting state-sequence values are therefore digest-safe. A
record's own body stays amendable until a terminal status (ADR-0188), but bodies of
already-Implemented ADRs that reference a number are frozen: a number that turns out
wrong at merge time cannot be corrected there and silently resolves to the wrong ADR.

Today's staged transition checks refuse a numbering commit on two independent grounds:
a claim's `Origin:` may not change without a declared operation (and even a declared
update must preserve it), and prior Applied events may not be mutated (the history-prefix
rule). A sanctioned transition shape is required; ADR-0182's aggregate merge mode is the
precedent for a validated special-purpose transition.

Merge commits run no rendered hook (verified empirically: awf renders only pre-commit,
commit-msg, and pre-push payloads), so a clean automerge to main currently bypasses the
duplicate-identity backstop. A plan links its ADRs through a numeric `adrs:` frontmatter
array whose entries must resolve to a `NNNN-*.md` file, so a plan authored beside a
not-yet-numbered ADR has no expressible link today.

No external adopter consumes ADR numbers programmatically; the internal cost of a second
reference surface is accepted.

## Decision

1. A third immutable lock cutoff, `adrFormatV3From`, routes the new format
   `current-state-v3` (precedent: the V1 and V2 cutoffs). A V3 record is a V2 record
   plus a mandatory `slug:` frontmatter key and the pending identity form below. Digest
   coverage and the history grammar are unchanged: the cutoff-immutability contract
   generalizes from exactly two permanent cutoffs to the full ordered cutoff set, and
   the amendability, stamp-chain, and Applied-immutability contract restates over both
   governed digest formats. Records below the cutoff are grandfathered and carry no
   slug key.
2. A V3 ADR is authored as a pending record: file `<slug>.md`, heading
   `# ADR-<slug>: <Title>`, identity form `ADR-<slug>`. The slug is slugify(title), the
   existing filename derivation, frozen at scaffold time; it does not track later title
   edits. The terminology is "pending" (a pending ADR can be fully Implemented inside a
   worktree); "draft" is wrong and unused. Scaffolding refuses a title that slugifies
   to a reserved basename (readme, index, template) or to a slug already present
   anywhere in the corpus, pending or retained, so a collision fails at authoring time
   rather than at check time.
3. Numbering prepends `NNNN-`, giving the existing `NNNN-<slug>.md` convention and
   `# ADR-NNNN: <Title>` heading. The `slug:` key is retained forever after numbering,
   so an `ADR-<slug>` reference stays resolvable by grep without git archaeology. Slug
   uniqueness across every record that carries a slug key, pending plus retained, is a
   checked invariant (grandfathered pre-cutoff records carry none); it also removes
   slug-reuse ambiguity after an effort is deleted.
4. Numbered records route by the numeric cutoffs as today; a numberless record routes
   by its `format: current-state-v3` frontmatter marker, and a numberless file that
   does not declare that format is a corpus error. This is a deliberate departure from
   numeric-only cutoff routing: a pending file has no number to route by. Reserved
   basenames (README.md, INDEX.md, template.md) stay excluded from the corpus; any
   other file under the decisions directory that parses as neither a numbered nor a
   pending record is a corpus error. Previously such files were silently ignored; this
   is an accepted behavior change.
5. `awf new adr` becomes branch-aware: it scaffolds a numbered record when run on the
   integration branch and a pending record elsewhere. Branch detection lands as an
   entrypoint of the git seam decided by the in-flight git-seam ADR, never as an ad-hoc
   git call.
6. A new flat config key `integrationBranch` names the integration branch. It is
   required-explicit: the schema migration writes `integrationBranch: main` visibly into
   config.yaml and there is no in-code default (the silent-default shape ADR-0127
   removed stays removed). Audit range resolution must not read it: the audit range
   still reaches the audit only from the command line, so ADR-0127's no-config-base-
   branch boundary stays intact. The key is flat because it is one scalar project fact
   consumed by scaffolding and the check, not a settings group.
7. A pending ADR is blocked from the integration branch: the check fails on any pending
   record present there, forcing numbering at integration time. The block fires only
   when the git seam positively identifies the checkout as being on the integration
   branch; a detached HEAD or otherwise indeterminate result passes, and the
   branch-independent duplicate-identity check remains the backstop protecting
   automated detached-HEAD runs.
8. A new gated command group `adr` with child `awf adr number [<slug>...]` performs
   numbering. It runs in the effort worktree after merging the integration branch in and
   before merging back. With exactly one pending record, bare invocation numbers it.
   With several, `awf adr number <slug>...` numbers in explicit argument order; bare
   invocation refuses and lists the pending slugs.
9. Numbering performs exactly these effects and nothing else: rename `<slug>.md` to
   `NNNN-<slug>.md`; rewrite the heading to `# ADR-NNNN: <Title>`; substitute the slug
   with the number in `Origin:` and `Revised-by:` lines of the authored claim sources
   under the topic parts tree (never in a generated file); shift applied state-sequence
   values preserving relative order, slotting pending batches after the highest
   numbered sequence; re-render so the generated topic docs and the INDEX match and the
   numbering commit lands drift-clean. It prints the `<slug> -> NNNN` mapping and any
   sequence shifts for use in the integration commit message. Frozen bodies, plans, and
   commit messages keep their slug references.
10. Staged validation recognizes the numbering commit as a sanctioned numbering
    transition permitting exactly the item-9 effects; V3 transition pairing keys on the
    slug, so the rename is not misread as a delete plus an add. The red-gate window
    between merge-in and numbering is expected, so the command must not precondition on
    a full green check.
11. A number once assigned never changes. If the integration branch moves after
    numbering but before merge-back, the stale numbering is unmade, not corrected: the
    numbering commit is terminal and self-contained, so the retry recipe is reset the
    numbering commit, merge the integration branch again, re-run `awf adr number`, gate,
    and merge back. On a corpus with duplicate numbers and no pending record the command
    refuses and offers that recipe as a hint; it uses no git-provenance or merge-base
    logic. Duplicate identity on the integration branch's gate is the final backstop.
12. A pre-merge-commit hook payload is rendered alongside the existing three payloads so
    the duplicate-identity backstop also fires on a conflict-free true merge commit to
    the integration branch (a fast-forward creates no commit, and a conflicted merge
    already runs pre-commit). awf renders the payload only and never activates hooks:
    this repo and every adopter wire an executable `.githooks/pre-merge-commit` stub by
    hand, like the existing three stubs.
13. A plan's `adrs:` frontmatter entry may be a number or a slug. A slug entry resolves
    against a pending `<slug>.md` or the retained `slug:` key of a numbered record, so
    it stays valid after numbering without rewriting; an entry that resolves to neither
    fails link validation. Numbering never rewrites plan files.
14. All six added claims are invariants with `Backing: test`; proof markers land when
    the operations are Applied: `pending-adr-slug-identity` and
    `adr-slug-frontmatter-mandatory` on internal/adr tests;
    `pending-blocked-from-integration-branch`, `numbering-transition-mode`, and
    `adr-number-immutable` on internal/currentstate tests; `integration-branch-explicit`
    on the config schema-migration tests. Operation motivation: items 1 and 4 drive the
    `fresh-adoption-v1-cutoff`, `adr-status-enum-and-matrix`,
    `adr-v2-cutoff-atomic-immutable`, and `adr-amendable-until-terminal` updates; items
    2, 3, and 4 drive the `pending-adr-slug-identity` and
    `adr-slug-frontmatter-mandatory` adds and the `adr-new-heading-matches-file` and
    `corpus-single-identity-key` updates; item 5 drives the
    `adr-new-sequential-numbering` update; item 6 drives the
    `integration-branch-explicit` add; item 7 drives the
    `pending-blocked-from-integration-branch` add; items 8 through 11 drive the
    `numbering-transition-mode` and `adr-number-immutable` adds and the
    `applied-history-events-append-only` and `application-batch-sequence-order`
    updates; item 12 drives the `hook-payloads-rendered` update; item 13 drives the
    `plan-adr-link-resolved` update.
15. The documentation travels with the implementation commits: the ADR-template
    singleton source (the adr-template frontmatter part, which every adopter's scaffold
    copies) gains the V3 pending shape with the `slug:` key; the working-with-awf
    commands part documents `awf adr number` and the merge-in, number, merge-back
    procedure; the adr-system domain narrative's two-cutoff opening is corrected to the
    ordered cutoff set; and the generated surfaces (the config reference entry for
    `integrationBranch`, the agent-guide gated-command enumeration for the `adr` group,
    the topic docs, the INDEX) follow from their configspec, clispec, and topic sources
    through render.

## State changes

- update `adr-system/adr-lifecycle:fresh-adoption-v1-cutoff`
- update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`
- update `adr-system/adr-lifecycle:adr-amendable-until-terminal`
- update `adr-system/adr-lifecycle:adr-new-sequential-numbering`
- update `adr-system/adr-lifecycle:adr-new-heading-matches-file`
- update `adr-system/adr-lifecycle:corpus-single-identity-key`
- update `adr-system/adr-lifecycle:applied-history-events-append-only`
- add `adr-system/adr-lifecycle:pending-adr-slug-identity`
- add `adr-system/adr-lifecycle:adr-slug-frontmatter-mandatory`
- add `adr-system/adr-lifecycle:pending-blocked-from-integration-branch`
- add `adr-system/adr-lifecycle:numbering-transition-mode`
- add `adr-system/adr-lifecycle:adr-number-immutable`
- update `adr-system/plan-artifacts:plan-adr-link-resolved`
- update `config/migrations-and-locks:adr-v2-cutoff-atomic-immutable`
- add `config/configuration:integration-branch-explicit`
- update `invariants/current-state-authority:application-batch-sequence-order`
- update `rendering/singletons-and-payloads:hook-payloads-rendered`

## Consequences

- Merge-time reconciliation becomes one mechanical gated command; hand renumbering ends.
  The live 0186 collision in the git-seam worktree renumbers by hand one last time
  before this lands, and this record itself is numbered optimistically in a worktree, so
  it may be the final beneficiary of that manual recipe.
- Integration gains a mandatory numbering step even when no collision exists. Uniformity
  is chosen deliberately: a step that always runs is more reliable for agents than a
  conditional fix that runs only on collision.
- Every future ADR carries two identity spellings over its life. Readers must accept
  `ADR-<slug>` references in frozen bodies, plans, and commit messages; resolution is a
  grep for the retained `slug:` key.
- The corpus now carries two validated exemptions from Applied-event immutability and
  Origin preservation: ADR-0182's aggregate merge and this numbering transition. Each
  further exemption widens the surface a corrupt or hand-crafted transition commit
  could exploit, which is why item 9's effect list is exhaustive.
- A red gate between merge-in and numbering is a normal, expected state; procedures and
  the numbering command must tolerate it.
- Adopters carry a visible `integrationBranch` key after migration; there is no silent
  default to misconfigure.
- The adr-lifecycle topic grows from 19 to 24 claims, past the maxClaimsPerTopic
  advisory of 20. The trip is accepted at proposal time; the implementation plan
  decides whether to split the topic (ADR-0148 precedent) or carry the advisory.
- The pre-merge-commit payload is inert until a project wires its executable
  `.githooks/` stub by hand; until that adopter-visible step the automerge backstop is
  absent, exactly as hooks behave today.
- Stray markdown files in the decisions directory become errors instead of being
  silently ignored; the render path's silent last-wins duplicate handling remains a
  known pre-existing blindness to address during implementation.
- Implementation ripples across every `NNNN-*.md` filename gate (internal/adr,
  internal/currentstate, internal/project, internal/audit), the Atoi coverage-ignore
  reasons a slug-identified record falsifies, INDEX ordering (numbered records first by
  number, pending records after, alphabetically), and the `adr` command group's
  engagement of the existing help-listing and group-gating claims.
- Plan sequencing: branch detection depends on the in-flight git-seam decision, so the
  implementation plan for this record sequences after that ADR.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Renumber-on-collision command (optimistic numbers, fix the second lander) | A wrong number referenced from an already-terminal frozen body cannot be corrected and silently resolves to the wrong ADR; a conditional fix is also less reliable for agents than one uniform mandatory step |
| Number pre-allocation or central reservation | Number contiguity from 1 makes gaps illegal, and an abandoned effort would leak its reserved number forever |
| Placeholder state-sequence grammar in worktrees | Sequences must be contiguous from 1 at every commit; shifts are mechanical anyway, so a second grammar buys nothing |
| Content-hash or UUID identity replacing numbers | Destroys the human-readable ordered corpus and every existing reference surface for marginal gain |
| Keep renumbering by hand | Proven error-prone multi-surface rewrite (0151 -> 0153 precedent, live 0186 collision), and it recurs with every parallel effort now that worktrees are the default |

## Status history

- 2026-07-31: Proposed
