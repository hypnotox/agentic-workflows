# Mandatory Singleton Standard Docs

Implements [ADR-0043](../decisions/0043-mandatory-singleton-status-for-workflow-and-documentation-standards.md).
Design rationale lives there; this plan is the execution record only.

## Goal

Promote `workflow`, `doc-standard`, and `agents-md-standard` from toggleable `docs:` catalog
entries to always-on singletons (joining `adr-readme`/`adr-template`/`plans-readme`), consolidating
the six-member plain-singleton family onto one `internal/catalog`-sourced identity list, one
`internal/project` table, and the existing `renderKind` render primitive — fixing the dead-reference
bug on `agents-md-standard.md`'s hardcoded link and the Document Map regression a naive removal would
cause, and porting this repo's own `.awf/` tree in the same change since it currently enables all
three as plural docs.

## Architecture summary

- `internal/catalog`: `AdrReadme`/`AdrTemplate`/`PlansReadme` named fields are replaced by one
  `Singletons map[string]TargetSpec` (six entries), loaded from a new `singletons:` map in
  `templates/catalog.yaml`. A new `SingletonKinds` package var is the compile-time name list (seven
  entries, including `agents-doc`) that `internal/config.IsSingletonKind` consults directly — no
  `*Catalog` instance needed for that check. `DocSpec.Core` is deleted (no doc entry sets it once the
  three promoted docs leave `docs:`).
- `internal/config.IsSingletonKind` becomes a `slices.Contains(catalog.SingletonKinds, kind)` check.
- `internal/project` gains `singleton.go`'s `plainSingletons` table (the six non-`agents-doc`
  singletons: kind, template id, an `outPath func(Layout) string`, a
  `sections func(*catalog.Catalog) []string`). `render.go`'s hand-rolled 3-tuple loop is replaced by
  one `renderKind` call per table entry (reusing the primitive already used for docs/skills/agents);
  `validate.go`'s separately hand-duplicated 3-tuple list is replaced by a loop over the same table.
  `layout.go` gains `DocStandard`/`AgentsMdStandard` fields and drops `WorkflowRef`'s AGENTS.md
  fallback (retiring `inv: workflow-ref-fallback`, ADR-0013). `scaffold.go` stops seeding a
  core-doc-derived `docs:` array (none remain) and extends its var-collection to the six
  `plainSingletons` templates (a coverage gap the old `cat.Docs`-only scan would otherwise open).
- `internal/migrate` gains a `{To: 6, Name: "singleton-standard-docs"}` migration relocating each
  promoted doc's sidecar/convention-part paths from the plural shape to the singleton shape, then
  stripping it from any `docs:` array, mirroring `portAgentsDoc`'s precedent.
- Three templates change: `templates/agents-doc/AGENTS.md.tmpl`'s Document Map section gains three
  hardcoded static lines (matching its existing ADR-index/Active-ADRs/Plans lines);
  `templates/docs/agents-md-standard.md.tmpl`'s hardcoded `doc-standard.md` link becomes
  `{{ .layout.docStandard }}`; `templates/docs/doc-standard.md.tmpl` gains one new Rules bullet.
- This repo's own `.awf/config.yaml` (which currently enables all three as plural docs) and
  `.awf/docs/parts/workflow/local-hooks.md` are fixed by hand in the same commit that lands the
  catalog schema change — the schema change alone would otherwise make `validateAgainstCatalog`
  reject this repo's own config the instant it lands, so the cutover cannot be split across commits.

## Tech stack

Go 1.26 (module `github.com/hypnotox/agentic-workflows`). Packages touched: `internal/catalog`,
`internal/config`, `internal/project`, `internal/migrate`. Templates touched:
`templates/catalog.yaml`, `templates/agents-doc/AGENTS.md.tmpl`,
`templates/docs/agents-md-standard.md.tmpl`, `templates/docs/doc-standard.md.tmpl`. No new
dependencies.

## File structure

- Modified: `internal/catalog/catalog.go`
- Modified: `templates/catalog.yaml`
- Modified: `internal/config/config.go`
- Created: `internal/project/singleton.go`, `internal/project/singleton_test.go`
- Modified: `internal/project/render.go`, `internal/project/validate.go`, `internal/project/layout.go`,
  `internal/project/scaffold.go`
- Modified: `internal/project/project_test.go`, `internal/project/docs_sections_test.go`,
  `internal/project/scaffold_test.go`
- Modified: `cmd/awf/list_add_test.go`
- Modified: `.awf/config.yaml`
- Relocated: `.awf/docs/parts/workflow/local-hooks.md` → `.awf/parts/workflow/local-hooks.md`
- Created: `internal/migrate/singletonstandarddocs.go`, `internal/migrate/singletonstandarddocs_test.go`
- Modified: `internal/migrate/migrate.go`, `internal/migrate/migrate_test.go`
- Modified: `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/agents-md-standard.md.tmpl`,
  `templates/docs/doc-standard.md.tmpl`
- Modified: `.awf/docs/parts/architecture/components.md`, `.awf/domains/parts/rendering/current-state.md`,
  `.awf/domains/parts/config/current-state.md`, `.awf/domains/parts/tooling/current-state.md`
- Modified (rendered, via `./x sync`): `AGENTS.md`, `docs/architecture.md`, `docs/doc-standard.md`,
  `docs/agents-md-standard.md`, `docs/workflow.md`, `docs/domains/rendering.md`, `docs/domains/config.md`,
  `docs/domains/tooling.md`, `docs/decisions/ACTIVE.md`, `.awf/awf.lock`
- Modified: `docs/decisions/0043-mandatory-singleton-status-for-workflow-and-documentation-standards.md`
  (frontmatter `status` only)

## Phase 1 — Core mechanism cutover

This phase cannot be split across commits: the moment `templates/catalog.yaml` drops the three docs,
`validateAgainstCatalog`'s existing (unmodified) catalog-membership check rejects this repo's own
`.awf/config.yaml`, which still lists them. Every task below lands in one commit.

- [ ] **Task 1.1 — `internal/catalog/catalog.go`: add `Singletons`/`SingletonKinds`, remove the three
  named fields and `DocSpec.Core`.** Change:

  ```go
  // SkillSpec declares a skill's render sections plus its optional doc dependency:
  // a non-empty RequiresDoc gates the skill on that doc being enabled. Core marks a
  // skill as part of the workflow-core set awf init scaffolds by default (ADR-0022).
  type SkillSpec struct {
  	Sections    []string `yaml:"sections"`
  	RequiresDoc string   `yaml:"requiresDoc"`
  	Core        bool     `yaml:"core"`
  }

  // DocSpec declares a doc's catalog metadata. Core marks a doc as part of the
  // workflow-core set awf init scaffolds by default (ADR-0022).
  type DocSpec struct {
  	Title    string   `yaml:"title"`
  	Desc     string   `yaml:"desc"`
  	Sections []string `yaml:"sections"`
  	Core     bool     `yaml:"core"`
  }
  ```

  to:

  ```go
  // SkillSpec declares a skill's render sections plus its optional doc dependency:
  // a non-empty RequiresDoc gates the skill on that doc being enabled. Core marks a
  // skill as part of the workflow-core set awf init scaffolds by default (ADR-0022).
  type SkillSpec struct {
  	Sections    []string `yaml:"sections"`
  	RequiresDoc string   `yaml:"requiresDoc"`
  	Core        bool     `yaml:"core"`
  }

  // DocSpec declares a doc's catalog metadata. Docs no longer carry a Core marker
  // (ADR-0043): the three docs that used to set it are always-on singletons now,
  // outside this map entirely.
  type DocSpec struct {
  	Title    string   `yaml:"title"`
  	Desc     string   `yaml:"desc"`
  	Sections []string `yaml:"sections"`
  }

  // SingletonKinds lists every kind name that is an always-on singleton — never
  // toggled via an enable array (ADR-0004, ADR-0021, ADR-0043). It is a plain
  // compile-time list, not derived from a loaded Catalog, because
  // internal/config.IsSingletonKind needs this classification without holding a
  // *Catalog instance. internal/project tests its six non-agents-doc members
  // against both this list and Catalog.Singletons' loaded keys, so the compile-time
  // list and the YAML-driven map never drift apart silently.
  var SingletonKinds = []string{
  	"agents-doc", "adr-readme", "adr-template", "plans-readme",
  	"workflow", "doc-standard", "agents-md-standard",
  }
  ```

  Then change:

  ```go
  type Catalog struct {
  	Skills      map[string]SkillSpec  `yaml:"skills"`
  	Agents      map[string]TargetSpec `yaml:"agents"`
  	AgentsDoc   TargetSpec            `yaml:"agentsDoc"`
  	DomainDoc   TargetSpec            `yaml:"domainDoc"`
  	AdrReadme   TargetSpec            `yaml:"adrReadme"`
  	AdrTemplate TargetSpec            `yaml:"adrTemplate"`
  	PlansReadme TargetSpec            `yaml:"plansReadme"`
  	Docs        map[string]DocSpec    `yaml:"docs"`
  	Vars        []VarDescriptor       `yaml:"vars"`
  }
  ```

  to:

  ```go
  type Catalog struct {
  	Skills     map[string]SkillSpec  `yaml:"skills"`
  	Agents     map[string]TargetSpec `yaml:"agents"`
  	AgentsDoc  TargetSpec            `yaml:"agentsDoc"`
  	DomainDoc  TargetSpec            `yaml:"domainDoc"`
  	Singletons map[string]TargetSpec `yaml:"singletons"`
  	Docs       map[string]DocSpec    `yaml:"docs"`
  	Vars       []VarDescriptor       `yaml:"vars"`
  }
  ```

