# Implementation complete (2026-07-01)

# Plan: single-source-of-truth cleanup (Track 2)

## Goal

Collapse the remaining duplicated path/dispatch/parse knowledge surfaced by the
2026 duplication review into single sources, and fix one latent overwrite bug. This
is the mechanical follow-on to Track 1 (`internal/testsupport`, ADR-0044). It applies
already-Implemented decisions (ADR-0027 unified kind descriptor, ADR-0026 config
serialization owned by `internal/config`); **no new ADR is warranted** — every change
is behaviour-preserving except the one bugfix, which gets a regression test first.

## Architecture summary

Six independent single-source moves plus a file split, each behind the existing
gates (100% coverage, `./x check` drift, `go vet`, golangci-lint):

1. **Kind dispatch** — extend `kindDescriptor` with a `tid` accessor; route
   `scaffold.go`, `render.go`, and `check.go`'s `orphans()` through the one table
   (ADR-0027) instead of hand-rolled per-kind enumerations.
2. **Config-tree paths (absolute)** — add `config.DirName`/`RootDir`/`ConfigPath`/
   `LockPath`; funnel the ~24 hand-typed `filepath.Join(root, ".awf", …)` literals
   through them.
3. **Convention-part paths (relative)** — derive `render.go`'s `partRel` from the
   absolute `config.PartPath` (one structural source); route the remaining relative
   `.awf` tokens through `config.DirName`.
4. **`internal/config` micro-dedup** — extract the 4× YAML-parse preamble in
   `edit.go`; extract the 3× path-separator check in `config.go`; drop the dead
   `len(lines)==0` guard in `internal/frontmatter`.
5. **`internal/migrate` dedup + bugfix** — extract the scalar-config-edit skeleton
   (`enablebootstrap`/`drophooks`); reuse `writeFile` in `relocatePart`; extract the
   3× section-override port branch; **guard `relocate` against clobbering an existing
   destination** (regression test first).
6. **Cross-package micro-dedup** — extract `sortBySlug` in `internal/invariants`;
   unify the two diverged ADR-filename regexes on one exported `internal/adr` source;
   extract the regenerate-and-compare shape shared by `checkActiveMD`/`checkDomainDocs`.
7. **`cmd/awf/main.go` split** — move `runInit`→`init.go`, `runSync`→`sync.go`,
   `gate`/`lockVsBinary`/`normalizeSemver`→`gate.go`, leaving `main.go` as the
   dispatch table.

## Tech stack

- Go 1.26, module `github.com/hypnotox/agentic-workflows`.
- Packages touched: `cmd/awf`, `internal/config`, `internal/project`,
  `internal/migrate`, `internal/invariants`, `internal/adr`, `internal/audit`,
  `internal/frontmatter`.
- No new dependencies. No template changes ⇒ **`./x check` must stay drift-free
  after every commit** (this is the primary regression signal — the refactor must
  not alter one byte of rendered output, including `awf:edit` pointers).

## File structure

**Created:**
- `cmd/awf/init.go`, `cmd/awf/sync.go`, `cmd/awf/gate.go` (Phase 7, moved code)
- `internal/migrate/configedit.go` (Phase 5, shared scalar-edit helper)

**Modified:**
- `internal/project/kind.go`, `scaffold.go`, `render.go`, `check.go`
- `internal/config/config.go`, `edit.go`
- `internal/migrate/enablebootstrap.go`, `drophooks.go`, `treelayout.go`,
  `dropreplacewith.go`, `singletonstandarddocs.go`, `relocation.go`, `migrate.go`
- `internal/invariants/invariants.go`, `internal/adr/adr.go`, `internal/audit/audit.go`
- `internal/frontmatter/frontmatter.go`
- `cmd/awf/main.go`, `cmd/awf/list_add.go`, `internal/project/project.go`, `install.go`
- Test files as each task requires.

**Deleted:** none.

## Scope exclusions (considered, deliberately out)

- **Reviewer-template spine dedup** — the render engine (`internal/render/render.go`)
  parses each template standalone; there is no cross-file partial mechanism. True
  source-level dedup needs a new engine capability = load-bearing = a future
  template-partials ADR. Excluded per user decision (2026-07-01). Mirrors the
  `render.go` RenderAll exclusion below.
- **`render.go` RenderAll/renderKindSpec fold into `kindDescriptors`** — ADR-0027
  Decision item 3 explicitly defers this to "a later ADR" (`render.go` carries a
  skills-only doc-gate closure the descriptor has no slot for). Left as accepted debt.
