// Package project ties config, catalog, render, and manifest together to sync rendered files into a project and check them for drift.
package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agentic-workflows/internal/catalog"
	"agentic-workflows/internal/config"
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
	}
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
	// AgentsDoc.
	if p.Cfg.AgentsDoc != nil && !p.Cfg.AgentsDoc.Local {
		rf, err := p.renderTemplate("agents-doc/AGENTS.md.tmpl", p.Cfg.AgentsDoc.Sections, p.data(*p.Cfg.AgentsDoc), "AGENTS.md")
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

func (p *Project) Sync() error {
	files, err := p.RenderAll()
	if err != nil {
		return err
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
	var drift []manifest.Drift
	for _, path := range sortedKeys(lock.Files) {
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
		}
	}
	return drift, nil
}
