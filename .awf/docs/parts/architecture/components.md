## Components

- **`cmd/awf/`** — CLI entry point; `init`, `sync`, `check`, `list`, `add`, `remove`, `new`,
  `audit`, `invariants`, `commit-gate`, `upgrade`, `uninstall`, `changelog`, `version`
  subcommands. The gated commands enforce the binary-version gate (ADR-0010, ADR-0039) before
  opening the project.
- **`cmd/covercheck`, `cmd/deadcodecheck`, `cmd/mutants`, `cmd/repoaudit`** — repo-only gate,
  triage, and audit helpers: the 100% statement-coverage floor (ADR-0012), the dead-code gate
  (ADR-0063), the advisory mutation-survivor report (ADR-0066), and the repo-local conformance audit
  (`./x audit-local`, ADR-0073: changelog-entry Errors plus coverage-ignore re-evaluation
  Warnings). Not part of the rendered standard.
- **`internal/config/`** — owns `.awf/config.yaml`: the schema and strict load, its construction
  (`MarshalSkeleton`) and mutation (`SetArrayMember`, a comment-preserving `yaml.Node` round-trip)
  behind one `encode` funnel (ADR-0026; `internal/migrate` excepted), plus keyed sidecars.
  `IsSingletonKind` classifies off `internal/catalog`'s `SingletonKinds()` (ADR-0043, ADR-0061).
- **`internal/catalog/`** — declares the available skills, agents, docs, and their sections as a
  compile-time Go value (`catalog.Standard`; ADR-0060), with no runtime parse. Docs and always-on
  singletons are one `Docs` collection: each `DocEntry`'s `Mandatory` flag marks a singleton, and
  `SingletonKinds()` derives from it (ADR-0043, ADR-0061).
- **`internal/render/`** — Go `text/template` rendering (ADR-0001); first expands awf-owned
  `templates/partials/` bodies via `ExpandIncludes` (ADR-0052), then assembles section
  overlays (sidecar overrides + convention parts) and executes the template.
- **`internal/manifest/`** — reads and writes `.awf/awf.lock` (schema-versioned); drives
  drift detection for `awf check`.
- **`internal/migrate/`** — ordered schema-migration registry (ADR-0010); the `tree-layout`
  migration and the frozen legacy reader; powers `awf upgrade` and the sync/check version gate.
- **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and
  `Check()`; golden tests live here. A single ordered kind-descriptor table (`kind.go`) is the sole
  per-kind dispatch source — enable array, catalog pool, declared sections, output path, and labels
  resolve through it across `list`/`add`/`check`/`validate` (ADR-0027). `singleton.go`'s
  `plainSingletons` derives from the catalog's `Mandatory` non-agents-doc entries — the render/validate
  identity of the neutral always-on singletons, no hand-authored table (ADR-0043, ADR-0059, ADR-0061).
- **`internal/audit/`** — go-git-backed collection of the branch's commits plus the advisory
  workflow-conformance rules; powers `awf audit` and the blocking `awf commit-gate`
  (ADR-0017, ADR-0036).
- **`internal/invariants/`** — verifies every Implemented ADR's `inv:` slugs against
  `invariant:`-marker backing comments under the config-driven source globs (ADR-0008);
  powers `awf invariants` and the gated check.
- **`internal/pathglob/`** — awf's single glob dialect (ADR-0077): anchored full-path doublestar
  matching against slash-separated repo-relative paths, consumed by config validation, invariant
  scanning, and the audit's path matching. Leaf package.
- **`internal/refs/`** — pure, stdlib-only extraction of inline markdown link targets for the
  dead-reference scan (ADR-0020).
- **`internal/initspec/`** — `awf init`'s descriptor machinery: the catalog `vars:` descriptors,
  `--describe` JSON, `--set`/`--answers` merging and validation, and the interactive prompts
  (ADR-0029).
- **`internal/coverage/`** — merges the gate's cover profile and enforces the
  `// coverage-ignore: <reason>` contract for `cmd/covercheck` (ADR-0012). Repo-only, not part
  of the rendered standard.
- **`internal/frontmatter/`** — the single parser for `---`-delimited YAML frontmatter; used by
  `internal/adr` and skill/agent validation.
- **`internal/adr/`** — parses ADRs, regenerates `docs/decisions/ACTIVE.md` from their
  frontmatter, and scaffolds new ADR files (`NextNumber`/`NewFile`, ADR-0042); invoked by
  `awf sync` (`./x sync`) and `awf new adr`.
- **`templates/`** — embedded skill, agent, doc, and agent-guide template bodies the catalog names
  (the catalog itself is the compile-time `catalog.Standard` value in `internal/catalog`).
- **`changelog/`** — embeds the hand-maintained `CHANGELOG.md` (ADR-0041); a top-level package
  because `go:embed` cannot embed a file outside its own package directory.
- **`internal/changelog/`** — parses the embedded changelog into filterable entries; powers `awf
  changelog`.
