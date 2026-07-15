---
status: Proposed
date: 2026-07-16
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [template-residue, doc-standard]
related: [8, 36, 48, 82, 86, 88, 94, 113, 115, 116, 117, 118]
domains: [invariants, rendering, tooling, adr-system]
---
# ADR-0119: Repo-wide plain punctuation: the remaining surfaces and an opt-in prose gate

## Context

Three ADRs already govern the seven banned typographic substitutes (the em-dash U+2014, en-dash
U+2013, ellipsis U+2026, and the four curly quotes U+2018, U+2019, U+201C, U+201D). ADR-0115 gates
three shipped surfaces. ADR-0117 warns, advisorily, when a commit raises a count in authored
markdown. ADR-0118 swept the authored ADR and plan corpus clean and closed with item 10, which
states the remaining gap and defers it here: three sidecar sources are checked by nothing, because
ADR-0117 reads only markdown under the docs directory and skips generated paths, while their render
targets are generated and excluded at the other end.

This ADR answers the question ADR-0118 named but declined: **what counts as a checked surface.**

### The measurement

388 banned codepoints remain in 51 tracked files. Measured 2026-07-16, by codepoint, over
`git ls-files`:

| surface | glyphs | files | note |
|---|---|---|---|
| `docs/research/` | 186 | 2 | ADR-0118 item 1 excluded it |
| `_test.go` fixtures | 108 | 22 | ADR-0115 item 4 excluded it |
| `.awf/` sidecar sources | 24 | 3 | render into the next row |
| generated docs | 24 | 3 | `docs/pitfalls.md`, `docs/glossary.md`, `docs/decisions/README.md` |
| production Go comments | 17 | 11 | ADR-0115 item 4 excluded it |
| repo infrastructure | 15 | 7 | `x`, `.githooks/pre-commit`, CI, and four yaml files |
| `README.md` | 6 | 1 | inside an ASCII box diagram |
| exempt: ADR-0113 | 7 | 1 | ADR-0118 item 2 |
| exempt: the plan depiction | 1 | 1 | ADR-0118 item 3 |

Two figures in ADR-0118 item 10 are already stale and are corrected here rather than propagated.
It hands this ADR "production Go comments (13 occurrences, all ellipses)". The count is 17, and
they are not all ellipses: `changelog/embed.go:3` carries an em-dash. The count moved because the
parent effort's own commits added comment prose after item 10 was measured.

Two premises were verified mechanically rather than assumed, because both had been asserted from
memory and one turned out to be wrong:

- **Every production-Go occurrence sits in a comment; none is in a string literal.** ADR-0115's
  gate walks string literals through an AST, so it has no hole. The 17 are invisible to it by
  design, not by defect.
- **The 108 test-file occurrences are fixture inputs, not assertions about shipped output.** They
  are prose inside fixture bodies fed to a renderer or a parser. A sweep of all 22 files on a
  scratch copy left `go test ./...` fully green. No parser reads the glyph as a separator: the
  invariant ledger's `declRe` and `slugRe` bound a slug to `[a-z0-9-]+` and stop at the space
  before it, so the glyph is inert description text.

### The gofmt premise, corrected

ADR-0115 item 4 excluded Go comments for a stated mechanical reason: gofmt rewrites a
double-backtick pair into U+201C, so scanning comments "would pit the gate against gofmt in a loop
neither wins". The behaviour is real and was reproduced. The conclusion drawn from it was too
broad, and the shape of the rewrite was recorded imprecisely.

Tested directly: gofmt rewrites a double-backtick pair only in a **declaration-attached doc
comment**. It does not touch a free-floating comment, a block comment, or a comment inside a
function body. It never produces a dash, an ellipsis, or a single curly quote. A single backtick
survives untouched. The distinction is declaration-attached or not, and not, as the pitfalls entry
implies, a matter of comment syntax: a `//` comment above a `func` is a doc comment and is
rewritten.

That makes the surface gateable. When gofmt emits a curly quote, the author wrote a double
backtick, and the gate's answer is "use a single backtick". That is a loop the author wins, because
the author controls the input. The exclusion was a surrender to a fight that was never lost.

### Why an enumerated surface list is the wrong shape

ADR-0115's own Context faulted ADR-0113 for a check that "is therefore structurally blind to every
other way awf puts prose in front of an adopter", and then defined its own scope as a union of
three enumerated surfaces. That union is why 388 occurrences survive today: each new surface, the
next sidecar, the next top-level file, is born unchecked, and stays unchecked until someone
measures it. Answering "what counts as a checked surface" with a fourth enumeration would
reproduce the defect a fourth time.

