# Plan: Family-aligned AGENTS.md template + opt-in docs module

Design & rationale: [ADR-0004](../decisions/0004-agents-md-template-and-docs-module.md). This
plan is the execution record only — do not duplicate rationale; link to the ADR.

## Goal

Reshape `templates/agents-doc/AGENTS.md.tmpl` to the internal reference projects six-section family shape
(config-driven via `agentsDoc.data` + guarded fallbacks), and add an opt-in, managed `docs:`
module (`map[string]config.SkillConfig`, rendered + lock-tracked like skills) whose generated docs
auto-link from the new Document-map section. Dogfood both on awf.

## Architecture summary

- `docs:` is a new top-level `awf.yaml` key of shape `map[string]config.SkillConfig` (per-doc
  `data` + section overlays), validated against a new catalog `docs:` block (`DocSpec` =
  `title`/`desc`/`sections`). Each declared doc renders `templates/docs/<name>.md.tmpl` →
  `docs/<name>.md`, lock-tracked and drift-checked. A project diverges by overriding a doc's
  sections (`replaceWith` a part), never by hand-editing the rendered file.
- The agents-doc render call gains a `.docs` data key (resolved from the declared docs + catalog
  `title`/`desc`) so the Document-map section auto-links them.
- The AGENTS.md template becomes six `awf:section` blocks rendered from `.data`/`​.vars`/`.docs`
  with `missingkey=zero`-safe guarded fallbacks.

## Tech stack

- Go 1.26. Packages touched: `internal/config`, `internal/catalog`, `internal/project`,
  `templates` (embed + template tree). No new external dependencies. No lock-format change.

## File structure

Created:
- `templates/docs/architecture.md.tmpl`, `workflow.md.tmpl`, `testing.md.tmpl`,
  `development.md.tmpl`, `debugging.md.tmpl`, `pitfalls.md.tmpl`, `glossary.md.tmpl`,
  `roadmap.md.tmpl`
- `.claude/awf/parts/doc-architecture.md` (Phase 2, dogfood)

Modified:
- `internal/config/config.go`, `internal/catalog/catalog.go`, `internal/project/project.go`,
  `templates/embed.go`, `templates/catalog.yaml`, `templates/agents-doc/AGENTS.md.tmpl`,
  `.claude/awf.yaml`
- Tests: `internal/config/config_test.go`, `internal/project/spine_test.go`,
  `internal/project/project_test.go`

Deleted (Phase 2):
- `.claude/awf/parts/agents-doc-overview.md`, `.claude/awf/parts/agents-doc-layout.md`,
  `.claude/awf/parts/agents-doc-conventions.md`

---

## Phase 1 — `docs:` module plumbing (additive, single commit)

Everything here is additive: awf's `.claude/awf.yaml` gains no `docs:` key, so no `docs/*` files
render and `AGENTS.md` output is unchanged. Gate stays green throughout.

### Task 1.1 — Add `Docs` to the config schema

- [ ] In `internal/config/config.go`, add a `Docs` field to the `Config` struct. Change:

```go
type Config struct {
	Prefix    string                 `yaml:"prefix"`
	Vars      map[string]any         `yaml:"vars"`
	Skills    map[string]SkillConfig `yaml:"skills"`
	Agents    map[string]SkillConfig `yaml:"agents"`
	Hooks     []string               `yaml:"hooks"`
	AgentsDoc *SkillConfig           `yaml:"agentsDoc"`
	raw       []byte
}
```

to:

```go
type Config struct {
	Prefix    string                 `yaml:"prefix"`
	Vars      map[string]any         `yaml:"vars"`
	Skills    map[string]SkillConfig `yaml:"skills"`
	Agents    map[string]SkillConfig `yaml:"agents"`
	Hooks     []string               `yaml:"hooks"`
	AgentsDoc *SkillConfig           `yaml:"agentsDoc"`
	Docs      map[string]SkillConfig `yaml:"docs"`
	raw       []byte
}
```

- [ ] In the same file, in `Validate()`, add a docs section-override check. After the
  `for name, ac := range c.Agents { ... }` loop and before the `if c.AgentsDoc != nil` block, insert:

