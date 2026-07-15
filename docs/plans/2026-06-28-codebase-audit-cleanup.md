# Implementation complete (2026-06-28)

# Plan: Codebase audit cleanup

**Date:** 2026-06-28
**ADR:** [ADR-0027](../decisions/0027-unified-kind-descriptor.md) (covers Phase 5 only; Phases 1-4 are routine refactors/fixes, no ADR).

## Goal

Apply the findings from the full-codebase audit: mechanical cleanups, internal-cohesion
refactors, a CLI strictness fix, the audit-resolution re-home, and the unified kind-descriptor
table. No change to rendered output, lock format, or config schema except where noted (Phase 3
changes CLI exit codes and flag handling; both are pre-1.0, intentional).

## Architecture summary

The work is ordered smallest-blast-radius first. Phases 1-2 are internal refactors with no
behavioural change. Phase 3 is the one user-visible behaviour change (CLI flag rejection + exit
codes). Phase 4 re-homes audit resolution from `internal/config` into `internal/audit`,
mirroring the existing `internal/invariants` boundary. Phase 5 implements ADR-0027 and flips it
to `Implemented`.

Several tasks touch the same files (`check.go`, `render.go`); the phase order avoids conflicts.
Execute **inline and sequentially** (`awf-executing-plans`): the tasks are coupled through
shared files, not independent.

## Tech stack

- Go 1.26; `gopkg.in/yaml.v3`; stdlib `text/template`.
- Packages touched: `cmd/awf`, `internal/project`, `internal/audit`, `internal/config`,
  `internal/invariants`, `internal/coverage`, `internal/render`, `internal/migrate`.
- Gate: `./x gate` (100% coverage, golangci-lint) before every commit; `./x check` for drift.

## Non-goals (deliberate scope exclusions)

- **`fileExists` cross-package dedup** (`cmd/awf/main.go:199` ↔ `internal/migrate/dropreplacewith.go:71`):
  the only clean shared home is a new fs-util package, and `internal/migrate` is a deliberately
  minimal, ADR-0010-quarantined reader (imports no other internal package in `dropreplacewith.go`).
  Coupling it for a 4-line `os.Stat` wrapper is a net negative. Left as-is.
- **`RenderAll` per-kind render loops** and the init/uninstall business-logic-in-`cmd` findings:
  out of scope here (the render loops are named as a future ADR-0027 consumer; init/uninstall
  relocation is a separate effort).

## File structure

**Created:**
- `internal/project/layout.go`, `internal/project/banner.go`, `internal/project/confighash.go` (moves out of `render.go`)
- `internal/project/kind.go`, `internal/project/kind_test.go` (descriptor table + backing test)
- `internal/audit/settings.go`, `internal/audit/settings_test.go` (re-homed from config)

**Modified:** `internal/coverage/coverage.go`, `internal/render/render.go`,
`internal/invariants/invariants.go`, `cmd/awf/check.go`, `cmd/awf/invariants.go`,
`cmd/awf/main.go`, `cmd/awf/main_test.go`, `cmd/awf/list_add.go`,
`internal/project/render.go`, `internal/project/check.go`, `internal/project/project.go`,
`internal/config/config.go`, `internal/config/config_test.go`,
`internal/migrate/treelayout.go`, `internal/migrate/dropreplacewith.go`,
`docs/decisions/0027-unified-kind-descriptor.md`, `docs/architecture.md` (via `.awf/`),
domain narratives.

---

## Phase 1: Mechanical cleanups

### Task 1.1: `coverage.marker` → `const`
- [ ] In `internal/coverage/coverage.go:22`, change `var marker = "//" + " coverage-ignore"` to
  `const marker = "//" + " coverage-ignore"`. (Constant expression; the concatenation trick still
  keeps the literal directive off the line.)
- [ ] Verify: `go build ./internal/coverage/ && go test ./internal/coverage/` → PASS.
- [ ] `./x gate` → green. Commit: `refactor(awf): make coverage marker a const`.

