## Components

- **`cmd/awf/`**: CLI entry point; `init`, `sync`, `check`, `list`, `config`, `context`, `enable`,
  `disable`, `new`, `audit`, `invariants`, `commit-gate`, `prose-gate`, `upgrade`, `uninstall`,
  `changelog`, `version` subcommands, dispatched by a generic parse-once driver (`dispatch.go`) over the declarative
  `internal/clispec` command table (ADR-0094). The gated commands enforce the binary-version gate
  (ADR-0010, ADR-0039) before opening the project; the driver pre-gates the always-gated ones,
  while `config`/`context`/`new` gate in-handler after their static-fallback / name-validation check.
- **`internal/clispec/`**: the declarative CLI command table (ADR-0094): each command's flags,
  positional bounds, three-valued gating classification, runner-forwarding disposition, help text,
  and (for `new`) its subcommands, as a data-only importable leaf. `cmd/awf` attaches handler funcs
  to it, while `internal/project` derives the gated-command guidance and managed-runner dispatch
  from the same metadata. Excluded runner commands carry their user-facing safety reason.
- **`cmd/covercheck`, `cmd/deadcodecheck`, `cmd/mutants`, `cmd/pincheck`,
  `cmd/releasecheck`, `cmd/repoaudit`**: repo-only gate, release, triage, and audit
  helpers: the 100% statement-coverage floor (ADR-0012), the dead-code gate
  (ADR-0063), the advisory mutation-survivor report (ADR-0066), the workflow-pin
  check (ADR-0079), the release-time changelog pin (ADR-0078), and the repo-local
  conformance audit (`./x audit-local`, ADR-0073: changelog-entry Errors plus
  coverage-ignore re-evaluation Warnings). Not part of the rendered standard.
- **`internal/config/`**: owns `.awf/config.yaml`: the schema and strict load, its construction
  (`MarshalSkeleton`) and typed mutation (`SetArrayMember`, `SetArray`, `SetMappingScalar`, and
  `SetMappingInteger`, comment-preserving `yaml.Node` round trips) behind one `encode` funnel
  (ADR-0026; `internal/migrate` excepted), plus keyed sidecars. The `currentState` block configures
  strict topic validation, coverage/fan-out policy, and the positive default-20 non-failing
  `maxClaimsPerTopic` topic-size advisory; it is the authority the current-state model reads. The
  legacy `invariants` block it replaced was dropped at the project-atomic cutover.
  `IsSingletonKind` classifies off `internal/catalog`'s `SingletonKinds()` (ADR-0043, ADR-0061).
- **`internal/catalog/`**: declares the available skills, agents, docs, and their sections as a
  compile-time Go value (`catalog.Standard`; ADR-0060), with no runtime parse. Its core `exploring`
  skill provides six-target semantic rendering: Pi dispatches the structured extension tool while
  other targets request native fresh-context delegation. Docs and always-on
  singletons are one `Docs` collection: each `DocEntry`'s `Mandatory` flag marks a singleton, and
  `SingletonKinds()` derives from it (ADR-0043, ADR-0061).
- **`internal/adr/`**: parses the decisions directory and exposes it as one `Corpus` view,
  constructed once per invocation and threaded to every consumer (ADR-0130). The view answers
  ADR questions rather than exposing fields to re-derive them from: status predicates on the
  parsed record, declared invariant slugs, existence and metadata lookups, an enumerated
  raw-bytes accessor for the enumerated schema-migration and retired-key consumers, and
  a bytes-level `ParseBytes` seam for `internal/audit`, which reads git blobs rather than the
  working tree. Per-ADR format-v1 parsing enforces the closed frontmatter, section order,
  State-changes grammar, and Status-history digest and sequence, while the corpus adds contiguity and
  provenance across records. It also renders `INDEX.md` (In flight and History) from a `Corpus`
  rather than parsing.
- **`internal/topic/`**: the strict, path-derived current-state topic parser and one per-invocation
  corpus. It pairs metadata and constrained Markdown parts, resolves Implemented-ADR provenance and
  direct claim references, validates configured relevance, touches, and proof markers, computes
  focused topic coverage, and builds deterministic topic, index, and domain-navigation render models.
  Its claims are the active authority the current-state runtime reads for context, coverage, and
  invariant backing.
- **`internal/currentstate/`**: the tree-backed current-state loader and static checker. It loads the
  ADR and topic corpora from one `snapshot.Tree`, validates the resulting lifecycle, sequence,
  operation history, claim provenance, and absence rules (`Check`), and compares two universes for the
  staged and range transition handshake (`CheckPair`).
