---
status: Proposed
date: 2026-07-15
supersedes: [113]
retires_invariants: []
superseded_by: ""
tags: [template-residue, doc-standard, active-md, adr-lifecycle]
related: [28, 31, 73, 82, 112, 113]
domains: [rendering, adr-system, invariants, tooling]
---
# ADR-0115: Ban typographic punctuation substitutes in emitted prose

## Context

ADR-0113 banned the em-dash (U+2014) from the embedded `templates.FS` and backed the ban with
`TestTemplateNoEmDash`. That check walks the template FS. It is therefore structurally blind to
every other way awf puts prose in front of an adopter, and awf is currently falling through the
gap in its own tree:

- `internal/adr/adr.go:131` hardcodes an em-dash as the separator when it *generates*
  `docs/decisions/ACTIVE.md`. That is 114 occurrences in this repository's generated index and a
  matching set in every adopter's. The gate cannot see it, because a Go string literal is not a
  template file. This is the same defect commit `d0161c9` already fixed once in a different code
  path, when the `awf:edit` pointer separator became a colon.
- 72 further em-dashes sit in production Go string literals: error messages, and the output of
  `awf context` (`cmd/awf/context.go:156,162,174`) and `awf list` (`internal/initspec/initspec.go:248,293`).
  Those are awf speaking to an adopter in awf's own voice.
- ADR-0113 Decision item 2 asserted that hand-authored ADRs are "authored records, never rendered
  output". That premise is false. ADR *titles* are harvested into generated output: three titles
  (ADR-0007, ADR-0018, ADR-0022) carry em-dashes and render into four `docs/domains/*.md` files
  and into `ACTIVE.md`.

Measuring the rest of the surface reframed the rule itself. ADR-0113 kept the en-dash (U+2013)
and the ellipsis (U+2026) legitimate, on the reasoning that they "exist legitimately in the
templates today, so a broader ban would fail on landing and demand an unrequested cleanup". The
cleanup is now requested, and the measurement shows it is negligible: all 5 en-dashes in the
shipped surface are numeric ranges (`templates/skills/refactor-coupling-audit/SKILL.md.tmpl:33`,
`templates/partials/review-spine-tail.md:24`, `internal/catalog/standard.go:130,147,167`), which
read identically with an ASCII hyphen. The 31 ellipses mark elision in code and format examples,
which read identically as three periods. Curly quotes are at zero occurrences in both the
templates and production Go, so listing them costs no cleanup at all.

The full non-ASCII inventory also rules out the tempting simplification. `templates.FS` contains
43 rightwards arrows (U+2192) and 7 left-right arrows (U+2194) encoding the workflow chain, plus
U+2265, U+2260, U+00D7 and U+00B7; production Go carries U+00E4 and U+00FC in an author name. An
ASCII-only rule would fail on landing and would destroy meaningful notation. The coherent rule is
a blocklist of the characters that *substitute* for ASCII punctuation, not an allowlist of ASCII.

## Decision

1. **Emitted prose carries no typographic punctuation substitute.** Seven codepoints are banned:
   the em-dash (U+2014), the en-dash (U+2013), the ellipsis (U+2026), and the four curly quotes
   (U+2018, U+2019, U+201C, U+201D). Where an em-dash would have gone, use a colon, semicolon,
   comma, or parentheses; for a range, an ASCII hyphen; for elision, three periods; for quoting,
   the ASCII apostrophe and double quote.

2. **Notation is not punctuation and stays legal.** Arrows, mathematical symbols, and accented
   letters are unaffected. This is a closed blocklist of seven codepoints, never an ASCII-only
   allowlist. A future author extends the list by amending this ADR, not by reading intent into it.

3. **Scope is everything awf emits.** That is the union of two source surfaces: every file in the
   embedded `templates.FS`, and every string literal in production Go under `internal/` and
   `cmd/`. One gate test covers both.