- **`internal/audit` rule-body extraction** — the four rules (`ruleDomainDocStaleness`,
  `ruleUndocumentedDomain`, `ruleDependencyADR`, `rulePlanForLargeChange`) share a
  structural *rhythm* but differ in accumulator type, filter direction, single-vs-multi
  finding, and extra filters. A unifying helper would be more obscure than the
  repetition — rejected as premature abstraction (awf's own convention-alignment lens).
- **Error-prefix alignment** across `cmd/awf`/`internal/adr`/`internal/changelog` —
  cosmetic; needs a decided convention (which prefix wins) first. Deferred to its own
  focused pass.

## Execution note

Recommended execution: **`awf-executing-plans` (inline)**. Phases repeatedly revisit
the same files (`config.go` in 2/4; `render.go` in 1/3; `check.go` in 1/3/6;
`migrate/*` in 2/5), so import blocks and helper call-sites accumulate — a fresh
subagent per task would keep re-deriving that shared state. Run phases in order.

---

## Phase 1 — Kind-dispatch single-sourcing

### Task 1.1 — Add the `tid` accessor to `kindDescriptor`

- [ ] In `internal/project/kind.go`, add a `tid` field to the `kindDescriptor` struct
      (after `outPath`, line 20):

  ```go
  	outPath   func(t Target, prefix, name string) string      // rendered path; nil for neutral kinds
  	tid       func(name string) string                        // embedded template id
  ```

- [ ] Populate `tid` in each of the four `kindDescriptors` entries:
  - skills: `tid: func(n string) string { return fmt.Sprintf("skills/%s/SKILL.md.tmpl", n) },`
  - agents: `tid: func(n string) string { return fmt.Sprintf("agents/%s.md.tmpl", n) },`
  - docs: `tid: func(n string) string { return fmt.Sprintf("docs/%s.md.tmpl", n) },`
  - domains: `tid: func(string) string { return "domains/domain.md.tmpl" },`

- [ ] Add `"fmt"` to `kind.go`'s import block (currently `maps`, `slices`, then the
      two internal imports).

- [ ] Verify it compiles: `go build ./internal/project/` → no output.

### Task 1.2 — Route `scaffold.go` var-collection through `tid`

- [ ] In `internal/project/scaffold.go`, replace the three per-kind `fmt.Sprintf`
      template-id constructions (lines 33-50) so each loop derives its `tid` from the
      descriptor. Replace this block:

  ```go
  	for name := range cat.Skills {
  		path := fmt.Sprintf("skills/%s/SKILL.md.tmpl", name)
  		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog skill name has a backing template in the embedded FS, so collectVars cannot fail
  			return nil, err
  		}
  	}
  	for name := range cat.Agents {
  		path := fmt.Sprintf("agents/%s.md.tmpl", name)
  		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog agent name has a backing template in the embedded FS, so collectVars cannot fail
  			return nil, err
  		}
  	}
  	for name := range cat.Docs {
  		path := fmt.Sprintf("docs/%s.md.tmpl", name)
  		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog doc name has a backing template in the embedded FS, so collectVars cannot fail
  			return nil, err
  		}
  	}
  ```

  with:

  ```go
  	for _, kind := range []string{"skills", "agents", "docs"} {
  		d, _ := descriptorByPlural(kind)
  		for _, name := range d.poolNames(cat) {
  			if err := collectVars(templates.FS, d.tid(name), varSet); err != nil { // coverage-ignore: every catalog name has a backing template in the embedded FS, so collectVars cannot fail
  				return nil, err
  			}
  		}
  	}
  ```

  Note: `d.poolNames(cat)` returns the sorted catalog pool for skills/agents/docs
  (all three have a non-nil `poolNames`), matching the prior `range cat.Skills` sets.

- [ ] `go build ./internal/project/` → no output.

### Task 1.3 — Route `render.go`'s docs/skills/agents `tid` closures through `tid`

- [ ] In `internal/project/render.go` `RenderAll`, replace the three inline `tid`
      closures with the descriptor accessor. For the docs spec (line 121):

  ```go
  		tid:      func(n string) string { return fmt.Sprintf("docs/%s.md.tmpl", n) },
  ```
  →
  ```go
  		tid:      mustDescriptor("docs").tid,
  ```

  For the skills spec (line 135):
  ```go
  				tid:      func(n string) string { return fmt.Sprintf("skills/%s/SKILL.md.tmpl", n) },
  ```
  →
  ```go
  				tid:      mustDescriptor("skills").tid,
  ```

  For the agents spec (line 148):
  ```go
  				tid:      func(n string) string { return fmt.Sprintf("agents/%s.md.tmpl", n) },
  ```
  →
  ```go
  				tid:      mustDescriptor("agents").tid,
  ```

- [ ] Add a `mustDescriptor` helper to `internal/project/kind.go` (the three call
      sites use static, always-present kind names, so the missing branch is unreachable):

  ```go
  // mustDescriptor returns the descriptor for a plural kind known to exist at the
  // call site (static kind literals in RenderAll).
  func mustDescriptor(kind string) kindDescriptor {
  	d, _ := descriptorByPlural(kind)
  	return d
  }
  ```

- [ ] If `fmt` is now unused in `render.go`, leave it — it is still used by the
      `generateActiveMD`/domain paths and the plainSingletons loop. Confirm with
      `go build ./internal/project/` → no output.

### Task 1.4 — Route `check.go`'s `orphans()` through `kindDescriptors`

- [ ] In `internal/project/check.go` `orphans()` (lines 69-138), replace the
      hardcoded `enabled` map and the `[]string{"skills","agents","docs","domains"}`
      loop header with a range over `kindDescriptors`. Replace lines 70-77:

  ```go
  	enabled := map[string]map[string]bool{
  		"skills":  sliceSet(p.Cfg.Skills),
  		"agents":  sliceSet(p.Cfg.Agents),
  		"docs":    sliceSet(p.Cfg.Docs),
  		"domains": sliceSet(p.Cfg.Domains),
  	}
  	var drift []manifest.Drift
  	for _, kind := range []string{"skills", "agents", "docs", "domains"} {
  ```

  with:

  ```go
  	var drift []manifest.Drift
  	for _, desc := range kindDescriptors {
  		kind := desc.Plural
  		enabledSet := sliceSet(desc.enable(p.Cfg))
  ```

