# Plan: docsDir Layout Key and Built-In ACTIVE.md Generation

Implements **[ADR-0005](../decisions/0005-docsdir-layout-and-builtin-active-md.md)** (Accepted).
The design rationale lives in the ADR — this plan is the execution record. Do not re-argue
design here; if a step seems wrong, re-read the ADR.

## Goal

Make `docs/decisions/ACTIVE.md` generation real `awf` tooling: `awf sync` generates it,
`awf check` guards its staleness. Introduce a first-class `docsDir` config key (default
`"docs"`) that roots a fixed, awf-given docs layout exposed to templates under a new
`.layout` render namespace, replacing six hand-set doc-path vars. Retire the write-then-fail
`TestGenerateActiveMD` gate and the `./x adr` target.

## Architecture summary

- **config:** `Config` gains `DocsDir string`; absent ⇒ defaults to `"docs"` at load.
- **render data:** `Project.data()` injects a computed `layout` map (derived from `docsDir`)
  alongside `prefix`/`vars`/`data`. Templates read doc paths from `.layout.*`, never `.vars.*`.
  Because `render.ReferencedVars` only scans `.vars.X`, `ScaffoldConfig` never seeds the layout
  paths — they are strictly awf-given.
- **managed docs:** output path unifies from hardcoded `"docs/"` to `<docsDir>/<name>.md`.
- **generation:** a `generateActiveMD` helper calls `adrtools.GenerateActiveMD` on
  `<docsDir>/decisions`; `Sync` appends the result as an ordinary lock entry (empty
  `TemplateID`/`TemplateHash`); `Check` validates it via a dedicated regenerate-and-compare path
  and skips it in the generic lock-iteration loop. Zero ADRs ⇒ no file (and prune removes any
  prior one).

## Tech stack

- Go 1.26; packages touched: `internal/config`, `internal/project`, `internal/adrtools`,
  `internal/render` (read-only), `internal/manifest` (read-only); templates under `templates/`;
  the `./x` runner; `.claude/awf.yaml` (dogfood).
- Gate: `./x gate` (≈15s) runs on every code-touching commit via the pre-commit hook, which also
  runs `./x check`.

## File structure

**Modified:**
- `internal/config/config.go` — add `DocsDir` field, default, validation
- `internal/config/config_test.go` — docsDir default/explicit/invalid tests
- `internal/project/project.go` — `layout()`, `docOutPath()`, `data()`, `resolvedDocs`,
  `RenderAll` docs loop, `generateActiveMD()`, `Sync`, `Check`
- `internal/project/project_test.go` — `ACTIVE.md` sync/check tests; drop now-unused vars
- `internal/project/spine_test.go` — move doc-path keys from `vars` to `layout` in golden data
- `internal/adrtools/adrtools.go` — return `""` when no ADRs; repoint the package comment and
  generated-header literal off `go test ./internal/adrtools/` onto the sync mechanism
- `internal/adrtools/adrtools_test.go` — delete `TestGenerateActiveMD`; add empty-dir unit test;
  update the header-prefix assertion in `TestGenerateActiveMDGroupsByStatus`
- `x` — remove the `adr)` case
- `docs/decisions/README.md` — rewrite the "ACTIVE.md" how-to section
- `templates/agents/adr-reviewer.md.tmpl`, `templates/agents/plan-reviewer.md.tmpl`
- `templates/agents-doc/AGENTS.md.tmpl`
- `templates/skills/{proposing-adr,adr-lifecycle,executing-plans,subagent-driven-development,reviewing-adr,reviewing-plan,reviewing-plan-resync,brainstorming,writing-plans}/SKILL.md.tmpl`
- `.claude/awf.yaml` — remove the six doc-path vars, repoint `activeMdRegenCmd`, update
  `docCurrencyItems` strings; re-sync regenerates `.claude/**` + `AGENTS.md` + lock
- `.claude/awf/parts/doc-architecture.md` — repoint the `internal/adrtools/` bullet off
  `go test ./internal/adrtools/` (re-sync regenerates `docs/architecture.md`)
- `docs/decisions/0005-*.md` — final status flip `Accepted → Implemented`

**Created / deleted:** none.

---

## Phase 1 — `docsDir` config field + default

### Task 1.1 — Add the `DocsDir` field, default, and validation

- [ ] In `internal/config/config.go`, add the field to `Config` (after `Prefix`):

```go
type Config struct {
	Prefix    string                 `yaml:"prefix"`
	DocsDir   string                 `yaml:"docsDir"`
	Vars      map[string]any         `yaml:"vars"`
	Skills    map[string]SkillConfig `yaml:"skills"`
	Agents    map[string]SkillConfig `yaml:"agents"`
	Hooks     []string               `yaml:"hooks"`
	AgentsDoc *SkillConfig           `yaml:"agentsDoc"`
	Docs      map[string]SkillConfig `yaml:"docs"`
	raw       []byte
}
```

