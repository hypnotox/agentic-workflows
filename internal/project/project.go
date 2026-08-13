// Package project ties config, catalog, render, and manifest together to sync rendered files into a project and check them for drift.
package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"golang.org/x/mod/semver"
)

// Version is the awf release version - the single version authority
// (ADR-0049): gate comparisons, the lock stamp, the bootstrap pin, and the
// CLI output all read this const.
const Version = "0.37.0"

// BridgeTrancheComplete blocks publication while the two-plan current-state
// bridge tranche is only partially implemented. Plans 1 and 2 have both landed
// (migration readiness, attestation, and ordinary-command refusal are all
// present), so the tranche is complete and publication is unblocked.
const BridgeTrancheComplete = true

// minVersionBySchema maps each config-schema generation to the minimum
// project.Version allowed to render it; adding a migration without an entry
// here (and a matching const bump) fails the gate (ADR-0049 Decision 4).
var minVersionBySchema = map[int]string{
	6:  "0.6.0",
	7:  "0.11.0",
	8:  "0.12.0",
	9:  "0.17.0",
	10: "0.17.0",
	11: "0.17.0",
	12: "0.17.0",
	13: "0.17.0",
	14: "0.18.0",
	15: "0.20.0",
	16: "0.21.0",
	17: "0.22.0",
	18: "0.22.0",
	19: "0.23.0",
	20: "0.24.0",
	21: "0.25.0",
	22: "0.26.0",
	23: "0.27.0",
	24: "0.28.0",
	25: "0.29.0",
	26: "0.30.0",
	27: "0.30.0",
	28: "0.30.0",
	29: "0.30.0",
	30: "0.30.0",
	31: "0.30.0",
	32: "0.30.0",
	33: "0.30.0",
	34: "0.30.0",
	35: "0.30.0",
	36: "0.31.0",
	37: "0.31.0",
	38: "0.31.0",
	39: "0.31.0",
	40: "0.32.0",
	41: "0.33.0",
	42: "0.33.0",
	43: "0.34.0",
	44: "0.35.1",
	45: "0.37.0",
}

// ValidateSchemaMinimumVersion confirms that version is new enough to render a
// schema generation. The command gate calls it for the current generation, so
// registering a migration without its release mapping fails before rendering.
func ValidateSchemaMinimumVersion(schema int, version string) error {
	minimum, found := minVersionBySchema[schema]
	if !found {
		return fmt.Errorf("schema generation %d has no minimum awf version", schema)
	}
	if semver.Compare("v"+version, "v"+minimum) < 0 {
		return fmt.Errorf("awf %s cannot render schema generation %d; requires awf %s or newer", version, schema, minimum)
	}
	return nil
}

// LoadConfigTree loads one project's configuration tree.
type LoadConfigTree func(string) (*config.Config, error)

// ResolveResidentRoot maps an invoking checkout to the root that owns resident
// state. It takes the operation's context because the resolution reaches Git.
type ResolveResidentRoot func(context.Context, string) string

// Loader owns project-opening policy over explicitly selected dependencies.
type Loader struct {
	loadConfigTree      LoadConfigTree
	standard            *catalog.Catalog
	resolveResidentRoot ResolveResidentRoot
	repo                *awfgit.Repo
}

// NewLoader constructs project-opening policy with its required composed Git
// handle. A nil handle is always a composition error.
func NewLoader(loadConfigTree LoadConfigTree, standard *catalog.Catalog, resolveResidentRoot ResolveResidentRoot, repo *awfgit.Repo) *Loader {
	loader := newLoader(loadConfigTree, standard, resolveResidentRoot, repo)
	if repo == nil {
		panic("project Loader: missing git repository dependency")
	}
	return loader
}

// NewLoaderWithoutRepository is the explicit fresh-adoption path for a tree
// known not to be a repository.
func NewLoaderWithoutRepository(loadConfigTree LoadConfigTree, standard *catalog.Catalog, resolveResidentRoot ResolveResidentRoot) *Loader {
	return newLoader(loadConfigTree, standard, resolveResidentRoot, nil)
}

func newLoader(loadConfigTree LoadConfigTree, standard *catalog.Catalog, resolveResidentRoot ResolveResidentRoot, repo *awfgit.Repo) *Loader {
	if loadConfigTree == nil {
		panic("project Loader: missing load config tree dependency")
	}
	if standard == nil {
		panic("project Loader: missing standard catalog dependency")
	}
	if resolveResidentRoot == nil {
		panic("project Loader: missing resolve resident root dependency")
	}
	return &Loader{loadConfigTree: loadConfigTree, standard: standard, resolveResidentRoot: resolveResidentRoot, repo: repo}
}

