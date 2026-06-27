// Package project ties config, catalog, render, and manifest together to sync rendered files into a project and check them for drift.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"

	"gopkg.in/yaml.v3"
)

const Version = "0.1.0"

type RenderedFile struct {
	Path         string
	Content      string
	TemplateID   string
	TemplateHash string
	ConfigHash   string
}

type Project struct {
	Root   string
	Cfg    *config.Config
	Cat    *catalog.Catalog
	Target Target
}

func Open(root string) (*Project, error) {
	cfg, err := config.Load(filepath.Join(root, ".awf"))
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cat, err := catalog.Load(templates.FS)
	if err != nil { // coverage-ignore: catalog.Load over the embedded templates.FS cannot fail at runtime
		return nil, err
	}
	p := &Project{Root: root, Cfg: cfg, Cat: cat, Target: claudeTarget}
	if err := p.validateAgainstCatalog(); err != nil {
		return nil, err
	}
	return p, nil
}

// validateAgainstCatalog checks that every enabled non-local target is in the
// catalog and that its sidecar's section overrides name declared sections.
func (p *Project) validateAgainstCatalog() error {
	checkKind := func(kind string, names []string, specs func(string) ([]string, bool)) error {
		for _, name := range names {
			sc, err := p.Cfg.Sidecar(kind, name)
			if err != nil {
				return err
			}
			if sc.Local {
				continue
			}
			declared, ok := specs(name)
			if !ok {
				return fmt.Errorf("%s %q is not in the catalog", strings.TrimSuffix(kind, "s"), name)
			}
			if err := checkSectionsAllowed(kind, name, declared, sc.Sections); err != nil {
				return err
			}
		}
		return nil
	}
	if err := checkKind("skills", p.Cfg.Skills, func(n string) ([]string, bool) {
		s, ok := p.Cat.Skills[n]
		return s.Sections, ok
	}); err != nil {
		return err
	}
	if err := checkKind("agents", p.Cfg.Agents, func(n string) ([]string, bool) {
		a, ok := p.Cat.Agents[n]
		return a.Sections, ok
	}); err != nil {
		return err
	}
	if err := checkKind("docs", p.Cfg.Docs, func(n string) ([]string, bool) {
		d, ok := p.Cat.Docs[n]
		return d.Sections, ok
	}); err != nil {
		return err
	}
	// Hooks against catalog.
	catHooks := make(map[string]bool, len(p.Cat.Hooks))
	for _, h := range p.Cat.Hooks {
		catHooks[h] = true
	}
	for _, h := range p.Cfg.Hooks {
		if !catHooks[h] {
			return fmt.Errorf("hook %q is not in the catalog", h)
		}
	}
	// agents-doc section overrides against catalog (always-on singleton).
	ad, err := p.Cfg.Sidecar("agents-doc", "")
	if err != nil {
		return err
	}
	if !ad.Local {
		if err := checkSectionsAllowed("agents-doc", "", p.Cat.AgentsDoc.Sections, ad.Sections); err != nil {
			return err
		}
	}
	return nil
}

// parts resolves an absolute part path verbatim (convention parts pass an
// absolute path); a relative name resolves under <root>/.awf.
func (p *Project) data(sc config.Sidecar) map[string]any {
	return map[string]any{
		"prefix": p.Cfg.Prefix,
		"vars":   nonNil(p.Cfg.Vars),
		"data":   nonNil(sc.Data),
		"layout": p.layout(),
	}
}