- [ ] In `Load`, default the field after `c.raw = b`:

```go
	c.raw = b
	if c.DocsDir == "" {
		c.DocsDir = "docs"
	}
	return &c, nil
```

- [ ] In `Validate`, after the `Prefix` checks (before the `Skills` loop), reject absolute or
  parent-escaping paths:

```go
	if strings.HasPrefix(c.DocsDir, "/") || strings.Contains(c.DocsDir, "..") {
		return fmt.Errorf("docsDir %q must be a relative path without \"..\"", c.DocsDir)
	}
```

(`strings` is already imported.)

### Task 1.2 — Tests for the field

- [ ] Append to `internal/config/config_test.go`:

```go
func TestDocsDirDefaultsToDocs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "awf.yaml")
	if err := os.WriteFile(path, []byte("prefix: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DocsDir != "docs" {
		t.Errorf("DocsDir = %q, want \"docs\"", c.DocsDir)
	}
}

func TestDocsDirExplicitValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "awf.yaml")
	if err := os.WriteFile(path, []byte("prefix: example\ndocsDir: documentation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DocsDir != "documentation" {
		t.Errorf("DocsDir = %q, want \"documentation\"", c.DocsDir)
	}
}

func TestDocsDirRejectsEscapingPath(t *testing.T) {
	c := &Config{Prefix: "example", DocsDir: "../escape"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for escaping docsDir")
	}
}
```

(Confirm `os`, `path/filepath` are imported in `config_test.go`; add them if the file does not
already import them.)

- [ ] Verify: `go test ./internal/config/` → `ok`.

### Task 1.3 — Commit

- [ ] `./x gate` → expect `0 issues.` and all packages `ok`.
- [ ] `git add internal/config/config.go internal/config/config_test.go`
- [ ] `git commit -m "feat(awf): add docsDir config key defaulting to \"docs\""`

---

## Phase 2 — `.layout` namespace + managed-docs unification

### Task 2.1 — Add `layout()` and `docOutPath()` helpers

- [ ] In `internal/project/project.go`, add after `data()` (around line 128):

```go
// layout returns the fixed, awf-given docs layout derived from cfg.DocsDir.
// These paths are exposed to templates under the .layout namespace; they are
// not configurable through vars.
func (p *Project) layout() map[string]any {
	d := strings.TrimRight(p.Cfg.DocsDir, "/")
	dec := d + "/decisions"
	return map[string]any{
		"docsDir":     d,
		"adrDir":      dec,
		"activeMd":    dec + "/ACTIVE.md",
		"adrReadme":   dec + "/README.md",
		"adrTemplate": dec + "/template.md",
		"plansDir":    d + "/plans",
	}
}

// docOutPath is the output path for a managed doc, rooted at docsDir.
func (p *Project) docOutPath(name string) string {
	return strings.TrimRight(p.Cfg.DocsDir, "/") + "/" + name + ".md"
}
```

### Task 2.2 — Inject `layout` into render data

- [ ] In `internal/project/project.go`, extend `data()`:

```go
func (p *Project) data(sc config.SkillConfig) map[string]any {
	return map[string]any{
		"prefix": p.Cfg.Prefix,
		"vars":   nonNil(p.Cfg.Vars),
		"data":   nonNil(sc.Data),
		"layout": p.layout(),
	}
}
```

### Task 2.3 — Unify managed-docs output path under `docsDir`

- [ ] In `resolvedDocs` (line ~143), change the link path:

```go
		out = append(out, map[string]any{
			"name":  name,
			"title": spec.Title,
			"desc":  spec.Desc,
			"path":  p.docOutPath(name),
		})
```

- [ ] In the `RenderAll` docs loop (line ~233), change the output path:

```go
		rf, err := p.renderTemplate(tid, dc.Sections, p.data(dc), p.docOutPath(name))
```

### Task 2.4 — Unit test the layout map

- [ ] Append to `internal/project/project_test.go`:

```go
func TestLayoutDerivesFromDocsDir(t *testing.T) {
	p := &Project{Cfg: &config.Config{DocsDir: "documentation"}}
	l := p.layout()
	want := map[string]string{
		"docsDir":     "documentation",
		"adrDir":      "documentation/decisions",
		"activeMd":    "documentation/decisions/ACTIVE.md",
		"adrReadme":   "documentation/decisions/README.md",
		"adrTemplate": "documentation/decisions/template.md",
		"plansDir":    "documentation/plans",
	}
	for k, v := range want {
		if got := l[k]; got != v {
			t.Errorf("layout[%q] = %v, want %q", k, got, v)
		}
	}
	if got := p.docOutPath("architecture"); got != "documentation/architecture.md" {
		t.Errorf("docOutPath = %q, want documentation/architecture.md", got)
	}
}
```