```go
	for name, dc := range c.Docs {
		if err := checkSections("doc", name, dc); err != nil {
			return err
		}
	}
```

### Task 1.2 — Add `DocSpec` + `Docs` to the catalog

- [ ] In `internal/catalog/catalog.go`, add the `DocSpec` type after the `SkillSpec` type:

```go
type DocSpec struct {
	Title    string   `yaml:"title"`
	Desc     string   `yaml:"desc"`
	Sections []string `yaml:"sections"`
}
```

- [ ] Add a `Docs` field to the `Catalog` struct:

```go
type Catalog struct {
	Skills    map[string]SkillSpec `yaml:"skills"`
	Agents    map[string]SkillSpec `yaml:"agents"`
	Hooks     []string             `yaml:"hooks"`
	AgentsDoc SkillSpec            `yaml:"agentsDoc"`
	Docs      map[string]DocSpec   `yaml:"docs"`
}
```

### Task 1.3 — Create the eight doc templates

Each doc template is a single overridable `body` section with a guided fallback. Create these
eight files verbatim.

- [ ] `templates/docs/architecture.md.tmpl`:

```
# Architecture

<!-- awf:section body -->
<!-- Describe the system shape: top-level packages and what each owns, the data/control flow, and the key dependencies. Override via docs.architecture.sections.body (replaceWith a part) in .claude/awf.yaml. -->
<!-- awf:end -->
```

- [ ] `templates/docs/workflow.md.tmpl`:

```
# Workflow

<!-- awf:section body -->
<!-- Principles, the brainstorm/plan/ADR chain, commit discipline, and doc-currency rules. Override via docs.workflow.sections.body in .claude/awf.yaml. -->
<!-- awf:end -->
```

- [ ] `templates/docs/testing.md.tmpl`:

```
# Testing

<!-- awf:section body -->
<!-- Gate tiers, test layout, and what each tier covers. Override via docs.testing.sections.body in .claude/awf.yaml. -->
<!-- awf:end -->
```

- [ ] `templates/docs/development.md.tmpl`:

```
# Development

<!-- awf:section body -->
<!-- Local setup, the command runner, and the dependency reference. Override via docs.development.sections.body in .claude/awf.yaml. -->
<!-- awf:end -->
```

- [ ] `templates/docs/debugging.md.tmpl`:

```
# Debugging

<!-- awf:section body -->
<!-- Recipes for common failure modes, log flags, and inspection steps. Override via docs.debugging.sections.body in .claude/awf.yaml. -->
<!-- awf:end -->
```

- [ ] `templates/docs/pitfalls.md.tmpl`:

```
# Pitfalls

<!-- awf:section body -->
<!-- Recurring bugs and tricky areas to watch. Override via docs.pitfalls.sections.body in .claude/awf.yaml. -->
<!-- awf:end -->
```

- [ ] `templates/docs/glossary.md.tmpl`:

```
# Glossary

<!-- awf:section body -->
<!-- Project jargon and term ownership; start here when a term is unfamiliar. Override via docs.glossary.sections.body in .claude/awf.yaml. -->
<!-- awf:end -->
```

- [ ] `templates/docs/roadmap.md.tmpl`:

```
# Roadmap

<!-- awf:section body -->
<!-- Uncommitted ideas and future phases. Override via docs.roadmap.sections.body in .claude/awf.yaml. -->
<!-- awf:end -->
```

### Task 1.4 — Embed the docs template directory

- [ ] In `templates/embed.go`, change the embed directive line:

```go
//go:embed catalog.yaml skills hooks agents agents-doc
```

to:

```go
//go:embed catalog.yaml skills hooks agents agents-doc docs
```

(The eight files from Task 1.3 must already exist, or this fails to compile.)

### Task 1.5 — Declare the docs in the catalog

- [ ] In `templates/catalog.yaml`, add a top-level `docs:` block. Insert it immediately before the
  existing `agentsDoc:` block (keep `agentsDoc:` last). Verbatim:

