## Components

- **`cmd/awf/`** — CLI entry point; `init`, `sync`, `check`, `list`, `add`, `upgrade`
  subcommands. `sync`/`check` enforce the schema-generation gate (ADR-0010) before opening the project.
- **`internal/config/`** — owns `.awf/config.yaml`: the schema and strict load, its construction
  (`MarshalSkeleton`) and mutation (`SetArrayMember`, a comment-preserving `yaml.Node` round-trip)
  behind one `encode` funnel (ADR-0026; `internal/migrate` excepted), plus keyed sidecars.
  `IsSingletonKind` classifies off `internal/catalog`'s `SingletonKinds` (ADR-0043).
- **`internal/catalog/`** — reads `templates/catalog.yaml`; declares the available skills, agents,
  docs, and their sections. `Singletons` and the compile-time `SingletonKinds` list name every
  always-on singleton (ADR-0043).
- **`internal/render/`** — Go `text/template` rendering (ADR-0001); assembles section
  overlays (sidecar overrides + convention parts) then executes the template.
- **`internal/manifest/`** — reads and writes `.awf/awf.lock` (schema-versioned); drives
  drift detection for `awf check`.
- **`internal/migrate/`** — ordered schema-migration registry (ADR-0010); the `tree-layout`
  migration and the frozen legacy reader; powers `awf upgrade` and the sync/check version gate.
- **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and
  `Check()`; golden tests live here. A single ordered kind-descriptor table (`kind.go`) is the sole
  per-kind dispatch source — enable array, catalog pool, declared sections, output path, and labels
  resolve through it across `list`/`add`/`check`/`validate` (ADR-0027). `singleton.go`'s
  `plainSingletons` table is the analogous single source of truth for the six neutral always-on
  singletons' render/validate identity (ADR-0043).
- **`internal/frontmatter/`** — the single parser for `---`-delimited YAML frontmatter; used by
  `internal/adr` and skill/agent validation.
- **`internal/adr/`** — parses ADRs, regenerates `docs/decisions/ACTIVE.md` from their
  frontmatter, and scaffolds new ADR files (`NextNumber`/`NewFile`, ADR-0042); invoked by
  `awf sync` (`./x sync`) and `awf new adr`.
- **`templates/`** — embedded skill, agent, doc, and agent-guide templates; the catalog
  lives at `templates/catalog.yaml`.
- **`changelog/`** — embeds the hand-maintained `CHANGELOG.md` (ADR-0041); a top-level package
  because `go:embed` cannot embed a file outside its own package directory.
- **`internal/changelog/`** — parses the embedded changelog into filterable entries; powers `awf
  changelog`.
