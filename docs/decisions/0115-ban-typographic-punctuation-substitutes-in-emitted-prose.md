---
status: Implemented
date: 2026-07-15
tags: [template-residue, doc-standard, active-md, adr-lifecycle]
related: [28, 31, 73, 82, 112, 113, 117, 118, 119]
domains: [rendering, adr-system, invariants, tooling]
---
# ADR-0115: Ban typographic punctuation substitutes in emitted prose

## Context

ADR-0113 banned the em-dash (U+2014) from the embedded `templates.FS` and backed the ban with
`TestTemplateNoEmDash`. That check walks the template FS. It is therefore structurally blind to
every other way awf puts prose in front of an adopter, and awf is currently falling through the
gap in its own tree:

- `internal/adr/adr.go:131` hardcodes an em-dash as the separator when it *generates*
  `docs/decisions/ACTIVE.md`. That is one occurrence per ADR row, so it grows with the index: 115
  in this repository at the time of writing, and a matching set in every adopter's. The gate
  cannot see it, because a Go string literal is not a template file. This is the same defect
  commit `d0161c9` already fixed once in a different code path, when the `awf:edit` pointer
  separator became a colon.
- 70 further em-dashes sit in production Go string literals: error messages, and the output of
  `awf context` (`cmd/awf/context.go:156,162,174`) and `awf list` (`internal/initspec/initspec.go:248,293`).
  Those are awf speaking to an adopter in awf's own voice.
- `changelog/CHANGELOG.md` carries 103 em-dashes and 5 ellipses, and `awf changelog` prints it
  **verbatim** to the adopter (`cmd/awf/changelog.go:38`, `fmt.Fprint(stdout, e.Raw)`). It is
  embedded exactly as the templates are: `changelog/embed.go` is a top-level package for the same
  reason `templates/embed.go` is, because `go:embed` cannot reach outside its own directory. Two
  embedded FS values ship to adopters, and ADR-0113 scanned only one of them.
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
read identically with an ASCII hyphen. The 20 in-scope ellipses (10 in the templates, 5 in the
changelog, 5 in Go string literals) mark elision in code and format examples, and read identically
as three periods. Curly quotes are at zero occurrences across all three surfaces, so listing them
costs no cleanup at all. Every count here is measured over the scope this ADR defines: Go figures
come from an AST walk of string literals in non-test files, never a whole-file grep, which would
sweep in the comments Decision item 4 excludes.

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

3. **Scope is the awf-owned source surfaces that reach an adopter.** That is the union of three:
   every file in the embedded `templates.FS`, every file in the embedded `changelog.FS`, and every
   string literal in production Go under `internal/` and `cmd/`. One gate test covers all three.
   The deliberate exclusion is content that renders from `.awf/` sidecar data and convention
   parts: `docs/pitfalls.md`, `docs/glossary.md` and `docs/decisions/README.md` are emitted
   artifacts fed from per-project sources, which are adopter-owned by Decision item 6 and belong
   to the follow-up ADR's advisory rule. "Everything awf emits" would be the wrong claim: it is
   everything awf *ships*, which is what an invariant over awf's own source can honestly promise.

4. **Go comments and `_test.go` files are out of scope, by design and not by oversight.** Test
   failure strings are not emitted prose, and `internal/project/residue_scan_test.go:41` already
   contains an em-dash in a `t.Error` message. Comments carry a hard mechanical reason: gofmt's
   doc-comment normalization rewrites a double-backtick pair into U+201C, so scanning comments
   would pit the gate against gofmt in a loop neither wins (recorded in `.awf/docs/pitfalls.yaml`,
   entry "gofmt rewrites double backticks in doc comments into curly quotes").