type Project struct {
	// Root is the invoking checkout and remains the sole tracked-config authority.
	Root string
	// roots anchors output resolution: its tracked half mirrors Root, its
	// resident half is the primary checkout selected by Git's common control
	// root. Non-Git fixture projects retain Root so ordinary config-only tests
	// remain useful. Constructed once, where the project is.
	roots    resident.Roots
	Cfg      *config.Config
	Cat      *catalog.Catalog
	Targets  []Target
	standard *catalog.Catalog
	// read selects an immutable project-tree universe for render inputs. A nil
	// reader means ordinary filesystem rendering.
	read ProjectTreeReader
	// nested records that Root is an adopted subtree of the containing Git
	// repository, whose resident outputs live outside the subtree index.
	nested bool
	// repo is the Git handle selected at the composition root and written once
	// here, nil when the project tree carries no repository.
	repo *awfgit.Repo
}

// gitRepo returns the handle this project reads Git through, or the reason it
// has none. Opening is never retried per operation: the handle is chosen once,
// at construction, and this is the single place that reports its absence.
func (p *Project) gitRepo() (*awfgit.Repo, error) {
	if p.repo == nil {
		return nil, fmt.Errorf("%s: %w", p.Root, awfgit.ErrNotARepository)
	}
	return p.repo, nil
}

// Open is the transitional compatibility entry point for callers not yet
// migrated to outer composition. New code composes a Loader explicitly.
func Open(ctx context.Context, root string) (*Project, error) {
	repo, _, err := awfgit.OpenContaining(root)
	if err != nil && !errors.Is(err, awfgit.ErrNotARepository) {
		return nil, err
	}
	if repo == nil {
		return NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot).Open(ctx, root)
	}
	return NewLoader(config.Load, catalog.Standard, awfgit.ProjectResidentRoot, repo).Open(ctx, root)
}

// Open loads, validates, and derives one project with the Loader's dependencies.
func (l *Loader) Open(ctx context.Context, root string) (*Project, error) {
	cfg, err := l.loadConfigTree(config.RootDir(root))
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, errors.New("project Loader: load config tree returned nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := catalog.ValidateWorkflowProfiles(l.standard); err != nil {
		return nil, err
	}
	targets, err := resolveTargets(KnownTargets())
	if err != nil { // coverage-ignore: configured-target validation succeeded and KnownTargets is exhaustively backed by built-in descriptor tests
		return nil, err
	}
	p := &Project{
		Root:     root,
		roots:    resident.NewRoots(root, l.resolveResidentRoot(ctx, root)),
		Cfg:      cfg,
		Targets:  targets,
		standard: l.standard,
		repo:     l.repo,
	}
	p.Cat = l.standard
	if err := p.validateAgainstCatalog(); err != nil {
		return nil, err
	}
	return p, nil
}

// workingTree snapshots the project's working universe through its handle.
func (p *Project) workingTree(ctx context.Context) (*snapshot.Tree, error) {
	repo, err := p.gitRepo()
	if err != nil {
		return nil, err
	}
	return snapshot.WorkingTree(ctx, repo)
}

// indexTree snapshots the project's staged universe through its handle.
func (p *Project) indexTree(ctx context.Context) (*snapshot.Tree, error) {
	repo, err := p.gitRepo()
	if err != nil {
		return nil, err
	}
	return snapshot.IndexTree(ctx, repo)
}

// openRootProject composes the minimal project the staged entry points read
// through: a root plus the handle whose index they snapshot. They deliberately
// never load working-tree configuration, so no Loader is involved and the only
// dependency they need is the handle.
func openRootProject(root string) (*Project, error) {
	repo, prefix, err := awfgit.OpenContaining(root)
	if err != nil {
		return nil, err
	}
	return &Project{Root: root, roots: resident.NewRoots(root, ""), standard: catalog.Standard, nested: prefix != "", repo: repo}, nil
}

// Backup records a foreign file preserved before sync overwrote its path.
type Backup struct {
	Path  string // project-relative file that was overwritten
	Bak   string // project-relative backup copy (.awf-bak[.N])
	Index bool   // the file is the generated ADR/domain index (ownership-takeover note)
}