- [ ] Update the two `enabled[kind][…]` lookups in the loop body (lines 91 and 110)
      to `enabledSet[…]`:
  - line 91: `if !enabled[kind][name] {` → `if !enabledSet[name] {`
  - line 110: `if !enabled[kind][t.Name()] {` → `if !enabledSet[t.Name()] {`

- [ ] `go build ./internal/project/` → no output.

### Task 1.5 — Extract the bridge/bootstrap template-id constants

The bridge and bootstrap template ids are literals in both `render.go` (as the tids
rendered) and `check.go`'s `isManagedMarkdown` (as the exclusion list) — a coupling.

- [ ] In `internal/project/render.go`, add near the top (after the imports):

  ```go
  const (
  	bridgeTID    = "claude/CLAUDE.md.tmpl"
  	bootstrapTID = "bootstrap/awf-bootstrap.sh.tmpl"
  )
  ```

- [ ] In `render.go`, replace the two literal uses: line 187
      `"claude/CLAUDE.md.tmpl"` → `bridgeTID`; line 213
      `"bootstrap/awf-bootstrap.sh.tmpl"` → `bootstrapTID`.

- [ ] In `internal/project/check.go`, update `isManagedMarkdown` (lines 152-154):

  ```go
  func isManagedMarkdown(tid string) bool {
  	return tid != bridgeTID && tid != bootstrapTID
  }
  ```

- [ ] `go build ./internal/project/` → no output.

### Task 1.6 — Verify and commit Phase 1

- [ ] `./x gate` → passes (100% coverage, tests, vet, lint).
- [ ] `./x check` → `awf check: no drift` (rendered output byte-identical).
- [ ] Commit: `refactor(awf): route kind dispatch through kindDescriptors`

---

## Phase 2 — Config-tree path primitives (absolute)

### Task 2.1 — Add the path helpers to `internal/config`

- [ ] In `internal/config/config.go`, add after the `Load` function (before
      `Sidecar`, ~line 115):

  ```go
  // DirName is the config-tree directory name at the project root.
  const DirName = ".awf"

  // RootDir returns the config-tree directory for a project root (<root>/.awf).
  func RootDir(root string) string { return filepath.Join(root, DirName) }

  // ConfigPath returns the skeleton config.yaml path for a project root.
  func ConfigPath(root string) string { return filepath.Join(RootDir(root), "config.yaml") }

  // LockPath returns the awf.lock path for a project root.
  func LockPath(root string) string { return filepath.Join(RootDir(root), "awf.lock") }
  ```

  (`filepath` is already imported by `config.go`.)

- [ ] Point `Load` at the new helper — the caller already passes `<root>/.awf`, so
      leave `Load`'s body as-is; the helpers are for callers.

- [ ] Add a unit test `internal/config/config_test.go` (or extend the existing one)
      pinning the three helpers:

  ```go
  func TestPathHelpers(t *testing.T) {
  	root := filepath.Join("x", "y")
  	if got, want := RootDir(root), filepath.Join("x", "y", ".awf"); got != want {
  		t.Errorf("RootDir = %q, want %q", got, want)
  	}
  	if got, want := ConfigPath(root), filepath.Join("x", "y", ".awf", "config.yaml"); got != want {
  		t.Errorf("ConfigPath = %q, want %q", got, want)
  	}
  	if got, want := LockPath(root), filepath.Join("x", "y", ".awf", "awf.lock"); got != want {
  		t.Errorf("LockPath = %q, want %q", got, want)
  	}
  }
  ```

- [ ] `go test ./internal/config/` → ok.

### Task 2.2 — Route the `config.yaml` literals

Replace each `filepath.Join(<root>, ".awf", "config.yaml")` with
`config.ConfigPath(<root>)`. Add the `internal/config` import to any file lacking it
(`cmd/awf/main.go`, `internal/migrate/migrate.go` need it added; `list_add.go` and the
migrate `drophooks`/`enablebootstrap` files already import config).

- [ ] In `internal/migrate/drophooks.go` and `internal/migrate/enablebootstrap.go` the
      replaced literal is the *only* `path/filepath` use, so drop `path/filepath` from
      both import blocks in this same commit — otherwise the Phase 2 `go build`/gate
      fails on an unused import. (`os` stays until Phase 5 rewrites their bodies.)

- [ ] `cmd/awf/list_add.go:38, :82, :206` — `filepath.Join(root, ".awf", "config.yaml")` → `config.ConfigPath(root)`
- [ ] `cmd/awf/main.go:404` — `cfgPath := filepath.Join(root, ".awf", "config.yaml")` → `cfgPath := config.ConfigPath(root)` (add config import)
- [ ] `internal/migrate/drophooks.go:16` — → `config.ConfigPath(root)`
- [ ] `internal/migrate/enablebootstrap.go:18` — → `config.ConfigPath(root)`
- [ ] `internal/migrate/migrate.go:41` — `newTree := filepath.Join(root, ".awf", "config.yaml")` → `newTree := config.ConfigPath(root)` (add config import)