5. **The three em-dashed ADR titles are normalized retroactively. This is elective, and it is not
   a prose edit.** ADR-0007, ADR-0018 and ADR-0022 have the em-dash in their `# ` heading replaced
   by a colon or comma. Nothing forces this: the ban reaches templates, the changelog, and Go
   string literals, and ADR files are none of those, so the gate is green whether or not the
   titles change. It is taken deliberately, with maintainer approval, because those titles are
   harvested into `ACTIVE.md` and four domain docs that agents read as context, and a record that
   half-follows its own convention teaches the wrong one.

   Two forces are in genuine tension here and both are recorded rather than reconciled
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
   no meaning is normalization, not revision. The carve-out is deliberately narrow, and all five
   conditions must hold:

   1. It is limited to the `# ` heading line, never the body.
   2. It is limited to the seven banned codepoints.
   3. It must preserve every word.
   4. It requires explicit maintainer approval.
   5. It is never a licence to edit an ADR's argument.

   Condition 1 is the binding one and is stated first deliberately. Without it, the test in the
   paragraph above ("changes no word and no meaning") reads as a general licence, and a future
   agent could apply it to the 2344 em-dashes in Implemented ADR bodies. It cannot: those are out
   of scope, permanently, and no approval extends this carve-out to them.

6. **Adopter-authored strings are warned about, never rewritten.** awf does not normalize an
   adopter's ADR title when harvesting it into their `ACTIVE.md` or domain docs. Silently mutating
   adopter content would cross the boundary ADR-0113 drew and ADR-0082 draws. An adopter's own
   prose is the follow-up ADR's concern, which warns rather than rewrites.

7. **Depiction is handled by convention and scope, not an exemption list.** ADR-0113 Decision item
   3's no-escape-hatch posture carries over unchanged, and item 4's word-and-codepoint convention
   is extended from one codepoint to seven: a doc that must discuss a banned character names it by
   word and codepoint rather than typing the glyph, as this ADR does throughout. Where a doc must
   genuinely *depict* the glyph, scope rather than exemption makes room for it: the
   `.awf/docs/pitfalls.yaml` entry "gofmt rewrites double backticks in doc comments into curly
   quotes" types a curly quote to document that rewrite and stays legal because sidecar data is out
   of scope per item 3. No exemption list ships.

8. **The generated ACTIVE.md status separator becomes parentheses.** A row renders as
   `- [ADR-0001: Title](0001-file.md) (Accepted)`. A colon is unavailable because ADR titles
   already contain one, and a hyphen reads mushily against the list item's own leading hyphen.

9. **The documentation authoring standard carries the widened rule.**
   `templates/docs/doc-standard.md.tmpl:16` currently bans the em-dash alone; it is rewritten to
   name all seven codepoints by word and codepoint, and `./x sync` re-renders `docs/doc-standard.md`
   in the same commit. This discharges ADR-0113 item 4's obligation **for the authors the standard
   reaches**: whoever writes a doc learns the rule from the standard, not from a failing gate. It
   does not reach the author of a Go string literal, who is item 10's subject. The template is
   itself scanned by the new gate, so its rule text must not type the glyphs it bans.

   ADR-0117 item 8 widens this same line's scope clause, from shipped prose to all awf-managed
   prose. When both ADRs land in one effort the line is therefore written **once**, carrying both
   widenings, under a single changelog entry; an implementer following this item alone would write
   "Shipped prose" and silently undo that widening.

10. **The agent guide carries the rule, reversing ADR-0113's judgement on the widened scope.**
    ADR-0113 Consequences reasoned that `template-em-dash-free` earned no bullet in the agent
    guide's Invariants list: under the core-only criterion (ADR-0112) it was "a subsystem-specific
    rendering invariant reached on demand via `awf context`". That reasoning was sound for a ban
    scoped to `templates.FS`. It does not survive this ADR's widening. The criterion
    (`docs/agents-md-standard.md`) admits a rule *iff* it is not scoped to a single subsystem's
    files and instead constrains a whole cross-cutting surface, naming "all code" among them; item
    3's scope is every string literal in production Go under `internal/` and `cmd/`, which is all
    production code, plus two embedded FS values. The judgement is reversed here rather than left in
    a Superseded document, because a premise that changed silently is how a rule gets re-litigated.

    Item 9 is not sufficient on its own, and the gap is the reason this item exists. The
    documentation standard is a *documentation* authoring standard: it teaches whoever writes a
    doc. A Go string literal is not a doc, so an agent adding an error message would meet this
    rule only by tripping the gate, which is the precise failure ADR-0113 item 4
    (`cites: ADR-0113#4`) was written to prevent. The guide bullet is where that author actually
    reads.

    The bullet is one terse line naming the seven codepoints and the three surfaces, added to the
    `invariants:` list in the guide's sidecar data. It does not restate the mechanism: that lives
    here, per the criterion's own rule.