// Change records a sync-written file whose rendered bytes differ from the
// prior lock's or whose required mode was corrected, with the cause the lock's
// hashes able to attribute: "template"
// (the upstream template source moved), "config" (the project's effective
// inputs - vars, sidecar, parts - moved), "template+config" (both),
// "internal" (hashes unmoved: a non-hashed input such as the binary's version
// stamp), "regenerated" (a generated index, which carries no hashes to
// attribute), or "added" (no prior entry). The provenance triage signal for
// reviewing a large sync diff - upstream churn vs the project's own inputs.
type Change struct {
	Path  string
	Cause string
}

// SyncReport renders and writes the project, additionally backing up any
// foreign file (on disk but absent from the start-of-sync lock) before overwriting
// it - plus a pruned co-owned runner before removing it (ADR-0156 item 9) -
// and returning those backups (ADR-0035) plus the per-file provenance of
// output that changed against the prior lock and the lock-relative paths of the
// files its prune actually removed (both path-sorted; a file whose output is
// byte-identical, and first-adoption initialization with no prior lock reports
// no change - a routine re-sync stays silent).
func (p *Project) SyncReport(ctx context.Context) ([]Backup, []Change, []string, error) {
	// Refuse an unresolvable hook-command wiring before rendering anything
	// (ADR-0156 Decision 5); first-adoption InitializeReport stays exempt.
	if err := validateCommandWiring(p.Cfg); err != nil {
		return nil, nil, nil, err
	}
	return p.syncReport(ctx, nil)
}

// InitAuthority is the explicit provenance supplied only by first adoption.
type InitAuthority struct {
	InitializedWithVersion string
}

// InitializeReport renders a first adoption while sealing its existing ADR
// identities. It has the same reporting contract as SyncReport.
func (p *Project) InitializeReport(ctx context.Context, seed InitAuthority) ([]Backup, []Change, []string, error) {
	return p.syncReport(ctx, &seed)
}

// syncFilesystem is sync's cohesive, root-confined filesystem dependency.
type syncFilesystem interface {
	MkdirAll(string, fs.FileMode) error
	Chmod(string, fs.FileMode) error
	Publish(string, []byte, fs.FileMode) error
	Replace(string, []byte, fs.FileMode) error
	Remove(string) error
	Read(string) ([]byte, error)
	ReadWithMode(string) ([]byte, fs.FileMode, error)
	LinkInfo(string) (fs.FileInfo, error)
}

type syncFilesystems struct {
	tracked  syncFilesystem
	resident syncFilesystem
}

func (s syncFilesystems) output(rel string) (syncFilesystem, string) {
	if resident.IsResidentPath(rel) {
		return s.resident, rel
	}
	return s.tracked, rel
}

func (p *Project) openSyncFilesystems() (syncFilesystems, func(), error) {
	tracked, err := filesystem.Open(p.roots.Tracked)
	if err != nil {
		return syncFilesystems{}, nil, err
	}
	closeAll := func() { _ = tracked.Close() }
	if p.roots.Resident == p.roots.Tracked {
		return syncFilesystems{tracked: tracked, resident: tracked}, closeAll, nil
	}
	residentHandle, err := filesystem.Open(p.roots.Resident)
	if err != nil {
		closeAll()
		return syncFilesystems{}, nil, err
	}
	return syncFilesystems{tracked: tracked, resident: residentHandle}, func() {
		_ = residentHandle.Close()
		_ = tracked.Close()
	}, nil
}

func (p *Project) syncReport(ctx context.Context, seed *InitAuthority) (backups []Backup, changes []Change, pruned []string, err error) {
	corpus, pitfalls, topics, eff, err := p.deriveOperationStateWithPitfalls()
	if err != nil {
		return nil, nil, nil, err
	}
	filesystems, closeAll, err := p.openSyncFilesystems()
	if err != nil {
		return nil, nil, nil, err
	}
	defer closeAll()
	return p.syncReportWithPitfalls(ctx, seed, filesystems, corpus, pitfalls, topics, eff)
}