- [ ] **Task 1.2 — `templates/catalog.yaml`: move three docs into a new `singletons:` map.** In the
  `docs:` block, delete these three entries entirely:

  ```yaml
    workflow:
      core: true
      title: Workflow
      desc: principles, the brainstorm/ADR/plan chain, commit discipline
      sections: [principles, chain, commit-discipline, doc-currency, local-hooks]
  ```

  ```yaml
    doc-standard:
      core: true
      title: Documentation Standard
      desc: how-to-write rules for all awf-managed prose
      sections: [principles, rules, structure]
  ```

  ```yaml
    agents-md-standard:
      core: true
      title: Authoring AGENTS.md
      desc: layout, content, and rules for the agent guide
      sections: [layout, content, rules]
  ```

  (`architecture`, `testing`, `development`, `debugging`, `pitfalls`, `glossary`, `roadmap` remain,
  none `core: true`.)

  Then change:

  ```yaml
  adrReadme:
    sections:
      - intro
      - when
      - naming
      - frontmatter
      - invariants
      - active-md
  adrTemplate:
    sections:
      - frontmatter
      - body

  plansReadme:
    sections:
      - intro
      - naming
      - structure
  ```

  to:

  ```yaml
  singletons:
    adr-readme:
      sections:
        - intro
        - when
        - naming
        - frontmatter
        - invariants
        - active-md
    adr-template:
      sections:
        - frontmatter
        - body
    plans-readme:
      sections:
        - intro
        - naming
        - structure
    workflow:
      sections:
        - principles
        - chain
        - commit-discipline
        - doc-currency
        - local-hooks
    doc-standard:
      sections:
        - principles
        - rules
        - structure
    agents-md-standard:
      sections:
        - layout
        - content
        - rules
  ```

- [ ] **Task 1.3 — `internal/config/config.go`: `IsSingletonKind` becomes catalog-driven.** Change the
  import block from:

  ```go
  import (
  	"bytes"
  	"errors"
  	"fmt"
  	"os"
  	"path/filepath"
  	"strings"

  	"gopkg.in/yaml.v3"
  )
  ```

  to:

  ```go
  import (
  	"bytes"
  	"errors"
  	"fmt"
  	"os"
  	"path/filepath"
  	"slices"
  	"strings"

  	"github.com/hypnotox/agentic-workflows/internal/catalog"
  	"gopkg.in/yaml.v3"
  )
  ```

  Then change:

  ```go
  // IsSingletonKind reports whether kind is an always-on singleton whose sidecar lives at
  // <root>/<kind>.yaml and whose parts live under <root>/parts/<kind>/ (ADR-0021).
  func IsSingletonKind(kind string) bool {
  	switch kind {
  	case "agents-doc", "adr-readme", "adr-template", "plans-readme":
  		return true
  	}
  	return false
  }
  ```

  to:

  ```go
  // IsSingletonKind reports whether kind is an always-on singleton whose sidecar lives at
  // <root>/<kind>.yaml and whose parts live under <root>/parts/<kind>/ (ADR-0021, ADR-0043).
  func IsSingletonKind(kind string) bool {
  	return slices.Contains(catalog.SingletonKinds, kind)
  }
  ```

- [ ] **Task 1.4 — Create `internal/project/singleton.go`.**

  ```go
  package project

  import "github.com/hypnotox/agentic-workflows/internal/catalog"

  // singletonSpec is one plain (neutral, non-agents-doc) always-on singleton's
  // render/validate identity: a kind name, its embedded template id, and
  // accessors for its fixed output path and catalog sections. plainSingletons is
  // the single source of truth both RenderAll (via renderKind) and
  // validateAgainstCatalog range over — adding a 7th plain singleton means
  // appending one entry here, not hand-editing two separate loops (ADR-0043).
  type singletonSpec struct {
  	kind     string
  	tid      string
  	outPath  func(Layout) string
  	sections func(*catalog.Catalog) []string
  }

  var plainSingletons = []singletonSpec{
  	{
  		kind: "adr-readme", tid: "adr-readme/README.md.tmpl",
  		outPath:  func(l Layout) string { return l.AdrReadme },
  		sections: func(c *catalog.Catalog) []string { return c.Singletons["adr-readme"].Sections },
  	},
  	{
  		kind: "adr-template", tid: "adr-template/template.md.tmpl",
  		outPath:  func(l Layout) string { return l.AdrTemplate },
  		sections: func(c *catalog.Catalog) []string { return c.Singletons["adr-template"].Sections },
  	},
  	{
  		kind: "plans-readme", tid: "plans-readme/README.md.tmpl",
  		outPath:  func(l Layout) string { return l.PlansReadme },
  		sections: func(c *catalog.Catalog) []string { return c.Singletons["plans-readme"].Sections },
  	},
  	{
  		kind: "workflow", tid: "docs/workflow.md.tmpl",
  		outPath:  func(l Layout) string { return l.WorkflowRef },
  		sections: func(c *catalog.Catalog) []string { return c.Singletons["workflow"].Sections },
  	},
  	{
  		kind: "doc-standard", tid: "docs/doc-standard.md.tmpl",
  		outPath:  func(l Layout) string { return l.DocStandard },
  		sections: func(c *catalog.Catalog) []string { return c.Singletons["doc-standard"].Sections },
  	},
  	{
  		kind: "agents-md-standard", tid: "docs/agents-md-standard.md.tmpl",
  		outPath:  func(l Layout) string { return l.AgentsMdStandard },
  		sections: func(c *catalog.Catalog) []string { return c.Singletons["agents-md-standard"].Sections },
  	},
  }
  ```

- [ ] **Task 1.5 — `internal/project/render.go`: replace the hand-rolled singleton loop.** Change:

  ```go
  	// adr-readme + adr-template + plans-readme (always-on singletons unless local; ADR-0021, ADR-0020).
  	lay := p.layout()
  	for _, sg := range []struct {
  		kind, tid, out string
  		sections       []string
  	}{
  		{"adr-readme", "adr-readme/README.md.tmpl", lay.AdrReadme, p.Cat.AdrReadme.Sections},
  		{"adr-template", "adr-template/template.md.tmpl", lay.AdrTemplate, p.Cat.AdrTemplate.Sections},
  		{"plans-readme", "plans-readme/README.md.tmpl", lay.PlansReadme, p.Cat.PlansReadme.Sections},
  	} {
  		sc, err := p.Cfg.Sidecar(sg.kind, "")
  		if err != nil {
  			return nil, err
  		}
  		if sc.Local {
  			continue
  		}
  		rf, err := p.renderTarget(sg.kind, "", sg.tid, sg.sections, sc, p.data(sc), sg.out)
  		if err != nil {
  			return nil, err
  		}
  		out = append(out, rf)
  	}
  ```

  to:

  ```go
  	// Plain singletons: adr-readme, adr-template, plans-readme, workflow,
  	// doc-standard, agents-md-standard (always-on unless local; ADR-0021, ADR-0043).
  	lay := p.layout()
  	for _, sg := range plainSingletons {
  		rfs, err := p.renderKind(renderKindSpec{
  			kind: sg.kind, names: []string{""},
  			tid:      func(string) string { return sg.tid },
  			sections: func(string) []string { return sg.sections(p.Cat) },
  			outPath:  func(Target, string) string { return sg.outPath(lay) },
  		})
  		if err != nil {
  			return nil, err
  		}
  		out = append(out, rfs...)
  	}
  ```

- [ ] **Task 1.6 — `internal/project/validate.go`: replace the separately hand-duplicated list.**
  Change:

  ```go
  	for _, sg := range []struct {
  		kind     string
  		sections []string
  	}{
  		{"adr-readme", p.Cat.AdrReadme.Sections},
  		{"adr-template", p.Cat.AdrTemplate.Sections},
  		{"plans-readme", p.Cat.PlansReadme.Sections},
  	} {
  		sc, err := p.Cfg.Sidecar(sg.kind, "")
  		if err != nil {
  			return err
  		}
  		if !sc.Local {
  			if err := checkSectionsAllowed(sg.kind, "", sg.sections, sc.Sections); err != nil {
  				return err
  			}
  		}
  	}
  	return nil
  }
  ```

  to:

  ```go
  	for _, sg := range plainSingletons {
  		sc, err := p.Cfg.Sidecar(sg.kind, "")
  		if err != nil {
  			return err
  		}
  		if !sc.Local {
  			if err := checkSectionsAllowed(sg.kind, "", sg.sections(p.Cat), sc.Sections); err != nil {
  				return err
  			}
  		}
  	}
  	return nil
  }
  ```