The shape that does not decay is default-deny over every tracked file, with a named exemption for
each place the rule genuinely cannot reach.

### The maintainer's requirement

The requirement is standing and is quoted verbatim: "The repo MUST be clean, except for the
exceptions we defined before where it is valid to talk about", "After this is clean, I want NO new
additions that add any of the banned codepoints", and "Sidecar data is NOT out of scope, just to be
clear."

Those are two properties, not one. "No new additions" is a net increase per commit, which ADR-0117
already ships. "The repo is clean" is presence over the whole tree, which nothing checks. Presence
is the stronger of the two: once a tree is clean, presence-zero makes "no new additions"
automatic. Presence is also the property that cannot ship enabled, because it would fail every
existing adopter's build on the day they upgrade, which is precisely the outcome ADR-0117's
net-increase trigger was designed to avoid.

### ADR-0118 pre-rejected this gate, and the objection is answered rather than ignored

ADR-0118's Invariants section refuses to declare the property this ADR declares, and gives two
reasons. Both are answered here rather than left for a future reader to rediscover.

Its first reason: the property "would need a permanent exemption mechanism to express item 2's
judgement, which ADR-0113 Decision item 3 and ADR-0115 both rejected as a design". That is
correct, and this ADR adds the mechanism. What both predecessors rejected was an exemption list
**for awf's own voice**, where awf controls every byte and can therefore afford ADR-0115 item 7's
scope-over-exemption preference. The boundary that makes room here is one both predecessors already
drew for a different purpose: ADR-0115 item 6 and ADR-0117 item 5 both concede that adopter content
is treated differently from awf's own, because awf does not control it. A check that ships to an
adopter's tree cannot make awf's scope argument, because an adopter's depictions are not awf's to
scope around. The exemption is forced by the surface, not chosen as a design preference.

Its second reason: the property "would convert an elective hygiene act into a standing gate over
191 hand-authored records, which is precisely the burden ADR-0117 chose advisory severity to
avoid". This is the sharper objection and it is accepted as a cost, not dissolved. The burden is
real: after this ADR, a banned codepoint in any tracked file fails the gate. It is accepted because
the corpus is clean at the moment the gate lands, so the standing cost is only the cost of not
adding one; because the burden ADR-0117 feared was a gate over a **dirty** corpus, which is not the
situation after ADR-0118; and because the maintainer has since required exactly this.

## Decision

1. **The checked surface is every tracked text file, and the rule is default-deny.** The scope is
   `git ls-files`, minus files that are not text, minus the exemptions of item 5. There is no
   enumerated surface list, and a new file, sidecar, or directory is in scope the moment it is
   tracked. This is the whole point: the surface list is the thing that decayed, so the surface
   list is what this ADR removes. This partially supersedes **ADR-0115 Decision item 3**'s union of
   three surfaces, which stays correct for the check it governs (item 7 below) and is no longer the
   answer to "what counts as a checked surface".

2. **The remaining 356 occurrences are swept**, across the six live surfaces measured above:
   `docs/research/`, the test fixtures, the three `.awf/` sidecar sources, the production Go
   comments, the repo infrastructure, and `README.md`. The 24 occurrences in generated docs are not
   touched: they are fixed by cleaning their `.awf/` sources and re-rendering, because hand-editing
   a rendered file is forbidden outright.

   ADR-0118's method carries over unchanged and is the condition of the sweep, not a
   recommendation: replacement is by sentence structure (item 9 below), and a word-stream
   comparison must report zero delta across every swept file, proving that only punctuation moved.

3. **`docs/research/` is swept.** This partially supersedes **ADR-0118 Decision item 1**, which
   excluded it as "not part of the decision corpus agents are steered by, and no approval covers
   it". The second clause was the operative one, and this ADR is that approval. The corpus is safe
   to sweep on a ground the earlier exclusion did not consider: it carries zero curly quotes and no
   external quotation, so nothing in it is a verbatim quote a sweep could corrupt. Its occurrences
   are awf's own headings and callouts.

4. **Go comments and `_test.go` files are in scope.** This partially supersedes **ADR-0115 Decision
   item 4**, whose two reasons are both answered: the gofmt reason is refuted above, and "test
   failure strings are not emitted prose" is true but irrelevant to a rule whose subject is now the
   repository rather than the shipped artifact. All seven codepoints are scanned in comments,
   including the curly quotes, because the single-backtick fix makes that loop winnable.