// layout returns the fixed, awf-given docs layout derived from cfg.DocsDir.
// These paths are exposed to templates under the .layout namespace; they are
// not configurable through vars.
func (p *Project) layout() map[string]any {
	d := strings.TrimRight(p.Cfg.DocsDir, "/")
	dec := d + "/decisions"
	// docs maps every enabled doc name to its output path. Local docs are
	// included: the file still exists at that path and is citable. A key is
	// present iff the doc is enabled (inv: layout-docs-enabled-only).
	docs := map[string]any{}
	for _, name := range p.Cfg.Docs {
		docs[name] = p.docOutPath(name)
	}
	// workflowRef is the workflow doc's path when enabled, else AGENTS.md, so
	// the ~always-cited workflow reference always resolves (inv: workflow-ref-fallback).
	workflowRef := "AGENTS.md"
	if wp, ok := docs["workflow"]; ok {
		workflowRef = wp.(string)
	}
	return map[string]any{
		"docsDir":     d,
		"adrDir":      dec,
		"activeMd":    dec + "/ACTIVE.md",
		"adrReadme":   dec + "/README.md",
		"adrTemplate": dec + "/template.md",
		"plansDir":    d + "/plans",
		"docs":        docs,
		"workflowRef": workflowRef,
		"domainsDir":  d + "/domains", // inv: domains-dir-given
	}
}

