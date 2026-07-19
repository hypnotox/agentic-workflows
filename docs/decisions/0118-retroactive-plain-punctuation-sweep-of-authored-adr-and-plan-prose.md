---
status: Implemented
date: 2026-07-15
tags: [doc-standard, adr-lifecycle, plan-taxonomy, active-md, verification-discipline]
related: [28, 113, 115, 116, 117, 119]
domains: [adr-system, rendering, tooling]
---
# ADR-0118: Retroactive plain-punctuation sweep of authored ADR and plan prose

## Context

ADR-0115 banned seven typographic punctuation substitutes from the prose awf ships and, in its
Decision item 5, normalized three ADR titles retroactively. It bounded that carve-out explicitly:
the heading line only, never the body. ADR-0117 then made new authored prose advisory-checked by a
net-increase `awf audit` rule, which leaves every pre-existing occurrence grandfathered by
construction.

That boundary was the chain's own invention, not the maintainer's instruction. The approval on
record read: "It's not the first breach of append-only, we did quite some changes which were
approved by me. It just needs my approval, and we want a clean history to steer context of future
agents, so this is a necessary change. Prose does not change IMO, we just normalise retroactively."
The chain read that as authorising titles and derived a principle to fit ("titles are emitted into
`ACTIVE.md` and the domain docs; bodies are not"). The maintainer has since confirmed the approval
was always the broader one: the sweep covers authored ADR and plan bodies, and only prose that is
legitimately *about* a banned glyph is left alone.

Item 5's own rationale supports the wider reading. It justified normalizing titles because they are
"harvested into `ACTIVE.md` and four domain docs that agents read as context, and a record that
half-follows its own convention teaches the wrong one." Agents read ADR and plan *bodies* as
context too: that is what `awf context` exists to surface (ADR-0092), and what ADR-0104's relevance
tiering feeds them. The premise reaches further than the conclusion the chain drew from it.

Two measurements shape the decision, both verifiable at the time of writing:

- **The corpus is large but bounded.** 113 of the 118 numbered ADRs carry a banned codepoint,
  2341 em-dashes, 26 en-dashes, 130 ellipses and 0 curly quotes between them; 78 of the 79 dated
  plans carry 4347, 154, 228 and 0. About 7200 sites across 191 files. These counts describe the
  authored corpus only; the whole-directory figures are 1 en-dash and 3 ellipses higher because
  `docs/decisions/` also holds generated files, which item 1 excludes.
- **The two halves of the corpus behave differently, and the difference is load-bearing.** No
  occurrence in any ADR body sits inside a fenced code block (0 of 2341). The plans are the
  opposite: 928 occurrences sit inside fences, across 77 of the 78 (432 unlabeled, 312 `go`, 94
  `markdown`, 31 `yaml`, 29 `diff`, 25 `text`, 5 `bash`; the fences are opened with backticks and
  with tildes, and a count that reads only one delimiter misses 25), because the plan convention
  requires a task to give exact content. A rule derived from the ADR half alone would be wrong about
  the larger half.
- **The convention is already half-followed, exactly as item 5 warned.** ADR-0028's title carries an
  en-dash: "ADR-first ordering and a visible plan-ADR resync loop in the workflow chain". Item 5
  swept the three *em-dashed* titles, but ADR-0115 bans seven codepoints, so the en-dashed title
  survived and `docs/decisions/ACTIVE.md` and `docs/domains/adr-system.md` emit a banned codepoint
  today. Neither gate catches it: ADR-0115's gate scans `templates.FS`, `changelog.FS`, and
  production Go string literals, and generated indexes are none of those.

Nothing forces this sweep. The Tier-1 gate does not read `docs/`, and the Tier-2 audit rule triggers
only on a net increase, so removals are silent and the corpus is green either way. It is elective
corpus hygiene, taken because the corpus is the context future agents are steered by.

The standing objection is ADR-0028's own Alternatives table, which refused to rewrite ADR-0022's
prose on append-only grounds. That refusal is the reason this ADR exists rather than a bare sweep:
without a record, the next agent reads ADR-0115 item 5 and ADR-0028 and either refuses an approved
act or re-litigates it.

## Decision

1. **The retroactive normalization carve-out widens from ADR headings to the full authored body of
   every ADR and plan.** This is partial-item supersedence of **ADR-0115 Decision item 5**
   (`refines: ADR-0115#5`), whose "the heading line only, never the body" clause is overridden;
   ADR-0115 stays Implemented and every other item stands, and it carries `related: [118]` as the
   back-pointer (ADR-0116). The
   scope is the **authored** files only: `docs/decisions/[0-9]*.md` and the dated plans under
   `docs/plans/`. Generated files are excluded by construction, not by exemption: `ACTIVE.md`,
   `docs/decisions/README.md`, and `docs/plans/template.md` are rendered, and hand-editing a
   rendered file is forbidden outright, so a sweep could not reach them even in principle. Their
   residue is item 4's and the follow-up's business, not this item's. `docs/research/` is
   deliberately out of scope: it is not part of the decision corpus agents are steered by, and no
   approval covers it.

2. **ADR-0113 is exempt by maintainer designation, and the exemption rests on that alone.**
   `docs/decisions/0113-em-dash-free-shipped-templates.md`, the ADR about the em-dash, keeps its
   seven occurrences because the maintainer designated it, on the judgement that the record of a
   decision about a glyph should be left as it stood. No meaning-preservation claim is made, because
   none is available: all seven of its occurrences are ordinary prose punctuation, and normalizing
   them would destroy nothing. It could be swept safely; it is not, by choice.

3. **Prose that is genuinely about a banned glyph keeps that glyph.** Where an occurrence is the
   subject of its own sentence, rather than punctuation in it, normalizing it would make the
   sentence assert something false. This is the maintainer's rule, and it is a judgement, not a
   mechanical test, so item 7's completeness probe names its instances rather than inferring them.

   The class is small, because **ADR-0115 Decision item 7** already holds that depiction is handled
   by convention and scope rather than an exemption list, extending ADR-0113 item 4's rule that a
   doc discussing a banned character names it by word and codepoint instead of typing it. The
   corpus largely obeys that convention: ADR-0113 names its subject as "U+2014" and "Em-dash
   characters (U+2014)" without ever typing one, and ADR-0115, ADR-0117, and the plan that
   implemented them carry zero occurrences between them.

   It is not empty, though, and the sweep found the counterexample rather than assuming it away.
   `docs/plans/2026-07-13-invariant-backing-migration-to-enforced-test-scoped-backing.md` records
   that the generated config reference's live-state column "shows `<em-dash>` for
   `invariants.testGlobs`". The glyph there is the observed value, verified against the tree at the
   time of writing; punctuating it would turn a true report into a false one. It keeps its glyph.
   The second known instance, the `.awf/docs/pitfalls.yaml` entry that types a curly quote to
   document gofmt's rewrite, is sidecar data: out of this ADR's scope, governed by the follow-up in
   item 10, and **not** permanently exempt from cleaning, only from this sweep.

4. **The sweep covers all seven banned codepoints, not just the em-dash, and it includes headings.**
   ADR-0028's en-dashed title is normalized here, closing the gap ADR-0115 item 5 left when it swept
   for one codepoint under a seven-codepoint ban. Every emitted ADR title is then free of all seven.

5. **The five bounding conditions of ADR-0115 item 5 carry over unchanged, with one widened.**
   Retroactive normalization is permitted only when: it changes punctuation and nothing else; it is
   limited to the seven banned codepoints; **every word is preserved**; a maintainer approves it
   explicitly; and it is never a licence to edit an ADR's argument. Only the first condition's reach
   changes, from the heading line to the body. Append-only protects rationale, not orthography, and
   an argument's orthography is not its argument.

6. **"Every word is preserved" is proven mechanically, not asserted.** A word-stream comparison
   (strip every non-alphanumeric character, then diff the resulting token streams) must report zero
   delta across the swept corpus. This is the check that makes the sweep defensible: it reduces the
   risk from "content was silently lost across 191 frozen records" to "a punctuation choice reads
   awkwardly", which is recoverable prose, not lost rationale. The parent effort proved the
   technique on the changelog, where it caught the word count moving 8590 to 8593 and confirmed the
   delta was exactly the three words of one intended rephrasing.

7. **Completeness is proven by a scoped probe, run at the granularity the rule governs.** The sweep
   is done when a scan of the authored files named in item 1 for the seven codepoints returns
   nothing outside the instances items 2 and 3 name. The probe reads the authored corpus and names
   each exemption, rather than reading a wider unit and tolerating a residue, so a nonzero result is
   a defect and not a judgement call. This is deliberate: ADR-0115 item 5's own post-check read
   whole files for a heading-scoped rule and was therefore unsatisfiable, and that failure is
   recorded in the pitfalls doc.

8. **Fenced code blocks are swept, and the sweep reaches inside them.** The roughly 895 in-fence
   occurrences in the plans are not spared. A `go` fence quoting an em-dashed string literal is
   quoting code that ADR-0115 has since made illegal and the parent effort has already cleaned, so
   normalizing the fence makes the frozen plan agree with the code that actually shipped rather than
   preserving a specimen of a form the project now bans. The word-stream proof of item 6 applies
   inside fences exactly as it does outside, so no content can be lost there either.

9. **Replacement is by sentence structure, never a blind substitution.** An em-dash becomes the
   punctuation the sentence wants: a colon where what follows explains what precedes, a semicolon
   between independent clauses, a comma for a light aside, and parentheses for a parenthetical. A
   pair of em-dashes bracketing a clause becomes a pair of parentheses. An en-dash in a numeric or
   token range becomes an ASCII hyphen. An ellipsis becomes three periods. A bare hyphen is never
   substituted for an em-dash. A blind rule would corrupt the 694 occurrences that sit on lines
   carrying two or more, and the corpus is hard-wrapped, so a bracketing pair can span lines. Inside
   a fence the same rules apply to the prose the fence contains; where a fence holds code, the
   replacement is whatever the surrounding language makes correct, which for a Go string literal is
   the same colon-or-parentheses choice the parent effort applied to the real source.

10. **This authorises one act; it creates no new ongoing rule, and it closes no gate.** The
   forward-looking regime is unchanged and already owned: ADR-0115 gates the emitted surfaces,
   ADR-0117 warns on newly authored prose. This ADR declares no invariant and adds no check, because
   a completed sweep is history rather than a property to maintain.

   The reach of that regime is narrower than "everything", and this item states the gap rather than
   implying coverage that does not exist. ADR-0117's rule reads `.md` files under the docs directory
   and skips generated paths, so three live sources are checked by nothing: `.awf/docs/pitfalls.yaml`,
   `.awf/docs/glossary.yaml`, and `.awf/parts/adr-readme/invariants.md` are neither `.md` nor under
   the docs directory, while their render targets are generated and therefore excluded at the other
   end. A new pitfall entry containing an em-dash reaches emitted prose with no check at either end.

   That gap is real and it is **deferred, not accepted**. It is out of scope here because it asks a
   different question, what counts as a checked surface, where this ADR asks how far append-only
   bends; answering both here would pull a fourth domain and a gate change into a record about
   settled prose. An immediate follow-up ADR owns it, and owns the rest of the repository with it:
   the maintainer's standing requirement is that the whole repository be clean and that nothing
   reintroduce a banned codepoint afterwards, which needs a gate this ADR does not write. The
   surfaces that follow-up must reach are the three sidecar sources above, production Go comments
   (13 occurrences, all ellipses), the test files, and the tracked non-Go files. Sidecar data is
   explicitly **not** exempt: its only lasting exemption is the depiction in item 3.

## Invariants

None. This ADR authorises a single retroactive act and deliberately declares no invariant.

The property a reader might expect here, "the authored corpus contains no banned codepoint", is
deliberately **not** declared, for two reasons. It would need a permanent exemption mechanism to
express item 2's judgement, which ADR-0113 Decision item 3 and ADR-0115 both rejected as a design;
and it would convert an elective hygiene act into a standing gate over 191 hand-authored records,
which is precisely the burden ADR-0117 chose advisory severity to avoid. The ongoing property is
already declared by ADR-0115 (`emitted-prose-no-typographic-substitutes`) and ADR-0117
(`audit-plain-punctuation`); this ADR adds no third.

## Consequences

- **The decision corpus stops teaching what it forbids.** An agent reading any ADR or plan for
  context sees prose that follows the standard the same corpus states, so the corpus and the rule
  agree. That is the whole return on the effort.
- **ADR-0028's en-dashed title is fixed, and two generated files re-render clean**
  (`docs/decisions/ACTIVE.md` and `docs/domains/adr-system.md`). It is **not** the last banned
  codepoint in generated output: 24 remain, in `docs/pitfalls.md` (20), `docs/decisions/README.md`
  (3), and `docs/glossary.md` (1), rendered from the three `.awf/` sources item 10 names. Those are
  the same defect class as the ADR-0028 title, a banned codepoint reaching emitted output through
  data no gate reads, and they are the follow-up's business.
- **191 files are touched in a single sweep, and the diff is large and mechanical.** It is
  reviewable only by the word-stream proof plus spot reading, not by reading 7200 hunks. That is
  accepted deliberately: the proof is stronger evidence than a human diff read would be, because it
  is exhaustive where reading is a sample.
- **Punctuation choice is a judgement applied at scale, so some sites will read as merely
  acceptable rather than best.** Accepted: the alternative is leaving the corpus half-converted, and
  a slightly flat colon in a settled record costs less than a record that contradicts its own
  standard.
- **The append-only carve-out is now materially wider, and that is a real cost.** The mitigation is
  that it stays bounded by five conditions, one of which (word preservation) is mechanically proven
  rather than promised, and by the requirement of explicit maintainer approval per act. Append-only
  protects rationale; this touches only orthography.
- **Git blame over ADR and plan prose is disturbed for one commit.** Accepted, as it was for
  `a495521` and `8a2602d`; the rationale is unchanged and recoverable from history.
- **The `plain-punctuation` audit rule stays silent throughout**, because it triggers only on a net
  increase and a sweep is a net decrease. No rule, gate, or config changes.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Leave the bodies grandfathered (ADR-0115's status quo) | The maintainer's approval was always broader; the chain narrowed it on a principle it invented. The corpus keeps modelling the punctuation the project bans, to the agents it steers. |
| Sweep without an ADR, as pure orthography | ADR-0115 item 5 and the rendered adr-system narrative both say the carve-out is heading-only. Leaving them contradicting reality recreates the exact failure item 5 exists to prevent: the next agent reads them and refuses or re-litigates. |
| Amend ADR-0115 in place | It is Implemented, so its body is frozen; append-only permits editing only status and cross-reference metadata on a live ADR. Partial-item supersedence with a back-pointer is the sanctioned form (ADR-0116). |
| Declare an invariant and gate the authored corpus | Needs a permanent exemption mechanism for item 3's judgement, which ADR-0113 and ADR-0115 both refused; converts elective hygiene into a standing gate over hand-authored records, which is what ADR-0117's advisory severity deliberately avoids. |
| Blind mechanical substitution (every em-dash to a colon) | Corrupts bracketing pairs: "the template - and any future template - must describe" becomes ": and any future template:". Roughly 700 occurrences sit on lines carrying two or more, and pairs span the hard-wrapped lines. |
| Exempt fenced code blocks as specimens | Tempting, and it was the initial recommendation: a fence preserves the code as it stood at plan time. Rejected because most `go` fences quote string literals ADR-0115 has since made illegal and the parent effort already cleaned, so the "preserved" specimen is a record of a form the project bans, and it would leave about 895 occurrences and a probe that never returns clean. |
| Sweep prose fences but exempt code fences | Splits the 895 by language and needs a per-fence judgement the probe cannot express; the 424 unlabeled fences, the largest group, are heterogeneous and would each need classifying. |
| Extend the sweep to `docs/research/` | Not part of the decision corpus agents are steered by, and no approval covers it. Left out deliberately rather than swept for tidiness. |
| Fold the emitted-surface gap (the 24 glyphs in generated docs) into this ADR | A different topic: what counts as an emitted surface, not how far append-only bends. It would pull `invariants` into this ADR's domains and widen its tags to domain scale, and it changes a gate where this ADR changes none. Left to an immediate follow-up. |
| Sweep `docs/decisions` now, `docs/plans` later | Two efforts, two reviews, and a corpus that stays half-converted in between, for no reduction in total risk: the word-stream proof scales to both at once. |