5. **Exemptions are configured, keyed by path and codepoint, and live in `config.yaml`.** An entry
   names a path, a codepoint, and optionally a pinned count.

   The key is `{path, codepoint}` and not a bare path, because a bare path is a hole in the exact
   files this ADR names. `.awf/docs/pitfalls.yaml` needs one curly quote exempted while 19 of its
   ellipses must be swept; a whole-file exemption would permit all 20. The plan file of item 6
   carries one occurrence in 518 lines.

   The count is **optional**, because pinning serves two different roles. For a frozen record, a
   pinned count is free and turns the exemption into an assertion: ADR-0113 has exactly seven
   em-dashes and always will. For a living file, a pinned count would fail the gate on every edit
   that legitimately adds a depiction, forcing a config commit each time. awf pins its own; an
   adopter need not.

   The exemptions live in `config.yaml` and not in a file of their own, because ADR-0086's
   claimed-path model is an allowlist: a `.awf/prose-exemptions.yaml` would be unclaimed drift the
   day it was written.

6. **awf's own exemptions are exactly three, all pinned.** `docs/decisions/0113-em-dash-free-shipped-templates.md`
   (em-dash, 7), by ADR-0118 item 2's maintainer designation.
   `docs/plans/2026-07-13-invariant-backing-migration-to-enforced-test-scoped-backing.md` (em-dash,
   1), by ADR-0118 item 3. And the `.awf/docs/pitfalls.yaml` entry that types a curly quote to
   document gofmt's rewrite (left double quote, 1), which ADR-0115 item 7 kept legal by scope and
   which item 1 above has just brought into scope. That third exemption must name **both** the
   sidecar source and its `docs/pitfalls.md` render target, because the glyph exists twice.

   ADR-0118 item 3's justification for the second is not re-asserted. It reasoned that the plan
   reports a live-state column that "shows" an em-dash, so punctuating it would make a true report
   false. That is no longer true of the tree: `docs/config-reference.md` now shows a count, not an
   em-dash. The exemption stands on the narrower and durable ground that the plan is a frozen record
   of what was observed when it was written, and the em-dash is the token that was observed.

7. **ADR-0115's gate and its `emitted-prose-no-typographic-substitutes` invariant are kept, not
   retired.** The new gate does not subsume them, and the reason is specific: **ADR-0115's gate
   reads no configuration.** It walks two embedded filesystems and parses Go ASTs, and its answer
   cannot be changed by a config edit. The gate of item 8 reads a knob that defaults to false and an
   exemption list that any adopter, including awf, can extend. A promise about what awf **ships**
   must not be contingent on a knob, and an exemption naming a path under `templates/` would ship a
   banned codepoint that only ADR-0115's gate would catch.

   The two are mechanically redundant today and semantically distinct: one declares a property of
   the product, the other a property of the repository. They coincide only as long as no exemption
   ever names a shipped path, which is exactly the condition ADR-0115's gate exists to enforce.
   Retirement under ADR-0031 was considered and is recorded in Alternatives.

8. **A new shipped command, `awf prose-gate`.** It scans the tracked text files of the tree it is
   run in, reports every banned codepoint outside the configured exemptions, and exits non-zero on
   any finding. It is the blocking, presence-level check; adopters wire it into a `pre-commit` hook
   they own.

   The name mirrors `commit-gate`, the only other hyphenated command and the only other
   adopter-wired blocking check, because this is the same species. It is deliberately not
   `prose-check`: `awf check` is the drift oracle, a name AGENTS.md elevates to an invariant, and a
   sibling `-check` would read as a variant of a command with which it shares no machinery, no
   config, and no severity.

9. **The command self-gates on one knob, default false.** `proseGate` (bool, default `false`) is
   the only new configuration besides the exemption list. When it is false the command exits zero
   without scanning. Enforcement lives in the command rather than in the wiring, so that a hook or
   a runner may invoke it unconditionally, and so that turning the rule on is one config edit
   rather than a config edit plus a wiring edit.

   The default is false, alone among awf's advisory knobs, because this one blocks. ADR-0117's
   `plainPunctuation` defaults true and may: a warning costs an adopter nothing. Presence-level
   enforcement on an unswept tree costs them their build, so they must ask for it.