- **`internal/upgrade/`**: the final current-state cutover from a sealed bridge attestation. It
  verifies only the sealed facts (the prepared HEAD and tree digest), then drives the versioned
  `.awf/current-state-upgrade.journal` (`journal.go`) so plain `awf upgrade` consumes the seal
  (`FinalUpgrade`) and `awf upgrade --recover` replays the recovery table. It consumes seals; it never
  produces them.
- **`internal/render/`**: Go `text/template` rendering (ADR-0001); first expands awf-owned
  `templates/partials/` bodies via `ExpandIncludes` (ADR-0052), then assembles section
  overlays (sidecar overrides + convention parts) and executes the template.
- **`internal/manifest/`**: reads and writes `.awf/awf.lock` (schema-versioned); drives
  drift detection for `awf check`.
- **`internal/migrate/`**: ordered schema-migration registry (ADR-0010); the `tree-layout`
  migration and the frozen legacy reader; powers `awf upgrade` and the sync/check version gate.
  Two migrations write the ADR corpus itself via raw-byte surgery, importing `internal/adr`
  (acyclic: nothing on the render path imports migrate). The generation-10 `retirement-tokens`
  migration ported the legacy ADR-0031 retirement frontmatter to `supersedes-invariant:` tokens
  (ADR-0120), the precedent for corpus-writing migrations; the generation-12
  `supersession-keys` migration strips the `supersedes:`/`superseded_by:` keys, downgrades every
  pre-existing item token to `refines:`, and appends a bookkeeping Decision item retiring each
  predecessor's anchors (ADR-0128). The generation 13 migration reuses `applyCloseEnabledSet` to add `exploring`
  wherever an enabled brainstorming, debugging, or refactor coupling audit consumer requires it.
  Later generations add the current-state topic substrate and the project-atomic cutover that drops
  the legacy `invariants` config and promotes the ADR format cutoff and gaps into the permanent lock.
  Both corpus-writing migrations resolve their own decisions directory and run before a
  `Project` can be opened, so they construct the corpus through `adr.LoadCorpus` rather than
  taking a threaded view.
- **`internal/project/`**: orchestrates config + catalog + render + manifest into `Sync()` and
  `Check()`; golden tests live here. Its read-only context path builds one working- or index-backed
  universe, expands request paths to effective paths, applies the precedence-ordered primary
  classification, resolves safe authority and applicability, derives artifact navigation from
  reader-injected output declarations, and then selects the concise or full projection without
  writing or reloading from another universe. A single ordered kind-descriptor table (`kind.go`) is the sole
  per-kind dispatch source; enable array, catalog pool, declared sections, output path, and labels
  resolve through it across `list`/`enable`/`check`/`validate` (ADR-0027). `singleton.go`'s
  `plainSingletons` derives from the catalog's `Mandatory` non-agents-doc entries: the render/validate
  identity of the neutral always-on singletons, no hand-authored table (ADR-0043, ADR-0059, ADR-0061).
  The topic producer loads through a lazy invocation cache and contributes ordinary
  managed Markdown nodes, so sync, manifest membership, brownfield backup, drift comparison, and
  prune all use the shared output plan rather than topic-specific lock state. `CheckCurrentState` and
  `CheckStaged` run the current-state static and staged checks, and `Audit` evaluates per-commit claim
  transitions across a range.
- **`internal/audit/`**: go-git-backed collection of the branch's commits plus the advisory
  workflow-conformance rules; powers `awf audit` and the blocking `awf commit-gate`
  (ADR-0017, ADR-0036).
- **`internal/prosegate/`**: scans a project's tracked text files for the seven banned
  typographic punctuation substitutes; powers the opt-in blocking `awf prose-gate` (ADR-0119).
  The presence-level counterpart to `internal/audit`'s net-increase `plain-punctuation` rule:
  it answers whether the tree is clean, not whether a commit made it worse.
- **`internal/pathglob/`**: awf's single glob dialect (ADR-0077): anchored full-path doublestar
  matching against slash-separated repo-relative paths, consumed by config validation, invariant
  scanning, and the audit's path matching. Leaf package.
- **`internal/refs/`**: pure, stdlib-only extraction of inline markdown link targets for the
  dead-reference scan (ADR-0020).
- **`internal/initspec/`**: `awf init`'s descriptor machinery: the catalog `vars:` descriptors,
  `--describe` JSON, `--set`/`--answers` merging and validation, and the interactive prompts
  (ADR-0029).
- **`internal/coverage/`**: merges the gate's cover profile and enforces the
  `// coverage-ignore: <reason>` contract for `cmd/covercheck` (ADR-0012). Repo-only, not part
  of the rendered standard.
