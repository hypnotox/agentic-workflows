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

Commit 4c61356a ("feat(rendering): simplify Pi effort workflows (implements 0164)") left four
proof markers attached to nothing across two files. Two of them, in
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
would also have missed the container-gate stranding in `example_wiring_test.go`, where the marker
had a live but unrelated test directly beneath it.

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
   `internal/topic/markers.go`, not to the shared alternation, and it extends to the payload's
   final closing parenthesis rather than the first. A proof marker parsed without a name is an
   error.

2. The name is free text, not an identifier. Its only requirements are that it is non-empty,
   carries no leading or trailing whitespace, and does not contain the source family's closing
   token; it may itself contain parentheses, which is why item 1's capture is greedy. This is
   what keeps the rule portable: a Go or Python adopter names a function, while a JavaScript or
   TypeScript adopter's test is a string literal such as `it('strips the header')` and has no
   identifier to name.

3. The check is a string search over the marker's own file. The named text must occur verbatim on
   at least one line of that file whose trimmed form does not begin with the source family's
   marker token, and the occurrence must not be flanked on either side by a rune that is a
   letter, a digit, or an underscore. Flanking is tested over runes rather than ASCII bytes
   because item 2 defines the name as free text: an adopter whose identifiers or test labels
   carry non-ASCII letters gets the same rename protection an ASCII one does.

   For a family whose marker token is its comment leader, `//` for Go and `#` for Python, this
   excludes comments, and every marker line is a special case of a comment line. One condition
   therefore replaces two, the marker's own line needing no separate case, and it is purely
   syntactic and line-local: recognition tests a line's leading token, never whether that line
   resolves to a valid claim, so it introduces no error of its own and the scan keeps reporting
   the first failure in line order.

   The exclusion is exactly "opens with the marker token", which is narrower than "is a comment"
   for a family whose token is a prefixed or block-comment form. Such a family leaves an ordinary
   comment line searchable, so a name surviving only in prose could satisfy a stranded marker. A
   proof marker lives only in a `currentState.testGlobs` file, so the exposure is an adopter that
   configures a prefixed or block-comment token over its test file set; this repository's own
   prefixed family (`<!-- awf:comment`) covers config parts and templates, which no proof marker
   can inhabit. Widening the exclusion to general comment syntax would require per-language
   knowledge the check deliberately refuses (item 9), so this stays a documented boundary.

   The single exclusion subsumes two weaker ones, and measurement rather than intuition set the
   boundary. Excluding only the marker's own line lets a stack satisfy itself: twelve markers above one
   function in `internal/contextq/adapter_outputs_test.go` all name that function, so each is
   satisfied by its eleven neighbours and deleting the function strands twelve claims silently.
   Excluding marker lines alone is still not enough, because this repository's convention places a
   doc comment naming the test above the marker block. Simulating the deletion of every anchor
   function shows 120 of 425 markers above a test still finding their name in such a comment, a
   28% false-negative rate that includes that same twelve-marker stack. Excluding comments closes
   all 120 at no measured cost, since every anchor the migration writes is a code line.

   The flanking condition is load-bearing on both sides: without its trailing half a marker naming
   `TestFoo` would be satisfied by a surviving `TestFooBar`, and without its leading half by a
   surviving `XTestFoo`, missing exactly the rename this decision exists to catch. With it a
   free-text phrase flanked by quotes or parentheses still matches, because neither is an
   identifier character. A flanked hit does not abandon the line: the same name can occur twice on
   one line, once inside a longer identifier and once alone, as in a wrapper that calls the test,
   so the search continues past a rejected occurrence rather than reporting a live test deleted.

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

10. The migration writes the nearest unique anchor available, where unique means unique among the
    lines the check will search in that file, falling back to the enclosing or following
    function's identifier when no finer one exists. Item 3's comment exclusion is what makes that
    fallback well defined: a function identifier repeated in the file's own doc comment is unique
    within the searched lines even though it is not unique within the file. Where a marker sits
    above a single
    table row, the row's own literal is the better name, because deleting that row then fails the
    check while naming the enclosing function would not. The written string persists in the marker
    and is what the check verifies from then on, so a marker name means "the text that identifies
    what proves this claim" rather than uniformly "a function identifier".

