# awf Deep Analysis 2026-07-16 - Overview and Synthesis

*Part 00 of the 2026-07-16 analysis set: the synthesis and reading guide for a
four-report, whole-project review. This overview is the entry point; the detail,
the file:line evidence, and the fixes live in the four topic reports linked at the
bottom. Companion and delta to
[deep-analysis-2026-07-15.md](deep-analysis-2026-07-15.md) (the prior five-lens
audit) and [agentic-workflow-landscape-and-awf-standing-2026-07.md](agentic-workflow-landscape-and-awf-standing-2026-07.md)
(the field-landscape report).*

---

## What this is and how it was produced

This set was produced from a multi-agent analysis of the awf repository at commit
`ff7d9e2b` (the clean 2026-07-16 tree). Twelve independent dimension analysts
(architecture, code idioms, test quality, guardrails, CLI/UX, rendering/templates,
docs accuracy, ADR corpus, security/robustness, release/CI, adoption/DX, field
standing) each mapped their area, and a separate defect hunt across five code-seam
lenses looked for demonstrable bugs. Every claimed defect and risk was then put
through a three-lens adversarial verification pass (reproduce, design-intent,
severity/evidence audit) and kept only on a majority-confirm; each hunt bug carries
a reproduction command. The four topic reports were each drafted, critiqued for
accuracy and completeness against the source findings, and revised.

Two honesty caveats on the method. First, the defect hunt's loop-until-dry tail
(rounds 2 and 3) did not complete: it was truncated by usage limits. The confirmed
bugs below are a well-sampled but **not exhaustive** mining of the defect surface;
absence of a finding in a seam is not proof of its cleanliness. Second, the working
tree advanced while the analysis ran: HEAD is now `7a06af73` (2026-07-17), which
landed ADR-0122/0123/0124 (a multi-runtime output-plans surface). The reports were
re-verified against HEAD where a finding was load-bearing, and note where a line
number or count shifted; a few findings were already partly addressed at HEAD and
are marked as such.

The whole exercise is itself awf's own thesis turned on the project: probabilistic
agents producing analysis, wrapped in a deterministic verification pass. Findings
that survived that pass are labeled **CONFIRMED**; single-analyst judgments that did
not go through independent verification are labeled **ANALYST-OPINION**. Treat the
two tiers differently.

---

## State of the project (2026-07-16)

| Dimension | Reading |
|-----------|---------|
| Age / velocity | ~3.5 weeks old; 1508 commits; ~5 ADRs/day sustained. This remains an AI-agent-driven project whose guardrails are load-bearing for its own method. |
| Code | ~15.7k production Go LOC across 32 packages; acyclic, shallow import DAG; a real composition root. gofmt/goimports clean, zero TODO/FIXME, zero production panics, zero production interface types, a strong linter wall. |
| Tests | ~27.4k test LOC, testify-free; 100% statement coverage gate green today; behavioral assertions (byte-equality renders, corruption matrices, anti-vacuity guards) that hold up under the velocity. |
| Gate | `./x gate` passes clean on this tree: 100% coverage, deadcode clean, all workflow refs pinned, prose-gate clean. `awf audit` clean. |
| ADR corpus | 124 ADR files (117 Implemented, 3 Superseded, 1 Accepted); mechanically the healthiest it has been, with ADR-0120 making supersession machine-checked. |
| Release | v0.16.0 shipped 2026-07-11; Version const 0.17.0; schema 10. ADR-0096..0121 sit unreleased on main (roughly 120 commits, two schema generations). v0.17.0 is overdue by the project's own cadence. |
| Field | Still on-consensus or leading on 8 of 10 field pillars; the moat (drift/provenance productization, process-conformance audit) has acquired no visible competitor in the intervening window. |

The one-sentence state: **the engineering is unusually disciplined and the gate is
genuinely green, but the project is now carrying an overdue, unusually heavy release,
a handful of real defects in freshly shipped surfaces, and a process that raised its
craft after the 07-15 critique without raising its admission bar.**

---

## The three cross-cutting themes

Everything in the four reports rolls up to three themes. Each is a delta on 07-15,
not a repeat of it.

### Theme 1: the new surfaces shipped real defects, and they cluster

