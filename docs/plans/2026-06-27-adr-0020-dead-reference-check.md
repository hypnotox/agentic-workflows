# Plan: ADR-0020: Dead-Reference Check in `awf check`

Implements [ADR-0020](../decisions/0020-dead-reference-check.md). Design rationale lives in that
ADR; this plan is the execution record and does not restate the rationale.

## Goal

Make `awf check` fail on any dead internal markdown link in awf's managed rendered docs, and ship
the prerequisites that keep a fresh `awf init` green: always-render `ACTIVE.md`, a `plans-readme`
singleton, and the `workflow.md` link fix the check surfaces.

## Architecture summary

- A new pure `internal/refs` package extracts inline-markdown-link targets (stdlib-only, no I/O).
- `internal/project.Check` scans every awf-managed rendered markdown file (filter by template id,
  not `.md` suffix; exclude the `CLAUDE.md` bridge and `.githooks/*`) plus the generated `ACTIVE.md`
  and domain docs, resolves each target file-relative, `os.Stat`s it under the repo root, and emits
  a `dead-reference` `manifest.Drift` per miss.
- `internal/adr.RenderActiveMD` returns a placeholder index for a zero-ADR directory instead of `""`;
  `generateActiveMD` drops its `bool` (always produces a file), and `Check`'s orphaned-ACTIVE branch
  is removed.
- A new always-on `plans-readme` singleton renders `<docsDir>/plans/README.md`, mirroring the
  ADR-0021 `adr-readme`/`adr-template` wiring.
- `templates/docs/workflow.md.tmpl`'s ADR-README link is made `docsDir`-relative.

## Tech stack

- Go 1.26, module `github.com/hypnotox/agentic-workflows`. Stdlib only (`strings`, `os`,
  `path/filepath`). No new dependencies.
- Gate: `./x gate` (~15s) before every commit; 100% statement coverage enforced.

## File structure

**Created**
- `internal/refs/refs.go`
- `internal/refs/refs_test.go`
- `templates/plans-readme/README.md.tmpl`
- `docs/plans/README.md` (rendered by `./x sync` in Phase 3)

**Modified**
- `internal/adr/adr.go`: zero-ADR placeholder
- `internal/adr/adr_test.go`: placeholder assertion
- `internal/project/project.go`: drop `generateActiveMD` bool; remove orphaned-ACTIVE branch;
  `plans-readme` singleton wiring; `isManagedMarkdown` helper + dead-reference scan; `refs` import
- `internal/project/project_test.go`: zero-ADR sync test flip
- `internal/project/coverage_test.go`: delete orphaned-ACTIVE test
- `internal/catalog/catalog.go`: `PlansReadme` field
- `internal/config/config.go`: `IsSingletonKind` adds `plans-readme`
- `templates/catalog.yaml`: `plansReadme` sections
- `templates/embed.go`: embed `plans-readme`
- `templates/docs/workflow.md.tmpl`: `docsDir`-relative ADR-README link
- `docs/workflow.md`: re-rendered (Phase 4)
- `.awf/domains/parts/rendering/current-state.md`, `.awf/domains/parts/adr-system/current-state.md`:
  narrative refresh (Phase 5)
- `.awf/agents-doc.yaml`: dead-reference invariant bullet (Phase 5)
- `docs/domains/rendering.md`, `docs/domains/adr-system.md`, `docs/decisions/ACTIVE.md`, `AGENTS.md`:
  re-rendered (Phase 5)
- `docs/decisions/0020-dead-reference-check.md`: status → Implemented (Phase 5)
- `.awf/awf.lock`: re-rendered each phase that changes managed output

---

## Phase 1: `internal/refs` markdown-link extractor

ADR-0020 Decision 1. Pure, stdlib-only, no I/O.

- [ ] **Task 1.1: Create `internal/refs/refs.go`** with exactly this content:

```go
// Package refs extracts internal markdown link targets from rendered content.
// It is pure and stdlib-only: it performs no I/O and resolves no paths; callers
// resolve and stat the returned targets. (ADR-0020)
package refs

import "strings"

// Links returns the relative-path targets of inline markdown links ([text](target))
// in content, in order of appearance. It skips: external targets (http://,
// https://, mailto:, tel:) and bare #fragment anchors; and any link inside a fenced
// code block (opened by ``` or ~~~). A trailing #anchor and a (target "title") title
// are stripped, leaving the bare relative path. Reference-style links ([text][id])
// are out of scope.
func Links(content string) []string {
	var out []string
	inFence := false
	fence := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if inFence {
			if strings.HasPrefix(trimmed, fence) {
				inFence = false
			}
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inFence, fence = true, "```"
		case strings.HasPrefix(trimmed, "~~~"):
			inFence, fence = true, "~~~"
		default:
			out = append(out, lineLinks(line)...)
		}
	}
	return out
}

