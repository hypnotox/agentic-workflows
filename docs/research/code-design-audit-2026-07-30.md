# Full-Repo Code-Design Audit (2026-07-30)

*A full audit of the awf codebase to identify the patterns and decisions, repo-wide
and smaller-area, that follow-up efforts can turn into code-design guidance and then
apply to legacy code. Compiled 2026-07-30 from nine parallel fresh-context audit
passes (seams and mechanism access; state and lifetime; error handling and
diagnostics; package topology and duplicated policy; API shape and type modeling;
the cmd/ CLI layer; test design; filesystem, path, and process conventions;
templates and rendering). Every material claim below carries file:line evidence
verified against the working tree on the audit date; treat line numbers as
current-as-of writing.*

Existing authorities were treated as settled grounding, not audit targets:
`code-design/dependency-composition` (ADR-0178, Implemented),
`code-design/state-ownership` (ADR-0180, Proposed; its Consequences already
designate `internal/project` package cohesion/decomposition as the next
code-design decision), and the maintainable-code-design posture doc
(ADR-0168/0169). Findings below either extend those authorities, inventory their
legacy backlog, or propose new pattern territory.

Ten concrete defects found in passing (compile breaks, silent overwrites, a gating
test hole, fault-erasing reads) are deliberately excluded from this document; they
need fixes, not decisions, and are tracked in the dedicated cleanup effort
`code-design-audit-defect-cleanup`.

This document is the input to a sanctioning pass: each candidate decision below is
sized and prioritized so it can be accepted, deferred, or declined explicitly, and
the accepted ones routed through the brainstorm/ADR chain.

---

## The meta-finding: claimed concerns stay clean, unclaimed concerns fork

The repository already carries roughly seven ad hoc single-source claims
(`kind-dispatch-single-table`, `cli-command-spec-single-source`,
`catalog-go-single-source`, `git-range-parser-single-definition`,
`frontmatter-split`, `audit-shares-adr-parser`, `commit-gate-shared-rule`).
Every concern that got such a claim is consolidated and clean. Every comparable
concern that did not has silently forked into 2-6 parallel implementations, and in
at least four cases the copies have already diverged in behaviour (fence scanning,
cleanliness oracles, severity vocabularies, scanner-gate scannability rules).
"Single home for a concern" is currently decided case-by-case at ADR time rather
than being a standing obligation. That is the strongest evidence in this audit
that the nail-down-then-transition program works, and that its coverage, not its
mechanism, is the gap.

Two systemic tensions surfaced alongside it:

- **The 100% coverage gate actively incentivises forking.**
  `internal/plan/plan.go:81-85` documents that its fence scanner "deliberately
  drops `~~~` to avoid an uncovered branch". A shared full-fidelity implementation
  creates branches a narrow consumer cannot exercise, so each consumer writes a
  cheaper divergent copy. Sanctioning direction (2026-07-30): code quality and
  architecture outrank escape minimization. A shared implementation is never
  forked, narrowed, or degraded to avoid an uncovered branch; the accepted costs
  are a reasoned `// coverage-ignore` or exercising the branch through the owning
  package's tests. The decisions below inherit this as a settled premise, to be
  encoded as a claim by the first ADR that needs it.
- **Single-source claims scoped to `internal/` do not reach `cmd/`.**
  `kind-dispatch-single-table` is scoped "in the project package", and four kind
  facts live just outside it in `cmd/awf` (`list_add.go:110` `isGraphKind`,
  `new.go:31`, `list_add.go:235,248`). Future claims must name every layer the
  concern can reach.

---

## Tier 1: divergence already causing or risking wrong answers

### 1. One native-Git runner with unconditional environment isolation

Seven subprocess sites in five shapes; five bypass `IsolatedGitEnvironment`
(`internal/git/controlroot.go:472-495`), including both paths that touch effort
state. `internal/worktree/git.go:18-31` is a byte-for-byte copy of
`internal/git/controlroot.go:455-468`. Two `branchExists` implementations
(`internal/effort/service.go:100-112`, `internal/worktree/git.go:96-105`), only
one isolated. Two cleanliness oracles at different porcelain versions
(`internal/git/git.go:167-174` at v2 without isolation or stderr capture,
`internal/worktree/git.go:91` at v1 isolated) that can disagree about the same
checkout. Three stderr-enrichment implementations; three seam types for one
concept. `exec.CommandContext` is threaded through five packages, but every entry
point passes `context.Background()` and no production site sets a deadline, so a
git blocked on a stale `index.lock` hangs awf indefinitely, including inside the
pre-commit hook.