The 07-15 report assessed a codebase whose mechanical layer was strong. The
2026-07-16 delta is the first look at what the last two weeks *added*, and the
addition brought the bugs. The single richest vein is the freshly shipped
`awf prose-gate` command (ADR-0119): five distinct CONFIRMED holes in one command,
including two that block **every commit** for any adopter with a git submodule or an
unstaged deletion, and one that lets a banned codepoint through on partial staging.
The two new schema migrations (`internal/migrate`, gen-9 pitfalls and gen-10
retirement-tokens) can each **silently corrupt an adopter's tree during
`awf upgrade` behind a green gate**. This is the important inversion of the 07-15
picture: the danger has moved from "what the mechanical layer cannot verify" to
"defects in the code the project shipped this fortnight," and they concentrate in
the newest, least-exercised surfaces. Full ledger in report 02; the guardrail-level
view of why the gate does not catch them is in report 03.

### Theme 2: the guardrails verify a lot, honestly, but the seam between "slice" and "worktree" is half-open

The hermetic staged-slice pre-commit is still the single best guardrail in the
system, and 07-16 extended it to drift-check the example adopter. But the CONFIRMED
finding is that the slice only *builds* and *drift-checks* the staged bytes; every
*behavioral* gate (test, coverage, vet, lint, deadcode, prose-gate, pincheck) still
runs against the worktree. So the partial-staging class the slice exists to close is
only half-closed, and three of the prose-gate/pincheck holes above are reachable
through it at commit time. Alongside this, the verification-honesty story from 07-15
is unchanged: coverage remains a liveness metric (the largest and fastest-growing
coverage-ignore bucket encodes unverified cross-function call-order claims), and
mutation testing (the one tool that would catch "100% coverage, trivial assertion")
is still manual and absent from the gate. The invariant ledger is now honestly
repositioned as a ledger-not-proof by ADR-0114, and awf's own backings are mostly
genuine. Detail in report 03.

### Theme 3: craft rose after 07-15; the admission bar did not, and the release is the bill coming due

The recommendations from 07-15 that got executed are precisely the ones that *write
or repair an artifact*: ADR-0112 trimmed the guide invariants, ADR-0114 fixed the
"proof" positioning, the punctuation program landed. The recommendations that would
*slow production* are precisely the ones still open: the scope-ceiling "what awf is
not" ADR was not written, the ADR warrant bar still reads "when in doubt, write the
ADR," the coverage-ignore audit did not happen (the count grew), and mutation
automation did not land. Meanwhile the append-only guarantee took three separate
carve-outs in one week, and the punctuation policy alone consumed five ADRs in 48
hours with a decision half-life measured in hours. The accumulated cost of that
additive bias is now concrete and external: v0.17.0 is overdue, spans two schema
generations, rewrites every adopter's `docs/decisions/`, hard-fails brownfield
adopters with no on-ramp, and ships three doc-accuracy defects in its own artifacts,
while the mid-cycle window publicly breaks the example adopter's bootstrap. The moat
work the field actually rewards (the content-accuracy drift axis, a fixture harness)
sat untouched while the polish shipped. Detail in reports 03 and 04.

---

## Consolidated priorities (top of every report, deduplicated)

Ranked by leverage across all four reports. The first block is "before the next
release"; the second is "the structural moves that keep the project adoptable."

**Release blockers and shipped-defect fixes (do before the v0.17.0 tag):**

1. **Fix the prose-gate submodule/gitlink hard-fail and the index/worktree content
   mismatch** (report 02, C1; report 03, F1-F2). The first blocks every commit for a
   submodule adopter; the second defeats the gate's purpose. `internal/git/git.go:136`,
   `cmd/awf/prosegate.go:32`.
2. **Fix the two migration corruptions** (report 02, C2). `pitfalls.go:111` emits
   lossy/invalid YAML and deletes the source first; `retirementtokens.go:131` splices
   inside a fenced code block and mints a duplicate item number. Both hit `awf upgrade`
   to schema 9/10, which is exactly the path every current adopter runs next.
3. **Fix the three shipped doc-accuracy defects plus a guard** (report 04, move 3):
   the literal-command render in the writing-plans template
   (`templates/skills/writing-plans/SKILL.md.tmpl:54`), the missing `runner` kind in
   the four clispec enumerations, and the stale README command table. Add a test
   diffing `clispec.Commands` against the README and `./x` usage.
4. **Ship an ADR-0120 brownfield on-ramp** (report 04, move 2): the decision-format
   check currently hard-fails any repo with a pre-existing ADR corpus, with no knob and
   no documented recipe (`internal/project/supersession.go:87`).
5. **Rehearse the ADR-0096 curated-release-notes path pre-tag and add the
   post-publish verification job** (report 04, move 4). The publish path has never run
   live; the v0.17.0 tag should not be its first integration test, and the owed
   post-release check is still silently absent.

