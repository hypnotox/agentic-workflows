# Plan: Shared Frontmatter Parser and Skill/Agent Frontmatter Validation

Implements **[ADR-0006](../decisions/0006-frontmatter-parser-and-skill-validation.md)** (Accepted).
Design rationale lives in the ADR — this plan is the execution record. If a step seems wrong,
re-read the ADR.

## Goal

One tested `internal/frontmatter` package as the single home for `---`-delimited YAML
frontmatter; rename `internal/adrtools` → `internal/adr` refactored onto it (dedup); validate
that every rendered skill/agent has parseable frontmatter with non-empty `name`/`description`,
enforced at `awf sync` (fail before writing) and `awf check` (new `invalid-frontmatter` drift);
golden test proving every catalog skill/agent template renders valid frontmatter.

## Architecture summary

- `internal/frontmatter`: `Split(content) (yaml, body, found)` + `Parse(content, out) (body, found, err)`.
- `internal/adr` (renamed from `adrtools`): `ParseDir(dir) ([]ADR, error)`, `ADR{Number,Title,Status,Filename,Path}`, `RenderActiveMD(dir) (string, error)` — all built on `internal/frontmatter`. Byte-identical ACTIVE.md output.
- `internal/project`: `validateFrontmatter([]byte) error` + `isSkillOrAgent(tid)`; `Sync` fails fast on invalid skill/agent frontmatter; `Check` emits `invalid-frontmatter` drift (subordinate to hash kinds).
- Golden test renders every catalog skill/agent template and asserts valid frontmatter.

## Tech stack

- Go 1.26; packages: `internal/frontmatter` (new), `internal/adr` (renamed), `internal/project`, `internal/catalog` (read), `internal/render` (read). Dep: `gopkg.in/yaml.v3` (already direct).
- Gate: `./x gate` (≈15s) per code-touching commit via pre-commit, which also runs `./x check`.

## File structure

**Created:**
- `internal/frontmatter/frontmatter.go`, `internal/frontmatter/frontmatter_test.go`
- `internal/project/frontmatter_test.go` (all-templates golden + validator unit tests)

**Renamed:**
- `internal/adrtools/adrtools.go` → `internal/adr/adr.go`
- `internal/adrtools/adrtools_test.go` → `internal/adr/adr_test.go`

**Modified:**
- `internal/adr/adr.go` — package `adr`; refactor onto `internal/frontmatter`; `GenerateActiveMD` → `RenderActiveMD`; add `ADR`/`ParseDir`
- `internal/adr/adr_test.go` — package + import + call renames; add a `ParseDir` test
- `internal/project/project.go` — import `adr` + `frontmatter`; `generateActiveMD` calls `adr.RenderActiveMD`; `validateFrontmatter`/`isSkillOrAgent`; Sync fail-fast; Check drift
- `internal/project/project_test.go` — add `invalid-frontmatter` check test
- `.claude/awf/parts/doc-architecture.md` — repoint `internal/adrtools` → `internal/adr`, add `internal/frontmatter` (re-sync regenerates `docs/architecture.md`)
- `.claude/awf.yaml` — add the skill-frontmatter invariant to `agentsDoc.data.invariants` (re-sync regenerates `AGENTS.md`)
- `docs/decisions/0006-*.md` — final `Accepted → Implemented` flip

**Deleted:** none (renames only).

---

## Phase 1 — `internal/frontmatter` package

### Task 1.1 — Create the package

- [ ] Create `internal/frontmatter/frontmatter.go`:

