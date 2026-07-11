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
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

// Version is the awf release version — the single version authority
// (ADR-0049): gate comparisons, the lock stamp, the bootstrap pin, and the
// CLI output all read this const.
const Version = "0.15.0"

// minVersionBySchema maps each config-schema generation to the minimum
// project.Version allowed to render it; adding a migration without an entry
// here (and a matching const bump) fails the gate (ADR-0049 Decision 4).
var minVersionBySchema = map[int]string{
	6: "0.6.0",
	7: "0.11.0",
	8: "0.12.0",
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

// Backup records a foreign file preserved before sync overwrote its path.
type Backup struct {
	Path  string // project-relative file that was overwritten
	Bak   string // project-relative backup copy (.awf-bak[.N])
	Index bool   // the file is the generated ADR/domain index (ownership-takeover note)
}

// Change records a sync-written file whose rendered output differs from the
// prior lock's, with the cause the lock's hashes can attribute: "template"
// (the upstream template source moved), "config" (the project's effective
// inputs — vars, sidecar, parts — moved), "template+config" (both),
// "internal" (hashes unmoved: a non-hashed input such as the binary's version
// stamp), "regenerated" (a generated index, which carries no hashes to
// attribute), or "added" (no prior entry). The provenance triage signal for
// reviewing a large sync diff — upstream churn vs the project's own inputs.
type Change struct {
	Path  string
	Cause string
}

// SyncReport renders and writes the project, additionally backing up any
// foreign file (on disk but absent from the start-of-sync lock) before overwriting
// it, and returning those backups (ADR-0035) plus the per-file provenance of
// output that changed against the prior lock and the lock-relative paths of the
// files its prune actually removed (both path-sorted; a file whose output is
// byte-identical, and a first sync with no prior lock, report no change — a
// routine re-sync stays silent).
func (p *Project) SyncReport() ([]Backup, []Change, []string, error) {
	// Refuse before rendering or writing anything: a corrupt lock must never
	// produce a backup, skip a prune, or be overwritten (ADR-0076 Decision 2).
	// invariant: corrupt-lock-refuses
	old, _, err := manifest.LoadOptional(p.lockPath())
	if err != nil {
		return nil, nil, nil, err
	}
	files, err := p.RenderAll()
	if err != nil {
		return nil, nil, nil, err
	}
	for _, f := range files {
		if isSkillOrAgent(f.TemplateID) {
			if err := validateFrontmatter([]byte(f.Content)); err != nil { // coverage-ignore: rendered catalog skill/agent frontmatter is template-fixed (non-empty name/description guaranteed by inv templates-valid-frontmatter); it cannot be invalid at sync time
				return nil, nil, nil, fmt.Errorf("invalid frontmatter in %s: %w", f.Path, err)
			}
		}
	}
	var localErr error
	if err := p.checkLocalFrontmatter(func(path string, e error) {
		if localErr == nil {
			localErr = fmt.Errorf("local target %s: %w", path, e)
		}
	}); err != nil { // coverage-ignore: checkLocalFrontmatter only errors on a malformed local-target sidecar, which RenderAll above already surfaces earlier in Sync
		return nil, nil, nil, err
	}
	if localErr != nil {
		return nil, nil, nil, localErr
	}
	amd, err := p.generateActiveMD()
	if err != nil {
		return nil, nil, nil, err
	}
	rfs := files // the RenderAll set — the consumption input for the config reference
	files = append(files, amd)
	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: unreachable — generateActiveMD above parses the same decisions dir and fails first on a malformed ADR
		return nil, nil, nil, err
	}
	files = append(files, dds...)
	cref, ok, err := p.generateConfigReference(slices.Concat(rfs, dds))
	if err != nil { // reachable: the intro part is read here for the first time (RenderAll never renders the reference)
		return nil, nil, nil, err
	}
	if ok {
		files = append(files, *cref)
	}

	// Prior lock, read before any write (top of this func): membership decides
	// foreign (back up) vs awf-managed (overwrite silently), and drives pruning.
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
			return nil, nil, nil, err
		}
		if !prior[f.Path] {
			if _, statErr := os.Stat(abs); statErr == nil {
				// invariant: sync-backs-up-foreign
				bak, err := p.BackupFile(f.Path)
				if err != nil { // coverage-ignore: BackupFile only fails on a copyFile permission fault that root bypasses
					return nil, nil, nil, fmt.Errorf("back up %s: %w", f.Path, err)
				}
				backups = append(backups, Backup{Path: f.Path, Bak: bak, Index: p.isGeneratedIndex(f.Path)})
			} else if !errors.Is(statErr, os.ErrNotExist) { // coverage-ignore: os.Stat returns a non-NotExist error only on a permission/IO fault that root bypasses
				return nil, nil, nil, statErr
			}
		}
		if err := os.WriteFile(abs, []byte(f.Content), 0o644); err != nil {
			return nil, nil, nil, err
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
		return nil, nil, nil, err
	}
	for _, rel := range localPaths {
		want[rel] = true
	}
	// Prune files from the previous lock that are no longer produced, then remove
	// every directory left empty — walking all ancestors deepest-first, not just the
	// immediate parent, so dropping a target clears its whole tree (inv:
	// target-prune-ancestors; reuses Uninstall's idiom).
	// invariant: target-prune-ancestors
	var pruned []string
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
			// Report only an actual removal — a path whose file is already gone
			// must not be claimed pruned.
			if os.Remove(file) == nil {
				pruned = append(pruned, path)
			}
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
	slices.Sort(pruned) // lock-map iteration order is random; output must not be
	// Provenance: classify each written file whose output moved against the
	// prior lock, from the final lock state (one entry per path by
	// construction). A first sync has no baseline — report nothing rather
	// than flood a fresh adoption with "added" lines.
	var changes []Change
	if old != nil {
		for path, e := range lock.Files {
			oldE, ok := old.Files[path]
			if !ok {
				changes = append(changes, Change{Path: path, Cause: "added"})
				continue
			}
			if e.OutputHash == oldE.OutputHash {
				continue
			}
			tMoved, cMoved := e.TemplateHash != oldE.TemplateHash, e.ConfigHash != oldE.ConfigHash
			var cause string
			switch {
			case tMoved && cMoved:
				cause = "template+config"
			case tMoved:
				cause = "template"
			case cMoved:
				cause = "config"
			case e.TemplateHash == "":
				// Generated indexes carry no hashes to attribute; their
				// inputs are the scanned decision records.
				cause = "regenerated"
			default:
				// Real hashes, neither moved: a non-hashed input such as
				// the binary's version stamp.
				cause = "internal"
			}
			changes = append(changes, Change{Path: path, Cause: cause})
		}
		slices.SortFunc(changes, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	}
	return backups, changes, pruned, lock.Save(p.lockPath())
}

// isGeneratedIndex reports whether rel is the generated ADR index, a per-domain
// index, or the generated config reference — the awf-owned generated docs whose
// first-time takeover warrants a note.
func (p *Project) isGeneratedIndex(rel string) bool {
	lay := p.layout()
	return rel == lay.ActiveMd || strings.HasPrefix(rel, lay.DomainsDir+"/") || rel == p.crefRel()
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
	lock, _, err := manifest.LoadOptional(p.lockPath())
	if err != nil {
		return nil, err
	}
	if lock != nil {
		for path := range lock.Files {
			generated[path] = true
		}
	}
	domainPaths := map[string][]string{}
	for _, d := range p.Cfg.Domains {
		sc, err := p.Cfg.Sidecar("domains", d)
		if err != nil {
			return nil, err
		}
		for _, g := range sc.Paths {
			if err := pathglob.Validate(g); err != nil {
				return nil, fmt.Errorf("domain %q paths: %w", d, err)
			}
		}
		if len(sc.Paths) > 0 {
			domainPaths[d] = sc.Paths
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
		DomainPaths:       domainPaths,
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