```yaml
docs:
  architecture:
    title: Architecture
    desc: system shape, packages, key components, dependencies
    sections: [body]
  workflow:
    title: Workflow
    desc: principles, the brainstorm/plan/ADR chain, commit discipline
    sections: [body]
  testing:
    title: Testing
    desc: gate tiers, test layout, what each tier covers
    sections: [body]
  development:
    title: Development
    desc: local setup, the command runner, dependency reference
    sections: [body]
  debugging:
    title: Debugging
    desc: recipes for common failure modes
    sections: [body]
  pitfalls:
    title: Pitfalls
    desc: recurring bugs and tricky areas
    sections: [body]
  glossary:
    title: Glossary
    desc: project jargon and term ownership
    sections: [body]
  roadmap:
    title: Roadmap
    desc: uncommitted ideas and future phases
    sections: [body]
```

### Task 1.6 — Wire the render loop, validation, and `.docs` injection

- [ ] In `internal/project/project.go`, in `validateAgainstCatalog()`, add a docs check. Insert it
  immediately before the final `return nil`:

```go
	// Check docs against catalog.
	for _, name := range sortedKeys(p.Cfg.Docs) {
		dc := p.Cfg.Docs[name]
		if dc.Local {
			continue
		}
		spec, ok := p.Cat.Docs[name]
		if !ok {
			return fmt.Errorf("doc %q is not in the catalog", name)
		}
		if err := checkSectionsAllowed("doc", name, spec.Sections, dc.Sections); err != nil {
			return err
		}
	}
```

- [ ] In `RenderAll()`, add a docs render loop and inject `.docs` into the agents-doc data. Replace
  the existing `// AgentsDoc.` block:

```go
	// AgentsDoc.
	if p.Cfg.AgentsDoc != nil && !p.Cfg.AgentsDoc.Local {
		rf, err := p.renderTemplate("agents-doc/AGENTS.md.tmpl", p.Cfg.AgentsDoc.Sections, p.data(*p.Cfg.AgentsDoc), "AGENTS.md")
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, nil
```

with:

```go
	// Docs.
	for _, name := range sortedKeys(p.Cfg.Docs) {
		dc := p.Cfg.Docs[name]
		if dc.Local {
			continue
		}
		tid := fmt.Sprintf("docs/%s.md.tmpl", name)
		rf, err := p.renderTemplate(tid, dc.Sections, p.data(dc), "docs/"+name+".md")
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	// AgentsDoc.
	if p.Cfg.AgentsDoc != nil && !p.Cfg.AgentsDoc.Local {
		data := p.data(*p.Cfg.AgentsDoc)
		data["docs"] = p.resolvedDocs()
		rf, err := p.renderTemplate("agents-doc/AGENTS.md.tmpl", p.Cfg.AgentsDoc.Sections, data, "AGENTS.md")
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, nil
```

- [ ] Add the `resolvedDocs` helper. Insert it immediately after the `data` method (the one
  returning `map[string]any`):

```go
// resolvedDocs builds the Document-map entries for the agents-doc template from
// the docs declared in config, annotated with the catalog's title/desc.
func (p *Project) resolvedDocs() []map[string]any {
	out := []map[string]any{}
	for _, name := range sortedKeys(p.Cfg.Docs) {
		if p.Cfg.Docs[name].Local {
			continue
		}
		spec := p.Cat.Docs[name]
		out = append(out, map[string]any{
			"name":  name,
			"title": spec.Title,
			"desc":  spec.Desc,
			"path":  "docs/" + name + ".md",
		})
	}
	return out
}
```

### Task 1.7 — Tests for the docs module

- [ ] In `internal/config/config_test.go`, add a test that `docs:` parses and validates:

```go
func TestLoadParsesDocsMap(t *testing.T) {
	p := writeTemp(t, `prefix: example
docs:
  architecture:
    sections:
      body: {replaceWith: parts/doc-architecture.md}
`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Docs["architecture"].Sections["body"].ReplaceWith != "parts/doc-architecture.md" {
		t.Errorf("docs.architecture.body replaceWith = %q", c.Docs["architecture"].Sections["body"].ReplaceWith)
	}
}

func TestValidateRejectsDropAndReplaceOnDoc(t *testing.T) {
	p := writeTemp(t, `prefix: example