**Candidate rule.** Every native-Git subprocess is constructed by one runner in
`internal/git` that takes a context with an explicit deadline, pins the repository
root, applies the isolated environment unconditionally, and translates a non-zero
exit into an error carrying git's stderr; no package constructs
`exec.Command("git", ...)` directly, and the cleanliness oracle has exactly one
definition. Fits ADR-0178's mechanism-adapter frame with no new machinery.

**Size.** 7 call sites, 5 packages; 3 seam types collapse to 1. Three of the nine
audit passes independently ranked this top-3.

### 2. One read-universe contract, with honest errors

`internal/config/config.go:69-72` (`TreeReader`) and
`internal/project/output_plan.go:22-27` (`ProjectTreeReader`) are structurally
identical interfaces with byte-for-byte identical filesystem `ReadFile` bodies.
Config's implementation of `Paths` returns `[]string{}` unconditionally
(`config.go:83`): a contract that lies, sitting on the filesystem-vs-staged seam
that drift correctness depends on. The `(bytes, bool)` port shape folds a
permission fault into "absent" (`output_plan.go:62-67`), and loader twins
duplicate the same capability per universe (`topic.LoadCorpus` vs
`LoadCorpusFromTree`; `currentstate` tree-only vs `adr` filesystem-only;
`internal/project/topics.go:73-79` runs both universes in one function) even
though `snapshot.WorkingTree`/`IndexTree`/`CommitTree` already provide the
abstraction.

**Candidate rule.** One consumer-owned read-universe contract, declared once;
every implementation honours every method; a filesystem load is expressed by
supplying the filesystem implementation, not by a second loader function. Jointly:
a port whose implementation performs I/O returns an error; folding a fault into
absence, false, or a truncated list is permitted only where prior validation makes
the fault branch unreachable, stated at the site (`internal/pathglob/pathglob.go:26-29`
is the in-repo model).

**Size.** 2 interfaces to 1, 4 implementations to 2, roughly 350 lines of twin
loaders consolidated across `config`, `project`, `topic`, `adr`, `currentstate`.

### 3. Error identity, one severity vocabulary, one finding model

Error identity is carried by message text: zero exported error types or sentinels
in the five largest packages (project 99 error constructions, effort 125, topic
79, upgrade 46, migrate 46), compensated by 323 `strings.Contains` assertions on
error text across 15 test packages. Typed errors that exist are never matched
(`worktree.RefusalError`, `effort.CorruptError` with 17 construction sites,
`project.ContextFacetError`: zero `errors.As` consumers between them;
`git.HardSafetyError` is the counter-example done right). `os.IsNotExist` vs
`errors.Is(err, os.ErrNotExist)` splits 33/32, and
`internal/git/controlroot.go:572-580` exists purely to strip wrappers so the
shallow predicate can see through them.

Severity is modeled five incompatible ways with an inverted zero value:
`internal/currentstate/check.go:13-17` has `Error = iota` while
`internal/audit/audit.go:23-28` has `Warning = iota`, and
`internal/project/currentstate.go:404-409` converts between them, so an omitted
`Severity` field silently means the opposite thing per package. The same level
prints as "warn" from one subsystem and "warning" from another. Six finding
structs and duplicated verdict-line grammars sit on top, and the
prosegate/memorycite pair duplicates an entire pinned-exemption gate pipeline at
five layers (types, the four-branch pin-reconciliation algorithm, Format, the
11-step cmd driver, config structs, configspec blocks), already diverged on the
scannability rule.

**Candidate rules.** (a) A cause a caller must branch on is exported as a sentinel
or type with `Unwrap` and matched with `errors.Is`/`errors.As`; tests assert
identity through `Is`/`As` and message text only where the message is itself the
contract; the `os.Is*` predicates are retired. (b) Severity is one ordered type
with the safest rank as zero value, one spelling, one parse function; a finding
type carries only its domain fields; a comment-mirror clone is not a permitted
substitute for sharing. (c) A path-scoped gate with pinned exemptions is one
scan/report/exempt engine parameterised by its detector.

