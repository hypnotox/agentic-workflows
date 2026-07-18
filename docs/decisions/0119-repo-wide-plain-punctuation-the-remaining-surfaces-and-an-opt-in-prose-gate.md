---
status: Implemented
date: 2026-07-16
tags: [template-residue, doc-standard]
related: [8, 36, 39, 41, 48, 82, 86, 88, 94, 113, 115, 116, 117, 118]
domains: [invariants, rendering, tooling, config]
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

The arithmetic of what follows: 388 measured, 355 swept, 23 fixed by re-rendering cleaned sidecars,
and 10 exempt across four entries.

### The chain's own measurement fell to the defect this ADR fixes

ADR-0118 item 10 hands this ADR "production Go comments (13 occurrences, all ellipses)". The count
is 17 and they are not all ellipses: `changelog/embed.go:3` carries an em-dash.

The interesting part is why, because it is not staleness. That em-dash was introduced by commit
`5c08df2` on 2026-07-01, two weeks before ADR-0118 measured. It was never in the parent effort's
churn. It was missed because the measurer inherited an enumerated scope: ADR-0115 item 3 defines
awf's Go surface as `internal/` and `cmd/`, commit `8338840` swept those two, and item 10 counted
them. There are three packages. `changelog/` is a top-level package for the same reason `templates/`
is, because `go:embed` cannot reach outside its own directory (ADR-0041), and an enumeration naming
two of three saw two of three.

So the defect argued against below is not hypothetical, and its best evidence is not an external
example: it is an occurrence inside this chain's own measurement, produced by the chain's own
enumerated scope, at the exact spot where ADR-0115's Context had already faulted ADR-0113 for the
same shape.

### Two premises verified rather than assumed

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
reproduce the defect a fourth time, and the section above shows it reproducing a third.

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
   list is what this ADR removes.

   This supersedes nothing. ADR-0115 item 3's union of three surfaces remains exactly correct for
   the check it governs, which item 7 below keeps intact; this item defines the scope of a
   different check rather than overriding item 3's.

2. **The remaining 355 occurrences are swept**, across the six live surfaces measured above:
   `docs/research/`, the test fixtures, the three `.awf/` sidecar sources, the production Go
   comments, the repo infrastructure, and `README.md`. The 23 remaining occurrences in generated
   docs are not touched: they are fixed by cleaning their `.awf/` sources and re-rendering, because
   hand-editing a rendered file is forbidden outright.

   ADR-0118's method carries over unchanged and is the condition of the sweep, not a
   recommendation: replacement is by sentence structure (ADR-0118 item 9, as amended by item 12
   below), and a word-stream comparison must report zero delta across every swept file, proving
   that only punctuation moved.

3. **`docs/research/` is swept.** This partially supersedes **ADR-0118 Decision item 1**
   (`refines: ADR-0118#1`), which
   excluded it as "not part of the decision corpus agents are steered by, and no approval covers
   it". The second clause was the operative one, and this ADR is that approval. The corpus is safe
   to sweep on a ground the earlier exclusion did not consider: it carries zero curly quotes and no
   external quotation, so nothing in it is a verbatim quote a sweep could corrupt. Its occurrences
   are awf's own headings and callouts.

4. **Go comments and `_test.go` files are in scope.** This partially supersedes **ADR-0115 Decision
   item 4** (`refines: ADR-0115#4`), whose two reasons are both answered: the gofmt reason is refuted above, and "test
   failure strings are not emitted prose" is true but irrelevant to a rule whose subject is now the
   repository rather than the shipped artifact. All seven codepoints are scanned in comments,
   including the curly quotes, because the single-backtick fix makes that loop winnable.

5. **Exemptions are configured, keyed by path and codepoint, and live in `config.yaml`.** The keys
   are `proseGate.exemptions` (a list of `{path, codepoint, count}` mappings, default empty), each
   entry naming a path, a codepoint, and optionally a pinned count. They are namespaced under
   `proseGate` because every root key in the config tree is structural and every behavioural knob is
   namespaced, as `audit.*`, `invariants.*`, `bootstrap.enabled` and `hooks.enabled` all are.

   **This partially supersedes ADR-0115 Decision item 7** (`refines: ADR-0115#7`) on all
   three of its clauses. Item 7 holds
   that "depiction is handled by convention and scope rather than an exemption list", that the
   `.awf/docs/pitfalls.yaml` curly quote "stays legal because sidecar data is out of scope", and
   that "No exemption list ships". Item 1 above brings sidecar data into scope and so removes the
   scope that made room for the depiction; this item supplies the exemption list item 7 refused;
   and item 8's command ships it. The reasoning of item 7 is not thereby wrong: it is answered in
   Context, and it remains correct for ADR-0115's own gate, which keeps its no-exemption posture
   under item 7 below.

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