docs:
  architecture:
    sections:
      body: {drop: true, replaceWith: parts/x.md}
`)
	c, _ := Load(p)
	if err := c.Validate(); err == nil {
		t.Errorf("expected error: doc section cannot both drop and replaceWith")
	}
}
```

- [ ] In `internal/project/spine_test.go`, add a golden render test for a doc template:

```go
func TestDocArchitectureTemplate(t *testing.T) {
	out := renderGolden(t, "docs/architecture.md.tmpl", map[string]any{
		"prefix": "example",
		"vars":   map[string]any{},
		"data":   map[string]any{},
	})
	if !strings.Contains(out, "# Architecture") {
		t.Errorf("expected '# Architecture' heading:\n%s", out)
	}
}
```

- [ ] In `internal/project/project_test.go`, add an unknown-doc rejection test (mirrors
  `TestOpenRejectsUnknownHook`):

```go
func TestOpenRejectsUnknownDoc(t *testing.T) {
	root := scaffold(t, `prefix: example
skills: {}
agents: {}
hooks: []
docs:
  nonexistent: {}
`)
	_, err := Open(root)
	if err == nil {
		t.Errorf("expected error for doc 'nonexistent' not in catalog")
	}
}

func TestSyncRendersDeclaredDoc(t *testing.T) {
	root := scaffold(t, `prefix: example
skills: {}
agents: {}
hooks: []
docs:
  architecture: {}
`)
	p, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "docs", "architecture.md"))
	if err != nil {
		t.Fatalf("docs/architecture.md not written: %v", err)
	}
	if !strings.Contains(string(b), "# Architecture") {
		t.Errorf("docs/architecture.md missing heading:\n%s", b)
	}
}
```

### Task 1.8 — Verify and commit Phase 1

- [ ] Run `./x gate`. Expected: `0 issues.` and all packages `ok`.
- [ ] Run `./x check`. Expected: `awf check: clean` (awf's own `awf.yaml` has no `docs:` key, so
  `AGENTS.md` and the lock are unchanged).
- [ ] Stage and commit:

```
git add internal/config internal/catalog internal/project templates/embed.go templates/catalog.yaml templates/docs
git commit -m "feat(awf): add opt-in managed docs module"
```

Commit body should note: new `docs: map[string]SkillConfig` schema key, catalog `DocSpec`,
render loop + validation, eight doc templates, `.docs` injection into the agents-doc render.
Reference ADR-0004.

---

## Phase 2 — AGENTS.md reshape + awf dogfood (atomic, single commit)

The catalog `agentsDoc.sections` redefinition invalidates awf's current
`agentsDoc.sections` overlay (`overview`/`layout`/`conventions`), which fails
`checkSectionsAllowed` → `./x check` → the pre-commit gate. Therefore the template rewrite, the
catalog section redefinition, the `.claude/awf.yaml` rewrite, the part migration, and the re-sync
**must all land in one commit**.

### Task 2.1 — Rewrite the AGENTS.md template

- [ ] Replace the entire contents of `templates/agents-doc/AGENTS.md.tmpl` with:

