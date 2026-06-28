// Package project ties config, catalog, render, and manifest together to sync rendered files into a project and check them for drift.
package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/templates"
)

const Version = "0.1.0"

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

func sliceSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
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
	amd, err := p.generateActiveMD()
	if err != nil {
		return err
	}
	files = append(files, amd)
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
				_ = os.Remove(filepath.Dir(file)) // only succeeds if now empty
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
	return invariants.Check(p.decisionsDir(), p.Root, p.Cfg.Invariants)
}

// Audit runs the process-conformance audit (ADR-0017) over the branch range.
// baseOverride wins over the configured base branch when non-empty.
func (p *Project) Audit(baseOverride string) ([]audit.Finding, error) {
	s := audit.Resolve(p.Cfg.Audit)
	if baseOverride != "" {
		s.BaseBranch = baseOverride
	}
	lay := p.layout()
	generated := map[string]bool{}
	if lock, err := manifest.Load(p.lockPath()); err == nil {
		for path := range lock.Files {
			generated[path] = true
		}
	}
	return audit.Run(p.Root, audit.Inputs{
		Settings:          s,
		GeneratedPaths:    generated,
		ADRDir:            lay.ADRDir,
		ActiveMd:          lay.ActiveMd,
		PlansDir:          lay.PlansDir,
		ConfiguredDomains: p.Cfg.Domains,
		DomainsPartsDir:   ".awf/domains/parts",
	})
}