4. **Go comments and `_test.go` files are out of scope, by design and not by oversight.** Test
   failure strings are not emitted prose, and `internal/project/residue_scan_test.go:38` already
   contains an em-dash in a `t.Error` message. Comments carry a hard mechanical reason: gofmt's
   doc-comment normalization rewrites a double-backtick pair into U+201C, so scanning comments
   would pit the gate against gofmt in a loop neither wins (recorded in `.awf/docs/pitfalls.yaml:267`).

5. **The three em-dashed ADR titles are normalized retroactively, and this is not a prose edit.**
   ADR-0007, ADR-0018 and ADR-0022 have the em-dash in their `# ` heading replaced by a colon or
   comma. Two forces are in genuine tension here and both are recorded rather than reconciled
   away. Against: ADR-0028's Alternatives table refused to rewrite ADR-0022's prose on
   append-only grounds, and a heading is prose. For: the project has twice normalized Implemented
   ADRs mechanically and retroactively with maintainer approval, in commit `a495521`
   ("normalize related/supersedes lists to bare ints") and commit `8a2602d` (retro-tagging every
   ADR), which establishes that a meaning-preserving sweep across settled records is accepted
   practice; those two touched frontmatter rather than a heading, so this ADR extends the practice
   by one step rather than inventing it.

   The line this ADR draws: **append-only protects rationale, not orthography.** No argument,
   decision item, alternative, or consequence in an Implemented ADR may be rewritten, and
   ADR-0028's refusal stands in full for that. A punctuation substitution that changes no word and
   no meaning is normalization, not revision. The carve-out is deliberately narrow, and all four
   conditions must hold: it is limited to the seven banned codepoints, it must preserve every
   word, it requires explicit maintainer approval, and it is never a licence to edit an ADR's
   argument. The motivation is that these titles are read by agents as context, so a clean
   historical record is a working input, not cosmetics.

6. **Adopter-authored strings are warned about, never rewritten.** awf does not normalize an
   adopter's ADR title when harvesting it into their `ACTIVE.md` or domain docs. Silently mutating
   adopter content would cross the boundary ADR-0113 drew and ADR-0082 draws. An adopter's own
   prose is the follow-up ADR's concern, which warns rather than rewrites.

7. **A depiction exemption replaces ADR-0113's no-escape-hatch posture.** A doc that documents one
   of the banned codepoints must be able to depict it. The convention is to name the character by
   word and codepoint rather than typing the glyph, as this ADR does throughout. Because the ban
   is scoped to templates and Go string literals, `.awf/` sidecar data stays out of scope, which
   is what lets `.awf/docs/pitfalls.yaml:267` keep typing a curly quote to document gofmt's
   rewrite. No exemption list ships; the scope boundary does the work.

8. **The generated ACTIVE.md status separator becomes parentheses.** A row renders as
   `- [ADR-0001: Title](0001-file.md) (Accepted)`. A colon is unavailable because ADR titles
   already contain one, and a hyphen reads mushily against the list item's own leading hyphen.

## Invariants

- `` `invariant: emitted-prose-no-typographic-substitutes` ``: no file in the embedded
  `templates.FS`, and no string literal in a non-test Go file under `internal/` or `cmd/`,
  contains U+2014, U+2013, U+2026, U+2018, U+2019, U+201C, or U+201D. Backed by
  `TestEmittedProseNoTypographicSubstitutes` in `internal/project/residue_scan_test.go`, which
  walks the template FS and parses each production Go file with `go/parser`, inspecting every
  `*ast.BasicLit` of kind `STRING`. The test carries a seen-count guard so that a mis-anchored
  walk root fails rather than passes vacuously.

This invariant strictly subsumes `template-em-dash-free` (ADR-0113): the same boundary, widened
from one codepoint to seven and from one surface to two.

