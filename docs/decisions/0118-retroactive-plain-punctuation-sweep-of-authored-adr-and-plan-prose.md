---
status: Proposed
date: 2026-07-15
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [doc-standard, adr-lifecycle, plan-taxonomy, active-md, verification-discipline]
related: [28, 113, 115, 117]
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

- **The corpus is large but bounded.** `docs/decisions`: 113 files carrying 2341 em-dashes, 27
  en-dashes, 133 ellipses, 0 curly quotes. `docs/plans`: 78 files carrying 4347, 154, 228, 0. About
  7200 sites across 191 files. No occurrence in any ADR body sits inside a fenced code block (0 of
  2341), so no code example is at risk.
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
   every ADR and plan.** This is partial-item supersedence of **ADR-0115 Decision item 5**, whose
   "the heading line only, never the body" clause is overridden; ADR-0115 stays Implemented and
   every other item stands, and it carries `related: [118]` as the back-pointer (ADR-0116). The
   scope is `docs/decisions/**.md` and `docs/plans/**.md`. `docs/research/` is deliberately out of
   scope: it is not part of the decision corpus agents are steered by, and no approval covers it.

2. **Prose that is about a banned glyph keeps its glyph.** Where a document names, depicts, or
   discusses one of the seven codepoints as its subject, the occurrence stays; normalizing it would
   destroy the meaning the sentence carries. At the time of writing this exempts exactly one file,
   `docs/decisions/0113-em-dash-free-shipped-templates.md`, the ADR about the em-dash. The rule is
   stated by intent rather than as a file list, so it stays true as the corpus grows. This is a
   judgement, not a mechanical test, which is why the completeness check in item 6 names the
   exemption explicitly rather than inferring it.

3. **The sweep covers all seven banned codepoints, not just the em-dash, and it includes headings.**
   ADR-0028's en-dashed title is normalized here, closing the gap ADR-0115 item 5 left when it swept
   for one codepoint under a seven-codepoint ban. Every emitted ADR title is then free of all seven.

4. **The five bounding conditions of ADR-0115 item 5 carry over unchanged, with one widened.**
   Retroactive normalization is permitted only when: it changes punctuation and nothing else; it is
   limited to the seven banned codepoints; **every word is preserved**; a maintainer approves it
   explicitly; and it is never a licence to edit an ADR's argument. Only the first condition's reach
   changes, from the heading line to the body. Append-only protects rationale, not orthography, and
   an argument's orthography is not its argument.

5. **"Every word is preserved" is proven mechanically, not asserted.** A word-stream comparison
   (strip every non-alphanumeric character, then diff the resulting token streams) must report zero
   delta across the swept corpus. This is the check that makes the sweep defensible: it reduces the
   risk from "content was silently lost across 191 frozen records" to "a punctuation choice reads
   awkwardly", which is recoverable prose, not lost rationale. The parent effort proved the
   technique on the changelog, where it caught the word count moving 8590 to 8593 and confirmed the
   delta was exactly the three words of one intended rephrasing.

6. **Completeness is proven by a scoped probe, run at the granularity the rule governs.** The sweep
   is done when a scan of `docs/decisions/**.md` and `docs/plans/**.md` for the seven codepoints
   returns nothing outside the item-2 exemption. The probe names the exemption rather than reading a
   wider unit and tolerating a residue, so a nonzero result is a defect and not a judgement call.

7. **Replacement is by sentence structure, never a blind substitution.** An em-dash becomes the
   punctuation the sentence wants: a colon where what follows explains what precedes, a semicolon
   between independent clauses, a comma for a light aside, and parentheses for a parenthetical. A
   pair of em-dashes bracketing a clause becomes a pair of parentheses. An en-dash in a numeric or
   token range becomes an ASCII hyphen. An ellipsis becomes three periods. A bare hyphen is never
   substituted for an em-dash. A blind rule would corrupt the roughly 700 occurrences that sit on
   lines carrying two or more, and the corpus is hard-wrapped, so a bracketing pair can span lines.

8. **This authorises one act; it creates no new ongoing rule.** The forward-looking regime is
   unchanged and already owned: ADR-0115 gates the emitted surfaces, ADR-0117 warns on newly
   authored prose. This ADR declares no invariant and adds no check, because a completed sweep is
   history rather than a property to maintain: nothing reintroduces the glyphs except new authoring,
   which ADR-0117 already covers.

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
- **ADR-0028's en-dashed title is fixed, and with it the last banned codepoint in generated
  output.** `docs/decisions/ACTIVE.md` and `docs/domains/adr-system.md` re-render clean.
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
| Declare an invariant and gate the authored corpus | Needs a permanent exemption mechanism for item 2's judgement, which ADR-0113 and ADR-0115 both refused; converts elective hygiene into a standing gate over hand-authored records, which is what ADR-0117's advisory severity deliberately avoids. |
| Blind mechanical substitution (every em-dash to a colon) | Corrupts bracketing pairs: "the template - and any future template - must describe" becomes ": and any future template:". Roughly 700 occurrences sit on lines carrying two or more, and pairs span the hard-wrapped lines. |
| Extend the sweep to `docs/research/` | Not part of the decision corpus agents are steered by, and no approval covers it. Left out deliberately rather than swept for tidiness. |
| Sweep `docs/decisions` now, `docs/plans` later | Two efforts, two reviews, and a corpus that stays half-converted in between, for no reduction in total risk: the word-stream proof scales to both at once. |