11. **Supersedence bookkeeping (migrated from supersedes: by awf upgrade,
   ADR-0128).** This ADR retires every anchor of ADR-0113: `supersedes: ADR-0113#1`, `supersedes: ADR-0113#2`, `supersedes: ADR-0113#3`, `supersedes: ADR-0113#4`, `supersedes-invariant: ADR-0113#template-em-dash-free`

## Invariants

- `` `invariant: emitted-prose-no-typographic-substitutes` ``: no file in the embedded
  `templates.FS`, no file in the embedded `changelog.FS`, and no string literal in a non-test Go
  file under `internal/` or `cmd/`, contains U+2014, U+2013, U+2026, U+2018, U+2019, U+201C, or
  U+201D. Backed by `TestEmittedProseNoTypographicSubstitutes` in
  `internal/project/residue_scan_test.go`, which walks both embedded FS values and parses each
  production Go file with `go/parser`, inspecting every `*ast.BasicLit` of kind `STRING`. The test
  carries a seen-count guard for each of the three surfaces, so that a mis-anchored walk root
  fails rather than passes vacuously.

This invariant strictly subsumes `template-em-dash-free` (ADR-0113): the same boundary, widened
from one codepoint to seven and from one shipped surface to three.

The implementing commit deletes `TestTemplateNoEmDash` and its `invariant: template-em-dash-free`
proof marker (`internal/project/residue_scan_test.go:90`), replacing both with the new test and
marker atomically. Until it lands, `awf invariants` reports an advisory note that the marker names
a slug no Implemented ADR declares. That note is expected, is visible on `main` from the commit
that proposed this ADR, and is the correct transitional signal rather than a defect: the ban stays
enforced throughout, because `TestTemplateNoEmDash` keeps running in the gate regardless of its
ledger status.

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

- **The emission hole closes.** The class of defect that put an em-dash into every row of every
  adopter's generated index, invisible to a green gate, is now caught deterministically at its
  source, across all three shipped surfaces.
- **This is an adopter-facing release, not a quiet internal fix.** Changing the ACTIVE.md
  separator makes every adopter's committed `ACTIVE.md` drift against the new binary, so their
  `awf check` fails until they re-sync. The flip commit therefore carries a changelog entry and an
  upgrade note. `cmd/repoaudit` independently flags an adopter-facing change whose `[Unreleased]`
  section is unchanged, as a `warning` rather than an error: ADR-0107 downgraded that rule because
  a path heuristic cannot tell a benign change from a behavioural one. The obligation here is
  therefore the ADR's, not the tooling's. The in-repo adopter example is affected identically:
  `./x` runs `awf sync`
  and `awf check` inside `examples/sundial`, so the landing commit must stage the regenerated
  `examples/sundial/docs/decisions/ACTIVE.md` (3 rows) or `./x check` fails on drift there.
- **Past release notes are rewritten, and published ones diverge.** Cleaning the 103 em-dashes and
  5 ellipses in `changelog/CHANGELOG.md` edits entries for versions already released, so the
  repository's changelog no longer matches the release notes published for those tags. Accepted
  deliberately: `awf changelog` prints the embedded file verbatim, so leaving it dirty would mean
  awf shipping the banned glyph to every adopter who runs the command, which is the precise defect
  this ADR exists to close. No word changes; only punctuation.
- **The append-only invariant gains a stated, narrow exception.** Recording it here is the point:
  without it, a future agent reads ADR-0028 and refuses an identical normalization, or worse,
  reads this ADR's retitles as licence to edit ADR rationale. Decision item 5 is written to make
  both readings impossible.
- **The rule is easier to follow than its predecessor.** Seven banned codepoints with no
  conditional carve-out replaces "em-dash banned, en-dash and ellipsis permitted", which required
  an author to remember which typographic character was acceptable and why.
