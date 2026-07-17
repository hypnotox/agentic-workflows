# awf Deep Analysis 2026-07-16, Part 04: Roadmap, Adoption, and Field Standing

*Part of the 2026-07-16 analysis set (companion to `00-overview.md`; siblings cover
code/architecture, guardrails, and rendered output). A one-day delta on
`deep-analysis-2026-07-15.md` and the field reference
`agentic-workflow-landscape-and-awf-standing-2026-07.md`. Tree at commit
`ff7d9e2b`, adding ADR-0112 through ADR-0121 on top of the 07-15 baseline.*

*Currency note (2026-07-17): this report's analysis baseline stays `ff7d9e2b`; as of
HEAD `7a06af73`, `docs/roadmap.md` is now enabled in awf's own tree and ADR-0122 through
ADR-0124 (multi-runtime targets, Pi subagent extension, output plans) have landed. Where
that moots a recommendation below it is marked inline. Note that the next window's actual
output was more rendering/adapter breadth and a new subagent execution mode, not the
content-accuracy or provenance moat gaps this report flags, which corroborates the
polish/breadth-over-moat reading in section (4).*

## In one paragraph

The machinery is ahead of the story. Release engineering, adoption speed, and
field position are all genuinely strong: `awf init` renders a clean, publication-safe
adoption in about 0.05 s, every remote CI action is SHA-pinned and gate-enforced,
the version-authority chain is internally consistent, and awf still leads the field
on the two pillars where the field is thinnest (drift/provenance productization and
process-conformance audit), with ADR-0120's machine-checked supersession graph the
one real moat deepener in the delta. But the last ten ADRs were mostly polish, not
moat-widening: five of them (0113, 0115, 0117, 0118, 0119) are the punctuation
program, and all four of the 07-15 adoption recommendations went untouched. Three
concrete facts now bite. v0.17.0 is overdue by the project's own cadence and has
grown into a heavy two-schema-generation breaking release whose curated-release-notes
publish path (ADR-0096) will run live for the very first time on that tag; the shipped
doc surfaces the drift oracle does not cover have drifted behind the CLI, including a
template that renders a nonexistent command for every non-`awf` adopter; and ADR-0120's
decision-format check hard-fails brownfield adopters with pre-existing ADR corpora, with
no knob and no on-ramp, right as it heads into that release. Cutting v0.17.0 is the
highest-leverage immediate action; the highest-leverage moat-widening build is to convert
the content-accuracy advisory into a real check, the one axis the field passed awf on and
the class behind three of this report's defects.

## What is genuinely good

Lead with the real strengths, because the discipline here is not in doubt.

- **Release engineering is mature for a 3.5-week solo project.** Every remote action
  pins a full 40-hex SHA, `cmd/pincheck` (run by `./x gate`) fails any unpinned
  `uses:`, undigested `docker://`, or floated GoReleaser version, and the release
  pipeline's own wiring is gate-tested: `cmd/releasecheck/main_test.go` asserts that
  `release.yml` runs ancestry, gate, check, and notes-extraction before the GoReleaser
  step, so reverting ADR-0096 or unwiring a guard fails the next commit's gate. The
  by-design post-tag-only failure surface is tiny: `ci.yml` runs `goreleaser check`
  plus a full snapshot build on every PR, and the tag workflow re-runs the whole gate
  before any artifact builds.
- **Version authority is consistent today.** `Version` const 0.17.0
  (`internal/project/project.go:27`), lock `awfVersion` 0.17.0 / `schemaVersion` 10,
  `minVersionBySchema[9]=[10]=0.17.0`, migrate registry top `{To: 10}`, newest tag
  v0.16.0 matching `changelog/CHANGELOG.md`. `go run ./cmd/releasecheck` exits 1 with
  exactly the two mid-cycle messages the runbook predicts, so the machinery
  self-reports the not-yet-releasable state correctly.
- **Adoption is fast and honest out of the box.** A live build renders 34 files, passes
  `awf check` immediately, and produces an 81-line generic AGENTS.md with a single
  flagged identity stub, proving the publication-safe-templates invariant in practice.
  Collisions refuse safely with `.awf-bak` backups, `awf uninstall` is a real escape
  hatch, and the changelog names the exact remediation for every breaking change instead
  of burying it.