**Structural moves (the ones 07-15 asked for and did not get):**

6. **Build content-accuracy drift axis v1** (report 04, move 6): deterministic
   dead-path / dead-command verification over the rendered guide and doc map. Highest
   moat move; the one axis the field passed awf on; it would have caught priorities 1
   and 3 above. Promote `domain-code-staleness` from advisory toward a real check
   (`internal/audit/audit.go:388`).
7. **Do the `renderedSnapshot` refactor** (report 01): one shared parsed/rendered
   value threaded through `Check`, `AdvisoryNotes`, and `SyncReport` removes the
   double-render and dozen-fold ADR-reparse, and is the enabling move for the deferred
   `check.go` split (965 -> ~550 LOC) and `SyncReport` decomposition, both of which
   grew rather than shrank since 07-15.
8. **Close the staged-slice behavioral gap for the two cheapest gates** (report 03,
   priority 1): run prose-gate and pincheck inside the hermetic slice, or document
   explicitly that only build and drift are per-commit-hermetic.
9. **Harden pincheck against non-line YAML** (report 03, F3): its bypass is the one
   guardrail hole that injects unreviewed third-party CI code.
10. **Raise the ADR warrant bar, write the scope-ceiling ADR, and declare a cadence
    compact** (reports 03 and 04): the unexecuted half of the 07-15 critique, and the
    reason the corpus grows faster than it prunes. Add a coverage-ignore revalidation
    pass and single-source the banned-rune set (report 01) to retire the one delta
    regression.

Say-no is doing real work here: no sixth punctuation ADR, freeze `awf context` and
prose-gate as feature-complete, and require a moat-ADR in flight before accepting
another prose/process ADR. In a solo-plus-agents project, author attention is the
scarce adoption resource, and it is currently spent on polish.

---

## The four reports

- **[01 - Code Quality and Architecture](deep-analysis-2026-07-16-01-code-quality-and-architecture.md).**
  The clean DAG and the spec-package anti-drift pattern (genuinely good), then the
  `internal/project` god-package trajectory, the `renderedSnapshot` enabling refactor
  with three concrete `check.go` split seams, and the idiom debts (banned-rune
  triplication, stale ADR-0034 comment family, string-based error model). Mostly
  ANALYST-OPINION smells: maintainability judgments, not proven defects.
- **[02 - Issues and Defects](deep-analysis-2026-07-16-02-issues-and-defects.md).**
  The meatiest report: roughly two dozen findings in six severity-ordered clusters
  (prose-gate, migrations, rendering/ingestion, invariant-ledger, CLI/lock, security),
  each with file:line, mechanism, blast radius, a one-line fix, and a repro command
  for the hunt bugs. Ends with a 19-row prioritized fix table.
- **[03 - Guardrails and Verification](deep-analysis-2026-07-16-03-guardrails-and-verification.md).**
  What the gate verifies for real (strong), the half-open staged-slice seam with four
  concrete bypass scenarios and whether the full gate catches them (it catches none),
  verification honesty (coverage-ignore buckets, ledger-not-proof, manual mutation),
  and ADR-process health (three append-only carve-outs in a week, unraised warrant
  bar, hours-long decision half-life).
- **[04 - Roadmap, Adoption, and Field Standing](deep-analysis-2026-07-16-04-roadmap-and-field-standing.md).**
  Release health (overdue, heavy, one path that has never run), documentation
  accuracy as the content-accuracy product gap, adoption/positioning, field standing
  one day on, a ranked next-10-moves list, and proposed v0.17.0 content.

---

## Bottom line

Nothing in this delta overturns the 07-15 verdict: the core is proportionate, the
engineering is unusually disciplined, the gate is genuinely green, and the moat is
real and uncontested. What the 24-hour-later, defect-hunting view adds is sharper and
more actionable than the prior audit could be, because it looked at what the last two
weeks shipped. Three things are true at once. The mechanical foundation is strong.
The newest surfaces (prose-gate, the migrations, the shipped skill/README docs) carry
real, reproducible defects that should be fixed before they ride out in v0.17.0. And
the process that produces all of this raised its craft but not its restraint, so the
project keeps adding faster than it prunes, and the overdue release is where that bill
is now visible. The highest-leverage response is not more machinery: it is to cut the
release cleanly (defects fixed, brownfield on-ramp in place), then spend the next
cycle on the one moat move the field actually rewards (the content-accuracy drift
axis) and on the restraint decisions 07-15 already asked for. Fix the shipped defects,
raise the bar, widen the moat: in that order.
