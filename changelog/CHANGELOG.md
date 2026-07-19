# Changelog

All notable changes to `awf` are documented here, newest first. Entries are grouped per release
into up to four categories (Breaking changes, Features, Bug fixes, Others) chosen by actual
adopter-facing effect (does it change rendered template output, CLI behavior, or config/lock
schema), not by mirroring a commit's Conventional Commits type. Run `awf changelog --help` to
query a single version or a range.

## [Unreleased]

### Breaking changes
- Version 0.18.0 introduces schema generation 14, `current-state-topic-substrate`, and the optional
  strict `currentState` bridge-preparation config beside unchanged legacy `invariants`. The new keys
  describe marker sources, proof test globs, topic coverage and fan-out severities, and a positive
  per-path topic budget (default 8), but they do not switch normal context or invariant authority.
  This is not migration readiness: Plans 1 and 2 are one unreleased bridge tranche and must both
  land before any bridge release; the release check refuses publication in the interim.
- Pi's `subagent_explore` now requires `{task, breadth, detail}` (ADR-0132). Breadth is
  `targeted`, `bounded`, or `broad`; detail is `paths`, `summary`, or `analysis`. Hand-authored
  calls that pass only `task` must add both fields.
- The new core `exploring` skill gives every target one bounded exploration and reporting
  protocol. schema-13 `exploring-skill-closure` automatically adds it to adopted configs that
  enable brainstorming, debugging, or refactor-coupling-audit; run `awf upgrade`.
- The `supersedes:` and `superseded_by:` ADR frontmatter keys are removed, and full ADR
  supersession is now derived from anchor coverage rather than declared (ADR-0128, ADR-0129,
  ADR-0130). **Run `awf upgrade`: the generation-12 migration rewrites `docs/decisions/`**,
  stripping both keys, downgrading every pre-existing `` `supersedes: ADR-NNNN#<item>` `` token
  to the new `` `refines:` `` relation, appending a supersedence-bookkeeping Decision item that
  retires each superseded predecessor's anchors, backfilling the predecessors' `related:`
  back-pointers, and rewriting `status: Superseded by ADR-NNNN` to bare `status: Superseded`.
  `awf check` refuses either key with upgrade guidance.

  Supersession now has one encoding and two relations: `` `supersedes:` `` and
  `` `supersedes-invariant:` `` **retire** an anchor (a Decision item or a declared invariant
  slug), while the new `` `refines:` `` **adapts** one and counts toward nothing. An ADR is
  `Superseded` exactly when every one of its anchors carries a retirement from a carrier that
  has shipped (`Implemented` or `Superseded` - superseding an ADR does not un-supersede what
  that ADR superseded, so chains deeper than two generations resolve); the status stays hand-authored and `awf check` refuses drift in both directions, naming
  the required edit. The status is bare because coverage may split across several successors.
  The mechanical migration downgrades every existing item token to `refines:` deliberately - it
  asserts less, and promoting a genuine retirement back is a reviewable edit, whereas a wrong
  retirement silently kills an ADR.

  Two further checks arrive with it: the `related:` back-pointer is now owed for token targets
  of **any** status (previously live targets only), and a token claiming its own carrier's
  anchor, or a retirement cycle among fully covered ADRs, is refused. The advisory for a token
  into an already-superseded target is dropped, since that is now the normal shape of every
  completed supersedence.