- **The awf-managed doc surfaces (the ones the drift oracle covers) are accurate.** A
  15-key-plus spot-check of `docs/config-reference.md` found zero inaccuracies; every
  post-ADR-0112 AGENTS.md invariant bullet verifies verbatim against code (scopes,
  banned codepoints, gated-command set, coverage-ignore and deadcode mechanics).
  `docs/pitfalls.md` is a living document, with a same-day 2026-07-16 recurrence note.
- **Field position held and slightly strengthened.** awf is still on-consensus or
  leading on 8 of 10 landscape pillars, and its moat (Pillars 8 and 9) has acquired no
  visible competitor. ADR-0120 is a genuine advance in that lane: successor-side
  `supersedes:` is parsed for the first time, three-way full-supersession symmetry is
  enforced, partial supersession is tokenized against stable Decision-item anchors, and
  ACTIVE.md renders a Supersedence section. Nothing in the field machine-checks its own
  decision corpus.
- **The review-to-action pipeline is fast.** ADR-0112 (guide trim) and ADR-0114 (ledger
  positioning) both landed within a day of the 07-15 review that recommended them. When
  a moat move is pointed at, it ships quickly.

## (1) Release health: overdue, heavy, and one path that has never run

**v0.17.0 is overdue by the project's own cadence and has become a two-schema breaking
release. CONFIRMED (risk).** `changelog/CHANGELOG.md:9` carries a 230-line `[Unreleased]`
section against a prior cadence of releases on 07-08, 07-09, two on 07-10, and two on
07-11. Live re-measure today: `git log v0.16.0..HEAD` is 318 commits (the slice captured
288, so the window is still growing with no release cut). Two schema generations both map
to 0.17.0 (`internal/project/project.go:36-37`), so adopters jump generation 8 to 10 in a
single upgrade, carrying the ADR-0099 pitfalls-sidecar migration, the ADR-0105/0106
invariant-backing rewrite, and the ADR-0120 supersession migration at once. **Fix:** cut
v0.17.0 promptly; the changelog is release-ready and the machinery is green. Treat
"Version const ahead of newest tag" as calendar-time to minimize, and split future cycles
when a second schema generation would otherwise stack onto an unreleased one.

**The mid-cycle window publicly breaks adopter bootstrap. CONFIRMED (risk).** The
committed example installer pins the unreleased version
(`examples/sundial/.awf/bootstrap.sh:10`, `AWF_VERSION 0.17.0`) and its download 404s:
`curl -sI .../v0.17.0/awf_0.17.0_linux_amd64.tar.gz` returns HTTP 404 while v0.16.0
returns 302. ADR-0049 explicitly accepted this window with local-first resolution as
relief, but that acceptance assumed sub-day windows; it has now been broken in public for
5-plus days. The overdue-release fix closes this as a side effect.

**The ADR-0096 curated-release-notes publish path has never run live; v0.17.0 is its
first execution. CONFIRMED (risk).** The notes extraction and the `--release-notes` handoff
(`.github/workflows/release.yml:43`) landed 2026-07-12, after v0.16.0 was tagged 2026-07-11.
No tag has been pushed since, so the rewritten publish path is validated only by
string-matching wiring tests and `goreleaser check`. Critically, `ci.yml`'s snapshot job
runs `release --snapshot --clean` **without** `--release-notes`, so the flag, the
interpolation, and the file handoff are never rehearsed pre-tag. A failure (arg splitting,
empty notes file, GoReleaser flag behavior at v2.17.0) reds the workflow or malforms the
release body only after the tag exists. **Fix:** in the `release-config` job, generate a
notes file from the newest changelog entry and pass `--release-notes` to the snapshot run,
making it a faithful dress rehearsal.