### Task 1.2: Delete the dead `render.Render` wrapper
`render.Render` (`internal/render/render.go:70-74`) is referenced only by tests; production
(`renderTarget`) calls `Assemble`/`Execute` directly. **Decided approach: delete** `Render` and
update its test callers (verified: the only callers are `internal/project/docs_sections_test.go:43,132`,
`frontmatter_test.go:54`, `spine_test.go:19`: all `*_test.go`). This is the clean outcome: no
double parse, smaller API. `renderTarget` is left untouched (it already calls `Assemble`/`Execute`
directly and needs `assembled` locally for the hash).
- [ ] First confirm no production caller appeared: `rg 'render\.Render\(' --type go` shows only
  `*_test.go`. (If a production caller exists, instead route it through `render.Render` and skip the
  delete, but at plan time there is none.)
- [ ] Delete `Render` (`internal/render/render.go:70-74`) and rewrite each test call
  `render.Render(src, nil, data)` → `render.Execute(render.Assemble(render.ParseSections(src), nil), data)`
  (the test calls pass `nil` for the plan).
- [ ] Verify: `go test ./internal/render/ ./internal/project/` → PASS.
- [ ] `./x gate` → green. Commit: `refactor(awf): drop unused render.Render wrapper`.

### Task 1.3: Shared invariant-finding formatter
`cmd/awf/check.go:30` and `cmd/awf/invariants.go:24` format the same `invariants.Finding` two
ways with a duplicated format string.
- [ ] In `internal/invariants/invariants.go`, add a method on `Finding`:
  ```go
  // Line renders the finding as a single human-readable line (no leading indent/column).
  func (f Finding) Line() string {
  	return fmt.Sprintf("%s: invariant %q %s", f.ADR, f.Slug, f.Detail())
  }
  ```
  (Add `"fmt"` to imports if not already present.)
- [ ] In `cmd/awf/invariants.go:24`, replace the `Fprintf` with:
  `fmt.Fprintf(stdout, "  %s\n", f.Line())`.
- [ ] In `cmd/awf/check.go:30`, replace the `Fprintf` with:
  `fmt.Fprintf(stdout, "  %-14s %s\n", "invariant", f.Line())`.
- [ ] Verify: `go test ./internal/invariants/ ./cmd/awf/` → PASS (existing tests assert the same
  output strings; they must still match: the rendered text is byte-identical).
- [ ] `./x gate` → green. Commit: `refactor(awf): share invariant-finding line formatter`.

### Task 1.4: `orphans()` distinguishes absent from unreadable
`internal/project/check.go:73` and `:91` swallow all `os.ReadDir` errors as "absent", masking IO
faults: inconsistent with the project's `errors.Is(err, os.ErrNotExist)` discipline elsewhere.
- [ ] Change `internal/project/check.go` so `orphans()` returns an error. Update its signature to
  `func (p *Project) orphans() ([]manifest.Drift, error)` and:
  - line 73-76: `if err != nil { continue }` → `if errors.Is(err, os.ErrNotExist) { continue } else if err != nil { return nil, err }`
  - line 91-94: same treatment for the `partsDir` read.
  - The existing `coverage-ignore`'d read at line 108-111 (enabled target's parts dir) stays as-is
    (it is already discriminated by being unreachable except on permission fault).
  - Final `return drift` → `return drift, nil` (line 126).
- [ ] Add `"errors"` to `check.go` imports if absent (it is; confirm; the file imports
  `fmt, maps, os, path/filepath, slices, sort, strings`).
- [ ] Update the sole caller. Find it: `rg 'p\.orphans\(\)|\.orphans\(\)' internal/project/`.
  In `Check()` (the call appends orphan drift), thread the error: `od, err := p.orphans(); if err != nil { return nil, err }` and append `od`.
- [ ] Add a regression test in `internal/project/check_test.go` (or the nearest existing
  orphans test file): create a project, make `.awf/skills` a regular file (not a dir) so
  `os.ReadDir` returns a non-`NotExist` error, assert `orphans()`/`Check()` returns that error.
  This is required for the 100% gate (new error arm). If the arm is genuinely unreachable as
  non-root, annotate with `// coverage-ignore: <reason>` instead, but a file-in-place-of-dir
  ReadDir fault IS reachable and testable, so prefer the test.
- [ ] Verify: `go test ./internal/project/ -run Orphan` → PASS.
- [ ] `./x gate` → green. Commit: `fix(awf): surface non-absent ReadDir faults in orphan scan`.

---

## Phase 2: Internal cohesion refactors

