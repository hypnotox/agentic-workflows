# Plan: Invariant-Backing Tooling

Implements **[ADR-0007](../decisions/0007-invariant-backing-tooling.md)** (Accepted). Design
rationale lives in the ADR — this plan is the execution record.

## Goal

Ship the `// invariant:` convention in the binary: extend `internal/adr` with section parsing;
add `internal/invariants` (`Check`); expose `awf invariants` and fold it into `awf check`; update
the ADR-authoring convention surfaces; retro-tag ADR-0005 and ADR-0006. After this, an Implemented
ADR's `` `inv: <slug>` `` bullets must each be backed by a `// invariant: <slug>` test or `awf check`
fails.

## Architecture summary

- `internal/adr`: `ADR` gains `Sections map[string]string`, populated by splitting the body on `## `.
- `internal/invariants`: `Check(decisionsDir, root) ([]Finding, error)` — required slugs from
  Implemented ADRs' Invariants sections vs `// invariant:` tags in `*.go`; duplicate slug = error.
- `internal/project`: `CheckInvariants()` wraps `invariants.Check(<docsDir>/decisions, root)`.
- `cmd/awf`: `awf invariants` subcommand; `runCheck` folds in `CheckInvariants` (distinct from drift).
- Convention: `proposing-adr` skill, `docs/decisions/template.md`, `docs/decisions/README.md`,
  and the `AGENTS.md` invariant row (via `agentsDoc.data.invariants` in `.claude/awf.yaml`).
- Retro-tag ADR-0005/0006 invariant bullets + the named backing tests.

## Tech stack

- Go 1.26; packages: `internal/adr`, `internal/invariants` (new), `internal/project`, `cmd/awf`. No new deps.
- Gate: `./x gate` per code commit; pre-commit also runs `./x check` (which, after Phase 3, runs the invariant check).

## File structure

**Created:** `internal/invariants/invariants.go`, `internal/invariants/invariants_test.go`, `cmd/awf/invariants.go`, `cmd/awf/invariants_test.go`

**Modified:** `internal/adr/adr.go` (+`Sections`), `internal/adr/adr_test.go` (sections test), `internal/project/project.go` (+`CheckInvariants`), `cmd/awf/main.go` (dispatch+usage), `cmd/awf/check.go` (fold), `x` (invariants target), `templates/skills/proposing-adr/SKILL.md.tmpl`, `docs/decisions/template.md`, `docs/decisions/README.md`, `.claude/awf.yaml` (+AGENTS.md invariant row) + `AGENTS.md` (re-rendered), `docs/decisions/0005-*.md` + `0006-*.md` (retro-tag bullets), `internal/config/config_test.go` + `internal/project/project_test.go` + `internal/frontmatter/frontmatter_test.go` (backing `// invariant:` comments), `docs/decisions/0007-*.md` (Implemented flip).

**Slug ↔ test map** (authoritative for this plan):

| slug | ADR | backing test (file) |
|---|---|---|
| `adr-sections-parsed` | 0007 | `TestParseDirExtractsSections` (adr_test.go) |
| `invariants-implemented-only` | 0007 | `TestCheckImplementedOnly` (invariants_test.go) |
| `invariants-unbacked-detected` | 0007 | `TestCheckUnbackedAndBacked` (invariants_test.go) |
| `invariants-duplicate-slug` | 0007 | `TestCheckDuplicateSlug` (invariants_test.go) |
| `invariants-in-check` | 0007 | `TestRunCheckFailsOnUnbackedInvariant` (cmd/awf/invariants_test.go) |
| `docsdir-default` | 0005 | `TestDocsDirDefaultsToDocs` (config_test.go) |
| `layout-derivation` | 0005 | `TestLayoutDerivesFromDocsDir` (project_test.go) |
| `sync-generates-active-md` | 0005 | `TestSyncGeneratesActiveMDAndCheckDetectsStaleness` (project_test.go) |
| `check-active-md-stale` | 0005 | `TestSyncGeneratesActiveMDAndCheckDetectsStaleness` (project_test.go) |
| `frontmatter-split` | 0006 | `TestSplitWellFormed` + `TestSplitNoFrontmatter` (frontmatter_test.go) |
| `render-active-md` | 0006 | `TestRenderActiveMDGroupsByStatus` + `TestRenderActiveMDEmptyWhenNoADRs` (adr_test.go) |
| `check-invalid-frontmatter` | 0006 | `TestCheckDetectsInvalidFrontmatter` (project_test.go) |
| `templates-valid-frontmatter` | 0006 | `TestAllTemplatesProduceValidFrontmatter` (frontmatter_test.go) |

