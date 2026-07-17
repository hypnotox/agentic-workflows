# awf Deep Analysis 2026-07-16 - Code Quality and Architecture

*Part of the 2026-07-16 analysis set (companion to `00-overview.md`); a one-day
delta on [deep-analysis-2026-07-15.md](deep-analysis-2026-07-15.md) and the
[landscape report](agentic-workflow-landscape-and-awf-standing-2026-07.md).
Scope: the import DAG and composition root, the `internal/project` god-package
trajectory, the enabling snapshot refactor, code-idiom debts (banned-rune
triplication, error model, package-doc thinness, function-length outliers, ADR
back-pointer currency, coverage-ignore growth). Almost every finding here is a
maintainability judgment (ANALYST-OPINION), not a proven defect; each is marked.*

## In one paragraph

The architecture is genuinely well-layered and the code idioms are unusually
uniform for agent-written, high-velocity Go: a strictly acyclic import DAG with
`internal/project` as a real composition root entered only by mains and a
test-only package, single-authority leaf packages, a declarative spec-package
pattern that structurally prevents doc-vs-dispatch drift, ADR back-pointer
comments that spot-check accurate at item granularity, and a clean placement
discipline where new concerns land in new files. The one structural liability is
concentrated in exactly one place and is still growing: `internal/project` is now
a 5,548-LOC, 85-method composition root whose `check.go` (975 LOC) double-renders
the whole artifact set and re-parses the 124-file decisions directory roughly a
dozen times per run, because no shared rendered-and-parsed snapshot value exists.
The 2026-07-15 recommendation to split `check.go`, decompose `SyncReport`, and add
a `doc.go` saw no action; in the single day since, the package grew by ~294 LOC as
ADR-0122/0123/0124 landed. A single refactor (introduce a `renderedSnapshot`
value) would enable the `check.go` split, the `SyncReport` decomposition, and the
`effSkills` decoupling in one move. The delta introduced one new smell (the
seven-codepoint banned-rune set now lives in three hand-synced copies bound only
by comments) and one stale-comment family (three "byte-for-byte verbatim
(ADR-0034)" comments that ADR-0121 quietly retired). None of this is a defect
today; it is accreting maintenance cost that the project's own discipline is
otherwise built to prevent.

## What is genuinely good

Lead honestly: the engineering here is real, and most of the architecture is a
model of the discipline the project preaches.

- **Acyclic, shallow DAG with a real composition root.** `go list` confirms zero
  cycles across the 32 packages. `internal/project` (15 internal imports) is a
  deliberate composition root: it is imported only by the two mains
  (`cmd/awf`, `cmd/releasecheck`) and the test-only `internal/evals`; no package
  below it reaches up. The punctuation wave (ADR-0115/0118/0119) and the ADR-0120
  supersession work added no new package edges, so the "clean acyclic layering"
  verdict from 07-15 still holds exactly.
- **Single-authority leaf discipline.** `internal/pathglob` pins the one glob
  dialect for its four importers; `internal/refs` is pure stdlib link extraction;
  `internal/render` stays a pure leaf and absorbed the ADR-0121 comment stripper
  as `internal/render/comment.go` rather than as project logic. The ADR-0119
  banned-rune scanner landed as a cohesive stdlib-only leaf (`internal/prosegate`)
  wired directly from `cmd/awf`, correctly bypassing `project`.
- **The spec-package pattern is the standout move.** `internal/clispec` (data
  only, no handlers, no upward imports) feeds both `cmd/awf`'s runtime dispatcher
  and `internal/project`'s generated command docs; `internal/configspec` does the
  same for the config surface. The generated reference *cannot* drift from the
  real dispatch/config surface because both read one declarative source. A
  fails-when-stale parity test (`TestHandlerRegistryParity`) pins the handler map
  to `clispec`.
- **Placement discipline under accretion.** New concerns land in new files:
  ADR-0120 supersession went to `internal/project/supersession.go`, ADR-0119 to a
  new package, ADR-0121 to `internal/render/comment.go`. `check.go` grew only ~10
  lines across three feature waves precisely because the bodies went elsewhere.
- **Idiom uniformity at velocity.** gofmt/goimports clean across 215 Go files,
  zero TODO/FIXME/HACK in production, panics confined to `internal/testsupport`,
  and zero interface types anywhere in production - the concrete-only design is
  applied without exception. Tests are table-driven and stdlib-only (no testify),
  with a functional-options ADR fixture builder in testsupport.
- **ADR back-pointer discipline is executed, not aspirational.** A spot-check of
  11 comment sites (including ADRs landed within 24 hours: ADR-0120 items 7/10/12,
  ADR-0121 Decision 2/5) found all 11 accurate at item granularity; a separate
  stale ADR-0034 comment family (below) is the lone exception in the broader
  comment corpus. No production comment cites a fully superseded ADR. This matters because ADR-0116 deliberately
  keeps back-pointer currency *procedural, not checked*, so the discipline is the
  only thing holding it.
- **Package docs are mostly substantive.** `internal/invariants`, `clispec`,
  `configspec`, `migrate`, `git`, and `prosegate` carry multi-sentence contract
  docs (added 2026-06-24 and maintained). The 07-15 blanket "package docs are
  one-liners" claim was overstated; it holds mainly for `internal/project`, the
  one package where it matters most.

## The god-package trajectory (ANALYST-OPINION, smell)

This is the single structural liability, and it is the same one 07-15 named. The
split recommendation was not acted on, and the trend line is up, not flat.

| Metric | 07-15 | 07-16 (slice, ff7d9e2b) | 07-17 (live, 7a06af73) |
|---|---|---|---|
| `internal/project` non-test LOC | 4,953 | 5,254 | 5,548 |
| `*Project` methods | 81 | 85 | 85 |
| `check.go` LOC | 955 | 965 | 975 |
| `SyncReport` LOC | 182 | 183 | 183 |
| `project` `doc.go` | none | none | none |

The slice was computed at commit ff7d9e2b (2026-07-16). The working tree has since
advanced to 7a06af73, landing ADR-0122 (multi-runtime targets), ADR-0123 (Pi
workflow subagents), and ADR-0124 (deterministic output plans). That work added
~294 LOC to `internal/project` in a single day - so the delta since the slice
*reinforces* the finding: the package is not merely holding steady, it is the
place every new render concern accretes. Fair framing, unchanged from the slice:
only a day or two has elapsed and it was spent landing features, so this is "not
yet" rather than "declined." But the direction is unambiguous.

Two structural signals point at the same fix and are worth stating precisely
because they are the levers for the deferred split:

**1. `awf check` double-renders and re-parses the corpus, with no shared snapshot
value.** CONFIRMED by direct read. `cmd/awf/check.go:25` calls `p.AdvisoryNotes()`
then `cmd/awf/check.go:35` calls `p.Check()`. `AdvisoryNotes`
(`internal/project/check.go:28-44`) runs `RenderAll` + `generateDomainDocs` +
`generateConfigReference`; `Check` (`internal/project/check.go:466+`) runs all
three again. `adr.ParseDir` over the 124-file decisions directory is called at
`check.go:81`, `:773`, `:860`, `:912`, `:957`, plus `supersession.go:23` and
`:69`, plus the ACTIVE.md and per-domain regenerations inside the domain-doc
pass - roughly a dozen parses per run. Wall time is fine today (`awf check` runs
in well under a second, gate green), so this is *structural*, not a performance
defect: every new checker re-derives its inputs because none are threaded.

**2. Hidden temporal coupling via `effSkills`.** CONFIRMED by direct read.
`Project.effSkills` (`internal/project/project.go:45-49`) is documented as
"populated by RenderAll"; it is assigned at `render.go:438` and consumed by
`checkDeadSkillRefs` at `check.go:534`. `Check()` only works because `RenderAll`
happens to run first in the same method. Calling `checkDeadSkillRefs` on a fresh
`Project` would silently see a nil map (every skill reported disabled) rather than
failing loudly. Mutable order-dependent state on the composition root is exactly
what a snapshot value eliminates.

## The enabling refactor: one `renderedSnapshot` value

The single highest-leverage move for this topic. Introduce a `renderedSnapshot`
struct computed once per command - `{files, activeMD, domainDocs, configRef,
effSkills, parsed ADRs, parsed plans}` - and pass it to `Check`, `AdvisoryNotes`,
and `SyncReport`. This one change:

- halves `check`'s render work and collapses the ~dozen `ParseDir` calls to one;
- deletes the `effSkills` field and its temporal coupling (return it in the
  snapshot instead);
- gives `SyncReport` and the check cluster the explicit parameter surface the
  split needs;
- should *shrink* the coverage-ignore count, because several of `check.go`'s
  "pre-empted by an earlier Check() step" ignore comments
  (`check.go:528`, `:542`, `:547`, `:552`, `:557`) exist only to justify
  unreachable re-read error branches that disappear when inputs are threaded
  instead of re-derived.

### Best seams for the deferred `check.go` split

Verified by reading the dependency surface of each cluster, these are the cleanest
cut lines, in the order they should be taken:

1. **Docs-corpus cluster first (zero render dependency).** `checkPlans`,
   `checkPitfalls`, `checkTagVocabulary`, `checkADRRelatedLinks`, and the
   supersession checkers (`check.go:769` onward plus `supersession.go`) touch only
   `p.Root`, `p.Cfg`, and parse results - never `RenderAll` output. This cluster
   can move today to a `corpus.go` and later to an `internal/corpus` package that
   takes parse-once inputs, which is also what kills the repeated `adr.ParseDir`
   calls. ~209 LOC and ~6 methods leave `check.go` in this cut.
2. **Advisory-notes cluster** (`AdvisoryNotes`, `unsetVarNotes`, `stubNotes`,
   `markerNotes`, `tagHealthNotes`, `check.go:28-233`, ~205 LOC).
3. **Lock-vs-rendered drift cluster** (`checkLockedFiles`, `regenDrift`,
   `checkActiveMD`, `checkDomainDocs`, `checkConfigReference`) becomes `drift.go`
   over `(lock, snapshot)`.

Result: `check.go` drops to roughly half its size (~550 LOC of pure
lock-vs-rendered drift) and `*Project` sheds ~10 methods.

### `SyncReport` decomposition

CONFIRMED shape: `SyncReport` (`internal/project/project.go:103-285`) is a 183-LOC
six-phase pipeline returning a positional 4-tuple
(`func (p *Project) SyncReport() ([]Backup, []Change, []string, error)`). The
phases are well-commented and sequential, so this is accretion, not tangle. The
provenance-classification loop at `project.go:250-283` is *pure* (it derives
`added`/`template`/`config`/`template+config`/`regenerated`/`internal` causes from
`old` and `lock` alone) and is trivially unit-testable in isolation. Decompose
into `assembleSnapshot()` (render + generate, shared with `Check`),
`persist(snapshot)` (backup/write/chmod/prune), and `provenance(old, lock)`, and
return a `SyncResult` struct instead of the 4-tuple.

## The banned-rune triplication (ANALYST-OPINION, smell; the one delta regression)

CONFIRMED by direct read. The ADR-0115 seven-codepoint banned set now exists in
three hand-maintained copies:

- `prosegate.Banned` (`internal/prosegate/prosegate.go:24`, exported, the blocking
  gate);
- `bannedProseRunes` (`internal/audit/audit.go:421`, unexported, the advisory
  ADR-0117 rule);
- `bannedRunes` (`internal/project/residue_scan_test.go:172`, the residue-scan
  test).

`prosegate.go`'s own comment says the residue-scan map "must stay in agreement
with this one," but `grep -rn prosegate internal/audit internal/project` returns
nothing: no import, no cross-check test binds them. If one copy is edited alone,
the advisory audit rule and the blocking prose-gate silently diverge on which
glyphs they flag. This is the one architectural regression in the delta, and it is
notable precisely because it is the *opposite* of the fails-when-stale pattern the
repo applies everywhere else (`nonHandoffRequires`, the requiresSkills sweep,
`TestHandlerRegistryParity`).

**Fix (roughly one hour):** make `prosegate.Banned` the single authority.
`prosegate` is a stdlib-only leaf, so `internal/audit` importing it is acyclic;
`residue_scan_test.go` asserts equality against it instead of re-declaring the
map. A related, smaller instance: `cmd/repoaudit/main.go:180` re-declares
`coverageIgnoreMarker` as a split literal with a comment noting it duplicates
`internal/coverage/coverage.go:23`'s unexported `marker`; export that const and
delete the copy (repoaudit is in-module and `covercheck` already imports the
package).

## The stale ADR-0034 comment family (ANALYST-OPINION, smell)

CONFIRMED behavior, opinion on impact. Three production comments still describe the
verbatim-parts contract that ADR-0121 (landed 2026-07-16) formally superseded:

- `internal/render/section.go:71` - "parts render byte-for-byte verbatim, marker
  included (ADR-0034, ADR-0070)";
- `internal/project/local.go:23` - "Parts are raw (ADR-0034)";
- `internal/project/render.go:700` - a coverage-ignore reason citing "raw
  convention parts (ADR-0034)".

The code no longer does what these say. `internal/project/render.go:232-247` runs
`render.StripAuthoringComments` then `substitutePlaceholders` and only *then*
`render.HasStubMarker`, so the body handed to the marker check is stripped and
substituted, not byte-for-byte verbatim. ADR-0121 Decision 4 re-read the contract
as "verbatim except whole-line `awf:comment` lines." The flip commit renamed the
proof markers (`render.go` and `render_test.go` now say
`parts-raw-except-authoring-comments`) but left the prose behind. This is
demonstrable inaccuracy, not a matter of taste; what is opinion is only whether it
warrants a fix now. It is the exact failure mode ADR-0116 accepts by choosing not
to machine-check back-pointer currency - so it is on-strategy that no gate caught
it, but it is still drift the "docs travel with the change" invariant would
prefer swept. **Fix:** one `docs(rendering)` commit updating the three sites to
the successor reading, and a decision on whether ADR-0070's own
`stub-part-verbatim` slug needs the same narrow re-reading or a
`supersedes-invariant` token.

**Opportunity worth naming:** ADR-0120's supersession index already knows every
retired ADR#item anchor. A curated `{superseded-anchor -> banned-phrase}` map in
`cmd/repoaudit`, in the fails-when-stale style the repo already uses, would flag
exactly the stale-ADR-0034 failure without violating ADR-0116's "not a blocking
check" stance (repoaudit is advisory). This is the cheapest way to close the one
back-pointer edge ADR-0116 deliberately left open.

## Error-handling idioms (ANALYST-OPINION, smell)

CONFIRMED state, unchanged since 07-15 item 5. Production defines exactly one error
type (`usageErr`, `cmd/awf/main.go:117`) and zero `var Err...` sentinels
(`grep 'var Err' over production Go returns 0`). Every `errors.Is` target in
production is `os`/`fs.ErrNotExist` (17 sites) plus one external go-git
`ErrRepositoryNotExists`; there is no project sentinel any caller can branch on
except the single CLI-misuse choke point at `dispatchErr` (`main.go:104-119`).
Tests couple to message wording at 104 substring-on-error-message sites
(e.g. `internal/project/placeholders_test.go` asserting the substring
`commitScopeTable`).

Calibration matters here and cuts both ways. This is *partly deliberate*: the
project's "the error is the repair instruction" doctrine makes messages a user
contract, and substring asserts (not exact-equality) tolerate rewording around the
asserted token. But any reworded key token is still a silent multi-test edit, and
no programmatic caller can branch on failure kind. **Proportionate fix:**
introduce 3 to 5 sentinels only at the seams tests hit most and callers could
branch on (corrupt lock, missing lock, not-an-awf-project, malformed part),
migrate the densest substring clusters to `errors.Is`, and keep
message-as-contract everywhere else. This shrinks the substring corpus without
building an error taxonomy a CLI does not need.

## Smaller idiom findings

- **Package-doc thinness (ANALYST-OPINION, nit).** `internal/project`
  (`project.go:1`) opens with a single line; the largest, hardest package in the
  tree has no `doc.go` orientation while far smaller packages carry multi-sentence
  contracts. staticcheck ST1000 is disabled, so nothing forces it. A 30-line
  `doc.go` mapping the file-per-concern layout (render/check/sweep/supersession/
  context) is the cheapest newcomer aid and was already asked for on 07-15.
- **Divergent fence tracker (ANALYST-OPINION, nit; not-confirmed as a live bug).**
  `internal/plan/plan.go:88` `commitSubjects` only toggles on ` ``` ` fences,
  while the tree's two other fence walkers - `refs.WithoutFences`
  (`refs.go:41-44`) and `render.StripAuthoringComments` (`comment.go:38-42`) -
  both treat ` ``` ` and `~~~` as openers. The divergence is real and CONFIRMED by
  read, but the slice's adversarial pass downgraded the original "risk" framing to
  nit: triggering misbehavior needs a `~~~`-wrapped ` ```commit ` block, which no
  current plan contains (the six plans with `~~~` lines all nest them safely inside
  ` ``` ` fences). ADR-0121's Context documents why `WithoutFences` could not be
  reused for the strip (drop-lines vs keep-lines), but no shared line-walker
  primitive was extracted, so this is now the third hand-rolled fence walker. The
  clean fix is one fence-walking primitive (callback per line with an in-fence
  flag) that all three consume; at minimum teach `commitSubjects` the `~~~`
  opener.