6. **awf's own exemptions are three judgements across four entries, all pinned.**
   `docs/decisions/0113-em-dash-free-shipped-templates.md` (em-dash, 7), by ADR-0118 item 2's
   maintainer designation.
   `docs/plans/2026-07-13-invariant-backing-migration-to-enforced-test-scoped-backing.md` (em-dash,
   1), by ADR-0118 item 3. And the `.awf/docs/pitfalls.yaml` entry that types a curly quote to
   document gofmt's rewrite (left double quote, 1), which is **two** entries rather than one: the
   glyph exists both in the sidecar source and in its `docs/pitfalls.md` render target, and an
   entry names a path, so both paths are named. Three judgements, four entries; an implementer who
   writes three leaves `docs/pitfalls.md` failing the gate.

   ADR-0118 item 3's justification for the second is not re-asserted. It reasoned that the plan
   reports a live-state column that "shows" an em-dash, so punctuating it would make a true report
   false. That is no longer true of the tree: `docs/config-reference.md` now shows a count, not an
   em-dash. The exemption stands on the narrower and durable ground that the plan is a frozen record
   of what was observed when it was written, and the em-dash is the token that was observed.

7. **ADR-0115's gate and its `emitted-prose-no-typographic-substitutes` invariant are kept, not
   retired.** The new gate does not subsume them, and the reason is modal rather than mechanical:
   **ADR-0115's gate reads no configuration.** It walks two embedded filesystems and parses Go
   ASTs, and its answer cannot be changed by a config edit. The gate of item 8 reads a knob that
   defaults to false and an exemption list that any adopter, including awf, can extend. A promise
   about what awf **ships** must not be contingent on a knob, and an exemption naming a path under
   `templates/` would ship a banned codepoint that only ADR-0115's gate would catch.

   The two are mechanically redundant today and semantically distinct: one declares a property of
   the product, the other a property of the repository. They coincide only as long as no exemption
   ever names a shipped path, which is exactly the condition ADR-0115's gate exists to enforce.
   Retirement under ADR-0031 was considered and is recorded in Alternatives.

8. **A new shipped command, `awf prose-gate`.** It scans the tracked text files of the tree it is
   run in, reports every banned codepoint outside the configured exemptions, and exits non-zero on
   any finding. It is the blocking, presence-level check.

   The name mirrors `commit-gate`, the only other hyphenated command and the only other
   adopter-facing blocking check, because this is the same species. It is deliberately not
   `prose-check`: `awf check` is the drift oracle, a name AGENTS.md elevates to an invariant, and a
   sibling `-check` would read as a variant of a command with which it shares no machinery, no
   config, and no severity.

   The analogy holds at both ends. **`templates/hooks/pre-commit.sh.tmpl` gains an unconditional
   `awf prose-gate` line**, exactly as `templates/hooks/commit-msg.sh.tmpl` already ends in
   `awf commit-gate "$1"`, through a new `proseGateCmd` var on the same `with`/`else` idiom, so the
   line renders `awf prose-gate` when the var is unset and degrades to no no-value token either way
   (ADR-0001, ADR-0045). ADR-0032 is not contradicted: it removed hook **activation**, and ADR-0048
   renders an inert payload the adopter wires into a hook they own. Wiring the line into that
   payload is not activation.

   **The payload's bootstrap shim guard widens with it, and this is a condition of the line rather
   than a detail.** That template emits its `awf()` bootstrap-pinning shim only under
   `{{ if not .vars.checkCmd }}`, and its `gateCmd` line carries no `{{ else }}`, so today it can
   never render a bare unshimmed `awf`. A second line that falls back to `awf prose-gate` breaks
   that property for any adopter who sets `checkCmd` and not `proseGateCmd`: they would render one
   bare call that bypasses bootstrap pinning. The guard therefore becomes a disjunction over every
   var that can render a bare `awf` (`or (not .vars.checkCmd) (not .vars.proseGateCmd)`), not a
   conjunction: the shim is needed when **any** call site lacks its var, not only when all do.

   awf sets `proseGateCmd` to a new `./x prose-gate` arm, because it sets `checkCmd` and builds from
   source with `bootstrap.enabled: false`, so a bare PATH `awf` is exactly what it must not call.
   The cost is that awf's own pre-commit scans twice, once through the payload's `./x gate` line
   (item 11) and once through this one. It is accepted and named rather than engineered around: a
   rendered payload cannot know what a project's runner already folds into its gate, and a second
   pass over a clean tree is cheap.