### Task 2.3 — Route the `awf.lock` literals

Replace each `filepath.Join(<root>, ".awf", "awf.lock")` with `config.LockPath(<root>)`.

- [ ] `cmd/awf/main.go:492` — `manifest.Load(filepath.Join(root, ".awf", "awf.lock"))` → `manifest.Load(config.LockPath(root))`
- [ ] `internal/migrate/migrate.go:45, :68` — → `config.LockPath(root)`
- [ ] `internal/project/install.go:90` — → `config.LockPath(root)` (add config import)
- [ ] `internal/project/project.go:185` — `lockPath()` body `filepath.Join(p.Root, ".awf", "awf.lock")` → `config.LockPath(p.Root)`

### Task 2.4 — Route the bare `.awf` root literals

- [ ] `cmd/awf/list_add.go:224` — `awf := filepath.Join(root, ".awf")` → `awf := config.RootDir(root)`
- [ ] `internal/migrate/singletonstandarddocs.go:25` — `awfDir := filepath.Join(root, ".awf")` → `awfDir := config.RootDir(root)` (add config import)
- [ ] `internal/migrate/relocation.go:15` — `newDir := filepath.Join(root, ".awf")` → `newDir := config.RootDir(root)` (add config import; leave `oldDir` = `.claude/awf` legacy literal)
- [ ] `internal/project/project.go:33` — `config.Load(filepath.Join(root, ".awf"))` → `config.Load(config.RootDir(root))`

### Task 2.5 — Verify and commit Phase 2

- [ ] `go build ./...` → no output (catches any missing/unused import).
- [ ] `./x gate` → passes.
- [ ] `./x check` → no drift.
- [ ] Commit: `refactor(awf): funnel config-tree paths through config helpers`

---

## Phase 3 — Convention-part paths (relative), single-sourced

### Task 3.1 — Derive `partRel` from `PartPath`

- [ ] In `internal/project/render.go`, replace the free function `partRel` (lines
      38-44):

  ```go
  // partRel is the project-relative convention part path the awf:edit pointer names.
  func partRel(kind, artifact, section string) string {
  	if config.IsSingletonKind(kind) {
  		return ".awf/parts/" + kind + "/" + section + ".md"
  	}
  	return ".awf/" + kind + "/parts/" + artifact + "/" + section + ".md"
  }
  ```

  with a method deriving the relative form from the absolute `PartPath` (one
  structural source; `ToSlash` keeps the awf:edit pointer forward-slashed):

  ```go
  // partRel is the project-relative convention part path the awf:edit pointer names,
  // derived from the absolute PartPath so the parts-path structure has one source.
  func (p *Project) partRel(kind, artifact, section string) string {
  	rel, err := filepath.Rel(p.Root, p.Cfg.PartPath(kind, artifact, section))
  	if err != nil { // coverage-ignore: PartPath is always rooted under p.Root, so Rel cannot fail
  		return ""
  	}
  	return filepath.ToSlash(rel)
  }
  ```

- [ ] Update the caller in `planSections` (render.go:52):
      `EditPath: partRel(kind, artifact, s)` → `EditPath: p.partRel(kind, artifact, s)`.

- [ ] Add `"path/filepath"` to `render.go`'s import block.

- [ ] **Critical verify:** `./x check` → no drift. The awf:edit pointers in every
      rendered skill/agent/doc must be byte-identical; a drift here means `ToSlash`/
      `Rel` diverged from the old string form.

### Task 3.2 — Route the remaining relative `.awf` tokens through `config.DirName`

- [ ] `internal/project/project.go:216` — `DomainsPartsDir: ".awf/domains/parts"` →
      `DomainsPartsDir: config.DirName + "/domains/parts"`.
- [ ] `internal/project/check.go:78` — `base := filepath.Join(p.Root, ".awf", kind)` →
      `base := filepath.Join(config.RootDir(p.Root), kind)` (add config import to check.go).
- [ ] `internal/project/check.go:93` — `filepath.Join(".awf", kind, e.Name())` →
      `filepath.Join(config.DirName, kind, e.Name())`.
- [ ] `internal/project/check.go:112` — `filepath.Join(".awf", kind, "parts", t.Name())` →
      `filepath.Join(config.DirName, kind, "parts", t.Name())`.
- [ ] `internal/project/check.go:129` — `filepath.Join(".awf", kind, "parts", t.Name(), sf.Name())` →
      `filepath.Join(config.DirName, kind, "parts", t.Name(), sf.Name())`.
- [ ] `internal/project/install.go:93` — the error-message path
      `filepath.Join(".awf", "awf.lock")` → `filepath.Join(config.DirName, "awf.lock")`.

### Task 3.3 — Verify and commit Phase 3

- [ ] `./x gate` → passes.
- [ ] `./x check` → no drift.
- [ ] Commit: `refactor(awf): derive convention-part paths from PartPath`

---

## Phase 4 — `internal/config` + `internal/frontmatter` micro-cleanups

### Task 4.1 — Extract the YAML-parse preamble in `edit.go`

