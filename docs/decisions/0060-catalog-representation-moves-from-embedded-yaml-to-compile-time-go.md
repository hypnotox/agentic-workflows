---
status: Proposed
date: 2026-07-05
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [catalog, rendering, config, refactor]
related: [0022, 0027, 0043]
domains: [rendering, config]
---
# ADR-0060: Catalog representation moves from embedded YAML to compile-time Go

## Context

The catalog (`templates/catalog.yaml`) is awf's static description of the standard: every skill,
agent, doc, singleton, the domain-doc spec, and the fillable `vars`. It is embedded via
`templates/embed.go`'s `//go:embed` directive and parsed at runtime by `catalog.Load(fs.FS)` into a
`catalog.Catalog` (`internal/catalog/catalog.go`).

The catalog is **not adopter-facing**. Adopters carry only `.awf/config.yaml` enable arrays plus
sidecars/parts; the catalog itself lives in the binary. Grounding confirmed it is not part of the
config-schema gate (`minVersionBySchema`, `internal/migrate`) — a catalog change needs no adopter
migration or version bump (ADR-0022 already notes catalog edits are "No config-tree schema bump").
Tests already construct `catalog.Catalog` values directly in Go.

Two costs follow from storing this Go-internal data as parsed YAML:

- **A runtime parse that cannot meaningfully fail.** All four production `catalog.Load(templates.FS)`
  call sites (`internal/project/project.go`, `internal/project/scaffold.go`, `cmd/awf/init.go`,
  and the evals fixture) carry `// coverage-ignore` comments because the embedded parse is
  unreachable-to-fail, and `internal/catalog` keeps two error-path tests (malformed/missing YAML)
  that exist only to cover branches that can never trigger in production.
- **Forced compile-time duplication.** `catalog.SingletonKinds` is a hand-maintained `[]string`
  kept *separate* from the loaded `Catalog.Singletons` map precisely because `config.IsSingletonKind`
  needs the classification without holding a `*Catalog` — `config` imports `catalog` but must not
  load the FS. The two are bound only by a parity test. This is a standing drift site that exists
  solely because the real data is behind a runtime parse.

The import graph makes the move clean: `config → catalog`; `catalog` imports nothing internal and
takes `fs.FS` as a parameter; `templates` imports only `embed`. Dropping the parse turns `catalog`
into a pure-data leaf with no `fs.FS`/`yaml` dependency and introduces no cycle.

This ADR changes **only the representation**. The `Catalog` struct shape, `SingletonKinds`, and every
existing invariant are preserved unchanged; unifying the doc model and collapsing the resulting
projections is deferred to a successor ADR so this step stays a behavior-preserving refactor.

## Decision

1. **The standard catalog is a compile-time Go value.** Package `internal/catalog` exposes the
   catalog as a package-level value (`Standard`) built from Go literals — the same `Catalog` struct
   shape it has today (`Skills`, `Agents`, `Docs`, `Singletons`, `DomainDoc`, `Vars`). Entry metadata
   stays typed (`Sections []string`, etc.); the per-artifact freeform default `Data` bag stays
   `map[string]any`, written as Go literals.

2. **`templates/catalog.yaml` and the runtime parse are removed.** Delete the YAML file, drop its
   token from `templates/embed.go`'s `//go:embed` list, and delete `catalog.Load` along with its two
   error-path tests. The template `*.tmpl` bodies stay embedded and are still read by `renderTarget`.
   Package `catalog` no longer imports `io/fs` or a YAML decoder.

3. **Callers read the package value.** Every `catalog.Load(templates.FS)` site (production and test)
   reads `catalog.Standard` directly; the three `// coverage-ignore` comments on the former
   `Load` error branches are removed with them. `config.IsSingletonKind` continues to read
   `catalog.SingletonKinds` — no behavioural change.

4. **Behaviour-preserving.** Rendered output is byte-identical: the migration reproduces the current
   default `Data` values with the same runtime shapes YAML produced (`map[string]any` / `[]any` /
   scalars), so the per-file `ConfigHash` and the committed `.awf/awf.lock` stay stable and
   `awf check` remains clean. If faithfully reproducing a value proves impractical, a one-time
   `awf.lock` regeneration is acceptable — output equality is the contract, not lock byte-identity —
   but no rendered artifact may change.

5. **Scope boundary.** `SingletonKinds` remains a hand-maintained list and every invariant listed in
   ADR-0043/0027 keeps its current wording and backing. Merging the toggleable-doc and singleton
   collections, deriving classification and paths from the catalog, and reconciling the affected
   invariants are the subject of the successor doc-model ADR, not this one.

## Invariants

- `inv: catalog-go-single-source` — the standard catalog has exactly one authoritative
  representation: the compile-time Go value in `internal/catalog`. There is no embedded
  `catalog.yaml` and no runtime catalog parse. Backed by a test asserting `templates.FS` contains no
  `catalog.yaml` entry and that `catalog.Standard` is populated across all kinds.
- Rendered artifacts are unchanged by the representation move (textual contract, verified once by the
  drift-clean gate at implementation): the catalog carries the same values, only in a different
  encoding.

## Consequences

- The `catalog` package becomes a pure-data leaf with no filesystem dependency; the dependency graph
  simplifies and `config` keeps reading it exactly as before.
- Net coverage simplifies: the two never-failing error-path tests and three `// coverage-ignore`
  comments disappear with the parse, and the Go literal is data initialization (no branches), so the
  100% gate (ADR-0012) is easier to satisfy, not harder.
- The doc-model unification is unblocked: once the catalog is Go, deriving `SingletonKinds`, layout
  paths, and the document-map set from a single collection is ordinary Go code rather than a
  runtime-parse workaround. That work — and its ADR-0043 invariant reconciliation — is the successor
  ADR.
- Live awf-managed docs that name `templates/catalog.yaml` in prose (`docs/architecture.md`,
  `docs/testing.md`, and the convention parts `.awf/docs/parts/architecture/components.md` and
  `.awf/docs/parts/testing/layout.md` that render into them) update in the same commit that removes
  the file (docs-travel-with-change). Frozen ADRs and plans are append-only and stay as written.
- No adopter impact: the catalog lives in the binary, so no schema bump, no migration, and — with
  faithful value reproduction — no `awf check` drift for adopters on upgrade.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the catalog as embedded YAML | Preserves the never-failing runtime parse, its dead error-path tests and coverage-ignores, and — most importantly — the `SingletonKinds`-vs-`Singletons` duplication that exists *only* because the data is behind a parse. The data is Go-internal and never adopter-edited, so YAML buys nothing. |
| Move only the doc metadata to Go, leave skills/agents/vars in YAML | Splits the single source of static truth across two representations — worse than either pure form, and leaves the runtime parse in place for the rest. |
| Fold the representation change and the doc-model unification into one ADR | Couples two independently load-bearing decisions and entangles the ADR-0043 invariant reconciliation with a mechanical refactor; the bounded-ADR convention favours sequencing representation first, then the merge. |
