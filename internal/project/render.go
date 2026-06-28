package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"

	"gopkg.in/yaml.v3"
)

type RenderedFile struct {
	Path         string
	Content      string
	TemplateID   string
	TemplateHash string
	ConfigHash   string
}

// data assembles the template data namespace for a target: the prefix, the
// project vars, the sidecar's structured data, and the awf-given docs layout.
func (p *Project) data(sc config.Sidecar) map[string]any {
	return map[string]any{
		"prefix": p.Cfg.Prefix,
		"vars":   nonNil(p.Cfg.Vars),
		"data":   nonNil(sc.Data),
		"layout": p.layout().templateMap(),
	}
}

// Layout is the fixed, awf-given docs layout derived from cfg.DocsDir, in typed
// form for Go consumers. These paths are not configurable through vars.
// templateMap projects it into the .layout template namespace (templates read a
// map, not unexported struct fields) and into the per-file ConfigHash.
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

// templateMap projects the layout into the map the .layout template namespace and
// the per-file ConfigHash consume, reproducing the historical layout() map exactly
// so the ConfigHash stays byte-identical (no drift).
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

// docOutPath is the output path for a managed doc, rooted at docsDir.
func (p *Project) docOutPath(name string) string {
	return strings.TrimRight(p.Cfg.DocsDir, "/") + "/" + name + ".md"
}

// decisionsDir is the absolute ADR decisions directory.
func (p *Project) decisionsDir() string {
	return filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")
}

// resolvedDocs builds the Document-map entries for the agents-doc template from
// the docs declared in config, annotated with the catalog's title/desc. Local
// docs are excluded.
func (p *Project) resolvedDocs() ([]map[string]any, error) {
	out := []map[string]any{}
	names := append([]string(nil), p.Cfg.Docs...)
	sort.Strings(names)
	for _, name := range names {
		sc, err := p.Cfg.Sidecar("docs", name)
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
		spec := p.Cat.Docs[name]
		out = append(out, map[string]any{
			"name":  name,
			"title": spec.Title,
			"desc":  spec.Desc,
			"path":  p.docOutPath(name),
		})
	}
	return out, nil
}

// partRel is the project-relative convention part path the awf:edit pointer names.
func partRel(kind, target, section string) string {
	if config.IsSingletonKind(kind) {
		return ".awf/parts/" + kind + "/" + section + ".md"
	}
	return ".awf/" + kind + "/parts/" + target + "/" + section + ".md"
}

// planSections resolves each catalog-declared section into a render.SectionPlan:
// a sidecar drop wins; otherwise an existing convention part substitutes its body;
// otherwise the template default renders. Precedence: drop > convention part > default.
func (p *Project) planSections(kind, target string, declared []string, sec map[string]config.SectionOverride) (map[string]render.SectionPlan, error) {
	plan := map[string]render.SectionPlan{}
	for _, s := range declared {
		sp := render.SectionPlan{EditPath: partRel(kind, target, s)}
		if ov, ok := sec[s]; ok && ov.Drop {
			sp.Drop = true
			plan[s] = sp
			continue
		}
		b, err := os.ReadFile(p.Cfg.PartPath(kind, target, s))
		if err == nil {
			sp.HasPart = true
			sp.PartBody = string(b)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read part %s/%s/%s: %w", kind, target, s, err)
		}
		plan[s] = sp
	}
	return plan, nil
}