10. **awf enables the knob for itself and wires the command into its own gate.** awf is the first
    adopter of what it ships, so it does not ship an opt-in it declines to take.

11. **Outside a git repository the command refuses; it never falls back to a filesystem walk.** A
    walk is not a degradation, it is a different and broader check wearing the same name: it would
    sweep vendored code, build output, and untracked scratch files. ADR-0039's degraders fall back
    to **less** information and a static answer that is still true; there is no static answer to a
    prose scan. A gate that cannot see its input set must refuse rather than pass, for the same
    reason a mis-anchored walk must fail loudly rather than pass vacuously.

12. **A bare hyphen is a legitimate replacement.** This partially supersedes **ADR-0118 Decision
    item 9**'s "A bare hyphen is never substituted for an em-dash". A hyphen is ASCII and was never
    within the ban; item 9 raised a style preference to a prohibition, and commit `8338840` had
    already swept the Go comments using one. The prohibition is withdrawn rather than enforced
    retroactively. This costs nothing already done: the ADR-0118 sweep used no hyphens, so no
    swept file changes.

13. **The shipped documentation standard names the hyphen.** `templates/docs/doc-standard.md.tmpl`
    enumerates "a colon, semicolon, comma, or parentheses where an em-dash would go" and omits the
    hyphen. The enumeration gains it, so that the rule an adopter renders states what item 12
    decides. ADR-0117 item 8 already widened that line's scope to "shipped or authored", so scope is
    not touched here; only the replacement list is.

14. **This overrides ADR-0117 Decision item 5's "never fails an adopter's build over house style".**
    The override is named rather than glossed, because item 5 is an absolute and this ADR ships a
    check that fails a build over house style.

    What is preserved is item 5's substance: awf never **imposes** a house style. The knob defaults
    false, so awf's posture toward an adopter who has not asked is unchanged, and an adopter who
    turns it on has chosen it exactly as item 4 contemplates when it says "an adopter who disagrees
    sets it to `false`". What changes is the letter: item 5 assumed the only severity awf would ever
    offer for prose was a warning, and offers one more.

    ADR-0117 item 6's reasoning is unaffected. It made advisory severity the depiction escape hatch
    for its own rule, which has no exemption surface and needs none, because a warning an author
    ignores costs nothing.

15. **The two rules diverge in scope and severity, deliberately.** After this ADR an adopter with
    the knob on runs two checks over the same subject, and they disagree in two ways that are stated
    rather than reconciled.

    ADR-0117's rule skips generated paths; `prose-gate` scans them, because "sidecar data is NOT out
    of scope" and `docs/pitfalls.md` is where a sidecar's occurrences become visible. ADR-0117's
    rule honours no exemptions; `prose-gate` does. So an exempted depiction still warns from the
    audit rule on the commit that adds it. That is one warning, once, on a net increase, and never
    again: it is item 6's escape hatch working as designed, not a defect. Unifying the two would
    mean giving the advisory rule an exemption surface item 6 deliberately refused.

## Invariants

- `` `invariant: prose-gate-tracked-file-scan` ``: with `proseGate` enabled, `awf prose-gate`
  reports every banned codepoint in every tracked text file outside the configured exemptions and
  exits non-zero; it exits zero when the tree is clean, when every occurrence is exempted, and when
  the knob is false. An exemption with a pinned count fails when the file's count for that codepoint
  differs; an exemption without one permits any count. Backed by a test.

- `` `unbacked-invariant: prose-gate-refuses-without-git` ``: outside a git repository, or where
  the tracked-file set cannot be enumerated, `awf prose-gate` refuses with a diagnostic and never
  degrades to a filesystem walk. **Verify:** run `awf prose-gate` in a directory that is not a git
  repository, and in a tree built by `git checkout-index --prefix` (which has no `.git`); both must
  refuse rather than report findings.

The property "awf's own tree contains no banned codepoint" is deliberately **not** declared as a
separate invariant. It is what the first invariant checks when run in this repository, and
declaring it twice would put the same property in the ledger under two slugs.

## Consequences

**The surface list stops being a maintenance obligation.** A new sidecar, doc, or top-level file is
checked the moment it is tracked. This is the defect ADR-0113 had, that ADR-0115 named and then
reproduced, and that ADR-0118 item 10 handed forward; it does not recur.