- **The agent guide spends one of its scarce slots, and that cost is the point of item 10.** The
  guide is always-on context, so every bullet is a recurring token cost on every session and the
  core-only criterion (ADR-0112) exists to keep the list short. This rule earns the slot because it
  binds every author of a production Go string, which is most changes to this repository, and
  because no other always-on doc reaches that author: the documentation standard teaches doc
  authors. The flip commit therefore also edits the guide's sidecar data and re-renders `AGENTS.md`,
  an obligation ADR-0113's flip commit explicitly did not carry.
- **Authored ADR and plan bodies remain untouched.** 2344 em-dashes in authored ADR bodies under
  `docs/decisions/` and 4347 under `docs/plans/` are out of scope and stay. The line is principled
  rather than pragmatic: titles are harvested into generated output, bodies are not.
- **Two blind spots remain, both stated rather than overlooked.** First, content rendered from
  `.awf/` sidecar data and convention parts: 23 ellipses reach `docs/pitfalls.md`,
  `docs/glossary.md` and `docs/decisions/README.md` from per-project sources the scan does not
  read. Decision item 7 depends on this, so it is a designed boundary, not an accident, and the
  follow-up ADR's advisory rule is where it gets addressed. Second, seven tracked files outside
  any scanned surface carry em-dashes (`x`, `.githooks/pre-commit`, both `.github/workflows/`
  files, `.goreleaser.yaml`, `.gremlins.yaml`, `codecov.yml`). Two of those matter: `x` and
  `.githooks/` are co-owned units whose *rendered* counterparts must satisfy the ban, so awf's own
  from-source runner violates the rule its own template must pass. awf disables both units for
  itself, so no rendered artifact is affected, but it sits against the agent guide's requirement
  that awf model what it generates. Deferred deliberately.
- **Adopter prose stays the adopter's.** Decision item 6 keeps this ADR strictly about awf's own
  voice, which is what lets it supersede ADR-0113 without reopening ADR-0113's boundary.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep ADR-0113 and fix `adr.go:131` as a plain bug | Treats the symptom. The gate would stay blind to Go string literals, so the next generated-output em-dash lands the same way, which is exactly the "removed, then reintroduced with nothing to catch it" signal ADR-0113 was written to answer. |
| Ban all non-ASCII characters | Measurably fails on landing: 43 arrows encode the workflow chain in `templates.FS`, and an author's name carries U+00E4. Notation is not a punctuation substitute. |
| Ban the em-dash and en-dash only, keeping the ellipsis | Leaves one machine-set substitute uncovered and forces the rule to carry a stated exception, for a saving of 20 mechanical edits. |
| Leave the three ADR titles em-dashed | The ban does not reach `docs/decisions/`, so the gate passes either way and this costs nothing. Rejected because harvested titles are read by agents as context, and a record that half-follows its own convention teaches the wrong one. Elective, and taken only with maintainer approval. |
| Normalize ADR titles at harvest time instead of retitling | Respects append-only without a carve-out, but it would silently rewrite an *adopter's* title in their own generated index, crossing the adopter-content boundary. Warning beats rewriting. |
| Leave `changelog/CHANGELOG.md` out of scope | Would avoid rewriting released entries and keep the repository's changelog matching published release notes. Rejected because `awf changelog` prints the embedded file verbatim, so the banned glyph would still reach every adopter through awf's own voice, leaving the invariant's name a lie. |
| Fold the authored-prose rule into this ADR | Two decisions with different rationales (publication safety here, house style there), different boundaries (awf's voice vs the adopter's), and different severities (gate vs advisory). Split so neither blocks the other; the follow-up ADR carries Tier 2. |
| Extend the residue guard (ADR-0082) instead of a new test | The residue guard's invariant is about ADR citations and repo identity. ADR-0113 already reasoned that a separate slug keeps each guard's meaning clean, and that reasoning survives supersession. |

## Migration history

- 2026-07-15: retired invariant `ADR-0113#template-em-dash-free`; basis: encoded