- **Enable/disable graph logic in package main (ANALYST-OPINION, nit).**
  `cmd/awf/list_add.go` (the largest cmd file) holds the requirement-closure
  planner for `enable` and the transitive-dependents refusal for `disable`, while
  `internal/catalog` holds only the `RequiresSkills` data. The algorithm is
  testable only through package main. Co-locating it with its data (the spec
  pattern the repo uses everywhere else) would complete the thin-main architecture.
- **Same-concept casing split (ANALYST-OPINION, nit).** `ActiveMd` (fields)
  coexists with `ActiveMD`/`RenderActiveMD` (functions) in adjacent code
  (`layout.go:20` assigns `ActiveMd: lay.ActiveMd` in the same file that calls
  `generateActiveMD`). ST1003 is disabled, so no linter arbitrates; harmless but a
  grep for one spelling misses half the sites. One chore rename settles it.
- **Error-message prefix depth varies by package (ANALYST-OPINION, nit).** Leaf
  packages self-prefix their errors (`coverage: ...` at
  `internal/coverage/coverage.go:310`, `changelog: ...` at
  `internal/changelog/changelog.go:62`, `retirement-tokens: ...` in
  `internal/migrate/retirementtokens.go:93`) while `internal/project` returns bare
  messages (`no lock (run awf sync)` at `check.go:473`). It reads inconsistent, but
  it is harmless: every path is uniformly re-prefixed `awf: ` at the `dispatchErr`
  choke point (`cmd/awf/main.go:107`), so a package prefix is redundant rather than
  wrong. Not worth a dedicated change; drop the leaf prefixes opportunistically if
  the sentinel work above touches those seams.
