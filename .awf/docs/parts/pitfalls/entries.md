## `awf audit` and `extensions.worktreeConfig`

`git.PlainOpen` (go-git) refuses to open a repo whose `.git/config` has `extensions.worktreeConfig = true` — a flag `git worktree add` can leave behind even after the worktree is removed — regardless of `core.repositoryformatversion`. Cause: go-git's extension-support check lowercases the extension name before comparing it against its allow-list, whose key is mixed-case, so the lookup never matches. `internal/git`'s `OpenRepo` works around it by opening through a `storage.Storer` wrapper that hides the `[extensions]` config section from go-git before the check runs; awf's git-reading commands never read repo extensions, so hiding the section is safe. Any future awf code opening a repo must go through `internal/git.OpenRepo`, not `git.PlainOpen` directly (the go-git handling was extracted from `internal/audit` into the shared `internal/git` package so `awf audit` and `awf context` share one tolerant open path).

## Stdout is API in command-substitution scripts

`.awf/bootstrap.sh` is consumed as `"$(bash .awf/bootstrap.sh)" <args>`, so its stdout is the
binary path and nothing else — a checksum tool's `<asset>: OK` line on stdout execs as part of
the command and fails only on cache-miss runs, which presents as flaky CI. Every diagnostic in
rendered shell must carry `>&2`; `TestBootstrapStdoutPathOnly` pins this (`inv:
bootstrap-stdout-path-only`, ADR-0049).

## Adding a catalog skill: what the guards force

A new `SkillSpec` in `internal/catalog/standard.go` is covered automatically by the
catalog-derived eval fixture (ADR-0053) and by ADR-0080's derived guards, which fail
loudly and name the missing piece:

- `TestCatalogTemplatesDegradeLeakFree` sweeps the template under empty data — an
  unconditional reference to another skill must be declared in `RequiresSkills`
  (exact both ways: undeclared references and stale declarations each fail).
- `TestConditionalTemplatesHaveFallbackCases` requires a hand-authored
  `unsetFallbackCases` entry when the template carries conditional fallback prose —
  the degraded phrases themselves stay human-authored.
- `TestEveryCatalogArtifactHasGoldenTest` requires a `Test<Skill>Template` golden in
  `internal/project/spine_test.go`.
- Chain-enabling fixtures derive from the catalog (`chainClosureConfig`), so a new
  chain skill joins them without a hand edit.

Exemptions follow default-inclusion semantics (ADR-0080 Decision 7): every exception
is an explicit entry that itself fails when stale.

## Hard-coded counts in domain narratives drift

The domain current-state narratives (`.awf/domains/parts/*/current-state.md`) are free prose, so a
spelled-out cardinal ("the nine chain-progression skills", "only the ten workflow-chain skills")
is invisible to every deterministic check and goes stale the moment the catalog grows — a 2026-07-07
currency sweep found three such stale counts from ADR-0067/0068 alone, in paragraphs the
feature commits never touched. When a narrative must quantify the catalog, prefer count-free
phrasing ("the `core`-flagged skills", "the chain-progression skills") or expect to sweep every
count in the doc — not just the paragraph you are editing — whenever a chain artifact lands.

## README.md is outside the drift oracle

`README.md` is hand-owned — not rendered, so `awf check` never flags it. Its command table and
feature claims drift silently when the CLI grows (`awf new` and `awf commit-gate` shipped without
README rows). Adding or changing a CLI command means updating the README table in the same
change, per the docs-travel-with-the-change invariant; no deterministic check will remind you.

## `//go:embed` silently skips `_`- and `.`-prefixed paths

The project-local base templates live at `templates/skills/_base/SKILL.md.tmpl` and
`templates/agents/_base.md.tmpl` (ADR-0068). Go's `//go:embed` walk **excludes** every file or
directory whose name begins with `_` or `.` unless the pattern is `all:`-prefixed — so the bare
`//go:embed skills agents …` form embeds neither `_base`, and `fs.ReadFile(templates.FS, …)` fails
at render with a bare `file does not exist`. `templates/embed.go` therefore uses
`all:skills all:agents`. Do not "simplify" it back: dropping `all:` silently drops the base
templates. `TestUnsetFallbackRenders` reads both base templates from `templates.FS`, so a
regression fails the gate — but the failure is a confusing missing-file error, not an obvious embed
bug, hence this note.

## Concurrent agents in one worktree share the git index