---

## Phase 1 — `internal/adr` section parsing

### Task 1.1 — Add `Sections` to `ADR` and parse it

- [ ] In `internal/adr/adr.go`, add the field to the `ADR` struct (after `Path`):

```go
	Path     string // path as globbed
	Sections map[string]string // `## ` heading -> section body
```

- [ ] In `parse`, populate it. Replace the `a := ADR{Status: fm.Status}` line with:

```go
	a := ADR{Status: fm.Status, Sections: sections(string(body))}
```

- [ ] Add the `sections` helper at the end of `internal/adr/adr.go`:

```go
// sections splits a markdown body into a map of `## ` heading text -> section
// body (the lines until the next `## ` heading).
func sections(body string) map[string]string {
	out := map[string]string{}
	var name string
	var b strings.Builder
	flush := func() {
		if name != "" {
			out[name] = b.String()
		}
		b.Reset()
	}
	for _, line := range strings.Split(body, "\n") {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			name = strings.TrimSpace(h)
			continue
		}
		if name != "" {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	flush()
	return out
}
```

### Task 1.2 — Test (backs `adr-sections-parsed`)

- [ ] Append to `internal/adr/adr_test.go`:

```go
// invariant: adr-sections-parsed
func TestParseDirExtractsSections(t *testing.T) {
	dir := t.TempDir()
	content := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0009: S\n## Context\nctx body\n## Invariants\n- `inv: example-slug` — a thing.\n## Consequences\ncons\n"
	if err := os.WriteFile(filepath.Join(dir, "0009-s.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	if len(adrs) != 1 {
		t.Fatalf("expected 1 ADR, got %d", len(adrs))
	}
	inv := adrs[0].Sections["Invariants"]
	if !strings.Contains(inv, "inv: example-slug") {
		t.Errorf("Invariants section missing tag; got: %q", inv)
	}
	if !strings.Contains(adrs[0].Sections["Context"], "ctx body") {
		t.Errorf("Context section wrong: %q", adrs[0].Sections["Context"])
	}
}
```

- [ ] Verify: `go test ./internal/adr/` → `ok`.

### Task 1.3 — Commit

- [ ] `./x gate` → `0 issues.`
- [ ] `git add internal/adr/adr.go internal/adr/adr_test.go`
- [ ] `git commit -m "feat(awf): parse ADR sections in internal/adr"`

---

## Phase 2 — `internal/invariants` package

### Task 2.1 — Create the checker

- [ ] Create `internal/invariants/invariants.go`:

```go
// Package invariants checks that each Implemented ADR's `inv: <slug>` invariant
// tags are backed by a `// invariant: <slug>` comment in the project's Go source.
package invariants

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"agentic-workflows/internal/adr"
)

// Finding is an Implemented-ADR invariant slug with no backing test.
type Finding struct {
	Slug string
	ADR  string // filename of the declaring ADR
}

var (
	tagRe  = regexp.MustCompile("`inv:\\s*([a-z0-9-]+)`")
	testRe = regexp.MustCompile(`//\s*invariant:\s*([a-z0-9-]+)`)
)

// Check returns a Finding for each Implemented-ADR invariant slug under
// decisionsDir lacking a `// invariant: <slug>` comment in any *.go file under
// root. A slug declared by two ADRs is an error.
func Check(decisionsDir, root string) ([]Finding, error) {
	adrs, err := adr.ParseDir(decisionsDir)
	if err != nil {
		return nil, err
	}
	required := map[string]string{} // slug -> declaring ADR filename
	for _, a := range adrs {
		if a.Status != "Implemented" {
			continue
		}
		for _, m := range tagRe.FindAllStringSubmatch(a.Sections["Invariants"], -1) {
			slug := m[1]
			if prev, ok := required[slug]; ok {
				return nil, fmt.Errorf("duplicate inv slug %q (in %s and %s)", slug, prev, a.Filename)
			}
			required[slug] = a.Filename
		}
	}
	present, err := scanTags(root)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for slug, file := range required {
		if !present[slug] {
			findings = append(findings, Finding{Slug: slug, ADR: file})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Slug < findings[j].Slug })
	return findings, nil
}

// scanTags collects every slug named by a `// invariant: <slug>` comment in a
// *.go file under root (skipping .git/vendor/node_modules).
func scanTags(root string) (map[string]bool, error) {
	present := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range testRe.FindAllStringSubmatch(string(data), -1) {
			present[m[1]] = true
		}
		return nil
	})
	return present, err
}
```

### Task 2.2 — Tests (back `invariants-implemented-only`, `invariants-unbacked-detected`, `invariants-duplicate-slug`)

Fixtures use the `fixture-` slug prefix so they never collide with real ADR slugs (the
`fixture-*` literals in this test source will register as "present" in a repo-wide scan, but no
real ADR requires them).

- [ ] Create `internal/invariants/invariants_test.go`:

```go
package invariants_test

