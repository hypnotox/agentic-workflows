package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

const (
	bridgeTID    = "claude/CLAUDE.md.tmpl"
	bootstrapTID = "bootstrap/awf-bootstrap.sh.tmpl"
	memoryTID    = "memory/gitignore.tmpl"
)

// hookNames are the git-hook payload scripts the hooks singleton renders as a
// unit under .awf/hooks/ (ADR-0048); template ids are hooks/<name>.sh.tmpl.
var hookNames = []string{"pre-commit", "commit-msg", "pre-push"}

// HookNames returns the git-hook payload names the hooks singleton renders
// (ADR-0048), for CLI surfaces that enumerate them (the KnownTargets pattern).
func HookNames() []string { return slices.Clone(hookNames) }

type RenderedFile struct {
	Path         string
	Content      string
	TemplateID   string
	TemplateHash string
	ConfigHash   string
	// assembled is the executed template source (post section-overlay, pre
	// execution); UnsetVarNotes scans it for referenced-but-unset vars (ADR-0045).
	assembled string
}

// data assembles the template data namespace for a target: the prefix, the
// project vars, the sidecar's structured data, and the awf-given docs layout.
func (p *Project) data(sc config.Sidecar) map[string]any {
	return map[string]any{
		"prefix":           p.Cfg.Prefix,
		"vars":             nonNil(p.Cfg.Vars),
		"data":             nonNil(sc.Data),
		"layout":           p.layout().templateMap(),
		"version":          Version,
		"skills":           p.effSkills,
		"commitScopes":     p.commitScopesDisplay(),
		"invariantMarkers": p.invariantMarkersDisplay(),
	}
}

// commitScopesDisplay returns the display-formatted allowed commit-scope list
// (e.g. "`adr`, `awf`, `plans`") resolved from audit.allowedScopes — the same
// audit.Resolve path awf commit-gate reads, so prose and gate agree by
// construction — or "" when scopes are accept-any (ADR-0051).
func (p *Project) commitScopesDisplay() string {
	scopes := audit.Resolve(p.Cfg.Audit).AllowedScopes
	if len(scopes) == 0 {
		return ""
	}
	quoted := make([]string, len(scopes))
	for i, s := range scopes {
		quoted[i] = "`" + s.Name + "`"
	}
	return strings.Join(quoted, ", ")
}

// invariantMarkersDisplay returns the inline glob→marker mapping derived from
// invariants.sources (e.g. "`*.go` → `//`, `*.py` → `#`"), the invariant-tagging
// analog of commitScopesDisplay — or "" when no sources are configured (ADR-0064).
func (p *Project) invariantMarkersDisplay() string {
	if p.Cfg.Invariants == nil || len(p.Cfg.Invariants.Sources) == 0 {
		return ""
	}
	entries := make([]string, len(p.Cfg.Invariants.Sources))
	for i, s := range p.Cfg.Invariants.Sources {
		globs := make([]string, len(s.Globs))
		for j, g := range s.Globs {
			globs[j] = "`" + g + "`"
		}
		entries[i] = strings.Join(globs, ", ") + " → `" + s.Marker + "`"
	}
	return strings.Join(entries, ", ")
}

// effectiveSkills returns the skill names whose files exist on disk under awf's
// model: enabled minus doc-gate-suppressed, keeping local-declared ones
// (hand-maintained but present) even where a doc gate would suppress the render.
// invariant: skills-context-effective-set
func (p *Project) effectiveSkills() (map[string]bool, error) {
	enabledDocs := sliceSet(p.Cfg.Docs)
	eff := map[string]bool{}
	for _, name := range p.Cfg.Skills {
		sc, err := p.Cfg.Sidecar("skills", name)
		if err != nil {
			return nil, err
		}
		if sc.Local || p.skillDocGateOpen(name, enabledDocs) {
			eff[name] = true
		}
	}
	return eff, nil
}

// skillDocGateOpen reports whether a doc-gated skill's required doc is enabled
// (inv: doc-gated-skill-suppressed shares this single source of truth).
func (p *Project) skillDocGateOpen(name string, enabledDocs map[string]bool) bool {
	req := p.Cat.Skills[name].RequiresDoc
	return req == "" || enabledDocs[req]
}

// partRel is the project-relative convention part path the awf:edit pointer names,
// derived from the absolute PartPath so the parts-path structure has one source.
func (p *Project) partRel(kind, artifact, section string) string {
	rel, err := filepath.Rel(p.Root, p.Cfg.PartPath(kind, artifact, section))
	if err != nil { // coverage-ignore: PartPath is always rooted under p.Root, so Rel cannot fail
		return ""
	}
	return filepath.ToSlash(rel)
}