func nonNil(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

const bannerText = "GENERATED by awf — do not edit; change .awf/ and run `awf sync`"

// injectBanner inserts the generated-by banner into rendered content: for hooks,
// a `#` comment after the shebang line; for frontmatter targets, an HTML comment
// after the closing `---`; otherwise an HTML comment as the first line.
// invariant: provenance-banner
func injectBanner(content, tid string) string {
	if strings.HasPrefix(tid, "hooks/") {
		line := "# " + bannerText + "\n"
		if i := strings.IndexByte(content, '\n'); i >= 0 {
			return content[:i+1] + line + content[i+1:]
		}
		return content + "\n" + line
	}
	line := "<!-- " + bannerText + " -->\n"
	if yamlBlock, body, found := frontmatter.Split([]byte(content)); found {
		return "---\n" + string(yamlBlock) + "---\n" + line + string(body)
	}
	return line + content
}

func (p *Project) RenderAll() ([]RenderedFile, error) {
	var out []RenderedFile
	// Skills.
	enabledDocs := sliceSet(p.Cfg.Docs)
	for _, name := range slices.Sorted(slices.Values(p.Cfg.Skills)) {
		sc, err := p.Cfg.Sidecar("skills", name)
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
		// Doc-gated skill: omit from the render set when its required doc is not
		// enabled (inv: doc-gated-skill-suppressed).
		if req := p.Cat.Skills[name].RequiresDoc; req != "" && !enabledDocs[req] {
			continue
		}
		rf, err := p.renderTarget("skills", name, fmt.Sprintf("skills/%s/SKILL.md.tmpl", name),
			p.Cat.Skills[name].Sections, sc, p.data(sc),
			p.Target.SkillPath(p.Cfg.Prefix, name))
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	// Agents.
	for _, name := range slices.Sorted(slices.Values(p.Cfg.Agents)) {
		sc, err := p.Cfg.Sidecar("agents", name)
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
		rf, err := p.renderTarget("agents", name, fmt.Sprintf("agents/%s.md.tmpl", name),
			p.Cat.Agents[name].Sections, sc, p.data(sc),
			p.Target.AgentPath(name))
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	// Hooks.
	for _, h := range p.Cfg.Hooks {
		rf, err := p.renderTarget("hooks", h, fmt.Sprintf("hooks/%s.tmpl", h),
			nil, config.Sidecar{}, p.data(config.Sidecar{}), ".githooks/"+h)
		if err != nil { // coverage-ignore: catalog hook templates are static, part-free, and reference only guarded vars; renderTarget cannot fail for a hook
			return nil, err
		}
		out = append(out, rf)
	}
	// Docs.
	for _, name := range slices.Sorted(slices.Values(p.Cfg.Docs)) {
		sc, err := p.Cfg.Sidecar("docs", name)
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
		rf, err := p.renderTarget("docs", name, fmt.Sprintf("docs/%s.md.tmpl", name),
			p.Cat.Docs[name].Sections, sc, p.data(sc), p.docOutPath(name))
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	// agents-doc (always-on singleton unless its sidecar is local).
	ad, err := p.Cfg.Sidecar("agents-doc", "")
	if err != nil {
		return nil, err
	}
	if !ad.Local {
		data := p.data(ad)
		docs, err := p.resolvedDocs()
		if err != nil { // coverage-ignore: resolvedDocs only errors on a docs-sidecar read failure, which RenderAll's docs loop already surfaces earlier
			return nil, err
		}
		data["docs"] = docs
		rf, err := p.renderTarget("agents-doc", "", "agents-doc/AGENTS.md.tmpl",
			p.Cat.AgentsDoc.Sections, ad, data, "AGENTS.md")
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
		// CLAUDE.md bridge: the Claude adapter imports AGENTS.md verbatim so Claude
		// Code ingests it regardless of its auto-fallback (ADR-0016). Tied to AGENTS.md
		// existing (the agents-doc render above).
		if p.Target.BridgeFile != "" {
			brf, err := p.renderTarget("claude", "", "claude/CLAUDE.md.tmpl",
				nil, config.Sidecar{}, p.data(config.Sidecar{}), p.Target.BridgeFile)
			if err != nil { // coverage-ignore: the bridge template is static, part-free, and references no vars, so renderTarget cannot produce <no value> or a read error
				return nil, err
			}
			out = append(out, brf)
		}
	}
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
	return out, nil
}

// PlannedOutputs returns the project-relative paths Sync would write: every
// RenderAll output plus the generated ACTIVE.md and domain docs. Used by
// awf init to detect collisions before writing (ADR-0016).
func (p *Project) PlannedOutputs() ([]string, error) {
	files, err := p.RenderAll()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	amd, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	paths = append(paths, amd.Path)
	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: unreachable — generateActiveMD above parses the same decisions dir and fails first on a malformed ADR
		return nil, err
	}
	for _, dd := range dds {
		paths = append(paths, dd.Path)
	}
	return paths, nil
}

// renderTarget assembles a target (sidecar sections + convention parts), executes
// the template, rejects publication-unsafe <no value> output, and projects the
// per-target ConfigHash over the target's effective inputs.
func (p *Project) renderTarget(kind, target, tid string, declared []string, sc config.Sidecar, data map[string]any, outPath string) (RenderedFile, error) {
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("read template %s: %w", tid, err)
	}
	plan, err := p.planSections(kind, target, declared, sc.Sections)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	assembled := render.Assemble(render.ParseSections(string(src)), plan)
	content, err := render.Execute(assembled, data)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	if strings.Contains(content, "<no value>") {
		return RenderedFile{}, fmt.Errorf("render %s: output contains \"<no value>\" — a referenced var or data key is unset", outPath)
	}
	content = injectBanner(content, tid)
	cfgHash, err := p.targetConfigHash(assembled, sc, p.consumedParts(kind, target, plan))
	if err != nil { // coverage-ignore: targetConfigHash only fails on an unreadable consumed part, but planSections above already read every HasPart part, so consumedParts holds only readable paths
		return RenderedFile{}, err
	}
	return RenderedFile{
		Path: outPath, Content: content, TemplateID: tid,
		TemplateHash: manifest.Hash(src), ConfigHash: cfgHash,
	}, nil
}

// consumedParts returns the absolute paths of the convention parts a target
// consumed; editing any reflags the target's drift.
func (p *Project) consumedParts(kind, target string, plan map[string]render.SectionPlan) []string {
	var paths []string
	for sec, sp := range plan {
		if sp.HasPart {
			paths = append(paths, p.Cfg.PartPath(kind, target, sec))
		}
	}
	return paths
}

// targetConfigHash projects the drift signal onto one rendered file: the prefix, the
// subset of vars the assembled template references, the target's sidecar (marshalled),
// and the bytes of every convention part it consumed — in deterministic order.
func (p *Project) targetConfigHash(assembled string, sc config.Sidecar, partPaths []string) (string, error) {
	refs := render.ReferencedVars(assembled)
	proj := map[string]any{"prefix": p.Cfg.Prefix, "layout": p.layout().templateMap()}
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
	if err != nil { // coverage-ignore: proj holds only YAML-sourced, marshalable values; yaml.Marshal cannot fail here
		return "", err
	}
	return manifest.Hash(enc), nil
}

// generateActiveMD renders the ADR index for the project's decisions directory.
// It always produces a file: a populated index when ADRs exist, else a placeholder
// (ADR-0020 Decision 6 — partial-item supersedence of ADR-0005/ADR-0006).
func (p *Project) generateActiveMD() (RenderedFile, error) {
	content, err := adr.RenderActiveMD(p.decisionsDir())
	if err != nil {
		return RenderedFile{}, err
	}
	return RenderedFile{Path: p.layout().ActiveMd, Content: content}, nil
}

// generateDomainDocs renders one content-only doc per declared domain
// (<docsDir>/domains/<name>.md): the domain template + its convention parts, with
// the per-domain ADR index injected as .data.decisions. Like ACTIVE.md, the result
// carries no TemplateID/Hash — drift is checked by regeneration, since the index
// depends on external ADR frontmatter state.
func (p *Project) generateDomainDocs() ([]RenderedFile, error) {
	decisionsDir := p.decisionsDir()
	lay := p.layout()
	var out []RenderedFile
	for _, name := range slices.Sorted(slices.Values(p.Cfg.Domains)) {
		index, err := adr.RenderDomainIndex(decisionsDir, name)
		if err != nil {
			return nil, err
		}
		data := p.data(config.Sidecar{})
		data["data"] = map[string]any{"domain": name, "decisions": index}
		rf, err := p.renderTarget("domains", name, "domains/domain.md.tmpl",
			p.Cat.DomainDoc.Sections, config.Sidecar{}, data,
			lay.DomainsDir+"/"+name+".md")
		if err != nil { // coverage-ignore: .data.domain/.data.decisions are always set and the template is embedded, so renderTarget cannot produce <no value> or a read error here
			return nil, err
		}
		out = append(out, RenderedFile{Path: rf.Path, Content: rf.Content})
	}
	return out, nil
}