Two agents working in the same checkout share one staging area: `git commit` commits the
**whole index**, so another agent's `git add` between your `git add` and `git commit` sweeps
their files into your commit (this bit the ADR-0069 session — a foreign 526-line plan file
landed in a feat commit). Always pathspec-limit the commit itself — `git add <paths> && git
commit -m … -- <paths>` — so only your named paths land regardless of index state; note an
untracked file must still be `git add`-ed first, a bare pathspec commit rejects it. Two more
shared-tree symptoms: `awf audit`'s `uncommitted-changes` error fires on the *other* agent's
dirty files (a false positive for your session — never commit or discard their work to appease
it), and a pre-commit drift-gate failure may be their stale generated file — `./x sync`
freshens the disk, then still commit only your paths. Recurred harder in the ADR-0088/0089
dual session: pathspec discipline alone cannot make a commit *hermetic* — the gate validates
the worktree, but the commit is a slice of the index, so a foreign hunk staged in a shared
file (or a slice cut while the other session's symbol is unstaged) can land a commit that does
not build on its own, breaking bisect (4ef80e0 carries a call to a function a later commit
defines). The deterministic backstop is the `.githooks/pre-commit` staged-slice build: it
checks out the index into a throwaway directory and runs `go build ./...` there, so a
non-building slice refuses at commit time. The real prevention is one git worktree per
concurrent session; when a shared tree is unavoidable, stage HEAD-plus-only-your-hunks
versions of mixed files (write the merged file back to the worktree after `git add`) and
verify the staged slice deliberately.

## A new var reference in a previously var-free template is adopter-visible

Referencing a `{{ .vars.* }}` value in a template that referenced none before changes more than
the prose: every adopter without that var set starts seeing an ADR-0045 unset-var advisory for
the artifact on `awf check`/`awf init`. awf's own repo sets the common vars (`gateCmd` etc.), so
the edit looks silent here and the new advisory only surfaces downstream. Before adding a var
reference, decide whether the sentence really needs it — generic wording avoids the advisory
entirely (this kept `gateCmd` out of the plans README when the self-contained-phases rule
landed) — and if it does, keep the `{{ with }}…{{ else }}` fallback publication-safe and accept
the advisory as intended signal.

## A `data:` list override replaces the catalog defaults wholesale

Setting a list key in an artifact's `.awf/<kind>/<name>.yaml` `data:` block (e.g. the
plan-reviewer's `focusItems`) replaces the catalog's default list — it does not append. Adding
one project-local focus item this way silently dropped the two default items
(`step-exactness`, `dependency-order`) from this repo's rendered reviewer for a day, and
nothing flagged it: the render is drift-clean because the config said exactly that. When adding
a project-local list entry, copy the catalog defaults you still want into the override
alongside it (they live in `internal/catalog/standard.go`), and eyeball the rendered diff for
deleted default lines before committing.

## Registry-relative constants in migration code drift

`migrate.Generation` returned `Current()-1` for a lockless pre-relocation tree — correct when
the To:3 relocation was the newest migration, silently wrong once To:4..6 registered: the tree
gated forever while `awf upgrade` applied only post-relocation no-ops (fixed 2026-07-07). A
generation pinned relative to the *growing* registry moves with every new migration, but a
layout detected by shape sits at a *fixed* point in history — pin the absolute generation (a
lockless `.claude/awf/` tree is the tree-layout port's output, so 1). Treat any `Current()±k`
in migration or versioning code as a red flag unless k describes the current head by
definition. Since ADR-0076 the sentinel generations apply only to genuinely *absent* locks:
a present-but-unreadable lock is a hard error from `Generation` (and every other lock
reader), never `Current()`/`1` — so a corrupt lock can no longer silently bypass the schema
gate the way a mispinned sentinel once mis-gated a healthy tree.

## Binary-side render changes do not reflag rendered outputs

`awf check` hashes config bytes and template bytes — a change to the *Go render logic*
(a new render key, a changed derivation like `taskSkills`) alters what a fresh render
produces while every hash still matches, so check stays clean over now-stale outputs. This
shipped a stale AGENTS.md mid-review on 2026-07-07: the commit changed the derivation but the
rendered guide was produced by the intermediate version, and nothing flagged it. The gap is
deliberate (ADR-0039 keeps `.version` out of the config hash; a binary upgrade advises
"run awf sync" as a note instead of failing). The discipline: any commit that changes render
output for unchanged config/templates must run `./x sync` *after the final code state* and
commit the refreshed outputs — verify by grepping the rendered file for the change, not by
trusting a clean check.

## Documenting a literal `\{{=awf:key}}` token in the agent guide trips the render-guide brace check

`internal/project/guide_scopes_test.go`'s `renderGuide` helper rejects any `{{` or `}}` in the
rendered agent guide as an unrendered template action. A guide default that documents a literal
awf placeholder token renders real braces into the guide (e.g. a bullet mentioning
`\{{=awf:sectionDefault}}`), which trips that guard even though the braces are intentional
content — and it surfaces as a `TestGuideScopesDerived` failure, not an obvious match for a doc
edit. This bit ADR-0072, whose plan specified embedding the full token in the override bullet.
The honest fix is to reference the placeholder by its bare key name in the guide
(`` `sectionDefault` ``) and leave the full token syntax to `working-with-awf.md`, whose render
path tolerates it — the guide is a pointer, that doc is the reference. If the guide must show the
full token, strip `awfPlaceholderRE` matches before the brace check rather than deleting the
guard. (This very entry backslash-escapes its tokens because the pitfalls doc is itself a raw
convention part run through the same substitution.)

## A dispatched fix-applying subagent sometimes commits on a new branch, not `main`

A dispatched subagent that applies fixes as commits — the implementer subagents in
`<prefix>-subagent-driven-development` (as of ADR-0074 the review subagents are report-only and no
longer commit) — occasionally creates a `resync/…` or `review/…` branch and commits there instead
of on the working branch (`main`), leaving the main-thread session behind by those commits (seen
with a resync agent's `89a80c9` on 2026-07-07, and earlier during the ADR-0064 effort). It surfaces as
`git branch --show-current` returning an unexpected branch after a subagent returns, or the
reviewer reporting a commit SHA that `git log` on `main` does not show. Reconcile before
continuing: `git checkout main && git merge --ff-only <branch> && git branch -d <branch>` (the
branch is normally `main` plus the fix commits, so the fast-forward is clean; if it is not, the
subagent also diverged and you must inspect). Prevention is partial — the dispatch brief can say
"commit on the current branch; never create a branch" — so always verify the branch after a
fix-applying subagent returns rather than trusting it stayed put.

## Zero-padded ADR numbers in YAML lists are octal to YAML-1.1 parsers

A bare `0017` inside `related:`/`supersedes:` frontmatter is a leading-zero integer, which a
YAML-1.1 parser (PyYAML and kin) resolves as octal — `0017` becomes 15, silently pointing at the
wrong ADR the moment any tooling starts parsing those fields. Nothing parses them today
(`internal/adr`'s frontmatter struct has no `related` field), the whole corpus was normalized to
bare sorted ints on 2026-07-08, and the ADR-README example now models `[1]` — but the padded form
reads naturally (it matches the `NNNN-` filename convention) so an author copying an old diff can
reintroduce it. Write bare ints (`related: [17, 50]`). If a padded list lands again despite this
note, promote to a gate check that scans `docs/decisions/*.md` frontmatter for `[0`-prefixed list
ints instead of re-recording it.

## Enabled linters constrain API shape — sketch signatures against them

An ADR- or plan-sketched Go signature can be unimplementable as written: the `nilnil` linter
forbids returning a nil pointer beside a nil error, so a "missing → `(nil, nil)`" empty-state
API must carry a `found bool` (or a sentinel error) instead — discovered mid-execution on
ADR-0076's `manifest.LoadOptional`, forcing an amendment-while-Proposed after the signature
had survived two reviews. `errorlint` (wrap with `%w`, no `!=` on errors) and `perfsprint`
(no zero-arg `fmt.Errorf`) similarly bite embedded plan code at commit time. When a design
artifact pins an exact signature or error string, check it against `.golangci.yml`'s enabled
set before the plan freezes; the plan-reviewer's gate-clean-embedded focus covers what it
enumerates, not novel linter/API interactions.

## Growing a pinned set breaks exact-assertion tests the change forgot to enumerate

Several gate tests deliberately pin a *complete current state*: `TestCurrentIsSeven` pins the
migration registry head, `TestUpgradeAppliesInOrderIdempotent`/`TestUpgradeStampsTreeLock` pin
the exact applied-migration list (assertion string **and** the `t.Errorf` want-list message —
the ADR-0077 session fixed the strings and still shipped stale messages), and the config
validation tests pin what the validators accept. Adding a migration, changing a default
set, or bumping the version therefore reds the gate in places far from the edit. When planning
such a change, grep for the pinned tests up front (`Current()`, the applied-list literal, the
version const) and enumerate each update as a plan task — the ADR-0077 plan review found four
of these as blockers precisely because the plan hadn't.

## A partial amendment needs a forward pointer on the amended ADR

ADR-0079 Decision 4 revised one sentence of Implemented ADR-0065's Decision 3 (partial-item
supersedence: the successor cites the item in prose and carries `related: [65]`, the
predecessor stays live) — but nothing pointed 0065 → 0079, so 0065's Decision 3 read as
current guidance with no signal it had been amended; caught only at impl review. The
lifecycle rules regulate amendment-while-Proposed and full supersedence (predecessor status
flips) but say nothing about the reverse edge of a partial amendment. Convention: when ADR X
partially amends ADR Y, add X to Y's `related:` frontmatter in the same commit — a
metadata-only edit, consistent with the mutable `superseded_by` field; the body stays
append-only. This recurred despite the note — ADR-0093's partial supersedence of ADR-0024's
Decision items 1 and 6 landed with no 0024 → 0093 back-pointer, again caught only in
retrospective (fixed 2026-07-11). The recurrence has fired the promotion trigger: a
deterministic check is now warranted — a gate or `repoaudit` rule that, for each ADR body
citing another ADR's specific "Decision item", requires the cited ADR's `related:` to name
the citing ADR.

## A milestone-time check must not double as an every-commit test

`cmd/releasecheck` (ADR-0078) holds a condition that is *supposed* to be false mid-cycle: the
exact changelog pin only has to be true at tag time, and a normal in-cycle repo carries a
non-empty `[Unreleased]`. An early draft of its test suite ran the real check against the live
embedded changelog in the ordinary gate run, with a skip for only one of the two mid-cycle
failure modes — which would have silently re-imposed the every-commit pin the ADR exists to
remove, reddening the gate on the first in-cycle changelog entry (caught before commit,
2026-07-08). When a check's whole point is to hold only at a milestone (a tag, a release, a
schema migration), its unit tests belong on synthetic fixtures; the live repo state is input
for the milestone gate alone.

## gofmt rewrites double backticks in doc comments into curly quotes

Go's doc-comment normalization (gofmt since Go 1.19) treats a literal double-backtick pair in
a doc comment as the old quoting convention and rewrites it to a curly quote (`“`) — so a
comment trying to *depict* markdown double-backtick spans gets silently mangled into wrong
typography, and restoring the backticks verbatim just re-triggers the rewrite (hit twice on
2026-07-09 while landing ADR-0080's sweep). In a doc-comment position, spell the construct out
in words ("a double-backtick quoting span"); literal backtick pairs are only safe inside
non-doc comments or raw strings.

## An ADR's `domains:` list dictates which domain docs a flip commit stages

`./x sync` regenerates the generated ADR index of **every** domain doc named in an ADR's
`domains:` frontmatter, not just the one whose current-state part the effort edited. The
ADR-0080 flip commit staged `docs/domains/rendering.md` (its part changed) but missed the
index-only refresh of `docs/domains/tooling.md` — a gap the plan's file list and three review
passes also missed, caught only by `git status` after the commit (2026-07-09). After the sync
in a status-flip commit, stage from `git status`, not from a memorised file list; every
`domains:` entry implies its rendered doc.

## The atomic `.awf/awf.lock` forces multi-scope rendering work into one commit

`.awf/awf.lock` is a single JSON manifest of every rendered file's hashes, so a change that
regenerates outputs across several commit *scopes* — skill templates (`rendering`), an ADR flip's
domain indexes (`adr`), an `agents-doc.yaml` invariant promotion (`invariants`) — produces one
lock diff whose hunks cannot be sliced by pathspec: `git add <path>` is all-or-nothing on a file
and interactive `git add -p` is unavailable here. Don't fight it into per-scope commits — the
coupled regeneration *is* the concern. Bundle the source edits, their rendered outputs, and the
whole lock into one commit; "one concern per commit" is satisfied because the concern is the
coupled change, not each scope it happens to touch (ADR-0092 stage b, 2026-07-11: skill adoption
+ ADR flip + invariant promotion landed as one `feat(rendering)` commit — the impl review
confirmed the boundary was reasoned, not a scope violation). A genuinely independent, non-rendered
file — a `./x` arm, a lockless script — still commits on its own.

## Obsoleting rendered prose: sweep parts and whole narratives, not just templates

Fixing a template's default sentence does not reach a project whose convention part
*overrides* that section — awf's own `.awf/parts/agents-doc/awf-setup.md` kept ADR-0081's
obsoleted "disable them as a unit" instruction past the template rewrite, a sync, and three
review passes (caught only by the impl review's dogfood check, 2026-07-09). The same session
also left a domain current-state narrative self-contradicting by *appending* the correction
while an earlier clause still asserted the old behavior. When a change obsoletes prose, grep
the repo for the phrase being retired (`templates/`, `.awf/**/parts/`, and the rendered
outputs will surface every carrier, parts included) and rewrite stale clauses in place —
an appended correction beside a surviving old claim is worse than either alone.

## An empty scan result only counts once the probe provably ran

A compound shell command that errors mid-sequence silently drops every later segment: a
`grep A; echo ---; grep B` probe whose separator failed to parse never ran the second grep,
and its missing output was read as "no matches — templates carry no repo-identity literals"
(2026-07-09, ADR-0082 brainstorm). The false premise shaped a design decision and survived
until a fresh-context grounding check re-ran the scan and found two hits. Absence of output
is not verified absence: run verification probes as separate invocations (or with an
explicit sentinel/exit-code check), and treat any scan whose success path you did not
observe as unrun, not clean.

## A coverage-ignore justification is stale the moment its line is refactored

A `coverage-ignore` comment asserts a reachability claim about the code shape it was written
against; a refactor that merges call sites, changes a signature, or widens who runs the code
invalidates the claim silently while the comment rides along verbatim. The schema-6 migration
rework (2026-07-09) carried three pre-existing ignores through exactly such a refactor — the
"fresh trees" justification was already false for a partial-prior-migration adopter tree, and
ENOTDIR/EISDIR reached the "permission fault only" branches with no permissions involved; the
impl review refuted all three (plus a fourth on a directed probe) by staging the state each
declared impossible. When an edit touches a line carrying an ignore, the ignore is part of
the edit: re-probe the claim (try to stage the "impossible" state — a leftover destination
file, a path component as a regular file, a config path as a directory) or drop it and cover
the branch; `./x audit-local`'s `coverage-ignore-added` warning flags every touched ignore in
the range for exactly this re-evaluation. A fourth recurrence (ADR-0088) sharpened the
heuristic: an ignore claiming "X already exercised this, so it cannot fail here" is false
whenever the guarded call consumes an input X never touched — five "cannot fail after
RenderAll succeeded" ignores guarded the generated config reference, whose intro convention
part RenderAll never reads because the reference renders outside it; a directory staged at
the part path reached every one. A fifth recurrence (the 2026-07-10 worktree fix) refuted an
"only a delete race loses it" claim on a read-after-stat: file *permissions* are the standard
refuting move for any "stat just succeeded, so the read cannot fail" shape — a `chmod 0o000`
file stats fine and fails the read deterministically (guard the test with a
`os.Geteuid() == 0` skip, since root bypasses it).

## An attribute-filtered pinned-set test exempts every other attribute value

A pinned-list test that selects its population by an attribute (`if d.Kind == "string"`)
pins only that slice: anything added under a different attribute value bypasses the pin
entirely while the invariant it backs claims the whole set is closed. The first
`var-descriptor-set-pinned` backing test (2026-07-09, ADR-0084) filtered descriptors by
`Kind == "string"`, so a prose knob reintroduced as `Kind: "enum"` would have passed both
descriptor-parity and the pin without the successor ADR the invariant exists to force; the
impl review caught it the same day. When a pin means "this set is closed", enumerate the
partition exhaustively — iterate everything, route each element to an asserted bucket
(pinned set, known-exempt set), and fail on any element that lands in neither — instead of
filtering to the bucket you thought of first.

## A Proposed ADR's same-commit state-doc update must not speak in present tense

The proposing step updates the shifted domain's current-state doc in the ADR's own commit,
but the ADR is still `Proposed` there — writing "the four prose knobs are removed" while the
catalog still carries all four makes the state doc false until implementation lands, and
permanently false if the ADR is rejected (ADR-0084 review, 2026-07-09). Write the
propose-commit sentence in decision tense anchored to the status — "ADR-NNNN (Proposed)
narrows the policy … and will remove …" — and flip it to present tense in the
implementation commit that makes it true, alongside the status flip.

## Retiring an Implemented ADR's invariant couples the feature to the successor's status flip

The invariant scanner demands every `inv:` slug of an Implemented ADR stay backed until an
*Implemented* successor retires it — so a commit that removes or renames the backing marker
cannot land while the successor is still Proposed, and the green-gate-per-commit rule then
forces the feature and the successor's `Implemented` flip into one commit. The ADR-0085
implementation (2026-07-10) planned "feature commit, then flip commit" and hit this at the
first `./x check`: retiring `bootstrap-pin` for `bootstrap-env-override` unbacked an
Implemented ADR-0040 slug. When an effort carries a `retires_invariants:` entry, plan the
final implementation commit to include the status flip from the start.

## An unescaped consumable placeholder in a part is silently rewritten, check-clean

The guide-brace-check entry above covers a token tripping a test; the worse variant trips
nothing. A convention part that *documents* a consumable placeholder (`\{{=awf:gateCmd}}`,
`\{{=awf:checkCmd}}`) without the backslash escape gets substituted like any other part:
the rendered doc shows the var's *value* where the token *name* belongs, `awf check` stays
clean (the output is exactly what the config produces), and only a human reading the
rendered file notices. Bit the ADR-0086 docs commit (2026-07-10): the rendering domain's
current-state part quoted both tokens bare while the same file escaped
`\{{=awf:sectionDefault}}` two sentences earlier. Machine-checking this is off the table —
substituting inside backticks is also a legitimate pattern — so it is promoted to a
code-review focus item (`part-placeholder-escaping`): quoted-as-syntax means escaped,
meant-to-resolve means bare, and verify by reading the rendered file, not the part.

## Moving a check earlier in the pipeline steals a later stage's error-branch coverage

A fixture that corrupts state up front (a directory where a sidecar file belongs, an
unreadable file) to pin a *late* stage's error propagation silently changes meaning when a
new earlier stage starts reading the same state: the error now surfaces there, the late
branch goes uncovered, and the 100% gate flags a line nobody edited. ADR-0086's open-time
domain-sidecar validation did this to `TestAuditPropagatesDomainSidecarReadError`
(2026-07-10) — Audit's read-error branch was suddenly unreachable from a pre-corrupted
tree. The repair pattern: corrupt *after* the earlier stage has run (post-`Open`
mutation), so each stage's own error branch keeps a test that reaches it; and when adding
an earlier check, grep the tests for fixtures corrupting the state it newly reads.

## Parallel sessions share one git index

Two agent sessions (or an agent and a human) working in the same checkout share the staging
area: a bare `git commit` after `git add <own files>` commits the *whole index*, silently
sweeping whatever the other session had staged — it happened between the ADR-0087 and
ADR-0088 efforts (2026-07-10), folding one effort's staged review fixes into the other's
feature commit. Repair: `git reset --soft HEAD~1`, then re-commit with an explicit pathspec
(`git commit -m … -- <paths>`), which also leaves the foreign entries staged exactly as
found. Prevention: when a `git status` shows staged entries you did not stage, commit with a
pathspec (tracked files only — stage a brand-new file first) or move one effort to a
worktree; also prefer targeted reverts over `git checkout <file>` while your own edits are
uncommitted, which is how a verify-mutation revert erased a just-written test the same day.

**This recurred within hours of being recorded** (same day, opposite direction: the
ADR-0088 session's feature commit swept the ADR-0089 effort's `render.go` hunks, leaving a
commit that does not compile standalone — green-gate-per-commit is unenforceable across a
shared tree because the hook validates the *tree*, not the commit). Pathspec discipline
cannot separate two efforts' hunks inside one file, and prose did not prevent the second
occurrence. The real rule: **parallel sessions get separate git worktrees, full stop.** A
shared checkout is single-writer; the moment a second effort starts, move it
(`git worktree add`) or serialize.

## Link ADRs by their on-disk filename, never by constructing one from the title

An ADR's kebab filename is derived from its title at `awf new adr` time, but retellings
drift ("convention-parts-raw-not-templated" vs the actual "convention-parts-are-raw-input")
— three invented link targets landed in ADR-0087's first draft (2026-07-10) and survived to
the verify pass because the ADR-0020 dead-link scan covers awf-managed *rendered* docs
only, not `docs/decisions/`. `ls docs/decisions/ | grep <number>` first, then link.

## `adr.ADR.Title` already carries the `ADR-NNNN: ` prefix

`adr.ParseDir` reads `Title` verbatim from the `# ADR-NNNN: …` heading, so it includes the
`ADR-NNNN: ` prefix while `Number` carries the digits separately. A new consumer that prints
both — as `awf context`'s `ADRRef` did in its first draft (2026-07-11) — double-prints the
number (`ADR-0092 … ADR-0092: Title`); plan review caught it. Strip the prefix
(`strings.TrimPrefix(a.Title, "ADR-"+a.Number+": ")`) when surfacing Title alongside Number.
`awf context` was the first `adr.ParseDir` consumer outside `internal/{adr,invariants,audit}`,
so the gotcha only surfaces as awf grows ADR-aware tooling.

## Repo opens must resolve the `.git` gitfile (use `internal/git.OpenRepo`)

In a linked worktree (`git worktree add`) — and the submodule layout — `.git` is a
`gitdir:` pointer file, not a directory; a naive `<root>/.git` filesystem open dies with
`open repo: … .git/config: not a directory` (bit `awf audit` 2026-07-10, running the
ADR-0090 impl review in a session worktree — every parallel session uses one). Fixed the
same day: `OpenRepo`'s `dotGitFs` resolves the pointer and routes shared state through the
`commondir` via `dotgit.NewRepositoryFilesystem`, mirroring go-git's
`EnableDotGitCommonDir`; regression tests hand-craft the worktree layout (go-git cannot
create one). The standing rule from the first entry above still governs: any future
awf code opening a repo goes through `internal/git.OpenRepo` — `git.PlainOpen` gets neither
the extensions workaround nor the gitfile resolution. (The helper lived in `internal/audit`
until ADR-0092's stage-a extracted it into the shared `internal/git` package.)

## Section parts carry their own heading

A convention part replaces its section's *body*, and for most doc/guide sections the
`## Heading` line lives inside that body — a part written without it renders a headless
section (ADR-0090's identity part landed headingless on first sync; only comparing the
rendered file against this repo's own parts caught it). When authoring a part, check the
section's default (or an existing adopter's part) for whether the heading is yours to write.
No note fires: the stub advisory clears the moment the part exists, whatever its shape.

## Verify compound-chain side effects by reading the target back

A `cd <dir> && <edit>` chain silently skips the edit when the `cd` fails (wrong cwd
assumption across tool calls), while an unchained command later in the same block still
runs and succeeds — the block "worked", green output and all, minus the one side effect
that mattered (an `invariants:` config append vanished this way in the ADR-0090 session;
only `tail`-ing the file exposed it). After any compound chain that edits state, verify the
target's content, not the block's exit status.

## Hand-rolled repo enumeration crosses repository boundaries

Anything that walks or opens the repo tree itself must define its repository boundary;
the Go toolchain gets this free (dot-dirs and nested modules are invisible to `./...`),
hand-rolled walkers and repo opens do not. Three independent instances bit on 2026-07-10
alone: `awf audit` could not open a linked-worktree checkout (`.git` gitfile — the entry
above), `TestLoadReadsTreeRoot`'s legacy-ref sweep flagged `internal/migrate` copies
inside `.claude/worktrees/` session checkouts, and the invariants scanner let markers in
nested checkouts silently back this project's slugs — the worst shape, a false green in
both directions. The rule: a filesystem walk prunes any subdirectory carrying its own
`.git` entry (directory in a primary clone, gitdir-pointer file in a worktree or
submodule) and takes a deliberate stance on hidden directories; a repo open resolves the
gitfile layout; enumeration derived from git history (commit diffs, `git ls-files`) is
immune by construction and preferred where it fits.

## An `inv:` backing marker must open its own line, not trail a statement

The invariant-backing scanner (`internal/invariants`) matches a slug only when the marker
opens its line after indentation — `strings.TrimLeft(line, " \t")` must start with the
configured marker, then `^\s*invariant:\s*<slug>`. This is deliberate: a mid-line match
could sit inside a string literal (a test fixture's source-code string) and falsely back a
slug. The consequence is a natural-looking backing that silently does not count: a trailing
`clone.Docs = maps.Clone(...) // invariant: local-doc-catalog-clone` reads as *unbacked*,
and because `awf check` only enforces backing once the ADR is `Implemented`, the gap hides
until the status flip — exactly when the effort is trying to conclude. Put the
`// invariant: <slug>` on its own line directly above the statement it describes.