(Confirm `internal/project/project_test.go` imports `agentic-workflows/internal/config`; it is
used elsewhere in the package's tests — add the import to this file if missing.)

- [ ] Verify: `go test ./internal/project/` → `ok` (templates still read `.vars.*`; `.layout`
  is injected but not yet consumed — no behaviour change, default `docsDir` keeps every path
  byte-identical).

### Task 2.5 — Commit

- [ ] `./x gate` → `0 issues.`
- [ ] `git add internal/project/project.go internal/project/project_test.go`
- [ ] `git commit -m "feat(awf): inject .layout namespace and root managed docs at docsDir"`

---

## Phase 3 — Built-in `ACTIVE.md` generation

### Task 3.1 — `adrtools` returns empty when no ADRs

- [ ] In `internal/adrtools/adrtools.go`, in `GenerateActiveMD`, after the `for _, path := range
  matches` loop that builds `entries` (around line 69, immediately before the `// Group by
  status.` comment), add:

```go
	if len(entries) == 0 {
		return "", nil // no ADRs — signal "produce nothing"
	}
```

### Task 3.2 — `generateActiveMD` helper, wired into `Sync` and `Check`

- [ ] In `internal/project/project.go`, add `"agentic-workflows/internal/adrtools"` to the import
  block.

- [ ] Add the helper (place near `Sync`):

```go
// generateActiveMD renders the ADR index for the project's decisions directory,
// or returns nil when that directory holds no ADRs (so no index file is produced).
// ACTIVE.md is generated from ADR frontmatter, not a template, so it carries no
// TemplateID/TemplateHash in the lock.
func (p *Project) generateActiveMD() (*RenderedFile, error) {
	d := strings.TrimRight(p.Cfg.DocsDir, "/")
	dir := filepath.Join(p.Root, d, "decisions")
	content, err := adrtools.GenerateActiveMD(dir)
	if err != nil {
		return nil, err
	}
	if content == "" {
		return nil, nil
	}
	return &RenderedFile{Path: d + "/decisions/ACTIVE.md", Content: content}, nil
}
```

- [ ] In `Sync`, immediately after `files, err := p.RenderAll()` (and its error check), append the
  generated index so the existing write/lock/prune loop handles it:

```go
	if amd, err := p.generateActiveMD(); err != nil {
		return err
	} else if amd != nil {
		files = append(files, *amd)
	}
```

- [ ] In `Check`, add the dedicated path. Replace the body from `var drift []manifest.Drift`
  through the closing `return drift, nil` with:

```go
	activeMdRel := strings.TrimRight(p.Cfg.DocsDir, "/") + "/decisions/ACTIVE.md"
	var drift []manifest.Drift
	for _, path := range sortedKeys(lock.Files) {
		if path == activeMdRel {
			continue // generated artifact — checked separately below
		}
		e := lock.Files[path]
		rf, ok := rendered[path]
		if !ok {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "in lock but no longer produced"})
			continue
		}
		if rf.TemplateHash != e.TemplateHash || cfgHash != e.ConfigHash {
			// stale takes precedence: a re-sync overwrites any hand-edit, so it
			// is the actionable signal — one drift entry per path.
			drift = append(drift, manifest.Drift{Path: path, Kind: "stale", Detail: "template or config changed; run awf sync"})
			continue
		}
		onDisk, err := os.ReadFile(filepath.Join(p.Root, path))
		if err != nil {
			drift = append(drift, manifest.Drift{Path: path, Kind: "missing", Detail: "file absent; run awf sync"})
			continue
		}
		if manifest.Hash(onDisk) != e.OutputHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "hand-edited", Detail: "on-disk output differs from lock"})
		}
	}
	// ACTIVE.md is generated from ADR frontmatter, not a template, so its staleness
	// cannot be detected by the template/config hash comparison above. Regenerate and
	// compare directly.
	amd, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	if amd != nil {
		onDisk, err := os.ReadFile(filepath.Join(p.Root, activeMdRel))
		if err != nil {
			drift = append(drift, manifest.Drift{Path: activeMdRel, Kind: "missing", Detail: "ADR index absent; run awf sync"})
		} else if manifest.Hash(onDisk) != manifest.Hash([]byte(amd.Content)) {
			drift = append(drift, manifest.Drift{Path: activeMdRel, Kind: "stale", Detail: "ADR index out of date; run awf sync"})
		}
	} else if _, locked := lock.Files[activeMdRel]; locked {
		drift = append(drift, manifest.Drift{Path: activeMdRel, Kind: "orphaned", Detail: "no ADRs remain; run awf sync"})
	}
	return drift, nil
```

### Task 3.3 — Tests for generation + staleness

- [ ] Add to `internal/adrtools/adrtools_test.go` a focused empty-dir unit test:

```go
func TestGenerateActiveMDEmptyWhenNoADRs(t *testing.T) {
	dir := t.TempDir()
	// A non-ADR markdown file must not count.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := adrtools.GenerateActiveMD(dir)
	if err != nil {
		t.Fatalf("GenerateActiveMD: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty output for an ADR-less dir, got:\n%s", got)
	}
}
```

- [ ] Add to `internal/project/project_test.go` an end-to-end sync+check test for the index.
  This test scaffolds a project, writes an ADR under `docs/decisions/`, and asserts sync writes
  the index and check is clean, then mutates frontmatter and asserts check flags it:

```go
func TestSyncGeneratesActiveMDAndCheckDetectsStaleness(t *testing.T) {
	yaml := `prefix: example
skills: {}
agents: {}
hooks: []
`
	root := scaffold(t, yaml)
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adr := "---\nstatus: Accepted\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: First\n## Context\nx\n"
	if err := os.WriteFile(filepath.Join(adrDir, "0001-first.md"), []byte(adr), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	active, err := os.ReadFile(filepath.Join(adrDir, "ACTIVE.md"))
	if err != nil {
		t.Fatalf("ACTIVE.md not generated: %v", err)
	}
	if !strings.Contains(string(active), "ADR-0001: First") {
		t.Errorf("ACTIVE.md missing the ADR entry:\n%s", active)
	}
	if drift, err := p.Check(); err != nil || len(drift) != 0 {
		t.Fatalf("expected clean check after sync, got drift=%#v err=%v", drift, err)
	}

	// Change frontmatter status; the on-disk ACTIVE.md is now stale.
	adr2 := strings.Replace(adr, "status: Accepted", "status: Implemented", 1)
	if err := os.WriteFile(filepath.Join(adrDir, "0001-first.md"), []byte(adr2), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, d := range drift {
		if strings.HasSuffix(d.Path, "decisions/ACTIVE.md") && d.Kind == "stale" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected stale drift for ACTIVE.md, got %#v", drift)
	}
}

func TestSyncProducesNoActiveMDWithoutADRs(t *testing.T) {
	yaml := `prefix: example
skills: {}
agents: {}
hooks: []
`
	root := scaffold(t, yaml)
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "decisions", "ACTIVE.md")); !os.IsNotExist(err) {
		t.Errorf("expected no ACTIVE.md when no ADRs exist, stat err=%v", err)
	}
	if drift, err := p.Check(); err != nil || len(drift) != 0 {
		t.Errorf("expected clean check with no ADRs, got drift=%#v err=%v", drift, err)
	}
}
```

(Confirm the `scaffold` test helper writes the given yaml to `.claude/awf.yaml` under a temp root;
it is used throughout `project_test.go`. `strings`, `os`, `path/filepath` are already imported there.)

- [ ] Verify: `go test ./internal/adrtools/ ./internal/project/` → `ok` (the legacy
  `TestGenerateActiveMD` write-then-fail gate still exists here and still passes because the
  repo's own `ACTIVE.md` is current; it is removed in Phase 4).

### Task 3.4 — Re-sync the dogfood lock and commit

- [ ] `./x sync` — this now also writes/locks `docs/decisions/ACTIVE.md`; `.claude/awf.lock`
  gains the `docs/decisions/ACTIVE.md` entry. `ACTIVE.md` content is unchanged.
- [ ] `./x check` → expect `awf check: clean`.
- [ ] `./x gate` → `0 issues.`
- [ ] `git add internal/adrtools/adrtools.go internal/adrtools/adrtools_test.go internal/project/project.go internal/project/project_test.go .claude/awf.lock`
- [ ] `git commit -m "feat(awf): generate ACTIVE.md in sync, check its staleness"`

---

## Phase 4 — Retire the test-as-tool mechanism

### Task 4.1 — Delete the write-then-fail gate test

- [ ] In `internal/adrtools/adrtools_test.go`, delete the entire `TestGenerateActiveMD` function
  (the staleness-gate test, currently lines ~113-140 — the one that writes `ACTIVE.md` and calls
  `t.Fatalf("ACTIVE.md was stale — regenerated; re-stage it")`). Keep
  `TestGenerateActiveMDGroupsByStatus` and `TestGenerateActiveMDEmptyWhenNoADRs`.

### Task 4.1b — Repoint the generator's self-referencing strings

The generator embeds the retired command in two places that the rest of Phase 4 leaves
dangling; both must move to the sync mechanism so the regenerated `ACTIVE.md` no longer
instructs readers to run a command that no longer exists.

- [ ] In `internal/adrtools/adrtools.go`, change the package doc comment (line ~2)
  `// Run via: go test ./internal/adrtools/` → `// Generated by awf sync (regenerates docs/decisions/ACTIVE.md).`
- [ ] In `internal/adrtools/adrtools.go`, change the generated header literal (line ~104)
  from `<!-- GENERATED by internal/adrtools — do not edit by hand. Run: go test ./internal/adrtools/ -->`
  to `<!-- GENERATED by awf sync — do not edit by hand. -->`.

(This changes the first line of every regenerated `ACTIVE.md`, so Task 4.4 must re-sync and
stage `docs/decisions/ACTIVE.md`; otherwise `./x check` flags it stale against the lock.
Update the `TestGenerateActiveMDGroupsByStatus` header-prefix assertion in
`internal/adrtools/adrtools_test.go` — it checks `strings.HasPrefix(got, "<!-- GENERATED by internal/adrtools")` —
to match the new prefix, e.g. `"<!-- GENERATED by awf sync"`.)

### Task 4.2 — Remove the `./x adr` target

- [ ] In `x`, delete the `adr)` case block:

```
  adr)
    # Regenerate docs/decisions/ACTIVE.md (the generator runs as a test).
    go test ./internal/adrtools/
    ;;
```

- [ ] In `x`, update the usage string at the bottom, removing `adr`:

```
    echo "usage: ./x <gate [full]|lint|fmt|test|sync|check|setup|build|install>" >&2
```

### Task 4.3 — Rewrite the README "ACTIVE.md" section

- [ ] In `docs/decisions/README.md`, replace the `## ACTIVE.md` section (the `go test
  ./internal/adrtools/` how-to and the two trailing sentences) with:

```markdown
## ACTIVE.md

`ACTIVE.md` is a generated index — **do not edit it by hand**. It is regenerated by `awf sync`
(here, `./x sync`) from the ADR frontmatter, and `awf check` (`./x check`, run by the pre-commit
hook) fails if it is stale. After adding an ADR or changing a `status:` field, run `./x sync` and
stage the regenerated `ACTIVE.md` alongside your change.
```

### Task 4.4 — Verify and commit

- [ ] `./x sync` — regenerates `docs/decisions/ACTIVE.md` with the new header line (and
  refreshes the lock entry's `OutputHash`).
- [ ] `go test ./internal/adrtools/` → `ok` (no write-then-fail).
- [ ] `./x gate` → `0 issues.`; `./x check` → `awf check: clean`.
- [ ] `git add internal/adrtools/adrtools.go internal/adrtools/adrtools_test.go x docs/decisions/README.md docs/decisions/ACTIVE.md .claude/awf.lock`
- [ ] `git commit -m "refactor(awf): drop ./x adr and the write-then-fail ACTIVE.md gate"`

---

## Phase 5 — Migrate templates to `.layout` and prune the vars

### Task 5.1 — Global `.vars.* → .layout.*` rename across skill/agent templates

Apply these exact substitutions to **every** `.tmpl` file under `templates/skills/` and
`templates/agents/` (i.e. all files **except** `templates/agents-doc/AGENTS.md.tmpl` and
`templates/skills/writing-plans/SKILL.md.tmpl`, handled in 5.2-5.3). Each is an unambiguous,
all-occurrences replacement:

- [ ] `.vars.adrDir` → `.layout.adrDir`
- [ ] `.vars.plansDir` → `.layout.plansDir`
- [ ] `.vars.adrReadme` → `.layout.adrReadme`
- [ ] `.vars.activeMdPath` → `.layout.activeMd`
- [ ] `.vars.adrTemplatePath` → `.layout.adrTemplate`

Affected files (from `grep -rn '\.vars\.\(adrDir\|plansDir\|adrReadme\|activeMdPath\|adrTemplatePath\)' templates/skills templates/agents`):
`agents/adr-reviewer.md.tmpl`, `agents/plan-reviewer.md.tmpl`,
`skills/proposing-adr/SKILL.md.tmpl`, `skills/adr-lifecycle/SKILL.md.tmpl`,
`skills/executing-plans/SKILL.md.tmpl`, `skills/subagent-driven-development/SKILL.md.tmpl`,
`skills/reviewing-adr/SKILL.md.tmpl`, `skills/reviewing-plan/SKILL.md.tmpl`,
`skills/reviewing-plan-resync/SKILL.md.tmpl`, `skills/brainstorming/SKILL.md.tmpl`.

`{{ .vars.activeMdRegenCmd }}` is **not** touched by the global rename — it stays a var (a
command, not a path). See the one targeted exception in Task 5.1b.

- [ ] Verify no stragglers: `grep -rn '\.vars\.\(adrDir\|plansDir\|adrReadme\|activeMdPath\|adrTemplatePath\)' templates/skills templates/agents` → no output.

### Task 5.1b — Repoint the invariant-confirmation command in `proposing-adr`

`templates/skills/proposing-adr/SKILL.md.tmpl:65` reuses `{{ .vars.activeMdRegenCmd }}` to mean
"run the tests to confirm each Invariant has a `// invariant:` test" — this only works today
because the regen command happens to be a test command. Once Task 5.6 repoints
`activeMdRegenCmd` to `./x sync` (which runs no tests), that instruction becomes wrong. Point
this one line at the gate command instead (the gate runs the tests). The other two
`activeMdRegenCmd` references in this file (lines ~69, ~88, the actual ACTIVE.md regeneration
steps) are **not** changed.

- [ ] In `templates/skills/proposing-adr/SKILL.md.tmpl`, on the "Pair each Invariant with a test"
  step, replace only that line's command:

old:
```
1. **Pair each Invariant with a test.** Each Invariants section bullet must be accompanied by at least one `// invariant: <normalised bullet title>` test shipping in the same commit. Run `{{ .vars.activeMdRegenCmd }}` to confirm.
```
new:
```
1. **Pair each Invariant with a test.** Each Invariants section bullet must be accompanied by at least one `// invariant: <normalised bullet title>` test shipping in the same commit. Run `{{ .vars.gateCmd }}` to confirm.
```

(The `// invariant:` convention itself remains dormant and is made real by ADR-0006; this edit
only prevents the `activeMdRegenCmd` repoint from corrupting the instruction's meaning.)

### Task 5.2 — `agents-doc/AGENTS.md.tmpl`: drop the guards (collapse to unconditional)

The layout keys are always populated, so the `{{ with }}…{{ else }}…{{ end }}` guards become dead.

- [ ] Replace line 23 (the Append-only ADRs bullet):

old:
```
- **Append-only ADRs.** Decision rationale lives under `{{ with .vars.adrDir }}{{ . }}{{ else }}docs/decisions{{ end }}/`{{ with .vars.activeMdPath }}; `{{ . }}` is generated — never hand-edited{{ end }}.
```
new:
```
- **Append-only ADRs.** Decision rationale lives under `{{ .layout.adrDir }}/`; `{{ .layout.activeMd }}` is generated — never hand-edited.
```

- [ ] Replace the three Document-map bullets (lines 65-67):

old:
```
{{ with .vars.adrReadme }}- **ADR index:** [{{ . }}]({{ . }}) — architecture decisions and lifecycle.
{{ end }}{{- with .vars.activeMdPath }}- **Active ADRs:** [{{ . }}]({{ . }}) — generated status index; do not hand-edit.
{{ end }}{{- with .vars.plansDir }}- **Plans:** [{{ . }}]({{ . }}) — implementation plans for complex work.
{{ end }}{{- range .docs }}- **{{ .title }}:** [{{ .path }}]({{ .path }}) — {{ .desc }}
```
new:
```
- **ADR index:** [{{ .layout.adrReadme }}]({{ .layout.adrReadme }}) — architecture decisions and lifecycle.
- **Active ADRs:** [{{ .layout.activeMd }}]({{ .layout.activeMd }}) — generated status index; do not hand-edit.
- **Plans:** [{{ .layout.plansDir }}]({{ .layout.plansDir }}) — implementation plans for complex work.
{{ range .docs }}- **{{ .title }}:** [{{ .path }}]({{ .path }}) — {{ .desc }}
```

(The `{{ with .vars.gateCmd }}` guards on lines 25 and 44 stay — `gateCmd` remains a var.)

### Task 5.3 — `writing-plans/SKILL.md.tmpl`: remove the planTemplatePath section

- [ ] Empty the `plan-template-ref` section body (drop the conditional line, keep the markers,
  matching the existing empty-`doc-currency-check` pattern). Replace:

old:
```
<!-- awf:section plan-template-ref -->
{{ if .vars.planTemplatePath }}1. **Use `{{ .vars.planTemplatePath }}` as the starting shape** for the plan file. The template carries the required header fields and task structure.
{{ end }}<!-- awf:end -->
```
new:
```
<!-- awf:section plan-template-ref -->
<!-- awf:end -->
```

- [ ] Migrate the remaining `.vars.plansDir` references in this file (description line 3 and
  body lines 9, 21): `.vars.plansDir` → `.layout.plansDir`.
- [ ] Verify: `grep -n 'planTemplatePath' templates/` → no output.

### Task 5.4 — Update golden tests to supply `layout`

In `internal/project/spine_test.go`, the golden tests call `render.Render` directly, so they
must now provide a `layout` map for any template that reads `.layout.*`. For each test below,
**remove** the listed keys from its `vars` map and **add** a sibling `"layout"` entry. Renames:
`activeMdPath → activeMd`, `adrTemplatePath → adrTemplate`.

- [ ] `TestAdrReviewerAgent` — remove `activeMdPath`, `adrDir` from `vars` (keep
  `invariantTestPath`, `activeMdRegenCmd`); add:
```go
		"layout": map[string]any{"adrDir": "docs/decisions", "activeMd": "docs/decisions/ACTIVE.md"},
```
- [ ] `TestPlanReviewerAgent` — remove `plansDir`; add
  `"layout": map[string]any{"plansDir": "docs/plans"},`.
- [ ] `TestExecutingPlansTemplate` — remove `plansDir`, `activeMdPath` (keep
  `activeMdRegenCmd`, `workflowDoc`, `gateCmd`, `gateDuration`, `hostGitAdrRef`,
  `oracleStateDoc`, `keyInvariantAdrRef`); add
  `"layout": map[string]any{"plansDir": "docs/plans", "activeMd": "docs/decisions/ACTIVE.md"},`.
- [ ] `TestSubagentDrivenDevelopmentTemplate` — remove `plansDir`, `activeMdPath`; add the same
  `layout` block as above.
- [ ] `TestProposingAdrTemplate` — remove `adrDir`, `adrTemplatePath`, `activeMdPath`,
  `adrReadme` (keep `activeMdRegenCmd`, `workflowDoc`, `stateDocsPath`, `adrProposeCommitFmt`);
  **add `"gateCmd": "./x gate"` to `vars`** (Task 5.1b introduces a new bare `{{ .vars.gateCmd }}`
  reference in this template — without it the golden render emits `<no value>`); add:
```go
		"layout": map[string]any{
			"adrDir": "docs/decisions", "adrTemplate": "docs/decisions/template.md",
			"activeMd": "docs/decisions/ACTIVE.md", "adrReadme": "docs/decisions/README.md",
		},
```
- [ ] `TestAdrLifecycleTemplate` — remove `adrDir`, `activeMdPath`, `adrReadme` (keep
  `activeMdRegenCmd`, `workflowDoc`, `stateDocsPath`, `gateCmd`); add:
```go
		"layout": map[string]any{
			"adrDir": "docs/decisions", "activeMd": "docs/decisions/ACTIVE.md",
			"adrReadme": "docs/decisions/README.md",
		},
```
- [ ] `TestBrainstormingTemplate` — remove `adrReadme`; add
  `"layout": map[string]any{"adrReadme": "docs/decisions/README.md"},`.
- [ ] `TestReviewingPlanTemplate` — remove `plansDir`; add
  `"layout": map[string]any{"plansDir": "docs/plans"},`.
- [ ] `TestReviewingPlanResyncTemplate` — remove `plansDir`; add the same `layout` block.
- [ ] `TestReviewingAdrTemplate` — remove `adrDir`, `plansDir`; add
  `"layout": map[string]any{"adrDir": "docs/decisions", "plansDir": "docs/plans"},`.
- [ ] `TestWritingPlansTemplate` — remove `planTemplatePath`, `plansDir`; add
  `"layout": map[string]any{"plansDir": "docs/plans"},`.
- [ ] `TestAgentsDocTemplate` — remove `adrDir`, `plansDir` (keep `testCmd`, `gateCmd`); add
  `"layout": map[string]any{"adrDir": "docs/decisions", "activeMd": "docs/decisions/ACTIVE.md", "adrReadme": "docs/decisions/README.md", "plansDir": "docs/plans"},`.
- [ ] `TestAgentsDocTemplateConfigDriven` — remove `adrReadme`, `activeMdPath` from `vars` (keep
  `gateCmd: ""` to exercise the empty-gate guard); add a fully-populated `layout` block and keep
  the `]()` assertion (it now guards the gate/data fallbacks, not the always-present layout links):
```go
		"layout": map[string]any{
			"adrReadme": "docs/decisions/README.md", "activeMd": "docs/decisions/ACTIVE.md",
			"plansDir": "docs/plans",
		},
```

- [ ] Verify: `go test ./internal/project/` → `ok`.

### Task 5.5 — Re-sync and commit the template migration

- [ ] `./x sync` — rendered `.claude/**` and `AGENTS.md` content is byte-identical (default
  `docsDir` ⇒ identical paths); only `.claude/awf.lock` may change.
- [ ] `./x check` → `awf check: clean`; `./x gate` → `0 issues.`
- [ ] `git add templates/ internal/project/spine_test.go .claude/`
- [ ] `git commit -m "refactor(awf): migrate doc-path templates from .vars to .layout"`

### Task 5.6 — Prune the now-unused doc-path vars and repoint the regen command

- [ ] In `.claude/awf.yaml`, under `vars:`, delete these six lines:
  `activeMdPath`, `adrDir`, `adrReadme`, `adrTemplatePath`, `plansDir`, `planTemplatePath`.
- [ ] In `.claude/awf.yaml`, repoint:
```yaml
  activeMdRegenCmd: "./x sync"
```
- [ ] In `.claude/awf.yaml`, update the two `docCurrencyItems` check strings that name the old
  command (under `agents.adr-reviewer.data.docCurrencyItems` and
  `agents.code-reviewer.data.docCurrencyItems`): change
  `regenerated via go test ./internal/adrtools/` → `regenerated by ./x sync`.
- [ ] In `.claude/awf/parts/doc-architecture.md`, repoint the `internal/adrtools/` bullet (line
  ~12) off the retired command: change `regenerates `docs/decisions/ACTIVE.md` from ADR
  frontmatter; run via `go test ./internal/adrtools/`` → `generates `docs/decisions/ACTIVE.md`
  from ADR frontmatter; invoked by `awf sync``. (This part feeds the managed doc
  `docs/architecture.md` via `docs.architecture.sections.body.replaceWith`, so the re-sync below
  also refreshes `docs/architecture.md`.)
- [ ] In `internal/project/project_test.go`, remove the now-dead `adrDir`/`plansDir` lines from
  the `TestSyncRendersAgentsDoc` yaml (lines ~467-468) and the `adrReadme: ""`/`activeMdPath: ""`
  lines from `TestSyncAutoLinksDocsInAgentsDoc` (lines ~241-242) — those vars no longer feed the
  template (the Document-map links now come from `.layout`, injected by `Open`→`data`). Leave
  `gateCmd: ""` in place where present.
- [ ] `./x sync` — regenerates `.claude/agents/adr-reviewer.md` and `code-reviewer.md` (the
  docCurrency string change), `docs/architecture.md` (the part change) and refreshes the lock
  (`configHash` changed).
- [ ] `./x check` → `awf check: clean`; `./x gate` → `0 issues.`
- [ ] `git add .claude/ docs/architecture.md internal/project/project_test.go`
- [ ] `git commit -m "chore(awf): drop derived doc-path vars; regen ACTIVE.md via ./x sync"`

---

## Phase 6 — Finalize: flip ADR-0005 to Implemented

### Task 6.1 — Flip status and regenerate the index via the new path

- [ ] In `docs/decisions/0005-docsdir-layout-and-builtin-active-md.md`, change frontmatter
  `status: Accepted` → `status: Implemented`.
- [ ] `./x sync` — regenerates `docs/decisions/ACTIVE.md` (now via the built-in generator),
  moving ADR-0005 into the Implemented section.
- [ ] `./x check` → `awf check: clean`; `./x gate` → `0 issues.`
- [ ] `git add docs/decisions/0005-docsdir-layout-and-builtin-active-md.md docs/decisions/ACTIVE.md`
- [ ] `git commit -m "docs(adr): mark 0005 Implemented"`

### Task 6.2 — Terminal handoff

- [ ] Invoke `awf-reviewing-impl` against the implementation commit range (Phases 1-6) per the
  workflow chain.

---

## Notes

- **Ordering is load-bearing.** Phase 2 (inject `.layout`) must precede Phase 5 (templates read
  `.layout`), and within Phase 5 the template edits + golden-test updates (5.1-5.5) must land
  before the var pruning (5.6) — a template reading `.layout.X` while the var is still present is
  fine, but pruning a var while a template still reads `.vars.X` would render `<no value>` and
  fail the gate. Because default `docsDir` makes every derived path equal to the old hand-set
  value, rendered output stays byte-identical throughout; only `.claude/awf.lock` churns.
- **No ADR README index row** is owed (this repo's README is a how-to guide; `ACTIVE.md` is the
  generated index — ADR-0003/0004 precedent).
- ADR-0006 (the `// invariant:` test-tagging convention + checker) is a separate follow-up and is
  out of scope here; it will retro-apply tagged tests to ADR-0005's invariants.