```go
// Package frontmatter splits and parses YAML frontmatter delimited by leading
// "---" lines in markdown content. It is the single home for this concern.
package frontmatter

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Split separates a leading YAML frontmatter block (opened and closed by a line
// containing exactly "---") from the body. When content has no such block, found
// is false and body is the original content unchanged.
func Split(content []byte) (yamlBlock []byte, body []byte, found bool) {
	lines := bytes.SplitAfter(content, []byte("\n")) // keep line terminators
	if len(lines) == 0 || strings.TrimRight(string(lines[0]), "\r\n") != "---" {
		return nil, content, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(string(lines[i]), "\r\n") == "---" {
			return bytes.Join(lines[1:i], nil), bytes.Join(lines[i+1:], nil), true
		}
	}
	return nil, content, false // no closing delimiter
}

// Parse splits frontmatter from content and unmarshals the YAML block into out.
// When no frontmatter is present, found is false and out is left unchanged.
func Parse(content []byte, out any) (body []byte, found bool, err error) {
	yamlBlock, body, found := Split(content)
	if !found {
		return body, false, nil
	}
	if err := yaml.Unmarshal(yamlBlock, out); err != nil {
		return body, true, fmt.Errorf("parse frontmatter: %w", err)
	}
	return body, true, nil
}
```

### Task 1.2 — Tests

- [ ] Create `internal/frontmatter/frontmatter_test.go`:

```go
package frontmatter_test

import (
	"strings"
	"testing"

	"agentic-workflows/internal/frontmatter"
)

func TestSplitWellFormed(t *testing.T) {
	in := "---\nname: x\ndesc: y\n---\nbody here\n"
	yamlBlock, body, found := frontmatter.Split([]byte(in))
	if !found {
		t.Fatal("expected found")
	}
	if !strings.Contains(string(yamlBlock), "name: x") {
		t.Errorf("yaml block wrong: %q", yamlBlock)
	}
	if string(body) != "body here\n" {
		t.Errorf("body wrong: %q", body)
	}
}

func TestSplitNoFrontmatter(t *testing.T) {
	in := "# heading\nno frontmatter\n"
	yamlBlock, body, found := frontmatter.Split([]byte(in))
	if found {
		t.Error("expected not found")
	}
	if yamlBlock != nil {
		t.Errorf("yaml block should be nil, got %q", yamlBlock)
	}
	if string(body) != in {
		t.Errorf("body should equal input, got %q", body)
	}
}

func TestSplitMissingClosing(t *testing.T) {
	in := "---\nname: x\nbody never closes\n"
	_, body, found := frontmatter.Split([]byte(in))
	if found {
		t.Error("expected not found for missing closing delimiter")
	}
	if string(body) != in {
		t.Errorf("body should equal input, got %q", body)
	}
}

func TestSplitCRLF(t *testing.T) {
	in := "---\r\nname: x\r\n---\r\nbody\r\n"
	yamlBlock, _, found := frontmatter.Split([]byte(in))
	if !found {
		t.Fatal("expected found with CRLF")
	}
	if !strings.Contains(string(yamlBlock), "name: x") {
		t.Errorf("yaml block wrong: %q", yamlBlock)
	}
}

func TestParseIntoStruct(t *testing.T) {
	var fm struct {
		Name string `yaml:"name"`
	}
	in := "---\nname: hello\n---\nbody\n"
	body, found, err := frontmatter.Parse([]byte(in), &fm)
	if err != nil || !found {
		t.Fatalf("Parse: found=%v err=%v", found, err)
	}
	if fm.Name != "hello" {
		t.Errorf("Name = %q, want hello", fm.Name)
	}
	if string(body) != "body\n" {
		t.Errorf("body = %q", body)
	}
}

func TestParseMalformedYAML(t *testing.T) {
	var fm struct {
		Name string `yaml:"name"`
	}
	in := "---\nname: [unterminated\n---\nbody\n"
	if _, _, err := frontmatter.Parse([]byte(in), &fm); err == nil {
		t.Error("expected error for malformed YAML")
	}
}
```

- [ ] Verify: `go test ./internal/frontmatter/` → `ok`.

### Task 1.3 — Commit

- [ ] `./x gate` → `0 issues.`
- [ ] `git add internal/frontmatter/`
- [ ] `git commit -m "feat(awf): add internal/frontmatter YAML frontmatter parser"`

---

## Phase 2 — Rename `adrtools` → `adr`, refactor onto `frontmatter`

### Task 2.1 — Move the files

- [ ] `git mv internal/adrtools internal/adr`
- [ ] `git mv internal/adr/adrtools.go internal/adr/adr.go`
- [ ] `git mv internal/adr/adrtools_test.go internal/adr/adr_test.go`

### Task 2.2 — Rewrite `internal/adr/adr.go`