- **Function-length outliers (context, not a finding).** The top five production
  functions are `SyncReport` (183), `ContextFor` (`context.go:83`, 180), `RenderAll`
  (`render.go`, 158), `applyRetirementTokens` (`internal/migrate`, 142), and
  `runInit` (`cmd/awf/init.go`, 123). Only `SyncReport` has a clean decomposition
  seam; the others are cohesive.

## Coverage-ignore growth as it bears on code shape (ANALYST-OPINION, smell)

The live tree carries ~204 non-test `coverage-ignore` markers (221 repo-wide;
the analysis snapshot cited 219). The count is honestly annotated and abuse is
low, but it drives visible ceremony: the densest cluster is in
`check.go`/`render.go`, and a recognizable subset are the "pre-empted by an
earlier Check() step" markers whose only reason to exist is that each checker
re-derives inputs an earlier step already produced. The snapshot refactor above
would delete that subset outright rather than force each new checker to re-justify
an unreachable re-read branch. The `os.Stat`/`Chmod`/`copyFile` fault cluster
(flagged 07-15) remains testable with an injected FS. This is not a coverage-gate
weakness; it is evidence that `check.go`'s shape forces the ceremony.

## Delta since 2026-07-15

- **The Tier 3 code-health recommendation was not acted on.** Split `check.go`,
  decompose `SyncReport`, add `project/doc.go`: none landed. `check.go` grew
  955 -> 975, the package grew 4,953 -> 5,548, `SyncReport` is byte-for-byte the
  same shape. Charitably "not yet" (the interval was spent landing
  ADR-0115..0124), but the trajectory is up.