// planSections resolves each catalog-declared section into a render.SectionPlan:
// a sidecar drop wins; otherwise an existing convention part substitutes its body;
// otherwise the template default renders. Precedence: drop > convention part > default.
func (p *Project) planSections(kind, artifact string, declared []string, sec map[string]config.SectionOverride) (map[string]render.SectionPlan, error) {
	plan := map[string]render.SectionPlan{}
	reg, err := p.placeholderRegistry()
	if err != nil {
		return nil, err
	}
	for _, s := range declared {
		sp := render.SectionPlan{EditPath: p.partRel(kind, artifact, s)}
		if ov, ok := sec[s]; ok && ov.Drop {
			sp.Drop = true
			plan[s] = sp
			continue
		}
		b, err := os.ReadFile(p.Cfg.PartPath(kind, artifact, s))
		if err == nil {
			body, serr := p.substitutePlaceholders(p.partRel(kind, artifact, s), string(b), reg)
			if serr != nil {
				return nil, serr
			}
			sp.HasPart = true
			sp.PartBody = body
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
	// defaults returns the artifact's catalog default data (nil = none).
	defaults func(name string) map[string]any
}

// skillTID resolves a skill's template id: the shared base template for a
// synthesized local entry, else the name-derived catalog path (ADR-0068).
// invariant: local-renders-from-base
func (p *Project) skillTID(n string) string {
	if p.Cat.Skills[n].Base {
		return baseSkillTID
	}
	return mustDescriptor("skills").tid(n)
}

// agentTID mirrors skillTID for agents.
func (p *Project) agentTID(n string) string {
	if p.Cat.Agents[n].Base {
		return baseAgentTID
	}
	return mustDescriptor("agents").tid(n)
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
		if spec.defaults != nil {
			sc = withDefaultData(sc, spec.defaults(name))
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
	eff, err := p.effectiveSkills()
	if err != nil {
		return nil, err
	}
	p.effSkills = eff
	// Neutral: docs render once — the output path is docsDir-relative, not adapter-placed.
	docsRfs, err := p.renderKind(renderKindSpec{
		kind: "docs", names: p.Cfg.Docs,
		tid:      mustDescriptor("docs").tid,
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
				tid:      p.skillTID,
				sections: func(n string) []string { return p.Cat.Skills[n].Sections },
				outPath:  func(t Target, n string) string { return t.SkillPath(p.Cfg.Prefix, n) },
				// Doc-gated skill: omit from the render set when its required doc is not
				// enabled (inv: doc-gated-skill-suppressed).
				// invariant: doc-gated-skill-suppressed
				gate:     func(n string) bool { return p.skillDocGateOpen(n, enabledDocs) },
				defaults: func(n string) map[string]any { return p.Cat.Skills[n].Data },
			},
			{
				kind: "agents", names: p.Cfg.Agents, target: t,
				tid:      p.agentTID,
				sections: func(n string) []string { return p.Cat.Agents[n].Sections },
				outPath:  func(t Target, n string) string { return t.AgentPath(n) },
				defaults: func(n string) map[string]any { return p.Cat.Agents[n].Data },
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
		ad = withDefaultData(ad, p.Cat.Docs["agents-doc"].Data)
		data := p.data(ad)
		docs, err := p.resolvedDocs()
		if err != nil { // coverage-ignore: resolvedDocs only errors on a docs-sidecar read failure, which RenderAll's docs loop already surfaces earlier
			return nil, err
		}
		data["docs"] = docs
		data["mandatoryDocs"] = p.documentMapDocs()
		rf, err := p.renderTarget("agents-doc", "", "agents-doc/AGENTS.md.tmpl",
			p.Cat.Docs["agents-doc"].Sections, ad, data, "AGENTS.md")
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
			brf, err := p.renderTarget("claude", "", bridgeTID,
				nil, config.Sidecar{}, p.data(config.Sidecar{}), t.BridgeFile)
			if err != nil { // coverage-ignore: the bridge template is static, part-free, and references no vars, so renderTarget cannot produce <no value> or a read error
				return nil, err
			}
			out = append(out, brf)
		}
	}
	// Plain singletons: every Mandatory non-agents-doc entry in the catalog doc
	// collection, derived into plainSingletons (always-on unless local; ADR-0021,
	// ADR-0043, ADR-0059, ADR-0061).
	lay := p.layout()
	for _, sg := range plainSingletons {
		rfs, err := p.renderKind(renderKindSpec{
			kind: sg.kind, names: []string{""},
			tid:      func(string) string { return sg.tid },
			sections: func(string) []string { return sg.sections(p.Cat) },
			outPath:  func(Target, string) string { return sg.outPath(lay) },
			defaults: func(string) map[string]any { return p.Cat.Docs[sg.kind].Data },
		})
		if err != nil {
			return nil, err
		}
		out = append(out, rfs...)
	}
	// .awf/bootstrap.sh (neutral config-tree singleton; rendered only when enabled —
	// ADR-0040, relocated by ADR-0047). No catalog spec / no overridable sections,
	// like the CLAUDE.md bridge.
	if p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled {
		brf, err := p.renderTarget("bootstrap", "", bootstrapTID,
			nil, config.Sidecar{}, p.data(config.Sidecar{}), config.DirName+"/bootstrap.sh")
		if err != nil { // coverage-ignore: the bootstrap template references only .version (always set) and no parts, so renderTarget cannot produce <no value> or a read error
			return nil, err
		}
		out = append(out, brf)
	}
	// .awf/hooks/*.sh git-hook payloads (neutral config-tree singleton; rendered
	// as a unit only when enabled — ADR-0048). No catalog spec / no overridable
	// sections, like the bootstrap; awf never activates them.
	if p.Cfg.Hooks != nil && p.Cfg.Hooks.Enabled {
		for _, name := range hookNames {
			hrf, err := p.renderTarget("hooks", "", "hooks/"+name+".sh.tmpl",
				nil, config.Sidecar{}, p.data(config.Sidecar{}), config.DirName+"/hooks/"+name+".sh")
			if err != nil { // coverage-ignore: every var reference in the hook templates is with/else- or if-wrapped (ADR-0045), and they use no parts, so renderTarget cannot produce <no value> or a read error
				return nil, err
			}
			out = append(out, hrf)
		}
	}
	// .awf/memory/.gitignore (neutral config-tree singleton; ALWAYS rendered —
	// ADR-0069, no config gate unlike bootstrap/hooks). Self-ignoring, so the
	// working-memory convention's ephemerality is mechanical, not remembered.
	// Deliberately non-configurable: no catalog spec, no sections, no CLI kind.
	mrf, err := p.renderTarget("memory", "", memoryTID,
		nil, config.Sidecar{}, p.data(config.Sidecar{}), config.DirName+"/memory/.gitignore")
	if err != nil { // coverage-ignore: the memory gitignore template is static, part-free, and references no vars, so renderTarget cannot produce <no value> or a read error
		return nil, err
	}
	out = append(out, mrf)
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
	expanded, err := render.ExpandIncludes(string(src), templates.FS)
	if err != nil { // coverage-ignore: awf-owned embedded templates never author a missing/nested/section-bearing include, so ExpandIncludes cannot fail through RenderAll; its error branches are unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	plan, err := p.planSections(kind, artifact, declared, sc.Sections)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	assembled, parts := render.Assemble(render.ParseSections(expanded), plan)
	content, err := render.Execute(assembled, data, parts, tid)
	if err != nil { // coverage-ignore: with raw convention parts (ADR-0034) and always-valid embedded template defaults, render.Execute cannot fail through RenderAll; its own parse/execute error branches are unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	if strings.Contains(content, "<no value>") {
		return RenderedFile{}, fmt.Errorf("render %s: output contains \"<no value>\" — a referenced var or data key is unset", outPath)
	}
	content = injectBanner(content, tid)
	cfgHash, err := p.artifactConfigHash(assembled, sc, p.consumedParts(kind, artifact, plan))
	if err != nil { // coverage-ignore: artifactConfigHash only fails on an unreadable consumed part, but planSections above already read every HasPart part, so consumedParts holds only readable paths
		return RenderedFile{}, err
	}
	return RenderedFile{
		Path: outPath, Content: content, TemplateID: tid,
		// TemplateHash covers the post-expansion source so an edit to an included
		// partial flags every including artifact stale (ADR-0052).
		// invariant: include-in-templatehash
		TemplateHash: manifest.Hash([]byte(expanded)), ConfigHash: cfgHash,
		assembled: assembled,
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
	content = injectBanner(content, "")
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
		rf, err := p.renderTarget("domains", name, mustDescriptor("domains").tid(name),
			p.Cat.DomainDoc.Sections, config.Sidecar{}, data,
			lay.DomainsDir+"/"+name+".md")
		if err != nil { // coverage-ignore: .data.domain/.data.decisions are always set and the template is embedded, so renderTarget cannot produce <no value> or a read error here
			return nil, err
		}
		out = append(out, RenderedFile{Path: rf.Path, Content: rf.Content})
	}
	return out, nil
}
