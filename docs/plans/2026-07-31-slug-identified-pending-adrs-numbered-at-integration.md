---
date: 2026-07-31
adrs: [190]
status: Proposed
---
# Plan: Slug-identified pending ADRs numbered at integration

## Goal

Implement ADR-0190: format `current-state-v3` pending ADRs (`<slug>.md`, mandatory
retained `slug:` frontmatter), branch-aware scaffolding against a new required-explicit
`integrationBranch` config key, the pending-on-integration-branch block, slug-capable
plan links, the gated `awf adr number` command with its sanctioned numbering transition,
and the pre-merge-commit hook payload. Non-goals: no adr-lifecycle topic split (ruled
out; the claim-budget limit is being removed separately), no change to digest coverage or
history grammar, no renumbering of any already-numbered ADR, and no plan rewriting by the
numbering command.

**Hard precondition:** the git-seam ADR ("git access through one semantic seam", in the
worktree `single-home-and-git-seam-decisions` at planning time) must be landed on main
before Phase 2 begins. Phase 1 verifies this and resyncs; if the seam ADR has not landed,
execution stops there.

## Architecture summary

Six phases, each an independently green transaction. Phase 1 merges the landed seam work
in and resyncs (including the possible one-time manual renumber of ADR-0190 itself).
Phase 2 makes pending records first-class corpus members: the `adrFormatV3From` lock
cutoff, the V3 parse path with slug identity, slug-aware corpus indexing with hard
duplicate errors, provenance matching for slug-form `Origin:`/`Revised-by:`, INDEX
ordering, and the stray-file corpus error. Phase 3 adds `integrationBranch`
(config + configspec + migration via a new top-level scalar editor), the seam
branch-detection entrypoint, branch-aware scaffolding with reserved-basename and
slug-collision refusals, and the pending-block check. Phase 4 widens plan `adrs:` links
to accept slugs. Phase 5 builds `awf adr number` and the slug-paired numbering
transition validation. Phase 6 renders the pre-merge-commit payload and lands the
documentation and changelog obligations.

ADR-0190's operations apply as five Implementing batches, one per implementation phase
(Phase 2 appends the Implementing status event first). Each batch's Applied event uses
the next unclaimed contiguous state sequence at commit time and lists its operations in
State-changes declaration order; each travels in the same commit as exactly its claim
mutations in `.awf/topics/parts/**` plus the re-rendered topic docs. Phase 6's batch is
the remainder, and its closing commit appends the `Implemented` flip event directly
after it (the V2 final pair; an `Implementing` record with nothing remaining is an
illegal state per `internal/adr/application.go:113-116`, and the 0187/0189 precedent
pairs the final batch with the flip in one commit). The plan's own
`status: Implemented` freeze still lands in the deferred post-review transaction.

Code-design constraints that bind throughout: single-home (the numbering engine is the
only writer of numbering effects; branch detection lives only in the git seam),
dependency-composition, and the seam ADR's entrypoint rules (deadlined context, contract
suite, no backend types across the surface).

## File structure