```
# {{ .prefix }} — Agent Guide

This document is the authoritative reference for AI agents working in the `{{ .prefix }}`
repository. Read it before taking any action; keep it current as decisions evolve.

<!-- awf:section you-and-this-project -->
## You and this project

{{ with .data.ownership }}{{ . }}{{ else }}You are a developer on `{{ .prefix }}`, responsible for its long-term health as well as the task in front of you. Bugs you notice in passing are yours; coverage gaps are yours; documentation drift is yours to fix in the same commit that caused it.{{ end }}
<!-- awf:end -->

<!-- awf:section identity -->
## Identity

{{ with .data.identity }}{{ . }}{{ else }}<!-- One dense paragraph: what `{{ .prefix }}` is, its stack and module path, its maturity, and who it serves. Set `agentsDoc.data.identity` in `.claude/awf.yaml`. -->{{ end }}
<!-- awf:end -->

<!-- awf:section invariants -->
## Invariants

Hard rules every change must respect:

- **Append-only ADRs.** Decision rationale lives under `{{ with .vars.adrDir }}{{ . }}{{ else }}docs/decisions{{ end }}/`{{ with .vars.activeMdPath }}; `{{ . }}` is generated — never hand-edited{{ end }}.
- **Docs travel with the change.** Reality and its documentation update in the same commit.
- **Green gate before every commit.** {{ with .vars.gateCmd }}`{{ . }}` must pass before any commit lands.{{ else }}The gate must pass before any commit lands.{{ end }}
{{- range .data.invariants }}
- {{ .text }}{{ with .ref }} ({{ . }}){{ end }}
{{- end }}
<!-- awf:end -->

<!-- awf:section workflow -->
## Workflow

Canonical chain for non-trivial work:

```
brainstorming → planning (if warranted) → ADR (if warranted) → review → implementation → review
```

Brainstorming is the hard prerequisite. **Planning** is warranted by *complexity* (multi-commit, interdependent steps); an **ADR** is warranted by *load-bearing-ness* (a design decision the project must remember). Many tasks need neither. Reviews are lightweight: the grounding-check inside `{{ .prefix }}-brainstorming` subsumes plan/ADR review, and `{{ .prefix }}-reviewing-impl` is the single terminal review.

**Chain skills** (invoke in order): `{{ .prefix }}-brainstorming`, `{{ .prefix }}-writing-plans`, `{{ .prefix }}-proposing-adr`, `{{ .prefix }}-executing-plans` / `{{ .prefix }}-subagent-driven-development`, `{{ .prefix }}-reviewing-impl`. **Task skills** (as needed): `{{ .prefix }}-tdd`, `{{ .prefix }}-bugfix`, `{{ .prefix }}-debugging`, `{{ .prefix }}-adr-lifecycle`.

{{ with .vars.gateCmd }}Run `{{ . }}` before every commit{{ with $.vars.gateCmdFull }}; `{{ . }}` is the full tier{{ end }}. {{ end }}Conventional Commits; one concern per commit.{{ with .vars.workflowDoc }} Full rules: [{{ . }}]({{ . }}).{{ end }}
<!-- awf:end -->

<!-- awf:section commands -->
## Commands

{{ if .data.commands -}}
```
{{ range .data.commands }}{{ .cmd }} — {{ .desc }}
{{ end -}}
```
{{- else -}}
{{ with .vars.testCmd }}- `{{ . }}` — run the test suite
{{ end }}{{ with .vars.gateCmd }}- `{{ . }}` — run the gate before committing
{{ end }}{{ with .vars.checkCmd }}- `{{ . }}` — check rendered files for drift
{{ end }}{{- end }}
<!-- awf:end -->

<!-- awf:section document-map -->
## Document map

{{ with .vars.adrReadme }}- **ADR index:** [{{ . }}]({{ . }}) — architecture decisions and lifecycle.
{{ end }}{{- with .vars.activeMdPath }}- **Active ADRs:** [{{ . }}]({{ . }}) — generated status index; do not hand-edit.
{{ end }}{{- with .vars.plansDir }}- **Plans:** [{{ . }}]({{ . }}) — implementation plans for complex work.
{{ end }}{{- range .docs }}- **{{ .title }}:** [{{ .path }}]({{ .path }}) — {{ .desc }}
{{ end }}{{- range .data.docMap }}- **{{ .path }}:** [{{ .path }}]({{ .path }}){{ with .desc }} — {{ . }}{{ end }}
{{ end -}}
<!-- awf:end -->
```

### Task 2.2 — Redefine the catalog agentsDoc sections

- [ ] In `templates/catalog.yaml`, replace the existing `agentsDoc:` block:

```yaml
agentsDoc:
  sections:
    - overview
    - build-test
    - workflow-chain
    - layout
    - conventions
```

with:

```yaml
agentsDoc:
  sections:
    - you-and-this-project
    - identity
    - invariants
    - workflow
    - commands
    - document-map
```