// lineLinks extracts the target of every [text](target) on a single line.
func lineLinks(line string) []string {
	var out []string
	for {
		open := strings.IndexByte(line, '[')
		if open < 0 {
			return out
		}
		rest := line[open+1:]
		mid := strings.Index(rest, "](")
		if mid < 0 {
			return out
		}
		dest := rest[mid+2:]
		end := strings.IndexByte(dest, ')')
		if end < 0 {
			line = rest
			continue
		}
		if t := normalizeTarget(dest[:end]); t != "" {
			out = append(out, t)
		}
		line = dest[end+1:]
	}
}

// normalizeTarget strips an optional title and trailing #anchor, unwraps an
// <...> destination, and returns "" for external or anchor-only targets.
func normalizeTarget(dest string) string {
	dest = strings.TrimSpace(dest)
	if i := strings.IndexAny(dest, " \t"); i >= 0 {
		dest = dest[:i]
	}
	dest = strings.TrimPrefix(dest, "<")
	dest = strings.TrimSuffix(dest, ">")
	if i := strings.IndexByte(dest, '#'); i >= 0 {
		dest = dest[:i]
	}
	if dest == "" {
		return ""
	}
	for _, scheme := range []string{"http://", "https://", "mailto:", "tel:"} {
		if strings.HasPrefix(dest, scheme) {
			return ""
		}
	}
	return dest
}
```

- [ ] **Task 1.2: Create `internal/refs/refs_test.go`** with exactly this content (one table test
  covering every branch: simple link, external schemes, anchor-only, anchor strip, title strip,
  angle-bracket dest, multiple links, both fence flavours, unterminated dest, bracket-without-link,
  empty):

```go
package refs_test

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/refs"
)

func TestLinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"simple", "see [a](b.md) here", []string{"b.md"}},
		{"http skipped", "[x](http://e.com)", nil},
		{"https skipped", "[x](https://e.com)", nil},
		{"mailto skipped", "[x](mailto:a@b.c)", nil},
		{"tel skipped", "[x](tel:123)", nil},
		{"anchor-only skipped", "[x](#frag)", nil},
		{"trailing anchor stripped", "[x](b.md#sec)", []string{"b.md"}},
		{"title stripped double", "[x](b.md \"T\")", []string{"b.md"}},
		{"title stripped single", "[x](b.md 'T')", []string{"b.md"}},
		{"angle-bracket dest", "[x](<b.md>)", []string{"b.md"}},
		{"multiple", "[a](x.md) and [c](y.md)", []string{"x.md", "y.md"}},
		{"none", "no links here", nil},
		{"bracket without link", "a [bracket only", nil},
		{"unterminated dest", "[a](b.md and more", nil},
		{"fenced backtick", "```\n[a](b.md)\n```", nil},
		{"fenced tilde", "~~~\n[a](b.md)\n~~~", nil},
		{"link after fence", "```\n[a](skip.md)\n```\n[b](keep.md)", []string{"keep.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := refs.Links(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("Links(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("Links(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}
```

- [ ] **Task 1.3: Verify the package.** Run:

```
go test ./internal/refs/
```

Expected: `ok  	github.com/hypnotox/agentic-workflows/internal/refs`.

- [ ] **Task 1.4: Gate and commit.** Run `./x gate` (expect `coverage: 100.0%`, `0 issues.`),
  then:

```
git add internal/refs/refs.go internal/refs/refs_test.go
git commit -m "feat(awf): add internal/refs markdown-link extractor"
```

---

## Phase 2: `ACTIVE.md` always renders

ADR-0020 Decision 6 (partial-item supersedence of ADR-0005 `sync-generates-active-md` and ADR-0006
`render-active-md`). No managed-output change in this repo (it has ADRs), so this phase is
code + tests only.

- [ ] **Task 2.1: `internal/adr/adr.go`: zero-ADR placeholder.** Replace:

```go
	if len(adrs) == 0 {
		return "", nil
	}
```

with:

```go
	if len(adrs) == 0 {
		return "<!-- GENERATED by awf sync: do not edit by hand. -->\n\n## Decisions\n\n_No decisions recorded yet._\n", nil
	}
```

Also update the function's doc comment: replace `// returns "" when dir holds no ADRs (so callers
produce no file).` with `// returns a placeholder index when dir holds no ADRs (ADR-0020).`

- [ ] **Task 2.2: `internal/adr/adr_test.go`: flip the zero-ADR assertion.** Replace the whole
  `TestRenderActiveMDEmptyWhenNoADRs` function (keep the `// invariant: render-active-md` marker
  line directly above it) with:

```go
// invariant: render-active-md
func TestRenderActiveMDPlaceholderWhenNoADRs(t *testing.T) {
	dir := t.TempDir()
	// A non-ADR markdown file must not count.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := adr.RenderActiveMD(dir)
	if err != nil {
		t.Fatalf("RenderActiveMD: %v", err)
	}
	if !strings.Contains(got, "No decisions recorded yet") {
		t.Errorf("expected placeholder index for an ADR-less dir, got:\n%s", got)
	}
}
```

- [ ] **Task 2.3: `internal/project/project.go`: drop the `generateActiveMD` bool.** Replace the
  whole function (lines around `func (p *Project) generateActiveMD()`), comment and body, with:

```go
// generateActiveMD renders the ADR index for the project's decisions directory.
// It always produces a file: a populated index when ADRs exist, else a placeholder
// (ADR-0020 Decision 6: partial-item supersedence of ADR-0005/ADR-0006).
func (p *Project) generateActiveMD() (RenderedFile, error) {
	dir := filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")
	content, err := adr.RenderActiveMD(dir)
	if err != nil {
		return RenderedFile{}, err
	}
	return RenderedFile{Path: strings.TrimRight(p.Cfg.DocsDir, "/") + "/decisions/ACTIVE.md", Content: content}, nil
}
```

- [ ] **Task 2.4: `internal/project/project.go`: update the `PlannedOutputs` caller.** Replace:

```go
	if amd, ok, err := p.generateActiveMD(); err != nil {
		return nil, err
	} else if ok {
		paths = append(paths, amd.Path)
	}
```

with:

```go
	amd, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	paths = append(paths, amd.Path)
```

(The `coverage-ignore` on the following `generateDomainDocs` error check stays valid: its
rationale, that `generateActiveMD` parses the same decisions dir and fails first, is unchanged.)

- [ ] **Task 2.5: `internal/project/project.go`: update the `Sync` caller.** Replace:

```go
	if amd, ok, err := p.generateActiveMD(); err != nil {
		return err
	} else if ok {
		files = append(files, amd)
	}
	dds, err := p.generateDomainDocs()
```

with:

```go
	amd, err := p.generateActiveMD()
	if err != nil {
		return err
	}
	files = append(files, amd)
	dds, err := p.generateDomainDocs()
```

- [ ] **Task 2.6: `internal/project/project.go`: update `Check` and remove the orphaned-ACTIVE
  branch.** Replace:

```go
	amd, ok, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	if ok {
		onDisk, err := os.ReadFile(filepath.Join(p.Root, activeMdRel))
		if err != nil {
			drift = append(drift, manifest.Drift{Path: activeMdRel, Kind: "missing", Detail: "ADR index absent; run awf sync"})
		} else if manifest.Hash(onDisk) != manifest.Hash([]byte(amd.Content)) {
			drift = append(drift, manifest.Drift{Path: activeMdRel, Kind: "stale", Detail: "ADR index out of date; run awf sync"})
		}
	} else if _, locked := lock.Files[activeMdRel]; locked {
		drift = append(drift, manifest.Drift{Path: activeMdRel, Kind: "orphaned", Detail: "no ADRs remain; run awf sync"})
	}
```

with:

```go
	amd, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	if onDisk, rerr := os.ReadFile(filepath.Join(p.Root, activeMdRel)); rerr != nil {
		drift = append(drift, manifest.Drift{Path: activeMdRel, Kind: "missing", Detail: "ADR index absent; run awf sync"})
	} else if manifest.Hash(onDisk) != manifest.Hash([]byte(amd.Content)) {
		drift = append(drift, manifest.Drift{Path: activeMdRel, Kind: "stale", Detail: "ADR index out of date; run awf sync"})
	}
```

- [ ] **Task 2.7: `internal/project/coverage_test.go`: delete the obsolete orphaned-ACTIVE test.**
  ACTIVE.md is now always produced, so it is never orphaned-to-absent. Delete the entire
  `func TestCheckReportsOrphanedActiveMDWhenADRsRemoved(t *testing.T) { ... }` function. (The
  `missing` and `stale` branches stay covered by `TestCheckReportsMissingActiveMD` and
  `TestSyncGeneratesActiveMDAndCheckDetectsStaleness`.)

- [ ] **Task 2.8: `internal/project/project_test.go`: flip the zero-ADR sync test.** Replace the
  whole `TestSyncProducesNoActiveMDWithoutADRs` function with the following. It carries
  `// invariant: sync-generates-active-md` so the test asserting the *changed* (zero-ADR) clause of
  that invariant holds its marker, per ADR-0020 Decision 6 (the with-ADRs test
  `TestSyncGeneratesActiveMDAndCheckDetectsStaleness` keeps its markers too):

```go
// invariant: sync-generates-active-md
func TestSyncRendersPlaceholderActiveMDWithoutADRs(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "decisions", "ACTIVE.md"))
	if err != nil {
		t.Fatalf("expected a placeholder ACTIVE.md when no ADRs exist: %v", err)
	}
	if !strings.Contains(string(got), "No decisions recorded yet") {
		t.Errorf("expected placeholder index, got:\n%s", got)
	}
	if drift, err := p.Check(); err != nil || len(drift) != 0 {
		t.Errorf("expected clean check with no ADRs, got drift=%#v err=%v", drift, err)
	}
}
```

- [ ] **Task 2.9: Verify and sync.** Run:

```
go test ./internal/adr/ ./internal/project/
./x sync && ./x check
```

Expected: tests `ok`; `awf sync: done`; `awf check: clean`. `git status --short` shows **no**
changed managed files (this repo has ADRs, so `ACTIVE.md` is byte-identical).

- [ ] **Task 2.10: Gate and commit.** Run `./x gate` (expect `coverage: 100.0%`, `0 issues.`),
  then:

```
git add internal/adr/adr.go internal/adr/adr_test.go internal/project/project.go internal/project/project_test.go internal/project/coverage_test.go
git commit -m "feat(awf): always render ACTIVE.md, even with zero ADRs"
```

---

## Phase 3: `plans-readme` managed singleton

ADR-0020 Decision 7. Mirrors the ADR-0021 `adr-readme` wiring exactly.

- [ ] **Task 3.1: Create `templates/plans-readme/README.md.tmpl`** with exactly this content
  (note: no markdown links, only backtick paths, so the singleton can never itself introduce a
  dead reference regardless of sync ordering):

````
# Implementation Plans

<!-- awf:section intro -->
A plan is the step-by-step execution record for a complex change: bite-sized tasks, exact file
paths, exact content or diffs, and the commands that verify each step. The design rationale lives in
the linked ADR (when one exists); a plan links to it rather than restating it.
<!-- awf:end -->

<!-- awf:section naming -->
## Naming & location

Plans live in `{{ .layout.plansDir }}/` and follow this pattern:

```
YYYY-MM-DD-kebab-topic.md
```

where the date is the day the plan is written (ISO-8601). Example:
`2026-01-15-extract-refs-package.md`.
<!-- awf:end -->

<!-- awf:section structure -->
## What a plan contains

- A header: goal, architecture summary, tech stack, and the file structure (created / modified /
  deleted).
- Phases of bite-sized tasks (~2-5 min each) as `- [ ]` checkboxes, each naming exact paths, the
  exact content or diff, and the exact verifying command with its expected output.
- A commit step at the end of each phase.

A plan is mutable while its ADR is `Proposed` (or while non-ADR work is in flight) and freezes once
the ADR is `Accepted`/`Implemented`. Plans stay in the repository permanently as the historical
record of how a change rolled out.
<!-- awf:end -->
````

- [ ] **Task 3.2: `templates/embed.go`: embed the new directory.** Replace:

```go
//go:embed catalog.yaml skills hooks agents agents-doc docs domains claude adr-readme adr-template
```

with:

```go
//go:embed catalog.yaml skills hooks agents agents-doc docs domains claude adr-readme adr-template plans-readme
```

- [ ] **Task 3.3: `templates/catalog.yaml`: declare the sections.** Directly after the
  `adrTemplate:` block (the `sections: [frontmatter, body]` one) and before `domainDoc:`, insert:

```yaml
plansReadme:
  sections:
    - intro
    - naming
    - structure
```

- [ ] **Task 3.4: `internal/catalog/catalog.go`: add the field.** In the `Catalog` struct, after
  the `AdrTemplate SkillSpec ...` line, add:

```go
	PlansReadme SkillSpec            `yaml:"plansReadme"`
```

- [ ] **Task 3.5: `internal/config/config.go`: register the singleton kind.** In `IsSingletonKind`,
  replace:

```go
	case "agents-doc", "adr-readme", "adr-template":
```

with:

```go
	case "agents-doc", "adr-readme", "adr-template", "plans-readme":
```

- [ ] **Task 3.6: `internal/project/project.go`: layout path.** In `layout()`'s returned map, after
  the `"plansDir":   d + "/plans",` entry, add:

```go
		"plansReadme": d + "/plans/README.md",
```

- [ ] **Task 3.7: `internal/project/project.go`: render the singleton.** In `RenderAll`'s singleton
  struct-slice (the one with the `adr-readme` and `adr-template` entries), add a third entry after
  the `adr-template` line:

```go
		{"plans-readme", "plans-readme/README.md.tmpl", lay["plansReadme"].(string), p.Cat.PlansReadme.Sections},
```

- [ ] **Task 3.8: `internal/project/project.go`: validate its section overrides.** In the
  section-override validation loop (the struct-slice with `{"adr-readme", p.Cat.AdrReadme.Sections}`
  and `{"adr-template", p.Cat.AdrTemplate.Sections}`), add after the `adr-template` entry:

```go
		{"plans-readme", p.Cat.PlansReadme.Sections},
```

- [ ] **Task 3.9: `internal/project/golden_test.go`: assert the singleton renders.** In
  `TestEndToEndGolden`, after the existing agent assertion block (the
  `code-reviewer.md` read), add:

```go
	plansReadme, err := os.ReadFile(filepath.Join(root, "docs/plans/README.md"))
	if err != nil {
		t.Fatalf("plans-readme not rendered: %v", err)
	}
	if !strings.Contains(string(plansReadme), "Implementation Plans") {
		t.Errorf("plans-readme not interpolated:\n%s", plansReadme)
	}
```

- [ ] **Task 3.10: `internal/project/docs_sections_test.go`: extend singleton section-parity.**
  The `plans-readme` singleton joins the parity guarantee (`inv: adr-singleton-section-parity`,
  ADR-0021), so a renamed/dropped marker in its template fails loudly instead of silently dropping a
  section. In `TestAdrSingletonSectionParity`: (1) add `"plansDir": "docs/plans"` to the `lay` map
  (the template cites `.layout.plansDir`, so without it the test's own `<no value>` check fires);
  and (2) add this row to the struct-slice after the `adr-template` entry:

```go
		{"plans-readme/README.md.tmpl", cat.PlansReadme.Sections},
```

- [ ] **Task 3.11: Sync and verify.** Run:

```
./x sync && ./x check
go test ./internal/...
```

Expected: `awf sync: done`; `awf check: clean`; tests `ok`. `git status --short` shows the new
`docs/plans/README.md` and a changed `.awf/awf.lock`.

- [ ] **Task 3.12: Gate and commit.** Run `./x gate` (expect `coverage: 100.0%`, `0 issues.`),
  then:

```
git add templates/plans-readme/README.md.tmpl templates/embed.go templates/catalog.yaml internal/catalog/catalog.go internal/config/config.go internal/project/project.go internal/project/golden_test.go internal/project/docs_sections_test.go docs/plans/README.md .awf/awf.lock
git commit -m "feat(awf): scaffold plans-readme managed singleton"
```

---

## Phase 4: Dead-reference scan + `workflow.md` link fix

ADR-0020 Decisions 1 (integration), 2, 3, 5. `workflow.md` is the **only** dead link in awf's
managed docs (verified), so fixing it in this commit keeps the gate green on introduction.

- [ ] **Task 4.1: `internal/project/project.go`: import `refs`.** In the internal-imports block,
  add between the `migrate` and `render` lines:

```go
	"github.com/hypnotox/agentic-workflows/internal/refs"
```

- [ ] **Task 4.2: `internal/project/project.go`: add the `isManagedMarkdown` helper.** Place it
  directly above `func (p *Project) Check(`:

```go
// isManagedMarkdown reports whether a RenderAll template id is awf-managed rendered
// markdown subject to the dead-reference scan (ADR-0020 Decision 3): everything
// RenderAll produces except the CLAUDE.md bridge and the .githooks scripts.
func isManagedMarkdown(tid string) bool {
	return tid != "claude/CLAUDE.md.tmpl" && !strings.HasPrefix(tid, "hooks/")
}
```

- [ ] **Task 4.3: `internal/project/project.go`: add the scan to `Check`.** Immediately before the
  final `return drift, nil` of `Check` (after the domain-docs orphan loop), insert:

```go
	// Dead-reference scan (inv: dead-reference-gated). Every awf-managed rendered
	// markdown file's inline links must resolve file-relative on disk; the generated
	// ACTIVE.md and domain docs are in scope, the CLAUDE.md bridge and hooks are not.
	scan := make([]RenderedFile, 0, len(files)+1+len(dds))
	for _, f := range files {
		if isManagedMarkdown(f.TemplateID) {
			scan = append(scan, f)
		}
	}
	scan = append(scan, amd)
	scan = append(scan, dds...)
	for _, f := range scan {
		base := filepath.Dir(f.Path)
		for _, target := range refs.Links(f.Content) {
			resolved := filepath.Join(p.Root, base, target)
			if _, err := os.Stat(resolved); err != nil {
				drift = append(drift, manifest.Drift{Path: f.Path, Kind: "dead-reference", Detail: target})
			}
		}
	}
```

> **Note (test interaction):** after this scan lands, any test that deletes `ACTIVE.md` after sync
> (e.g. `TestCheckReportsMissingActiveMD`) accrues an *additional* `dead-reference` drift, because
> `AGENTS.md`'s document map links to `docs/decisions/ACTIVE.md` (now absent), on top of the
> `missing` drift. Those tests assert drift *membership* (`found`), not a count, so they still pass
> unchanged: no edit needed.

- [ ] **Task 4.4: `templates/docs/workflow.md.tmpl`: make the ADR-README link `docsDir`-relative.**
  `workflow.md` renders directly under `docsDir`, and `.layout.adrReadme` is the root-relative
  `<docsDir>/decisions/README.md`; the fixed `docsDir`-relative segment is `decisions/README.md`.
  Replace:

```
see [`{{ .layout.adrReadme }}`]({{ .layout.adrReadme }}).
```

with:

```
see [`{{ .layout.adrReadme }}`](decisions/README.md).
```

- [ ] **Task 4.5: `internal/project/coverage_test.go`: add the dead-reference test.** Append this
  function (the `// invariant: dead-reference-gated` marker backs ADR-0020's invariant; it is inert
  until the ADR is `Implemented` in Phase 5, then required-and-satisfied). It injects a dead link
  via an `agents-doc` section-part override so `AGENTS.md` renders with a broken relative link:

```go
// invariant: dead-reference-gated
func TestCheckDetectsDeadReference(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\nhooks: []\n", map[string]string{
		"parts/agents-doc/identity.md": "See [missing](no/such/file.md).\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range drift {
		if d.Kind == "dead-reference" && d.Detail == "no/such/file.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dead-reference drift, got %#v", drift)
	}
}
```

- [ ] **Task 4.6: Sync and verify the gate is green on introduction.** Run:

```
go test ./internal/project/
./x sync && ./x check
```

Expected: tests `ok`; `awf sync: done`; `awf check: clean` (the `workflow.md` fix re-renders
`docs/workflow.md` and resolves the only dead link). `git status --short` shows changed
`docs/workflow.md` and `.awf/awf.lock`.

- [ ] **Task 4.7: Gate and commit.** Run `./x gate` (expect `coverage: 100.0%`, `0 issues.`),
  then:

```
git add internal/project/project.go internal/project/coverage_test.go templates/docs/workflow.md.tmpl docs/workflow.md .awf/awf.lock
git commit -m "feat(awf): gate dead internal references in managed docs"
```

---

## Phase 5: Domain narratives + ADR-0020 → Implemented

Final commit of the implementation range: refresh the two domain narratives ADR-0020 shifts and flip
the status (which regenerates `ACTIVE.md` and enforces the `dead-reference-gated` invariant backing
landed in Phase 4).

- [ ] **Task 5.1: `.awf/domains/parts/rendering/current-state.md`: append the check.** At the end
  of the single paragraph (after `... each suppressible with a `local: true` sidecar (ADR-0021).`),
  append this sentence (same paragraph, one space before it):

```
`awf check` also scans every managed rendered markdown file for inline links whose file-relative target is missing on disk and fails on a `dead-reference` drift; the link extractor is the pure `internal/refs` package (ADR-0020).
```

- [ ] **Task 5.2: `.awf/domains/parts/adr-system/current-state.md`: reflect always-render + the
  plans singleton.** Replace the final sentence:

```
The ADR guide (`README.md`) and skeleton (`template.md`) are awf-managed singletons (ADR-0021), so the `.layout.adrReadme` / `.layout.adrTemplate` references resolve in any adopter project.
```

with:

```
The ADR guide (`README.md`), the skeleton (`template.md`), and a plan-authoring guide (`plans/README.md`) are awf-managed singletons (ADR-0021, ADR-0020), so the `.layout.adrReadme` / `.layout.adrTemplate` / `.layout.plansReadme` references resolve in any adopter project. `ACTIVE.md` always renders (a placeholder index when no ADRs exist), so its document-map link resolves out of the box (ADR-0020, partial-item supersedence of ADR-0005/ADR-0006).
```

- [ ] **Task 5.3: `.awf/agents-doc.yaml`: add the dead-reference gate to the AGENTS.md Invariants
  list.** The other deterministic gates each carry a headline bullet here (drift oracle, backed
  invariants, 100% coverage), so dead-reference gating gets one too, making the expectation visible
  instead of buried. Append this entry to the `data.invariants` list (after the `ADR-0012` 100%
  coverage entry, keeping the ascending-ADR ordering):

```yaml
        - ref: ADR-0020
          text: '**No dead internal links.** `awf check` fails on any inline markdown link in an awf-managed rendered doc whose file-relative target is missing on disk.'
```

- [ ] **Task 5.4: `docs/decisions/0020-dead-reference-check.md`: flip status.** In the frontmatter,
  change `status: Accepted` to `status: Implemented`. (Per the lifecycle rule, `status` is the only
  in-place edit on a non-Proposed ADR; the body stays frozen.)

- [ ] **Task 5.5: Sync and verify.** Run:

```
./x sync && ./x check
```

Expected: `awf sync: done`; `awf check: clean`. `git status --short` shows changed
`docs/domains/rendering.md`, `docs/domains/adr-system.md`, `docs/decisions/ACTIVE.md` (0020 moves to
the Implemented group), `AGENTS.md` (the new invariant bullet; `CLAUDE.md` is the static `@AGENTS.md`
bridge and does **not** change), `.awf/awf.lock`, plus the four edited source files.

- [ ] **Task 5.6: Confirm the invariant is backed.** Run:

```
./x invariants
```

Expected: `awf invariants: clean` (the `dead-reference-gated` slug on now-Implemented ADR-0020 is
backed by `TestCheckDetectsDeadReference`).

- [ ] **Task 5.7: Gate and commit.** Run `./x gate full` (the full tier for the terminal commit;
  expect `coverage: 100.0%`, `0 issues.`), then:

```
git add docs/decisions/0020-dead-reference-check.md .awf/agents-doc.yaml .awf/domains/parts/rendering/current-state.md .awf/domains/parts/adr-system/current-state.md AGENTS.md docs/domains/rendering.md docs/domains/adr-system.md docs/decisions/ACTIVE.md .awf/awf.lock
git commit -m "feat(awf): mark ADR-0020 Implemented; refresh domain narratives"
```

---

## Verification checklist (end of implementation)

- [ ] `./x gate full` passes (100% coverage, 0 lint issues).
- [ ] `./x check` is clean.
- [ ] `awf invariants` reports the `dead-reference-gated` slug backed.
- [ ] `docs/decisions/ACTIVE.md` lists ADR-0020 under **Implemented**.
- [ ] `docs/plans/README.md` exists and renders the plan-authoring guide.
- [ ] `docs/workflow.md`'s ADR-README link is `decisions/README.md` (resolves).
- [ ] Terminal step: invoke `awf-reviewing-impl` over the implementation commits.