func (p *Project) syncReportWithPitfalls(ctx context.Context, seed *InitAuthority, filesystems syncFilesystems, corpus adr.Corpus, pitfalls pitfall.Corpus, topics topic.Corpus, eff map[string]bool) (backups []Backup, changes []Change, pruned []string, err error) {
	defer func() {
		slices.Sort(pruned)
		slices.SortFunc(changes, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	}()
	// Refuse before rendering or writing anything: a corrupt lock must never
	// produce a backup, skip a prune, or be overwritten (ADR-0076 Decision 2).
	lockPath := path.Join(config.DirName, "awf.lock")
	lockBytes, lockErr := filesystems.tracked.Read(lockPath)
	var old *manifest.Lock
	found := lockErr == nil
	if found {
		old, err = manifest.Parse(lockBytes)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
		}
	} else if !errors.Is(lockErr, fs.ErrNotExist) {
		return nil, nil, nil, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", lockErr)
	}
	if seed != nil {
		if found {
			return nil, nil, nil, errors.New("first-adoption initialization requires an absent lock")
		}
	} else {
		if !found {
			return nil, nil, nil, errors.New("pre-tracking authority: ordinary sync cannot create lock authority; use the bridge release to attest")
		}
		state, stateErr := old.AuthorityState()
		if stateErr != nil || state != manifest.AuthorityPermanent {
			return nil, nil, nil, errors.New("pre-tracking authority: ordinary sync requires a permanent lock; use the bridge release to attest")
		}
	}
	preservedResidents, err := resident.InspectRoots(p.roots.Resident)
	if err != nil {
		return nil, nil, nil, err
	}
	op, err := p.outputPlanWithPitfalls(ctx, corpus, pitfalls, topics, eff)
	if err != nil {
		return nil, nil, nil, err
	}
	files := op.writeFiles()
	for _, f := range files {
		if f.Policy.ValidateFrontmatter {
			if err := validateArtifact([]byte(f.Content), f.Encoder); err != nil { // coverage-ignore: rendered catalog skill/agent syntax is template-fixed and cannot be invalid at sync time
				return nil, nil, nil, fmt.Errorf("invalid agent artifact in %s: %w", f.Path, err)
			}
		}
	}
	// Prior lock, read before any write (top of this func): membership decides
	// foreign (back up) vs awf-managed (overwrite silently), and drives pruning.
	prior := map[string]bool{}
	if old != nil {
		for path := range old.Files {
			prior[path] = true
		}
	}

	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if old != nil {
		lock.InitializedWithVersion = old.InitializedWithVersion
	} else {
		lock.InitializedWithVersion = seed.InitializedWithVersion
	}
	want := map[string]bool{}
	for _, f := range files {
		filesystem, outputPath := filesystems.output(f.Path)
		dir := path.Dir(outputPath)
		if dir != "." {
			if err := filesystem.MkdirAll(dir, 0o755); err != nil {
				return backups, changes, pruned, err
			}
		}
		if strings.HasPrefix(f.Path, config.DirName+"/") && strings.HasSuffix(f.Path, "/.gitignore") && resident.IsResidentPath(strings.TrimSuffix(f.Path, "/.gitignore")) {
			if err := filesystem.Chmod(dir, 0o700); err != nil {
				return backups, changes, pruned, err
			}
		}
		info, infoErr := filesystem.LinkInfo(outputPath)
		if infoErr != nil && !errors.Is(infoErr, fs.ErrNotExist) {
			return backups, changes, pruned, infoErr
		}
		if !prior[f.Path] && infoErr == nil {
			// touches-state: rendering/sync-and-drift:sync-backs-up-foreign - foreign-file backup on sync; proof in project_test.go
			bak, err := p.backupFileConfined(outputPath, filesystem)
			if err != nil {
				return backups, changes, pruned, fmt.Errorf("back up %s: %w", f.Path, err)
			}
			backups = append(backups, Backup{Path: f.Path, Bak: bak, Index: f.RegenChecked})
		}
		perm := fs.FileMode(0o644)
		if strings.HasPrefix(f.Content, "#!") {
			perm = 0o755
		}
		modeChanged := false
		changedOutput := infoErr != nil
		if infoErr == nil && info.Mode()&fs.ModeSymlink == 0 {
			before, mode, readErr := filesystem.ReadWithMode(outputPath)
			if readErr != nil {
				return backups, changes, pruned, readErr
			}
			modeChanged = mode != perm
			changedOutput = string(before) != f.Content
		} else if infoErr == nil {
			// A managed final symlink is replaced as its entry without target access.
			changedOutput = true
		}
		recordChange := func() {
			if old == nil {
				return
			}
			oldE, ok := old.Files[f.Path]
			if !ok {
				changes = append(changes, Change{Path: f.Path, Cause: "added"})
				return
			}
			tMoved, cMoved := f.TemplateHash != oldE.TemplateHash, f.ConfigHash != oldE.ConfigHash
			cause := "internal"
			switch {
			case tMoved && cMoved:
				cause = "template+config"
			case tMoved:
				cause = "template"
			case cMoved:
				cause = "config"
			case f.RegenChecked:
				cause = "regenerated"
			}
			changes = append(changes, Change{Path: f.Path, Cause: cause})
		}
		if err := filesystem.Replace(outputPath, []byte(f.Content), perm); err != nil {
			return backups, changes, pruned, err
		}
		// Replacement commits bytes and final mode together, so a change becomes
		// reportable only after the namespace commit succeeds.
		if changedOutput || modeChanged {
			recordChange()
		}
		lock.Files[f.Path] = manifest.Entry{
			TemplateID: f.TemplateID, TemplateHash: f.TemplateHash,
			ConfigHash: f.ConfigHash, OutputHash: manifest.Hash([]byte(f.Content)),
			RegenChecked: f.RegenChecked,
		}
		want[f.Path] = true
	}
	// Prune files from the previous lock that are no longer produced, then remove
	// every directory left empty - walking all ancestors deepest-first, not just the
	// immediate parent, so dropping a target clears its whole tree (inv:
	// target-prune-ancestors; reuses Uninstall's idiom).
	if old != nil {
		type cleanupDir struct {
			filesystem syncFilesystem
			path       string
		}
		dirs := map[string]cleanupDir{}
		for path, entry := range old.Files {
			if want[path] || resident.PreserveRemoval(path, preservedResidents) {
				continue
			}
			// A non-local entry (corrupted or malicious lock) would delete outside
			// the root and send the ancestor walk below it, never reaching p.Root.
			if !filepath.IsLocal(filepath.FromSlash(path)) {
				continue
			}
			filesystem, outputPath := filesystems.output(path)
			// The outgoing co-owned runner is the one pruned output an adopter
			// hand-authored inside (its in-place verb bodies), so it is backed
			// up before removal for the one-time hand-port instead of vanishing
			// into git history (ADR-0156 item 9). A backup failure aborts the
			// prune - never a silent fall-through to deletion.
			switch entry.TemplateID {
			case localDocTID:
				if info, existsErr := filesystem.LinkInfo(outputPath); existsErr != nil && !errors.Is(existsErr, fs.ErrNotExist) {
					return backups, changes, pruned, fmt.Errorf("inspect pruned local document %s: %w", path, existsErr)
				} else if existsErr == nil {
					if info.Mode()&fs.ModeSymlink != 0 {
						return backups, changes, pruned, fmt.Errorf("unsafe pruned local document %s", path)
					}
					bak, bakErr := p.backupFileConfined(outputPath, filesystem)
					if bakErr != nil {
						return backups, changes, pruned, fmt.Errorf("back up pruned local document %s: %w", path, bakErr)
					}
					backups = append(backups, Backup{Path: path, Bak: bak})
				}
			case coOwnedRunnerTID:
				if info, existsErr := filesystem.LinkInfo(outputPath); existsErr == nil && info.Mode()&fs.ModeSymlink == 0 {
					bak, bakErr := p.backupFileConfined(outputPath, filesystem)
					if bakErr != nil {
						return backups, changes, pruned, fmt.Errorf("back up pruned runner %s: %w", path, bakErr)
					}
					backups = append(backups, Backup{Path: path, Bak: bak})
				}
			}
			// Report only an actual removal - a path whose file is already gone
			// must not be claimed pruned. Any other failure preserves the old lock
			// so the managed path remains visible and the operation can be retried.
			err := filesystem.Remove(outputPath)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return backups, changes, pruned, fmt.Errorf("remove retired output %s: %w", path, err)
			}
			if err == nil {
				pruned = append(pruned, path)
			}
			for d := filepath.ToSlash(filepath.Dir(filepath.FromSlash(outputPath))); d != "."; d = filepath.ToSlash(filepath.Dir(filepath.FromSlash(d))) {
				key := fmt.Sprintf("%t:%s", resident.IsResidentPath(path), d)
				dirs[key] = cleanupDir{filesystem: filesystem, path: d}
			}
		}
		dirList := slices.Collect(maps.Values(dirs))
		slices.SortFunc(dirList, func(a, b cleanupDir) int { return len(b.path) - len(a.path) })
		for _, d := range dirList {
			_ = d.filesystem.Remove(d.path) // removes only if now empty
		}
	}
	lockBytes, err = lock.Marshal()
	if err != nil { // coverage-ignore: sync constructs only authority-valid lock fields, so marshal failure requires a future representation change
		return backups, changes, pruned, err // coverage-ignore: sync constructs only authority-valid lock fields, so marshal failure requires a future representation change
	}
	return backups, changes, pruned, filesystems.tracked.Replace(lockPath, lockBytes, 0o644)
}