- [ ] In `internal/config/edit.go`, add a helper (near `encode`, ~line 182):

  ```go
  // parseMapping decodes src into a YAML document and returns the document plus its
  // root mapping node, the shared preamble of every awf-owned config.yaml edit.
  func parseMapping(src []byte) (doc *yaml.Node, root *yaml.Node, err error) {
  	doc = &yaml.Node{}
  	if err := yaml.Unmarshal(src, doc); err != nil {
  		return nil, nil, fmt.Errorf("config: parse: %w", err)
  	}
  	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
  		return nil, nil, errors.New("config: not a YAML mapping")
  	}
  	return doc, doc.Content[0], nil
  }
  ```

- [ ] In each of `SetArrayMember`, `SetArray`, `RemoveKey`, `SetMappingScalar`,
      replace the 8-line preamble:

  ```go
  	var doc yaml.Node
  	if err := yaml.Unmarshal(src, &doc); err != nil {
  		return nil, fmt.Errorf("config: parse: %w", err)
  	}
  	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
  		return nil, errors.New("config: not a YAML mapping")
  	}
  	root := doc.Content[0]
  ```

  with:

  ```go
  	doc, root, err := parseMapping(src)
  	if err != nil {
  		return nil, err
  	}
  ```

  Each function's tail already ends `return encode(&doc)`. Because `doc` is now a
  `*yaml.Node`, change each `encode(&doc)` to `encode(doc)`. Verify every `doc`/`root`
  reference in each body still compiles (they operate on `root`, which is unchanged).

- [ ] `go test ./internal/config/` → ok (existing edit tests cover all four funcs and
      the error branches, keeping 100% coverage of `parseMapping`).

- [ ] `./x gate` → passes. Commit: `refactor(awf): share the config.yaml parse preamble`

### Task 4.2 — Extract the path-separator check in `config.go`

- [ ] In `internal/config/config.go`, add (near `validateBasenameGlob`, ~line 216):

  ```go
  // hasPathSep reports whether s contains a path separator or a ".." segment — the
  // shared reject condition for prefix/target/domain names.
  func hasPathSep(s string) bool {
  	return strings.ContainsAny(s, "/\\") || strings.Contains(s, "..")
  }
  ```

- [ ] Replace the three uses, keeping each site's distinct message:
  - Prefix (lines 159-161): `if strings.ContainsAny(c.Prefix, "/\\") || strings.Contains(c.Prefix, "..") {` → `if hasPathSep(c.Prefix) {`
  - Targets (line 196): `if t == "" || strings.ContainsAny(t, "/\\") || strings.Contains(t, "..") {` → `if t == "" || hasPathSep(t) {`
  - `ValidateDomainName` (line 210): `if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {` → `if hasPathSep(name) {`
  - Leave the `DocsDir` check (`HasPrefix(c.DocsDir, "/")` variant) untouched — it is a
    different predicate.

- [ ] `go test ./internal/config/` → ok. `./x gate` → passes.
      Commit: `refactor(awf): share the config path-separator check`

### Task 4.3 — Drop the dead guard in `internal/frontmatter`

- [ ] In `internal/frontmatter/frontmatter.go` `Split` (line 18), remove the
      unreachable `len(lines) == 0 ||` disjunct (`bytes.SplitAfter` never returns an
      empty slice, so `lines[0]` is always safe):

  ```go
  	if len(lines) == 0 || strings.TrimRight(string(lines[0]), "\r\n") != "---" {
  ```
  →
  ```go
  	if strings.TrimRight(string(lines[0]), "\r\n") != "---" {
  ```

- [ ] `go test ./internal/frontmatter/` → ok (coverage stays 100%; the removed
      disjunct was never a taken branch).

- [ ] `./x gate` → passes. Commit: `refactor(awf): drop unreachable frontmatter guard`

---

## Phase 5 — `internal/migrate` dedup + relocate bugfix

### Task 5.1 — Extract the scalar-config-edit skeleton

`enablebootstrap.go` and `drophooks.go` share the read / `IsNotExist`-noop /
`coverage-ignore` permission arm / mutate / `WriteFile(0o644)` skeleton, differing
only in the single mutation call.

- [ ] Create `internal/migrate/configedit.go`:

  ```go
  package migrate

  import (
  	"os"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  )

  // editConfig applies mutate to the project's config.yaml, routing serialization
  // through internal/config (ADR-0026). A config absent on disk is a no-op
  // (idempotent re-run safe) — the shared skeleton of the scalar-edit migrations.
  func editConfig(root string, mutate func(src []byte) ([]byte, error)) error {
  	cfgPath := config.ConfigPath(root)
  	src, err := os.ReadFile(cfgPath)
  	if os.IsNotExist(err) {
  		return nil
  	}
  	if err != nil { // coverage-ignore: ReadFile faults only on a permission error that the test root bypasses
  		return err
  	}
  	out, err := mutate(src)
  	if err != nil {
  		return err
  	}
  	return os.WriteFile(cfgPath, out, 0o644)
  }
  ```

- [ ] Rewrite `internal/migrate/enablebootstrap.go`'s `applyEnableBootstrap` body
      (keep the doc comment) to:

  ```go
  func applyEnableBootstrap(root string) error {
  	return editConfig(root, func(src []byte) ([]byte, error) {
  		return config.SetMappingScalar(src, "bootstrap", "enabled", true)
  	})
  }
  ```