- **`internal/testsupport/`**: shared test helpers and the `gitfixture/` in-memory repo builder;
  a tooling-owned leaf boundary that may use the standard library and its own subpackages, with
  go-git permitted in `gitfixture`, but may not import another repository internal package. The
  dead-code gate's reachability rule names it explicitly, so a helper reachable only from here
  still counts as dead production code (ADR-0063). Repo-only, not part of the rendered standard.
- **`internal/evals/`**: test-only chain and fixture evaluations over `internal/catalog`.
  Repo-only, not part of the rendered standard.
- **`internal/frontmatter/`**: the single parser for `---`-delimited YAML frontmatter; used by
  `internal/adr` and skill/agent validation.
- **`internal/adr/`**: parses ADRs, regenerates `docs/decisions/INDEX.md` from the corpus,
  and scaffolds new ADR files (`NextNumber`/`NewFile`, ADR-0042); invoked by
  `awf sync` (`./x sync`) and `awf new adr`.
- **`internal/plan/`**: parses plan files under `docs/plans` and scaffolds new ones
  (`ParseDir`/`NewFile`); date-prefixed rather than sequentially numbered, unlike `internal/adr`
  (ADR-0097, ADR-0098). Read by the `awf check` plan-link validation and `awf new plan`.
- **`internal/git/`**: centralised tolerant go-git repo-open (linked worktrees, submodules, the
  `worktreeConfig`-extension workaround) plus eligible working-path, commit-blob, range-blob, and
  staged-blob readers (each carrying its executable mode); read-only,
  shared by `awf audit` and `awf context`, and feeding the `internal/snapshot` seam that
  the current-state checks and `awf prose-gate` consume (ADR-0092). It is also the sole
  definition site for `<a>..<b>` range parsing (`ParseRange`, ADR-0127): every command taking
  a range parses through it, and a test fails the build if a second parser reappears.
- **`internal/snapshot/`**: captures immutable, path-sorted file trees whose `File`s own private
  byte copies, so a consumer reads the captured content and mode without mutating the snapshot or the
  caller's data; `NewTree` rejects unsupported modes, unsafe paths, and duplicates. It exposes
  `IndexTree`, `WorkingTree`, `CommitTree`, and `RangePair` over `internal/git`'s blob readers
  (preserving executable mode, rejecting an unmerged index), consumed by the current-state checks,
  `awf context`, and `awf prose-gate`.
- **`internal/configspec/`**: the compile-time, adopter-facing description authority (ADR-0088):
  every config key, sidecar field, and per-artifact data key with adopter-voiced descriptions and
  availability clauses, var entries derived verbatim from the catalog descriptors. Bidirectional
  reflection/template parity and description-residue rules are test-enforced. Projected into the
  generated `docs/config-reference.md` (rendered by `internal/project` outside `RenderAll`,
  regeneration-checked) and the `awf config` CLI command (gated live mode, static pre-adoption
  fallback).
- **`templates/`**: embedded skill, agent, doc, agent-guide, and target-output template bodies.
  `templates/pi/awf-subagents/` contains the two-file Pi delegation extension. Pi-rendered workflow
  skills name its `subagent_grounding`, `subagent_explore`, `subagent_review`, and
  `subagent_implement` tools explicitly; other targets retain their native or generic dispatch
  language. Exploration requires task, breadth, and detail, which feed Pi's dynamic fixed prompt;
  every role resolves an optional exact model through Pi's registry and authentication state before
  any queue or side effect. A session-local limiter admits at most ten exploration children and
  schedules excess calls FIFO with abort-aware removal, while implementation retains its own queue.
  A current-leaf and tool-call-id-correlated preflight blocks mixed implementation batches at Pi's
  real tool-call event seam. The unchanged runner remains the process and context-isolation boundary.
  Bounded structured details drive one shared inline renderer for all four tools, while only final
  report or failure-summary content reaches the parent model. The catalog itself is the
  compile-time `catalog.Standard` value in `internal/catalog`.
- **`tools/pi-extension-test/`**: the Docker-only strict TypeScript and 100% coverage harness for
  the dogfooded generated extension. Its repo-keyed persistent container snapshots current source
  and keeps npm dependencies off the host.
- **`changelog/`**: embeds the hand-maintained `CHANGELOG.md` (ADR-0041); a top-level package
  because `go:embed` cannot embed a file outside its own package directory.
- **`internal/changelog/`**: parses the embedded changelog into filterable entries; powers `awf
  changelog`.
- **`examples/sundial/`**: the committed example adopter (ADR-0090): a fictional Go
  module (own `go.mod`, invisible to the repo's `./...` sweeps) whose full rendered
  surface is the rendered-output quality oracle: re-rendered by `./x sync`, gated
  by `./x check`. Not part of the rendered standard.