11. The two live stranded claims get real tests written rather than being retired to
    `Backing: unbacked`. Retirement would use the reasoned-contract escape hatch to avoid the work
    this check exists to expose. These two are repaired rather than migrated because item 10 has
    no answer for them: a marker below the final closing brace has neither an enclosing nor a
    following function, so there is no anchor to write.

12. Every commit in the implementation sequence leaves the gate green. The binding constraint is
    that no commit may leave the corpus and the checker disagreeing, not that the work arrives in
    one commit. Landing the name capture before anything requires it satisfies that: the payload
    accepts a named marker while no check demands one, the corpus is migrated under the permissive
    schema, and enforcement follows once every marker already carries a name. The two stranded
    claims are repaired first, because item 11 leaves them nothing to migrate. Sequencing this way
    is preferred over a single transaction, which would combine a subtle checker change, a
    corpus-wide mechanical sweep, and a documentation rewrite in one unreviewable commit.

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

One false negative is accepted: a name that survives its unit incidentally on a searched line,
such as `TestFoo` deleted while a `t.Run("TestFoo")` string remains elsewhere in the file.
Closing it would require knowing which occurrence is a declaration, which is the language
knowledge this decision refuses to take on.

The much larger comment-borne form of that residual is closed rather than accepted for a family
whose marker token is its comment leader, and it was measured rather than assumed. Naming a test
in a doc comment above its marker block is this repository's dominant convention, so a rule that
searched comments would have missed 120 of 425 markers above a test, and the tree already contains
an instance: `internal/project/example_wiring_test.go`
carries a doc comment naming `TestSundialCurrentStateMigrated`, a test that no longer exists
anywhere. Item 3's exclusion catches that today. For such a family the remaining residual needs
the name to survive on a code line, which is a far narrower accident than surviving in prose; for
a family whose token is a prefixed or block-comment form, item 3's boundary paragraph records that
ordinary comments stay searchable and the prose-borne residual is accepted rather than closed.

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

Documentation that specifies the payload grammar changes in the commit that makes the name
required, which is the commit where the documented grammar actually changes,
through the `.awf/` sources behind the agent guide, the invariants domain prose, the pitfalls and
glossary entries, and the code-reviewer agent, along with the roadmap entry this decision
resolves. It also changes `templates/skills/retrospective/SKILL.md.tmpl`, the shipped template
that teaches every adopter the marker grammar, which re-renders the retrospective skill for this
repository and for every target in `examples/sundial`.

Because this changes rendered template output and check behaviour for every adopter,
`changelog/CHANGELOG.md` gains an entry under `## [Unreleased]` / `### Breaking changes` alongside
that same enforcement commit, recording that existing proof markers become invalid, that no schema bump or
`awf upgrade` step signals the break, and that adopters migrate their markers themselves.

`invariants/topics-and-markers:invariant-marker-close-token` is unaffected and needs no operation.
The extractor strips one closing token from the whole payload before any component is parsed, so
the name is inside the stripped payload and the claim's enumeration of components is illustrative
rather than exhaustive.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Require content after the marker | Catches only the 2 end-of-file strandings, and of the two `example_wiring_test.go` strandings it would have caught only the strip marker, which was that file's last line. The container-gate marker had a live but unrelated test directly beneath it. |
| Name the proving unit in the claim instead of the marker | Splits one relationship across two files and forces a join the marker can no longer answer alone. Claims with markers in several files need a rule for which file must carry the name, and claims with several proofs need a list. |
| Name the unit and enforce marker position against it | Needs to know what a declaration is, which puts language knowledge in the shipped checker. The census shows 111 of 538 markers legitimately violate any positional rule. |
| Ship the rule as an opt-in gate like `check prose` | Costs a config key and a second enforcement path, and leaves adopters unprotected by default, to avoid a migration that is mechanical and currently small. |
| Fingerprint the proving region and detect drift by hash | Fails on every incidental edit to the region, making it a false-positive source rather than a drift oracle. |
| Record the resolved name on `MarkerSite` for reuse by `awf topic` and `awf context` | The check runs inside `scanMarkerBytes` with the parsed name already in hand, so the field has no consumer at this decision. Adding it would widen the JSON those commands emit for anticipated rather than actual reuse. |
| Mutation testing | Detects the stronger nominal-proof case but is slow and advisory today. It is the tracked candidate for that separate problem, and it is far more expensive than a string search for this one. |

## Status history

- 2026-07-31: Proposed