### Task 2.1: Route docsDir consumers through the typed `Layout`
Six inline `strings.TrimRight(p.Cfg.DocsDir, "/")` derivations bypass the `Layout` that exists to
own them. The two *definitions* (`layout()` at render.go:60, `docOutPath()` at render.go:112) stay;
the four *consumers* and the three absolute `filepath.Join(root, DocsDir, "decisions")` sites route
through `Layout`/a helper.
- [ ] Add a helper near `layout()` in `internal/project/render.go`:
  ```go
  // decisionsDir is the absolute ADR decisions directory.
  func (p *Project) decisionsDir() string {
  	return filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")
  }
  ```
- [ ] `render.go:427` `generateActiveMD`: `dir := filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")` → `dir := p.decisionsDir()`.
- [ ] `render.go:432` `generateActiveMD` return: `Path: strings.TrimRight(p.Cfg.DocsDir, "/") + "/decisions/ACTIVE.md"` → `Path: p.layout().ActiveMd`.
- [ ] `render.go:441` `generateDomainDocs`: `decisionsDir := filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")` → `decisionsDir := p.decisionsDir()`.
- [ ] `render.go:452` domain-doc out path: `strings.TrimRight(p.Cfg.DocsDir, "/")+"/domains/"+name+".md"` → `p.layout().DomainsDir+"/"+name+".md"`.
- [ ] `internal/project/project.go:128` `CheckInvariants`: `filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")` → `p.decisionsDir()`.
- [ ] `internal/project/check.go:164-165`: replace
  ```go
  	activeMdRel := strings.TrimRight(p.Cfg.DocsDir, "/") + "/decisions/ACTIVE.md"
  	domainsPrefix := strings.TrimRight(p.Cfg.DocsDir, "/") + "/domains/"
  ```
  with
  ```go
  	lay := p.layout()
  	activeMdRel := lay.ActiveMd
  	domainsPrefix := lay.DomainsDir + "/"
  ```
  Drop the now-unused `strings` import from `check.go` only if no other use remains
  (`rg 'strings\.' internal/project/check.go`: `orphans()` uses `strings.HasSuffix`/`TrimSuffix`,
  so it stays).
- [ ] Verify byte-stable ConfigHash: `./x check` → `awf check: clean` (these are path/display
  sites, never hash inputs, confirmed in brainstorm grounding).
- [ ] Verify: `go test ./internal/project/` → PASS.
- [ ] `./x gate` → green. Commit: `refactor(awf): route docsDir paths through typed Layout`.

### Task 2.2: `writeSidecarDoc` helper in `internal/migrate`
Three sites build `map[string]any` from data/sections/local then marshal-and-write, differing only
on the empty case (`treelayout.go` portSidecar/portAgentsDoc return nil; `dropreplacewith.go`
convertSidecar calls `os.Remove`).
- [ ] In `internal/migrate/treelayout.go` (or a shared `migrate` file), add:
  ```go
  // writeSidecarDoc marshals a sidecar doc {data, sections, local} to path. When the
  // doc is empty it removes path if removeIfEmpty (schema-conversion in place), else
  // is a no-op (fresh port to a new tree).
  func writeSidecarDoc(path string, data, sections map[string]any, local, removeIfEmpty bool) error {
  	doc := map[string]any{}
  	if len(data) > 0 {
  		doc["data"] = data
  	}
  	if len(sections) > 0 {
  		doc["sections"] = sections
  	}
  	if local {
  		doc["local"] = true
  	}
  	if len(doc) == 0 {
  		if removeIfEmpty {
  			return os.Remove(path)
  		}
  		return nil
  	}
  	b, err := yaml.Marshal(doc)
  	if err != nil { // coverage-ignore: doc holds only YAML-sourced marshalable values
  		return err
  	}
  	return writeFile(path, b)
  }
  ```
  (Match the exact marshal/write idiom the three sites currently use; `writeFile` already exists
  at `treelayout.go:200`. Confirm the kept-sections collection shape, `map[string]map[string]any{"drop": true}`, matches all three before substituting.)
- [ ] Replace the three inline blocks (`treelayout.go:102-115`, `treelayout.go:160-173`,
  `dropreplacewith.go:105-122`) with calls to `writeSidecarDoc`, passing `removeIfEmpty=false`
  for the two `port*` sites and `true` for `convertSidecar`.