- `awf audit` now requires an explicit commit range and has no default (ADR-0127). Pass a bare
  `<base>` (meaning `<base>..HEAD`) or a two-sided `<a>..<b>`; a no-argument invocation is refused.
  The `--base` flag is removed, superseded by the positional argument. `./x audit-local` (this
  repo's own tooling) loses its `origin/main..HEAD` default on the same grounds.
- The `audit.baseBranch` config key is removed, along with its `main` default. A schema-11
  migration strips it from `.awf/config.yaml` and reports the removal, so run `awf upgrade` after
  upgrading the binary. awf no longer holds an opinion about which branch you integrate into: a
  configured base that already contained HEAD silently emptied the range and made every history
  rule inert while the command still reported clean.
- Pi target adopters now receive executable project extension files under
  `.pi/extensions/awf-subagents/` on sync (ADR-0123). Pi's project-trust boundary applies and Pi
  0.80.9 or newer is required; `awf check` reports extension drift and `awf sync` repairs it. Pi
  workflow skills call the four governed extension tools explicitly, while other targets retain
  their existing dispatch language.
- ADR supersession is structured and machine-checked (ADR-0120), and the `retires_invariants:`
  frontmatter key is removed from the ADR schema. **Run `awf upgrade`: the generation-10
  migration rewrites `docs/decisions/`**, stripping every `retires_invariants:` key and
  appending a retirement-bookkeeping Decision item carrying one
  `` `supersedes-invariant: ADR-NNNN#<slug>` `` token per retired slug (plus the target's
  `related:` back-pointer); a corpus still carrying the raw key fails `awf check` until the
  migration runs. Two token grammars express partial supersession inside a successor's Decision
  section: `` `supersedes: ADR-NNNN#<item>` `` (a Decision item) and
  `` `supersedes-invariant: ADR-NNNN#<slug>` `` (an invariant, which an Implemented carrier
  retires from owed backing). New `awf check` errors, any status: every ADR's Decision section
  must be column-0 numbered items sequential from 1 (item numbers are the stable anchors);
  `supersedes:` frontmatter is finally parsed and its three-way symmetry enforced (claim,
  `Superseded by ADR-NNNN` status flip, scalar `superseded_by:`); every token's ref must
  resolve (existing non-Proposed target, in-range item or declared slug); a token into a live
  target requires the target's `related:` back-pointer; and one successor cannot both fully and
  partially supersede the same target. Two non-failing advisories: a token whose target was
  later fully superseded, and one anchor claimed by two live ADRs. `ACTIVE.md` gains a
  `## Supersedence` section (full chains plus superseded anchors on live ADRs; omitted for a
  supersession-free corpus) and `awf context` annotates surfaced ADRs with their overridden
  anchors.
- Seven typographic punctuation substitutes are banned from the prose awf ships (ADR-0115): the
  em-dash (U+2014), en-dash (U+2013), ellipsis (U+2026), and the four curly quotes (U+2018,
  U+2019, U+201C, U+201D). The generated `docs/decisions/ACTIVE.md` now renders a row's status in
  parentheses (`- [ADR-0001: Title](0001-file.md) (Accepted)`) instead of after an em-dash, so
  **every adopter's committed `ACTIVE.md` drifts until they run `awf sync`**, and `awf check`
  reports it until they do. The shipped templates, awf's own output strings, and this changelog
  are cleaned to match. The rendered documentation standard's plain-punctuation rule is rewritten
  to name all seven codepoints and now covers authored prose (ADRs, plans, and hand-written docs)
  as well as shipped prose (ADR-0117), so `docs/doc-standard.md` re-renders too. Nothing rewrites
  prose you have already written. Notation (arrows, mathematical symbols, accented letters) is
  unaffected.
- Invariant backing is redesigned into enforced test-backing with a two-marker
  vocabulary (ADR-0105, ADR-0106). The ADR Invariants-section declaration token is
  unified from `inv: <slug>` to `invariant: <slug>`: the same token the source
  proof marker uses (**adopters must rewrite `inv:`→`invariant:` in their own
  `docs/decisions/**`; awf cannot auto-migrate user-owned ADR prose**). Source
  markers split into a proof `invariant: <slug>` marker and an advisory
  `touches-invariant: <slug>, <note>` context marker. A new `invariants.testGlobs`
  config scopes the proof marker to test files (backing then means an executed test
  line); when it is empty or absent, backing falls back to source-glob scope, so
  the change is additive for projects that do not set it. Each invariant is declared
  `invariant:` (backed) or `unbacked-invariant:` (unbacked, carrying a `Verify:`
  note), symmetrically enforced: `awf check` fails a backed slug with no proof
  marker, an unbacked slug that has a proof marker, and an unbacked declaration
  missing its `Verify:` note; a marker naming an undeclared slug and a note-less
  `touches-invariant:` are non-failing advisories. `awf context` now labels each
  governing invariant `backed`/`unbacked` and surfaces its `Verify:`/touches site
  notes (the `--json` invariant refs carry per-invariant `class` and notes), reading
  as a risk map: its Tier-1 scan spans both markers across the source and test globs.
- `awf context <paths>` output is now relevance-tiered (ADR-0104). It no longer
  dumps every ADR/pitfall sharing a queried path's domain. The human render gains
  `## Governing ADRs (invariants backed here)` (Tier 1: ADRs whose invariants are
  backed under the queried paths), `## Related ADRs (shared tag)` / `## Related
  pitfalls (shared tag)` (Tier 2: sharing a finer-than-domain precise tag or
  `related:`-linked), and a one-line `## Domain background: N more ADR(s)` (Tier 3,
  collapsed). The `--json` shape changes accordingly: the flat `adrs` array is
  replaced by `governing` + `related` + an integer `background`; each ADR ref drops
  its `invariants` echo; and each pitfall ref carries `tags` instead of `domains`.
  Pitfalls now surface by shared tag, not by domain membership. The
  `context-surfaces-pitfalls` and `context-surfaces-linked-plans` invariants are
  retired for tiered successors. Read-only, output-parity, and static-fallback are
  unchanged.
- Pitfalls become a structured, domain-tagged sidecar-derived doc (ADR-0099).
  `docs/pitfalls.md` is no longer authored as a free-prose `entries` part; its
  entries now live as a `data.pitfalls` list of `{title, domains, related, body}`
  in `.awf/docs/pitfalls.yaml`, rendered by a transform (the ADR-0089 seam the
  glossary uses). The schema-9 `pitfalls-data` migration ports adopters on the
  next `awf upgrade`: it auto-splits an existing `entries.md` on its top-level
  `##` headings (fenced-code `##` lines skipped) into `data.pitfalls` entries with
  empty `domains`, deletes the part, and prints one provenance line per entry plus
  a review instruction. **Review the split and tag each entry's `domains:`**;
  untagged entries render but do not surface in `awf context`. An entry's optional
  `related:` ADR numbers render as plain `ADR-NNNN` text and are link-validated.
  `awf check` now fails on unparseable pitfalls data, a bad entry shape, an unknown
  `domains:`, or a dangling `related:`. Schema bumps to 9 (awf `0.17.0`).

### Features
- `awf upgrade --check` reports exhaustive current-state bridge readiness without writing the
  worktree, index, config, lock, approval input, or generated output. It inventories exact shipped
  legacy invariant declarations and retirements, plans idempotent Migration history/status/config and
  qualified-marker normalization, requires repository-reviewed strict
  `.awf/current-state-migration.yaml` evidence only after independently deriving each unique
  Origin/backing-preserving mapping, checks scoped topic coverage and migration-safe terminal output
  deletion, and emits deterministic human or `--json` findings, computed adjudications, and exact
  before/after path/mode/SHA-256 mutation records. The ephemeral approval input does not bump schema
  14, cannot disambiguate mappings, requires `invariantApprovals: []` for an empty live inventory,
  and is omitted from mutations when unchanged. The authority switch and runtime consumption of the
  attestation are intentionally still absent.
- `awf upgrade --attest-current-state` seals a ready, clean-HEAD prepared tree through a recoverable
  transaction. It reruns readiness, refuses any staged, unstaged, or untracked change, records the
  clean HEAD, a digest over the post-normalization config, domains, ADRs, topics, marker sources, and
  the required approval file, and the ADR cutoff and gaps in an optional `bridgeAttestation` lock
  block (old locks omit it). It then journals every normalization, marker, status, and terminal
  legacy-index deletion at `.awf/current-state-upgrade.journal`, applies them, and commits the
  attested lock last; the unchanged approval file never enters the mutations. Obtain and verify the
  matching current-state binary before attesting. Because the terminal projection prunes
  `docs/decisions/ACTIVE.md` and the domain ADR indexes without generating their replacements, the
  attested project is deliberately index-pruned and refuses every ordinary command until a later
  current-state release consumes the attestation.
- `awf upgrade --recover` replays the journal's recovery table: a precommit journal whose lock still
  differs from the sealed hash rolls every prior image back in reverse; a precommit or lock-committed
  journal already carrying the sealed lock cleans up the residue; a lock-committed journal with a
  different lock refuses rather than rolling committed authority back; a third-party edit halts and
  preserves the journal, naming the path. A committed journal or attestation now makes ordinary
  commands non-operational: with a journal present only `awf upgrade --recover` proceeds, with an
  attested lock only `awf upgrade --check` inspects it, a malformed journal refuses every mode
  (recovery included) with deterministic Git-restoration-and-bridge-reinstallation guidance, and
  `awf version`/`awf changelog`/`awf help` always bypass the transaction state. This is still one
  unreleased bridge tranche; no intermediate release may be cut.
- `awf topic <domain>/<topic>[:<claim>]` adds a version-gated, read-only active-state query with one
  deterministic human/JSON model. Defaults show current title/summary, claims, types, prose, and
  backing while hiding provenance and references. Independent `--history`, `--references`, and
  `--coverage` flags add direct ADR details, direct incoming/outgoing claim IDs, and declared/effective
  scope plus marker sites; no option traverses transitively or resolves removed history. Outside an
  adopted tree the command prints a static reference without gating. This remains unreleased bridge
  preparation until the bridge-migration tranche lands.
- `awf new topic <domain> "<title>"` scaffolds exactly the paired current-state metadata and authored
  part without syncing or mutating config, lock, or rendered docs. It allocates a collision-free
  kebab slug, protects the reserved `index` and either orphaned half, rolls back the first file if the
  second write fails, and prints both repository-relative paths. The scaffold contains a valid path
  placeholder, generic prose, and no invented claims; adopters must edit and author it manually. A
  zero-claim shell renders but does not satisfy scoped coverage. This is still unreleased bridge
  preparation, not migration readiness or runtime authority, and no intermediate release is allowed.
- The unreleased current-state topic producer strictly pairs
  `.awf/topics/metadata/<domain>/<topic>.yaml` with
  `.awf/topics/parts/<domain>/<topic>/current-state.md`, parses canonical rule and invariant claims,
  resolves Implemented-ADR provenance and direct references, and validates qualified configured
  state, touches, and proof markers. It renders managed topic pages and sorted per-domain indexes,
  adds compact domain navigation without removing Decisions, and joins ordinary output-plan,
  manifest, brownfield backup, drift, collision, and prune behavior. This is preparation substrate,
  not migration readiness or shadow authority: legacy context and invariant enforcement remain
  unchanged, and no release may occur before the following bridge-migration plan adds readiness,
  attestation, and ordinary-command refusal.
- **`awf check` reports a supersession claim stated in prose and never encoded** (ADR-0131). The
  new `adr-unencoded-claim` finding fires when an override verb occurs in the same Decision item
  as a citation of another ADR's anchor and that item carries no relation token for it, naming the
  carrier, its item, the anchor, and the token shapes that would satisfy it. Item citations are
  recognized in six spellings: `ADR-NNNN Decision item N`, `ADR-NNNN Decision N`,
  `ADR-NNNN item N`, `ADR-NNNN DN`, plus the possessive `ADR-NNNN's ...` and markdown-link
  `[ADR-NNNN](path) ...` wrappers. Exemptions are structural, never a marker: a `Proposed` target,
  a self-citation, a slug the target never declares, anything outside `## Decision`, and an **item**
  citation inside an inline code span, so an ADR can discuss the item-citation grammar without
  tripping it. A slug citation is recognized regardless of its code span: the backticks in
  `` `inv: <slug>` `` are the citation syntax itself rather than a quoting device, so masking them
  would recognize none. To record one as informational, write `cites-invariant:`.
- **Two new relation tokens, `cites: ADR-NNNN#<item>` and `cites-invariant: ADR-NNNN#<slug>`**
  (ADR-0131), for a Decision item that mentions, quotes, or reasons from another ADR's anchor
  without changing it. A citation asserts nothing: it contributes no anchor coverage, so it cannot
  retire an ADR or drop an invariant's backing, and it renders in no `ACTIVE.md` or domain-index
  annotation. It exists so the check above has a truthful answer for an informational citation;
  without it an author reaches for a relation token and records a supersession that never
  happened. It still owes the `related:` back-pointer every relation owes. Judge the key by the
  target's clause set, not the carrier's verb.
- `awf check` reports `adr-related-order` when an ADR's `related:` array does not ascend, naming
  the first descent (ADR-0131). A back-pointer edge has exactly one correct position, so appending
  a low-numbered carrier to an array that already names a higher one is an authoring slip that
  previously went unseen. Resolution and ordering are reported independently, so a descending
  array still has every entry checked against the corpus. Sorting an existing array is a
  meaning-preserving edit: `related:` carries an unordered set.
- `awf audit` reports the range and commit count it evaluated on every run, so a verdict is never
  readable without its scope. A range resolving to zero commits says so explicitly instead of
  printing `clean`, which previously made "examined forty commits" and "examined none"
  indistinguishable.
- Pi now ships dedicated `subagent_grounding` and binds brainstorming to it while retaining
  `subagent_explore` for general investigation and coupling audits. All four subagent roles show
  bounded inline progress from context-isolated details; only final report or failure-summary
  content reaches the parent model (ADR-0125).
- The Pi target ships `subagent_grounding`, `subagent_explore`, `subagent_review`, and
  `subagent_implement`: isolated no-session child processes for grounding, read-oriented
  exploration, the three governed reviewer bodies, and serialized same-checkout implementation
  with explicit commit permission (ADR-0123, ADR-0125).
- Codex, Pi, Gemini, and Copilot are now selectable targets for agent artifacts.
  Codex renders skills under `.agents/skills/` and validated custom-agent profiles
  under `.codex/agents/` as TOML with `name`, `description`, and
  `developer_instructions` fields. Pi renders generic review-dispatch wording;
  Gemini imports `AGENTS.md` through `GEMINI.md`; Copilot uses `.agent.md` agents.
- Whole-line `<!-- awf:comment ... -->` authoring comments in templates and convention parts
  (ADR-0121): stripped at render with their newline, so parts and templates can carry
  internal notes and `touches-invariant:` tags that never reach rendered output. Whole-line
  and exact-literal only (mid-line and whitespace-variant forms still render; a malformed
  whole-line opener is a hard render error naming the source; fenced demos are preserved).
  `invariants.sources` entries gain an optional `close:` token (`-->`, `*/`) stripped from
  marker lines before tag parsing, so block-comment-family markers - the new tagging recipe
  included - yield clean touches notes.
- New `awf prose-gate` command (ADR-0119): a blocking, presence-level scan of every tracked text
  file for the seven banned typographic punctuation substitutes, the counterpart to the advisory
  `plain-punctuation` audit rule. It is opt-in through `proseGate.enabled` (bool, default off,
  because a presence gate would fail an unswept tree on the day it lands) and exits zero without
  scanning when off, so a hook may invoke it unconditionally. Genuine depictions are pinned in
  `proseGate.exemptions` (a list of `path` plus `codepoint`, the codepoint spelled as `U+2014`
  rather than typed, with an optional `count`). Adopters wire it into a pre-commit hook: the
  rendered `.awf/hooks/pre-commit.sh` payload gains an `awf prose-gate` line, so **an adopter who
  upgrades sees their committed payload drift until they run `awf sync`**, even with the knob off.
  The documentation standard's plain-punctuation rule now also lists the bare hyphen as a valid
  em-dash replacement.
- `awf audit` gains an advisory `plain-punctuation` rule (ADR-0117), on by default and switched
  off with `audit.plainPunctuation: false`. It warns, and never errors, when a commit **raises**
  the count of a typographic punctuation substitute in an authored markdown file under `docsDir`.
  Prose already written never warns: only a net increase does, so there is no allowlist, no cutoff
  date, and nothing to migrate. Generated files are skipped.
- awf can now render **co-owned files with in-place-editable sections** (ADR-0100) and ships
  a **managed command-runner `x`** as their first consumer (ADR-0101). A section declared with
  the `inplace` marker has its body read back from the existing rendered output (bounded by its
  `awf:edit-in-place` provenance pointer and awf's next section pointer) and preserved across
  syncs, while awf regenerates every other section and the file structure; such a file is
  drift-checked by regeneration-with-read-back (a first-class `RegenChecked` attribute that
  replaced awf's hardcoded generated-index list). Two shell-script properties are now rendered per
  target off the one `#!`-shebang predicate: the surviving `awf:edit`-family pointers take the
  target's comment syntax (`#` for a shebang script, HTML otherwise), and a rendered `#!` file is
  written executable (`0755`, enforced every sync), so **the bootstrap and hook payloads flip
  from `0644` to `0755` on the next sync** (harmless; still `bash ...`-invoked). Enable the new
  `runner` singleton (`awf enable runner`, or set `runner.enabled` in `.awf/config.yaml`) to render
  `x` at the repo root: awf owns the awf-verb dispatch (`sync check invariants audit context
  commit-gate new`, delegating to the pinned binary via the bootstrap), and the setup and
  project-verb regions are yours to edit in place. awf itself keeps its from-source runner; the
  `examples/sundial` adopter demonstrates the feature.
- `awf check` now validates planned commit subjects in plans (ADR-0111). A plan marks
  a phase's closing-commit subject with a fenced code block tagged `commit`; `awf check`
  reads its first non-empty line and validates it against the project's `audit` settings:
  an over-length subject, a disallowed type, or a malformed shape is drift, while an
  unknown scope is a non-failing advisory note (a plan may be the change that adds the
  scope). Tag a display-only example `commit awf-ignore` to skip that one block. The rule
  is presence-triggered, so bare-fence plans are unaffected; the plans template and the
  writing-plans skill teach the fence in prose.
- `awf context --uncovered` now reports a clean coverage floor (ADR-0110). Every
  code package has a domain home, and the report additionally subtracts awf's own
  generated outputs (`PlannedOutputs`) and a new absent-safe top-level
  `contextIgnore` config key, a list of anchored globs naming genuinely non-domain
  paths (config source, docs, the example adopter, top-level non-code files), so a
  newly-unowned path surfaces as a real signal rather than standing noise. An empty
  or absent `contextIgnore` adds no exclusion.
- Narrow-topic tag taxonomy for precise `awf context` relevance (ADR-0109). Tags are
  redefined as sub-domain topics, never domain-scale buckets: `awf check` now fails
  if any `tags:` vocabulary member equals a configured domain name, and Tier 2 drops
  its domain-name filter (the precise set is the plain union of the Tier-1 tags), so
  a domain-scoped query returns a tight topical cluster instead of a third of the
  corpus. Two advisory, non-failing `awf check` notes flag tag health: a coarsening
  note for any tag on more than 25% of the tag-bearing artifacts, and an
  under-tagging note for any ADR or pitfall with zero tags; both inert under an
  empty vocabulary.
- Governed tag vocabulary and revived ADR/pitfall metadata (ADR-0103). ADR
  `tags:` and `related:` frontmatter (long authored but parsed-then-dropped)
  are now lifted into `adr.ADR`, and pitfall entries gain an optional `tags:`
  field. A new top-level `tags:` config key declares a vocabulary mapping each
  tag to a one-line meaning; when it is non-empty, `awf check` fails on any ADR
  or pitfall tag that is not a declared member and on any member with an empty
  meaning (an empty or absent vocabulary is inert, so tags stay free-form until
  you opt in). `awf check` also now resolves every ADR's `related:` numbers
  against `docs/decisions/`. The key is additive and absent-safe (no schema
  migration) and changes nothing about `awf context` output yet.
- `awf context --uncovered [<scan-root>...]` reports git-tracked-at-HEAD paths
  matched by no configured domain glob: the inverse of the per-path domain
  resolution, and an on-demand signal for where a domain is missing (ADR-0102).
  A fully-uncovered directory collapses to its topmost node; positional args are
  optional scan roots (matched on directory-segment boundaries), while
  `--staged`/`--range` are rejected in this mode. Human and `--json` output derive
  from one result, and the mode reuses `awf context`'s read-only and
  static-fallback contracts.
- `awf context <path>` now surfaces the pitfalls relevant to a queried area
  (ADR-0099): when the toggleable `pitfalls` doc is enabled, it lists each pitfall
  whose own `domains:` owns a queried path (by the entry's tag, like an ADR, not
  transitively like a plan) in both the human and `--json` output, on the same
  read-only `ContextResult`.
- Plans get a machine-readable spine and a uniform authored shape (ADR-0097,
  ADR-0098, ADR-0108). A new `plans-template` singleton renders `docs/plans/template.md`,
  the canonical taxonomy: `date`/`adrs`/`status` frontmatter, a `# Plan:` H1,
  the Goal/Architecture-summary/File-structure header (Goal carries a one-line non-goals
  statement; the template interpolates the project's configured gate command, not a
  hard-coded literal), phases, and
  optional Verification/Notes tails. `awf new plan "<Title>"` scaffolds a
  date-prefixed plan from it (no sequential number). `awf check` now validates
  plan frontmatter (`status` enum; unparseable YAML is a hard error) and
  plan→ADR links (`adrs:` must resolve to real ADRs); the grandfathered
  frontmatter-less corpus is skipped. `awf context` surfaces each plan whose
  `adrs:` links a reported ADR, in both the human and `--json` output. The
  plan convention itself is reframed: task granularity is now "one reviewable,
  logically-coherent change" (not wall-clock minutes), a sanctioned
  coupled-phase escape covers genuinely un-sliceable changes, and plans carry
  a two-state (`Proposed`→`Implemented`) lifecycle that freezes on the plan's
  own `status`, replacing the ad-hoc `# Implementation complete` line. The
  `awf-writing-plans`, `awf-executing-plans`, and
  `awf-subagent-driven-development` skills, the `plan-reviewer` agent, and the
  plans README are reconciled to it. Adopters get it all on their next
  `awf sync`.

### Bug fixes
- Collapsed Pi subagent activity now presents bounded history chronologically: omitted older events,
  hidden retained events, then the visible live event log with the newest event at the bottom.
- Pi child failures now retain bounded progress and diagnostics in tool details while preserving
  error status through Pi's result middleware; intermediate child activity remains outside the
  parent model's visible content (ADR-0125).

### Others
- The generated Pi extension files now carry a `// @ts-nocheck` directive on the line after
  their provenance banner (ADR-0126), so adopter IDEs no longer flag `.pi/extensions/awf-subagents/`
  with errors like `Cannot find name 'Buffer'` when no `@types/node` is resolvable above `.pi/`.
  Every rendered copy stays byte-identical, and the container test lane strips that one directive
  before `tsc` so the static type-check still covers the real extension code. Adopters get the
  reworded output on their next `awf sync`.
- The `adr-lifecycle` skill now states the partial-amendment back-pointer rule, and the
  `adr-reviewer` checks it (ADR-0116). When an ADR overrides a live ADR's Decision item
  without superseding it wholesale, the overridden ADR's `related:` must name the
  overriding ADR in the same commit; previously the skill named only the successor's
  `related:`, so the amended ADR's item read as current guidance with no signal. The
  skill's append-only statements are reworded to match: a live ADR now permits in-place
  edits to `status` **and** cross-reference metadata (`superseded_by:`, `related:`),
  since append-only protects rationale, not bookkeeping. The body stays frozen.
- Invariant backing is documented as a ledger, not a proof (ADR-0114). The marker
  scan is a textual line match with no assertion awareness, so a backed `invariant:`
  slug records that a test is declared to back it, not that the property is proven.
  The ADR-README, the ADR template, and the proposing-adr skill drop the
  "test-proven property" wording; the invariants domain doc gains a ledger-not-proof
  caveat cross-referenced to the coverage doc; a new glossary term defines invariant
  backing; and the `code-reviewer` testing-discipline lens now charges the semantic
  check that a backing test actually asserts the invariant it backs. Adopters get the
  reworded prose and the lens on their next `awf sync`.
- Shipped templates are now gate-checked em-dash-free (ADR-0113). A new gate
  scans awf's embedded templates and fails on the em-dash character (U+2014),
  and the documentation authoring standard gains a plain-punctuation rule.
  The ban is scoped to shipped templates; hand-authored ADRs and plans, and
  adopter-authored parts and sidecar data, are out of scope. Adopters get the
  reworded standard on their next `awf sync`.
- The agent guide's Invariants section is now core-only (ADR-0112). The
  `agents-md-standard.md` authoring guidance gains a decidable retention
  criterion: a rule belongs in the guide only when it is not scoped to a single
  subsystem's files (process, gate, commit-hygiene, the flagship rendering
  guarantee, the toolchain preconditions, and the invariant-backing meta-rule).
  Path- or subsystem-specific invariants stay in their owning ADR and are reached
  on demand via `awf context` and the generated ADR status index; do not mirror
  the ADR ledger into the guide. Adopters get the reworded standard on their next
  `awf sync`; awf's own guide list is trimmed from 84 bullets to 10 to match.
- Em-dashes are removed from the shipped template prose across every skill,
  review agent, doc, the agent guide, and the ADR/plan scaffolds, and from the
  `GENERATED by awf` banner atop every rendered file, in favour of plain
  punctuation (colons, semicolons, commas, parentheses). The `awf:edit` /
  `awf:edit-in-place` provenance-pointer separator likewise changes from an em-dash
  to `: `. The rendered wording is unchanged in meaning; only the punctuation reads
  less machine-set. Adopters get the reworded output on their next `awf sync`.

## [0.16.0] - 2026-07-11

### Features
- The plan convention sanctions a second task form, the **batch task**
  (ADR-0095): for a transformation repeated across many sites, a plan task may
  show one representative diff (plus an edge case, unless the shape is identical
  everywhere), name the affected-site set as an exhaustive list or a reproducing
  command, and a deterministic post-check that fails if any site is missed,
  instead of N near-identical diffs. The `awf-writing-plans` skill, the plans
  README, and the `plan-reviewer` `step-exactness` lens are reconciled so a
  well-formed batch task is not flagged as under-specified. Adopters get it on
  their next `awf sync`.
- Read-only `awf context <path>...` query command (ADR-0092): for a set of
  repo-relative paths it reports their owning domain(s), the invariant slugs
  backed by markers under those paths, and the related ADRs (with each ADR's
  own declared invariants): the deterministic context awf already holds,
  surfaced instead of reconstructed by grep. Human and `--json` output;
  `--staged`/`--range <a>..<b>` resolve the paths from git. Gated and degrading
  to a static notice outside an adopted tree, like `awf config`. The workflow
  skills (`awf-brainstorming`, `awf-reviewing-impl`, `awf-reviewing-plan`) now
  call `awf context` to ground their domain/invariant/ADR context instead of
  reconstructing it by grep.

### Breaking changes
- The config-toggle commands are renamed `awf add`/`awf remove` →
  `awf enable`/`awf disable` (ADR-0093), with no backward-compat alias. The
  verb now matches the operation (toggling an artifact's membership in the
  config enable arrays) instead of implying it creates or deletes something
  (which `awf new` does). Kinds, flags (`--dry-run`, `--with-dependents`), the
  closure/dependent behavior, and `awf list` are unchanged. An adopter's
  rendered `AGENTS.md` and docs switch to the new verbs on their next
  `awf sync`.

### Bug fixes
- `awf <cmd>` now rejects a repeated single-value flag (e.g. `awf audit --base a
  --base b`) with a usage error, instead of silently taking the last value.
  Repeatable flags like `awf init --set` are unaffected.
- `awf enable`/`awf disable` now reject a nameless singleton given a name (e.g.
  `awf enable bootstrap foo`) with a usage error, instead of silently ignoring
  the extra argument.
- `awf enable <kind>`/`awf disable <kind>` with the kind but no name (e.g. `awf
  enable target`) now say "requires a name" instead of the misleading "requires
  a kind" hint that treated the kind as if it were a name.
- `awf help new <kind>` (e.g. `awf help new adr`) now prints the subcommand's
  help; previously only `awf new adr --help` reached it and `awf help new adr`
  printed the group help.

### Others
- CLI dispatch is restructured onto a declarative command table
  (`internal/clispec`) driven by a generic parse-once dispatcher (ADR-0094):
  one path parses arguments, applies the gating classification, and calls the
  handler, replacing the hand-rolled per-command `switch`. `awf new <kind>
  --help` now prints kind-specific help. The resolver's internal
  `Add`/`Remove` vocabulary is renamed to `Enable`/`Disable`, completing
  ADR-0093's deferred rename. The rendered `AGENTS.md` binary-version-gate line
  and the gated-command list in the docs are now generated from the command
  table, so they list every gated command (adding `config`/`context`) and
  cannot drift.

## [0.15.0] - 2026-07-11

### Features
- Project-local custom docs (ADR-0091): `awf new doc <name> "<description>"`
  scaffolds a managed doc (a declaring sidecar plus a `content` convention
  part rendered from awf's base doc template) that joins the AGENTS.md
  document map and the dead-link check like any catalog doc. Names may be
  nested (e.g. `guides/ci`). A new toggleable `releasing` catalog doc
  (`awf add doc releasing`) ships a stub-default release runbook that imposes
  no structure.

### Bug fixes
- `awf sync` now fails loudly when two artifacts resolve to the same output
  path, so a path-aware local doc name colliding with awf's reserved
  `decisions/`, `plans/`, or `domains/` output is caught rather than silently
  overwriting the other file.
- `docs/config-reference.md` (and `awf config`) now document a project-local
  artifact's base data keys when a *synthesized* local skill, agent, or doc is
  enabled: the case where the base template actually renders those keys.
  Previously, for skills and agents the `_base` rows surfaced only for a
  hand-authored `local: true` opt-out (which renders nothing from the base
  template) and never for a `awf new`-created artifact, and for docs they never
  surfaced at all, so a real custom artifact's keys went undocumented.

## [0.14.1] - 2026-07-10

### Bug fixes
- The invariant-backing scan no longer descends into nested checkouts: a
  subdirectory carrying its own `.git` entry (a directory in a primary clone,
  a gitdir-pointer file in a linked worktree or submodule) is another
  repository's working tree, so a marker inside it can no longer silently
  keep this project's invariant "backed": previously a stale session
  worktree under `.claude/worktrees/` could preserve a deleted marker and
  hide an unbacked invariant from `awf check`.

## [0.14.0] - 2026-07-10

### Breaking changes
- The glossary doc is data-driven (ADR-0089): terms live in
  `.awf/docs/glossary.yaml` under `data.terms` as a `term: meaning` YAML map,
  and awf renders the table always sorted (case-insensitive), with `|`
  escaped in cells and content violations (empty terms or meanings, interior
  newlines, non-string values, case-insensitive duplicate terms) failing the
  render with the offending key named. The old `terms` section is gone: an
  authored `.awf/docs/parts/glossary/terms.md` part flags as orphaned drift
  after upgrading: move each table row into `data.terms` and delete the part.
  Framing prose goes in the new empty-by-default `prepend`/`append` sections.
  With no terms configured, the doc renders a placeholder line naming the
  authoring surface.

### Features
- `awf config [<key-or-var>]` (ADR-0088): print the configuration reference
  from the CLI: the full reference or a single entry, with live state inside
  a project (current values, consumers, dormant hints) and a static
  catalog-wide fallback outside one for pre-adoption discovery.
- `docs/config-reference.md`: a generated, always-on configuration reference
  (ADR-0088): every config key, var, sidecar field, and per-artifact data key
  with full descriptions, defaults, availability, and the project's live state
  (which vars are set/empty/absent, what consumes them, what enabling would
  activate). Regeneration-checked like the domain docs; the intro section is
  overridable, the generated tables are not, and `data:` on its sidecar
  refuses at open.
- Deleting a `vars:` key now acknowledges its unset-var note (ADR-0087): the
  advisory fires only for a key that is present with an empty (or null) value
  (the seeded open-to-do state), and an absent key is read as "considered and
  declined", permanently silencing the note for that var. The note text names
  both exits ("set a value, or delete the key to accept the generic prose").
  Deleting a key changes the referenced-var config hash, so expect a one-time
  stale flag until the next `awf sync`; and a var consumed by a part's
  `{{=awf:gateCmd}}`-style placeholder still hard-errors when deleted (the
  placeholder contract is unchanged). Rendering is untouched: absent, null,
  and empty all degrade to the same generic prose as before.

### Bug fixes
- `awf audit` (and every git-reading path) now works from a linked git
  worktree or submodule checkout, where `.git` is a `gitdir:` pointer file
  rather than a directory: the repo open resolves the pointer and routes
  shared state (objects, refs, config) through the worktree's `commondir`.
  Previously it failed with `open repo: ... .git/config: not a directory`.

### Others
- The repository now carries a committed example adopter (`examples/sundial/`):
  a full-surface worked example of an awf adoption, browsable in the repo and
  kept render-synced from awf's source by the repo's own checks (its ADR-0090).
- Dependency refresh: `golang.org/x/crypto` v0.51.0 → v0.53.0 (clears 13
  published SSH-package advisories: none reachable from awf, which only
  reads local git history), plus `x/mod` and `x/tools` to their
  current-minus-cooldown versions.

## [0.13.0] - 2026-07-10

### Breaking changes
- The `.awf/` tree is now closed (ADR-0086): `awf check` fails on any file or
  directory it cannot claim (strays like `.awf/notes.md`, files with the wrong
  extension in kind/parts dirs, parts of a `local: true` artifact) with a
  repair hint per entry, collapsing to the topmost unclaimed directory.
  Sync-written `<path>.awf-bak[.N]` collision backups are flagged as stale
  backups to review and delete (a brownfield adopt is therefore red on its
  first check until the backups are cleared; intended to-do surfacing).
  `.awf/memory/` stays exempt session scratch.
- `awf check` now fails on authored-but-unconsumed configuration (ADR-0086): a
  non-empty `vars:` key no rendered artifact references (`unused-var`), and a
  sidecar `data:` key the artifact's template never reads (`unused-data`): the
  typo that publication-safe degradation used to hide. Empty vars stay legal
  (the init scaffold is unchanged), but note that leftover keys from removed
  catalog vars (e.g. ADR-0084's) are now flagged when non-empty, and disabling
  a render unit (`awf remove hooks`) can strand the var only it consumed;
  delete the key in the same change.
- Inert sidecar fields now refuse at project open (ADR-0086): `paths:` on a
  non-domain sidecar, and anything but `paths:` on a domain sidecar (`data:`,
  `sections:`, `local: true`), fail every gated command with the exact file
  and fix named. These fields were silently ignored before; delete them (or
  move `paths:` to a domain sidecar) and re-run.
- The four prose-knob catalog vars (`docCurrencyTargets`, `adrProposeCommitFmt`,
  `gateDuration`, `modulePrefix`) are removed (ADR-0084): catalog vars now carry
  functional values only (commands, enforced identifiers, structural paths).
  The consuming templates render their former fallback prose unconditionally, so
  a project that set one of these sees the affected skill rewritten to the
  generic wording on its next `awf sync`: no warning is emitted; override the
  section with a convention part to restore concrete wording. Leftover keys in
  `vars:` are inert and can be deleted at leisure, but a saved init answers file
  (or `--set`) carrying a removed key now fails `awf init` on a fresh scaffold
  with an unknown-answer-key error.

### Features
- Interactive `awf init` now asks for the skill/doc selection first and then
  prompts only for the vars that selection's templates (plus the always-on
  singletons and hook payloads) actually reference (ADR-0086); every other
  catalog var is seeded empty as before. `--set`/answers-file values are
  honored for any var either way.
- Single-command upgrades: the bootstrap singleton now renders `.awf/upgrade.sh`
  alongside `.awf/bootstrap.sh` (ADR-0085). `bash .awf/upgrade.sh` resolves the
  newest release (or takes an exact version argument), fetches and verifies it
  through the bootstrap, and hands off to `awf upgrade`, closing the
  chicken-and-egg where every upgrade started with a manual binary fetch. The
  bootstrap itself now honors a pre-set `AWF_VERSION` environment override for
  which release to fetch; without one it resolves its pin exactly as before.
  `docs/working-with-awf.md` gains an "Upgrading awf" section covering the flow.
  (This upgrade is the bridging one: the script only exists in your tree after
  upgrading to a release that ships it; use
  `AWF_VERSION=<new> bash .awf/bootstrap.sh`, then `<printed path> upgrade`.)

### Bug fixes
- `awf upgrade` now always ends in a sync, even when no schema migration
  applies (ADR-0085): a same-schema binary bump re-renders every managed file
  and re-pins the bootstrap. Previously a template-only release left the
  rendered output stale until the next unrelated sync. The no-op message is
  now `config already at schema N`, followed by normal sync output.

### Others
- `awf sync` (and every command ending in a sync) now prints one provenance
  line per file whose rendered output actually changed, classifying the cause
  from the lock's hashes: `changed <path> (template)` for upstream template
  churn, `(config)` when your own vars/sidecars/parts caused it,
  `(template+config)` for both, `(internal)`/`(regenerated)` for non-hashed
  inputs (the pinned binary version; the generated decision indexes), and
  `added <path>` for newly shipped files: the triage signal for reviewing a
  large upgrade diff. A byte-identical re-render stays silent, and a first
  sync into a fresh project reports nothing.
- The rendered `docs/workflow.md` local-hooks section now documents the
  stub-as-override-point pattern: hook payloads are deliberately
  all-or-nothing, and a project-specific deviation (e.g. a docs-only fast
  path) belongs in the stub you own, commented as a deliberate deviation,
  keeping the payload canonical and sync-updated.

## [0.12.0] - 2026-07-09

### Breaking changes
- The catalog `requires*` declarations are now an enforced dependency graph
  (schema 8; run `awf upgrade`). A config enabling an artifact without its
  required skills/agents/docs is refused by every command; the migration
  closes your enabled set (adding missing requirements, printing each) and
  drops dormant doc-gated skills (enabled while their doc was disabled;
  they rendered nothing before, so your output is unchanged). `awf add`
  now enables the full requirement closure in one edit, printing a plan;
  `awf remove` refuses while enabled artifacts still require the target;
  `--with-dependents` removes them together, `--dry-run` previews either
  plan. `awf init` follows the same rule: a trimmed selection is
  closure-completed (missing requirements added, each printed) and the
  scaffolded agent set derives from the trimmed skills instead of always
  enabling every agent. The render-time suppression of doc-gated skills
  is gone: enabled now always means rendered.

### Others
- `awf check` and `awf init` now print a non-failing note when a convention
  part contains a whole line that is (or begins with) a section marker
  (`<!-- awf:section ... -->` / `<!-- awf:end -->`) which is inert inside a
  part and previously rendered into output silently. Inline quoting and
  fenced code examples never trigger the note; fencing is the remedy the
  note itself suggests.
- `awf sync` (and every command that ends in a sync: `upgrade`, `init`,
  `add`, `remove`, `new`) now prints `awf sync: pruned <path>` for each
  file its prune actually removes: a disabled artifact, a dropped
  target's tree, or a path relocated across versions no longer disappears
  silently. A routine re-sync still prints nothing.
- `awf upgrade` migrations now print one provenance line per config
  operation: the schema-6 migration reports each relocated sidecar/parts
  directory and each doc it strips from `docs:`, and the schema-7 migration
  reports each glob it anchors, matching the schema-8 migration's existing
  per-op lines, so an upgrade's config changes are readable from the output
  instead of the diff.
- Shipped templates no longer cite awf's own decision records: the agent
  guide's commit-scope bullet, the working-with-awf command overview, and
  the bootstrap comments drop their `ADR-NNNN` citations, and the
  working-with-awf glob examples switch from awf's repo layout to a neutral
  `src/` project. A source-level scan now bans concrete ADR citations and
  unexempted repo-identity literals in every template, all branches included.
- The bootstrap script's unsupported-OS/arch failure now points at the
  manual-install path (`https://github.com/hypnotox/agentic-workflows#install`),
  so Windows/git-bash users see the way forward instead of a bare error.
- The catalog now declares each skill's and agent's unconditional chain-skill
  coupling (`requiresSkills`), and the standard's test suite enforces the
  declarations both ways (undeclared reference and stale declaration each
  fail). Data only: no CLI or rendering behavior changes.

## [0.11.0] - 2026-07-08
### Breaking changes
- One anchored path-glob dialect everywhere (ADR-0077, schema 7; run `awf upgrade`). Every
  glob (`invariants.sources[].globs`, `audit.dependencyManifests`, and the new domain
  `paths`) now matches a file's full slash-separated repo-relative path: `*.go` means
  top-level `.go` files only, any-depth is written `**/*.go`, and path patterns like `cmd/**`
  or `internal/audit/*.go` are now legal. The migration rewrites existing no-slash patterns
  to `**/<pattern>`, so migrated configs behave exactly as before.
- A present-but-unreadable `.awf/awf.lock` is now a hard error in every command (ADR-0076),
  with one recovery hint: restore the lock from version control, or delete it deliberately
  to re-adopt. Previously an unparseable lock silently skipped the version sub-check
  (ADR-0039 Decision 5, partially superseded), read as schema-current to `awf upgrade`,
  and made `awf sync` treat every rendered file as foreign. A *missing* lock keeps its
  existing semantics everywhere.

### Features
- Domain territories and the `domain-code-staleness` audit rule (ADR-0077): a domain sidecar
  `.awf/domains/<name>.yaml` may declare the domain's file territory as anchored path globs
  under `paths:`; when a branch changes matching files without refreshing
  `.awf/domains/parts/<name>/current-state.md`, `awf audit` raises an advisory Warning,
  closing the ADR-less half of the domain-doc currency gap (ADR-0019 covers the ADR-driven
  half). Opt-in per domain; disable via `audit.domainCodeStaleness: false`.
- Trust-bearing writes are atomic (ADR-0076): `.awf/awf.lock` and migration rewrites of an
  existing `.awf/config.yaml` go through a same-directory temp-file-plus-rename helper, so
  an interrupted process can no longer leave a truncated lock or config.
- The agent guide's working-memory check is now on-demand (ADR-0075). The rendered guidance
  no longer tells the agent to check `.awf/memory/` on *every* start of work; instead it reads
  memory when the request implies earlier work to continue, or as a safety net when a fresh or
  context-compacted session finds `.awf/memory/` non-empty and unaccounted-for, and skips the
  check for a self-contained request. The resume-discipline (match → resume; ambiguous → ask;
  never silently resume a stale effort) is unchanged. Partial-item supersedence of ADR-0069
  Decision item 5; ADR-0069 stays Implemented.
- Review agents are now report-only (ADR-0074): the three reviewer subagents
  (`adr-reviewer`, `plan-reviewer`, `code-reviewer`) emit findings and a digest but no longer
  edit, commit, or re-review. The `<prefix>-reviewing-adr`/`-plan`/`-plan-resync`/`-impl`
  skills now own fix application, routing findings by classification (mechanical directly /
  reasoned with a one-line rationale / user-decision escalated), and run exactly one fresh
  verify-pass dispatch instead of the retired agent-side 3-round soft cap. Restores reviewer
  independence (a judge that never edits what it judged) and makes fix application visible on
  the main thread. Backed by the `reviewers-report-only` invariant.
- Convention parts can re-inject their section's own rendered default via the new
  `{{=awf:sectionDefault}}` sandbox placeholder (ADR-0072). Placing it in a convention part
  splices the overridden section's rendered default at that point, so a part can *extend* a
  shipped default (preamble, appendix, or wrap) instead of copying and forking it (which
  silently rots when awf revises the default). A part still replaces its section body; the
  placeholder just carries the default forward. Re-injecting a `stub` section's default (an
  authoring prompt) is a hard render error. Documented in the working-with-awf overrides
  section and placeholder key table.

### Others
- The rendered working-with-awf doc's command list now covers `awf uninstall` and
  `awf version`, and its `sectionDefault` key description states the stub re-injection
  failure mode precisely (a hard render error, not a silent skip).
- The rendered ADR-README's `supersedes:` example now models a bare int (`[1]`) instead
  of a zero-padded one (`[0001]`), which YAML-1.1 parsers read as octal.
- The two plan-execution skills' terminal-handoff line now attributes finding
  classification to the report-only review agent (ADR-0074): the reviewing skill routes
  findings by the agent's classification rather than "classifies" them itself.
- The plan-resync skill's verify-pass step now states which rule wins when the single
  verify pass surfaces an ADR-implicating residual: the amend-and-re-review return edge
  applies to initial-dispatch findings only, so verify-pass residuals escalate as
  `user-decision` items instead of re-entering the loop.

### Bug fixes
- A corrupt lock can no longer trigger `awf sync`'s backup storm: sync refuses before
  rendering or writing anything, so no spurious `.awf-bak` files are created and pruning is
  never silently skipped (ADR-0076). `awf check` and `awf uninstall` report a corrupt lock
  truthfully instead of "no lock"; `awf init` reports the lock error instead of listing
  every rendered path as a collision.
- `awf upgrade` no longer prints "already current" when the binary is behind the tree's
  schema (it gives the version-gate guidance) or when run outside any project; any
  project-requiring command that finds no `.awf/config.yaml` now says
  "not an awf project (run `awf init`)" instead of a raw file-not-found error (ADR-0076).
- ACTIVE.md and domain-index generation now group every ADR whose status carries the
  lifecycle convention's suffixed form (`Superseded by ADR-NNNN`) under one `Superseded`
  section, ordered by the status ranking. Previously the suffixed status never matched the
  bare `Superseded` ranking entry, so each successor minted its own alphabetical section.
  Entry lines keep the full status, so the successor stays visible per ADR.

## [0.10.0] - 2026-07-07
### Breaking changes
- The canonical workflow chain gains a terminal `retrospective` step, and the `reviewing-impl`
  skill now names `<prefix>-retrospective` unconditionally (ADR-0067). An existing project must
  enable the new Core skill after upgrading (`awf add skill retrospective`) or the next
  `awf check` fails with a dead skill reference from `reviewing-impl`.
### Features
- New `retrospective` chain skill (ADR-0067): a main-thread terminal step after `reviewing-impl`
  that reflects on the finished effort and routes recurring, codifiable findings up a four-rung
  promotion ladder: project invariant, gate test/lint rule, code-reviewer focus item,
  pitfalls entry. First-occurrence observations are noted rather than promoted, and promotion
  is never delegated or auto-applied unverified.
- Project-local skills and agents (ADR-0068): a project may enable skill/agent names outside
  the standard catalog by declaring a sidecar (`.awf/skills/<name>.yaml` /
  `.awf/agents/<name>.yaml`) and authoring a single `content` convention part; awf renders the
  artifact from an awf-owned base template per kind, with `{{=awf:key}}` placeholders available
  and publication-safe degradation for unset values. `awf new skill|agent <name> "<description>"`
  scaffolds the sidecar, starter part, and enable entry in one step; `awf list` shows local
  artifacts alongside their state. Local names may not shadow catalog names, and `local: true`
  keeps its existing meaning (fully hand-authored file, no rendering).
- Working-memory convention for chain session continuity (ADR-0069): `awf sync` now always
  renders a self-ignoring `.awf/memory/.gitignore`; the agent guide gains a working-memory
  section (per-effort `.awf/memory/<effort-slug>.md` files, resume protocol, JIT-retrieval
  guidance); brainstorming checkpoints its design brief continuously; the chain skills plus
  bugfix/debugging checkpoint phase/handoff state; the retrospective deletes the file.
- Must-replace template defaults are now declared with a `stub` attribute on their section
  marker, and `awf new`'s starter parts open with a whole-line `<!-- awf:stub -->` marker.
  `awf check` and `awf init` print a non-failing note per artifact with unauthored stub
  content; a stub section's rendered pointer reads `stub; replace by creating <path>`
  (ADR-0070). Upgrading re-renders every artifact whose template was swept; expect one large
  `awf sync` commit.
- A malformed `awf:section`/`awf:end` marker is now a hard render error instead of leaking
  verbatim into rendered output (ADR-0070).
- Plans must be phase-standalone: the writing-plans skill and the plans README now require every
  phase's closing commit to pass the project's gate on its own (each definition lands in the
  phase that first uses it), and the plan reviewer's executability lens checks the same rule.
### Bug fixes
- `awf check` now reports an enabled artifact whose output file was never synced (the drift scan
  previously iterated only lock entries, so an artifact enabled by hand-editing
  `.awf/config.yaml` was invisible until the next sync), and flags orphaned singleton convention
  parts under `.awf/parts/`: a typo'd section name or unknown kind directory was silently
  ignored.
- The sync prune and `awf uninstall` now skip lock entries that are not local relative paths; a
  corrupted or malicious `.awf/awf.lock` entry could previously delete a file outside the repo
  and then hang walking parent directories.
- `awf upgrade` no longer loops unrecoverably on a lockless pre-relocation (`.claude/awf/`)
  tree: generation detection anchors to the relocation migration instead of drifting upward as
  newer migrations register.
- `awf add`/`awf remove` enforce the binary-version gate before rewriting `.awf/config.yaml`; a
  stale binary previously failed only inside the chained sync, leaving a half-mutated config.
- `awf audit` fixes: a merge commit no longer attributes the merged-in branch's whole diff to
  the branch under audit; unparseable ADR frontmatter is surfaced as an `adr-frontmatter`
  finding instead of silently disabling the status-cochange rule; and the commit-subject length
  limit counts characters, not bytes.
- Dead-link check fixes: a badge-wrapped link (`[![CI](ci.svg)](docs/x.md)`) now has its outer
  destination checked; an angle-bracket target containing spaces (`[spec](<my file.md>)`)
  unwraps before checking; root-relative `/docs/...` targets resolve against the repo root; and a
  target escaping the repo root is dead by definition instead of depending on host contents.
- Invariant backing now requires the marker comment to open its line (after indentation), so a
  marker-shaped string inside a literal (e.g. a test fixture) no longer silently backs a slug;
  the rendered tagging guidance states the own-line contract.
- `awf init` with an existing config no longer walks through interactive prompts it then
  discards (it says it is keeping the config and flags ignored `--set`/`--answers` values),
  and unknown answer keys or out-of-options enum values are rejected instead of silently
  no-op'ing.
- Replayed migrations on a degraded lock no longer strip a modern `hooks:` mapping or overwrite
  an explicit bootstrap opt-out with the upgrade default.
- `awf new` refuses to overwrite an existing local artifact's sidecar or content part; a
  declared-but-disabled name was previously reset to the scaffold stub without warning.
- Unset-var notes now report each base-shared local artifact independently and are labeled by
  artifact path; previously all local artifacts collapsed onto one note keyed by the shared
  base-template id.
- Three skill descriptions no longer render a hardcoded article before the skill prefix
  ("a awf ADR" for vowel-initial prefixes).
### Others
- The agent guide's task-skill sentence now derives from the catalog via a `Chain` flag on the
  ten progression nodes, so enabled non-chain skills (e.g. `refactor-coupling-audit`) appear in
  the rendered guide instead of a hand-enumerated list.

## [0.9.0] - 2026-07-05
### Bug fixes
- `awf check` / `awf sync` now reject an `invariants.sources` entry that carries a comment marker
  but no globs: such a source scans no files, and was previously accepted silently (ADR-0064
  follow-up).
### Others
- ADR-system invariant-tagging guidance (`docs/decisions/README.md`) now derives its comment
  marker from `invariants.sources` instead of a hardcoded `//`: the adr-readme template renders
  the glob→marker mapping (via `.invariantMarkers`, degrading to marker-agnostic prose when no
  sources are set), and editing `invariants.sources` reflags the guidance (ADR-0064). Two new
  override placeholders (`invariantMarkerSentence`, `invariantMarkerTable`) are documented in
  the working-with-awf placeholder table.
- `awf init` no longer prompts for `invariantsMarker` / `invariantsGlobs` or accepts
  `--set invariantsMarker=...`; configure `invariants.sources` in `.awf/config.yaml` directly. The
  out-of-box default is unchanged (both descriptors defaulted empty, seeding no invariants
  config), so only the interactive/`--set` seeding path is removed (ADR-0064).
- Internal: the standard's catalog moves from an embedded `catalog.yaml` parsed at runtime to a
  compile-time Go value (`catalog.Standard`), and the toggleable docs and always-on singletons
  merge into one `DocEntry` collection from which every projection derives, so adding a mandatory
  doc is a single entry instead of ~6 hand-edited sites (ADR-0060, ADR-0061). Rendered output is
  byte-identical; no adopter migration or schema change.
- The `AGENTS.md` document map now renders its mandatory-doc lines from the catalog rather than
  hardcoded template lines, so a new mandatory doc appears with no template edit (ADR-0062). The
  four mandatory lines reorder to alphabetical and drop their trailing periods: the only
  adopter-visible output change.

## [0.8.0] - 2026-07-05
### Features
- Granular, domain-aligned commit scopes: `audit.allowedScopes` expands from `[adr, awf, plans]`
  to eight domain-named scopes, and each entry may carry a `{name, meaning}` mapping so the scope
  taxonomy renders from config (ADR-0055, ADR-0056).
- Convention parts can splice awf-derived values via the `awf:`-namespaced placeholder syntax: a
  dynamic, non-empty-only registry (scope list/table/sentence, prefix, gate commands),
  hard-error guards, and a backslash escape for documenting the syntax (ADR-0057, ADR-0058).
- New mandatory `working-with-awf` usage doc rendered into every project: a post-adoption guide
  to the CLI, overrides, the placeholder registry, and the sync/check loop (ADR-0059).
### Others
- The agent guide's commit-scope prose and the `docs/workflow.md` taxonomy table now derive from
  `audit.allowedScopes`; editing scopes reflags them (ADR-0055, ADR-0057). The guide's
  `awf-setup` section now points at the new usage doc rather than carrying the whole reference.

## [0.7.0] - 2026-07-04
### Breaking changes
- The brainstorming skill's terminal-handoff section is renamed from `terminal-handoff` to
  `terminal-step` for uniform chain-handoff naming (ADR-0054). Its rendered prose is unchanged,
  but any override at `.awf/skills/parts/brainstorming/terminal-handoff.md` must be renamed to
  `terminal-step.md` to keep applying.
### Features
- Add a `Red flags` rationalization-guard section (a "Rationalization | Reality" table) to the
  `tdd`, `debugging`, `executing-plans`, and `subagent-driven-development` skills, each
  overridable via `.awf/skills/parts/<skill>/red-flags.md`.
### Others
- Add a deterministic golden-task eval suite (`internal/evals`) that renders every catalog skill
  and agent through a full `Project.Sync` and asserts cross-artifact workflow-chain seams:
  forward handoffs name their successor on an invocation-verb line, and the chain graph is
  connected and reachable from `brainstorming` (ADR-0053, ADR-0054). Test-only; no change to
  rendered output or CLI behavior.
- Enforce `skill-section-parity`: every catalog skill/agent template's `awf:section` markers must
  match its declared sections, so a section rename can no longer half-land with a blank override
  path (ADR-0054).

## [0.6.2] - 2026-07-03
### Others
- The code-review agent's universal correctness lens is now paradigm-neutral: "race conditions,
  missing locks" broadens to "concurrency hazards (data races, unsynchronised shared state)" and
  the storage-layer concurrency clause is dropped: a project with a storage layer re-adds those
  traps via the reviewer sidecar's project-focus data.
- Add a general `awf:include` template-partials directive (awf-owned embedded partials under
  `templates/partials/`, spliced before section parsing, with the drift hash computed over the
  expanded source so a partial edit still flags dependent artifacts stale) and use it to
  deduplicate the review-discipline spine shared by the three reviewer agents. An awf-internal
  change: rendered template output is byte-for-byte unchanged (ADR-0052).

## [0.6.1] - 2026-07-03
### Bug fixes
- Converting a managed skill or agent to `local: true` no longer deletes its file on the next
  sync. The prune step now preserves every declared local artifact's output path, so a
  managed→local conversion keeps the hand-authored file instead of breaking later syncs with
  "local skill file absent".

## [0.6.0] - 2026-07-03
### Breaking changes
- The three standard docs (`workflow`, `doc-standard`, `agents-md-standard`) are now mandatory
  always-on singletons instead of toggleable catalog docs; config schema migrates to
  generation 6 (ADR-0043). Run `awf upgrade` after updating.
- The rendered bootstrap moves off the repo root into the config tree at `.awf/bootstrap.sh`
  (ADR-0047); update any hook or CI reference to the old `awf-bootstrap.sh` path.
- The `commitScope` var is removed: commit scopes now live only in `audit.allowedScopes`, set
  at init via the comma-separated `commitScopes` answer, quoted by the reviewing skills from
  the same storage `awf commit-gate` enforces, and folded into the drift signal (ADR-0051).
  A leftover var entry is inert; set `audit.allowedScopes` and re-sync to keep the prose.
### Features
- Render three inert git-hook payload scripts (`pre-commit`/`commit-msg`/`pre-push`) under
  `.awf/hooks/` via a `hooks` singleton: enabled by default at init, toggled with
  `awf add/remove hooks`; awf still never touches git config (ADR-0048).
- Add `awf new adr`, scaffolding the next sequential ADR from the rendered template (ADR-0042).
- Add `awf changelog` with `--version`/`--since`/`--range` filters over an embedded changelog
  (ADR-0041).
- `awf add domain` scaffolds the domain's `current-state.md` convention part alongside the
  config edit.
- The rendered workflow doc gains gate-composition and CI-backstop sections.
- Every var/data interpolation degrades to coherent generic prose when unset (an empty
  `awf init` renders publication-safe output), and `awf check`/`awf init` print advisory
  notes for referenced-but-unset vars (ADR-0045).
- `awf check` fails on a rendered reference to a catalog skill outside the enabled set, and
  templates can read the enabled-skill set to conditionalize prose (ADR-0046).
- Reviewing skills and their reviewer agents are pair-validated: `awf add skill` enables the
  missing agent, `awf remove agent` refuses while an enabled skill requires it, and gated
  commands fail on an unpaired config (ADR-0050).
- `awf init` refuses collisions before asking a single prompt, prints unset-var notes and a
  next-steps block after rendering, and falls silent when stdin hits EOF instead of
  streaming the remaining prompts.
- Bare `awf list` shows all seven kinds (including targets, bootstrap, and hooks), and
  `awf help <command>` prints that command's help text.
- The rendered `AGENTS.md` Commands section shows a self-describing placeholder when no
  commands are configured and de-duplicates identical command values.
### Bug fixes
- Single-source the binary version on `project.Version` so the version gate, lock stamp, and
  bootstrap pin cannot disagree; the bootstrap prefers a matching local binary, prints only
  the binary path on stdout, and falls back to `shasum` where `sha256sum` is missing
  (ADR-0049).
- Anchor the rendered skill-reference scanner on a token boundary, so prose like
  `example-bootstrap.sh` no longer trips the dead-skill-reference check.
- Restore the ADR title heading dropped when a project overrides the ADR template's
  sections, and route the generated ACTIVE.md through the canonical generated-by banner.
- The `add`/`remove`/`list` command help now enumerates the `target` kind those commands
  already dispatch, and `awf init` help documents the `--answers` file schema (a flat
  key→value map; multiselect answers comma-joined).
### Others
- Sweep chain-prose seams, tool-specific vocabulary, and repo residue from the rendered
  templates; hook-command descriptor options no longer suggest unpinned `awf` invocations;
  the `domains` frontmatter guidance now scopes itself to projects with configured domains.
- The `adr-lifecycle` skill drops the `Proposed→Deferred`/`Proposed→Declined` commit templates
  (states outside the default 4-state lifecycle) and rewords deferral as a Context amendment on
  a still-Proposed ADR; `refactor-coupling-audit` aligns its scope-shrink rule with that
  amendment form, and `reviewing-plan-resync` gains the ADR-amendment return edge.

## [0.5.1] - 2026-07-01
### Bug fixes
- Fix `awf audit`/`awf check` failing to open a repository with `extensions.worktreeConfig` set (a
  flag `git worktree add` can leave behind) due to an upstream go-git bug; also make the internal
  `.git` path join portable across OSes.

## [0.5.0] - 2026-06-30
### Breaking changes
- Add a self-pinning `awf-bootstrap.sh` installer singleton (toggle with `awf add/remove
  bootstrap`), pinned to the exact rendering binary's version and checksum-verified before
  install; config schema migrates to generation 5 (ADR-0040). Run `awf upgrade` after updating.
### Features
- Add a binary-version compatibility gate: `sync`/`check`/`invariants`/`audit`/`list` now refuse
  to run when the awf binary is behind the project's schema generation or recorded release
  version (ADR-0039).

## [0.4.0] - 2026-06-29
### Breaking changes
- Multi-target rendering goes live: adapter artifacts (skills, agents) now render once per
  enabled adapter via a `targets` config array (default `[claude]`), replacing the implicit
  Claude-only output path (ADR-0037, ADR-0038). Run `awf upgrade` after updating.
### Features
- Add a Cursor adapter (`.cursor/skills`, `.cursor/agents`, no `CLAUDE.md`-style bridge: Cursor
  reads `AGENTS.md` natively); manage adapters via `awf add/remove/list target <name>` (ADR-0037).
- Skill and agent prose is now tool-agnostic (neutral vocabulary instead of Claude Code-specific
  terms), so it reads correctly under any adapter (ADR-0038).

## [0.3.1] - 2026-06-29
### Others
- Sharpen the rendered workflow doc's guidance to explicitly name `awf check` as the pre-commit
  drift guard your own gate must run, rather than vaguely "your check and gate commands".

## [0.3.0] - 2026-06-29
### Features
- Convention-part bodies now render as raw input (never template-interpolated), closing a class
  of accidental-`{{`-breakage bugs (ADR-0034).
- `awf sync` now backs up any foreign file it would otherwise overwrite to a free
  `<path>.awf-bak[.N]` sibling, so adopting awf into an existing repo no longer risks silently
  clobbering unrelated files (ADR-0035).
- Add `awf commit-gate`, the deterministic, blocking counterpart to `awf audit`'s advisory
  Conventional-Commits rule; wire it into your own `commit-msg` hook (ADR-0036).
- Add `--help`/`-h` support to every subcommand.

## [0.2.0] - 2026-06-29
### Breaking changes
- Remove the `hook` artifact kind entirely; config schema migrates to generation 4, and awf no
  longer installs or manages git hooks (ADR-0032). Run `awf upgrade` after updating.
### Features
- Add an invariant-retirement mechanism: a successor ADR can now formally retire a predecessor's
  invariant tags via `retires_invariants` (ADR-0031).
- Add an opt-in local-hooks section to the rendered workflow doc, describing how to wire your own
  hooks now that awf doesn't (ADR-0032).
- `awf audit` gains a rule flagging an ADR status change whose per-domain index wasn't
  regenerated in the same commit (ADR-0033).

## [0.1.0] - 2026-06-28
_Initial public release._
### Features
- `awf init`/`sync`/`check` render a `.awf/`-configured tree of skills, review agents, docs, and
  the `AGENTS.md` agent guide from embedded templates into a project, with drift detection
  against a schema-versioned lock.
- `awf add`/`remove`/`list` manage which skills, agents, docs, and domains are enabled.
- `awf audit` reports advisory workflow-conformance findings (Conventional Commits, ADR/index
  co-change, and more) over a branch's git history.
- `awf upgrade` migrates a project's `.awf/` config tree across schema versions.
- Ships as prebuilt cross-platform binaries (linux/darwin/windows × amd64/arm64), with
  `go install` as a source fallback.