- [ ] Rewrite `internal/migrate/drophooks.go`'s `applyDropHooks` body (keep the doc
      comment) to:

  ```go
  func applyDropHooks(root string) error {
  	return editConfig(root, func(src []byte) ([]byte, error) {
  		return config.RemoveKey(src, "hooks")
  	})
  }
  ```

- [ ] Trim each file's now-unused imports (`os`, `path/filepath` drop out of both;
      `config` stays). `go build ./internal/migrate/` → no output.

- [ ] `go test ./internal/migrate/` → ok. `./x gate` → passes.
      Commit: `refactor(awf): share the migrate scalar-config-edit skeleton`

### Task 5.2 — Reuse `writeFile` in `relocatePart`

- [ ] In `internal/migrate/dropreplacewith.go` `relocatePart` (lines 128-131), replace
      the inline MkdirAll+WriteFile tail:

  ```go
  	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { // coverage-ignore: dst parent under awfDir is writable when the sidecar was readable
  		return err
  	}
  	return os.WriteFile(dst, in, 0o644)
  ```

  with a call to the package helper (`treelayout.go:197`):

  ```go
  	return writeFile(dst, in)
  ```

  Note `writeFile` has no `coverage-ignore` on its MkdirAll; the branch is already
  covered by other `writeFile` callers, so coverage stays 100%.

- [ ] Remove `path/filepath` from `dropreplacewith.go` imports if it is now unused
      (check remaining references first). `go build ./internal/migrate/` → no output.

- [ ] `go test ./internal/migrate/` → ok. `./x gate` → passes.
      Commit: `refactor(awf): reuse writeFile in relocatePart`

### Task 5.3 — Extract the section-override port branch

`portSidecar` (treelayout.go:88-101), `portAgentsDoc` (treelayout.go:133-146), and
`convertSidecar` (dropreplacewith.go) each carry the same
`replaceWith→copyPart / drop→keep` loop, differing only in the destination-path
builder. Extract a helper parameterized by that builder.

- [ ] In `internal/migrate/treelayout.go`, add:

  ```go
  // portSectionOverrides walks a legacy sidecar's section overrides: each replaceWith
  // section is copied out to the convention part at dst(sec); each drop is preserved
  // in the returned keptSections map. The shared body of the tree-layout ports.
  func portSectionOverrides(sections map[string]legacySectionOverride, dst func(sec string) string) (map[string]any, error) {
  	kept := map[string]any{}
  	for _, sec := range slices.Sorted(maps.Keys(sections)) {
  		ov := sections[sec]
  		if ov.ReplaceWith != "" {
  			if err := copyPart(filepath.Join(awfSrc(ov)), dst(sec)); err != nil {
  				return nil, err
  			}
  			continue
  		}
  		if ov.Drop {
  			kept[sec] = map[string]any{"drop": true}
  		}
  	}
  	return kept, nil
  }
  ```

  **Before writing this task's code, read `treelayout.go` and `dropreplacewith.go` in
  full** to confirm: (a) the exact `legacySectionOverride` type name and its
  `ReplaceWith`/`Drop` fields; (b) how `copyPart`'s source path is built in each caller
  (the `filepath.Join(awfDir, ov.ReplaceWith)` form) — thread `awfDir` in as needed
  rather than the `awfSrc` placeholder above; (c) whether `convertSidecar`'s `changed`
  flag / `relocatePart` (not `copyPart`) usage makes it diverge enough that only
  `portSidecar`/`portAgentsDoc` should share the helper. **If `convertSidecar` differs
  materially (it uses `relocatePart` + a `changed` bool), scope the helper to the two
  `treelayout.go` callers only and leave `convertSidecar` as-is** — note that decision
  in the commit body.

- [ ] Rewrite `portSidecar` to call the helper with
      `dst: func(sec string) string { return filepath.Join(awfDir, kind, "parts", name, sec+".md") }`,
      then `return writeSidecarDoc(filepath.Join(awfDir, kind, name+".yaml"), sc.Data, kept, sc.Local, false)`.

- [ ] Rewrite `portAgentsDoc`'s section loop (lines 133-146) to call the helper with
      `dst: func(sec string) string { return filepath.Join(awfDir, "parts", "agents-doc", sec+".md") }`,
      keeping its preamble (lines 111-132) unchanged.

- [ ] `go test ./internal/migrate/` → ok (the tree-layout migration tests exercise
      both ports; behaviour must stay byte-identical). `./x gate` → passes.
      Commit: `refactor(awf): extract the section-override port branch`

### Task 5.4 (bugfix, test-first) — Guard `relocate` against overwrite

`singletonstandarddocs.go`'s `relocate` calls `os.Rename` with no destination check,
so it silently clobbers an existing destination file — unlike `relocation.go`'s
`applyAwfRelocation`, which refuses. Add the guard, test first.

- [ ] **Failing test first.** In `internal/migrate/singletonstandarddocs_test.go`, add
      a case: create a source file and a *pre-existing* destination file with different
      content, call the To:6 migration path (or `relocate` directly if it is reachable
      from the test package), and assert it returns an error rather than overwriting.
      Run `go test ./internal/migrate/ -run <newtest>` → **fails** (currently clobbers).

