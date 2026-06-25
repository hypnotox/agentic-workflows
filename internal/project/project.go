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

	"agentic-workflows/internal/adr"
	"agentic-workflows/internal/catalog"
	"agentic-workflows/internal/config"
	"agentic-workflows/internal/frontmatter"
	"agentic-workflows/internal/invariants"
	"agentic-workflows/internal/manifest"
	"agentic-workflows/internal/render"
	"agentic-workflows/templates"
)

const Version = "0.1.0"

type RenderedFile struct {
	Path         string
	Content      string
	TemplateID   string
	TemplateHash string
}

type Project struct {
	Root string
	Cfg  *config.Config
	Cat  *catalog.Catalog
}

func Open(root string) (*Project, error) {
	cfg, err := config.Load(filepath.Join(root, ".claude", "awf.yaml"))
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cat, err := catalog.Load(templates.FS)
	if err != nil {
		return nil, err
	}
	p := &Project{Root: root, Cfg: cfg, Cat: cat}
	if err := p.validateAgainstCatalog(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Project) validateAgainstCatalog() error {
	// Check non-local skills against catalog.
	for _, name := range sortedKeys(p.Cfg.Skills) {
		sc := p.Cfg.Skills[name]
		if sc.Local {
			continue
		}
		spec, ok := p.Cat.Skills[name]
		if !ok {
			return fmt.Errorf("skill %q is not in the catalog", name)
		}
		if err := checkSectionsAllowed("skill", name, spec.Sections, sc.Sections); err != nil {
			return err
		}
	}
	// Check agents against catalog.
	for _, a := range sortedKeys(p.Cfg.Agents) {
		ac := p.Cfg.Agents[a]
		if ac.Local {
			continue
		}
		aspec, ok := p.Cat.Agents[a]
		if !ok {
			return fmt.Errorf("agent %q is not in the catalog", a)
		}
		if err := checkSectionsAllowed("agent", a, aspec.Sections, ac.Sections); err != nil {
			return err
		}
	}
	// Check hooks against catalog.
	catHooks := make(map[string]bool, len(p.Cat.Hooks))
	for _, h := range p.Cat.Hooks {
		catHooks[h] = true
	}
	for _, h := range p.Cfg.Hooks {
		if !catHooks[h] {
			return fmt.Errorf("hook %q is not in the catalog", h)
		}
	}
	// Check agentsDoc section overrides against catalog.
	if p.Cfg.AgentsDoc != nil && !p.Cfg.AgentsDoc.Local {
		if err := checkSectionsAllowed("agentsDoc", "", p.Cat.AgentsDoc.Sections, p.Cfg.AgentsDoc.Sections); err != nil {
			return err
		}
	}
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
	return nil
}

func (p *Project) parts() render.PartFunc {
	return func(name string) (string, error) {
		b, err := os.ReadFile(filepath.Join(p.Root, ".claude", "awf", name))
		return string(b), err
	}
}

func (p *Project) data(sc config.SkillConfig) map[string]any {
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
			"path":  p.docOutPath(name),
		})
	}
	return out
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
// singleton (e.g. agentsDoc).
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

func (p *Project) RenderAll() ([]RenderedFile, error) {
	var out []RenderedFile
	// Skills (sorted for deterministic order).
	for _, name := range sortedKeys(p.Cfg.Skills) {
		sc := p.Cfg.Skills[name]
		if sc.Local {
			continue
		}
		tid := fmt.Sprintf("skills/%s/SKILL.md.tmpl", name)
		rf, err := p.renderTemplate(tid, sc.Sections, p.data(sc),
			fmt.Sprintf(".claude/skills/%s-%s/SKILL.md", p.Cfg.Prefix, name))
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	// Agents (sorted for deterministic order).
	for _, name := range sortedKeys(p.Cfg.Agents) {
		ac := p.Cfg.Agents[name]
		if ac.Local {
			continue
		}
		tid := fmt.Sprintf("agents/%s.md.tmpl", name)
		rf, err := p.renderTemplate(tid, ac.Sections, p.data(ac),
			fmt.Sprintf(".claude/agents/%s.md", name))
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	// Hooks.
	for _, h := range p.Cfg.Hooks {
		tid := fmt.Sprintf("hooks/%s.tmpl", h)
		rf, err := p.renderTemplate(tid, nil, p.data(config.SkillConfig{}),
			".githooks/"+h)
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	// Docs.
	for _, name := range sortedKeys(p.Cfg.Docs) {
		dc := p.Cfg.Docs[name]
		if dc.Local {
			continue
		}
		tid := fmt.Sprintf("docs/%s.md.tmpl", name)
		rf, err := p.renderTemplate(tid, dc.Sections, p.data(dc), p.docOutPath(name))
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
}

func (p *Project) renderTemplate(tid string, sections map[string]config.SectionOverride, data map[string]any, outPath string) (RenderedFile, error) {
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("read template %s: %w", tid, err)
	}
	content, err := render.Render(string(src), sections, p.parts(), data)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	if strings.Contains(content, "<no value>") {
		return RenderedFile{}, fmt.Errorf("render %s: output contains \"<no value>\" — a referenced var or data key is unset in .claude/awf.yaml", outPath)
	}
	return RenderedFile{
		Path: outPath, Content: content, TemplateID: tid,
		TemplateHash: manifest.Hash(src),
	}, nil
}

// generateActiveMD renders the ADR index for the project's decisions directory,
// or returns nil when that directory holds no ADRs (so no index file is produced).
// ACTIVE.md is generated from ADR frontmatter, not a template, so it carries no
// TemplateID/TemplateHash in the lock.
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

func (p *Project) Sync() error {
	files, err := p.RenderAll()
	if err != nil {
		return err
	}
	for _, f := range files {
		if isSkillOrAgent(f.TemplateID) {
			if err := validateFrontmatter([]byte(f.Content)); err != nil {
				return fmt.Errorf("invalid frontmatter in %s: %w", f.Path, err)
			}
		}
	}
	if amd, ok, err := p.generateActiveMD(); err != nil {
		return err
	} else if ok {
		files = append(files, amd)
	}
	cfgHash := manifest.Hash(p.Cfg.Raw())
	lock := &manifest.Lock{AWFVersion: Version, Files: map[string]manifest.Entry{}}
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
			ConfigHash: cfgHash, OutputHash: manifest.Hash([]byte(f.Content)),
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
	return filepath.Join(p.Root, ".claude", "awf.lock")
}

// CheckInvariants reports Implemented-ADR invariant slugs that lack a backing
// `// invariant: <slug>` comment under the project root.
func (p *Project) CheckInvariants() ([]invariants.Finding, error) {
	return invariants.Check(filepath.Join(p.Root, p.Cfg.DocsDir, "decisions"), p.Root, p.Cfg.Invariants)
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
	cfgHash := manifest.Hash(p.Cfg.Raw())
	rendered := map[string]RenderedFile{}
	for _, f := range files {
		rendered[f.Path] = f
	}
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
	return drift, nil
}