9. **The command self-gates on one knob, `proseGate.enabled` (bool, default `false`).** When it is
   false the command exits zero without scanning. Enforcement lives in the command rather than in
   the wiring, which is what lets item 8's payload line be unconditional: a rendered hook does not
   need to know whether the rule is on, and turning the rule on is one config edit rather than a
   config edit plus a wiring edit.

   The default is false, alone among awf's advisory knobs, because this one blocks. ADR-0117's
   `audit.plainPunctuation` defaults true and may: a warning costs an adopter nothing. Presence-level
   enforcement on an unswept tree costs them their build, so they must ask for it.

10. **The command is `Ungated`.** It joins `commit-gate` in ADR-0094's ungated set rather than the
    gated one, and for the same reason `commit-gate` is there: a check wired into a commit hook
    (item 8) should not be the place ADR-0039's binary-version gate speaks, because refusing a
    commit over version skew is a refusal unrelated to the adopter's prose. This diverges from
    `check`, `invariants` and `audit`, which are `Gated` and are not hook-wired.

    **`Ungated` does not make the command skew-proof, and no claim is made that it does.** A stale
    binary meeting a newer tree still fails, by two paths this classification does not touch:
    `internal/config` decodes with `KnownFields(true)`, so a binary that does not know
    `proseGate` errors on the unknown key rather than leaving the rule off; and a binary old enough
    to lack the knob lacks the subcommand too, so the payload's line fails whatever its `Gating`
    says. The honest statement is narrower: the strict decoder already fails loudly on a newer tree,
    so ADR-0039's gate adds no protection this command lacks, while it would add a second refusal
    mode. `Ungated` removes the redundant one. The command still requires an adopted tree and still
    refuses without one, on its own terms rather than through the version gate.

11. **awf enables the knob for itself and wires the command into its own gate.** awf is the first
    adopter of what it ships, so it does not ship an opt-in it declines to take.

12. **Outside a git repository the command refuses; it never falls back to a filesystem walk.** A
    walk is not a degradation, it is a different and broader check wearing the same name: it would
    sweep vendored code, build output, and untracked scratch files. ADR-0039's degraders fall back
    to **less** information and a static answer that is still true; there is no static answer to a
    prose scan. A gate that cannot see its input set must refuse rather than pass, for the same
    reason a mis-anchored walk must fail loudly rather than pass vacuously.

13. **A bare hyphen is a legitimate replacement.** This partially supersedes **ADR-0118 Decision
    item 9**'s (`refines: ADR-0118#9`) "A bare hyphen is never substituted for an em-dash". A hyphen is ASCII and was never
    within the ban; item 9 raised a style preference to a prohibition, and commit `8338840` had
    already swept the Go comments using one. The prohibition is withdrawn rather than enforced
    retroactively. This costs nothing already done: the ADR-0118 sweep used no hyphens, so no
    swept file changes.

14. **The shipped documentation standard names the hyphen.** `templates/docs/doc-standard.md.tmpl`
    enumerates "a colon, semicolon, comma, or parentheses where an em-dash would go" and omits the
    hyphen. The enumeration gains it, so that the rule an adopter renders states what item 13
    decides. ADR-0117 item 8 already widened that line's scope to "shipped or authored", so scope is
    not touched here; only the replacement list is.

15. **This overrides ADR-0117 Decision item 5's "never fails an adopter's build over house
    style".** (`refines: ADR-0117#5`)
    The override is named rather than glossed, because item 5 is an absolute and this ADR ships a
    check that fails a build over house style.

    What is preserved is item 5's substance: awf never **imposes** a house style. The knob defaults
    false, so awf's posture toward an adopter who has not asked is unchanged, and an adopter who
    turns it on has chosen it exactly as item 4 contemplates when it says "an adopter who disagrees
    sets it to `false`". Note the asymmetry that makes this concrete: ADR-0117's own knob defaults
    **true**, so on the axis item 5 protects, this gate is the less imposing of the two. What
    changes is the letter: item 5 assumed the only severity awf would ever offer for prose was a
    warning, and this offers one more.

    ADR-0117 item 6's reasoning is unaffected. It made advisory severity the depiction escape hatch
    for its own rule, which has no exemption surface and needs none, because a warning an author
    ignores costs nothing.