- [ ] Add the guard to `relocate` (singletonstandarddocs.go, between the src-existence
      check and the `MkdirAll`, mirroring `relocation.go:19-21`):

  ```go
  	if _, err := os.Stat(dst); err == nil {
  		return fmt.Errorf("cannot relocate: %s already exists", dst)
  	}
  ```

  Add `"fmt"` to the file's imports if absent.

- [ ] Update `relocate`'s doc comment to state the refuse-on-existing-destination
      contract (mirroring `relocation.go`'s comment).

- [ ] `go test ./internal/migrate/` → the new test passes; existing tests still pass.
      Re-examine the two `// coverage-ignore` comments on the `applySingletonStandardDocs`
      caller (lines 27, 30) — the new error path may now be reachable from a test;
      adjust or add coverage as the gate requires.

- [ ] `./x gate` → passes. Commit: `fix(awf): refuse to clobber an existing relocate destination`

---

## Phase 6 — Cross-package micro-dedup

### Task 6.1 — Extract `sortBySlug` in `internal/invariants`

- [ ] In `internal/invariants/invariants.go`, add:

  ```go
  // sortBySlug orders findings by slug, the stable output order of both scans.
  func sortBySlug(f []Finding) {
  	sort.Slice(f, func(i, j int) bool { return f[i].Slug < f[j].Slug })
  }
  ```

  **Confirm first** that both sort sites (line 108 in the `mk` closure, line 126 in
  `Check`) sort the same element type `[]Finding`. The dossier reports line 108 sorts a
  local `out` and line 126 sorts `findings` — verify `out`'s element type is `Finding`;
  if `out` is a different type, keep 6.1 scoped to the two `Finding` sites only.

- [ ] Replace both `sort.Slice(x, func(i, j int) bool { return x[i].Slug < x[j].Slug })`
      calls with `sortBySlug(x)`.

- [ ] `go test ./internal/invariants/` → ok. `./x gate` → passes.
      Commit: `refactor(awf): extract sortBySlug in invariants`

### Task 6.2 — Unify the ADR-filename regex

`internal/adr/adr.go`'s `fileRe` (`^(\d{4})-.*\.md$`, capturing, used by `ParseDir`)
and `internal/audit/audit.go`'s `adrNameRe` (`^\d{4}-.+\.md$`, match-only) encode the
same "is this an ADR file" rule with a drift risk. Unify on one exported source in
`internal/adr`.

- [ ] In `internal/adr/adr.go`, keep the capturing regex (ParseDir reads group 1) but
      make it the single exported source and adopt the stricter `.+` middle (real ADR
      filenames always have a non-empty slug; `.*`→`.+` only rejects the theoretical
      empty-slug `0001-.md`). Rename/export:

  ```go
  // FilenameRe matches an ADR filename (NNNN-slug.md); group 1 is the 4-digit number.
  var FilenameRe = regexp.MustCompile(`^(\d{4})-.+\.md$`)
  ```

  Update `fileRe`'s in-file uses (`FindStringSubmatch` at adr.go:46, and the
  `coverage-ignore` comment referencing `fileRe` at adr.go:214) to `FilenameRe`.

- [ ] **Verify no test depends on the empty-slug edge.** Run
      `rg -n '0[0-9]{3}-\.md|"[0-9]{4}-\.md"' internal/` — expect no hits. If any test
      feeds an empty-slug filename, keep the `.*` form instead and only share the regex
      *value* (still exported), noting the choice in the commit body.

- [ ] In `internal/audit/audit.go`, replace `adrNameRe` and its `isADRFile` use:
  - Delete the `var adrNameRe = regexp.MustCompile(`^\d{4}-.+\.md$`)` declaration (line 100).
  - In `isADRFile` (line 342): `adrNameRe.MatchString(filepath.Base(path))` →
    `adr.FilenameRe.MatchString(filepath.Base(path))`.
  - Add the `internal/adr` import to `audit.go`. **Confirm no import cycle**:
    `internal/adr` must not import `internal/audit` (it imports only `internal/frontmatter`).
    Run `go build ./internal/audit/` → no output confirms it.
  - If `regexp` is now unused in `audit.go`, drop it from the imports.

- [ ] `go test ./internal/adr/ ./internal/audit/` → ok. `./x gate` → passes.
      Commit: `refactor(awf): unify the ADR-filename regex on internal/adr`

### Task 6.3 — Extract the regenerate-and-compare shape in `check.go`

`checkActiveMD` (check.go:249-256) and the per-doc body of `checkDomainDocs`
(check.go:262-273) share the "read on-disk file; missing → drift; hash-mismatch →
stale" shape.

- [ ] In `internal/project/check.go`, add:

  ```go
  // regenDrift compares a freshly-generated file's content against its on-disk copy:
  // a missing file or a hash mismatch yields one drift entry with the given details.
  func (p *Project) regenDrift(rel, content, missingDetail, staleDetail string) []manifest.Drift {
  	onDisk, err := os.ReadFile(filepath.Join(p.Root, rel))
  	if err != nil {
  		return []manifest.Drift{{Path: rel, Kind: "missing", Detail: missingDetail}}
  	}
  	if manifest.Hash(onDisk) != manifest.Hash([]byte(content)) {
  		return []manifest.Drift{{Path: rel, Kind: "stale", Detail: staleDetail}}
  	}
  	return nil
  }
  ```

