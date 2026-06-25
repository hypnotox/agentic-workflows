# Plan: Tree-Based Config Layout + Schema-Versioned Lock & `awf upgrade`

Implements **[ADR-0009](../decisions/0009-tree-based-config-layout.md)** (Accepted) and
**[ADR-0010](../decisions/0010-versioned-lock-and-awf-upgrade.md)** (Accepted). Design
rationale lives in the ADRs — this plan is the execution record.

## Goal

Split the monolithic `.claude/awf.yaml` into a tree under a single `.claude/awf/` root
(skeleton `config.yaml` with flat `[]string` enable lists, per-target sidecar YAMLs for
`data`/`sections`/`local`, prose parts bound by convention), relocate the lock to
`.claude/awf/awf.lock`, re-model the agents-doc prose into convention parts, add the
local-frontmatter check, and ship the migration mechanism (a `schemaVersion` lock field, an
ordered migration registry under `internal/migrate`, an `awf upgrade` subcommand, and a
sync/check version gate) — porting awf's own config with `awf upgrade` itself.

## Bootstrap constraint (drives the phase ordering)

awf dogfoods itself: the pre-commit hook runs `awf check` (= `go run ./cmd/awf check` →
`config.Load`) **and** `./x gate`. Two hard facts make the cutover **irreducibly atomic**:

1. **Go won't compile a half-changed `config.Config`.** The new tree shape changes the *shared*
   `config.Config` struct (`SkillConfig`→`Sidecar`, the `Skills`/`Agents`/`Docs` map fields →
   `[]string`, `AgentsDoc` dropped, `Raw()` removed, `Load(path)`→`Load(awfDir)`). Every live
   consumer — `internal/project` (`Open`, `validateAgainstCatalog`, `data`, `resolvedDocs`,
   `RenderAll`, `Sync`/`Check`'s `Raw()`), `cmd/awf/list_add.go` (`p.Cfg.Skills[n]` map index,
   `appendSkill`), and `internal/project/scaffold.go` — reads the old shape. The moment the struct
   changes, all of them must change in the **same commit** or `./x gate` (`go build`/`go test ./...`)
   is red.
2. **A tree-shaped `config.Load` cannot parse the repo's still-legacy `.claude/awf.yaml`.** So the
   instant the loader flips, this repo must already be ported to the tree — otherwise the
   pre-commit `awf check` (and `./x check`) fails to open the project at all. The loader flip and
   the repo port (`awf upgrade`) are therefore one commit.

**Option B (chosen) — collapse the cutover into one atomic Phase 3 commit.** Phases 1–2 add only
code with **no live consumers**, so each is independently gate-safe:

- **Phase 1** adds the single additive `manifest.Lock.SchemaVersion int` field (an absent field
  unmarshals to `0`) — no behaviour change, nothing reads it yet.
- **Phase 2** adds the whole `internal/migrate` package (frozen legacy reader, `tree-layout`
  migration, registry/gate predicate/`Upgrade`) and `cmd/awf/upgrade.go` + its dispatch case. None
  of this is wired into `runSync`/`runCheck`, and `internal/migrate` is imported by nothing on the
  live path, so the binary still loads via the legacy `config.Load`.
- **Phase 3** is the irreducible atomic cutover: the `config.Config` rewrite + every dependent
  rewrite in `internal/project`, `cmd/awf/list_add.go`, `internal/project/scaffold.go`, the gate
  wiring, all new tests, **and** running `awf upgrade` to port this repo — committed together so
  the working tree is fully tree-shaped and pre-commit `awf check` passes against the ported tree.
- **Phase 4** is cleanup/docs/flips: dead-template-branch removal, doc-currency, and the ADR
  Implemented flips with all 12 tagged slugs backed.

`go test ./...` uses embedded templates + temp-dir fixtures and never reads the repo's `.claude/`,
so every Phase 1–3 test runs against synthetic trees and the only commit that touches the repo's
own `.claude/` is the Phase 3 atomic cutover.

## Architecture summary

- `internal/config`: `Config` enable fields become `[]string` (`Skills`/`Agents`/`Docs`);
  `SkillConfig` is renamed `Sidecar` (same `Data`/`Sections`/`Local`); `Load` reads
  `.claude/awf/config.yaml`; new `(*Config).Sidecar(kind, name)` reads
  `<awfDir>/<kind>/<name>.yaml` (agents-doc: `<awfDir>/agents-doc.yaml`), empty when absent.
- `internal/render`: a 4th precedence tier (convention part) is injected by the project layer
  as a synthetic `SectionOverride{ReplaceWith}`; `render.Assemble` is unchanged.
- `internal/project`: `parts()`/`data()`/`RenderAll`/`Sync`/`Check`/`lockPath` move to the
  tree; per-target `ConfigHash` projection replaces whole-file `Raw()` hashing; local targets
  get on-disk frontmatter validation; `agents-doc` stays the always-on singleton.
- `internal/manifest`: `Lock` gains `SchemaVersion int`.
- `internal/migrate` (new): ordered migration registry, the `tree-layout` (`To: 1`) migration,
  a frozen legacy reader of `.claude/awf.yaml`, and the gate predicate.