- [ ] **Task 1.7 — `internal/project/layout.go`: add `DocStandard`/`AgentsMdStandard`, drop the
  `WorkflowRef` fallback.** Change:

  ```go
  type Layout struct {
  	DocsDir     string
  	ADRDir      string
  	ActiveMd    string
  	AdrReadme   string
  	AdrTemplate string
  	PlansDir    string
  	PlansReadme string
  	Docs        map[string]string // name -> output path; present iff enabled (inv: layout-docs-enabled-only)
  	WorkflowRef string
  	DomainsDir  string
  }

  func (p *Project) layout() Layout {
  	d := strings.TrimRight(p.Cfg.DocsDir, "/")
  	dec := d + "/decisions"
  	// Docs maps every enabled doc name to its output path. Local docs are included:
  	// the file still exists at that path and is citable.
  	docs := map[string]string{}
  	for _, name := range p.Cfg.Docs {
  		docs[name] = p.docOutPath(name)
  	}
  	// WorkflowRef is the workflow doc's path when enabled, else AGENTS.md, so the
  	// ~always-cited workflow reference always resolves (inv: workflow-ref-fallback).
  	workflowRef := "AGENTS.md"
  	if wp, ok := docs["workflow"]; ok {
  		workflowRef = wp
  	}
  	return Layout{
  		DocsDir:     d,
  		ADRDir:      dec,
  		ActiveMd:    dec + "/ACTIVE.md",
  		AdrReadme:   dec + "/README.md",
  		AdrTemplate: dec + "/template.md",
  		PlansDir:    d + "/plans",
  		PlansReadme: d + "/plans/README.md",
  		Docs:        docs,
  		WorkflowRef: workflowRef,
  		DomainsDir:  d + "/domains", // inv: domains-dir-given
  	}
  }
  ```

  to:

  ```go
  type Layout struct {
  	DocsDir          string
  	ADRDir           string
  	ActiveMd         string
  	AdrReadme        string
  	AdrTemplate      string
  	PlansDir         string
  	PlansReadme      string
  	Docs             map[string]string // name -> output path; present iff enabled (inv: layout-docs-enabled-only)
  	WorkflowRef      string
  	DocStandard      string
  	AgentsMdStandard string
  	DomainsDir       string
  }

  func (p *Project) layout() Layout {
  	d := strings.TrimRight(p.Cfg.DocsDir, "/")
  	dec := d + "/decisions"
  	// Docs maps every enabled doc name to its output path. Local docs are included:
  	// the file still exists at that path and is citable.
  	docs := map[string]string{}
  	for _, name := range p.Cfg.Docs {
  		docs[name] = p.docOutPath(name)
  	}
  	return Layout{
  		DocsDir:          d,
  		ADRDir:           dec,
  		ActiveMd:         dec + "/ACTIVE.md",
  		AdrReadme:        dec + "/README.md",
  		AdrTemplate:      dec + "/template.md",
  		PlansDir:         d + "/plans",
  		PlansReadme:      d + "/plans/README.md",
  		Docs:             docs,
  		WorkflowRef:      d + "/workflow.md",
  		DocStandard:      d + "/doc-standard.md",
  		AgentsMdStandard: d + "/agents-md-standard.md",
  		DomainsDir:       d + "/domains", // inv: domains-dir-given
  	}
  }
  ```

  Then change `templateMap`:

  ```go
  func (l Layout) templateMap() map[string]any {
  	docs := map[string]any{}
  	for k, v := range l.Docs {
  		docs[k] = v
  	}
  	return map[string]any{
  		"docsDir":     l.DocsDir,
  		"adrDir":      l.ADRDir,
  		"activeMd":    l.ActiveMd,
  		"adrReadme":   l.AdrReadme,
  		"adrTemplate": l.AdrTemplate,
  		"plansDir":    l.PlansDir,
  		"plansReadme": l.PlansReadme,
  		"docs":        docs,
  		"workflowRef": l.WorkflowRef,
  		"domainsDir":  l.DomainsDir,
  	}
  }
  ```

  to:

  ```go
  func (l Layout) templateMap() map[string]any {
  	docs := map[string]any{}
  	for k, v := range l.Docs {
  		docs[k] = v
  	}
  	return map[string]any{
  		"docsDir":          l.DocsDir,
  		"adrDir":           l.ADRDir,
  		"activeMd":         l.ActiveMd,
  		"adrReadme":        l.AdrReadme,
  		"adrTemplate":      l.AdrTemplate,
  		"plansDir":         l.PlansDir,
  		"plansReadme":      l.PlansReadme,
  		"docs":             docs,
  		"workflowRef":      l.WorkflowRef,
  		"docStandard":      l.DocStandard,
  		"agentsMdStandard": l.AgentsMdStandard,
  		"domainsDir":       l.DomainsDir,
  	}
  }
  ```

- [ ] **Task 1.8 — `internal/project/scaffold.go`: drop core-doc seeding, scan `plainSingletons` for
  vars.** Change:

  ```go
  	for name := range cat.Docs {
  		path := fmt.Sprintf("docs/%s.md.tmpl", name)
  		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog doc name has a backing template in the embedded FS, so collectVars cannot fail
  			return nil, err
  		}
  	}
  	varNames := slices.Sorted(maps.Keys(varSet))

  	// Enable the core skills and core docs; agents are all enabled (every
  	// one is workflow-essential).
  	// invariant: scaffold-core-only
  	var skillNames, docNames []string
  	for name, spec := range cat.Skills {
  		if spec.Core {
  			skillNames = append(skillNames, name)
  		}
  	}
  	for name, spec := range cat.Docs {
  		if spec.Core {
  			docNames = append(docNames, name)
  		}
  	}
  	// A non-nil trim dimension (ADR-0029 full-deselectable catalog trim) replaces the
  	// curated-core default verbatim; nil keeps exactly the core (scaffold-core-only).
  	// invariant: catalog-trim-applied
  	if trim != nil && trim.Skills != nil {
  		skillNames = slices.Clone(*trim.Skills)
  	}
  	if trim != nil && trim.Docs != nil {
  		docNames = slices.Clone(*trim.Docs)
  	}
  	slices.Sort(skillNames)
  	slices.Sort(docNames)
  ```

  to:

  ```go
  	for name := range cat.Docs {
  		path := fmt.Sprintf("docs/%s.md.tmpl", name)
  		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog doc name has a backing template in the embedded FS, so collectVars cannot fail
  			return nil, err
  		}
  	}
  	// Plain singletons (workflow, doc-standard, agents-md-standard included) always
  	// render — their vars must be seeded even though they left cat.Docs (ADR-0043).
  	for _, sg := range plainSingletons {
  		if err := collectVars(templates.FS, sg.tid, varSet); err != nil { // coverage-ignore: every plainSingletons entry has a backing template in the embedded FS, so collectVars cannot fail
  			return nil, err
  		}
  	}
  	varNames := slices.Sorted(maps.Keys(varSet))

  	// Enable the core skills; agents are all enabled (every one is
  	// workflow-essential). No core docs remain — workflow/doc-standard/
  	// agents-md-standard are mandatory singletons (ADR-0043), not toggleable.
  	// invariant: scaffold-core-only
  	var skillNames, docNames []string
  	for name, spec := range cat.Skills {
  		if spec.Core {
  			skillNames = append(skillNames, name)
  		}
  	}
  	// A non-nil trim dimension (ADR-0029 full-deselectable catalog trim) replaces the
  	// curated-core default verbatim; nil keeps exactly the core (scaffold-core-only).
  	// invariant: catalog-trim-applied
  	if trim != nil && trim.Skills != nil {
  		skillNames = slices.Clone(*trim.Skills)
  	}
  	if trim != nil && trim.Docs != nil {
  		docNames = slices.Clone(*trim.Docs)
  	}
  	slices.Sort(skillNames)
  	slices.Sort(docNames)
  ```

- [ ] **Task 1.9 — Fix `internal/project/project_test.go`'s `TestLayoutDerivesFromDocsDir`.** Change:

  ```go
  func TestLayoutDerivesFromDocsDir(t *testing.T) {
  	p := &Project{Cfg: &config.Config{DocsDir: "documentation", Docs: []string{"architecture", "workflow"}}}
  	l := p.layout()
  	if l.DocsDir != "documentation" || l.ADRDir != "documentation/decisions" ||
  		l.ActiveMd != "documentation/decisions/ACTIVE.md" || l.AdrReadme != "documentation/decisions/README.md" ||
  		l.AdrTemplate != "documentation/decisions/template.md" || l.PlansDir != "documentation/plans" ||
  		l.PlansReadme != "documentation/plans/README.md" {
  		t.Errorf("layout = %+v", l)
  	}
  	// invariant: domains-dir-given
  	if l.DomainsDir != "documentation/domains" {
  		t.Errorf("domainsDir = %q", l.DomainsDir)
  	}
  	// invariant: workflow-ref-fallback (enabled arm)
  	if l.WorkflowRef != "documentation/workflow.md" {
  		t.Errorf("workflowRef = %q", l.WorkflowRef)
  	}
  	// invariant: layout-docs-enabled-only
  	wantDocs := map[string]string{
  		"architecture": "documentation/architecture.md",
  		"workflow":     "documentation/workflow.md",
  	}
  	if !reflect.DeepEqual(l.Docs, wantDocs) {
  		t.Errorf("Docs = %v, want %v", l.Docs, wantDocs)
  	}
  	// templateMap reproduces the historical .layout map (ConfigHash stability).
  	tm := l.templateMap()
  	for _, k := range []string{"docsDir", "adrDir", "activeMd", "adrReadme", "adrTemplate",
  		"plansDir", "plansReadme", "docs", "workflowRef", "domainsDir"} {
  		if _, ok := tm[k]; !ok {
  			t.Errorf("templateMap missing key %q", k)
  		}
  	}
  	// invariant: workflow-ref-fallback (fallback arm) — without the workflow doc enabled,
  	// workflowRef resolves to the always-present AGENTS.md.
  	noWf := &Project{Cfg: &config.Config{DocsDir: "documentation", Docs: []string{"architecture"}}}
  	if got := noWf.layout().WorkflowRef; got != "AGENTS.md" {
  		t.Errorf("workflowRef fallback = %v, want AGENTS.md", got)
  	}
  	if got := p.docOutPath("architecture"); got != "documentation/architecture.md" {
  		t.Errorf("docOutPath = %q", got)
  	}
  }
  ```

  to:

  ```go
  func TestLayoutDerivesFromDocsDir(t *testing.T) {
  	p := &Project{Cfg: &config.Config{DocsDir: "documentation", Docs: []string{"architecture"}}}
  	l := p.layout()
  	if l.DocsDir != "documentation" || l.ADRDir != "documentation/decisions" ||
  		l.ActiveMd != "documentation/decisions/ACTIVE.md" || l.AdrReadme != "documentation/decisions/README.md" ||
  		l.AdrTemplate != "documentation/decisions/template.md" || l.PlansDir != "documentation/plans" ||
  		l.PlansReadme != "documentation/plans/README.md" {
  		t.Errorf("layout = %+v", l)
  	}
  	// invariant: domains-dir-given
  	if l.DomainsDir != "documentation/domains" {
  		t.Errorf("domainsDir = %q", l.DomainsDir)
  	}
  	// workflow/doc-standard/agents-md-standard are mandatory singletons (ADR-0043):
  	// their paths are always fixed, never fall back or depend on Cfg.Docs.
  	if l.WorkflowRef != "documentation/workflow.md" {
  		t.Errorf("workflowRef = %q", l.WorkflowRef)
  	}
  	if l.DocStandard != "documentation/doc-standard.md" {
  		t.Errorf("docStandard = %q", l.DocStandard)
  	}
  	if l.AgentsMdStandard != "documentation/agents-md-standard.md" {
  		t.Errorf("agentsMdStandard = %q", l.AgentsMdStandard)
  	}
  	// invariant: layout-docs-enabled-only
  	wantDocs := map[string]string{
  		"architecture": "documentation/architecture.md",
  	}
  	if !reflect.DeepEqual(l.Docs, wantDocs) {
  		t.Errorf("Docs = %v, want %v", l.Docs, wantDocs)
  	}
  	// templateMap reproduces the historical .layout map (ConfigHash stability).
  	tm := l.templateMap()
  	for _, k := range []string{"docsDir", "adrDir", "activeMd", "adrReadme", "adrTemplate",
  		"plansDir", "plansReadme", "docs", "workflowRef", "docStandard", "agentsMdStandard", "domainsDir"} {
  		if _, ok := tm[k]; !ok {
  			t.Errorf("templateMap missing key %q", k)
  		}
  	}
  	if got := p.docOutPath("architecture"); got != "documentation/architecture.md" {
  		t.Errorf("docOutPath = %q", got)
  	}
  }
  ```

