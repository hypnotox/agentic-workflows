## Components

- **`cmd/awf/`**: CLI entry point; `init`, `sync`, `check`, `list`, `config`, `context`, `enable`,
  `disable`, `new`, `audit`, `invariants`, `commit-gate`, `prose-gate`, `upgrade`, `uninstall`,
  `changelog`, `version` subcommands, dispatched by a generic parse-once driver (`dispatch.go`) over the declarative
  `internal/clispec` command table (ADR-0094). The gated commands enforce the binary-version gate
  (ADR-0010, ADR-0039) before opening the project; the driver pre-gates the always-gated ones,
  while `config`/`context`/`new` gate in-handler after their static-fallback / name-validation check.
- **`internal/clispec/`**: the declarative CLI command table (ADR-0094): each command's flags,
  positional bounds, three-valued gating classification, help text, and (for `new`) its
  subcommands, as a data-only importable leaf. `cmd/awf` attaches handler funcs to it, and
  `internal/project` reads its gated set to generate the docs' gated-command list.
- **`cmd/covercheck`, `cmd/deadcodecheck`, `cmd/mutants`, `cmd/pincheck`,
  `cmd/releasecheck`, `cmd/repoaudit`**: repo-only gate, release, triage, and audit
  helpers: the 100% statement-coverage floor (ADR-0012), the dead-code gate
  (ADR-0063), the advisory mutation-survivor report (ADR-0066), the workflow-pin
  check (ADR-0079), the release-time changelog pin (ADR-0078), and the repo-local
  conformance audit (`./x audit-local`, ADR-0073: changelog-entry Errors plus
  coverage-ignore re-evaluation Warnings). Not part of the rendered standard.
- **`internal/config/`**: owns `.awf/config.yaml`: the schema and strict load, its construction
  (`MarshalSkeleton`) and mutation (`SetArrayMember`, a comment-preserving `yaml.Node` round-trip)
  behind one `encode` funnel (ADR-0026; `internal/migrate` excepted), plus keyed sidecars.
  `IsSingletonKind` classifies off `internal/catalog`'s `SingletonKinds()` (ADR-0043, ADR-0061).
- **`internal/catalog/`**: declares the available skills, agents, docs, and their sections as a
  compile-time Go value (`catalog.Standard`; ADR-0060), with no runtime parse. Docs and always-on
  singletons are one `Docs` collection: each `DocEntry`'s `Mandatory` flag marks a singleton, and
  `SingletonKinds()` derives from it (ADR-0043, ADR-0061).
- **`internal/render/`**: Go `text/template` rendering (ADR-0001); first expands awf-owned
  `templates/partials/` bodies via `ExpandIncludes` (ADR-0052), then assembles section
  overlays (sidecar overrides + convention parts) and executes the template.
- **`internal/manifest/`**: reads and writes `.awf/awf.lock` (schema-versioned); drives
  drift detection for `awf check`.
- **`internal/migrate/`**: ordered schema-migration registry (ADR-0010); the `tree-layout`
  migration and the frozen legacy reader; powers `awf upgrade` and the sync/check version gate.
  The generation-10 `retirement-tokens` migration writes the ADR corpus itself - porting the
  legacy ADR-0031 retirement frontmatter to `supersedes-invariant:` tokens via raw-byte surgery
  (ADR-0120), the precedent for corpus-writing migrations - importing `internal/adr` and
  `internal/invariants` (acyclic: nothing on the render path imports migrate).
- **`internal/project/`**: orchestrates config + catalog + render + manifest into `Sync()` and
  `Check()`; golden tests live here. A single ordered kind-descriptor table (`kind.go`) is the sole
  per-kind dispatch source; enable array, catalog pool, declared sections, output path, and labels
  resolve through it across `list`/`enable`/`check`/`validate` (ADR-0027). `singleton.go`'s
  `plainSingletons` derives from the catalog's `Mandatory` non-agents-doc entries: the render/validate
  identity of the neutral always-on singletons, no hand-authored table (ADR-0043, ADR-0059, ADR-0061).
- **`internal/audit/`**: go-git-backed collection of the branch's commits plus the advisory
  workflow-conformance rules; powers `awf audit` and the blocking `awf commit-gate`
  (ADR-0017, ADR-0036).
- **`internal/prosegate/`**: scans a project's tracked text files for the seven banned
  typographic punctuation substitutes; powers the opt-in blocking `awf prose-gate` (ADR-0119).
  The presence-level counterpart to `internal/audit`'s net-increase `plain-punctuation` rule:
  it answers whether the tree is clean, not whether a commit made it worse.
- **`internal/invariants/`**: verifies every Implemented ADR's `invariant:` slugs against
  `invariant:`-marker backing comments under the config-driven source globs (ADR-0008);
  powers `awf invariants` and the gated check.
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
  the dead-code gate's reachability rule names it explicitly, so a helper reachable only from here
  still counts as dead production code (ADR-0063). Repo-only, not part of the rendered standard.
- **`internal/evals/`**: test-only chain and fixture evaluations over `internal/catalog`.
  Repo-only, not part of the rendered standard.
- **`internal/frontmatter/`**: the single parser for `---`-delimited YAML frontmatter; used by
  `internal/adr` and skill/agent validation.
- **`internal/adr/`**: parses ADRs, regenerates `docs/decisions/ACTIVE.md` from their
  frontmatter, and scaffolds new ADR files (`NextNumber`/`NewFile`, ADR-0042); invoked by
  `awf sync` (`./x sync`) and `awf new adr`.
- **`internal/plan/`**: parses plan files under `docs/plans` and scaffolds new ones
  (`ParseDir`/`NewFile`); date-prefixed rather than sequentially numbered, unlike `internal/adr`
  (ADR-0097, ADR-0098). Read by the `awf check` plan-link validation and `awf new plan`.
- **`internal/git/`**: centralised tolerant go-git repo-open (linked worktrees, submodules, the
  `worktreeConfig`-extension workaround) plus tracked-path and staged-blob readers; read-only,
  shared by `awf audit`, `awf context`, and `awf prose-gate` (ADR-0092). It is also the sole
  definition site for `<a>..<b>` range parsing (`ParseRange`, ADR-0127): every command taking
  a range parses through it, and a test fails the build if a second parser reappears.
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
  language. Bounded structured details drive one shared inline renderer for all four tools, while
  only final report or failure-summary content reaches the parent model. The catalog itself is the
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