- [ ] Replace the entire contents of `internal/adr/adr.go` with:

```go
// Package adr parses ADR files under docs/decisions and renders the ACTIVE.md
// index. Generated by awf sync (regenerates docs/decisions/ACTIVE.md).
package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"agentic-workflows/internal/frontmatter"
)

// statusOrder defines the section order in ACTIVE.md.
var statusOrder = []string{"Accepted", "Implemented", "Proposed", "Superseded"}

// ADR is a parsed ADR record.
type ADR struct {
	Number   string // e.g. "0001"
	Title    string // e.g. "ADR-0001: Template Overlay Rendering Engine"
	Status   string // e.g. "Accepted"
	Filename string // e.g. "0001-template-overlay-rendering-engine.md"
	Path     string // absolute or dir-relative path as globbed
}

var fileRe = regexp.MustCompile(`^(\d{4})-.*\.md$`)

// ParseDir scans dir for ADR files (NNNN-*.md) and parses each into an ADR.
func ParseDir(dir string) ([]ADR, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", dir, err)
	}
	var adrs []ADR
	for _, path := range matches {
		base := filepath.Base(path)
		m := fileRe.FindStringSubmatch(base)
		if m == nil {
			continue // skip ACTIVE.md, README.md, template.md
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", base, err)
		}
		a, err := parse(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", base, err)
		}
		a.Number, a.Filename, a.Path = m[1], base, path
		adrs = append(adrs, a)
	}
	return adrs, nil
}

// adrFrontmatter holds the YAML fields we care about.
type adrFrontmatter struct {
	Status string `yaml:"status"`
}

// parse extracts status (frontmatter) and title (first `# ` heading) from one ADR.
func parse(data []byte) (ADR, error) {
	var fm adrFrontmatter
	body, _, err := frontmatter.Parse(data, &fm)
	if err != nil {
		return ADR{}, err
	}
	a := ADR{Status: fm.Status}
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "# ") {
			a.Title = strings.TrimPrefix(line, "# ")
			break
		}
	}
	return a, nil
}