- `cmd/awf`: `upgrade` subcommand; version gate in `runSync`/`runCheck`; `list`/`add` rewritten
  to array enable lists + sidecars; `init`/scaffold emit the tree.
- `templates/agents-doc/AGENTS.md.tmpl`: `you-and-this-project`/`identity` bodies carry generic
  defaults; awf's prose moves to `.claude/awf/parts/agents-doc/*.md`.

## Tech stack

- Go 1.26; packages: `internal/config`, `internal/render`, `internal/project`,
  `internal/manifest`, `internal/migrate` (new), `cmd/awf`. Deps: stdlib only
  (`path/filepath`, `os`, `encoding/json`, `gopkg.in/yaml.v3` already vendored).
- Gate: `./x gate` (~15s, go test + vet + golangci-lint) per code commit; pre-commit also runs
  `./x check`.

## File structure

**Created:** `internal/migrate/migrate.go`, `internal/migrate/legacy.go`,
`internal/migrate/treelayout.go`, `internal/migrate/migrate_test.go`,
`cmd/awf/upgrade.go`; on cutover the ported tree `.claude/awf/config.yaml`,
`.claude/awf/awf.lock`, `.claude/awf/{skills,agents,docs}/*.yaml`,
`.claude/awf/**/parts/**/*.md`, `.claude/awf/agents-doc.yaml`,
`.claude/awf/parts/agents-doc/{you-and-this-project,identity}.md`.

**Modified:** `internal/config/config.go` (+`config_test.go`),
`internal/project/project.go`, `internal/project/scaffold.go` (+`spine_test.go` and other
`project` tests), `internal/manifest/manifest.go` (+`manifest_test.go`),
`cmd/awf/main.go`, `cmd/awf/list_add.go` (+`list_add` tests),
`templates/agents-doc/AGENTS.md.tmpl`, `docs/architecture.md`, plus re-synced `.claude/**`
and `AGENTS.md`.

**Deleted (on cutover):** `.claude/awf.yaml`, `.claude/awf.lock`.

**Tagged invariant slug → backing test (authoritative):**

| ADR | slug | backing test (file) |
|---|---|---|
| 0009 | `config-root` | `TestLoadReadsTreeRoot` (config_test.go) |
| 0009 | `enable-arrays` | `TestEnableListsAreArrays` (config_test.go) |
| 0009 | `sidecar-optional` | `TestSidecarAbsentRendersDefault` (project: render_tree_test.go) |
| 0009 | `parts-convention` | `TestConventionPartPrecedence` (project: render_tree_test.go) |
| 0009 | `local-frontmatter` | `TestLocalFrontmatterChecked` (project: render_tree_test.go) |
| 0009 | `drift-source-set` | `TestPerTargetDriftProjection` (project: drift_test.go) |
| 0009 | `agentsdoc-parts` | `TestAgentsDocPartsOverride` (project: render_tree_test.go) |
| 0010 | `schema-version-lock` | `TestSyncStampsSchemaVersion` (manifest_test.go / project) |
| 0010 | `upgrade-gate` | `TestGateBlocksWhenBehind` (migrate_test.go) |
| 0010 | `migration-ordering` | `TestUpgradeAppliesInOrderIdempotent` (migrate_test.go) |
| 0010 | `legacy-read-isolation` | `TestLegacyReadOnlyInMigrate` (migrate_test.go) |
| 0010 | `noop-autobump` | `TestNoopGapAutoBumps` (migrate_test.go) |