**Size.** (a) is incremental across 15 packages and compounds with every new
package left unfixed. (b) is 5 definition sites, ~15 use sites, 6 packages.
(c) is ~250 production lines collapsing to one engine plus two detectors.
Caution: the in-flight worktree `drop-severity-settings-and-unify-the-rank`
(branch active 2026-07-30) is already implementing (b): it unifies the audit
finding rank and drops the currentState severity settings. Sanction (a) and (c)
independently and scope (b) to whatever that effort leaves open.

### 4. The CLI-layer contract

`cmd/awf` (3,360 production lines) currently owns things `package main` cannot
own safely:

- **Project-mutating transactions, asymmetrically.** `awf new topic` has a
  125-line rollback-capable write transaction (plus 5 package-global seams to make
  it testable, `cmd/awf/new.go:193-201`); `awf new skill` writes 4 files with no
  rollback at all; `init.go:127-161` hand-unwinds three times. 25 `os.*` mutation
  sites total. The current state can lose user data on a mid-write failure.
- **Policy.** Kind facts outside the kind table (above),
  `list_add.go:289-326` (38-line catalog-graph disable-cascade computation),
  `commitgate.go:76-115` (git commit.cleanup=strip semantics).
- **Untyped boundaries.** `ConfigReferenceModel` crosses as `map[string]any` and
  `cmd/awf/config.go:103-110` discards type-assertion failures, so a renamed field
  renders as an empty string with a green gate, while `staticModel()` rebuilds the
  same shape independently.
- **Convention-held gating.** `GatedInHandler` is 8 hand-placed `gate()` calls in
  4 idioms; the enforcement test hardcodes 7 commands and misses one.
- **No declared output contract.** Findings and tool failures share exit 1 (hooks
  cannot distinguish "dirty" from "broken"); `cmdCtx` has no stderr field so awf
  findings go to stdout while the aux binaries split 4-vs-4 on the findings
  channel; three JSON conventions across three machine-readable paths; ~200
  unchecked write sites next to one 30-label accumulating writer.

**Candidate rules.** (a) A `main` package contains argument parsing, wiring,
renderer selection, and exit mapping; a domain rule, catalog computation,
durable-write transaction, or per-kind fact lives in the internal package that
owns the concern, and `cmd/*` never calls `os.WriteFile`/`MkdirAll`/`Remove`.
(b) A model crossing the boundary is a named struct; gating classification has one
execution helper and one clispec-enumerated proof. (c) A declared exit-code and
stream contract (2 = misuse; findings vs operational failure split declared
centrally; findings to stdout, diagnostics to stderr) and one versioned JSON
envelope. The aux binaries' `run(args, stdout, stderr) int` idiom is uniform and
good; the contract formalises rather than replaces it.

**Size.** (a) ~5 extractions, ~400 lines moved; (b) small; (c) ~8 binaries, wide
but mechanical. Highest consequence-to-effort item: the structural gating proof.

---

## Tier 2: high-leverage structure, drift confirmed but not yet biting

### 5. A standing single-home obligation for cross-package policy

The umbrella rule the topology pass proposes: a policy consumed by more than one
package has one declaration in the package that owns it; a second implementation
is a defect, not a variant. Its conversion backlog, each independently evidenced:

- **Markdown block-structure recognition**: six implementations at three fidelity
  levels (`internal/adr/adr.go:178-245` full CommonMark; `internal/adr/format.go:230-255`
  a hand copy that says "mirrors sections()"; `internal/refs/refs.go:28-48` and
  `internal/render/comment.go:26-52` simple toggles; `internal/plan/plan.go:86-110`
  drops `~~~` citing the coverage gate; `internal/migrate/pitfalls.go:73-79`
  backticks only). Whether a document line is authoritative or merely demonstrated
  currently depends on which check reads it.
- **Durable writes**: four mechanisms across ~30 sites with no stated tiering
  (`manifest.WriteFileAtomic`, effort's fsync-and-no-clobber publisher, 17-18
  plain `os.WriteFile` sites, one `O_EXCL`-plus-rollback in cmd); 13 `MkdirAll`
  sites choose modes locally.
- **The spill-notice wire format**: writer (`internal/contextdelivery/delivery.go:90`)
  and parser (`internal/contextspill/log.go:18,38-66`) share no constant; a
  version bump compiles clean and the parser reports "no notice" silently.