- [ ] Rewrite `checkActiveMD` to:

  ```go
  func (p *Project) checkActiveMD(activeMdRel string, amd RenderedFile) []manifest.Drift {
  	return p.regenDrift(activeMdRel, amd.Content,
  		"ADR index absent; run awf sync", "ADR index out of date; run awf sync")
  }
  ```

- [ ] In `checkDomainDocs`, replace the per-doc read/compare block (lines 266-272) —
      inside the `for _, dd := range dds` loop — with:

  ```go
  		produced[dd.Path] = true
  		drift = append(drift, p.regenDrift(dd.Path, dd.Content,
  			"domain doc absent; run awf sync", "domain doc out of date; run awf sync")...)
  ```

  Leave the trailing orphan loop (lines 274-278) unchanged.

- [ ] `go test ./internal/project/` → ok. `./x gate` → passes.
      Commit: `refactor(awf): share the regenerate-and-compare drift check`

---

## Phase 7 — `cmd/awf/main.go` split

Pure file moves within `package main`; no behaviour change. `main.go` (524 lines)
becomes the dispatch table + arg helpers; the three command bodies move to their own
files, matching the existing one-file-per-command layout (`audit.go`, `check.go`, …).

### Task 7.1 — Move `runInit` to `init.go`

- [ ] Create `cmd/awf/init.go` (`package main`) and move `runInit` (main.go:372-442)
      into it verbatim.
- [ ] Add the imports `runInit` needs to `init.go`: `fmt`, `os`, `path/filepath`
      (runInit still calls `filepath.Dir`/`filepath.Base` at former main.go:407/410/431),
      `strings`,
      `github.com/hypnotox/agentic-workflows/internal/catalog`,
      `github.com/hypnotox/agentic-workflows/internal/config`,
      `github.com/hypnotox/agentic-workflows/internal/initspec`,
      `github.com/hypnotox/agentic-workflows/internal/project`,
      `github.com/hypnotox/agentic-workflows/templates`. (`runInit` references
      `config.ConfigPath`/`config.RootDir` after Phase 2, `initspec.*`, `project.*`,
      `catalog.Load`, `templates.FS`, `stdin`, `isInteractive` — the latter two stay in
      `main.go` and are package-scoped, so no import needed for them.)
- [ ] Remove `runInit` from `main.go`.

### Task 7.2 — Move `runSync` to `sync.go`

- [ ] Create `cmd/awf/sync.go` (`package main`) and move `runSync` (main.go:504-524)
      into it verbatim.
- [ ] Add its imports: `fmt`, `io`,
      `github.com/hypnotox/agentic-workflows/internal/project`.
- [ ] Remove `runSync` from `main.go`.

### Task 7.3 — Move `gate` and its helpers to `gate.go`

- [ ] Create `cmd/awf/gate.go` (`package main`) and move `normalizeSemver`
      (main.go:448-454), `gate` (main.go:467-485), and `lockVsBinary` (main.go:491-502)
      into it verbatim. **Move each function together with its preceding doc comment**
      (normalizeSemver: 444-447; gate: 456-466; lockVsBinary: 487-490). gate's doc
      comment ends with `// invariant: version-compat-gate` (main.go:466) — that marker
      must travel with the function into `gate.go`, or it is orphaned in `main.go`
      (still backs the slug, but detaches it from its function and strands a doc comment
      with no body).
- [ ] Add its imports: `fmt`, `strings`,
      `github.com/hypnotox/agentic-workflows/internal/config`,
      `github.com/hypnotox/agentic-workflows/internal/manifest`,
      `github.com/hypnotox/agentic-workflows/internal/migrate`, `golang.org/x/mod/semver`.
      (`lockVsBinary` uses `config.LockPath` after Phase 2 and `manifest.Load`.)
- [ ] Remove the three functions from `main.go`.

### Task 7.4 — Fix up `main.go`'s now-unused imports and verify

- [ ] After the moves, prune `main.go`'s import block to what the dispatch table +
      arg helpers still use (`errors`, `fmt`, `io`, `os`, `slices`, `strings`, plus
      `catalog`/`initspec`/`project`/`templates` only if still referenced — `runInit`'s
      departure likely drops `catalog`, `initspec`, `manifest`, `migrate`, `semver`,
      `config`; let `go build` and `goimports` decide).
- [ ] `go build ./cmd/awf/` → no output. `go vet ./cmd/awf/` → clean.
- [ ] Existing `cmd/awf` tests are unchanged (same package, same symbols) — they must
      pass as-is: `go test ./cmd/awf/` → ok.
- [ ] `./x gate` → passes. `./x check` → no drift.
- [ ] Commit: `refactor(awf): split main.go into init/sync/gate files`

---

## Terminal step

Invoke `awf-reviewing-plan` on this plan (`docs/plans/2026-07-01-single-source-of-truth-cleanup.md`)
before any implementation. There is no linked ADR (mechanical application of
ADR-0027/ADR-0026), so after review findings are resolved the reviewing skill routes
straight to implementation — no plan↔ADR resync. Commit the plan itself with
`docs(plans): add 2026-07-01-single-source-of-truth-cleanup`. Then execute via
`awf-executing-plans` (inline; see Execution note).