- [ ] Verify: `go test ./internal/migrate/` → PASS (existing migration golden/round-trip tests
  cover both empty and non-empty cases; confirm coverage holds).
- [ ] `./x gate` → green. Commit: `refactor(awf): extract writeSidecarDoc in migrate`.

### Task 2.3: Split `Project.Check` into per-phase helpers
`Check()` (check.go:151-264) does five jobs sharing only a `drift` accumulator.
- [ ] Extract four unexported methods on `*Project`, each taking the inputs it needs and returning
  `([]manifest.Drift, error)` (or appending to a passed slice): `checkLockedFiles(lock, rendered, lay)`,
  `checkActiveMD(rendered, lay)`, `checkDomainDocs(rendered, lay)`, `checkDeadRefs(files)`. Move the
  corresponding blocks verbatim; `Check()` becomes a short orchestrator that calls each and
  concatenates drift. Preserve exact ordering and the `activeMdRel`/`domainsPrefix` skip logic.
- [ ] This is a pure move: no behaviour change. Verify ConfigHash/drift unaffected: `./x check` →
  `awf check: clean`.
- [ ] Verify: `go test ./internal/project/` → PASS; coverage still 100% (no new branches).
- [ ] `./x gate` → green. Commit: `refactor(awf): split Project.Check into per-phase helpers`.

### Task 2.4: Split `render.go` into cohesive files
`render.go` is 459 lines spanning six concerns. Pure moves within package `project` (no signature
changes).
- [ ] Create `internal/project/layout.go`: move `Layout` type (42-57), `layout()` (59-86),
  `templateMap()` (88-108), `docOutPath()` (110-113), `resolvedDocs()` (115-139), and the new
  `decisionsDir()` helper from Task 2.1. Add `package project` and the imports they need.
- [ ] Create `internal/project/banner.go`: move `bannerText` const (180) and `injectBanner()`
  (182-199). Keep the `// invariant: provenance-banner` marker on `injectBanner`.
- [ ] Create `internal/project/confighash.go`: move `consumedParts()` (382-392) and
  `targetConfigHash()` (394-421).
- [ ] `render.go` retains: `RenderAll`, `renderTarget`, `PlannedOutputs`, `planSections`,
  `partRel`, `nonNil`, `generateActiveMD`, `generateDomainDocs`, and any `sliceSet`/helpers local
  to it. Run `goimports`/`gofmt` on all four files; remove now-unused imports from `render.go`.
- [ ] Verify: `go build ./... && go test ./internal/project/` → PASS; `./x check` → clean (moves
  do not change rendered bytes or hashes).
- [ ] `./x gate` → green. Commit: `refactor(awf): split render.go into layout/banner/confighash`.

---

## Phase 3: CLI strictness (behaviour change)

### Task 3.1: Reject unknown flags / extra positionals; usage errors exit 2
Today `hasFlag`/`baseFlag` silently ignore unrecognized `--flags` and extra positionals, and usage
errors return a mix of exit 1 and 2. Make misuse explicit and exit-2-consistent.
- [ ] In `cmd/awf/main.go`, add a usage-error sentinel and a validator:
  ```go
  // usageErr marks a CLI-misuse error, mapped to exit code 2 by run().
  type usageErr struct{ msg string }

  func (e *usageErr) Error() string { return e.msg }

  // checkArgs rejects unrecognized --flags and enforces the positional count for a
  // subcommand. rest is args[2:]; allowed lists the value-less and value-taking flags;
  // minPos/maxPos bound the non-flag operands (maxPos < 0 = unbounded).
  func checkArgs(cmd string, rest, allowed []string, minPos, maxPos int) error {
  	pos := 0
  	for i := 0; i < len(rest); i++ {
  		a := rest[i]
  		if strings.HasPrefix(a, "-") {
  			name := a
  			if !slices.Contains(allowed, name) {
  				return &usageErr{fmt.Sprintf("awf %s: unknown flag %q", cmd, a)}
  			}
  			continue
  		}
  		pos++
  	}
  	if pos < minPos || (maxPos >= 0 && pos > maxPos) {
  		return &usageErr{fmt.Sprintf("awf %s: unexpected arguments", cmd)}
  	}
  	return nil
  }
  ```
  NOTE on `--base <value>`: `audit` takes a value-flag. At execution, either (a) special-case
  `--base`'s following token as consumed (skip `i++`), or (b) accept that the value token counts as
  a positional and set `audit`'s `maxPos` accordingly. Prefer (a): make `checkArgs` take a
  `valueFlags []string` set and advance `i` past a recognized value-flag's argument, erroring if it
  is the trailing token (this also fixes the `awf audit --base` silent-default edge).