// docOutPath is the output path for a managed doc, rooted at docsDir.
func (p *Project) docOutPath(name string) string {
	return strings.TrimRight(p.Cfg.DocsDir, "/") + "/" + name + ".md"
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
	if kind == "agents-doc" {
		return ".awf/parts/agents-doc/" + section + ".md"
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

// sortedKeys returns the keys of m in ascending sorted order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// checkSectionsAllowed verifies that every key in used appears in declared.
// kind and name are used only for error formatting; name may be empty for a
// singleton (e.g. agents-doc).
func checkSectionsAllowed(kind, name string, declared []string, used map[string]config.SectionOverride) error {
	allowed := make(map[string]bool, len(declared))
	for _, s := range declared {
		allowed[s] = true
	}
	label := kind
	if name != "" {
		label = fmt.Sprintf("%s %q", kind, name)
	}
	for sec := range used {
		if !allowed[sec] {
			return fmt.Errorf("%s: unknown section %q (not declared in the catalog)", label, sec)
		}
	}
	return nil
}

func nonNil(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

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

// validateFrontmatter checks that content has parseable frontmatter with a
// non-empty name and description.
func validateFrontmatter(content []byte) error {
	var fm skillFrontmatter
	_, found, err := frontmatter.Parse(content, &fm)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("missing frontmatter")
	}
	if strings.TrimSpace(fm.Name) == "" {
		return errors.New("frontmatter name is empty")
	}
	if strings.TrimSpace(fm.Description) == "" {
		return errors.New("frontmatter description is empty")
	}
	return nil
}

// localOutPath returns the conventional output path awf would render a local
// skill/agent to (the same formulas RenderAll uses).
func (p *Project) localOutPath(kind, name string) string {
	switch kind {
	case "skills":
		return p.Target.SkillPath(p.Cfg.Prefix, name)
	case "agents":
		return p.Target.AgentPath(name)
	default:
		return ""
	}
}

func (p *Project) RenderAll() ([]RenderedFile, error) {
	var out []RenderedFile
	// Skills.
	enabledDocs := sliceSet(p.Cfg.Docs)
	for _, name := range sortedStrings(p.Cfg.Skills) {
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
	for _, name := range sortedStrings(p.Cfg.Agents) {
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
	for _, name := range sortedStrings(p.Cfg.Docs) {
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
	if amd, ok, err := p.generateActiveMD(); err != nil {
		return nil, err
	} else if ok {
		paths = append(paths, amd.Path)
	}
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
	proj := map[string]any{"prefix": p.Cfg.Prefix, "layout": p.layout()}
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

func sortedStrings(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

// generateActiveMD renders the ADR index for the project's decisions directory,
// or returns nil when that directory holds no ADRs (so no index file is produced).
func (p *Project) generateActiveMD() (RenderedFile, bool, error) {
	dir := filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")
	content, err := adr.RenderActiveMD(dir)
	if err != nil {
		return RenderedFile{}, false, err
	}
	if content == "" {
		return RenderedFile{}, false, nil
	}
	return RenderedFile{Path: strings.TrimRight(p.Cfg.DocsDir, "/") + "/decisions/ACTIVE.md", Content: content}, true, nil
}

// generateDomainDocs renders one content-only doc per declared domain
// (<docsDir>/domains/<name>.md): the domain template + its convention parts, with
// the per-domain ADR index injected as .data.decisions. Like ACTIVE.md, the result
// carries no TemplateID/Hash — drift is checked by regeneration, since the index
// depends on external ADR frontmatter state.
func (p *Project) generateDomainDocs() ([]RenderedFile, error) {
	decisionsDir := filepath.Join(p.Root, p.Cfg.DocsDir, "decisions")
	var out []RenderedFile
	for _, name := range sortedStrings(p.Cfg.Domains) {
		index, err := adr.RenderDomainIndex(decisionsDir, name)
		if err != nil {
			return nil, err
		}
		data := p.data(config.Sidecar{})
		data["data"] = map[string]any{"domain": name, "decisions": index}
		rf, err := p.renderTarget("domains", name, "domains/domain.md.tmpl",
			p.Cat.DomainDoc.Sections, config.Sidecar{}, data,
			strings.TrimRight(p.Cfg.DocsDir, "/")+"/domains/"+name+".md")
		if err != nil { // coverage-ignore: .data.domain/.data.decisions are always set and the template is embedded, so renderTarget cannot produce <no value> or a read error here
			return nil, err
		}
		out = append(out, RenderedFile{Path: rf.Path, Content: rf.Content})
	}
	return out, nil
}

// checkLocalFrontmatter validates the on-disk frontmatter of every declared local
// skill/agent at its conventional output path. fail wraps a path+error into the
// caller's accumulator (a hard error for Sync, a drift entry for Check).
func (p *Project) checkLocalFrontmatter(fail func(path string, err error)) error {
	for _, kv := range []struct {
		kind  string
		names []string
	}{{"skills", p.Cfg.Skills}, {"agents", p.Cfg.Agents}} {
		for _, name := range kv.names {
			sc, err := p.Cfg.Sidecar(kv.kind, name)
			if err != nil {
				return err
			}
			if !sc.Local {
				continue
			}
			rel := p.localOutPath(kv.kind, name)
			b, err := os.ReadFile(filepath.Join(p.Root, rel))
			if err != nil {
				fail(rel, fmt.Errorf("local %s file absent", strings.TrimSuffix(kv.kind, "s")))
				continue
			}
			if err := validateFrontmatter(b); err != nil {
				fail(rel, err)
			}
		}
	}
	return nil
}

func (p *Project) Sync() error {
	files, err := p.RenderAll()
	if err != nil {
		return err
	}
	for _, f := range files {
		if isSkillOrAgent(f.TemplateID) {
			if err := validateFrontmatter([]byte(f.Content)); err != nil { // coverage-ignore: rendered catalog skill/agent frontmatter is template-fixed (non-empty name/description guaranteed by inv templates-valid-frontmatter); it cannot be invalid at sync time
				return fmt.Errorf("invalid frontmatter in %s: %w", f.Path, err)
			}
		}
	}
	var localErr error
	if err := p.checkLocalFrontmatter(func(path string, e error) {
		if localErr == nil {
			localErr = fmt.Errorf("local target %s: %w", path, e)
		}
	}); err != nil { // coverage-ignore: checkLocalFrontmatter only errors on a malformed local-target sidecar, which RenderAll above already surfaces earlier in Sync
		return err
	}
	if localErr != nil {
		return localErr
	}
	if amd, ok, err := p.generateActiveMD(); err != nil {
		return err
	} else if ok {
		files = append(files, amd)
	}
	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: unreachable — generateActiveMD above parses the same decisions dir and fails first on a malformed ADR
		return err
	}
	files = append(files, dds...)
	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	want := map[string]bool{}
	for _, f := range files {
		abs := filepath.Join(p.Root, f.Path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if filepath.Dir(f.Path) == ".githooks" {
			mode = 0o755
		}
		if err := os.WriteFile(abs, []byte(f.Content), mode); err != nil {
			return err
		}
		lock.Files[f.Path] = manifest.Entry{
			TemplateID: f.TemplateID, TemplateHash: f.TemplateHash,
			ConfigHash: f.ConfigHash, OutputHash: manifest.Hash([]byte(f.Content)),
		}
		want[f.Path] = true
	}
	// Prune files from the previous lock that are no longer produced.
	if old, err := manifest.Load(p.lockPath()); err == nil {
		for path := range old.Files {
			if !want[path] {
				file := filepath.Join(p.Root, path)
				_ = os.Remove(file)
				parentDir := filepath.Dir(file)
				os.Remove(parentDir) // ignore error - only removes if empty
			}
		}
	}
	return lock.Save(p.lockPath())
}

func (p *Project) lockPath() string {
	return filepath.Join(p.Root, ".awf", "awf.lock")
}

// CheckInvariants reports Implemented-ADR invariant slugs that lack a backing
// `<marker> invariant: <slug>` comment (per the project's configured invariant
// sources) under the project root.
func (p *Project) CheckInvariants() ([]invariants.Finding, error) {
	return invariants.Check(filepath.Join(p.Root, p.Cfg.DocsDir, "decisions"), p.Root, p.Cfg.Invariants)
}

// Audit runs the process-conformance audit (ADR-0017) over the branch range.
// baseOverride wins over the configured base branch when non-empty.
func (p *Project) Audit(baseOverride string) ([]audit.Finding, error) {
	base, types, scopes, manifests, subjectMax, threshold, domStale, undoc := p.Cfg.AuditSettings()
	if baseOverride != "" {
		base = baseOverride
	}
	lay := p.layout()
	generated := map[string]bool{}
	if lock, err := manifest.Load(p.lockPath()); err == nil {
		for path := range lock.Files {
			generated[path] = true
		}
	}
	return audit.Run(p.Root, audit.Inputs{
		BaseBranch:          base,
		AllowedTypes:        types,
		AllowedScopes:       scopes,
		SubjectMaxLength:    subjectMax,
		DependencyManifests: manifests,
		DiffThreshold:       threshold,
		GeneratedPaths:      generated,
		ADRDir:              lay["adrDir"].(string),
		ActiveMd:            lay["activeMd"].(string),
		PlansDir:            lay["plansDir"].(string),
		ConfiguredDomains:   p.Cfg.Domains,
		DomainsPartsDir:     ".awf/domains/parts",
		DomainDocStaleness:  domStale,
		UndocumentedDomain:  undoc,
	})
}

// orphans reports sidecar and convention-part files whose target is not in the
// matching enable list, plus convention-part files of an enabled target whose
// section is not catalog-declared (inv: drift-source-set; ADR-0011 section-orphan-flagged).
func (p *Project) orphans() []manifest.Drift {
	enabled := map[string]map[string]bool{
		"skills":  sliceSet(p.Cfg.Skills),
		"agents":  sliceSet(p.Cfg.Agents),
		"docs":    sliceSet(p.Cfg.Docs),
		"domains": sliceSet(p.Cfg.Domains),
	}
	var drift []manifest.Drift
	for _, kind := range []string{"skills", "agents", "docs", "domains"} {
		base := filepath.Join(p.Root, ".awf", kind)
		// Sidecars: <kind>/<name>.yaml.
		entries, err := os.ReadDir(base)
		if err != nil {
			continue // kind branch absent → nothing to orphan
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".yaml")
			if !enabled[kind][name] {
				drift = append(drift, manifest.Drift{
					Path: filepath.Join(".awf", kind, e.Name()),
					Kind: "orphaned", Detail: "sidecar for a target not in the enable list",
				})
			}
		}
		// Parts: <kind>/parts/<target>/<section>.md.
		partsDir := filepath.Join(base, "parts")
		targets, err := os.ReadDir(partsDir)
		if err != nil {
			continue
		}
		for _, t := range targets {
			if !t.IsDir() {
				continue
			}
			if !enabled[kind][t.Name()] {
				drift = append(drift, manifest.Drift{
					Path: filepath.Join(".awf", kind, "parts", t.Name()),
					Kind: "orphaned", Detail: "convention parts for a target not in the enable list",
				})
				continue
			}
			// Enabled target: flag part files whose section is not catalog-declared.
			declared := sliceSet(p.declaredSections(kind, t.Name()))
			sections, err := os.ReadDir(filepath.Join(partsDir, t.Name()))
			if err != nil { // coverage-ignore: os.ReadDir on an enabled target's existing parts directory fails only on a permission fault (a no-op as root)
				continue
			}
			for _, sf := range sections {
				if sf.IsDir() || !strings.HasSuffix(sf.Name(), ".md") {
					continue
				}
				if section := strings.TrimSuffix(sf.Name(), ".md"); !declared[section] {
					drift = append(drift, manifest.Drift{
						Path: filepath.Join(".awf", kind, "parts", t.Name(), sf.Name()),
						Kind: "orphaned", Detail: "convention part for a section not in the target's declared set",
					})
				}
			}
		}
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i].Path < drift[j].Path })
	return drift
}

// declaredSections returns the catalog-declared section names for a target.
func (p *Project) declaredSections(kind, name string) []string {
	switch kind {
	case "skills":
		return p.Cat.Skills[name].Sections
	case "agents":
		return p.Cat.Agents[name].Sections
	case "docs":
		return p.Cat.Docs[name].Sections
	case "domains":
		return p.Cat.DomainDoc.Sections
	}
	return nil
}

func sliceSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}

func (p *Project) Check() ([]manifest.Drift, error) {
	lock, err := manifest.Load(p.lockPath())
	if err != nil {
		return nil, fmt.Errorf("no lock (run awf sync): %w", err)
	}
	files, err := p.RenderAll()
	if err != nil {
		return nil, err
	}
	rendered := map[string]RenderedFile{}
	for _, f := range files {
		rendered[f.Path] = f
	}
	activeMdRel := strings.TrimRight(p.Cfg.DocsDir, "/") + "/decisions/ACTIVE.md"
	domainsPrefix := strings.TrimRight(p.Cfg.DocsDir, "/") + "/domains/"
	var drift []manifest.Drift
	for _, path := range sortedKeys(lock.Files) {
		if path == activeMdRel || strings.HasPrefix(path, domainsPrefix) {
			continue // generated artifacts — checked separately below
		}
		e := lock.Files[path]
		rf, ok := rendered[path]
		if !ok {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "in lock but no longer produced"})
			continue
		}
		if rf.TemplateHash != e.TemplateHash || rf.ConfigHash != e.ConfigHash {
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
	// Local skills/agents are not rendered, so their hand-authored frontmatter is
	// validated directly on disk.
	if err := p.checkLocalFrontmatter(func(path string, e error) {
		drift = append(drift, manifest.Drift{Path: path, Kind: "invalid-frontmatter", Detail: e.Error()})
	}); err != nil { // coverage-ignore: checkLocalFrontmatter only errors on a malformed local-target sidecar, which RenderAll above already surfaces earlier in Check
		return nil, err
	}
	// Orphan sidecars/parts (second clause of inv: drift-source-set).
	drift = append(drift, p.orphans()...)
	// ACTIVE.md is generated from ADR frontmatter, not a template, so its staleness
	// cannot be detected by the template/config hash comparison above. Regenerate and
	// compare directly.
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
	// Domain docs are generated from ADR frontmatter + convention parts, so like
	// ACTIVE.md their staleness cannot be detected by the lock hash. Regenerate and
	// compare each; flag a lock entry no longer produced (domain removed).
	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: unreachable — the ACTIVE.md regenerate above parses the same decisions dir and fails first on a malformed ADR
		return nil, err
	}
	produced := map[string]bool{}
	for _, dd := range dds {
		produced[dd.Path] = true
		onDisk, err := os.ReadFile(filepath.Join(p.Root, dd.Path))
		if err != nil {
			drift = append(drift, manifest.Drift{Path: dd.Path, Kind: "missing", Detail: "domain doc absent; run awf sync"})
		} else if manifest.Hash(onDisk) != manifest.Hash([]byte(dd.Content)) {
			drift = append(drift, manifest.Drift{Path: dd.Path, Kind: "stale", Detail: "domain doc out of date; run awf sync"})
		}
	}
	for _, path := range sortedKeys(lock.Files) {
		if strings.HasPrefix(path, domainsPrefix) && !produced[path] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "domain removed; run awf sync"})
		}
	}
	return drift, nil
}