**Post-release verification is still owed and, unlike attestation, is not documented as an
accepted deferral. CONFIRMED (risk).** `release.yml` ends at the GoReleaser publish step;
`.github/workflows/` contains only `ci.yml` and `release.yml`. Nothing downloads a
published archive, verifies it against `checksums.txt`, runs `awf version` against the tag,
or asserts a non-empty release body. `docs/releasing.md:79-80` reduces verification to
"Watch it in the GitHub Actions tab." ADR-0079 Decision 5 records attestation/cosign as a
deliberate deferral; this item, owed since the 2026-07-08 dive, appears in no ADR, runbook
note, or backlog artifact. It is silently absent, against the project's own
document-your-deferrals practice. **Fix:** add a post-publish job that runs
`gh release download`, `sha256sum -c`, executes the linux/amd64 binary asserting
`awf version` equals the tag, and asserts a non-empty body; or write the one-paragraph note
that defers it explicitly. This same job would also de-risk the ADR-0096 first-live-run
above.

**The releasing runbook contradicts the ADR-0096 pipeline it documents elsewhere.
CONFIRMED (defect).** `.awf/docs/parts/releasing/content.md:9` (rendered to
`docs/releasing.md:13`) still says the Release workflow runs GoReleaser to "generate a
Conventional-Commits changelog." That has been false since ADR-0096:
`.goreleaser.yaml` sets `changelog: disable: true` and `release.yml` feeds curated notes
via `--release-notes`. The implementing commit updated the workflow, config, and wiring
tests but never touched this doc, violating docs-travel-with-the-change; the runbook now
contradicts its own step 2. **Fix:** edit the part to say release notes are the curated
changelog section extracted via `awf changelog --version` (ADR-0096), then sync and commit
the rendered doc in the same commit that cuts the release.

**The breaking-change treadmill. CONFIRMED (risk).** `grep -c Breaking changelog/CHANGELOG.md`
returns 13 breaking-change sections across about 20 tagged releases in ~13 days
(2026-06-29 to 2026-07-11; the changelog now carries 21 version headings). The pending
release adds three more breaking items, including a gen-10 migration that rewrites every
adopter's `docs/decisions/`. `awf upgrade` and the pinned bootstrap soften each step, but
with two external adopters (adopter A, adopter B) every breaking release costs an
upgrade-and-sweep session, and no doc sets a cadence expectation pre-1.0. **Fix:** declare
a cadence compact in `docs/releasing.md` and the README (breaking changes batched, at most
one breaking release per two weeks, one migration each). It costs nothing today and is the
cheapest trust signal available.

Lower-severity release-CI items (all **ANALYST-OPINION**): a stale
"release CI runs no tests" comment (`cmd/releasecheck/main.go:44`, false since ADR-0079);
the CI gate job running the full suite twice (`.github/workflows/ci.yml:27`); and no
`timeout-minutes` or `concurrency` guards on either workflow (`.github/workflows/release.yml:12`),
so a hung contents:write tag job holds the 6-hour default.

## (2) Documentation accuracy as a product gap

The drift oracle keeps rendered output equal to config. It does not keep config equal to
reality, and that is where the live cost sits. All three of the following are on surfaces
outside the oracle, exactly where the 07-15 report predicted the failures would land.

**A shipped skill renders a nonexistent command for every adopter whose prefix is not
`awf`. CONFIRMED (defect).** `templates/skills/writing-plans/SKILL.md.tmpl:54` interpolates
the artifact prefix as a binary name: ``run `{{ .prefix }} new plan "<Title>"` ``.
`config-reference.md` defines `prefix` as the skill-name prefix, not a binary. The example
adopter proves the failure today:
`examples/sundial/.claude/skills/sundial-writing-plans/SKILL.md:46` instructs agents to run
`sundial new plan "<Title>"`, and no such executable exists in `examples/sundial` (its
runner is `./x`, whose verb is `./x new`). It renders correctly only in awf's own repo,
where `prefix` coincidentally equals the binary name. The sibling template
`templates/skills/proposing-adr/SKILL.md.tmpl` correctly hardcodes `awf new adr`, which
confirms the intended form. **Fix:** replace `{{ .prefix }}` with the literal `awf` (or a
dedicated binary-name value), re-render, and add a template-source lint banning
`{{ .prefix }}` immediately before a CLI verb.

