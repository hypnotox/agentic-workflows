---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0199: Proof markers name the unit that proves them

## Context

A `Backing: test` claim is satisfied by a proof `invariant: <domain>/<topic>:<slug>` comment on a
test in a `currentState.testGlobs` file. Building the marker index never associates that comment
with anything. `scanMarkerBytes` in `internal/topic/markers.go` splits a source file into lines
and resolves any line whose prefix matches a configured marker token; `finalizeMarkerIndex` then
asks one question per claim, whether the proof count is zero. A proof marker's only constraints
are that it targets a test-backed invariant and that its *file* matches `currentState.testGlobs`.

A marker therefore keeps satisfying `Backing: test` after the test it was proving is deleted,
renamed, or moved. The claim becomes proven by nothing and the gate stays green.

This inverts the symmetry ADR-0134 established. `Backing: test` is the strong form and
`Backing: unbacked` is the reasoned-contract form that must carry a `Verify:` line. A stranded
marker gives a claim the appearance of the strong form with less real assurance than the weak
one, because an unbacked claim is at least honest about being a contract.

Commit 4c61356a ("feat(rendering): simplify Pi effort workflows (implements 0164)") deleted three
marker-bearing tests and left four proof markers attached to nothing. Two of them, in
`internal/project/example_wiring_test.go`, stayed in place after `TestPiExtensionContainerGateWiring`
and `TestPiExtensionEditorQuietStrip` were deleted; ADR-0198 has since repaired both. The other
two are still live: the same commit deleted `TestWorkflowTelemetryConfigContract` from
`internal/config/config_test.go` and added markers for
`config/configuration:config-serialization-owned` and
`config/migrations-and-locks:migration-ordering` at the end of that file, where they sit below the
final closing brace with no test beneath them. Those two were detached from the moment they were
written. ADR-0164's `State changes` never touched either claim, so this was an unremediated
regression from an unrelated refactor rather than a sanctioned retirement. `docs/roadmap.md`
carries the deferred record of the general problem, naming only the two `example_wiring_test.go`
strandings; this decision resolves it and corrects that count.

Two facts constrain the fix.

First, awf publishes a language-agnostic standard. Marker syntax is configured per source family
by comment tokens in `currentState.sources`, and adopters are not assumed to be writing Go. A
checker that must parse a language to enforce backing is not shippable as part of that standard.

Second, a census of all 538 proof markers in this repository, classified by the next non-blank
non-comment line after each marker, found 425 directly above a top-level test function, 96 inside
a test body, 15 above a non-test helper function, and 2 at end of file with nothing following.
The 96 in-body markers are deliberate: they sit on the assertion that proves the claim rather
than above the enclosing function, a pattern ADR-0131 introduced. So 111 of 538 markers
legitimately do not sit above a test function, and any rule that associates a marker with a
following declaration rejects a fifth of the corpus however precisely it is computed. Such a rule
would also have missed the `example_wiring_test.go` case, where a stranded marker had a live but
unrelated test directly beneath it.

Position is therefore close to uninformative here. What has to be detected is that the thing the
marker was written to point at is gone, which is a naming problem rather than a parsing problem.

Markers also stack. `internal/contextq/adapter_outputs_test.go` carries twelve consecutive proof
markers above a single test function, and 76 consecutive-marker pairs exist across the corpus.
Any rule that searches a file for a marker's name has to account for the other markers in that
file, or a stack becomes self-satisfying.

## Decision

1. The proof marker payload gains a required trailing name in parentheses:
   `invariant: <domain>/<topic>:<slug> (<name>)`. The name identifies the unit that proves the
   claim. The name capture is scoped to the `invariant:` alternative of `markerPayloadRE` in
   `internal/topic/markers.go`, not to the shared alternation, and a proof marker parsed without
   a name is an error.

2. The name is free text, not an identifier. Its only requirements are that it is non-empty,
   carries no leading or trailing whitespace, and does not contain the source family's closing
   token. This is what keeps the rule portable: a Go or Python adopter names a function, while a
   JavaScript or TypeScript adopter's test is a string literal such as `it('strips the header')`
   and has no identifier to name.