- **Template-ID resolution**: four parallel encodings inside `internal/project`
  (descriptor closures, constants, fallback helpers, and an inline reimplementation
  in the drift oracle's input at `output_plan.go:148-153`), while `internal/topic`
  re-reads the same embedded templates it executes; hash authority and render
  authority live in different packages.
- **Owner-validated no-follow I/O**: `internal/effort/safeio*` does it correctly
  across three platforms in ~600 lines; `internal/contextspill/log.go:212-250`
  re-rolled ~40 untagged lines of the same policy (and is the package that broke
  cross-compilation).
- **YAML/frontmatter emission**: parsing is exemplary and claimed
  (`frontmatter-split`); emission is hand-rolled at three sites, and
  `internal/project/agent.go:73-79` `yamlPlainSafe` approximates YAML plain-scalar
  rules with Go (`strconv.Quote`) escaping in the highest-blast-radius output awf
  produces (every adopter's rendered agent frontmatter), while `gopkg.in/yaml.v3`
  is already a dependency.

**Size.** The tree-reader collapse (Tier 1 item 2) is the natural first
conversion; the scanner-gate engine (~250 lines) is the largest mechanical win;
the wire-format fix is ~15 lines. The markdown-scanner decision must name the
coverage-gate interaction explicitly.

### 6. Typed domain identities

The qualified claim ID (`domain/topic:slug`) is the repo's central identity and is
decomposed seven different ways (`strings.Split`/`Cut`/`Index`/`SplitN`/regex
sub-capture) with its grammar re-inlined in five files, one of which
(`internal/adr/declarations.go:18`) is already looser than the others (accepts
leading/trailing hyphens). `topic.TopicID` exists as a struct but every consuming
API takes a string, so callers flatten it at each boundary (~10 `.String()`
map-key sites). Artifact kind carries two vocabularies (singular
`catalog.Node.Kind`, plural `kindDescriptor`/`configspec`) bridged by eight
string-surgery conversions (`kind+"s"`, `TrimSuffix(kind, "s")`) despite
`PluralKind` existing, and dispatch sites compare descriptor fields back to
literals, defeating the declared single table. ADR identity exists as both
zero-padded string and int in one package (`adr.Number string` vs
`Related []int`), with ~15 `%04d` round-trips and the `"ADR-NNNN: "` title-strip
duplicated at four sites.

**Candidate rules.** A `ClaimID` value type parsed once at each ingestion
boundary, with `Domain()`/`Topic()`/`Slug()` accessors; cross-package APIs accept
`ClaimID`/`TopicID`, never a string that happens to hold one. An artifact-kind
type with `Singular()`/`Plural()` methods and one parse function; no string
surgery. One ADR-identity representation with the text form and ordinal as
methods. The in-repo model to copy is `internal/adr/status.go`: unexported
representation, exported predicates, zero literal comparisons outside the package.

**Size.** ClaimID ~25-30 sites across 5 packages; kind ~120 literals across 14
files (mostly mechanical); ADR number ~20 sites. The maintainable-code-design
posture (no mechanical wrapper types) is satisfied: each of these concepts is
re-parsed and re-validated at multiple sites today, which is exactly the
duplicated-policy trigger the posture doc names.

### 7. Path spaces and containment

The slash-space (repo-relative, slash-separated) vs OS-space (absolute, native)
distinction is real, load-bearing, and documented in the glossary, but it is
carried by bare `string` through ~50 exported path parameters with no relativity
documented, crossed by 85 scattered `ToSlash`/`FromSlash` calls with no named
boundary. Containment ("is X inside root") has six hand-rolled predicates with
divergent semantics, including reversed argument orders between
`contextdelivery.containedBy(root, candidate)` and `audit.underDir(path, dir)`,
lexical vs symlink-resolved behaviour undeclared, and `filepath.IsLocal` used at
exactly one of seven opportunities. `project.outputPath` owns the
resident-vs-tracked anchoring policy but is bypassed by 10 raw `p.Root` joins.

**Candidate rule.** Repo-relative paths are manipulated only with `path`, OS paths
only with `path/filepath`; every crossing is an explicit conversion at a named
boundary; every exported path parameter names its space. Containment has one
predicate per space, container-first, stating lexical-vs-resolved. Anchored-path
resolution goes through the single owner of the anchoring policy.

**Size.** Containment: 6 predicates to 2, mechanical. Space discipline: ~20
audit-and-annotate sites if review-enforced, or a small named type touching ~8
packages if compiler-enforced. A full repo-wide path type is not recommended
outside the `internal/project` decomposition.

### 8. Template authoring patterns

- **Include granularity and single-authorship.** `awf:include` is whole-line-only
  with no arguments (`internal/render/include.go:13`), so sentence-level contract
  prose is copy-pasted: the model-tier selection block appears at 29 sites in 11
  files (including both arms of conditionals) and has already leaked into an
  `.awf/parts/` convention part, meaning adopters carry a stale copy. The
  `.awf/efforts/<slug>/memory.md` path literal appears at 34 template sites with
  no `.layout` key, for a path that already moved once (ADR-0175). The
  reviewer-spine dedup is half-done: the agent templates are clean via partials,
  but the four reviewing-skill templates carry a four-way byte-identical
  `classify-route-findings` body, so the finding-classification taxonomy is stated
  in five places.
- **The Go/template prose boundary.** Five Go functions assemble rendered
  markdown or sentence prose (`internal/project/render.go:113-146` builds the
  entire AGENTS.md "Enabled skills" list as one string, including a second,
  Go-side implementation of the ADR-0045 degradation contract with hardcoded
  fallbacks at `render.go:130-133`; plus the gated-commands sentence, two commit-
  scope formatters, and glossary rows). This prose is invisible to template
  review, carries no `awf:edit` pointer, and cannot be overridden by adopters.
- **Root render-context validation.** `.vars` and `.data` (adopter-owned) are
  rigorously extracted and checked; the awf-owned root namespace (12 keys plus
  ad hoc per-family augmentation) has no allowlist, and `missingkey=zero` means a
  misspelled boolean capability flag silently takes the `{{else}}` arm forever
  with a green gate, across 27 conditional sites. This is the highest-consequence
  silent failure mode found in the audit; it ships the wrong protocol arm to a
  runtime and both arms read as plausible prose.
- **Small.** Fallback prose drift (`gateCmd` has 3 spellings across 12 sites; the
  awf-verb chain is open-coded 13 times, one of them 4 levels deep); no FuncMap is
  registered at all; section-name vocabulary splits (`when-to-invoke` x7 vs
  `when-fires` x6 for the same slot); `templates/partials/*.md` execute template
  actions without the `.tmpl` suffix; the Pi TS modules copy-paste four private
  helpers across an existing import boundary under `@ts-nocheck`.

**Candidate rules.** A contract stated at more than one authoring site is authored
once as a partial, and the include mechanism gains the granularity the contract
actually has; every awf-owned path a template names goes through `.layout`; Go
supplies structured values and templates own formatting and fallbacks; the root
render context is a declared per-family set validated at check time, where an
unknown boolean key is a hard error.

**Size.** Include mechanism ~50 lines plus ~29 conversions; layout key plus ~34
edits; prose boundary 5 functions/4 files; context validation ~80-120 lines with
zero template edits if the current 27 sites are verified correct (verify as part
of the work; if any is already wrong, this item moves to Tier 1).

### 9. Test-design decisions

- **The global-seam backlog, quantified and owned.** 13 pure test seams (roughly
  35 package-level seam variables counting the syscall-shaped blocks), ~143 test
  mutation sites split across three incompatible swap idioms (`testsupport.SwapVar`
  83 sites, hand-rolled save/restore ~60, `SetNowForTest` 4), with the measured
  cost that `t.Parallel()` appears 8 times in ~50,000 test lines. The worktree
  named `awf/replace-filesystem-and-global-test-seams` is paused with all 13 seams
  intact; the backlog has no owner. The cheapest large win is `cmd/awf`'s
  process-environment globals (`getwd`/`stdin`/`isInteractive`): ~3 production
  lines (the handler context already carries the fields) frees 69 of 83 `SwapVar`
  sites and all 7 `t.Chdir` sites. The TS test lane (`tools/pi-extension-test`)
  is already fully injection-based and is the working model.
- **Fixture-builder consolidation.** The project-fixture builder is reimplemented
  in 7 packages under 13 names plus 6 leaf-writer duplicates of
  `testsupport.WriteFile`, and the copies have already diverged semantically
  (`csRepo` injects a default ADR that `gitProjectFiles` does not). Largest
  test-LOC reduction available; drives the 2.2-2.5x test-heaviness of
  `internal/project` and `cmd/awf`.
- **Coverage-escape hygiene as a design signal.** 39 byte-identical escape
  justifications in `internal/git/controlroot.go` are one design decision stored
  39 times (a missing extraction); 47 of 502 escapes are spent on test-support
  code and `_test.go` files. Candidate rules: a verbatim-repeated justification is
  evidence of a missing extraction; test-support packages are gate-exempt by
  category and no escape appears in a `_test.go` file.
- **Structure.** In-package vs external `_test` packages are chosen per file with
  no rule (two packages mix both); the `export_test.go` idiom is undeclared and
  duplicated verbatim across `adr`/`plan`. Invariant proof markers are 100%
  uniform in form (516 markers, zero variants) but 96 are indented inside
  function bodies, including one test body proving four claims; a placement rule
  (one marker, one test declaration or one subtest) is a natural `awf check`
  extension.
- **Table packages test their own contract.** `internal/clispec` (0.36x
  test-to-production) and `internal/refs` reach 100% coverage entirely through
  other packages' tests; `internal/evals` is 593 test lines in a package with no
  production code.

### 10. State-ownership follow-on slice (before the project decomposition)

Sites the state pass found outside ADR-0180's named scope, worth settling before
the `internal/project` split so the split does not carry the shapes into new
boundaries:

- **Validation never mutates; defaulting happens once.** `config.Validate()`
  writes defaults keyed on decode-only presence flags (the defect is tracked in
  the cleanup effort; the rule that prevents recurrence is this decision).
  `internal/audit/settings.go:30-62` is the in-repo model.
- **No `init()` completion of authority values.** The repo's only `init()`
  completes `catalog.Standard` two-phase and erases literal-declared data.
- **One construction invariant per type.** Hollow `(&Project{Root: root})`
  literals reach methods that read only `Root`; nothing distinguishes them from
  methods requiring the full invariant, so a future field read nil-panics on one
  path only.
- **Per-operation scope is a parameter.** `worktree.Manager` and `effort.Service`
  store a `context.Context` at construction and reuse it for every later
  operation, including destructive git operations a caller cannot cancel.
- **Thread the derived value (rendered code).** The Pi preference store is the
  strongest instance of ADR-0180's anti-pattern in the repo and lives in rendered
  adopter-facing code: closure-mutable slots, a remembered two-call
  reload/revalidate protocol, and consumers re-deriving from the live holder what
  the caller already computed. The pure derivation function already exists and is
  exported.

Also in this territory: `catalog.Standard` is read directly at 17 sites that
bypass the injected catalog parameter, which makes the ADR-0178 injection partly
decorative until converted.

---

## Sequence later: adopter-breaking, needs awf upgrade migrations

- **Part-path unification.** Four config-tree part-path shapes for one relation,
  split on a catalog boolean adopters cannot see; the sweep classifier enumerates
  order-dependent `len(segs)` arms. Rule: part location is a pure function of
  (kind, artifact, section); toggles change what is enabled, never where parts
  live.
- **Sidecar list-item field vocabulary.** Five spellings for "the text of a list
  item" (`description`/`desc`/`check`/`body`/`item`) plus single-field wrapper
  maps and one heterogeneous list; a wrong guess renders silently empty.
- **`topic --json` versioned envelope.** Three JSON conventions today; effort's
  schema-versioned envelope is the model. Compatibility break to schedule.
- **Section-name unification.** `when-to-invoke` vs `when-fires` for the same
  slot is published override-key API; renaming needs an upgrade rename step
  (zero adopter part files affected in this repo and sundial today, so cheap if
  done soon).

## Feed into the designated internal/project decomposition

ADR-0180 already designates the decomposition as the next code-design decision.
This audit adds evidence that belongs inside that decision rather than beside it:

- **`internal/git` is three disjoint responsibilities** (go-git blob access
  consumed almost exclusively by `snapshot`; subprocess control-root topology
  consumed by effort/worktree/project/migrate; a pure-string range parser consumed
  by cmd tools) behind the repo's highest fan-in (9 importers); `cmd/repoaudit`
  links go-git for 43 lines of string parsing.
- **Export-surface minimization.** `internal/project` exports 86 top-level
  symbols; 47 are never named outside the package; the package has effectively one
  production consumer. `topic` (18/44) and `git` (15/23) follow. The dead-code
  gate sees functions, not types or fields, so this surface is invisible to
  existing gates. Unexport after the split, or symbols get renamed twice.
- **Template-ID ownership** (hash authority vs render authority split across
  `project` and `topic`).
- **Any full path-anchoring type** (Tier 2 item 7's larger half).
- **Report rendering ownership** (Tier 1 item 4a moves ~600 lines of presentation
  down; where it lands is a decomposition question).

## Confirmed consistently fine (checked; no decision needed)

- Import graph: a clean 6-level DAG with zero cycles; one layering violation
  (`config` reads `catalog.SingletonKinds()`, a package global, where the sibling
  `NonMandatoryDocNames(c *Catalog)` shows the injected form).
- `internal/pathglob` as sole glob matcher; `internal/snapshot` as sole tree
  reader; frontmatter parsing; the conventional-commit rule; `internal/clispec`
  as a data-only command table with a both-directions parity test. These are the
  models the candidate rules generalise.
- Mechanism discipline: `time.Now` never called inline (4 hits, all declared clock
  seams); zero ambient `os.Getenv` in production; no shell invocation anywhere;
  no service locator, DI container, or sync primitive in production; ~65 of ~100
  package-level vars are genuinely immutable tables.
- API health: all 5 production interfaces are consumer-side and narrow; zero mixed
  receivers; constructor naming (`New*`/`Open*`) consistent; `internal/adr`'s
  status predicates as the identity-modeling exemplar; effort slugs and commit
  scopes correctly untyped.
- Error-message conventions: the `"<verb> <object>: %w"` wrap grammar is near
  universal; no sentence-cased or punctuated messages; zero string-matching on
  error text in production control flow; `errors.Join` rollback aggregation
  correct at all 8 sites.
- Output safety: no TTY/color handling anywhere (pipe-safe); `usageErr` single
  classifier; the checker-cmd `run(args, stdout, stderr) int` idiom uniform across
  all 8 binaries and mechanically enforced.
- Tests: pure-stdlib assertion style with zero third-party matchers; `t.Helper`
  discipline; the golden-test completeness check is self-policing in both
  directions; environment isolation via `RunIsolated`/`t.Setenv`/`t.Chdir` is
  consistent; no committed-fixture or golden-file drift exists (everything is
  built in-process); `internal/testsupport` is a claimed, clean leaf.
- Templates: cross-skill references 100% catalog-backed or guarded; `awf:edit`
  pointer emission fully centralised; section markers structurally cannot leak;
  the `<no value>` guard covers string interpolation; YAML reading (config,
  topics, ADR frontmatter) is strict-decoded consistently; suspicion of
  duplicated YAML-reading policy was checked and disproven.
- Deliberate, documented duplication that is not drift: `cmd/repoaudit`'s
  severity mirror (ADR-0073) and `releasecheck`'s `unreleasedSection` (ADR-0078).

## Sequencing observations

1. The defect backlog is independent of all decisions and can proceed immediately
   (cleanup effort `code-design-audit-defect-cleanup`).
2. The paused worktree `awf/replace-filesystem-and-global-test-seams` holds zero
   commits ahead of main (an empty placeholder); re-own or remove it as part of
   sanctioning Tier 2 item 9.
3. The in-flight worktree `drop-severity-settings-and-unify-the-rank` is actively
   implementing Tier 1 item 3(b) (verified 2026-07-30: finding-rank unification
   plus dropping the currentState severity settings); sanction 3(a) and 3(c)
   independently and leave (b) to that effort.
4. Tier 1 items 1 and 2 are extensions of ADR-0178's existing frame and need the
   least new conceptual machinery; item 3 is the largest compounding tax; item 4
   contains the only finding that can lose user data.
5. The single-home obligation (Tier 2 item 5) can be one umbrella ADR with a named
   conversion backlog, or its backlog items can ride the nearest concrete decision;
   the coverage-gate premise is settled (see the meta-finding), and new claims
   must be scoped to reach `cmd/`.
6. Everything in "Feed into the decomposition" should wait for that ADR;
   everything in "Sequence later" should wait for a migration-capable release
   window.