- **ADR-0119 landed clean but introduced the banned-rune triplication** - the one
  architectural regression in the delta (see above).
- **ADR-0120 supersession landed as new files** (`supersession.go`, plus
  `parseRefs`/`SupersessionIndex` in `internal/adr`), continuing the by-file
  mitigation - but it added two more `adr.ParseDir` call sites and one more checker
  onto the very orchestration the 07-15 report said to split, worsening the
  parse-count symptom.
- **ADR-0121 landed exemplary placement** (a pure function in the `internal/render`
  leaf, one wiring point) but produced the stale ADR-0034 comment family, because
  the marker rename was done and the prose sweep was not.
- **ADR-0122/0123/0124 (07-17, past the slice)** added multi-runtime targets and Pi
  subagents; the ~294-LOC single-day growth in `internal/project` they caused is
  the freshest evidence that the god-package is where render concerns land.
- **The DAG shape is unchanged.** No delta ADR added a package edge; the
  composition-root claim still holds exactly (only mains plus test-only `evals`
  import `project`).
- **The 07-15 "package docs are one-liners" claim is corrected here**: most are
  substantive; it holds only for `internal/project`.

## Prioritized closing list for this topic

1. **Introduce the `renderedSnapshot` value.** Highest leverage. Enables the
   `check.go` split, the `SyncReport` decomposition, and the `effSkills`
   decoupling in one refactor, and shrinks the coverage-ignore cluster. Do this
   before the package passes ~5.7k LOC.