16. **The two rules diverge in scope and severity, deliberately.** After this ADR an adopter with
    the knob on runs two checks over the same subject, and they disagree in two ways that are stated
    rather than reconciled.

    ADR-0117's rule skips generated paths; `prose-gate` scans them, because "sidecar data is NOT out
    of scope" and `docs/pitfalls.md` is where a sidecar's occurrences become visible. ADR-0117's
    rule honours no exemptions; `prose-gate` does. So an exempted depiction still warns from the
    audit rule on the commit that adds it. That is one warning, once, on a net increase, and never
    again: it is item 6's escape hatch working as designed, not a defect. Unifying the two would
    mean giving the advisory rule an exemption surface item 6 deliberately refused.

## Invariants

- `` `invariant: prose-gate-tracked-file-scan` ``: with `proseGate.enabled` true, `awf prose-gate`
  reports every banned codepoint in every tracked text file outside `proseGate.exemptions` and exits
  non-zero; it exits zero when the tree is clean, when every occurrence is exempted, and when the
  knob is false. An exemption with a pinned `count` fails when the file's count for that codepoint
  differs; an exemption without one permits any count. Backed by a test.

- `` `invariant: prose-gate-refuses-without-git` ``: with `proseGate.enabled` true, outside a git
  repository, or wherever the tracked-file set cannot be enumerated, `awf prose-gate` refuses with a
  diagnostic and exits non-zero rather than degrading to a filesystem walk or reporting a clean tree.
  Backed by a test.

  The knob qualifier is load-bearing rather than throat-clearing, and it is the same one the
  invariant above carries. Item 9 has the command check the knob **before** it enumerates, which is
  what lets a hook invoke it unconditionally; so with the knob false and no git, the command exits
  zero and refuses nothing. Stated unqualified, this invariant would be false of the implementation
  item 9 specifies, and the ledger would carry a claim it cannot honour (ADR-0114).

The property "awf's own tree contains no banned codepoint" is deliberately **not** declared. It is
not what the invariants above check: they are backed by tests of the command's behaviour against
fixtures, whereas the property holds only because `./x gate` invokes the command against this tree.
That wiring is a line in a hand-maintained bash file no check reads, which is exactly the position
ADR-0012's coverage gate and ADR-0063's dead-code gate are already in. Declaring a property whose
enforcement nothing verifies would put a claim in the ledger that the ledger cannot honour
(ADR-0114). The gap is real, pre-existing, and general; it is named in Consequences and belongs to
its own ADR.

## Consequences

**The surface list stops being a maintenance obligation.** A new sidecar, doc, or top-level file is
checked the moment it is tracked. This is the defect ADR-0113 had, that ADR-0115 named and then
reproduced, that silently cost ADR-0118 item 10 an em-dash in a third package, and that ADR-0118
handed forward; it does not recur.

**The exemption list is a new trust boundary, and awf's own is the only thing standing in front of
the shipped surfaces.** An exemption naming a path under `templates/` would ship a banned codepoint.
ADR-0115's gate catches exactly that, which is why item 7 keeps it. A reviewer reading a new
exemption entry should treat a shipped path as a defect.

**Two invariants now cover overlapping ground and a reader must be able to tell them apart.**
`emitted-prose-no-typographic-substitutes` is a promise about the product, checked unconditionally;
`prose-gate-tracked-file-scan` is a property of a command, checked against fixtures. The distinction
is real but it is subtle, and it is the main cost of item 7.

**"No new additions" rests on a bash line that no check reads, exactly as every other gate does.**
`./x` is hand-maintained and nothing tests the repository's own copy: deleting the coverage arm
would silently retire ADR-0012's gate, and the same is true of ADR-0063's and of this one. CI runs
the same unprotected line. This ADR does not claim to close its requirement more tightly than the
coverage or dead-code gates close theirs. Protecting gate wiring is a real gap, it is pre-existing
and general, and it belongs to its own ADR rather than to this one.

**Adopters who upgrade see one change, and it is a re-render.** The knob defaults false and the
advisory rule is untouched, so no adopter's build starts failing. But item 8 adds a line to the
rendered `pre-commit` payload, so every adopter's `.awf/hooks/pre-commit.sh` changes and their
`awf check` reports drift until they re-sync. That is the same upgrade cost ADR-0115 recorded for a
template change, it is the price of the command being wired the way `commit-gate` is, and the
changelog entry must say so.