// RenderActiveMD renders the ACTIVE.md index for dir, grouped by status. It
// returns "" when dir holds no ADRs (so callers produce no file).
func RenderActiveMD(dir string) (string, error) {
	adrs, err := ParseDir(dir)
	if err != nil {
		return "", err
	}
	if len(adrs) == 0 {
		return "", nil
	}

	groups := make(map[string][]ADR)
	for _, a := range adrs {
		groups[a.Status] = append(groups[a.Status], a)
	}
	for k := range groups {
		sort.Slice(groups[k], func(i, j int) bool { return groups[k][i].Number < groups[k][j].Number })
	}

	seen := make(map[string]bool)
	var ordered []string
	for _, s := range statusOrder {
		if len(groups[s]) > 0 {
			ordered = append(ordered, s)
			seen[s] = true
		}
	}
	var extra []string
	for k := range groups {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	ordered = append(ordered, extra...)

	var sb strings.Builder
	sb.WriteString("<!-- GENERATED by awf sync — do not edit by hand. -->\n")
	for _, status := range ordered {
		sb.WriteString("\n## ")
		sb.WriteString(status)
		sb.WriteString("\n\n")
		for _, a := range groups[status] {
			fmt.Fprintf(&sb, "- [%s](%s) — %s\n", a.Title, a.Filename, a.Status)
		}
	}
	return sb.String(), nil
}
```

### Task 2.3 — Update `internal/adr/adr_test.go`

- [ ] In `internal/adr/adr_test.go`: change the package clause to `package adr_test`, the import to `"agentic-workflows/internal/adr"`, and every `adrtools.GenerateActiveMD(` call to `adr.RenderActiveMD(`. Rename the two test functions `TestGenerateActiveMDGroupsByStatus` → `TestRenderActiveMDGroupsByStatus` and `TestGenerateActiveMDEmptyWhenNoADRs` → `TestRenderActiveMDEmptyWhenNoADRs`. Also update the now-stale references to the old name: the doc comment on line ~12 (`// TestGenerateActiveMDGroupsByStatus …`) and both `t.Fatalf("GenerateActiveMD: %v", err)` message strings → `t.Fatalf("RenderActiveMD: %v", err)`. (`os` and `filepath` are already imported, so the new `ParseDir` test below needs no import change.)
- [ ] Append a `ParseDir` test to `internal/adr/adr_test.go`:

```go
func TestParseDirExtractsStatusAndTitle(t *testing.T) {
	dir := t.TempDir()
	content := "---\nstatus: Accepted\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0007: Example Title\n## Context\nx\n"
	if err := os.WriteFile(filepath.Join(dir, "0007-example.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-ADR file must be skipped.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	got := adrs[0]
	if got.Number != "0007" || got.Status != "Accepted" || got.Title != "ADR-0007: Example Title" || got.Filename != "0007-example.md" {
		t.Errorf("unexpected ADR: %+v", got)
	}
}
```

### Task 2.4 — Update `internal/project/project.go` import + call site

- [ ] In `internal/project/project.go`, change the import `"agentic-workflows/internal/adrtools"` → `"agentic-workflows/internal/adr"`.
- [ ] In `generateActiveMD` (project.go ~299), change `adrtools.GenerateActiveMD(dir)` → `adr.RenderActiveMD(dir)`.

### Task 2.5 — Repoint the architecture doc part

- [ ] In `.claude/awf/parts/doc-architecture.md`, replace the `internal/adrtools/` bullet line with two lines:

old:
```
- **`internal/adrtools/`** — regenerates `docs/decisions/ACTIVE.md` from ADR frontmatter; invoked by `awf sync` (`./x sync`).
```
new:
```
- **`internal/frontmatter/`** — the single parser for `---`-delimited YAML frontmatter; used by `internal/adr` and skill/agent validation.
- **`internal/adr/`** — parses ADRs and regenerates `docs/decisions/ACTIVE.md` from their frontmatter; invoked by `awf sync` (`./x sync`).
```

### Task 2.6 — Re-sync, verify, commit

- [ ] `./x sync` — regenerates `docs/architecture.md` from the part; `ACTIVE.md` unchanged (byte-identical render).
- [ ] `go test ./internal/adr/ ./internal/project/` → `ok`.
- [ ] `./x gate` → `0 issues.`; `./x check` → `awf check: clean`.
- [ ] `git add internal/adr/ internal/project/project.go .claude/awf/parts/doc-architecture.md docs/architecture.md .claude/awf.lock`
- [ ] `git commit -m "refactor(awf): rename internal/adrtools to internal/adr on shared parser"`

---

## Phase 3 — Skill/agent frontmatter validation in sync + check

### Task 3.1 — Add the validator and kind helper

- [ ] In `internal/project/project.go`, add `"agentic-workflows/internal/frontmatter"` to the import block (keep imports sorted: it sorts before `internal/manifest`, after `internal/config`).
- [ ] Add these helpers (place just after the `nonNil` helper):

```go
// skillFrontmatter is the rendered skill/agent frontmatter contract Claude Code
// requires.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// isSkillOrAgent reports whether a template id renders a skill or agent (the
// outputs that must carry name/description frontmatter).
func isSkillOrAgent(templateID string) bool {
	return strings.HasPrefix(templateID, "skills/") || strings.HasPrefix(templateID, "agents/")
}

// validateFrontmatter checks that content has parseable frontmatter with a
// non-empty name and description.
func validateFrontmatter(content []byte) error {
	var fm skillFrontmatter
	_, found, err := frontmatter.Parse(content, &fm)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("missing frontmatter")
	}
	if strings.TrimSpace(fm.Name) == "" {
		return fmt.Errorf("frontmatter name is empty")
	}
	if strings.TrimSpace(fm.Description) == "" {
		return fmt.Errorf("frontmatter description is empty")
	}
	return nil
}
```

### Task 3.2 — Fail fast in `Sync`

- [ ] In `Sync`, immediately after the `files, err := p.RenderAll()` error check and before the `generateActiveMD` append, insert:

```go
	for _, f := range files {
		if isSkillOrAgent(f.TemplateID) {
			if err := validateFrontmatter([]byte(f.Content)); err != nil {
				return fmt.Errorf("invalid frontmatter in %s: %w", f.Path, err)
			}
		}
	}
```

### Task 3.3 — Emit `invalid-frontmatter` drift in `Check`

- [ ] In `Check`, in the generic lock loop, replace the final hash-equality block:

old:
```go
		if manifest.Hash(onDisk) != e.OutputHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "hand-edited", Detail: "on-disk output differs from lock"})
		}
	}
```
new:
```go
		if manifest.Hash(onDisk) != e.OutputHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "hand-edited", Detail: "on-disk output differs from lock"})
			continue
		}
		// In-sync skill/agent files must still carry valid frontmatter (subordinate
		// to the hash kinds above — a re-sync is the fix for those).
		if isSkillOrAgent(rf.TemplateID) {
			if err := validateFrontmatter(onDisk); err != nil {
				drift = append(drift, manifest.Drift{Path: path, Kind: "invalid-frontmatter", Detail: err.Error()})
			}
		}
	}
```

### Task 3.4 — Tests

- [ ] Append to `internal/project/project_test.go` a check test that a hand-edit breaking frontmatter (while keeping the output hash in sync) is reported. The cleanest deterministic form: sync a project, hand-edit a skill to blank frontmatter, then re-point the lock's OutputHash to the edited content so the hash check passes and the frontmatter check fires:

```go
func TestCheckDetectsInvalidFrontmatter(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, _ := Open(root)
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	skillPath := ".claude/skills/example-tdd/SKILL.md"
	broken := "---\nname: \"\"\ndescription: \"\"\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(root, skillPath), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-point the lock OutputHash to the edited content so the file is "in sync"
	// by hash and the frontmatter check is what fires.
	lock, err := manifest.Load(filepath.Join(root, ".claude", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	e := lock.Files[skillPath]
	e.OutputHash = manifest.Hash([]byte(broken))
	lock.Files[skillPath] = e
	if err := lock.Save(filepath.Join(root, ".claude", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range drift {
		if d.Path == skillPath && d.Kind == "invalid-frontmatter" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid-frontmatter drift for %s, got %#v", skillPath, drift)
	}
}
```

(`manifest` is already imported in `project_test.go`.)

- [ ] Verify: `go test ./internal/project/` → `ok`.

### Task 3.5 — Commit

- [ ] `./x sync` (no output change expected — validates current skills, which are valid) ; `./x gate` → `0 issues.`; `./x check` → `awf check: clean`.
- [ ] `git add internal/project/project.go internal/project/project_test.go .claude/awf.lock`
- [ ] `git commit -m "feat(awf): validate skill/agent frontmatter in sync and check"`

---

## Phase 4 — Golden test: every template produces valid frontmatter

### Task 4.1 — All-templates frontmatter test

- [ ] Create `internal/project/frontmatter_test.go`:

```go
package project

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"agentic-workflows/internal/catalog"
	"agentic-workflows/internal/frontmatter"
	"agentic-workflows/internal/render"
	"agentic-workflows/templates"
)

// TestAllTemplatesProduceValidFrontmatter renders every catalog skill and agent
// template with a minimal-adopter data set (prefix + every referenced var seeded
// empty + full layout) and asserts the frontmatter parses with non-empty
// name/description and no leaked <no value> token.
func TestAllTemplatesProduceValidFrontmatter(t *testing.T) {
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	layout := map[string]any{
		"docsDir": "docs", "adrDir": "docs/decisions", "activeMd": "docs/decisions/ACTIVE.md",
		"adrReadme": "docs/decisions/README.md", "adrTemplate": "docs/decisions/template.md",
		"plansDir": "docs/plans",
	}
	check := func(tid string) {
		t.Helper()
		src, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			t.Fatalf("read %s: %v", tid, err)
		}
		vars := map[string]any{}
		for _, v := range render.ReferencedVars(string(src)) {
			vars[v] = ""
		}
		data := map[string]any{"prefix": "awf", "vars": vars, "layout": layout, "data": map[string]any{}}
		out, err := render.Render(string(src), nil, func(string) (string, error) { return "", nil }, data)
		if err != nil {
			t.Fatalf("render %s: %v", tid, err)
		}
		var fm skillFrontmatter
		_, found, err := frontmatter.Parse([]byte(out), &fm)
		if err != nil {
			t.Fatalf("%s: frontmatter parse: %v", tid, err)
		}
		if !found {
			t.Errorf("%s: no frontmatter", tid)
			return
		}
		if strings.TrimSpace(fm.Name) == "" {
			t.Errorf("%s: empty name", tid)
		}
		if strings.TrimSpace(fm.Description) == "" {
			t.Errorf("%s: empty description", tid)
		}
		if strings.Contains(fm.Name+fm.Description, "<no value>") {
			t.Errorf("%s: <no value> leaked into frontmatter", tid)
		}
	}
	for name := range cat.Skills {
		check(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name))
	}
	for name := range cat.Agents {
		check(fmt.Sprintf("agents/%s.md.tmpl", name))
	}
}
```

  The test is `package project` (internal test), so it reuses the unexported `skillFrontmatter` type from `project.go` and calls `frontmatter.Parse` directly via the import above.

- [ ] Verify: `go test ./internal/project/ -run TestAllTemplatesProduceValidFrontmatter -v` → `PASS`. If any template fails with empty/`<no value>` description, that template's frontmatter references an unguarded empty var — fix by guarding it in the template (and re-sync), then re-run.

### Task 4.2 — Commit

- [ ] `./x gate` → `0 issues.`
- [ ] `git add internal/project/frontmatter_test.go`
- [ ] `git commit -m "test(awf): assert every skill/agent template renders valid frontmatter"`

---

## Phase 5 — Doc-currency + finalize

### Task 5.1 — Add the AGENTS.md invariant

- [ ] In `.claude/awf.yaml`, under `agentsDoc.data.invariants`, append after the Conventional-Commits entry:

```yaml
      - text: "**Valid skill/agent frontmatter.** Rendered skills and agents carry parseable YAML frontmatter with non-empty `name`/`description`; `awf sync` fails fast and `awf check` reports `invalid-frontmatter` otherwise."
        ref: ADR-0006
```

- [ ] `./x sync` — regenerates `AGENTS.md` with the new invariant; refreshes the lock.
- [ ] `./x gate` → `0 issues.`; `./x check` → `awf check: clean`.
- [ ] `git add .claude/awf.yaml AGENTS.md .claude/awf.lock`
- [ ] `git commit -m "docs(awf): record skill-frontmatter validation invariant in AGENTS.md"`

### Task 5.2 — Flip ADR-0006 to Implemented

- [ ] In `docs/decisions/0006-frontmatter-parser-and-skill-validation.md`, change frontmatter `status: Accepted` → `status: Implemented`.
- [ ] `./x sync` — regenerates `docs/decisions/ACTIVE.md` (moves 0006 to Implemented).
- [ ] `./x gate` → `0 issues.`; `./x check` → `awf check: clean`.
- [ ] `git add docs/decisions/0006-frontmatter-parser-and-skill-validation.md docs/decisions/ACTIVE.md .claude/awf.lock`
- [ ] `git commit -m "docs(adr): mark 0006 Implemented"`

### Task 5.3 — Terminal handoff

- [ ] Invoke `awf-reviewing-impl` against the implementation commit range (Phases 1–5).

---

## Notes

- **Ordering:** Phase 1 (`internal/frontmatter`) precedes Phase 2 (`internal/adr` depends on it) and Phase 3 (validator uses it). Phase 2's rename must land atomically (move + package rewrite + import update + re-sync) to keep the build green and `awf check` clean.
- **Default `docsDir`** (ADR-0005) means `generateActiveMD` keeps using `<docsDir>/decisions`; nothing in this plan changes ACTIVE.md output (the rename is behaviour-preserving — an Invariant of ADR-0006).
- The frozen ADR-0005 and its plan, and the `spine_test.go` `activeMdRegenCmd` fixture strings, keep their historical `internal/adrtools` text — they are not code imports and are out of scope.
- The `// invariant:` tagging convention is **not** in scope (ADR-0007); ADR-0006's invariants are plain textual contracts here.
