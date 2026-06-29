// Package project ties config, catalog, render, and manifest together to sync rendered files into a project and check them for drift.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/templates"
)

const Version = "0.3.0"

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

// Backup records a foreign file preserved before sync overwrote its path.
type Backup struct {
	Path  string // project-relative file that was overwritten
	Bak   string // project-relative backup copy (.awf-bak[.N])
	Index bool   // the file is the generated ADR/domain index (ownership-takeover note)
}

func (p *Project) Sync() error {
	_, err := p.SyncReport()
	return err
}

// SyncReport renders and writes the project like Sync, additionally backing up any
// foreign file (on disk but absent from the start-of-sync lock) before overwriting
// it, and returning those backups (ADR-0035).
func (p *Project) SyncReport() ([]Backup, error) {
	files, err := p.RenderAll()
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if isSkillOrAgent(f.TemplateID) {
			if err := validateFrontmatter([]byte(f.Content)); err != nil { // coverage-ignore: rendered catalog skill/agent frontmatter is template-fixed (non-empty name/description guaranteed by inv templates-valid-frontmatter); it cannot be invalid at sync time
				return nil, fmt.Errorf("invalid frontmatter in %s: %w", f.Path, err)
			}
		}
	}
	var localErr error
	if err := p.checkLocalFrontmatter(func(path string, e error) {
		if localErr == nil {
			localErr = fmt.Errorf("local target %s: %w", path, e)
		}
	}); err != nil { // coverage-ignore: checkLocalFrontmatter only errors on a malformed local-target sidecar, which RenderAll above already surfaces earlier in Sync
		return nil, err
	}
	if localErr != nil {
		return nil, localErr
	}
	amd, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	files = append(files, amd)
	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: unreachable — generateActiveMD above parses the same decisions dir and fails first on a malformed ADR
		return nil, err
	}
	files = append(files, dds...)

	// Prior lock, read before any write: membership decides foreign (back up) vs
	// awf-managed (overwrite silently), and drives pruning below.
	old, _ := manifest.Load(p.lockPath())
	prior := map[string]bool{}
	if old != nil {
		for path := range old.Files {
			prior[path] = true
		}
	}

	var backups []Backup
	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	want := map[string]bool{}
	for _, f := range files {
		abs := filepath.Join(p.Root, f.Path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, err
		}
		if !prior[f.Path] {
			if _, statErr := os.Stat(abs); statErr == nil {
				// invariant: sync-backs-up-foreign
				bak, err := p.BackupFile(f.Path)
				if err != nil { // coverage-ignore: BackupFile only fails on a copyFile permission fault that root bypasses
					return nil, fmt.Errorf("back up %s: %w", f.Path, err)
				}
				backups = append(backups, Backup{Path: f.Path, Bak: bak, Index: p.isGeneratedIndex(f.Path)})
			} else if !errors.Is(statErr, os.ErrNotExist) { // coverage-ignore: os.Stat returns a non-NotExist error only on a permission/IO fault that root bypasses
				return nil, statErr
			}
		}
		if err := os.WriteFile(abs, []byte(f.Content), 0o644); err != nil {
			return nil, err
		}
		lock.Files[f.Path] = manifest.Entry{
			TemplateID: f.TemplateID, TemplateHash: f.TemplateHash,
			ConfigHash: f.ConfigHash, OutputHash: manifest.Hash([]byte(f.Content)),
		}
		want[f.Path] = true
	}
	// Prune files from the previous lock that are no longer produced.
	if old != nil {
		for path := range old.Files {
			if !want[path] {
				file := filepath.Join(p.Root, path)
				_ = os.Remove(file)
				_ = os.Remove(filepath.Dir(file)) // only succeeds if now empty
			}
		}
	}
	return backups, lock.Save(p.lockPath())
}

// isGeneratedIndex reports whether rel is the generated ADR index or a per-domain
// index — the awf-owned generated docs whose first-time takeover warrants a note.
func (p *Project) isGeneratedIndex(rel string) bool {
	lay := p.layout()
	return rel == lay.ActiveMd || strings.HasPrefix(rel, lay.DomainsDir+"/")
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
		DomainsIndexDir:   lay.DomainsDir,
	})
}