- [ ] Wire each subcommand to call `checkArgs` before dispatch, with its flag/positional spec:
  - `init`: allowed `[--force --force-hooks]`, positionals 0..0
  - `sync`, `check`, `invariants`, `upgrade`, `uninstall`, `version`: no flags, 0..0
  - `audit`: valueFlag `[--base]`, positionals 0..0
  - `list`: no flags, positionals 0..1
  - `add`, `remove`: no flags, positionals 2..2
  - `setup`: allowed `[--force-hooks]`, positionals 0..0
  Apply each as `if err := checkArgs(args[1], args[2:], ...); err != nil { cmdErr = err; break }` at
  the top of the relevant `case`, or validate once before the switch via a per-command spec table.
- [ ] Convert the inline `return 1` usage errors to `usageErr` and fall through to the central
  handler: `add` arity (main.go:85,88), `remove` arity (main.go:93), unknown command
  (main.go:106-107). Keep their exact messages (preserve the helpful `awf add requires a kind...`
  text) but wrap in `&usageErr{...}` and `cmdErr = ...; break` instead of `Fprintln + return 1`.
- [ ] Update the central handler (main.go:109-112):
  ```go
  	if cmdErr != nil {
  		fmt.Fprintln(stderr, "awf:", cmdErr)
  		var ue *usageErr
  		if errors.As(cmdErr, &ue) {
  			return 2
  		}
  		return 1
  	}
  ```
- [ ] `hasFlag`/`baseFlag` stay (still used to read recognized flags); add `"slices"` import.
- [ ] Add tests in `cmd/awf/main_test.go`: unknown flag (`awf check --bogus`) → exit 2 with
  `unknown flag`; extra positional (`awf sync extra`) → exit 2; `awf audit --base` trailing → exit 2;
  `awf add foo` (missing name) still exits 2 (was 1); valid invocations unchanged. Cover every new
  branch (100% gate).
- [ ] Verify: `go test ./cmd/awf/` → PASS.
- [ ] `./x gate` → green. Commit: `feat(awf): reject unknown flags and standardize usage exit code`.

---

## Phase 4: Re-home audit resolution (config → audit)

### Task 4.1: Move `AuditSettings`/`ResolveAudit`/defaults into `internal/audit`
Mirrors the `internal/invariants` boundary (config holds only the raw schema). `internal/audit`
already imports `internal/config` (audit.go:15), so no cycle (config imports no audit).
- [ ] Create `internal/audit/settings.go`:
  - Define `type Settings struct { ... }` with the exact fields currently on
    `config.AuditSettings` (config.go:82-92): `BaseBranch, AllowedTypes, AllowedScopes,
    DependencyManifests []string; SubjectMaxLength, DiffThreshold int; DomainDocStaleness,
    UndocumentedDomain, UncommittedChanges bool`.
  - Move `defaultAllowedTypes()` (config.go:138-140) and `defaultDependencyManifests()`
    (config.go:142-149) verbatim.
  - Add `func Resolve(a *config.AuditConfig) Settings { ... }`: the body of
    `config.ResolveAudit` (config.go:95-136) with `c.Audit` replaced by the `a` parameter and
    `AuditSettings{...}` → `Settings{...}`.