import (
	"os"
	"path/filepath"
	"testing"

	"agentic-workflows/internal/invariants"
)

func writeADR(t *testing.T, dir, name, status, invBody string) {
	t.Helper()
	content := "---\nstatus: " + status + "\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-X: T\n## Invariants\n" + invBody + "\n## Consequences\nc\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// invariant: invariants-implemented-only
func TestCheckImplementedOnly(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	// Implemented ADR with an unbacked slug -> required.
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-impl` — x.")
	// Proposed/Accepted/Superseded ADRs -> not required, even though unbacked.
	writeADR(t, dir, "0002-b.md", "Proposed", "- `inv: fixture-prop` — x.")
	writeADR(t, dir, "0003-c.md", "Accepted", "- `inv: fixture-acc` — x.")
	writeADR(t, dir, "0004-d.md", "Superseded by ADR-0001", "- `inv: fixture-sup` — x.")
	findings, err := invariants.Check(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Slug != "fixture-impl" {
		t.Errorf("expected only fixture-impl required, got %#v", findings)
	}
}

// invariant: invariants-unbacked-detected
func TestCheckUnbackedAndBacked(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-backed` — x.\n- `inv: fixture-missing` — y.")
	// Back only one slug, in a .go file under root.
	src := "package x\n// invariant: fixture-backed\nfunc T() {}\n"
	if err := os.WriteFile(filepath.Join(root, "x.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := invariants.Check(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Slug != "fixture-missing" {
		t.Errorf("expected only fixture-missing unbacked, got %#v", findings)
	}
}

// invariant: invariants-duplicate-slug
func TestCheckDuplicateSlug(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-dup` — x.")
	writeADR(t, dir, "0002-b.md", "Implemented", "- `inv: fixture-dup` — y.")
	if _, err := invariants.Check(dir, root); err == nil {
		t.Error("expected error for duplicate slug")
	}
}
```

- [ ] Verify: `go test ./internal/invariants/` → `ok`.

### Task 2.3 — Commit

- [ ] `./x gate` → `0 issues.`
- [ ] `git add internal/invariants/`
- [ ] `git commit -m "feat(awf): add internal/invariants backing checker"`

---

## Phase 3 — Binary surface: `awf invariants` + fold into `check`

### Task 3.1 — `Project.CheckInvariants`

- [ ] In `internal/project/project.go`, add `"agentic-workflows/internal/invariants"` to the import block (sorts after `internal/frontmatter`, before `internal/manifest`).
- [ ] Add the method (place after `Check`):

```go
// CheckInvariants reports Implemented-ADR invariant slugs that lack a backing
// // invariant: test under the project root.
func (p *Project) CheckInvariants() ([]invariants.Finding, error) {
	return invariants.Check(filepath.Join(p.Root, p.Cfg.DocsDir, "decisions"), p.Root)
}
```

### Task 3.2 — Fold into `runCheck`

- [ ] Replace the body of `runCheck` in `cmd/awf/check.go` with:

```go
func runCheck(root string) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	drift, err := p.Check()
	if err != nil {
		return err
	}
	findings, err := p.CheckInvariants()
	if err != nil {
		return err
	}
	for _, d := range drift {
		fmt.Printf("  %-14s %s — %s\n", d.Kind, d.Path, d.Detail)
	}
	for _, f := range findings {
		fmt.Printf("  %-14s %s — invariant %q has no backing // invariant: test\n", "unbacked-inv", f.ADR, f.Slug)
	}
	if len(drift) == 0 && len(findings) == 0 {
		fmt.Println("awf check: clean")
		return nil
	}
	return fmt.Errorf("awf check: %d drift(s), %d unbacked invariant(s)", len(drift), len(findings))
}
```

### Task 3.3 — `awf invariants` subcommand

- [ ] Create `cmd/awf/invariants.go`:

```go
package main

import (
	"fmt"

	"agentic-workflows/internal/project"
)

func runInvariants(root string) error {
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	findings, err := p.CheckInvariants()
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		fmt.Println("awf invariants: clean")
		return nil
	}
	for _, f := range findings {
		fmt.Printf("  %s — invariant %q has no backing // invariant: test\n", f.ADR, f.Slug)
	}
	return fmt.Errorf("awf invariants: %d unbacked invariant(s)", len(findings))
}
```

- [ ] In `cmd/awf/main.go`, add the dispatch case after `case "check":`:

```go
	case "invariants":
		fatalIf(runInvariants(cwd))
```

- [ ] In `cmd/awf/main.go`, update the usage string:

```go
		fmt.Fprintln(os.Stderr, "usage: awf <init|sync|check|invariants|list|add|setup> [args]")
```

### Task 3.4 — `./x invariants`

- [ ] In `x`, add a case after the `check)` block:

```
  invariants)
    go run ./cmd/awf invariants "$@"
    ;;
```

- [ ] In `x`, update the usage string to include `invariants`:

```
    echo "usage: ./x <gate [full]|lint|fmt|test|sync|check|invariants|setup|build|install>" >&2
```

### Task 3.5 — Test (backs `invariants-in-check`)

- [ ] Create `cmd/awf/invariants_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// invariant: invariants-in-check
func TestRunCheckFailsOnUnbackedInvariant(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "prefix: example\nskills: {}\nagents: {}\nhooks: []\n"
	if err := os.WriteFile(filepath.Join(root, ".claude", "awf.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSync(root); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adr := "---\nstatus: Implemented\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-0001: X\n## Invariants\n- `inv: cmd-needs-backing` — x.\n## Consequences\nc\n"
	if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte(adr), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-sync so ACTIVE.md is generated and the tree stays drift-clean; the only
	// outstanding issue is the unbacked invariant.
	if err := runSync(root); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	if err := runCheck(root); err == nil {
		t.Error("expected runCheck to fail on unbacked invariant")
	}
	// Back the slug with a .go file under root -> runCheck clean.
	src := "package x\n// invariant: cmd-needs-backing\nfunc T() {}\n"
	if err := os.WriteFile(filepath.Join(root, "backing.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(root); err != nil {
		t.Errorf("expected runCheck clean after backing, got: %v", err)
	}
}
```

- [ ] Verify: `go test ./cmd/awf/` → `ok`.

### Task 3.6 — Commit

- [ ] `./x gate` → `0 issues.`; `./x check` → `awf check: clean` (0005/0006 not yet tagged → no required slugs; 0007 still Accepted).
- [ ] `git add internal/project/project.go cmd/awf/check.go cmd/awf/invariants.go cmd/awf/invariants_test.go x`
- [ ] `git commit -m "feat(awf): add awf invariants subcommand, fold into awf check"`

---

## Phase 4 — Convention surfaces

### Task 4.1 — `proposing-adr` skill

- [ ] In `templates/skills/proposing-adr/SKILL.md.tmpl`, replace the `invariants-rule` step (line ~65):

old:
```
1. **Pair each Invariant with a test.** Each Invariants section bullet must be accompanied by at least one `// invariant: <normalised bullet title>` test shipping in the same commit. Run `{{ .vars.gateCmd }}` to confirm.
```
new:
```
1. **Tag enforceable Invariants and back them with a test.** Give each machine-checkable Invariants bullet an explicit slug, ``- `inv: <slug>` — …``, and add a `// invariant: <slug>` comment to a test that exercises it, shipping in the same commit. `{{ .prefix }}-check` fails once the ADR is `Implemented` if a tagged slug has no backing test. Bullets without a slug remain textual contracts (not machine-checked). Run `{{ .vars.gateCmd }}` and `{{ .vars.checkCmd }}` to confirm.
```

(Confirm `{{ .vars.checkCmd }}` is set in this repo's `.claude/awf.yaml` — it is, `"./x check"`.)

### Task 4.2 — ADR template

- [ ] In `docs/decisions/template.md`, replace the Invariants section body:

old:
```
Checkable constraints that must hold as long as this decision stands — conditions that
should trigger a new ADR if violated:

- ...
```
new:
```
Checkable constraints that must hold as long as this decision stands — conditions that
should trigger a new ADR if violated. Tag each machine-enforceable bullet with a slug and back
it with a `// invariant: <slug>` test; `awf check` enforces tagged slugs once the ADR is
`Implemented`. Untagged bullets are textual contracts.

- `inv: <slug>` — ...
```

### Task 4.3 — ADR README

- [ ] In `docs/decisions/README.md`, append a section before the `## ACTIVE.md` section:

```markdown
## Invariant tagging

Give each machine-enforceable Invariants bullet an explicit slug —
``- `inv: <slug>` — …`` — and add a matching `// invariant: <slug>` comment to a test that
exercises it. `awf check` (here `./x check`) and the standalone `awf invariants` (`./x invariants`)
fail when an **Implemented** ADR has a tagged slug with no backing test. Proposed/Accepted ADRs
are not yet enforced (tests land with implementation); Superseded ADRs are skipped. Bullets
without a slug are textual contracts, not machine-checked.
```

### Task 4.4 — AGENTS.md invariant row

This repo's `AGENTS.md` "Invariants" list is the agent guide's record of machine-enforced
conventions; ADR-0006 added a row there for its frontmatter-validation convention (the
`ref: ADR-0006` entry under `agentsDoc.data.invariants`). ADR-0007 introduces a directly
analogous machine-enforced convention, so it gets a parallel row (kept in lockstep with the
README/skill/template surfaces of Tasks 4.1–4.3).

- [ ] In `.claude/awf.yaml`, add an entry to `agentsDoc.data.invariants` (after the
  `ref: ADR-0006` entry):

```yaml
      - text: "**Backed invariants.** Each machine-enforceable ADR Invariants bullet carries an `inv: <slug>` tag and a `// invariant: <slug>` test; `awf check` (and `awf invariants`) fail when an Implemented ADR has an unbacked tagged slug."
        ref: ADR-0007
```

### Task 4.5 — Re-sync and commit

- [ ] `./x sync` — re-renders `.claude/skills/awf-proposing-adr/SKILL.md` and `AGENTS.md`.
- [ ] `./x gate` → `0 issues.`; `./x check` → `awf check: clean`.
- [ ] `git add templates/skills/proposing-adr/SKILL.md.tmpl docs/decisions/template.md docs/decisions/README.md .claude/ AGENTS.md`
- [ ] `git commit -m "docs(awf): document inv: tagging convention and checker"`

---

## Phase 5 — Retro-tag ADR-0005 and ADR-0006

Authorised by ADR-0007 Decision 6. Each commit adds the `inv:` tag to the ADR bullet **and** the
`// invariant:` comment to the named test together, so `awf check` stays clean.

### Task 5.1 — Tag ADR-0005 bullets + tests

- [ ] In `docs/decisions/0005-*.md`, prefix the four backed bullets (insert `` `inv: <slug>` — `` right after the `- `):
  - `- ` + `` `config.Config` has a `docsDir` field;`` → `` - `inv: docsdir-default` — `config.Config` has a `docsDir` field; `` (rest unchanged)
  - `- The layout paths (` → `` - `inv: layout-derivation` — The layout paths (``
  - `- `awf sync`, run with ≥1` → `` - `inv: sync-generates-active-md` — `awf sync`, run with ≥1``
  - `- `awf check` reports drift for `ACTIVE.md`` → `` - `inv: check-active-md-stale` — `awf check` reports drift for `ACTIVE.md```
- [ ] Add backing comments (one line directly above each `func`):
  - `internal/config/config_test.go`: `// invariant: docsdir-default` above `func TestDocsDirDefaultsToDocs`
  - `internal/project/project_test.go`: `// invariant: layout-derivation` above `func TestLayoutDerivesFromDocsDir`
  - `internal/project/project_test.go`: two lines above `func TestSyncGeneratesActiveMDAndCheckDetectsStaleness`:
    ```go
    // invariant: sync-generates-active-md
    // invariant: check-active-md-stale
    ```
- [ ] `./x sync` (ACTIVE.md unchanged — body edits don't affect the index); `./x gate` → `0 issues.`; `./x check` → `awf check: clean`.
- [ ] `git add docs/decisions/0005-docsdir-layout-and-builtin-active-md.md internal/config/config_test.go internal/project/project_test.go`
- [ ] `git commit -m "docs(adr): retro-tag 0005 invariants with inv: slugs"`

### Task 5.2 — Tag ADR-0006 bullets + tests

- [ ] In `docs/decisions/0006-*.md`, prefix the four backed bullets:
  - `- `frontmatter.Split` on content without` → `` - `inv: frontmatter-split` — `frontmatter.Split` on content without``
  - `- `internal/adr.RenderActiveMD` produces` → `` - `inv: render-active-md` — `internal/adr.RenderActiveMD` produces``
  - `- `awf check` reports an `invalid-frontmatter`` → `` - `inv: check-invalid-frontmatter` — `awf check` reports an `invalid-frontmatter```
  - `- Every catalog skill and agent template,` → `` - `inv: templates-valid-frontmatter` — Every catalog skill and agent template,``
- [ ] Add backing comments:
  - `internal/frontmatter/frontmatter_test.go`: `// invariant: frontmatter-split` above `func TestSplitWellFormed` and above `func TestSplitNoFrontmatter`
  - `internal/adr/adr_test.go`: `// invariant: render-active-md` above `func TestRenderActiveMDGroupsByStatus` and above `func TestRenderActiveMDEmptyWhenNoADRs`
  - `internal/project/project_test.go`: `// invariant: check-invalid-frontmatter` above `func TestCheckDetectsInvalidFrontmatter`
  - `internal/frontmatter/frontmatter_test.go`: `// invariant: templates-valid-frontmatter` is in project, so add it in `internal/project/frontmatter_test.go` above `func TestAllTemplatesProduceValidFrontmatter`
- [ ] `./x sync`; `./x gate` → `0 issues.`; `./x check` → `awf check: clean`.
- [ ] `git add docs/decisions/0006-frontmatter-parser-and-skill-validation.md internal/frontmatter/frontmatter_test.go internal/adr/adr_test.go internal/project/project_test.go internal/project/frontmatter_test.go`
- [ ] `git commit -m "docs(adr): retro-tag 0006 invariants with inv: slugs"`

---

## Phase 6 — Finalize

### Task 6.1 — Flip ADR-0007 to Implemented

- [ ] In `docs/decisions/0007-*.md`, change `status: Accepted` → `status: Implemented`. (Its five `inv:` slugs are already backed by the Phase 1-3 test comments, so `awf check` stays clean once 0007 is enforced.)
- [ ] `./x sync` — regenerates `ACTIVE.md`.
- [ ] `./x gate` → `0 issues.`; `./x check` → `awf check: clean`; `./x invariants` → `awf invariants: clean`.
- [ ] `git add docs/decisions/0007-invariant-backing-tooling.md docs/decisions/ACTIVE.md .claude/awf.lock`
- [ ] `git commit -m "docs(adr): mark 0007 Implemented"`

### Task 6.2 — Terminal handoff

- [ ] Invoke `awf-reviewing-impl` against the implementation commit range (Phases 1–6).

---

## Notes

- **Ordering / no chicken-and-egg:** the checker folds into `check` (Phase 3) while ADR-0005/0006 carry no `inv:` tags yet and 0007 is still Accepted — so `awf check` requires nothing and stays clean. Phase 5 adds each tag together with its backing comment (atomic per commit). Phase 6 flips 0007 to Implemented only after its five slugs are already backed (Phases 1–3).
- **Fixture isolation:** `internal/invariants` tests use `fixture-`-prefixed slugs and scan temp dirs, so no fixture satisfies or requires a real ADR slug.
- **Editing Implemented ADR bodies** (0005/0006) is the retro-tag mandated by ADR-0007 Decision 6 — an additive annotation, not a decision change.