- **Created:**
  - `internal/migrate/adrformatv3.go`, `internal/migrate/adrformatv3_test.go`
  - `internal/migrate/integrationbranch.go`, `internal/migrate/integrationbranch_test.go`
  - `internal/project/adrnumber.go`, `internal/project/adrnumber_test.go`
  - `cmd/awf/adr.go`, `cmd/awf/adr_test.go`
  - `templates/hooks/pre-merge-commit.sh.tmpl`
  - `.githooks/pre-merge-commit` (this repo's hand-wired stub)
- **Modified:**
  - `internal/adr/adr.go`, `internal/adr/format.go`, `internal/adr/corpus.go`,
    `internal/adr/index.go` (+ their tests)
  - `internal/manifest/manifest.go` (+ test)
  - `internal/currentstate/load.go`, `internal/currentstate/check.go`,
    `internal/currentstate/transition.go` (+ tests)
  - `internal/topic/topic.go` (+ test)
  - `internal/plan/plan.go` (+ test)
  - `internal/project/project.go`, `internal/project/check.go`,
    `internal/project/currentstate.go`, `internal/project/context_adr.go`,
    `internal/project/context_artifacts.go`, `internal/project/render.go` (+ tests)
  - `internal/audit/audit.go`, `internal/upgrade/digest.go`
  - `internal/config/config.go`, `internal/config/edit.go`, `internal/configspec/spec.go`
    (+ tests)
  - `internal/clispec/clispec.go`, `cmd/awf/dispatch.go`, `internal/git` (seam
    entrypoint; exact file per the landed seam layout)
  - `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md`,
    `.awf/topics/parts/adr-system/plan-artifacts/current-state.md`,
    `.awf/topics/parts/config/configuration/current-state.md`,
    `.awf/topics/parts/config/migrations-and-locks/current-state.md`,
    `.awf/topics/parts/invariants/current-state-authority/current-state.md`,
    `.awf/topics/parts/rendering/singletons-and-payloads/current-state.md`
  - `.awf/parts/adr-template/frontmatter.md`, `.awf/parts/workflow/local-hooks.md`,
    the working-with-awf commands part, `.awf/domains/parts/adr-system/current-state.md`
  - `internal/project/scaffold.go` (+ test)
  - `templates/adr-readme/README.md.tmpl`,
    `templates/skills/proposing-adr/SKILL.md.tmpl`,
    `templates/skills/reviewing-adr/SKILL.md.tmpl`
  - `.awf/awf.lock`, `.awf/config.yaml`, `examples/sundial/.awf/awf.lock`,
    `examples/sundial/.awf/config.yaml` (self-migration via `awf upgrade`)
  - `docs/decisions/0190-slug-identified-pending-adrs-numbered-at-integration.md`
    (status history events), `changelog/`, rendered docs via `./x render`
- **Deleted:** none.

## Phase 1: Seam baseline and resync

**Execution mode: inline.** Baseline: `git status --short` empty in this worktree;
`./x gate` green.

- [ ] **Task 1.1: Verify the git-seam ADR landed.** Run
  `git fetch origin 2>/dev/null; git log main --oneline -5` and
  `git ls-tree main:docs/decisions | grep git-access-through-one-semantic-seam`.
  Expected: exactly one `NNNN-git-access-through-one-semantic-seam.md` entry. If the
  grep returns no output, STOP: the plan's hard precondition is unmet; record the block
  in the effort memory and end execution.
- [ ] **Task 1.2: Merge main and resolve the expected ADR-number collision.** Run
  `git merge main`. If main now contains a different ADR numbered 0190, renumber this
  effort's record one last time by hand: rename
  `docs/decisions/0190-slug-identified-pending-adrs-numbered-at-integration.md` to the
  next free number `NNNN-...`, rewrite its `# ADR-0190:` heading to `# ADR-NNNN:`,
  update the `adrs: [190]` entry in this plan's frontmatter to the new number, run
  `./x render` to regenerate `INDEX.md`, and update every `ADR-0190` reference in this
  plan and in `.awf/` parts staged by later phases. ADR-0190 is `Proposed` with no
  Applied events, so no state sequences move. Verification: `./awf check` reports clean.
- [ ] **Task 1.3: Re-verify the seam entrypoint shape.** Read the landed seam ADR and
  the current `internal/git` surface (`grep -n "func (" internal/git/*.go | grep -v
  _test`). Confirm the handle-plus-entrypoint shape (constructed handle, deadlined
  context, contract suites) that Phase 3's Task 3.4 assumes. If the landed shape differs
  materially (no handle, different context rules), add a dated finding to this plan's
  Notes section describing the delta and adapt Task 3.4's wording in the same commit;
  a materially different seam is a plan-resync trigger, not a silent improvisation.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage the resync edits (plan
  frontmatter/Notes, any renumber effects); run `awf check --staged` then `./x gate`;
  commit (the merge itself commits separately as git requires):

```commit
docs(plans): record the seam-landing baseline for the numbering plan
```

## Phase 2: Format V3 core - lock cutoff, pending identity, corpus and checks

**Execution mode: subagent-driven.** Baseline commands: `git status --short` prints
nothing; `./x gate` exits 0; `./awf check` prints clean. This phase closes with ADR-0190
entering `Implementing` and Applied batch 1.

- [ ] **Task 2.1: Lock field `ADRFormatV3From`.** In `internal/manifest/manifest.go`,
  mirror the `ADRFormatV2From` triple exactly (field at :46, presence bit at :57, raw
  presence wiring in `Parse` near :200, canonical `Marshal` struct near :230): add
  `ADRFormatV3From int` (`json:"adrFormatV3From,omitempty"` - the lock manifest is
  JSON, matching the V2 field's tag), `adrFormatV3FromPresent bool`, and
  `_, l.adrFormatV3FromPresent = raw["adrFormatV3From"]`, mirrored in the canonical
  `Marshal` struct literal. In
  `AuthorityState()` (:70-132) add: when present, `ADRFormatV3From` must be positive and
  `>= ADRFormatV2From`; when `SchemaVersion >=` the Task 2.2 migration's `To`, absence
  is an error (mirror the schema-15-requires-V2From check at :107-117). Tests mirror the
  existing V2From cases per branch.
- [ ] **Task 2.2: Sealing migration.** New `internal/migrate/adrformatv3.go` cloned from
  the `applyADRFormatV2CutoffWithSave` shape (`internal/migrate/adrformatv2.go:22-63`):
  `lockSaver` seam, no-op when lock absent or already at/above target, skip the corpus
  branch unless authority is Permanent, otherwise seal
  `lock.ADRFormatV3From = corpus.NextIdentity()`, set `SchemaVersion`/`AWFVersion`,
  save, and print `adr-format-v3-cutoff: sealed ADR V3 cutoff at %d`. Register in
  `internal/migrate/migrate.go`'s registry after the current top entry as
  `{To: <next generation>, Name: "adr-format-v3-cutoff", Apply: ..., OwnsSchemaStamp:
  true}`. Note: another in-flight effort may also append migrations; take the next free
  `To` at execution time and use that same value in Task 2.1's schema floor. Tests
  mirror `adrformatv2` coverage including the injected-save failure branch. Then
  self-migrate both bundled trees: run `./awf upgrade` in the repo root and in
  `examples/sundial`, and stage `.awf/awf.lock` and `examples/sundial/.awf/awf.lock`
  with the phase (the binary-version gate reds every gated command on a
  behind-generation tree, and `runner-example-adopted` reds on a stale sundial).
- [ ] **Task 2.3: V3 parse path and slug identity in `internal/adr`.** In `format.go`:
  add `V3FormatMarker = "current-state-v3"` beside :17-20; add a `v3Frontmatter` struct
  `{Format, Status, Date, Slug string}` decoded with `KnownFields(true)` (V1/V2 parsing
  stays slug-rejecting); add `ParseV3(name, data)` accepting both forms: numbered
  `NNNN-<slug>.md` with heading `# ADR-NNNN: <Title>`, and pending `<slug>.md` with
  heading `# ADR-<slug>: <Title>`. Hard errors: empty or missing `slug:`; slug not equal
  to `slugify(slug)` (reuse `slugify`, `adr.go:328-339`); filename slug segment not
  equal to the frontmatter slug (pending basename `<slug>.md`; numbered `NNNN-<slug>.md`
  suffix); heading identity token not matching the filename form. Add `Slug string` to
  the `ADR` struct; pending records keep `Number == ""`. Extend `FormatBoundaries`
  (:189-192) with `V3From int` and route in `ParseRecord` (:194-219): numbered files
  with `num >= V3From > 0` parse as V3; a file NOT matching `FilenameRe` is now parsed
  instead of skipped - peek the frontmatter `format:` value (via
  `frontmatter`-package split plus the lenient struct) and route `current-state-v3` to
  `ParseV3`; any other numberless file returns the error
  `"<name>: not an ADR record (expected NNNN-*.md or a pending current-state-v3 file)"`.
  V2 behavior below the V3 cutoff is unchanged. internal/adr's own `FilenameRe` uses
  (adr.go:94 and :157, format.go:68 and :197) are in this task's scope: :157 and
  format.go:68 legitimately leave `Number` empty for a pending record.
- [ ] **Task 2.4: Callers stop silently skipping strays; reserved basenames stay
  excluded.** Batch task over the `FilenameRe` gate sites. Representative -
  `internal/adr/adr.go:94-97` (`ParseDir`): replace the match-or-`continue` with:
  `base := filepath.Base(path)`; skip exactly `README.md`, `INDEX.md`, `template.md`;
  everything else goes to `ParseRecord` and a parse error aborts with the file named.
  Edge - `internal/currentstate/load.go:62-70` (`adrsFromTree`): same skip-list change;
  numbers are collected only from records with `Number != ""` so `checkADRContiguity`
  (load.go:83-116) and its duplicate error stay number-scoped. Remaining sites, each
  gaining pending-awareness without the skip-list semantics: `internal/project/
  context_adr.go:34-41` and `internal/project/context_artifacts.go:79-83` (classify a
  pending `<slug>.md` under the decisions dir as a decision-record artifact, resolved
  via the Task 2.5 slug index), `internal/audit/audit.go:437-439` (`isADRFile` also
  true for a non-reserved `.md` directly under the ADR dir), `internal/upgrade/
  digest.go:126` (include pending files in the digest walk),
  `internal/project/currentstate.go:338-349` (`nextADRIdentityFromTree` skips
  `Number == ""` records before `Atoi`). Post-check:
  `grep -rn "adr\.FilenameRe" internal/ cmd/ --include="*.go" | grep -v _test` -
  every match site is one of the sites this task enumerates outside internal/adr
  (internal/plan's own unrelated `FilenameRe` symbol and internal/adr's in-package
  uses, owned by Task 2.3, are out of scope), and each handles the pending case per
  this task.
- [ ] **Task 2.5: Corpus slug index and hard duplicate errors.** In
  `internal/adr/corpus.go`: `NewCorpus` (:45-51) gains a `bySlug map[string]ADR` and
  now returns `(Corpus, error)`: a duplicate non-empty `Number` or a duplicate
  non-empty `Slug` (across pending plus retained records) yields the typed error
  `*DuplicateIdentityError{Numbers, Slugs []string}` whose message reads
  `"ADR number %s is declared by more than one file"` /
  `"ADR slug %q is declared by more than one file"` (closing the silent last-wins
  blindness ADR-0190's Consequences name). The returned `Corpus` is still populated
  (last-wins) alongside the typed error, documented as being for the numbering
  command's refusal path only; every other caller treats the error as fatal. Add `HasSlug(slug string) bool` and `BySlug(slug string) (ADR, bool)`. Update
  every `NewCorpus`/`LoadCorpus` caller (enumerate via `grep -rn "NewCorpus\|LoadCorpus"
  internal/ cmd/ --include="*.go" | grep -v _test`) to propagate the error. In
  `NextNumber` (`adr.go:307-326`), `NextIdentity` (`corpus.go:94-108`), and
  `AdoptionBoundary` (`adr.go:264-305`): guard `if a.Number == "" { continue }` before
  the `Atoi`, and rewrite the three `coverage-ignore` reasons (adr.go:318, corpus.go:100,
  adr.go:280) plus `internal/project/currentstate.go:343` to reflect the guard (or drop
  the ignore where the error branch becomes reachable and test it).
- [ ] **Task 2.6: Slug-form provenance.** In `internal/topic/topic.go`: widen the
  `Origin:`/`Revised-by:` grammar (`adrRE`, :22) to accept `ADR-<slug>` where `<slug>`
  matches `^[a-z0-9]+(-[a-z0-9]+)*$` and is non-numeric; store the reference as its
  string form. In `internal/currentstate/check.go` `checkBackward` (:336-379): the
  operation index key becomes the owning record's identity - `Number` when numbered,
  slug otherwise - and a slug-form `Origin:`/`Revised-by:` entry resolves ONLY against a
  pending record's slug. A slug reference with no matching pending record is the
  finding `"claim %s cites pending ADR-%s which is not in the corpus"` - after
  numbering, a leftover slug reference is therefore an error, which is what forces the
  substitution to be complete. `checkSequences` (:106-135) needs no change: pending
  records' batches enter `bySeq` like any others and stay under the global contiguity
  rule.
- [ ] **Task 2.7: INDEX ordering.** In `internal/adr/index.go`
  `renderIndexSection` (:33-47): replace the plain `Number` string sort with: both
  numbered - compare `Number`; exactly one numbered - the numbered record sorts first;
  neither - compare `Slug`. Golden-update the index tests with a pending fixture
  asserting numbered-first, pending-alphabetical-after ordering.
- [ ] **Task 2.8: Cutoff-set parity sweep.** Batch task: every production site reading
  or sealing `ADRFormatV2From` gains the parallel `ADRFormatV3From` handling (boundary
  construction in `internal/project/currentstate.go:119-130` `attestationBoundaries`
  and :357 `loadTreeCurrentState`; fresh-adoption sealing wherever `AdoptionBoundary`'s
  result is written). Representative: `attestationBoundaries` adds
  `V3From: lock.ADRFormatV3From`. Edge: the fresh-adoption path seals V3From to the
  same value as the V2From seal (highest identity plus one), keeping the ordered-cutoff
  invariant. Post-check: `grep -rln "ADRFormatV2From" internal/ cmd/ --include="*.go" |
  grep -v _test` and the same for `ADRFormatV3From` name identical file sets.
- [ ] **Task 2.9: Claim mutations and ADR transition.** In
  `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md`: rewrite
  `fresh-adoption-v1-cutoff` (cutoff set now ordered V1 <= V2 <= V3, sealing per Task
  2.2/2.8), `adr-status-enum-and-matrix` (three cutoffs; numberless records route by the
  `current-state-v3` marker), `adr-amendable-until-terminal` (contract restated over V2
  and V3), `corpus-single-identity-key` (identity key is the number for numbered
  records and the retained slug for V3 records; a non-reserved file matching neither
  form is a corpus error; duplicates of either key are errors), and add
  `pending-adr-slug-identity` (pending form `<slug>.md` / `# ADR-<slug>:`, slug frozen
  at scaffold, `format: current-state-v3` routing, reserved basenames excluded) and
  `adr-slug-frontmatter-mandatory` (V3 records carry a mandatory `slug:` key equal to
  the filename derivation, retained after numbering; corpus-wide uniqueness over
  slug-carrying records), each `Backing: test` with `Origin: ADR-0190` and Revised-by
  appended on the updates. In
  `.awf/topics/parts/config/migrations-and-locks/current-state.md`: update
  `adr-v2-cutoff-atomic-immutable` from "both permanent format cutoffs" to the full
  ordered cutoff set. Add proof markers `// invariant: <domain>/<topic>:<slug>` on the
  matching tests from Tasks 2.1-2.7. Append to the ADR's Status history: the
  `Implementing` event (with current content digest), then the Applied batch 1 event
  listing, in declaration order: update `fresh-adoption-v1-cutoff`, update
  `adr-status-enum-and-matrix`, update `adr-amendable-until-terminal`, update
  `corpus-single-identity-key`, add `pending-adr-slug-identity`, add
  `adr-slug-frontmatter-mandatory`, update `adr-v2-cutoff-atomic-immutable`, with the
  next unclaimed state sequence. Run `./x render`.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage everything; run
  `awf check --staged` then `./x gate`; both must pass with zero findings.

```commit
feat(adr-system): make pending V3 records first-class corpus members
```

## Phase 3: integrationBranch, branch-aware scaffold, pending block

**Execution mode: subagent-driven.** Baseline: `git status --short` empty; `./x gate`
exits 0; `./awf check` clean.

- [ ] **Task 3.1: Top-level scalar config editor.** In `internal/config/edit.go`, add
  `SetString(src []byte, key, value string) ([]byte, error)`: create-or-replace a
  top-level scalar mapping entry, mirroring `SetArray`'s node handling (:107-122) with a
  scalar value node. Test: create-new, replace-existing, preserved comments/order,
  invalid-yaml error. The `config-serialization-owned` claim's closed editor
  enumeration gains `SetString` (the claim update is declared by ADR-0190 and applies
  in this phase's batch, Task 3.7).
- [ ] **Task 3.2: `integrationBranch` config key.** In `internal/config/config.go`: add
  `IntegrationBranch string` (`yaml:"integrationBranch"`) beside `Prefix` (:44); NO
  default in `ParseTree` (the `Prefix` precedent, not `DocsDir`). Validation: required
  non-empty and free of whitespace; slashes are legal (`release/1.0`); leading `-` is
  rejected. If `Validate` runs on pre-migration trees loaded by `loadForMigration`,
  gate the requiredness on the lock schema being at or past Task 3.3's `To` (verify
  which applies by reading `internal/migrate/configedit.go` and the `loadForMigration`
  call chain; pick the variant that keeps `awf upgrade` runnable on a schema-26 tree
  and write a test proving it). In `internal/configspec/spec.go` add the entry beside
  `prefix` (:78-82) with `Default: "none: required; the schema migration writes
  integrationBranch: main"`; the reflection parity test must pass unmodified. The
  scaffold path must also write the key or `awf init` emits a config that fails its
  own validation: add `IntegrationBranch string` to `config.Skeleton`
  (`internal/config/edit.go:19-30`) and set it to `"main"` in `ScaffoldConfig`
  (`internal/project/scaffold.go:73-88`), with a test asserting a freshly scaffolded
  config validates.
- [ ] **Task 3.3: Migration writing the key.** New
  `internal/migrate/integrationbranch.go` on the `applyOrientingSkillBackfill` shape
  (`orientingbackfill.go:19-38`): inside `editConfig`, call
  `config.SetString(src, "integrationBranch", "main")` only when the key is absent, and
  print `integration-branch-explicit: set integrationBranch: main`. Register
  `{To: <next free>, Name: "integration-branch-explicit", Apply: ...}` (no
  `OwnsSchemaStamp`). Tests: key written visibly, idempotent when present, output line
  exact. Then self-migrate both bundled trees again: `./awf upgrade` in the repo root
  and in `examples/sundial`, staging `.awf/awf.lock`, `.awf/config.yaml`,
  `examples/sundial/.awf/awf.lock`, and `examples/sundial/.awf/config.yaml` (both
  configs gain the visible `integrationBranch: main` line; without it Task 3.2's
  required-key validation reds both trees).
- [ ] **Task 3.4: Seam branch-detection entrypoint.** In `internal/git`, following the
  landed seam ADR's entrypoint pattern (Phase 1 Task 1.3 verified it): add a
  current-branch entrypoint implementing `git symbolic-ref -q --short HEAD` semantics -
  returns the branch name when HEAD is symbolic, and reports detached HEAD as a
  distinct non-error outcome (empty name), matching the probe idiom the seam uses for
  ref absence. Deadlined context per the seam rules; no backend types cross the
  surface. Ship the entrypoint's backend-agnostic contract-suite entry as the seam's
  `pinned-entrypoint-semantics` claim requires (on-branch, detached-HEAD, and
  not-a-repository cases). The existing `symbolic-ref` call inside
  `internal/worktree/manager.go` (Integrate's own-branch refusal) converts to this
  entrypoint if the seam work has not already converted it; after this task,
  `grep -rn "symbolic-ref" internal/ cmd/ --include="*.go" | grep -v _test | grep -v
  internal/git/` returns no output.
- [ ] **Task 3.5: Branch-aware scaffold with refusals.** In
  `internal/project/project.go` `NewADR` (:549-564): resolve the current branch through
  the Task 3.4 entrypoint; positive match against `cfg.IntegrationBranch` scaffolds
  numbered (existing path; format V3 when the allocated number is at or past
  `lock.ADRFormatV3From`); any other outcome (different branch, detached, probe
  failure) scaffolds pending. In `internal/adr` extend the scaffold (`NewFile`,
  adr.go:351-425): a V3 scaffold injects `slug: <slug>` into the frontmatter after the
  format-marker substitution (:379-406); the pending form writes `<slug>.md` with
  heading `# ADR-<slug>: <Title>` (adjust the `replaceOnce` heading fill at :417).
  Refusals before any write, both forms: the slugified title equals a reserved basename
  stem (`readme`, `index`, `template`) - error `"title slugifies to reserved name %q"`;
  the slug already exists in the corpus (pending file, retained `slug:` key, or the
  slug segment of any numbered filename) - error `"slug %q already used by %s"`. Update
  the `new adr` `HelpBody` in `internal/clispec/clispec.go:305-312` to describe the
  branch-conditional shape. Tests: numbered-on-integration-branch,
  pending-elsewhere, pending-on-detached, both refusal paths, slug key present in both
  scaffold outputs.
- [ ] **Task 3.6: Pending block on the integration branch.** In
  `internal/project/check.go`, in the corpus-level check path that already walks
  decisions (near `checkPlans`, :596): when the corpus contains at least one pending
  record AND the Task 3.4 entrypoint positively identifies the current checkout's
  branch as `cfg.IntegrationBranch`, emit drift
  `{Kind: "pending-adr-on-integration-branch", Detail: "<slug>"}` per pending record.
  Detached HEAD, another branch, or a probe error emits nothing (positive
  identification only, per ADR-0190 item 7). Test both firing and all three
  non-firing outcomes with a mocked seam.
- [ ] **Task 3.7: Claim mutations and batch 2.** In the adr-lifecycle part: update
  `adr-new-sequential-numbering` (branch-aware: highest-plus-one numbering on the
  integration branch, pending elsewhere; NextNumber semantics preserved over the
  numbered subset) and `adr-new-heading-matches-file` (covers `# ADR-<slug>:` matching
  `<slug>.md`); add `pending-blocked-from-integration-branch` (check fails on a pending
  record only under positive integration-branch identification). In the
  config/configuration part: add `integration-branch-explicit` (required-explicit key,
  migration writes `integrationBranch: main` visibly, no in-code default, audit range
  resolution never reads it); update `config-serialization-owned` (the closed editor
  enumeration gains the top-level `SetString`). Proof markers on the Task
  3.1/3.2/3.3/3.5/3.6 tests. Placement note: ADR-0190 item 14 names
  internal/currentstate as `pending-blocked-from-integration-branch`'s proof home, but
  the block is a corpus-level drift check implemented in internal/project (Task 3.6),
  so its marker lands on the internal/project test - a deliberate, stated deviation
  (`currentState.testGlobs` admits it). Append Applied batch 2, declaration-ordered:
  update `adr-new-sequential-numbering`, update `adr-new-heading-matches-file`, add
  `pending-blocked-from-integration-branch`, add `integration-branch-explicit`, update
  `config-serialization-owned`; next unclaimed sequence. `./x render`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(config): branch-aware ADR scaffolding behind integrationBranch
```

## Phase 4: Slug-capable plan links

**Execution mode: inline.** Baseline: clean tree, green gate.

- [ ] **Task 4.1: Widen `adrs:` entries.** In `internal/plan/plan.go`: replace
  `ADRs []int` (:31,42) with `ADRs []ADRLink` where
  `type ADRLink struct { Number int; Slug string }` implements `UnmarshalYAML`: an
  integer node fills `Number`; a string node matching the slug grammar fills `Slug`;
  anything else errors naming the entry. In `internal/project/check.go:618-622`: a
  `Number` entry resolves via `corpus.Has(fmt.Sprintf("%04d", n))` as today; a `Slug`
  entry resolves via `corpus.HasSlug` (pending or retained); unresolved entries emit
  the existing `plan-adr-link` drift kind with the slug or number as detail. Update the
  proof-marker test (`internal/project/check_test.go:302`) with slug-resolving and
  slug-unresolved cases; existing numeric plans parse unchanged.
- [ ] **Task 4.2: Claim mutation and batch 3.** In the adr-system/plan-artifacts part:
  update `plan-adr-link-resolved` (an entry is a number resolved against `NNNN-*.md` or
  a slug resolved against a pending file or retained slug key; numbering never rewrites
  plans). Append Applied batch 3: update `plan-adr-link-resolved`; next unclaimed
  sequence. `./x render`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(adr-system): resolve plan adrs links by number or pending slug
```

## Phase 5: awf adr number and the numbering transition

**Execution mode: subagent-driven.** Baseline: clean tree, green gate, clean check.

- [ ] **Task 5.1: Command surface.** In `internal/clispec/clispec.go` add the top-level
  group after the `effort` shape (:169-212):
  `{Name: "adr", Summary: "ADR lifecycle operations", MaxPos: 0, Gating: Gated,
  Children: []Command{{Name: "number", Summary: "Number pending ADRs at integration",
  MinPos: 0, MaxPos: -1, HelpBody: "Usage: awf adr number [<slug>...]\n\nNumber pending
  ADRs after merging the integration branch in and before merging back. Bare invocation
  numbers a single pending ADR; several pending ADRs require explicit slugs in the
  intended order.\n"}}}`. Wire `"adr": runADR` in `cmd/awf/dispatch.go`'s handlers map
  and add `cmd/awf/adr.go` with `runADR` switching on `c.sub == "number"` (the
  `TestHandlerRegistryParity` test in `cmd/awf/main_test.go:44-58` enforces the
  bijection). `runADR` calls `gate(root)` and reaches Task 5.2's engine WITHOUT the
  eager corpus load: verify the project-open path used (internal/project/project.go
  near :474 calls `adr.LoadCorpus`, which Task 2.5 makes fatal on duplicates); if open
  loads the corpus eagerly, add a narrow open variant for this command so the typed
  duplicate error reaches the engine's refusal logic instead of aborting the open.
  `runADR` prints the engine's report to stdout. The gated-command enumeration regenerates on
  render with no manual doc edit.
- [ ] **Task 5.2: Numbering engine.** New `internal/project/adrnumber.go`:
  `func (p *Project) NumberPendingADRs(slugs []string) (NumberingReport, error)`.
  Behavior, in order:
  1. Load the corpus through the corpus seam (`corpus-parsed-once` binds), catching
     Task 2.5's `*DuplicateIdentityError` and keeping its populated corpus: duplicates
     are data for step 2, not an abort. MUST NOT precondition on a green full check -
     the red-gate window is the expected operating state.
  2. Refusals: duplicate numbers present and at least one pending record - error
     `"duplicate ADR numbers present; resolve the corpus before numbering"`; no
     pending records and duplicate numbers present - error embedding the
     recipe hint verbatim: `"duplicate ADR numbers with no pending record: if a stale
     numbering commit collided, run: git reset --hard HEAD~1 && git merge <integration
     branch> && awf adr number, then gate and merge back"`; no pending records and no
     duplicates - error `"no pending ADR to number"`; multiple pending and no args -
     error listing every pending slug, one per line; an arg naming a non-pending slug -
     error naming it.
  3. Assignment order: explicit args order, else the single pending record. For each:
     next number = highest existing number plus one at assignment time (incrementing
     across multiple assignments); rename `<slug>.md` to `NNNN-<slug>.md`; rewrite the
     heading to `# ADR-NNNN: <Title>`; the `slug:` key stays.
  4. Sequence shifts: collect the pending records' Applied events; new sequences start
     at the highest sequence held by any numbered record plus one, assigned in
     numbering order then ascending original sequence within each record, preserving
     relative order; rewrite only the `state-sequence: <n>` values in those records'
     Status history lines.
  5. Substitution: over `.awf/topics/parts/**/current-state.md` files only, on lines
     beginning `Origin:` or `Revised-by:`, replace the exact token `ADR-<slug>` with
     `ADR-NNNN` for each mapping. Never touch generated files, plans, or ADR bodies.
  6. Re-render (the same path `./x render` drives) so generated topic docs and INDEX
     match.
  7. Report: one `<slug> -> NNNN` line per assignment plus one
     `state-sequence <old> -> <new> (<file>)` line per shift, in a stable order fit for
     pasting into the integration commit message.
  Forbidden: modifying any record that already has a number (beyond nothing), touching
  plan files, or writing outside the enumerated effects. Tests cover every refusal, the
  multi-pending ordering, the shift arithmetic, the substitution's line anchoring (a
  body mention of `ADR-<slug>` is NOT rewritten), and report formatting.
- [ ] **Task 5.3: Slug-paired numbering transition validation.** In
  `internal/currentstate/transition.go`: the pairing key (`byNumber`, :410-417, and
  `pairOps`' lookups) becomes identity-based - a record's pair key is its `Slug` when
  non-empty (V3), else its `Number` - so a pending record pairs with its numbered
  successor and two pending records never collide on `""`. For a pair whose before is
  pending and after is numbered with the same slug, validate the numbering shape:
  permitted deltas are exactly the `Number`/filename/heading gain, an order-preserving
  rewrite of the record's own Applied-event sequences (every event field except
  `Sequence` equal, relative order preserved - a sequence-modulo variant of
  `historiesEqual`, format.go:178-185), and, in claims, `Origin:`/`Revised-by:` entries
  changing from `ADR-<that slug>` to `ADR-<that pair's new number>` (relax the
  unconditional rejections at transition.go:347-349 and :368 for exactly this
  substitution); everything else about the pair must be byte-identical per the existing
  rules. No new `TransitionMode` value: the relaxation keys off the pending-to-numbered
  pair shape itself, is comment-cited to this ADR (use its final integrated number),
  and composes with both `AuthoredCommit` and `MergeAggregate`. A transition deleting a
  pending record without a slug-paired numbered successor remains the existing
  ADR-deletion error. Tests: legal numbering pair passes; sequence reorder fails;
  body-content delta fails; Origin substitution without the paired numbering fails;
  pending deletion fails.
- [ ] **Task 5.4: Claim mutations and batch 4.** In the adr-lifecycle part: add
  `numbering-transition-mode` (staged validation admits the pending-to-numbered
  slug-paired shape permitting exactly the ADR's item-9 effects; the command never
  preconditions on a green check) and `adr-number-immutable` (a number once assigned
  never changes; stale numbering is unmade by reset-remake; the command refuses a
  duplicate-number corpus with the recipe hint and uses no git provenance); update
  `applied-history-events-append-only` (prefix-append-only except the sanctioned
  numbering rewrite of a pending record's own sequences). In the
  invariants/current-state-authority part: update `application-batch-sequence-order`
  (the shared contiguous namespace spans V1, V2, and V3 batches; the numbering
  transition re-slots pending batches after the highest numbered sequence). Proof
  markers on Task 5.2/5.3 tests. Append Applied batch 4, declaration-ordered: update
  `applied-history-events-append-only`, add `numbering-transition-mode`, add
  `adr-number-immutable`, update `application-batch-sequence-order`; next unclaimed
  sequence. `./x render`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(adr-system): add awf adr number and its numbering transition
```

## Phase 6: pre-merge-commit payload, docs, changelog

**Execution mode: inline.** Baseline: clean tree, green gate, clean check.

- [ ] **Task 6.1: Fourth hook payload.** Create
  `templates/hooks/pre-merge-commit.sh.tmpl` carrying the same staged-check invocation
  as `templates/hooks/pre-commit.sh.tmpl` (the `awf check --staged` line and its
  interpolated command var; omit the prose/memory gate blocks - this hook's single job
  is the duplicate-identity and transition backstop on a conflict-free true merge
  commit). The embed directive (`templates/embed.go:6`) already covers the directory.
  Add `"pre-merge-commit"` to `hookNames` (`internal/project/render.go:40`); the render
  loop (:628-637) and var collection (`internal/project/configreference.go:60`) pick it
  up. Create this repo's executable stub `.githooks/pre-merge-commit` as the one-line
  delegate `exec bash .awf/hooks/pre-merge-commit.sh "$@"` (mode 755, matching
  `.githooks/commit-msg`). Update the rendered-payload tests for the four-payload set.
- [ ] **Task 6.2: Documentation obligations.** Update the authored sources named by
  ADR-0190 item 15: `.awf/parts/adr-template/frontmatter.md` so a V3 scaffold's output
  carries the `slug:` key and the pending shape (keep every interpolation
  publication-safe); the working-with-awf commands part (document `awf adr number` and
  the merge-in, number, merge-back procedure including the reset-remake retry recipe);
  `.awf/parts/workflow/local-hooks.md` (four payloads and the merge-commit backstop);
  `.awf/domains/parts/adr-system/current-state.md` (the two-cutoff opening becomes the
  ordered three-cutoff set; pending identity and numbering-at-integration described).
  Update the three shipped templates that still teach the numbered-only convention:
  `templates/adr-readme/README.md.tmpl` (:30, :33, :48 - `NNNN-kebab-title.md`,
  next-available-number, `# ADR-NNNN:`), `templates/skills/proposing-adr/SKILL.md.tmpl`
  (:29, :42 - "next sequential number"), and the matching mention in
  `templates/skills/reviewing-adr/SKILL.md.tmpl`, describing the branch-conditional
  scaffold output and the merge-in, number, merge-back step; keep interpolations
  publication-safe and golden-update the residue tests.
  Add the `[Unreleased]` changelog entry naming: pending slug ADRs, `awf adr number`,
  the required `integrationBranch` key written by migration, the fourth hook payload
  and its manual stub wiring, and the stray-file corpus error behavior change.
- [ ] **Task 6.3: Claim mutation and batch 5.** In the
  rendering/singletons-and-payloads part: update `hook-payloads-rendered` (exactly four
  payloads including pre-merge-commit; absence when disabled unchanged). Proof marker on
  the Task 6.1 test. Append Applied batch 5 (the remainder): update
  `hook-payloads-rendered`; next unclaimed sequence. Directly after it, append the
  `Implemented` status event repeating the latest content digest (the V2 final pair;
  0187/0189 precedent pairs the final batch and the flip in one commit, and an
  `Implementing` record with nothing remaining is refused by
  `internal/adr/application.go:113-116`). `./x render`.
- [ ] **Phase-close: stage, check, gate, and commit.**

```commit
feat(rendering): render the pre-merge-commit duplicate-identity backstop
```

## Verification

- `./x gate` exits 0 (coverage 100%, deadcode clean, prose and memory scans clean) and
  `./awf check` prints clean at the final phase close.
- End-to-end numbering rehearsal in a scratch worktree of this repo: scaffold a pending
  ADR off the integration branch (`awf new adr` produces `<slug>.md`), implement a
  trivial claim against it, run `awf adr number`, and verify: the file and heading are
  numbered, the slug key survives, `Origin:` lines substituted, sequences contiguous,
  `awf check` clean, and the printed mapping matches the git diff. Then re-run
  `awf adr number` and verify the `"no pending ADR to number"` refusal.
- On the integration branch, `awf new adr` still scaffolds numbered; a tree with a
  pending record checked out on the integration branch fails `awf check` with
  `pending-adr-on-integration-branch`; the same tree on a detached HEAD passes.
- `awf upgrade` on a pre-change tree prints the `integration-branch-explicit` and
  `adr-format-v3-cutoff` lines and leaves `integrationBranch: main` visible in
  config.yaml.
- ADR-0190's Status history shows Implementing, five Applied batches covering every
  declared operation exactly once, and the closing Implemented flip event; `awf check`
  accepts the final state.

## Notes

- Sequencing: Phase 1 hard-gates on the git-seam ADR having landed; Task 1.3 records
  and routes any material seam-shape drift through plan resync instead of silent
  adaptation.
- This plan's own ADR link (`adrs: [190]`) is optimistic; Task 1.2 renumbers it if the
  seam ADR (or anything else) lands as 0190 first - the last manual renumber this
  repository should ever need.
- No adr-lifecycle topic split despite the claim-budget advisory (user ruling
  2026-07-31; the `maxClaimsPerTopic` limit is being removed by a parallel effort). If
  the advisory still fires at execution time it is non-failing noise, not a task.
- Migration `To` values and state sequences are taken fresh at execution time (parallel
  efforts may consume generations and sequences first); the plan asserts methods, never
  counts.
- The ADR's Implemented flip lands with the final batch in Phase 6's close (an
  `Implementing` record with nothing remaining is an illegal state); the plan's own
  `status: Implemented` freeze still lands in the deferred post-review transaction.