3. The check is a string search over the marker's own file. The named text must occur verbatim on
   at least one line of that file that the scan does not recognise as a current-state marker, and
   the occurrence must not be flanked on either side by a character in `[A-Za-z0-9_]`.

   Excluding every marker line, rather than only the marker's own line, is what stops a stack from
   satisfying itself. Twelve markers above one function all name that function, so under a
   self-line-only exclusion each is satisfied by its eleven neighbours and deleting the function
   strands twelve claims silently. The flanking condition is load-bearing in both directions:
   without it a marker naming `TestFoo` would be satisfied by a surviving `TestFooBar`, missing
   exactly the rename this decision exists to catch, while with it a free-text phrase flanked by
   quotes or parentheses still matches, because neither is an identifier character.

4. `state:` and `touches-state:` markers are unchanged, and neither gains an error of its own.
   Because the name capture is scoped to the `invariant:` alternative, a `state:` marker carrying
   a name fails the payload match and falls through to the existing
   `malformed current-state marker %q` error. A `touches-state:` marker needs no rule at all: its
   ` - <note>` grammar absorbs a trailing parenthetical into the note, so a name is
   indistinguishable from note text.

5. The check lives in `scanMarkerBytes`, which is the byte-fed scan core shared by the filesystem
   walker and the snapshot loader. Both entry paths inherit one implementation.

6. Two errors are added, each carrying the `<path>:<line>:` prefix the surrounding scan errors
   already use. One reports a proof marker with no name. The other reports a named unit that does
   not occur in the file, and names deletion, renaming, and moving as the causes, because the
   marker's own text cannot distinguish them.

7. This is a breaking change to marker syntax, shipped unconditionally with no opt-in gate and no
   per-adopter configuration. Existing markers become invalid and adopters migrate.

8. It requires no schema generation bump and no `awf upgrade` migration. Schema generation tracks
   the shape of `.awf/config.yaml` and the tree it renders, not marker semantics inside adopter
   source files. ADR-0105 is direct precedent: it renamed the marker token unconditionally in this
   same subsystem, and its implementing commit changed no `schemaVersion`.

9. Migrating this repository's 538 markers is done by a throwaway repo-local script that is
   neither committed nor shipped. It may use whatever language knowledge is convenient, including
   the Go AST, to derive each name. Language knowledge is admissible once, to migrate, and never
   to enforce. That separation is what makes the rule adoptable by a project awf cannot parse.

10. The migration writes the nearest unique anchor available, falling back to the enclosing or
    following function's identifier when no finer one exists. Where a marker sits above a single
    table row, the row's own literal is the better name, because deleting that row then fails the
    check while naming the enclosing function would not. The written string persists in the marker
    and is what the check verifies from then on, so a marker name means "the text that identifies
    what proves this claim" rather than uniformly "a function identifier".

11. The two live stranded claims get real tests written rather than being retired to
    `Backing: unbacked`. Retirement would use the reasoned-contract escape hatch to avoid the work
    this check exists to expose. These two are repaired rather than migrated because item 10 has
    no answer for them: a marker below the final closing brace has neither an enclosing nor a
    following function, so there is no anchor to write.

12. The checker change, the corpus migration, and the two replacement tests land in one commit.
    Separated, the gate is red in between.

13. The claim this decision adds is an invariant with `Backing: test`, proven by the new scan
    tests. It applies to itself: its own proof marker carries a name under the rule it
    establishes.

## State changes

- add `invariants/topics-and-markers:proof-marker-names-its-unit`

## Consequences

The drift this decision targets becomes a scan failure rather than a silent green gate. Deleting,
renaming, or moving a test whose marker remains behind fails `awf check` at the marker's line.

