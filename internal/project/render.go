package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
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

// partRel is the project-relative convention part path the awf:edit pointer names.
func partRel(kind, artifact, section string) string {
	if config.IsSingletonKind(kind) {
		return ".awf/parts/" + kind + "/" + section + ".md"
	}
	return ".awf/" + kind + "/parts/" + artifact + "/" + section + ".md"
}

// planSections resolves each catalog-declared section into a render.SectionPlan:
// a sidecar drop wins; otherwise an existing convention part substitutes its body;
// otherwise the template default renders. Precedence: drop > convention part > default.
func (p *Project) planSections(kind, artifact string, declared []string, sec map[string]config.SectionOverride) (map[string]render.SectionPlan, error) {
	plan := map[string]render.SectionPlan{}
	for _, s := range declared {
		sp := render.SectionPlan{EditPath: partRel(kind, artifact, s)}
		if ov, ok := sec[s]; ok && ov.Drop {
			sp.Drop = true
			plan[s] = sp
			continue
		}
		b, err := os.ReadFile(p.Cfg.PartPath(kind, artifact, s))
		if err == nil {
			sp.HasPart = true
			sp.PartBody = string(b)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read part %s/%s/%s: %w", kind, artifact, s, err)
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

// renderKindSpec drives one catalog-backed render loop (skills/agents/docs): the
// kinds that share the sort → sidecar → skip-local → render → append shape. tid
// and sections derive from the artifact name; outPath also takes the adapter
// target (ignored by neutral kinds like docs); target is the adapter this pass
// renders for (zero for neutral kinds). gate (optional, nil = always render)
// suppresses an artifact — used for the skills doc-gate.
type renderKindSpec struct {
	kind     string
	names    []string
	target   Target
	tid      func(name string) string
	sections func(name string) []string
	outPath  func(t Target, name string) string
	gate     func(name string) bool
}

func (p *Project) renderKind(spec renderKindSpec) ([]RenderedFile, error) {
	var out []RenderedFile
	for _, name := range slices.Sorted(slices.Values(spec.names)) {
		sc, err := p.Cfg.Sidecar(spec.kind, name)
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
		if spec.gate != nil && !spec.gate(name) {
			continue
		}
		rf, err := p.renderTarget(spec.kind, name, spec.tid(name), spec.sections(name), sc, p.data(sc), spec.outPath(spec.target, name))
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, nil
}

func (p *Project) RenderAll() ([]RenderedFile, error) {
	var out []RenderedFile
	enabledDocs := sliceSet(p.Cfg.Docs)
	// Neutral: docs render once — the output path is docsDir-relative, not adapter-placed.
	docsRfs, err := p.renderKind(renderKindSpec{
		kind: "docs", names: p.Cfg.Docs,
		tid:      func(n string) string { return fmt.Sprintf("docs/%s.md.tmpl", n) },
		sections: func(n string) []string { return p.Cat.Docs[n].Sections },
		outPath:  func(_ Target, n string) string { return p.docOutPath(n) },
	})
	if err != nil {
		return nil, err
	}
	out = append(out, docsRfs...)
	// Adapter: skills + agents render once per enabled target (inv: multi-target-render).
	// invariant: multi-target-render
	for _, t := range p.Targets {
		for _, spec := range []renderKindSpec{
			{
				kind: "skills", names: p.Cfg.Skills, target: t,
				tid:      func(n string) string { return fmt.Sprintf("skills/%s/SKILL.md.tmpl", n) },
				sections: func(n string) []string { return p.Cat.Skills[n].Sections },
				outPath:  func(t Target, n string) string { return t.SkillPath(p.Cfg.Prefix, n) },
				// Doc-gated skill: omit from the render set when its required doc is not
				// enabled (inv: doc-gated-skill-suppressed).
				// invariant: doc-gated-skill-suppressed
				gate: func(n string) bool {
					req := p.Cat.Skills[n].RequiresDoc
					return req == "" || enabledDocs[req]
				},
			},
			{
				kind: "agents", names: p.Cfg.Agents, target: t,
				tid:      func(n string) string { return fmt.Sprintf("agents/%s.md.tmpl", n) },
				sections: func(n string) []string { return p.Cat.Agents[n].Sections },
				outPath:  func(t Target, n string) string { return t.AgentPath(n) },
			},
		} {
			rfs, err := p.renderKind(spec)
			if err != nil {
				return nil, err
			}
			out = append(out, rfs...)
		}
	}
	// agents-doc / AGENTS.md (always-on singleton unless its sidecar is local), neutral — once.
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
		// Bridge: each adapter that wants one imports AGENTS.md verbatim (ADR-0037).
		// Gated on the agents-doc render above — a local (hand-maintained) AGENTS.md
		// must not get a bridge pointing at an un-rendered file. cursor has an empty
		// BridgeFile and emits nothing (inv: cursor-no-bridge).
		// invariant: cursor-no-bridge
		for _, t := range p.Targets {
			if t.BridgeFile == "" {
				continue
			}
			brf, err := p.renderTarget("claude", "", "claude/CLAUDE.md.tmpl",
				nil, config.Sidecar{}, p.data(config.Sidecar{}), t.BridgeFile)
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

// renderTarget assembles an artifact (sidecar sections + convention parts), executes
// the template, rejects publication-unsafe <no value> output, and projects the
// per-artifact ConfigHash over the artifact's effective inputs.
func (p *Project) renderTarget(kind, artifact, tid string, declared []string, sc config.Sidecar, data map[string]any, outPath string) (RenderedFile, error) {
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("read template %s: %w", tid, err)
	}
	plan, err := p.planSections(kind, artifact, declared, sc.Sections)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	assembled, parts := render.Assemble(render.ParseSections(string(src)), plan)
	content, err := render.Execute(assembled, data, parts, tid)
	if err != nil { // coverage-ignore: with raw convention parts (ADR-0034) and always-valid embedded template defaults, render.Execute cannot fail through RenderAll; its own parse/execute error branches are unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	if strings.Contains(content, "<no value>") {
		return RenderedFile{}, fmt.Errorf("render %s: output contains \"<no value>\" — a referenced var or data key is unset", outPath)
	}
	content = injectBanner(content)
	cfgHash, err := p.artifactConfigHash(assembled, sc, p.consumedParts(kind, artifact, plan))
	if err != nil { // coverage-ignore: artifactConfigHash only fails on an unreadable consumed part, but planSections above already read every HasPart part, so consumedParts holds only readable paths
		return RenderedFile{}, err
	}
	return RenderedFile{
		Path: outPath, Content: content, TemplateID: tid,
		TemplateHash: manifest.Hash(src), ConfigHash: cfgHash,
	}, nil
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