### Task 2.3 — Author awf's architecture part (migrate Repository Layout)

- [ ] Create `.claude/awf/parts/doc-architecture.md` with awf's layout content (migrated from the
  soon-deleted `agents-doc-layout.md`), verbatim:

```
awf ties a per-project `.claude/awf.yaml` config to an embedded template catalog, renders the
standard's skills/agents/hooks/docs/agent-guide, and drift-checks them against a lock file.

Key directories:

- **`cmd/awf/`** — CLI entry point; `init`, `sync`, `check`, `list`, `add`, `setup` subcommands.
- **`internal/config/`** — parses and validates `.claude/awf.yaml`; owns the config schema.
- **`internal/catalog/`** — reads `templates/catalog.yaml`; declares available skills/agents/hooks/docs/sections.
- **`internal/render/`** — Go `text/template` rendering with `missingkey=zero`; applies `data`, `sections` (drop / replaceWith), and per-template part injection.
- **`internal/manifest/`** — writes and reads `.claude/awf.lock`; drives drift detection for `awf check`.
- **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and `Check()`; golden tests live here.
- **`internal/adrtools/`** — regenerates `docs/decisions/ACTIVE.md` from ADR frontmatter; run via `go test ./internal/adrtools/`.
- **`templates/`** — embedded skill, agent, hook, doc, and agents-doc templates; catalog lives at `templates/catalog.yaml`.
- **`docs/decisions/`** — ADRs; `ACTIVE.md` is auto-generated; `README.md` is the human index.
- **`docs/plans/`** — implementation plans written by `awf-writing-plans`.
- **`.claude/skills/awf-*/`**, **`.claude/agents/`**, **`.githooks/`** — rendered artifacts (committed; do not hand-edit; re-sync from config and parts).
```

### Task 2.4 — Rewrite awf's agentsDoc config and enable the architecture doc

- [ ] In `.claude/awf.yaml`, replace the existing `agentsDoc:` block:

```yaml
agentsDoc:
  sections:
    overview:
      replaceWith: parts/agents-doc-overview.md
    layout:
      replaceWith: parts/agents-doc-layout.md
    conventions:
      replaceWith: parts/agents-doc-conventions.md
```

with the config-driven block plus an enabled, overridden architecture doc:

```yaml
agentsDoc:
  data:
    ownership: |
      You are a developer on `awf` — the Agentic Workflows CLI and standard. You are responsible for its long-term health as well as the task in front of you. Bugs you notice in passing are yours; coverage gaps are yours; documentation drift is yours to fix in the same commit that caused it. awf is both the tool that publishes the standard and the first adopter of it, so its own setup must model what it generates.
    identity: |
      `awf` scaffolds, renders, and drift-checks a suite of Claude Code skills, review agents, git hooks, docs, and this agent guide into any Go project from a single `.claude/awf.yaml` config file. The full workflow chain is expressed as project-owned skill files under `.claude/skills/awf-*/` and review agents under `.claude/agents/`; hooks under `.githooks/` enforce the gate. Module path `agentic-workflows`; Go 1.26; private, pre-1.0, no external API stability.
    invariants:
      - text: "**Publication-safe templates.** Every template renders with `missingkey=zero`; never emit a `<no value>` token for an empty var — wrap optional output in a conditional. Run `awf check` after any sync to verify."
        ref: ADR-0001
      - text: "**`awf check` is the drift oracle.** After editing `.claude/awf.yaml` or any part, run `./x sync && ./x check`. Commit rendered files alongside config changes; never hand-edit a rendered file."
      - text: "**Conventional Commits, `awf` scope.** One concern per commit; stage files explicitly (no `git add -A`)."
    docMap: []
  docs:
    architecture:
      sections:
        body:
          replaceWith: parts/doc-architecture.md
```

### Task 2.5 — Delete the obsolete agents-doc parts

- [ ] Delete the three parts no longer referenced by `.claude/awf.yaml`:

```
git rm .claude/awf/parts/agents-doc-overview.md .claude/awf/parts/agents-doc-layout.md .claude/awf/parts/agents-doc-conventions.md
```