Authoring cost rises in two places. Renaming or moving a test now means editing every marker that
names it, which is up to twelve markers in one stack in this repository. And because
`scanMarkerBytes` returns on its first error, a bulk migration or repair surfaces one failure per
run rather than a list. The scan keeps its abort-on-first-error model, because changing it is a
separate concern from this decision; the migration script is expected to produce correct names in
one pass, so the serial diagnostic is a cost paid during hand repair rather than during migration.

One false negative is accepted: a name that survives its unit incidentally, such as `TestFoo`
deleted while `t.Run("TestFoo")` remains elsewhere in the file. Closing it would require knowing
which occurrence is a declaration, which is the language knowledge this decision refuses to take
on. The residual requires the name to persist in prose after the unit dies, which no observed
stranding resembles.

More fundamentally, the check constrains the *form* of a marker, not the *discrimination* of its
name. Nothing requires the name to be specific, so a weak or over-general name that happens to
appear elsewhere in the file satisfies the check permanently and silently returns that claim to
its pre-decision state. The rule's real strength therefore rests on item 10's nearest-unique-anchor
migration and on authors and reviewers choosing discriminating names afterwards. Marker review
stays load-bearing after this check ships.

The check also verifies that a named unit exists, not that it exercises the claim. A marker naming
a live test that proves nothing relevant still passes. That is the separate nominal-proof problem
`docs/roadmap.md` tracks as a mutation-testing candidate, and this decision neither solves nor
worsens it.

Adopters with existing markers must migrate before the next `awf check` passes, and awf supplies
no migration tool for them, since deriving names requires knowing their language. The rule is
mechanical enough to script per language, and adoption of the marker corpus is currently thin.

Within this repository the migration touches 538 markers across the test corpus, and the two
stranded claims need tests written before the gate can pass.

Documentation that specifies the payload grammar changes with the code, in the same commit,
through the `.awf/` sources behind the agent guide, the invariants domain prose, the pitfalls and
glossary entries, and the code-reviewer agent, along with the roadmap entry this decision
resolves. It also changes `templates/skills/retrospective/SKILL.md.tmpl`, the shipped template
that teaches every adopter the marker grammar, which re-renders the retrospective skill for this
repository and for every target in `examples/sundial`.

Because this changes rendered template output and check behaviour for every adopter,
`changelog/CHANGELOG.md` gains an entry under `## [Unreleased]` / `### Breaking changes` in the
same commit, recording that existing proof markers become invalid, that no schema bump or
`awf upgrade` step signals the break, and that adopters migrate their markers themselves.

`invariants/topics-and-markers:invariant-marker-close-token` is unaffected and needs no operation.
The extractor strips one closing token from the whole payload before any component is parsed, so
the name is inside the stripped payload and the claim's enumeration of components is illustrative
rather than exhaustive.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Require content after the marker | Catches only the 2 end-of-file strandings. Would not have caught the `example_wiring_test.go` case, where a live but unrelated test sat directly beneath the marker. |
| Name the proving unit in the claim instead of the marker | Splits one relationship across two files and forces a join the marker can no longer answer alone. Claims with markers in several files need a rule for which file must carry the name, and claims with several proofs need a list. |
| Name the unit and enforce marker position against it | Needs to know what a declaration is, which puts language knowledge in the shipped checker. The census shows 111 of 538 markers legitimately violate any positional rule. |
| Ship the rule as an opt-in gate like `check prose` | Costs a config key and a second enforcement path, and leaves adopters unprotected by default, to avoid a migration that is mechanical and currently small. |
| Fingerprint the proving region and detect drift by hash | Fails on every incidental edit to the region, making it a false-positive source rather than a drift oracle. |
| Record the resolved name on `MarkerSite` for reuse by `awf topic` and `awf context` | The check runs inside `scanMarkerBytes` with the parsed name already in hand, so the field has no consumer at this decision. Adding it would widen the JSON those commands emit for anticipated rather than actual reuse. |
| Mutation testing | Detects the stronger nominal-proof case but is slow and advisory today. It is the tracked candidate for that separate problem, and it is far more expensive than a string search for this one. |

## Status history

- 2026-07-31: Proposed