2. **Promote `prosegate.Banned` to the single banned-rune authority** (audit
   imports it; the residue-scan test asserts equality). Converts a comment-bound
   invariant into the fails-when-stale form the repo prefers; roughly one hour.
3. **Sweep the three stale ADR-0034 comments** to the ADR-0121 successor reading
   in one `docs(rendering)` commit, and decide ADR-0070's slug fate.
4. **Execute the docs-corpus split seam** (cut 1 above) as the first concrete step
   of the deferred `check.go` split, even ahead of the full snapshot refactor - it
   has zero render dependency and moves ~209 LOC out today.
5. **Add `internal/project/doc.go`** mapping the file-per-concern layout.
6. **Introduce 3 to 5 sentinel errors** at the highest-traffic seams; keep
   message-as-contract elsewhere.
7. **Nits, batch when convenient:** teach `commitSubjects` the `~~~` opener (or
   extract the shared fence primitive), unify `ActiveMd`/`ActiveMD`, co-locate the
   enable/disable graph logic with `internal/catalog`, and export
   `coverageIgnoreMarker`.

*Cross-links: [00-overview.md](00-overview.md);
[deep-analysis-2026-07-15.md](deep-analysis-2026-07-15.md) (Tier 3 items 8-11);
[agentic-workflow-landscape-and-awf-standing-2026-07.md](agentic-workflow-landscape-and-awf-standing-2026-07.md).
Sibling 2026-07-16 reports carry the guardrail-system, rendered-output, and
proportionality dimensions.*
