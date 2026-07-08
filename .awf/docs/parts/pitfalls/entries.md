## `awf audit` and `extensions.worktreeConfig`

`git.PlainOpen` (go-git) refuses to open a repo whose `.git/config` has `extensions.worktreeConfig = true` — a flag `git worktree add` can leave behind even after the worktree is removed — regardless of `core.repositoryformatversion`. Cause: go-git's extension-support check lowercases the extension name before comparing it against its allow-list, whose key is mixed-case, so the lookup never matches. `internal/audit/git.go`'s `openRepo` works around it by opening through a `storage.Storer` wrapper that hides the `[extensions]` config section from go-git before the check runs; neither `Collect` nor `ruleUncommittedChanges` reads repo extensions, so hiding the section is safe. Any future `internal/audit` code opening a repo must go through `openRepo`, not `git.PlainOpen` directly.

## Stdout is API in command-substitution scripts

`.awf/bootstrap.sh` is consumed as `"$(bash .awf/bootstrap.sh)" <args>`, so its stdout is the
binary path and nothing else — a checksum tool's `<asset>: OK` line on stdout execs as part of
the command and fails only on cache-miss runs, which presents as flaky CI. Every diagnostic in
rendered shell must carry `>&2`; `TestBootstrapStdoutPathOnly` pins this (`inv:
bootstrap-stdout-path-only`, ADR-0049).

## Adding a catalog skill: hand-enumerated test touch points

A new `SkillSpec` in `internal/catalog/standard.go` is covered automatically by the
catalog-derived eval fixture (`inv: evals-full-catalog-coverage`, ADR-0053), but several sibling
tests still hand-enumerate their skill set and fail only at `./x gate` time, not at authoring:

- `TestUnsetFallbackRenders` (`internal/project/spine_test.go`) — the publication-safety regression
  lock that renders each skill under empty data and bans leaks; a skill with conditional
  (`{{ if … }}`) branches must be added here or its degradation goes unlocked.
- The per-skill `Test<Skill>Template` golden in the same file — one per skill by convention.
- Chain-enabling fixtures that hardcode a skills list (e.g. `TestScopesEditReflagsReferencingArtifacts`
  in `internal/project/drift_test.go`): once a Core chain skill is referenced unconditionally by a
  sibling — as `reviewing-impl` names `retrospective` and `executing-plans` — every such fixture
  must enable it too, or trip the no-dead-skill-references gate (ADR-0046).

The durable fix is to derive these skill sets from `catalog.Standard` the way ADR-0053 did for the
eval fixture, closing the silent-rot gap; until then, adding a skill means updating these by hand.

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
freshens the disk, then still commit only your paths.

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