- [ ] **Task 1.10 — Fix `internal/project/docs_sections_test.go`'s `TestAdrSingletonSectionParity`.**
  Change:

  ```go
  // invariant: adr-singleton-section-parity
  func TestAdrSingletonSectionParity(t *testing.T) {
  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatal(err)
  	}
  	lay := map[string]any{"adrDir": "docs/decisions", "domainsDir": "docs/domains", "plansDir": "docs/plans"}
  	for _, c := range []struct {
  		tid      string
  		sections []string
  	}{
  		{"adr-readme/README.md.tmpl", cat.AdrReadme.Sections},
  		{"adr-template/template.md.tmpl", cat.AdrTemplate.Sections},
  		{"plans-readme/README.md.tmpl", cat.PlansReadme.Sections},
  	} {
  		src, err := fs.ReadFile(templates.FS, c.tid)
  		if err != nil {
  			t.Fatalf("read %s: %v", c.tid, err)
  		}
  		var markers []string
  		for _, s := range render.ParseSections(string(src)) {
  			if s.IsSection {
  				markers = append(markers, s.Name)
  			}
  		}
  		if strings.Join(markers, ",") != strings.Join(c.sections, ",") {
  			t.Errorf("%s markers %v != catalog sections %v", c.tid, markers, c.sections)
  		}
  		asm, parts := render.Assemble(render.ParseSections(string(src)), nil)
  		out, err := render.Execute(asm, map[string]any{
  			"prefix": "awf", "vars": map[string]any{}, "layout": lay, "data": map[string]any{}}, parts, "test")
  		if err != nil {
  			t.Fatalf("render %s: %v", c.tid, err)
  		}
  		if strings.Contains(out, "<no value>") {
  			t.Errorf("%s: <no value> leaked", c.tid)
  		}
  	}
  }
  ```

  to:

  ```go
  // invariant: adr-singleton-section-parity
  func TestAdrSingletonSectionParity(t *testing.T) {
  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatal(err)
  	}
  	lay := map[string]any{
  		"docsDir": "docs", "adrDir": "docs/decisions", "domainsDir": "docs/domains",
  		"plansDir": "docs/plans", "docStandard": "docs/doc-standard.md",
  		"agentsMdStandard": "docs/agents-md-standard.md",
  	}
  	for _, sg := range plainSingletons {
  		src, err := fs.ReadFile(templates.FS, sg.tid)
  		if err != nil {
  			t.Fatalf("read %s: %v", sg.tid, err)
  		}
  		var markers []string
  		for _, s := range render.ParseSections(string(src)) {
  			if s.IsSection {
  				markers = append(markers, s.Name)
  			}
  		}
  		wantSections := sg.sections(cat)
  		if strings.Join(markers, ",") != strings.Join(wantSections, ",") {
  			t.Errorf("%s markers %v != catalog sections %v", sg.tid, markers, wantSections)
  		}
  		asm, parts := render.Assemble(render.ParseSections(string(src)), nil)
  		out, err := render.Execute(asm, map[string]any{
  			"prefix": "awf", "vars": map[string]any{}, "layout": lay, "data": map[string]any{}}, parts, "test")
  		if err != nil {
  			t.Fatalf("render %s: %v", sg.tid, err)
  		}
  		if strings.Contains(out, "<no value>") {
  			t.Errorf("%s: <no value> leaked", sg.tid)
  		}
  	}
  }
  ```

  (`plainSingletons` is package-private and lives in the same `project` package as this test, so no
  import changes are needed for it; `fs`, `templates`, `catalog`, `render`, and `strings` stay
  imported exactly as before.)

- [ ] **Task 1.11 — Fix `internal/project/scaffold_test.go`'s two `Core`-dependent tests.** Change:

  ```go
  func TestScaffoldEnablesCoreTargets(t *testing.T) {
  	b, err := ScaffoldConfig("myproj", nil, nil, nil)
  	if err != nil {
  		t.Fatalf("ScaffoldConfig: %v", err)
  	}
  	cfg, err := config.Load(writeScaffold(t, b))
  	if err != nil {
  		t.Fatalf("config.Load: %v", err)
  	}

  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatalf("catalog.Load: %v", err)
  	}

  	wantSkills := map[string]bool{}
  	for name, spec := range cat.Skills {
  		if spec.Core {
  			wantSkills[name] = true
  		}
  	}
  	if got := sliceSet(cfg.Skills); !maps.Equal(got, wantSkills) {
  		t.Errorf("scaffold skills = %v, want core set %v",
  			slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wantSkills)))
  	}

  	wantDocs := map[string]bool{}
  	for name, spec := range cat.Docs {
  		if spec.Core {
  			wantDocs[name] = true
  		}
  	}
  	if got := sliceSet(cfg.Docs); !maps.Equal(got, wantDocs) {
  		t.Errorf("scaffold docs = %v, want core set %v",
  			slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wantDocs)))
  	}

  	// Concrete negative: a known opt-in skill must not be scaffolded.
  	if slices.Contains(cfg.Skills, "tdd") {
  		t.Errorf("scaffold should not enable the opt-in skill tdd")
  	}
  }
  ```

  to:

  ```go
  func TestScaffoldEnablesCoreTargets(t *testing.T) {
  	b, err := ScaffoldConfig("myproj", nil, nil, nil)
  	if err != nil {
  		t.Fatalf("ScaffoldConfig: %v", err)
  	}
  	cfg, err := config.Load(writeScaffold(t, b))
  	if err != nil {
  		t.Fatalf("config.Load: %v", err)
  	}

  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatalf("catalog.Load: %v", err)
  	}

  	wantSkills := map[string]bool{}
  	for name, spec := range cat.Skills {
  		if spec.Core {
  			wantSkills[name] = true
  		}
  	}
  	if got := sliceSet(cfg.Skills); !maps.Equal(got, wantSkills) {
  		t.Errorf("scaffold skills = %v, want core set %v",
  			slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wantSkills)))
  	}

  	// No doc remains core (ADR-0043 promoted the only three core docs — workflow,
  	// doc-standard, agents-md-standard — to mandatory singletons outside cat.Docs).
  	if len(cfg.Docs) != 0 {
  		t.Errorf("scaffold docs = %v, want none (no core docs remain)", cfg.Docs)
  	}

  	// Concrete negative: a known opt-in skill must not be scaffolded.
  	if slices.Contains(cfg.Skills, "tdd") {
  		t.Errorf("scaffold should not enable the opt-in skill tdd")
  	}
  }
  ```

  Then change:

  ```go
  func TestScaffoldCatalogTrim(t *testing.T) {
  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatalf("catalog.Load: %v", err)
  	}
  	coreDocs := map[string]bool{}
  	for name, spec := range cat.Docs {
  		if spec.Core {
  			coreDocs[name] = true
  		}
  	}

  	// Skills selected verbatim (incl. deselecting core); Docs nil -> keep core.
  	pickSkills := []string{"tdd", "brainstorming"}
  	b, err := ScaffoldConfig("myproj", nil, nil, &config.CatalogTrim{Skills: &pickSkills})
  	if err != nil {
  		t.Fatalf("ScaffoldConfig: %v", err)
  	}
  	cfg, err := config.Load(writeScaffold(t, b))
  	if err != nil {
  		t.Fatalf("config.Load: %v", err)
  	}
  	if got := sliceSet(cfg.Skills); !maps.Equal(got, map[string]bool{"tdd": true, "brainstorming": true}) {
  		t.Errorf("trim skills = %v, want [brainstorming tdd]", slices.Sorted(maps.Keys(got)))
  	}
  	if got := sliceSet(cfg.Docs); !maps.Equal(got, coreDocs) {
  		t.Errorf("nil docs trim should keep core docs, got %v", slices.Sorted(maps.Keys(got)))
  	}

  	// Docs deselected to empty; Skills nil -> keep core skills.
  	emptyDocs := []string{}
  	coreSkills := map[string]bool{}
  	for name, spec := range cat.Skills {
  		if spec.Core {
  			coreSkills[name] = true
  		}
  	}
  	b2, err := ScaffoldConfig("myproj", nil, nil, &config.CatalogTrim{Docs: &emptyDocs})
  	if err != nil {
  		t.Fatalf("ScaffoldConfig: %v", err)
  	}
  	cfg2, err := config.Load(writeScaffold(t, b2))
  	if err != nil {
  		t.Fatalf("config.Load: %v", err)
  	}
  	if len(cfg2.Docs) != 0 {
  		t.Errorf("empty docs trim should enable no docs, got %v", cfg2.Docs)
  	}
  	if got := sliceSet(cfg2.Skills); !maps.Equal(got, coreSkills) {
  		t.Errorf("nil skills trim should keep core skills, got %v", slices.Sorted(maps.Keys(got)))
  	}
  }
  ```

  to:

  ```go
  func TestScaffoldCatalogTrim(t *testing.T) {
  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatalf("catalog.Load: %v", err)
  	}

  	// Skills selected verbatim (incl. deselecting core); Docs nil -> no core docs to keep.
  	pickSkills := []string{"tdd", "brainstorming"}
  	b, err := ScaffoldConfig("myproj", nil, nil, &config.CatalogTrim{Skills: &pickSkills})
  	if err != nil {
  		t.Fatalf("ScaffoldConfig: %v", err)
  	}
  	cfg, err := config.Load(writeScaffold(t, b))
  	if err != nil {
  		t.Fatalf("config.Load: %v", err)
  	}
  	if got := sliceSet(cfg.Skills); !maps.Equal(got, map[string]bool{"tdd": true, "brainstorming": true}) {
  		t.Errorf("trim skills = %v, want [brainstorming tdd]", slices.Sorted(maps.Keys(got)))
  	}
  	if len(cfg.Docs) != 0 {
  		t.Errorf("nil docs trim should yield no docs (no core docs remain), got %v", cfg.Docs)
  	}

  	// Docs deselected to empty; Skills nil -> keep core skills.
  	emptyDocs := []string{}
  	coreSkills := map[string]bool{}
  	for name, spec := range cat.Skills {
  		if spec.Core {
  			coreSkills[name] = true
  		}
  	}
  	b2, err := ScaffoldConfig("myproj", nil, nil, &config.CatalogTrim{Docs: &emptyDocs})
  	if err != nil {
  		t.Fatalf("ScaffoldConfig: %v", err)
  	}
  	cfg2, err := config.Load(writeScaffold(t, b2))
  	if err != nil {
  		t.Fatalf("config.Load: %v", err)
  	}
  	if len(cfg2.Docs) != 0 {
  		t.Errorf("empty docs trim should enable no docs, got %v", cfg2.Docs)
  	}
  	if got := sliceSet(cfg2.Skills); !maps.Equal(got, coreSkills) {
  		t.Errorf("nil skills trim should keep core skills, got %v", slices.Sorted(maps.Keys(got)))
  	}
  }
  ```

  Then change `TestScaffoldVarsCoverAllReferenced` (extend the independently-derived path list to the
  six `plainSingletons` templates, closing the coverage gap left by their removal from `cat.Docs` —
  and incidentally covering `adr-readme`/`adr-template`/`plans-readme`'s templates too, which this
  test never scanned before):

  ```go
  	var paths []string
  	for name := range cat.Skills {
  		paths = append(paths, "skills/"+name+"/SKILL.md.tmpl")
  	}
  	for name := range cat.Agents {
  		paths = append(paths, "agents/"+name+".md.tmpl")
  	}
  	for name := range cat.Docs {
  		paths = append(paths, "docs/"+name+".md.tmpl")
  	}
  	for _, tmplPath := range paths {
  ```

  to:

  ```go
  	var paths []string
  	for name := range cat.Skills {
  		paths = append(paths, "skills/"+name+"/SKILL.md.tmpl")
  	}
  	for name := range cat.Agents {
  		paths = append(paths, "agents/"+name+".md.tmpl")
  	}
  	for name := range cat.Docs {
  		paths = append(paths, "docs/"+name+".md.tmpl")
  	}
  	for _, sg := range plainSingletons {
  		paths = append(paths, sg.tid)
  	}
  	for _, tmplPath := range paths {
  ```