// IsLocalDocTemplate is the bounded recognition policy outer composition passes
// to uninstall without leaking the template identity into resident.
func IsLocalDocTemplate(templateID string) bool { return templateID == localDocTID }

func (p *Project) lockPath() string {
	return config.LockPath(p.Root)
}

// deriveOperationState derives the values a lifecycle operation needs from
// disk: the parsed ADR and pitfall corpora, current-state topics, and effective
// rendered skill set. The operation that calls this owns
// the result and threads it to its consumers, so nothing derived here outlives
// the call and no consumer re-derives it (ADR-0180).
//
// Deriving per operation is what keeps Check's contract honest. Check compares
// rendered output against the decisions directory as it is on disk right now,
// so a corpus held on the Project across calls would make a Check following a
// Sync miss an ADR written in between, silently blinding the drift oracle
// rather than merely serving a stale read. A value that cannot outlive the
// operation cannot go stale, so no caller has to remember to reset it.
func (p *Project) deriveOperationStateWithPitfalls() (adr.Corpus, pitfall.Corpus, topic.Corpus, map[string]bool, error) {
	corpus, err := adr.LoadCorpus(p.decisionsDir())
	if err != nil {
		return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	pitfalls, err := p.loadPitfallCorpus()
	if err != nil {
		return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	topics, err := topic.LoadCorpus(p.Root, p.Cfg, corpus)
	if err != nil {
		return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	eff, err := p.effectiveSkills()
	if err != nil {
		return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	return corpus, pitfalls, topics, eff, nil
}

// Audit runs the process-conformance audit (ADR-0017) over the caller-supplied
// commit range. No config key supplies a base: the range is always explicit
// (ADR-0127 Decision 3).
func (p *Project) Audit(ctx context.Context, base, head string) ([]audit.Finding, int, error) {
	s := audit.Resolve(config.AuditScopes(p.Cfg.Audit))
	lay := p.layout()
	generated := map[string]bool{}
	lock, _, err := manifest.LoadOptional(p.lockPath())
	if err != nil {
		return nil, 0, err
	}
	if lock != nil {
		for path := range lock.Files {
			generated[path] = true
		}
	}
	return audit.Run(ctx, p.Root, base, head, audit.Inputs{
		Settings:       s,
		GeneratedPaths: generated,
		ADRDir:         lay.ADRDir,
		DocsDir:        lay.DocsDir,
		IndexMd:        lay.IndexMd,
		PlansDir:       lay.PlansDir,
	})
}

// onIntegrationBranch reports whether this checkout is positively identified as
// sitting on the configured integration branch. Every indeterminate outcome -
// no repository, a probe failure, a detached HEAD - reports false rather than an
// error, because both consumers must degrade to the safe answer instead of
// failing: the scaffold writes a pending record, and the pending-record check
// stays silent (ADR-0202 item 7). A detached HEAD needs no separate test: the
// seam reports it as an empty branch name, and integrationBranch is validated
// non-empty, so the comparison cannot match.
func (p *Project) onIntegrationBranch(ctx context.Context) bool {
	repo, err := p.gitRepo()
	if err != nil {
		return false
	}
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return false
	}
	return branch == p.Cfg.IntegrationBranch
}

// NewADR scaffolds a new ADR file under the project's decisions dir from the
// rendered template, with its title/date filled in and marker comments
// stripped, refusing to overwrite an existing file. It is branch-aware
// (ADR-0202 item 5): on the integration branch it allocates the next sequential
// number, and anywhere else - including a detached HEAD or an unreadable
// repository - it writes a slug-identified pending record that `awf adr number`
// numbers at integration. Mirrors the CheckInvariants/Audit pattern - cmd/awf
// reaches this only through this exported method, never internal/project.Layout
// directly.
func (p *Project) NewADR(ctx context.Context, title string) (string, error) {
	if !p.onIntegrationBranch(ctx) {
		return adr.NewPendingFile(p.decisionsDir(), title)
	}
	return adr.NewFile(p.decisionsDir(), title)
}

// NewPlan scaffolds a new plan under docsDir/plans from the rendered plans
// template. Mirrors NewADR minus sequential numbering (ADR-0098).
func (p *Project) NewPlan(title string) (string, error) {
	return plan.NewFile(filepath.Join(p.Root, config.DocsDir, "plans"), title)
}