### Task 2.6 — Golden tests for the reshaped template

- [ ] In `internal/project/spine_test.go`, replace the body of `TestAgentsDocTemplate` with
  assertions covering the data-absent fallback path, the no-`reviewing-plan-resync` invariant, and
  the empty-string-var safety:

```go
func TestAgentsDocTemplate(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"testCmd":  "go test ./...",
			"gateCmd":  "make gate",
			"adrDir":   "docs/decisions",
			"plansDir": "docs/plans",
		},
		"data": map[string]any{},
	}
	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	for _, phrase := range []string{
		"## You and this project",
		"## Identity",
		"## Invariants",
		"## Workflow",
		"## Commands",
		"## Document map",
		"example-brainstorming",
		"example-reviewing-impl",
		"make gate",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	if strings.Contains(out, "reviewing-plan-resync") {
		t.Errorf("Workflow must not present reviewing-plan-resync as a primary step:\n%s", out)
	}
}
```

- [ ] In `internal/project/spine_test.go`, add a test for the config-driven path + docs auto-link +
  empty-string-var safety:

```go
func TestAgentsDocTemplateConfigDriven(t *testing.T) {
	data := map[string]any{
		"prefix": "example",
		"vars": map[string]any{
			"adrReadme":    "",
			"activeMdPath": "",
			"gateCmd":      "",
		},
		"data": map[string]any{
			"identity":  "Example is a widget.",
			"ownership": "You own example.",
			"invariants": []map[string]any{
				{"text": "**Custom rule.**", "ref": "ADR-0009"},
			},
		},
		"docs": []map[string]any{
			{"title": "Architecture", "desc": "system shape", "path": "docs/architecture.md"},
		},
	}
	out := renderGolden(t, "agents-doc/AGENTS.md.tmpl", data)
	for _, phrase := range []string{
		"Example is a widget.",
		"You own example.",
		"**Custom rule.** (ADR-0009)",
		"[docs/architecture.md](docs/architecture.md)",
	} {
		if !strings.Contains(out, phrase) {
			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
		}
	}
	if strings.Contains(out, "]()") {
		t.Errorf("empty-string vars must not render empty-target links:\n%s", out)
	}
}
```

(`renderGolden` already fails on any `<no value>` leak via `assertNoLeaks`, covering the
no-`<no value>` invariant.)

### Task 2.7 — Sync, verify, and commit Phase 2

- [ ] Run `./x sync`. Expected: regenerates `AGENTS.md` (new six-section shape) and writes
  `docs/architecture.md` from the architecture template + `parts/doc-architecture.md` override.
- [ ] Open `AGENTS.md` and confirm by eye: six `##` headings (You and this project, Identity,
  Invariants, Workflow, Commands, Document map); no `## What this project is NOT`; no
  `## Repository Layout`; the Document map links `docs/architecture.md`.
- [ ] Run `./x check`. Expected: `awf check: clean`.
- [ ] Run `./x gate`. Expected: `0 issues.` and all packages `ok`.
- [ ] Stage and commit everything atomically:

```
git add templates/agents-doc/AGENTS.md.tmpl templates/catalog.yaml .claude/awf.yaml .claude/awf/parts .claude/awf.lock AGENTS.md docs/architecture.md internal/project/spine_test.go
git commit -m "feat(awf): reshape AGENTS.md to family shape; dogfood docs module"
```

Commit body: reshape to the six-section family shape (config-driven via `agentsDoc.data`), enable
the managed architecture doc, migrate Repository Layout into `docs/architecture.md`. Reference
ADR-0004.

---

## Done criteria

- `./x gate` and `./x check` both clean after each phase's commit.
- awf's `AGENTS.md` matches the six-section family shape; `docs/architecture.md` carries the
  migrated layout; the Document map links it.
- A fresh `awf init` still emits neither `agentsDoc:` nor `docs:` (both opt-in) — unchanged
  `ScaffoldConfig`.
- ADR-0004 status flip to `Implemented` happens in the final commit of the implementation
  sequence (handled by the execution skill, not this plan).