- [ ] **Task 1.12 — Fix `cmd/awf/list_add_test.go`'s stale scaffold comment.** Change:

  ```go
  // scaffoldedProject writes a curated-default scaffold (10 core skills, 3 agents,
  // 3 docs, no domains) and syncs it.
  ```

  to:

  ```go
  // scaffoldedProject writes a curated-default scaffold (10 core skills, 3 agents,
  // 0 docs — no doc is core after ADR-0043 — no domains) and syncs it.
  ```

- [ ] **Task 1.13 — Fix this repo's own `.awf/config.yaml`.** Read the current `docs:` block and
  remove the `agents-md-standard`, `doc-standard`, and `workflow` entries (keep `architecture`,
  `development`, `glossary`, `pitfalls`, `testing` — verify the exact remaining set against the file on
  disk, since this task must not silently drop an entry this plan didn't anticipate). Use
  `go run ./cmd/awf remove doc agents-md-standard`, `go run ./cmd/awf remove doc doc-standard`, and
  `go run ./cmd/awf remove doc workflow` if the binary already reflects Tasks 1.1-1.2 at this point in
  the sequence (it will, since Task 1.1's catalog struct field rename means `awf remove doc` for these
  three now requires the catalog to no longer list them as removable-only-if-present — since this is a
  same-commit cutover, do this as a **direct hand-edit** of `.awf/config.yaml`'s `docs:` array instead
  of via the CLI, to avoid depending on an intermediate half-migrated binary state).

- [ ] **Task 1.14 — Relocate `.awf/docs/parts/workflow/local-hooks.md`.**

  ```
  mkdir -p .awf/parts/workflow
  git mv .awf/docs/parts/workflow/local-hooks.md .awf/parts/workflow/local-hooks.md
  ```

  (`doc-standard` and `agents-md-standard` have no sidecar or convention-part overrides in this repo
  today — confirmed via `ls .awf/docs/parts/` showing only `architecture`, `glossary`, `pitfalls`,
  `workflow` — so no relocation is needed for those two.)

- [ ] **Task 1.15 — Extend `TestAdrSingletonsRenderedAndSuppressible` to all six `plainSingletons`
  entries.** ADR-0043's `inv: plain-singleton-via-renderkind` requires "a table-driven test exercises
  `RenderAll` for each of the six plain singletons ... and asserts each produces its expected output
  path and content through `plainSingletons` + `renderKind`" — the existing test only covers
  `adr-readme`/`adr-template`. In `internal/project/project_test.go`, change:

  ```go
  func TestAdrSingletonsRenderedAndSuppressible(t *testing.T) {
  	root := scaffold(t, sampleYAML)
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	files, err := p.RenderAll()
  	if err != nil {
  		t.Fatal(err)
  	}
  	want := map[string]bool{"docs/decisions/README.md": false, "docs/decisions/template.md": false}
  	for _, f := range files {
  		if _, ok := want[f.Path]; ok {
  			want[f.Path] = true
  		}
  	}
  	for path, seen := range want {
  		if !seen {
  			t.Errorf("%s not rendered", path)
  		}
  	}
  	// local: true suppresses the README singleton.
  	if err := os.WriteFile(filepath.Join(root, ".awf", "adr-readme.yaml"), []byte("local: true\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	p2, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	files2, err := p2.RenderAll()
  	if err != nil {
  		t.Fatal(err)
  	}
  	for _, f := range files2 {
  		if f.Path == "docs/decisions/README.md" {
  			t.Error("README should be suppressed by local: true")
  		}
  	}
  }
  ```

  to:

  ```go
  // invariant: plain-singleton-via-renderkind
  func TestAdrSingletonsRenderedAndSuppressible(t *testing.T) {
  	root := scaffold(t, sampleYAML)
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	files, err := p.RenderAll()
  	if err != nil {
  		t.Fatal(err)
  	}
  	want := map[string]bool{
  		"docs/decisions/README.md":   false,
  		"docs/decisions/template.md": false,
  		"docs/plans/README.md":       false,
  		"docs/workflow.md":           false,
  		"docs/doc-standard.md":       false,
  		"docs/agents-md-standard.md": false,
  	}
  	for _, f := range files {
  		if _, ok := want[f.Path]; ok {
  			want[f.Path] = true
  		}
  	}
  	for path, seen := range want {
  		if !seen {
  			t.Errorf("%s not rendered", path)
  		}
  	}
  	// local: true suppresses the README singleton.
  	if err := os.WriteFile(filepath.Join(root, ".awf", "adr-readme.yaml"), []byte("local: true\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	// local: true also suppresses a newly-mandatory singleton, matching the other four (ADR-0043
  	// Decision item 1: "not togglable" keeps the local: true escape hatch).
  	if err := os.WriteFile(filepath.Join(root, ".awf", "workflow.yaml"), []byte("local: true\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	p2, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	files2, err := p2.RenderAll()
  	if err != nil {
  		t.Fatal(err)
  	}
  	for _, f := range files2 {
  		if f.Path == "docs/decisions/README.md" {
  			t.Error("README should be suppressed by local: true")
  		}
  		if f.Path == "docs/workflow.md" {
  			t.Error("workflow.md should be suppressed by local: true")
  		}
  	}
  }
  ```

- [ ] **Task 1.16 — Create `internal/project/singleton_test.go`.** Backs ADR-0043's
  `inv: singleton-kind-single-source` ("a test asserts the two sets are identical") and
  `inv: mandatory-docs-not-in-docs-catalog` ("`templates/catalog.yaml`'s `docs:` block contains no
  entry named `workflow`, `doc-standard`, or `agents-md-standard`").

  ```go
  package project

  import (
  	"slices"
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/catalog"
  	"github.com/hypnotox/agentic-workflows/templates"
  )

  // invariant: singleton-kind-single-source
  func TestPlainSingletonsMatchCatalogSingletonKinds(t *testing.T) {
  	var got []string
  	for _, sg := range plainSingletons {
  		got = append(got, sg.kind)
  	}
  	slices.Sort(got)

  	var want []string
  	for _, k := range catalog.SingletonKinds {
  		if k == "agents-doc" {
  			continue
  		}
  		want = append(want, k)
  	}
  	slices.Sort(want)

  	if !slices.Equal(got, want) {
  		t.Errorf("plainSingletons kinds = %v, want catalog.SingletonKinds minus agents-doc = %v", got, want)
  	}
  }

  // invariant: mandatory-docs-not-in-docs-catalog
  func TestCatalogDocsExcludeSingletonKinds(t *testing.T) {
  	cat, err := catalog.Load(templates.FS)
  	if err != nil {
  		t.Fatalf("catalog.Load: %v", err)
  	}
  	for name := range cat.Docs {
  		if slices.Contains(catalog.SingletonKinds, name) {
  			t.Errorf("cat.Docs contains %q, which is a singleton kind and must not be a toggleable doc", name)
  		}
  	}
  }
  ```

- [ ] **Task 1.17 — Verify and commit.**
  - Run `go build ./...`. Expect no errors.
  - Run `./x sync`. Expect `awf sync: done` — `docs/workflow.md`, `docs/doc-standard.md`, and
    `docs/agents-md-standard.md` now render via the singleton path (still at the same output paths, so
    their content should be byte-identical modulo the `local-hooks.md` relocation taking effect).
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.` — if a specific branch is uncovered
    (e.g. an unreachable `MkdirAll`/`Rename` error path introduced by this phase), add a
    `// coverage-ignore: <reason>` comment matching this codebase's existing convention rather than
    writing a test for a fault an in-process test cannot trigger.
  - Run `./x check`. Expect `awf check: clean`.
  - Stage `internal/catalog/catalog.go internal/config/config.go internal/project/singleton.go
    internal/project/singleton_test.go internal/project/render.go internal/project/validate.go
    internal/project/layout.go internal/project/scaffold.go internal/project/project_test.go
    internal/project/docs_sections_test.go internal/project/scaffold_test.go
    cmd/awf/list_add_test.go templates/catalog.yaml .awf/config.yaml .awf/parts/workflow/local-hooks.md
    docs/workflow.md docs/doc-standard.md docs/agents-md-standard.md .awf/awf.lock`. Commit:
    `refactor(awf): promote standard docs to singletons`

## Phase 2 — Migration for other adopters

- [ ] **Task 2.1 — Create `internal/migrate/singletonstandarddocs.go`.**

  ```go
  package migrate

  import (
  	"errors"
  	"os"
  	"path/filepath"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  	"gopkg.in/yaml.v3"
  )

  // singletonStandardDocNames are the three docs ADR-0043 promotes from
  // toggleable `docs:` catalog entries to always-on singletons.
  var singletonStandardDocNames = []string{"workflow", "doc-standard", "agents-md-standard"}

  // applySingletonStandardDocs ports each of singletonStandardDocNames from the
  // plural docs shape to the singleton shape (config.IsSingletonKind), mirroring
  // portAgentsDoc's relocation when agents-doc first became a singleton: its
  // sidecar moves from <awfDir>/docs/<name>.yaml to <awfDir>/<name>.yaml, its
  // convention-part dir from <awfDir>/docs/parts/<name>/ to <awfDir>/parts/<name>/,
  // then <name> is stripped from the docs: array — each step a no-op if its
  // source is already absent, so a repeated run is idempotent.
  func applySingletonStandardDocs(root string) error {
  	awfDir := filepath.Join(root, ".awf")
  	for _, name := range singletonStandardDocNames {
  		if err := relocate(filepath.Join(awfDir, "docs", name+".yaml"), filepath.Join(awfDir, name+".yaml")); err != nil {
  			return err
  		}
  		if err := relocate(filepath.Join(awfDir, "docs", "parts", name), filepath.Join(awfDir, "parts", name)); err != nil {
  			return err
  		}
  		if err := removeFromDocsArray(filepath.Join(awfDir, "config.yaml"), name); err != nil {
  			return err
  		}
  	}
  	return nil
  }

  // relocate renames src to dst if src exists (file or directory); a no-op when
  // src is absent.
  func relocate(src, dst string) error {
  	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
  		return nil
  	} else if err != nil { // coverage-ignore: Stat fails here only on a permission fault a test cannot trigger
  		return err
  	}
  	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { // coverage-ignore: dst's parent is under the just-Stat'd .awf dir; fails only on a permission fault a test cannot trigger
  		return err
  	}
  	return os.Rename(src, dst)
  }

  // removeFromDocsArray strips name from the docs: array in the config.yaml at
  // path, if both the config and the array member are present. SetArrayMember
  // errors on removing an absent member or an absent key, so membership is
  // checked first via a plain decode.
  func removeFromDocsArray(path, name string) error {
  	src, err := os.ReadFile(path)
  	if errors.Is(err, os.ErrNotExist) {
  		return nil
  	}
  	if err != nil { // coverage-ignore: ReadFile faults only on a permission error that the test root bypasses
  		return err
  	}
  	var probe struct {
  		Docs []string `yaml:"docs"`
  	}
  	if err := yaml.Unmarshal(src, &probe); err != nil {
  		return err
  	}
  	present := false
  	for _, d := range probe.Docs {
  		if d == name {
  			present = true
  			break
  		}
  	}
  	if !present {
  		return nil
  	}
  	updated, err := config.SetArrayMember(src, "docs", name, false)
  	if err != nil { // coverage-ignore: the membership check above guarantees name is present under docs:, and yaml.Unmarshal above already validated src parses, so SetArrayMember cannot error here
  		return err
  	}
  	return os.WriteFile(path, updated, 0o644)
  }
  ```

- [ ] **Task 2.2 — Register the migration.** In `internal/migrate/migrate.go`, change:

  ```go
  var registry = []Migration{
  	{To: 1, Name: "tree-layout", Apply: applyTreeLayout},
  	{To: 2, Name: "drop-replacewith", Apply: applyDropReplaceWith},
  	{To: 3, Name: "awf-dir-relocation", Apply: applyAwfRelocation},
  	{To: 4, Name: "drop-hooks", Apply: applyDropHooks},
  	{To: 5, Name: "enable-bootstrap", Apply: applyEnableBootstrap},
  }
  ```

  to:

  ```go
  var registry = []Migration{
  	{To: 1, Name: "tree-layout", Apply: applyTreeLayout},
  	{To: 2, Name: "drop-replacewith", Apply: applyDropReplaceWith},
  	{To: 3, Name: "awf-dir-relocation", Apply: applyAwfRelocation},
  	{To: 4, Name: "drop-hooks", Apply: applyDropHooks},
  	{To: 5, Name: "enable-bootstrap", Apply: applyEnableBootstrap},
  	{To: 6, Name: "singleton-standard-docs", Apply: applySingletonStandardDocs},
  }
  ```

- [ ] **Task 2.3 — Create `internal/migrate/singletonstandarddocs_test.go`.**

  ```go
  package migrate

  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  )

  func TestSingletonStandardDocsRelocatesSidecarAndParts(t *testing.T) {
  	root := t.TempDir()
  	awf := filepath.Join(root, ".awf")
  	mustWrite(t, filepath.Join(awf, "config.yaml"), "prefix: ex\ndocs:\n  - architecture\n  - workflow\n  - doc-standard\n")
  	mustWrite(t, filepath.Join(awf, "docs", "workflow.yaml"), "data:\n  k: v\n")
  	mustWrite(t, filepath.Join(awf, "docs", "parts", "workflow", "local-hooks.md"), "LOCAL HOOKS BODY\n")

  	if err := applySingletonStandardDocs(root); err != nil {
  		t.Fatalf("applySingletonStandardDocs: %v", err)
  	}

  	if b, err := os.ReadFile(filepath.Join(awf, "workflow.yaml")); err != nil || !strings.Contains(string(b), "k: v") {
  		t.Errorf("workflow sidecar not relocated: %v, %s", err, b)
  	}
  	if _, err := os.Stat(filepath.Join(awf, "docs", "workflow.yaml")); !os.IsNotExist(err) {
  		t.Errorf("old workflow sidecar location should be gone, stat err = %v", err)
  	}
  	if b, err := os.ReadFile(filepath.Join(awf, "parts", "workflow", "local-hooks.md")); err != nil || string(b) != "LOCAL HOOKS BODY\n" {
  		t.Errorf("workflow part not relocated: %v, %s", err, b)
  	}
  	if _, err := os.Stat(filepath.Join(awf, "docs", "parts", "workflow")); !os.IsNotExist(err) {
  		t.Errorf("old workflow parts dir should be gone, stat err = %v", err)
  	}
  	cfg, err := os.ReadFile(filepath.Join(awf, "config.yaml"))
  	if err != nil {
  		t.Fatal(err)
  	}
  	if strings.Contains(string(cfg), "workflow") || strings.Contains(string(cfg), "doc-standard") {
  		t.Errorf("docs: array should no longer list workflow/doc-standard:\n%s", cfg)
  	}
  	if !strings.Contains(string(cfg), "- architecture") {
  		t.Errorf("untouched docs: entry lost:\n%s", cfg)
  	}
  }

  func TestSingletonStandardDocsIdempotent(t *testing.T) {
  	root := t.TempDir()
  	awf := filepath.Join(root, ".awf")
  	mustWrite(t, filepath.Join(awf, "config.yaml"), "prefix: ex\n")
  	if err := applySingletonStandardDocs(root); err != nil {
  		t.Fatalf("first run: %v", err)
  	}
  	if err := applySingletonStandardDocs(root); err != nil {
  		t.Fatalf("second run (no sidecars/parts/docs entries present) should be a no-op: %v", err)
  	}
  }

  func TestSingletonStandardDocsAbsentConfig(t *testing.T) {
  	if err := applySingletonStandardDocs(t.TempDir()); err != nil {
  		t.Errorf("applySingletonStandardDocs with no .awf/config.yaml should be a no-op, got %v", err)
  	}
  }

  func TestSingletonStandardDocsMalformedConfig(t *testing.T) {
  	root := t.TempDir()
  	mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), "docs: [a, b\n")
  	if err := applySingletonStandardDocs(root); err == nil {
  		t.Error("expected error surfaced from the malformed docs: probe decode")
  	}
  }
  ```

- [ ] **Task 2.4 — Fix `internal/migrate/migrate_test.go`'s three registry-count assertions.** Change:

  ```go
  func TestCurrentIsFive(t *testing.T) {
  	if Current() != 5 {
  		t.Errorf("Current() = %d, want 5", Current())
  	}
  }
  ```

  to:

  ```go
  func TestCurrentIsSix(t *testing.T) {
  	if Current() != 6 {
  		t.Errorf("Current() = %d, want 6", Current())
  	}
  }
  ```

  Then in `TestUpgradeAppliesInOrderIdempotent`, change:

  ```go
  	if strings.Join(applied, ",") != "tree-layout,drop-replacewith,awf-dir-relocation,drop-hooks,enable-bootstrap" {
  		t.Errorf("first Upgrade applied = %v, want [tree-layout drop-replacewith awf-dir-relocation drop-hooks enable-bootstrap]", applied)
  	}
  ```

  to:

  ```go
  	if strings.Join(applied, ",") != "tree-layout,drop-replacewith,awf-dir-relocation,drop-hooks,enable-bootstrap,singleton-standard-docs" {
  		t.Errorf("first Upgrade applied = %v, want [tree-layout drop-replacewith awf-dir-relocation drop-hooks enable-bootstrap singleton-standard-docs]", applied)
  	}
  ```

  Then in `TestUpgradeStampsTreeLock`, change:

  ```go
  	if strings.Join(applied, ",") != "drop-replacewith,awf-dir-relocation,drop-hooks,enable-bootstrap" {
  		t.Errorf("applied = %v, want [drop-replacewith awf-dir-relocation drop-hooks enable-bootstrap]", applied)
  	}
  ```

  to:

  ```go
  	if strings.Join(applied, ",") != "drop-replacewith,awf-dir-relocation,drop-hooks,enable-bootstrap,singleton-standard-docs" {
  		t.Errorf("applied = %v, want [drop-replacewith awf-dir-relocation drop-hooks enable-bootstrap singleton-standard-docs]", applied)
  	}
  ```

- [ ] **Task 2.5 — Verify and commit.**
  - Run `go build ./...`. Expect no errors.
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Run `./x check`. Expect `awf check: clean`.
  - Stage `internal/migrate/singletonstandarddocs.go internal/migrate/singletonstandarddocs_test.go
    internal/migrate/migrate.go internal/migrate/migrate_test.go`. Commit:
    `feat(awf): add singleton-standard-docs schema migration`

## Phase 3 — Template content fixes

- [ ] **Task 3.1 — `templates/agents-doc/AGENTS.md.tmpl`: add three static Document Map lines.**
  Change:

  ```
  <!-- awf:section document-map -->
  ## Document map

  - **ADR index:** [{{ .layout.adrReadme }}]({{ .layout.adrReadme }}) — architecture decisions and lifecycle.
  - **Active ADRs:** [{{ .layout.activeMd }}]({{ .layout.activeMd }}) — generated status index; do not hand-edit.
  - **Plans:** [{{ .layout.plansDir }}]({{ .layout.plansDir }}) — implementation plans for complex work.
  {{ range .docs }}- **{{ .title }}:** [{{ .path }}]({{ .path }}) — {{ .desc }}
  {{ end }}{{- range .data.docMap }}- **{{ .path }}:** [{{ .path }}]({{ .path }}){{ with .desc }} — {{ . }}{{ end }}
  {{ end -}}
  <!-- awf:end -->
  ```

  to:

  ```
  <!-- awf:section document-map -->
  ## Document map

  - **ADR index:** [{{ .layout.adrReadme }}]({{ .layout.adrReadme }}) — architecture decisions and lifecycle.
  - **Active ADRs:** [{{ .layout.activeMd }}]({{ .layout.activeMd }}) — generated status index; do not hand-edit.
  - **Plans:** [{{ .layout.plansDir }}]({{ .layout.plansDir }}) — implementation plans for complex work.
  - **Workflow:** [{{ .layout.workflowRef }}]({{ .layout.workflowRef }}) — principles, the brainstorm/ADR/plan chain, commit discipline.
  - **Documentation Standard:** [{{ .layout.docStandard }}]({{ .layout.docStandard }}) — how-to-write rules for all awf-managed prose.
  - **Authoring AGENTS.md:** [{{ .layout.agentsMdStandard }}]({{ .layout.agentsMdStandard }}) — layout, content, and rules for the agent guide.
  {{ range .docs }}- **{{ .title }}:** [{{ .path }}]({{ .path }}) — {{ .desc }}
  {{ end }}{{- range .data.docMap }}- **{{ .path }}:** [{{ .path }}]({{ .path }}){{ with .desc }} — {{ . }}{{ end }}
  {{ end -}}
  <!-- awf:end -->
  ```

- [ ] **Task 3.2 — `templates/docs/agents-md-standard.md.tmpl`: layout-based cross-reference.**
  Change:

  ```
  The agent guide (`AGENTS.md`) is the one doc loaded every session. Follow the [Documentation Standard](doc-standard.md) for how to write; this doc adds what is specific to the guide.
  ```

  to:

  ```
  The agent guide (`AGENTS.md`) is the one doc loaded every session. Follow the [Documentation Standard]({{ .layout.docStandard }}) for how to write; this doc adds what is specific to the guide.
  ```

  (Both docs render to the same `docsDir`, so `{{ .layout.docStandard }}` — root-relative — resolves
  as a same-directory filename with no `../` adjustment needed, matching how `agents-md-standard.md`
  and `doc-standard.md` already sit side by side under `docsDir`.)

- [ ] **Task 3.3 — `templates/docs/doc-standard.md.tmpl`: add the reference-if-enabled rule.**
  Change:

  ```
  - **Tool-agnostic, action-first.** Rendered skill and agent prose names the action an agent takes, not the tool one runtime exposes to take it: "dispatch a fresh-context subagent", not a runtime's tool name. Project-specific identifiers (skill names, command names) are not runtime tools and stay.
  <!-- awf:end -->
  ```

  to:

  ```
  - **Tool-agnostic, action-first.** Rendered skill and agent prose names the action an agent takes, not the tool one runtime exposes to take it: "dispatch a fresh-context subagent", not a runtime's tool name. Project-specific identifiers (skill names, command names) are not runtime tools and stay.
  - **Reference optional docs only when enabled.** Link a toggleable doc only if it is currently enabled in the project's `docs:` array; the dead-reference gate enforces this mechanically for markdown links (ADR-0020).
  <!-- awf:end -->
  ```

- [ ] **Task 3.4 — Add a regression test for the Document Map's unconditional mandatory-doc lines.**
  Backs ADR-0043's `inv: document-map-lists-mandatory-docs` ("`AGENTS.md`'s document-map section
  always cites `.layout.workflowRef`, `.layout.docStandard`, and `.layout.agentsMdStandard`,
  regardless of the project's `docs:` array contents") — no existing test in
  `internal/project/project_test.go` renders `AGENTS.md` with an empty `docs:` array and checks for
  these three lines. Add:

  ```go
  // invariant: document-map-lists-mandatory-docs
  func TestAgentsDocDocumentMapListsMandatorySingletonsUnconditionally(t *testing.T) {
  	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ndocs: []\n")
  	p, err := Open(root)
  	if err != nil {
  		t.Fatalf("Open: %v", err)
  	}
  	if err := p.Sync(); err != nil {
  		t.Fatalf("Sync: %v", err)
  	}
  	b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
  	if err != nil {
  		t.Fatalf("AGENTS.md not written: %v", err)
  	}
  	got := string(b)
  	for _, want := range []string{
  		"[docs/workflow.md](docs/workflow.md)",
  		"[docs/doc-standard.md](docs/doc-standard.md)",
  		"[docs/agents-md-standard.md](docs/agents-md-standard.md)",
  	} {
  		if !strings.Contains(got, want) {
  			t.Errorf("Document map should unconditionally cite %s (docs: array is empty):\n%s", want, got)
  		}
  	}
  }
  ```

  Append it to `internal/project/project_test.go` (its imports already cover `os`, `filepath`,
  `strings`, and `testing`).

- [ ] **Task 3.5 — Sync, verify, commit.**
  - Run `./x sync`. Expect `awf sync: done` (re-renders `AGENTS.md`, `docs/agents-md-standard.md`,
    `docs/doc-standard.md`).
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Run `./x check`. Expect `awf check: clean` (confirms the new Document Map lines and the
    `.layout.docStandard`-based link introduce no dead reference).
  - Stage `templates/agents-doc/AGENTS.md.tmpl templates/docs/agents-md-standard.md.tmpl
    templates/docs/doc-standard.md.tmpl internal/project/project_test.go AGENTS.md
    docs/agents-md-standard.md docs/doc-standard.md .awf/awf.lock`. Commit:
    `docs(awf): reference mandatory singletons in the Document Map`

## Phase 4 — Doc currency and ADR flip

- [ ] **Task 4.1 — Update the architecture doc.** In `.awf/docs/parts/architecture/components.md`,
  change:

  ```
  - **`internal/catalog/`** — reads `templates/catalog.yaml`; declares the available skills, agents,
    docs, and their sections.
  ```

  to:

  ```
  - **`internal/catalog/`** — reads `templates/catalog.yaml`; declares the available skills, agents,
    docs, and their sections. `Singletons` and the compile-time `SingletonKinds` list name every
    always-on singleton (ADR-0043).
  ```

  Then change:

  ```
  - **`internal/config/`** — owns `.awf/config.yaml`: the schema and strict load, its construction
    (`MarshalSkeleton`) and mutation (`SetArrayMember`, a comment-preserving `yaml.Node` round-trip)
    behind one `encode` funnel (ADR-0026; `internal/migrate` excepted), plus keyed sidecars.
  ```

  to:

  ```
  - **`internal/config/`** — owns `.awf/config.yaml`: the schema and strict load, its construction
    (`MarshalSkeleton`) and mutation (`SetArrayMember`, a comment-preserving `yaml.Node` round-trip)
    behind one `encode` funnel (ADR-0026; `internal/migrate` excepted), plus keyed sidecars.
    `IsSingletonKind` classifies off `internal/catalog`'s `SingletonKinds` (ADR-0043).
  ```

  Then change:

  ```
  - **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and
    `Check()`; golden tests live here. A single ordered kind-descriptor table (`kind.go`) is the sole
    per-kind dispatch source — enable array, catalog pool, declared sections, output path, and labels
    resolve through it across `list`/`add`/`check`/`validate` (ADR-0027).
  ```

  to:

  ```
  - **`internal/project/`** — orchestrates config + catalog + render + manifest into `Sync()` and
    `Check()`; golden tests live here. A single ordered kind-descriptor table (`kind.go`) is the sole
    per-kind dispatch source — enable array, catalog pool, declared sections, output path, and labels
    resolve through it across `list`/`add`/`check`/`validate` (ADR-0027). `singleton.go`'s
    `plainSingletons` table is the analogous single source of truth for the six neutral always-on
    singletons' render/validate identity (ADR-0043).
  ```

- [ ] **Task 4.2 — Update the `rendering` domain narrative.** In
  `.awf/domains/parts/rendering/current-state.md`, change:

  ```
  Always-on neutral singletons render the agent guide, the two ADR-system files (`README.md`, `template.md`), and a plan-authoring guide (`plans/README.md`) via shared `renderTarget` machinery, each suppressible with a `local: true` sidecar (ADR-0021, ADR-0020).
  ```

  to:

  ```
  Always-on neutral singletons render the agent guide, the two ADR-system files (`README.md`, `template.md`), a plan-authoring guide (`plans/README.md`), and — since ADR-0043 promoted them from toggleable docs — the workflow, documentation-standard, and agent-guide-authoring-standard docs, each suppressible with a `local: true` sidecar. The six non-agent-guide singletons render and validate through one shared `internal/project/singleton.go` table (`plainSingletons`) driving the existing `renderKind` primitive, rather than a hand-rolled loop duplicated between rendering and validation (ADR-0021, ADR-0020, ADR-0043).
  ```

- [ ] **Task 4.3 — Update the `config` domain narrative.** In
  `.awf/domains/parts/config/current-state.md`, change:

  ```
  The `config.yaml` that `awf init` scaffolds enables a curated workflow-core set (ADR-0022) — only the catalog's `core`-flagged skills and docs, plus all agents — while seeding every template-referenced var (across all template families) so a later opt-in `awf add` renders cleanly.
  ```

  to:

  ```
  The `config.yaml` that `awf init` scaffolds enables a curated workflow-core set (ADR-0022) — only the catalog's `core`-flagged skills, plus all agents — while seeding every template-referenced var (across all template families, including the always-on plain singletons) so a later opt-in `awf add` renders cleanly. No doc carries `core` any longer: the three docs that used to (`workflow`, `doc-standard`, `agents-md-standard`) are mandatory singletons outside the toggleable `docs:` catalog (ADR-0043), and a schema migration (`{To: 6}`) relocates their sidecar/convention-part paths and strips them from an upgrading project's `docs:` array.
  ```

- [ ] **Task 4.4 — Update the `tooling` domain narrative.** In
  `.awf/domains/parts/tooling/current-state.md`, change:

  ```
  It scaffolds a curated workflow-core default (ADR-0022): only the ten workflow-chain skills and three workflow docs are enabled, alongside all agents; the remaining catalog skills and docs are opt-in via the config arrays or `awf add`.
  ```

  to:

  ```
  It scaffolds a curated workflow-core default (ADR-0022): only the ten workflow-chain skills are enabled, alongside all agents; `workflow`/`doc-standard`/`agents-md-standard` are mandatory always-on singletons outside the toggleable `docs:` catalog and no longer scaffold-enabled docs, and no doc carries `core` any longer (ADR-0043); the remaining catalog skills are opt-in via the config arrays or `awf add`.
  ```

  (This sentence is otherwise unchanged by this plan — it still describes `awf init`'s adapter/pre-flight/prompting behaviour, none of which this ADR touches.)

- [ ] **Task 4.5 — Flip ADR-0043 to Implemented.** In
  `docs/decisions/0043-mandatory-singleton-status-for-workflow-and-documentation-standards.md`,
  change the frontmatter `status: Proposed` to `status: Implemented`.

- [ ] **Task 4.6 — Sync, verify, commit.**
  - Run `./x sync`. Expect `awf sync: done` (re-renders `docs/architecture.md`,
    `docs/domains/rendering.md`, `docs/domains/config.md`, `docs/domains/tooling.md`,
    `docs/decisions/ACTIVE.md`).
  - Run `./x gate`. Expect `coverage: 100.0%` and `0 issues.`
  - Run `./x invariants`. Expect `awf invariants: clean` — confirm every new `inv:` slug from
    ADR-0043 (`singleton-kind-single-source`, `plain-singleton-via-renderkind`,
    `singleton-doc-migration-relocates-parts`, `document-map-lists-mandatory-docs`,
    `mandatory-docs-not-in-docs-catalog`) has a backing `// invariant: <slug>` comment somewhere in
    this phase's or Phase 1-3's source — add any missing tag now if `awf invariants` reports one
    unbacked (a natural spot: `singleton.go`'s package doc for `plain-singleton-via-renderkind`,
    `catalog.go`'s `SingletonKinds` doc for `singleton-kind-single-source`,
    `singletonstandarddocs.go`'s package doc for `singleton-doc-migration-relocates-parts`,
    `AGENTS.md.tmpl`'s document-map section for `document-map-lists-mandatory-docs`, and
    `catalog.go`'s `docs:` removal for `mandatory-docs-not-in-docs-catalog`).
  - Run `./x check`. Expect `awf check: clean`.
  - Stage `.awf/docs/parts/architecture/components.md .awf/domains/parts/rendering/current-state.md
    .awf/domains/parts/config/current-state.md .awf/domains/parts/tooling/current-state.md
    docs/decisions/0043-mandatory-singleton-status-for-workflow-and-documentation-standards.md
    docs/architecture.md docs/domains/rendering.md docs/domains/config.md docs/domains/tooling.md
    docs/decisions/ACTIVE.md .awf/awf.lock`. Commit:
    `docs(awf): document singleton consolidation and implement ADR-0043`

## Verification (whole change)

- `./x gate` green; `./x check` clean; `./x invariants` clean throughout every phase.
- `docs/workflow.md`, `docs/doc-standard.md`, `docs/agents-md-standard.md` exist and render
  regardless of `.awf/config.yaml`'s `docs:` array contents — confirm by temporarily emptying
  `docs:` in a scratch scaffolded project and running `awf sync`: all three still render.
  `local: true` on `.awf/workflow.yaml` (or `.awf/doc-standard.yaml`/`.awf/agents-md-standard.yaml`)
  still suppresses that one singleton's content, matching the other four.
- `AGENTS.md`'s Document Map lists Workflow, Documentation Standard, and Authoring AGENTS.md
  unconditionally, even in a scratch project with an empty `docs:` array.
- Initializing a scratch project with `docs=agents-md-standard` alone (no `doc-standard`) — the
  exact reproduction that motivated ADR-0043 — no longer applies, since `agents-md-standard` is not a
  valid `docs:` array member anymore; confirm `awf add doc agents-md-standard` now errors
  `"agents-md-standard" is not a catalog doc`.
- A project scaffolded before this change (a fixture with `docs: [workflow, doc-standard,
  agents-md-standard]` and a `.awf/docs/parts/workflow/local-hooks.md`) runs cleanly through
  `awf upgrade`: the sidecar/part relocate, the `docs:` array loses all three names, and a subsequent
  `awf sync`/`check` is clean.
- `Current()` is `6`; `awf upgrade` on a schema-5 project applies exactly
  `singleton-standard-docs`.
- ADR-0043 is `Implemented`; its five `inv:` slugs are backed.

## Execution

Phases are ordered and sequentially dependent: Phase 2's migration references
`config.IsSingletonKind`'s post-Phase-1 semantics only incidentally (it doesn't call it directly, but
its whole premise — porting docs into the singleton shape — presumes Phase 1 already defines that
shape) and Phase 3/4 cite `.layout.docStandard`/`.layout.agentsMdStandard`, which only exist after
Phase 1. Execute inline with `awf-executing-plans` (one task at a time, `./x gate` per commit) — the
phases are tightly sequential with a shared package (`internal/project`) touched throughout, so
per-task subagent dispatch would add fresh-context re-grounding overhead without the isolation payoff
`awf-subagent-driven-development` is for.