**The sweep touches 355 sites across 51 files, and its risk is the risk ADR-0118 already
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

**The agent guide's own invariant bullet becomes false and must move with this ADR.** AGENTS.md
tells the Go author that "Go comments and tests are out of scope"; items 1, 4 and 11 make that
false for this repository. The bullet is rendered from `.awf/agents-doc.yaml`, so the flip commit
edits the sidecar and re-renders. ADR-0115 item 10 spent a scarce guide slot arguing that this
bullet is where the Go author actually reads the rule; leaving it stating an exclusion the gate
rejects would recreate precisely the ADR-0113 item 4 defect of warning against an unstated rule.

**New adopter-facing surface carries the usual obligations, traced against the ADR-0117
precedent.** That rule's commit (`4889c0f`) is the template: a field on the `internal/config`
struct, its resolved default, `configspec` entries, and a live-state case in
`internal/project/configreference.go`. Here that means five `configspec` entries rather than four,
because item 9 adds `proseGate.enabled` alongside the list key and its three struct fields; a
description that cites no ADR and names neither the repository nor its owner, which the description
residue guard enforces; a `clispec` entry; a new `proseGateCmd` var and the widened shim guard of
item 8; a regenerated `docs/config-reference.md` and its `examples/sundial` counterpart, plus both
lock files; a regenerated `ACTIVE.md` on the status flip; and a changelog entry, which ADR-0115's
gate scans, so it must describe the ban without typing what it bans.

Two things that look owed and are not, stated so an implementer does not manufacture work.
`GatedCommandNames()` filters on `Gating != Ungated`, so an `Ungated` command changes neither the
derived list nor the `want` slice in `internal/clispec/clispec_test.go` nor the AGENTS.md bullet
that renders from it: an implementer who adds `prose-gate` to any of the three has mis-classified
the command under item 10. And **no schema migration is owed**: `4889c0f` added an optional key
with a default and touched no `internal/migrate` file, and bumping the schema generation would
force every adopter through `awf upgrade` for nothing (ADR-0039).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| A fourth enumerated surface list | The shape that decayed three times, most recently inside this chain's own measurement. |
| Retire `emitted-prose-no-typographic-substitutes` under ADR-0031 | Sanctioned, and ADR-0115's own tail names `retires_invariants` as the mechanism for a slug whose ADR stays Implemented. Rejected because the successor reads a knob and an adopter-extensible exemption list, so it cannot make an unconditional promise about shipped prose. |
| Make the shipped command refuse exemptions under `templates/` | Restores subsumption and would justify retirement, but hardcodes awf's own repo layout into an adopter-facing binary: the residue class ADR-0082 exists to ban. |
| Extend `awf commit-gate` to scan the tree | ADR-0036 item 1 defines it as reading one commit message from a `commit-msg` hook, which never sees the worktree. Would make one command mean two unrelated things. |
| Escalate ADR-0117's rule to Error severity | Smallest new surface, but the rule is net-increase over a commit range and cannot express presence over a tree. Wrong property. |
| Ship the gate enabled by default | Fails every existing adopter's build on upgrade over house style, which is the outcome ADR-0117's net-increase trigger exists to avoid. |
| Classify the command `Gated` | Matches `check`/`invariants`/`audit`, but a hook-wired check that refuses on binary-version skew blocks an adopter's commit for a reason unrelated to their prose. `commit-gate` is `Ungated` for this reason. |
| Leave the rendered pre-commit payload unwired | Zero adopter churn, but breaks the `commit-gate` analogy the command's name and shape rest on, and leaves the opt-in requiring two edits rather than one. |
| A repo-local checker under `cmd/` | Correct while the check was awf's alone; the maintainer's requirement is that adopters can opt in, and a repo-local checker cannot ship. |
| Exempt `docs/research/` by designation | Cheaper than sweeping 186 occurrences, but makes the largest concentration in the tree permanent and turns the exemption list from three frozen judgements into a directory. |
| A separate `.awf/prose-exemptions.yaml` | Unclaimed drift under ADR-0086's claimed-path model from the day it is written. |
| Fall back to a filesystem walk outside git | Silently substitutes a broader check: vendored code, build output, untracked files. A different check wearing the same name. |