`template-em-dash-free` leaves enforcement through **full supersession, not through
`retires_invariants`**, and the distinction is load-bearing rather than stylistic.
`DeclaringADRs` (`internal/invariants/invariants.go:133`) builds the required set from
`Implemented` ADRs only, so flipping ADR-0113 to `Superseded by ADR-0115` already drops the slug.
Listing it in this ADR's `retires_invariants` as well would then match nothing in the required set
and fail `awf check` with `dangling retirement` (ADR-0031 Decision item 3). The two mechanisms are
mutually exclusive: `retires_invariants` exists to retire a slug whose declaring ADR *remains*
`Implemented`, which is why ADR-0032 could retire `setup-guards-hookspath` while ADR-0023
continued to declare it. Do not "repair" this ADR by populating `retires_invariants`.

## Consequences

- **The emission hole closes.** The class of defect that put 114 em-dashes into every adopter's
  generated index, invisible to a green gate, is now caught deterministically at its source.
- **This is an adopter-facing release, not a quiet internal fix.** Changing the ACTIVE.md
  separator makes every adopter's committed `ACTIVE.md` drift against the new binary, so their
  `awf check` fails until they re-sync. The flip commit therefore carries a changelog entry and an
  upgrade note. `cmd/repoaudit` independently flags an adopter-facing change with no changelog
  entry as an Error.
- **The append-only invariant gains a stated, narrow exception.** Recording it here is the point:
  without it, a future agent reads ADR-0028 and refuses an identical normalization, or worse,
  reads this ADR's retitles as licence to edit ADR rationale. Decision item 5 is written to make
  both readings impossible.
- **The rule is easier to follow than its predecessor.** Seven banned codepoints with no
  conditional carve-out replaces "em-dash banned, en-dash and ellipsis permitted", which required
  an author to remember which typographic character was acceptable and why.
- **Authored ADR and plan bodies remain untouched.** 2461 em-dashes under `docs/decisions/` and
  4347 under `docs/plans/` are out of scope and stay. The line is principled rather than
  pragmatic: titles are emitted into generated output, bodies are not.
- **Non-Go, non-template files remain a stated blind spot.** Seven tracked files carry em-dashes
  outside the scanned surface (`x`, `.githooks/pre-commit`, both `.github/workflows/` files,
  `.goreleaser.yaml`, `.gremlins.yaml`, `codecov.yml`). Two matter: `x` and `.githooks/` are
  co-owned units whose *rendered* counterparts must satisfy the ban, so awf's own from-source
  runner would violate the rule its own template must pass. Deliberately deferred rather than
  overlooked; awf disables both units for itself, so no rendered artifact is affected.
- **Adopter prose stays the adopter's.** Decision item 6 keeps this ADR strictly about awf's own
  voice, which is what lets it supersede ADR-0113 without reopening ADR-0113's boundary.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep ADR-0113 and fix `adr.go:131` as a plain bug | Treats the symptom. The gate would stay blind to Go string literals, so the next generated-output em-dash lands the same way, which is exactly the "removed, then reintroduced with nothing to catch it" signal ADR-0113 was written to answer. |
| Ban all non-ASCII characters | Measurably fails on landing: 43 arrows encode the workflow chain in `templates.FS`, and an author's name carries U+00E4. Notation is not a punctuation substitute. |
| Ban the em-dash and en-dash only, keeping the ellipsis | Leaves one machine-set substitute uncovered and forces the rule to carry a stated exception, for a saving of 31 mechanical edits. |
| Normalize ADR titles at harvest time instead of retitling | Respects append-only without a carve-out, but it would silently rewrite an *adopter's* title in their own generated index, crossing the adopter-content boundary. Warning beats rewriting. |
| Fold the authored-prose rule into this ADR | Two decisions with different rationales (publication safety here, house style there), different boundaries (awf's voice vs the adopter's), and different severities (gate vs advisory). Split so neither blocks the other; the follow-up ADR carries Tier 2. |
| Extend the residue guard (ADR-0082) instead of a new test | The residue guard's invariant is about ADR citations and repo identity. ADR-0113 already reasoned that a separate slug keeps each guard's meaning clean, and that reasoning survives supersession. |