**CLI help omits the `runner` kind the binary accepts. CONFIRMED (defect).**
`internal/clispec/clispec.go:230` prints "`<kind> is skill, agent, doc, domain, target,
bootstrap, or hooks`" (same omission at the enable Summary `:226` and the disable HelpBody
`:243`, and the list HelpBody at `:125`). But `runner` shipped with ADR-0101 and is
accepted: `cmd/awf/list_add.go:17`'s own usage error names it, and `awf enable runner`
succeeds live. The rendered docs (AGENTS.md, `working-with-awf.md:23`) list `runner`
correctly, so the binary's hand-maintained help contradicts both its own error text and
the drift-checked docs. **Fix:** add `runner` to the four enumerations, and better, derive
or test-pin the kind list from one source shared with `list_add.go`.

**The README command table is three commands and one kind behind the CLI. CONFIRMED
(defect).** `README.md:141-154`'s table has no rows for `awf config` (ADR-0088),
`awf context` (ADR-0092), or `awf prose-gate` (ADR-0119), and line 145 states an
exhaustive kind set that omits `runner` (ADR-0101): "`<kind> in skill, agent, doc, domain,
target, bootstrap, hooks`." This is precisely the class `docs/pitfalls.md:57`
("README.md is outside the drift oracle") predicts, recurring after the pitfall was
recorded. **Fix:** add the rows and the kind now; then promote the pitfall to a
deterministic check. A test diffing `clispec.Commands` names against the README table rows
would be about 20 lines and closes this class permanently.

**This is the clearest place to convert an advisory into a real check.** The
content-accuracy axis (config vs reality) remains advisory-only and inert by default:
`internal/audit/audit.go:388`'s `domain-code-staleness` rule emits `Warning` severity and
is never part of a gate, and the domains list is empty by default so the rule does not even
fire. Both prior reports flagged this axis (deep-analysis Missing #3 / Actionable 9;
landscape Part 2); the delta added a new prose axis instead. **The narrow, deterministic
slice worth building first (CONFIRMED opportunity):** verify that every file path and every
fenced command in the rendered AGENTS.md and doc map resolves against the tree. That
dead-path/dead-command check is as mechanical as the drift oracle, stays in awf's lane, and
would have caught the three defects above deterministically. It reuses the promotion ladder
(advisory audit rule to opt-in gate) that the punctuation program already debugged
end-to-end.

The lower-severity doc findings (all **ANALYST-OPINION**) reinforce the same axis: a
fast/full gate "tier split" that AGENTS.md (`:63`) and two rendered skills assert while
`workflow.md:58-59` and `testing.md:54-57` explicitly deny it exists; `architecture.md:41`
omitting `internal/git` and `internal/plan` from its component inventory; the `./x` usage
line (`x:118`) and `development.md` table omitting the `context` verb; and
`working-with-awf.md:27` omitting `awf new plan`.

## (3) Adoption and positioning

**The adoption STORY has not kept pace with the adoption MACHINERY.** None of the four
07-15 positioning recommendations has been started, even though ADR-0112 made two of them
strictly cheaper.

**The README first screen leads with the commoditized layer. ANALYST-OPINION (smell).**
`README.md:10` opens with "awf renders an opinionated agentic-development workflow into
your repo: a chain of Claude Code skills," and reaches the drift gate only in the final
clause. The landscape report scores multi-adapter rendering as a crowded 2026 category and
drift/provenance plus process-audit as the uncontested whitespace. Commit `bdccc015`
(07-15) fixed the "wraps the agent" over-claim inside the Why section but did not reorder
the lead. **Fix:** a one-commit rewrite of the tagline and first paragraph leading with the
governance moat (drift oracle, backed-invariant ledger, append-only ADR provenance,
`awf audit`), with rendering as the delivery mechanism.

**The what-you-inherit side-by-side never landed. ANALYST-OPINION (smell).** The Quickstart
(`README.md:116-127`) ends at file generation, before the value moment, and nothing tells
the adopter that the `<prefix>-*` skills load automatically and the chain walks them from
brainstorm to retrospective. ADR-0112 made the honest numbers strictly favorable (measured
today: 81-line out-of-box guide, 98-line sundial guide, 103-line awf's own guide), so the
strongest counter to the "it looks heavy" objection is now true and still unused. **Fix:**
add a short "What you actually inherit" block with the three line counts plus a 10-line
"your first chain run" narrative immediately after Quickstart.

**ADR-0120's decision-format check hard-fails brownfield adopters with no on-ramp.
CONFIRMED (risk).** Demonstrated live: in a scratch repo, `awf init` then adding a classic
prose-style Nygard/adr-tools ADR under `docs/decisions/` makes `awf check` exit 1 with
`adr-decision-format ...: Decision section has no column-0 numbered items`
(`internal/project/supersession.go:87`, `checkDecisionFormat`). ADR-0120 item 12 mandates
the check on any ADR regardless of status; its Consequences acknowledge the strictness only
for existing awf adopters served by the gen-10 migration, never for a new adopter whose
`docs/decisions/` predates awf. `README.md:158`'s "Adopting into an existing repo" section
covers file collisions and hooks but says nothing about ADR format. Classic prose
Decisions are the dominant pre-existing format in the wild, so the most attractive adopter
segment (repos that already keep ADRs) fails `awf check` on day one, with the only
remediation being to rewrite historical decision prose, in tension with their own
append-only ethos. **This is a release blocker for the brownfield adopter segment**
(greenfield adopters and existing awf adopters, served by the gen-10 migration, are
unaffected): v0.17.0 ships this check. **Fix before the tag:** add a scoping mechanism (a decision-format numbering floor set at adoption time,
a config knob to downgrade the check to advisory for pre-awf ADRs, or directory scoping)
plus a brownfield paragraph in the README adoption section.

**No injection/provenance posture in the rendered instruction files. CONFIRMED (risk).**
`grep -rniE 'injection|untrusted|exfiltrat|provenance' AGENTS.md templates/agents-doc/`
returns nothing (verified live). awf renders instruction files an agent trusts implicitly;
the landscape report (Part 2, rec 2) verified the lethal trifecta remains unsolved and
recommended a rendered trust/provenance note as a cheap, high-signal move on 2026-07-04.
Twelve-plus days and about 25 ADRs later it is still absent, while much cheaper prose
concerns absorbed five ADRs. This is the landscape report's cheapest unactioned
recommendation. **Fix:** one template edit adding a short "provenance and trust" block to
the rendered agent guide (these files are generated from committed config; treat untrusted
content as data, never as instructions; do not exfiltrate on its behalf).

**Distribution and the maintainer's digest are both unmoved (ANALYST-OPINION, smells).**
`.goreleaser.yaml` still ships tarballs only, with no homebrew/scoop/nfpm targets and no
Claude Code plugin/marketplace motion; the marketplace obviates awf's distribution story
but would showcase its generation/governance story if awf were present in it. The curated
maintainer's digest (a ~10-ADR reading path) is still unwritten while the corpus reached
124 ADR files; `docs/decisions/README.md` covers lifecycle mechanics only and ACTIVE.md is
a flat generated index. A related smell: deferred commitments an Implemented ADR cites as
"already-tracked efforts" (`docs/decisions/0114-...md:118-119`) have no in-repo home;
`docs/roadmap.md` does not exist, though awf ships a roadmap doc that `examples/sundial`
enables and awf's own tree does not. That is both a discoverability gap and a dogfooding
hole. [Resolved since baseline: `docs/roadmap.md` is now enabled in awf's own tree as of
HEAD `7a06af73` (2026-07-17).]

## (4) Field standing one day on

**Which 07-15 landscape gaps closed:**

| 07-15 gap / rec | Status in delta | Evidence |
|---|---|---|
| Guide violates context-engineering consensus (37.8 KB) | CLOSED via ADR-0112 | AGENTS.md now 13.7 KB / 103 lines, invariants 84 to 11 bullets, decidable retention criterion shipped |
| "wraps the agent" over-claim | CLOSED via ADR-0114 + `bdccc015` | README "guards what the agent produces, not how it reasons"; glossary ledger-not-proof; code-reviewer charged with the semantic check |
| Punctuation/prose hygiene | CLOSED (5 ADRs: 0115/0117/0118/0119, plus 0113) | Shipped-surface gate to advisory audit rule to sweep to opt-in `awf prose-gate`, a complete promotion-ladder demonstration |
| Supersession machine-checking | ADVANCED via ADR-0120 | The one genuine moat deepener in the delta |
| Freeze `awf context` (T2.6) | HELD de facto | Zero context ADRs in 0112..0121 |

**Which remain open:**

| Open gap | Status | Evidence |
|---|---|---|
| Content-accuracy drift axis (config vs reality) | NO MOTION | `internal/audit/audit.go:388` still `Warning`, inert without domains |
| Deterministic golden-fixture harness (landscape rec 1) | NO MOTION | `golden_test.go` asserts rendered bytes, not guidance outcomes |
| Injection/provenance note (landscape rec 2) | NO MOTION | Absent from every guide surface (grep, above) |
| Scope-ceiling "what awf is not" ADR (T2.7) | NOT WRITTEN | No such ADR in the corpus |
| README repositioning + plugin distribution (T3.10) | NO MOTION | README still leads with rendering; tarballs only |
| Maintainer's digest (T3.11) | NOT WRITTEN | Corpus at 124 ADR files |
| Mutation automation | SETTLED-MANUAL, no scheduled advisory | `.github/workflows/` still only ci + release |

**Does the recent output point at the moat or at polish? Mostly polish. CONFIRMED as fact;
the causal interpretation is contested.** The countable facts are solid: 5 of the 10 delta
ADRs (0113, 0115, 0117, 0118, 0119) are the punctuation program, and the moat-serving ADRs
(0112, 0114, 0120) were reactions to the previous day's external review or an explicit user
directive. The stronger interpretive claim, that the self-generated agenda structurally
"drifts toward prose hygiene," was marked not-confirmed in verification and should be held
as a watch-item rather than a proven trend on one day of evidence. The honest reading: the
review-to-action pipeline is excellent, and left to external prompting the project points at
the moat; the open question is what the internally-generated agenda does across the next
window, and the punctuation plurality is a caution flag, not a verdict. Note also that the
canonical landscape report's own self-state is now 13 minor versions stale
(`agentic-workflow-landscape-and-awf-standing-2026-07.md:364` still says v0.6.2, 52 ADRs,
one adopter) and should carry a dated currency marker (ANALYST-OPINION, nit).

## Delta since 2026-07-15

- **Acted on:** T2.4 (tier AGENTS.md invariants) via ADR-0112; T1.1a + T2.5 (stop
  over-claiming, ledger positioning, charge the reviewer) via ADR-0114; landscape rec 4
  follow-through (staged-slice pre-commit now drift-checks `examples/sundial`, commit
  `ff7d9e2b`). A new moat capability not in either prior report landed: ADR-0120 structured
  supersession.
- **Not acted on:** content-accuracy axis (T3.9), golden-fixture harness (landscape rec 1),
  injection/provenance note (landscape rec 2), scope-ceiling ADR (T2.7), README
  repositioning and plugin distribution (T3.10), maintainer's digest (T3.11), mutation
  automation. The cost is now measurable rather than hypothetical: the README table lags
  the CLI by three commands plus a kind, and the writing-plans prefix-as-binary defect
  shipped to the example adopter uncaught, both instances of the one axis the field passed
  awf on.
- **New friction the 07-15 report could not have seen:** ADR-0120's decision-format check
  hard-fails brownfield corpora (demonstrated live), the gen-10 migration rewrites adopters'
  `docs/decisions/`, and ADR-0115's ACTIVE.md format change forces a one-time drift on every
  adopter until they sync. All honestly documented in the changelog's breaking block.
- **Scale:** commits 1389 to 1508, ADR files 111 to 124 (118 Implemented per ACTIVE.md),
  the unreleased window grew and the public bootstrap 404 crossed 5 days. Full gate
  (`./x gate`) passes clean on this tree; 100% coverage holds; production coverage-ignore
  markers rose 194 to about 202.

## Closing: the next 10 moves for this topic

Ranked by leverage. A mix of build-this and stop-doing / say-no.

1. **Cut v0.17.0 now (build).** The changelog is release-ready and the machinery is green.
   Every extra day grows the adopter jump and extends the public bootstrap 404.
2. **Ship a brownfield ADR-0120 on-ramp before the tag (build; release blocker for the
   brownfield adopter segment).** A
   decision-format numbering floor, an advisory-mode knob for pre-awf ADRs, or directory
   scoping, plus a README brownfield paragraph. Do not ship a day-one `awf check` failure
   to the best-fit adopter segment (`internal/project/supersession.go:87`).
3. **Fix the three shipped doc-accuracy defects and add the ~20-line guard (build).**
   Literal `awf` in the writing-plans template (`:54`), `runner` in the four clispec
   enumerations, the three missing README rows plus the kind, all in one commit, then a
   test diffing `clispec.Commands` against the README table and `./x` usage.
4. **Rehearse the ADR-0096 `--release-notes` handoff pre-tag and add the post-publish
   verification job (build).** Turns v0.17.0's publish path from a production first-run into
   a rehearsed one, and closes the silently-owed post-release check
   (`.github/workflows/release.yml:43,49`).
5. **Fix the releasing runbook Conventional-Commits contradiction in the release commit
   (build).** `.awf/docs/parts/releasing/content.md:9`.
6. **Build content-accuracy drift axis v1 (build).** Deterministic dead-path/dead-command
   verification over the rendered guide and doc map. This is the single highest-leverage
   moat move: the one axis the field passed awf on, mechanical, in-lane, and it would have
   caught defects 1 and 3 above (`internal/audit/audit.go:388`).
7. **Add the injection/provenance block to the rendered guide (build).** One template edit,
   the landscape report's cheapest unactioned recommendation, twelve-plus days idle.
8. **Declare a cadence compact and a moat-budget rule, out loud (stop-doing / say-no).** At
   most one breaking release per two weeks, one migration each; and require one of the two
   ranked field gaps (content-accuracy axis, fixture harness) to have an ADR in flight
   before accepting another prose/process ADR. Keep `awf context` and the prose-gate
   feature-frozen as already dispositioned. No sixth punctuation ADR.
9. **Reposition the README first screen (build).** Lead with the governance moat plus the
   honest what-you-inherit numbers (81 / 98 / 103). (The companion move, `awf enable doc
   roadmap` to give deferred commitments an in-repo home and close a dogfooding hole
   (`docs/decisions/0114-...md:118`), has since landed at HEAD `7a06af73`.)
10. **Write the maintainer's digest (build).** A curated ~10-ADR reading path linked from
    the README; simultaneously bus-factor insurance, evaluator onboarding, and the
    outward-facing drift/provenance/supersession writeup that stakes the Pillar-8 whitespace
    publicly, now materially stronger with ADR-0120.

Say-no is doing real work in this list: items 8's freezes and the no-sixth-punctuation-ADR
line are how the freed author attention (the scarcest adoption resource in a solo project)
gets spent on 1 through 7 and 9 through 10.

## Proposed content for v0.17.0

**Already in `[Unreleased]` and release-ready (ship as-is):** ADR-0096 curated release
notes, ADR-0099 pitfalls sidecar + gen-9 migration, ADR-0104 context JSON shape,
ADR-0105/0106 invariant-backing redesign, ADR-0112 core-only guide, ADR-0114 ledger
positioning, ADR-0115/0117/0118/0119 punctuation program, ADR-0120 supersession + gen-10
migration, ADR-0121 authoring comments. The version chain, releasecheck, and pincheck are
all consistent; this slice is genuinely done.

**Land before the tag (release blockers, small):**
- The ADR-0120 brownfield on-ramp (move 2), a blocker for the brownfield adopter segment
  specifically (greenfield and existing awf adopters are unaffected). Do not release a
  day-one brownfield `awf check` failure without an escape hatch and a documented recipe.
- The three doc-accuracy defect fixes plus the releasing-runbook correction (moves 3 and 5).
  These are content that ships in v0.17.0's own artifacts; fix them in the release.
- The pre-tag `--release-notes` rehearsal (move 4, the snapshot-job half), so the tag is
  not the first integration test of the publish path.

**Explicitly not in v0.17.0 (say-no):** no new punctuation or prose ADR, no `awf context`
extension. If the injection/provenance block (move 7) and content-accuracy axis v1 (move 6)
are ready in time they are welcome, but they must not delay the cut; the overdue window and
the public bootstrap 404 are the higher cost today.

---

*Cross-links: `00-overview.md` (this set's synthesis and cross-dimension opportunities);
sibling parts on code/architecture, guardrails, and rendered output;
`deep-analysis-2026-07-15.md` and `agentic-workflow-landscape-and-awf-standing-2026-07.md`
(the two documents this report deltas against).*