Each backing test carries a `// invariant: <slug>` comment (config marker `//`, glob `*.go` —
matches the repo's `invariants.sources`). `awf check` enforces these once the ADRs flip to
`Implemented` (Phase 4 final task); they are added in the phase that implements each feature.
Under option B the slug homes by phase are: `upgrade-gate`/`migration-ordering`/
`legacy-read-isolation`/`noop-autobump` land in **Phase 2** (the `internal/migrate` commit);
`config-root`/`enable-arrays`/`sidecar-optional`/`parts-convention`/`local-frontmatter`/
`drift-source-set`/`schema-version-lock` land in **Phase 3** (the atomic cutover); `agentsdoc-parts`
lands in **Phase 4** (the template cleanup). All are backed before the Phase 4 Implemented flip.

---

## Phase 1 — `manifest.SchemaVersion` (additive, gate-safe)

A single additive field on the lock. No live consumer reads it yet, an absent field unmarshals to
`0`, so the binary's behaviour is unchanged and `./x gate` stays green.

### Task 1.1 — `SchemaVersion` on the lock

- [ ] In `internal/manifest/manifest.go`, add the field:

```go
type Lock struct {
	AWFVersion    string           `json:"awfVersion"`
	SchemaVersion int              `json:"schemaVersion"`
	Files         map[string]Entry `json:"files"`
}
```

- [ ] Add `TestLoadOldLockZeroSchema` (manifest_test.go): a lock JSON without `schemaVersion`
  unmarshals with `SchemaVersion == 0`. (Backs ADR-0010's untagged "older lock → generation 0"
  contract; no `// invariant:` tag needed.)
- [ ] `go test ./internal/manifest/` → `ok`.

### Task 1.2 — Commit Phase 1

- [ ] `./x gate` → `0 issues.`
- [ ] `git add internal/manifest/ && git commit -m "feat(awf): add schemaVersion field to the lock"`
  (body: additive field, zero-value for older locks; no consumer until the migrate gate lands).

---

## Phase 2 — `internal/migrate` + `awf upgrade` (dormant)

Adds the whole migration package (frozen legacy reader, `tree-layout` migration, registry + gate
predicate + `Upgrade`) and the `upgrade` subcommand. **Nothing on the live path imports
`internal/migrate`**, and the gate is **not** wired into `runSync`/`runCheck` yet (that lands in
the Phase 3 cutover), so the binary still loads via the legacy `config.Load` and every commit is
gate-safe. All migrate logic is unit-tested against temp-dir fixtures.

### Task 2.1 — Frozen legacy reader

- [ ] Create `internal/migrate/legacy.go` carrying a **snapshot** of the pre-ADR-0009 config
  shape (`legacyConfig` with map-typed `Skills`/`Agents`/`Docs`, `AgentsDoc *legacySidecar`,
  `Vars`, `Hooks`, `Prefix`, `DocsDir`, `Invariants`) and `readLegacy(awfYAMLPath string)
  (*legacyConfig, error)` using a strict yaml decoder. This is the **only** reader of
  `.claude/awf.yaml` and must never import `internal/config`.
- [ ] Add `TestLegacyReadOnlyInMigrate` (migrate_test.go): a guard test asserting (a)
  `readLegacy` parses a fixture monolith, and (b) — by `go list`/import inspection or a comment
  contract — no non-migrate package references `.claude/awf.yaml`. Tag
  `// invariant: legacy-read-isolation`. (Co-owns the legacy-read exemption with `config-root`'s
  `TestLoadReadsTreeRoot`, landing in Phase 3 — the two are written to agree.)
- [ ] `go test ./internal/migrate/ -run TestLegacyReadOnlyInMigrate` → `ok`.

### Task 2.2 — `tree-layout` migration

- [ ] Create `internal/migrate/treelayout.go`: `applyTreeLayout(root string) error` reads
  `<root>/.claude/awf.yaml` via `readLegacy`, then writes `<root>/.claude/awf/config.yaml`
  (skeleton: `prefix`, `invariants`, `vars`, `hooks`, **`docsDir` when the legacy config set it
  non-default** — ADR-0009 Decision 2 lists `docsDir` as a skeleton field; carry it through so a
  non-`docs` adopter is not silently reset — and `skills`/`agents`/`docs` as sorted
  name arrays), one sidecar per target that had `data`/`sections`/`local`
  (`<root>/.claude/awf/<kind>/<name>.yaml`), every `replaceWith` part copied to its convention
  path, the agents-doc `data` (minus `ownership`/`identity`) to
  `<root>/.claude/awf/agents-doc.yaml`, and the `ownership`/`identity` scalars to
  `<root>/.claude/awf/parts/agents-doc/{you-and-this-project,identity}.md`. Finally removes
  `.claude/awf.yaml` and the legacy `.claude/awf.lock`. Idempotent: a no-op (returns nil) when
  `.claude/awf.yaml` is absent.
- [ ] **Heading caveat (byte-identical — load-bearing).** A convention part replaces the section's
  *entire inner body*, and in the agents-doc template that body **includes the `## You and this
  project` / `## Identity` heading lines** (they sit between the `awf:section`/`awf:end` markers —
  verified in `templates/agents-doc/AGENTS.md.tmpl:6-16`, unlike `docs/architecture`'s `body`
  section which is heading-free, so its existing part carries no heading). The migration must
  therefore write each agents-doc part as `## You and this project\n\n<ownership prose>` (resp.
  `## Identity\n\n<identity prose>`) — **heading + blank line included**, not the bare scalar —
  or the rendered `AGENTS.md` loses both headings at the cutover and the byte-identical claim
  fails. `TestTreeLayoutPortsMonolith` must assert each agents-doc part begins with its `## `
  heading.
- [ ] Add `TestTreeLayoutPortsMonolith` (migrate_test.go): given a fixture monolith, assert the
  produced tree (config.yaml arrays, a representative sidecar, a representative convention part,
  the two agents-doc parts) and that the legacy file is gone.
- [ ] `go test ./internal/migrate/ -run TestTreeLayoutPortsMonolith` → `ok`.

### Task 2.3 — Registry, gate predicate, auto-bump

- [ ] Create `internal/migrate/migrate.go`:

```go
package migrate

// A Migration ports a project from the generation below To up to To.
type Migration struct {
	To    int
	Name  string
	Apply func(root string) error
}

// registry is ordered ascending by To; current schema = last To.
var registry = []Migration{
	{To: 1, Name: "tree-layout", Apply: applyTreeLayout},
}

func Current() int { return registry[len(registry)-1].To }

// Generation reports the project's schema generation: 0 if the legacy single-file
// layout is present (.claude/awf.yaml exists and .claude/awf/config.yaml does not),
// else the lock's SchemaVersion (0 when no lock).
func Generation(root string) int {
	legacy := filepath.Join(root, ".claude", "awf.yaml")
	tree := filepath.Join(root, ".claude", "awf", "config.yaml")
	_, legacyErr := os.Stat(legacy)
	_, treeErr := os.Stat(tree)
	if legacyErr == nil && os.IsNotExist(treeErr) {
		return 0
	}
	l, err := manifest.Load(filepath.Join(root, ".claude", "awf", "awf.lock"))
	if err != nil {
		return 0
	}
	return l.SchemaVersion
}

// registryTos returns the To values of every registered migration.
func registryTos() []int {
	tos := make([]int, len(registry))
	for i, m := range registry {
		tos[i] = m.To
	}
	return tos
}

// gateStateFor is the pure classifier (extracted for testability): "ok" when gen is
// at/above current; "gate" when at least one To lands in the open interval
// (gen, current]; "autobump" otherwise.
func gateStateFor(gen, current int, tos []int) string {
	if gen >= current {
		return "ok"
	}
	for _, to := range tos {
		if to > gen && to <= current {
			return "gate"
		}
	}
	return "autobump"
}

// GateState classifies a project: "ok" | "gate" | "autobump".
func GateState(root string) string {
	return gateStateFor(Generation(root), Current(), registryTos())
}

// Upgrade applies every registered migration with To > Generation(root), in
// ascending To order, and returns the applied migration names. Idempotent: at the
// current generation it applies nothing and returns an empty slice, nil error.
func Upgrade(root string) ([]string, error) {
	from := Generation(root)
	var applied []string
	for _, m := range registry { // registry is already ascending by To
		if m.To <= from {
			continue
		}
		if err := m.Apply(root); err != nil {
			return applied, fmt.Errorf("migration %q (to %d): %w", m.Name, m.To, err)
		}
		applied = append(applied, m.Name)
	}
	return applied, nil
}
```

- [ ] Imports for `migrate.go`: `os`, `path/filepath`, `fmt`, and
  `github.com/hypnotox/agentic-workflows/internal/manifest` (read-only — `manifest.Load`; note
  this does **not** breach `legacy-read-isolation`, which is about `.claude/awf.yaml`, not the
  lock). The lock write at the highest applied `To` is performed by the terminal `runSync` in
  `runUpgrade` (Task 2.4), which stamps `SchemaVersion: Current()`; `Upgrade` itself does not
  write the lock.
- [ ] **Testability of the no-op gap:** with only `{To: 1}` registered, generation `0` always has
  a covering migration, so `"autobump"` is unreachable against the package `registry`. Make the
  predicate testable by extracting the pure core: `gateStateFor(gen, current int, tos []int) string`
  (and have `GateState` call it with `Generation(root)`, `Current()`, and the registry's `To` set).
  `TestNoopGapAutoBumps` then calls `gateStateFor` with a `tos` slice and a `(gen, current]` window
  that no element lands in — e.g. `gateStateFor(2, 5, []int{1, 2})` → `"autobump"`. Keep the
  file-touching `GateState`/`Generation` exercised by `TestGateBlocksWhenBehind` and
  `TestUpgradeAppliesInOrderIdempotent`.
- [ ] Add `TestGateBlocksWhenBehind` (`GateState` returns `"gate"` for a legacy project) tagged
  `// invariant: upgrade-gate`; `TestUpgradeAppliesInOrderIdempotent` (Upgrade ports then a
  second Upgrade is a no-op) tagged `// invariant: migration-ordering`; `TestNoopGapAutoBumps`
  (via `gateStateFor` per above → `"autobump"`) tagged `// invariant: noop-autobump`.
- [ ] `go test ./internal/migrate/` → `ok`.

### Task 2.4 — `awf upgrade` subcommand (dormant in dispatch)

- [ ] Create `cmd/awf/upgrade.go`: `runUpgrade(root string) error` calls `migrate.Upgrade`,
  prints each applied migration name (or "already current"), then runs `runSync(root)` to write
  the lock and verify render.
- [ ] In `cmd/awf/main.go`, add `case "upgrade": fatalIf(runUpgrade(cwd))` to the switch and
  `upgrade` to the usage string. Do **not** wire the gate yet.
- [ ] `go build ./cmd/awf` → succeeds. `go test ./...` → `ok` (no behavior change to the repo —
  `runUpgrade` is dispatchable but the live load path is still legacy `config.Load`).

### Task 2.5 — Commit Phase 2

- [ ] `./x gate` → `0 issues.`
- [ ] `git add internal/migrate/ cmd/awf/ && git commit -m "feat(awf): migrate registry, tree-layout migration, upgrade (dormant)"`
  (body: gate predicate and migration exist but are not wired into sync/check until the atomic
  cutover; `internal/migrate` has no live-path consumer).

---

## Phase 3 — ATOMIC cutover (one large single-concern commit)

The irreducible commit (see Bootstrap, option B). Everything that depends on the new tree-shaped
`config.Config` lands together — because Go will not compile a half-changed shared struct, and a
tree-shaped `config.Load` cannot parse the repo's still-legacy `.claude/awf.yaml`, so the repo must
be ported in the **same** commit. Concern: "cut over to the tree layout." Sub-steps 3.1–3.5 build
the change; 3.6 ports this repo and commits the lot atomically. Tests run against temp-dir trees
throughout; only step 3.6 touches the repo's own `.claude/`.

### Task 3.1 — Rewrite `internal/config` to the tree shape

- [ ] In `internal/config/config.go`: rename type `SkillConfig` to `Sidecar` (keep fields
  `Data map[string]any`, `Sections map[string]SectionOverride`, `Local bool`). Update
  `checkSections`'s parameter type to `Sidecar`.
- [ ] Change `Config` enable fields to arrays and drop `AgentsDoc`/`raw`, add `root`:

```go
type Config struct {
	Prefix     string           `yaml:"prefix"`
	DocsDir    string           `yaml:"docsDir"`
	Vars       map[string]any   `yaml:"vars"`
	Skills     []string         `yaml:"skills"`
	Agents     []string         `yaml:"agents"`
	Hooks      []string         `yaml:"hooks"`
	Docs       []string         `yaml:"docs"`
	Invariants *InvariantConfig `yaml:"invariants"`
	root       string // <project>/.claude/awf, for sidecar/part resolution
}
```

- [ ] Replace `Load(path string)` with `Load(awfDir string)` that reads
  `filepath.Join(awfDir, "config.yaml")` with the existing strict decoder, sets
  `c.root = awfDir`, defaults `DocsDir` to `"docs"`. Remove `Raw()`.
- [ ] Add sidecar loading (empty when the file is absent; strict decode otherwise):

```go
// Sidecar reads <root>/<kind>/<name>.yaml; agents-doc lives at <root>/agents-doc.yaml.
// A missing file yields a zero Sidecar (publication-safe: empty data/sections).
func (c *Config) Sidecar(kind, name string) (Sidecar, error) {
	var rel string
	if kind == "agents-doc" {
		rel = "agents-doc.yaml"
	} else {
		rel = filepath.Join(kind, name+".yaml")
	}
	b, err := os.ReadFile(filepath.Join(c.root, rel))
	if errors.Is(err, os.ErrNotExist) {
		return Sidecar{}, nil
	}
	if err != nil {
		return Sidecar{}, fmt.Errorf("read sidecar %s: %w", rel, err)
	}
	var s Sidecar
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return Sidecar{}, fmt.Errorf("parse sidecar %s: %w", rel, err)
	}
	return s, nil
}

// PartPath returns the convention part path for a section of a target.
func (c *Config) PartPath(kind, target, section string) string {
	if kind == "agents-doc" {
		return filepath.Join(c.root, "parts", "agents-doc", section+".md")
	}
	return filepath.Join(c.root, kind, "parts", target, section+".md")
}
```

- [ ] Update `Validate()`: drop the `Skills`/`Agents`/`Docs`/`AgentsDoc` map iterations (section
  checks move to sidecar-load time, Task 3.2); keep `Prefix`/`DocsDir`/`Invariants` checks.
- [ ] Add `"errors"` to imports if not present (it is).
- [ ] Add `TestLoadReadsTreeRoot` (write a temp `<dir>/config.yaml` **and** a sibling
  `<dir>/../awf.yaml` decoy; assert `Load(dir)` reads `config.yaml` and that the loader never reads
  the legacy `.claude/awf.yaml`) with `// invariant: config-root`. Per ADR-0010 Decision 3 this
  test co-owns the legacy-read exemption with `inv: legacy-read-isolation` (Phase 2's
  `TestLegacyReadOnlyInMigrate`): encode the migrate-package exemption here too (a comment contract
  or import-graph assertion that only `internal/migrate` reads the legacy path), so the two
  invariants are written to agree rather than splitting the contract across two independent tests.
  Add `TestEnableListsAreArrays` (assert `skills: [a, b]` parses to `[]string{"a","b"}`; a
  `data:` key at root is a parse error) with `// invariant: enable-arrays`.
- [ ] `go test ./internal/config/` → `ok` (this package compiles in isolation; the dependent
  packages below are fixed in the same commit before the package-set gate runs).

### Task 3.2 — Catalog validation, convention parts, drift projection in `internal/project`

- [ ] Rewrite `validateAgainstCatalog` to iterate the `[]string` enable lists, load each target's
  sidecar via `p.Cfg.Sidecar(kind, name)`, skip catalog/section checks when `sc.Local`, and run
  `checkSectionsAllowed` on `sc.Sections`. agents-doc: load `p.Cfg.Sidecar("agents-doc", "")` and
  check its sections unless `Local`. Keep the hooks-against-catalog check.
- [ ] Add the convention-part overlay helper (sidecar `drop`/explicit `replaceWith` win; otherwise,
  if `PartPath` exists, inject `SectionOverride{ReplaceWith: <abs part path>}`):

```go
// overlaySections returns sidecar sections with convention parts injected for any
// catalog-declared section that the sidecar did not drop or explicitly replace and
// whose convention part file exists. Precedence: drop > explicit replaceWith >
// convention part > template default.
func (p *Project) overlaySections(kind, target string, declared []string, sec map[string]config.SectionOverride) map[string]config.SectionOverride {
	out := map[string]config.SectionOverride{}
	for k, v := range sec {
		out[k] = v
	}
	for _, s := range declared {
		if _, set := out[s]; set {
			continue
		}
		pp := p.Cfg.PartPath(kind, target, s)
		if _, err := os.Stat(pp); err == nil {
			out[s] = config.SectionOverride{ReplaceWith: pp}
		}
	}
	return out
}
```

- [ ] Change `parts()` to read an absolute path verbatim (convention parts pass an absolute path)
  and resolve a legacy relative name under `<root>`:

```go
func (p *Project) parts() render.PartFunc {
	return func(name string) (string, error) {
		path := name
		if !filepath.IsAbs(name) {
			path = filepath.Join(p.Root, ".claude", "awf", name)
		}
		b, err := os.ReadFile(path)
		return string(b), err
	}
}
```

- [ ] Add the per-target drift projection helper (replaces whole-file `Raw()` hashing):

```go
// targetConfigHash projects the drift signal onto one rendered file: the prefix, the
// subset of vars the assembled template references, the target's sidecar (marshalled),
// and the bytes of every convention part it consumed — in deterministic order.
func (p *Project) targetConfigHash(assembled string, sc config.Sidecar, partPaths []string) (string, error) {
	refs := render.ReferencedVars(assembled)
	proj := map[string]any{"prefix": p.Cfg.Prefix, "layout": p.layout()}
	vs := map[string]any{}
	for _, r := range refs {
		vs[r] = p.Cfg.Vars[r]
	}
	proj["vars"] = vs
	proj["sidecar"] = sc
	sort.Strings(partPaths)
	parts := map[string]string{}
	for _, pp := range partPaths {
		b, err := os.ReadFile(pp)
		if err != nil {
			return "", err
		}
		parts[filepath.Base(filepath.Dir(pp))+"/"+filepath.Base(pp)] = manifest.Hash(b)
	}
	proj["parts"] = parts
	enc, err := yaml.Marshal(proj)
	if err != nil {
		return "", err
	}
	return manifest.Hash(enc), nil
}
```

- [ ] Add `"gopkg.in/yaml.v3"` to `project` imports.
- [ ] Add `TestConventionPartPrecedence` (project render_tree_test.go): a target whose catalog
  section has a convention part file renders the part; a sidecar `drop` beats the part; an
  explicit `replaceWith` beats the part. Tag `// invariant: parts-convention`.
- [ ] Add `TestSidecarAbsentRendersDefault`: an enabled target with no sidecar and no parts renders
  the template default with no `<no value>`. Tag `// invariant: sidecar-optional`.
- [ ] Add `TestPerTargetDriftProjection` (project drift_test.go): two targets with sidecars;
  editing target A's sidecar changes A's hash but not B's; editing a part A consumes changes A;
  an unrelated vars edit (a var A does not reference) does not change A's hash; and a sidecar/part
  for a target absent from the enable list is reported as an orphan (covering both clauses of the
  invariant — the orphan-walk lands in `Check`, Task 3.3). Tag `// invariant: drift-source-set`.
- [ ] `go test ./internal/project/ -run 'TestConventionPartPrecedence|TestSidecarAbsentRendersDefault|TestPerTargetDriftProjection'` → `ok`.

### Task 3.3 — Wire the tree loader, gate, and schemaVersion into the live path

- [ ] In `internal/project/project.go`: change `Open` to `config.Load(filepath.Join(root,
  ".claude", "awf"))`; change `lockPath()` to `filepath.Join(p.Root, ".claude", "awf",
  "awf.lock")`. Rewrite `RenderAll` to iterate the `[]string` enable lists, load each sidecar,
  apply `overlaySections`, skip `Local`, and render agents-doc always-on from its sidecar
  (`p.Cfg.Sidecar("agents-doc","")`) with `data["docs"]` from `resolvedDocs` (which now iterates
  `p.Cfg.Docs []string` + sidecars, loading each doc's sidecar to read its `Local` flag).
- [ ] **Apply `overlaySections` to agents-doc too** (kind `"agents-doc"`, empty target, declared =
  `p.Cat.AgentsDoc.Sections`). This is what injects this repo's `you-and-this-project`/`identity`
  convention parts (`PartPath` special-cases agents-doc to `<root>/parts/agents-doc/<section>.md`).
  Without it, the migrated `agents-doc.yaml` no longer carries `data.ownership`/`identity`, so the
  template's `{{ with .data.ownership }}` falls to its **generic default** and `AGENTS.md` changes
  at the cutover — breaking the byte-identical claim (Task 3.6) and the Phase-4 re-model's empty
  `git diff AGENTS.md`. The part injection must be live in this commit, before the Phase-4 template
  re-model.
- [ ] Replace `cfgHash := manifest.Hash(p.Cfg.Raw())` in `Sync` and `Check` with per-file
  `targetConfigHash` (compute the assembled source + consumed part paths per target). `Sync` writes
  `SchemaVersion: migrate.Current()`.
- [ ] In `Sync`/`Check`: validate local targets' on-disk frontmatter — for each `Local`
  skill/agent, derive its output path (the same `.claude/skills/<prefix>-<name>/SKILL.md` /
  `.claude/agents/<name>.md` formulas), read the file, and run `validateFrontmatter`; missing file
  or invalid frontmatter is an error/drift.
- [ ] In `Check` (and as a `Sync` warning/error), report **orphan sidecars/parts**: walk
  `.claude/awf/{skills,agents,docs}/*.yaml` and `.claude/awf/{skills,agents,docs}/parts/**`; any
  sidecar or part file whose `<target>` is not in the matching enable list (and the agents-doc
  parts when agents-doc is somehow disabled) is reported as `manifest.Drift{Kind: "orphaned"}`.
  This satisfies the second clause of `inv: drift-source-set` (orphan reporting), which the
  per-target hash projection alone does not cover.
- [ ] In `cmd/awf/main.go`: at the top of `runSync` and `runCheck`, before `project.Open`, call
  `migrate.GateState(root)` — on `"gate"` return an error
  `fmt.Errorf("config schema is behind (generation %d < %d); run awf upgrade", migrate.Generation(root), migrate.Current())`;
  on `"autobump"` proceed (Sync stamps the current version).
- [ ] Add `TestSyncStampsSchemaVersion` (project) tagged `// invariant: schema-version-lock`,
  `TestLocalFrontmatterChecked` tagged `// invariant: local-frontmatter`, and update existing
  `project`/`spine` tests to the tree fixtures (build a temp `.claude/awf/` instead of a monolith).

### Task 3.4 — Port `cmd/awf/list_add.go` to arrays + sidecars

- [ ] **Required in this commit (not Phase 4):** the `map → []string` field change breaks
  `cmd/awf/list_add.go` (`p.Cfg.Skills[n]` map index, `skillState(config.SkillConfig, bool)`), so it
  must compile here or `go build ./...` / `./x gate` is red. `runList` checks slice membership
  (`slices.Contains(p.Cfg.Skills, n)`) and loads the sidecar to compute state;
  `skillState(name, sidecar, enabled)` returns `available | enabled | tuned | local` (`tuned` =
  sidecar has data/sections; `local` = sidecar.Local). `runAdd` appends `- <skill>` under `skills:`
  in `.claude/awf/config.yaml` (replace `appendSkill`'s `name: {}` insertion with array-append,
  handling the `skills: []`/`skills:` forms), then `runSync`.
- [ ] Update `cmd/awf` list/add tests to the tree layout.

### Task 3.5 — Port `internal/project/scaffold.go` + `init` to emit the tree

- [ ] **Required in this commit:** `ScaffoldConfig` reads the new `Config` shape, so it must compile
  here. Change `ScaffoldConfig` to emit a skeleton `config.yaml` (vars seeded as today;
  `skills`/`agents`/`docs` as sorted `- name` arrays; `hooks` as today). In `cmd/awf/main.go`
  `runInit`, write it to `.claude/awf/config.yaml` (mkdir `.claude/awf`).
- [ ] Update `scaffold` tests.
- [ ] `go test ./...` → `ok` (whole package set now compiles on the tree shape and passes against
  temp-dir fixtures; the repo's own `.claude/` is still legacy and untouched until Task 3.6).

### Task 3.6 — Port this repo and commit atomically

- [ ] Run `go run ./cmd/awf upgrade` → prints `applied: tree-layout`; creates
  `.claude/awf/config.yaml`, sidecars, parts, `.claude/awf/agents-doc.yaml`,
  `.claude/awf/parts/agents-doc/{you-and-this-project,identity}.md`, `.claude/awf/awf.lock`
  (schemaVersion 1); removes `.claude/awf.yaml` and `.claude/awf.lock`.
- [ ] `go run ./cmd/awf check` → `awf check: clean` (rendered `.claude/**` and `AGENTS.md` are
  byte-identical — the agents-doc parts carry this repo's exact prior prose **including their `## `
  headings**, so no rendered file changes).
- [ ] `git status` shows: new `.claude/awf/**`, deleted `.claude/awf.yaml` + `.claude/awf.lock`,
  modified `internal/config/`, `internal/project/`, `cmd/awf/`; **no** changes under
  `.claude/skills/`, `.claude/agents/`, `.githooks/`, `AGENTS.md`, `docs/`.
- [ ] `./x gate` → `0 issues.`
- [ ] `git add -A .claude internal/config internal/project cmd/awf && git commit -m "feat(awf): cut over to .claude/awf tree and port repo via upgrade"`
  (body: atomic cutover — config tree shape + project/list_add/scaffold rewrites + gate +
  schemaVersion wired and the repo ported via `awf upgrade` in one commit, so the pre-commit
  `awf check` runs against the already-ported tree). Confirm pre-commit prints `awf check: clean`.

---

## Phase 4 — agents-doc re-model, CLI, docs, ADR flips

### Task 4.1 — Re-model the agents-doc template prose

- [ ] In `templates/agents-doc/AGENTS.md.tmpl`, replace the `{{ with .data.ownership }}{{ . }}
  {{ else }}…{{ end }}` in `you-and-this-project` and the `{{ with .data.identity }}…{{ end }}`
  in `identity` with **generic, adopter-neutral default prose written directly in the section
  body** (no `.data.ownership`/`.data.identity` reference). Leave `invariants`/`docMap` as
  `.data.*` loops. **Keep the `## You and this project` / `## Identity` heading lines inside the
  section markers**, exactly where they are today — the Phase-3 convention parts carry those
  headings (see Task 2.2 heading caveat), and `overlaySections` replaces the whole section body, so
  moving the heading out (or changing its text) would diverge the part-overridden render from the
  generic default and break `git diff AGENTS.md`-empty. Only the prose *under* the heading changes.
- [ ] Confirm this repo already supplies `.claude/awf/parts/agents-doc/{you-and-this-project,
  identity}.md` (written by the Task 3.6 port) so its rendered `AGENTS.md` stays byte-identical.
- [ ] `go run ./cmd/awf sync && go run ./cmd/awf check` → `awf check: clean`; `git diff
  AGENTS.md` is empty.
- [ ] Add `TestAgentsDocPartsOverride` (project) tagged `// invariant: agentsdoc-parts`: with a
  part present the section renders the part; absent, the generic default; both publication-safe
  with empty `invariants`/`docMap`.
- [ ] `./x gate` → `0 issues.`; `git add templates/agents-doc/ internal/project/ && git commit -m "refactor(awf): agents-doc prose via parts, generic template default"`.

> Note: `list`/`add` (`cmd/awf/list_add.go`) and `init`/`scaffold`
> (`internal/project/scaffold.go`) were already ported to the array+sidecar tree shape in the
> Phase 3 atomic cutover (Tasks 3.4 and 3.5) — they read the new `Config` and so had to compile in
> that commit. They are intentionally **not** repeated as Phase 4 commits.

### Task 4.2 — Doc-currency + ADR flips to Implemented

- [ ] Update `docs/architecture.md`: describe the `.claude/awf/` root (config.yaml, per-kind
  branches, sidecars, convention parts), the relocated `.claude/awf/awf.lock`, `internal/migrate`,
  and the `awf upgrade` command + version gate. (Architecture doc is rendered from
  `.claude/awf/docs/parts/architecture/body.md` — edit the part, then sync.)
- [ ] Edit the relocated agents-doc invariants data
  (`.claude/awf/agents-doc.yaml` `data.invariants`) so the "`awf check` is the drift oracle"
  bullet references `.claude/awf/config.yaml` (and any `.claude/awf.yaml` mention in the
  publication-safe bullet) and the broadened source set.
- [ ] Satisfy ADR-0010's Commands doc-currency obligation: the `commands` section currently
  renders the `vars.{testCmd,gateCmd,checkCmd}` default (no `data.commands` key in this repo),
  so add an `awf upgrade` line. Either (a) add a `commands:` list under
  `.claude/awf/agents-doc.yaml` `data` carrying the existing `./x` commands plus `awf upgrade`
  (the template's `{{ if .data.commands }}` branch then renders it), or (b) add a convention
  part `.claude/awf/parts/agents-doc/commands.md` that lists them. Pick (a) to keep the commands
  data-driven; this is an intended `AGENTS.md` change (not byte-identical) gated to this commit.
- [ ] Flip both ADRs to `Implemented` (`status: Implemented`) in
  `docs/decisions/0009-tree-based-config-layout.md` and `0010-versioned-lock-and-awf-upgrade.md`.
- [ ] `go run ./cmd/awf sync` (regenerates `ACTIVE.md`, re-renders `AGENTS.md`/architecture) →
  `awf sync: done`; `go run ./cmd/awf check` → `awf check: clean` (all 12 tagged slugs now
  backed, so the invariant check passes on Implemented ADRs).
- [ ] `./x gate` → `0 issues.`
- [ ] `git add docs/ .claude/ AGENTS.md && git commit -m "docs(awf): tree-layout doc currency; implement 0009 and 0010"`.

---

## Notes

- The plan is mutable while the ADRs are `Accepted`; it freezes when they flip to `Implemented`
  (Task 4.2).
- `go test ./...` uses embedded templates + temp-dir fixtures, never the repo's `.claude/`, so
  every Phase 1–2 commit is gate-safe despite the repo remaining on the legacy layout until the
  Phase 3 atomic cutover (the only commit that touches the repo's own `.claude/`).
- Design rationale is in ADR-0009 / ADR-0010 — this plan does not restate it.
