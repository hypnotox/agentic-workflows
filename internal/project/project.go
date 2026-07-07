// Package project ties config, catalog, render, and manifest together to sync rendered files into a project and check them for drift.
package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
)

// Version is the awf release version — the single version authority
// (ADR-0049): gate comparisons, the lock stamp, the bootstrap pin, and the
// CLI output all read this const.
const Version = "0.10.0"

// minVersionBySchema maps each config-schema generation to the minimum
// project.Version allowed to render it; adding a migration without an entry
// here (and a matching const bump) fails the gate (ADR-0049 Decision 4).
var minVersionBySchema = map[int]string{
	6: "0.6.0",
}

type Project struct {
	Root    string
	Cfg     *config.Config
	Cat     *catalog.Catalog
	Targets []Target
	// effSkills is the effective rendered skill set (enabled minus doc-gate-
	// suppressed, local kept), populated by RenderAll; templates read it as
	// .skills and artifactConfigHash folds it in for .skills-referencing
	// templates (ADR-0046).
	effSkills map[string]bool
}

func Open(root string) (*Project, error) {
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	targets, err := resolveTargets(cfg.Targets)
	if err != nil {
		return nil, err
	}
	p := &Project{Root: root, Cfg: cfg, Targets: targets}
	cat, err := p.effectiveCatalog()
	if err != nil {
		return nil, err
	}
	p.Cat = cat
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

// SyncReport renders and writes the project, additionally backing up any
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
	// Local artifacts are not rendered (skipped by RenderAll), so their hand-authored
	// files never enter `want` above. Protect them from the prune below so converting
	// a managed skill/agent to local does not delete its file.
	localPaths, err := p.localTargetPaths()
	if err != nil { // coverage-ignore: checkLocalFrontmatter above already surfaced any malformed local sidecar
		return nil, err
	}
	for _, rel := range localPaths {
		want[rel] = true
	}
	// Prune files from the previous lock that are no longer produced, then remove
	// every directory left empty — walking all ancestors deepest-first, not just the
	// immediate parent, so dropping a target clears its whole tree (inv:
	// target-prune-ancestors; reuses Uninstall's idiom).
	// invariant: target-prune-ancestors
	if old != nil {
		dirs := map[string]bool{}
		for path := range old.Files {
			if want[path] {
				continue
			}
			// A non-local entry (corrupted or malicious lock) would delete outside
			// the root and send the ancestor walk below it, never reaching p.Root.
			if !filepath.IsLocal(filepath.FromSlash(path)) {
				continue
			}
			file := filepath.Join(p.Root, path)
			_ = os.Remove(file)
			for d := filepath.Dir(file); d != p.Root; d = filepath.Dir(d) {
				dirs[d] = true
			}
		}
		dirList := slices.Collect(maps.Keys(dirs))
		slices.SortFunc(dirList, func(a, b string) int { return len(b) - len(a) })
		for _, d := range dirList {
			_ = os.Remove(d) // removes only if now empty
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
	return config.LockPath(p.Root)
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
		DomainsPartsDir:   config.DirName + "/domains/parts",
		DomainsIndexDir:   lay.DomainsDir,
	})
}

// NewADR scaffolds a new ADR file under the project's decisions dir: the next
// sequential number, the rendered template with its title/date filled in and
// marker comments stripped, refusing to overwrite an existing file. Mirrors
// the CheckInvariants/Audit pattern — cmd/awf reaches this only through this
// exported method, never internal/project.Layout directly.
func (p *Project) NewADR(title string) (string, error) {
	return adr.NewFile(p.decisionsDir(), title)
}