**The exemption list is a new trust boundary, and awf's own is the only thing standing in front of
the shipped surfaces.** An exemption naming a path under `templates/` would ship a banned codepoint.
ADR-0115's gate catches exactly that, which is why item 7 keeps it. A reviewer reading a new
exemption entry should treat a shipped path as a defect.

**Two invariants now cover overlapping ground and a reader must be able to tell them apart.**
`emitted-prose-no-typographic-substitutes` is a promise about the product, checked unconditionally;
`prose-gate-tracked-file-scan` is a property of a repository, checked when a knob says so. The
distinction is real but it is subtle, and it is the main cost of item 7.

**"No new additions" is enforced by a bash line that no check reads, exactly as every other gate
is.** `./x` is hand-maintained and nothing tests the repository's own copy: deleting the coverage
arm would silently retire ADR-0012's gate, and the same is true of ADR-0063's and of this one. CI
runs the same unprotected line. This ADR does not claim to close its requirement more tightly than
the coverage or dead-code gates close theirs. Protecting gate wiring is a real gap, it is
pre-existing and general, and it belongs to its own ADR rather than to this one.

**Adopters gain a blocking option and lose nothing by default.** The knob defaults false and the
advisory rule is untouched, so an adopter who upgrades and does nothing sees no change.

**The sweep touches 356 sites across 51 files, and its risk is the risk ADR-0118 already
managed.** The word-stream proof reduces the failure mode from lost content to awkward prose.
Three sites need care beyond the mechanical rule, and the plan names them: `cmd/awf/context_test.go`
holds a fixture and an assertion that must move together, because the assertion captures the
fixture's rest-of-line through the command's output; `README.md`'s box diagram needs re-padding,
because three periods occupy three columns where the ellipsis occupied one, on two alignment axes;
and `internal/project/residue_scan_test.go` carries occurrences of its own, so the file that hosts
the existing exemption pin currently violates the rule it enforces.

**`internal/project/spine_test.go` loses an incidental demonstration.** Its fixtures feed
adopter-shaped sidecar data containing em-dashes through the renderer, and the golden output shows
awf passing them through unrewritten, which is ADR-0115 item 6's behaviour. Sweeping the fixtures
removes the only place that behaviour is visible. It was never asserted, so nothing fails; it is
recorded here so the loss is deliberate.

**New adopter-facing surface carries the usual obligations.** A command means a `clispec` entry, a
gating classification of `Gated` (it requires an adopted tree and has no static fallback), and an
update to the hardcoded gated-command list in `internal/clispec`, which AGENTS.md renders. A config
key means four `configspec` entries under the slice-of-struct convention, a description that cites
no ADR, a schema migration, and a regenerated `docs/config-reference.md`. The changelog entry is
scanned by ADR-0115's gate, so it must describe the ban without typing what it bans.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| A fourth enumerated surface list | The shape that decayed three times; each new surface is born unchecked. |
| Retire `emitted-prose-no-typographic-substitutes` under ADR-0031 | Sanctioned, and ADR-0115's own tail names `retires_invariants` as the mechanism for a slug whose ADR stays Implemented. Rejected because the successor reads a knob and an exemption list, so it cannot make an unconditional promise about shipped prose. |
| Make the shipped command refuse exemptions under `templates/` | Restores subsumption and would justify retirement, but hardcodes awf's own repo layout into an adopter-facing binary: the residue class ADR-0082 exists to ban. |
| Extend `awf commit-gate` to scan the tree | ADR-0036 item 1 defines it as reading one commit message from a `commit-msg` hook, which never sees the worktree. Would make one command mean two unrelated things. |
| Escalate ADR-0117's rule to Error severity | Smallest new surface, but the rule is net-increase over a commit range and cannot express presence over a tree. Wrong property. |
| Ship the gate enabled by default | Fails every existing adopter's build on upgrade over house style, which is the outcome ADR-0117's net-increase trigger exists to avoid. |
| A repo-local checker under `cmd/` | Correct while the check was awf's alone; the maintainer's requirement is that adopters can opt in, and a repo-local checker cannot ship. |
| Exempt `docs/research/` by designation | Cheaper than sweeping 186 occurrences, but makes the largest concentration in the tree permanent and turns the exemption list from three frozen instances into a directory. |
| A separate `.awf/prose-exemptions.yaml` | Unclaimed drift under ADR-0086's claimed-path model from the day it is written. |
| Fall back to a filesystem walk outside git | Silently substitutes a broader check: vendored code, build output, untracked files. A different check wearing the same name. |