- [ ] In `internal/audit/audit.go`: change `Inputs` (audit.go:76-77) to embed `Settings` instead of
  `config.AuditSettings`; update the doc comment (audit.go:71-75). Remove the now-unused
  `internal/config` import **only if** nothing else in `audit.go` references `config`
  (`rg 'config\.' internal/audit/audit.go`: `Resolve` lives in settings.go and still needs the
  import there, so `audit.go`'s import may drop; `settings.go` keeps it).
- [ ] In `internal/config/config.go`: delete `AuditSettings` (82-92), `ResolveAudit` (94-136),
  `defaultAllowedTypes` (138-140), `defaultDependencyManifests` (142-149). Keep the raw
  `AuditConfig` schema struct (unchanged). Remove imports that become unused in config.go.
- [ ] In `internal/project/project.go:134`: `s := p.Cfg.ResolveAudit()` → `s := audit.Resolve(p.Cfg.Audit)`.
  At line 146, `AuditSettings: s` → `Settings: s` (the embedded field is now named `Settings`).
- [ ] Move the three resolution tests `config_test.go:426-493`
  (`TestAuditSettingsDefaultsWhenNil`/`Zero`/`Explicit`) into a new
  `internal/audit/settings_test.go`, rewriting `cfg.ResolveAudit()` → `audit.Resolve(cfg.Audit)`
  and building a `*config.AuditConfig` directly (or a minimal `*config.Config` and passing
  `.Audit`). Delete them from `config_test.go`. Confirm `config` package coverage stays 100% after
  removal (the deleted funcs took their statements with them).
- [ ] Verify: `go build ./... && go test ./internal/audit/ ./internal/config/ ./internal/project/` → PASS.
- [ ] `./x gate` → green. Commit: `refactor(awf): re-home audit resolution into internal/audit`.

---

## Phase 5: Unified kind descriptor (ADR-0027)

### Task 5.1: Introduce the descriptor table and route all dispatch through it
Implements ADR-0027. Replaces ~6 parallel per-kind switches with one function-accessor table.
- [ ] Create `internal/project/kind.go`:
  ```go
  package project

  import (
  	"maps"
  	"slices"

  	"github.com/hypnotox/agentic-workflows/internal/catalog"
  	"github.com/hypnotox/agentic-workflows/internal/config"
  )

  // kindDescriptor resolves the per-kind facets the dispatch sites share. Facets are
  // accessor funcs so the table absorbs the catalog map-vs-slice and adapter-vs-neutral
  // path asymmetries (ADR-0027). A nil/"" facet means "absent" for that kind.
  type kindDescriptor struct {
  	Plural    string
  	Singular  string
  	enable    func(*config.Config) []string
  	poolNames func(*catalog.Catalog) []string                  // nil for domains (no pool)
  	sections  func(*catalog.Catalog, string) ([]string, bool)  // declared sections + catalog presence
  	outPath   func(t Target, prefix, name string) string       // "" for neutral kinds
  }

  // kindDescriptors is the single ordered source of per-kind dispatch (inv:
  // kind-dispatch-single-table). Order is the `awf list` display order.
  // invariant: cli-config-kinds
  var kindDescriptors = []kindDescriptor{
  	{
  		Plural: "skills", Singular: "skill",
  		enable:    func(c *config.Config) []string { return c.Skills },
  		poolNames: func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Skills)) },
  		sections:  func(c *catalog.Catalog, n string) ([]string, bool) { s, ok := c.Skills[n]; return s.Sections, ok },
  		outPath:   func(t Target, prefix, n string) string { return t.SkillPath(prefix, n) },
  	},
  	{
  		Plural: "agents", Singular: "agent",
  		enable:    func(c *config.Config) []string { return c.Agents },
  		poolNames: func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Agents)) },
  		sections:  func(c *catalog.Catalog, n string) ([]string, bool) { a, ok := c.Agents[n]; return a.Sections, ok },
  		outPath:   func(t Target, _ , n string) string { return t.AgentPath(n) },
  	},
  	{
  		Plural: "docs", Singular: "doc",
  		enable:    func(c *config.Config) []string { return c.Docs },
  		poolNames: func(c *catalog.Catalog) []string { return slices.Sorted(maps.Keys(c.Docs)) },
  		sections:  func(c *catalog.Catalog, n string) ([]string, bool) { d, ok := c.Docs[n]; return d.Sections, ok },
  		outPath:   nil,
  	},
  	{
  		Plural: "hooks", Singular: "hook",
  		enable:    func(c *config.Config) []string { return c.Hooks },
  		poolNames: func(c *catalog.Catalog) []string { return slices.Sorted(slices.Values(c.Hooks)) },
  		sections:  func(c *catalog.Catalog, _ string) ([]string, bool) { return nil, false },
  		outPath:   nil,
  	},
  	{
  		Plural: "domains", Singular: "domain",
  		enable:    func(c *config.Config) []string { return c.Domains },
  		poolNames: nil, // freeform: no catalog pool
  		sections:  func(c *catalog.Catalog, _ string) ([]string, bool) { return c.DomainDoc.Sections, false },
  		outPath:   nil,
  	},
  }

  func descriptorByPlural(kind string) (kindDescriptor, bool) {
  	for _, d := range kindDescriptors {
  		if d.Plural == kind {
  			return d, true
  		}
  	}
  	return kindDescriptor{}, false
  }

  func descriptorBySingular(kind string) (kindDescriptor, bool) {
  	for _, d := range kindDescriptors {
  		if d.Singular == kind {
  			return d, true
  		}
  	}
  	return kindDescriptor{}, false
  }
  ```
  Confirm `Target.SkillPath(prefix, name)` / `AgentPath(name)` signatures against
  `internal/project/target.go` before finalizing the `outPath` closures.
- [ ] Add the exported surface `cmd/awf` needs (in `kind.go`). `cmd/awf` currently uses
  `kindKey` (singular→plural), `kindsOrdered`, `enabledNames(cfg, kind)`, `catalogNames(cat, kind)`.
  Expose:
  ```go
  // Kinds returns the singular CLI kind tokens in display order.
  func Kinds() []string {
  	out := make([]string, len(kindDescriptors))
  	for i, d := range kindDescriptors {
  		out[i] = d.Singular
  	}
  	return out
  }

  // EnabledNames returns the config enable array for a singular CLI kind.
  func EnabledNames(c *config.Config, singular string) ([]string, bool) {
  	d, ok := descriptorBySingular(singular)
  	if !ok {
  		return nil, false
  	}
  	return d.enable(c), true
  }

  // CatalogNames returns the catalog pool for a singular CLI kind; ok=false for a
  // kind with no catalog pool (domains).
  func CatalogNames(cat *catalog.Catalog, singular string) ([]string, bool) {
  	d, ok := descriptorBySingular(singular)
  	if !ok || d.poolNames == nil {
  		return nil, false
  	}
  	return d.poolNames(cat), true
  }

  // PluralKind maps a singular CLI kind token to its config enable-array key.
  func PluralKind(singular string) (string, bool) {
  	d, ok := descriptorBySingular(singular)
  	return d.Plural, ok
  }
  ```
- [ ] Rewrite `cmd/awf/list_add.go`:
  - Delete `kindKey` (18-24), `kindsOrdered` (27), `enabledNames` (34-47), `catalogNames` (51-64).
    Replace usages: `kindKey[kind]` → `project.PluralKind(kind)`; `kindsOrdered` →
    `project.Kinds()`; `enabledNames(cfg, kind)` → `project.EnabledNames(cfg, kind)`;
    `catalogNames(cat, kind)` → `project.CatalogNames(cat, kind)`.
    NOTE: `PluralKind`/`EnabledNames`/`CatalogNames` all return `(value, bool)`, so the
    single-value call sites do not substitute verbatim; bind/discard the bool at each:
    `slices.Contains(enabledNames(p.Cfg, kind), name)` (lines 82, 107, 184) →
    `names, _ := project.EnabledNames(p.Cfg, kind); slices.Contains(names, name)`;
    `kindKey[kind]` in `Fprintf` (165) and `p.Cfg.Sidecar(kindKey[kind], name)` (189) →
    `plural, _ := project.PluralKind(kind); ...`. The two-value sites (`key, ok := kindKey[kind]`
    at 67/99, `_, ok := kindKey[kindFilter]` at 159, `pool, ok := catalogNames(...)` at 75/166)
    map directly.
  - Keep `unknownKind` and the `// invariant: cli-config-kinds` marker semantics, but the marker
    **moves** to `kind.go`'s `kindDescriptors` (done above). Remove the marker comment from
    `list_add.go:17`. (Per ADR-0027 Decision 5 / the re-homed-marker note: ADR-0024 stays the sole
    declaring ADR; only the source comment relocates. Do **not** add a new `inv:` bullet anywhere.)
  - Adjust imports (`maps`/`slices` may become unused in `list_add.go`).
- [ ] Rewrite `internal/project/check.go` dispatch:
  - `localOutPath` (18-27) → resolve via descriptor: `if d, ok := descriptorByPlural(kind); ok && d.outPath != nil { return d.outPath(p.Target, p.Cfg.Prefix, name) }; return ""`.
  - `declaredSections` (130-142) → `if d, ok := descriptorByPlural(kind); ok && d.sections != nil { s, _ := d.sections(p.Cat, name); return s }; return nil`.
  - `strings.TrimSuffix(kind, "s")` at check.go:48 → `d, _ := descriptorByPlural(kv.kind); d.Singular`
    (`descriptorByPlural` returns `(kindDescriptor, bool)`, so bind it: no method chain on the
    two-value result; `kv.kind` here is `"skills"`/`"agents"`, always found).
- [ ] Rewrite `internal/project/validate.go` `validateAgainstCatalog` (34-61): replace the three
  `checkKind("skills"/"agents"/"docs", ...)` closures and the separate hooks block with a loop over
  the catalog-backed descriptors:
  ```go
  for _, d := range kindDescriptors {
  	if d.poolNames == nil { // domains: freeform, not catalog-validated
  		continue
  	}
  	if err := p.checkKindAgainstCatalog(d); err != nil {
  		return err
  	}
  }
  ```
  where `checkKindAgainstCatalog` uses `d.enable(p.Cfg)` for the enabled names and `d.Singular` for
  error text. **Catalog membership must be resolved via `d.poolNames`, not the `sections` presence
  bool**: `slices.Contains(d.poolNames(p.Cat), name)` is the "is in the catalog" check (equivalent to
  today's `_, ok := p.Cat.Skills[name]` for skills/agents/docs, and the only correct check for hooks).
  The `sections` facet is used **only** for section validation: hooks return `sections == (nil,false)`
  for every name, so using that bool for presence would reject every enabled hook; instead, when
  `d.sections(p.Cat, name)`'s declared slice is `nil`/empty, skip `checkSectionsAllowed` (preserving
  today's hooks behaviour: catalog-membership check only, no section validation). The singleton blocks
  (agents-doc, adr-readme, ...) below line 62 are NOT kinds; leave them untouched.
  `strings.TrimSuffix(kind,"s")` at validate.go:26 → `d.Singular`.
- [ ] Create `internal/project/kind_test.go` backing `inv: kind-dispatch-single-table`:
  ```go
  // invariant: kind-dispatch-single-table
  func TestKindDescriptorsCoverAllKinds(t *testing.T) {
  	got := make([]string, len(kindDescriptors))
  	for i, d := range kindDescriptors {
  		got[i] = d.Plural
  	}
  	want := []string{"skills", "agents", "docs", "hooks", "domains"}
  	if !slices.Equal(got, want) { t.Fatalf("kind set drift: got %v want %v", got, want) }
  	// every catalog-backed kind must resolve a pool; domains must not.
  	for _, d := range kindDescriptors {
  		hasPool := d.poolNames != nil
  		if (d.Plural == "domains") == hasPool {
  			t.Errorf("%s poolNames presence wrong", d.Plural)
  		}
  	}
  }
  ```
  Add a test asserting `descriptorBySingular`/`descriptorByPlural`/exported accessors return the
  right values and `(_, false)` for an unknown kind (cover every branch for the 100% gate).
- [ ] Flip ADR-0027 to Implemented: in `docs/decisions/0027-unified-kind-descriptor.md`
  frontmatter, `status: Accepted` → `status: Implemented`. Run `./x sync` to regenerate
  `ACTIVE.md` and the domain indexes; stage them.
- [ ] Update doc currency (ADR-0027 obligations): in `.awf/docs/parts/architecture/components.md`
  add the descriptor table to the `internal/project` entry; note single-table dispatch in the
  `rendering`/`tooling` domain narratives (`.awf/domains/parts/...` or the domain doc source). Run
  `./x sync`; stage regenerated `docs/architecture.md` + domain docs.
- [ ] Verify: `go test ./...` → PASS; `./x check` → `awf check: clean` (the
  `kind-dispatch-single-table` slug is now enforced because ADR-0027 is Implemented: the backing
  test must exist and the `cli-config-kinds` marker must resolve at its new location).
- [ ] `./x gate` → green. Commit:
  `refactor(awf): unify per-kind dispatch behind a descriptor table (ADR-0027)`.

---

## Final verification
- [ ] `./x gate full` → green (full tier).
- [ ] `./x check` → `awf check: clean`.
- [ ] `git log --oneline` shows ~12 focused commits, Conventional-Commits, `awf` scope.
- [ ] Invoke `awf-reviewing-impl` (terminal review of the implementation series).
