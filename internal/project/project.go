// Package project ties config, catalog, render, and manifest together to sync rendered files into a project and check them for drift.
package project

import (
	"context"
	_ "embed"
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
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"golang.org/x/mod/semver"
)

// versionFile is the single embedded version authority.
//
//go:embed VERSION
var versionFile string

// Version is the awf release version. Gate comparisons, the lock stamp, the
// bootstrap pin, and CLI output all read this value (ADR-0049).
var Version = strings.TrimSuffix(versionFile, "\n")

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
	46: "0.39.0",
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

// CheckVersionAuthority validates the embedded version and its compatibility
// with the current config schema generation.
func CheckVersionAuthority() error {
	return validateVersionAuthority(versionFile, Version, migrate.Current())
}

func validateVersionAuthority(raw, exposed string, schema int) error {
	if !strings.HasSuffix(raw, "\n") || strings.Count(raw, "\n") != 1 {
		return errors.New("canonical version file must contain one version followed by one newline")
	}
	embedded := strings.TrimSuffix(raw, "\n")
	if exposed != embedded {
		return fmt.Errorf("embedded version %q does not match project.Version %q", embedded, exposed)
	}
	if canonical := semver.Canonical("v" + exposed); canonical != "v"+exposed {
		return fmt.Errorf("project.Version %q is not a canonical semantic version without a v prefix", exposed)
	}
	return ValidateSchemaMinimumVersion(schema, exposed)
}

// LoadConfigTree loads one project's configuration tree.
type LoadConfigTree func(string) (*config.Config, error)

// ResolveResidentRoot maps an invoking checkout to the root that owns resident
// state. It takes the operation's context because the resolution reaches Git.
type ResolveResidentRoot func(context.Context, string) string

// Loader owns project-opening policy over explicitly selected dependencies.
type Loader struct {
	loadConfigTree      LoadConfigTree
	view                catalog.View
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
	return &Loader{loadConfigTree: loadConfigTree, view: catalog.NewView(standard), resolveResidentRoot: resolveResidentRoot, repo: repo}
}

// ProjectState preserves the RF-002 compatibility name for the lower immutable state owner.
type ProjectState struct{ state *projectstate.ProjectState }

// Root returns the invoking checkout root.
func (s *ProjectState) Root() string { return s.state.Root() }

// Config returns a defensive copy of the immutable loaded configuration facts.
func (s *ProjectState) Config() *config.Config { return s.state.Config() }

// Targets returns a defensive copy of the resolved targets.
func (s *ProjectState) Targets() []Target { return s.state.Targets() }

func (s *ProjectState) roots() resident.Roots             { return s.state.Roots() }
func (s *ProjectState) nested() bool                      { return s.state.Nested() }
func (s *ProjectState) facts() config.Facts               { return s.state.Facts() }
func (s *ProjectState) catalog() *catalog.Catalog         { return s.state.Catalog() }
func (s *ProjectState) completeCatalog() *catalog.Catalog { return s.state.CompleteCatalog() }

func newProjectState(root string, roots resident.Roots, nested bool, cfg *config.Config, selected, complete *catalog.Catalog, targets []Target) (*ProjectState, error) {
	state, err := projectstate.New(root, roots, nested, cfg, selected, complete, targets)
	if err != nil {
		return nil, err
	}
	return &ProjectState{state: state}, nil
}

// renderInputs is the small rendering concern boundary. Immutable loaded facts
// remain in ProjectState; cfg and read are the selected operation tree. Git is
// deliberately not a field: repository operations take it explicitly.
type renderInputs struct {
	state *ProjectState
	cfg   *config.Config
	read  ProjectTreeReader
}

func newRenderInputs(state *ProjectState, cfg *config.Config, read ProjectTreeReader) renderInputs {
	return renderInputs{state: state, cfg: cfg, read: read}
}

func (p renderInputs) root() string                      { return p.state.Root() }
func (p renderInputs) residentRoots() resident.Roots     { return p.state.roots() }
func (p renderInputs) targets() []Target                 { return p.state.Targets() }
func (p renderInputs) catalog() *catalog.Catalog         { return p.state.catalog() }
func (p renderInputs) completeCatalog() *catalog.Catalog { return p.state.completeCatalog() }
func (p renderInputs) isNested() bool                    { return p.state.nested() }

// gitRepo returns the handle this project reads Git through, or the reason it
// has none. Opening is never retried per operation: the handle is chosen once,
// at construction, and this is the single place that reports its absence.
func gitRepo(root string, repo *awfgit.Repo) (*awfgit.Repo, error) {
	if repo == nil {
		return nil, fmt.Errorf("%s: %w", root, awfgit.ErrNotARepository)
	}
	return repo, nil
}

// Open is the transitional compatibility entry point for callers not yet
// migrated to outer composition. A supplied repository preserves one composed
// handle for an existing compatibility caller; new code composes a Loader.
func Open(ctx context.Context, root string, selected ...*awfgit.Repo) (*ProjectState, error) {
	if len(selected) > 1 {
		return nil, errors.New("project Open: multiple repository dependencies")
	}
	var repo *awfgit.Repo
	if len(selected) == 1 {
		repo = selected[0]
	} else {
		var err error
		repo, _, err = awfgit.OpenContaining(root)
		if err != nil && !errors.Is(err, awfgit.ErrNotARepository) {
			return nil, err
		}
	}
	if repo == nil {
		return NewLoaderWithoutRepository(config.Load, catalog.CompleteView().Catalog(), awfgit.ProjectResidentRoot).Open(ctx, root)
	}
	return NewLoader(config.Load, catalog.CompleteView().Catalog(), awfgit.ProjectResidentRoot, repo).Open(ctx, root)
}

// Open loads, validates, and derives one project's immutable facts.
func (l *Loader) Open(ctx context.Context, root string) (*ProjectState, error) {
	state, _, err := l.OpenForOperation(ctx, root)
	return state, err
}

// OpenForOperation returns immutable state together with the one concrete
// configuration tree selected during loading. Commands pass that tree only to
// operations that read sidecars, parts, or source bytes.
func (l *Loader) OpenForOperation(ctx context.Context, root string) (*ProjectState, *config.Config, error) {
	cfg, err := l.loadConfigTree(config.RootDir(root))
	if err != nil {
		return nil, nil, err
	}
	if cfg == nil {
		return nil, nil, errors.New("project Loader: load config tree returned nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	completeCat := l.view.Catalog()
	selected := catalog.NewProfileView(completeCat, cfg.Profile)
	cat := selected.Catalog()
	if err := catalog.ValidateWorkflowProfiles(cat); err != nil {
		return nil, nil, err
	}
	targets, err := resolveTargets(KnownTargets())
	if err != nil { // coverage-ignore: configured-target validation succeeded and KnownTargets is exhaustively backed by built-in descriptor tests
		return nil, nil, err
	}
	roots := resident.NewRoots(root, l.resolveResidentRoot(ctx, root))
	nested := l.repo != nil && l.repo.IsNested()
	state, err := newProjectState(root, roots, nested, cfg, cat, completeCat, targets)
	if err != nil {
		return nil, nil, err
	}
	cfg = cfg.OperationTree().Bind(state.facts())
	if err := validateAgainstCatalog(newRenderInputs(state, cfg, nil)); err != nil {
		return nil, nil, err
	}
	return state, cfg, nil
}

// workingTree snapshots the project's working universe through its handle.
func workingTree(root string, repo *awfgit.Repo, ctx context.Context) (*snapshot.Tree, error) {
	repo, err := gitRepo(root, repo)
	if err != nil {
		return nil, err
	}
	return snapshot.WorkingTree(ctx, repo)
}

// indexTree snapshots the project's staged universe through its handle.
func indexTree(root string, repo *awfgit.Repo, ctx context.Context) (*snapshot.Tree, error) {
	repo, err := gitRepo(root, repo)
	if err != nil {
		return nil, err
	}
	return snapshot.IndexTree(ctx, repo)
}

// stagedProject composes the minimal index-only rendering inputs. It never
// invokes Loader or reads working-tree configuration; each staged operation
// supplies the repository selected at its boundary.
func stagedProject(root string, prefix string) renderInputs {
	complete := catalog.CompleteView().Catalog()
	state := &ProjectState{state: projectstate.NewDerived(root, resident.NewRoots(root, ""), prefix != "", complete, complete, nil)}
	return newRenderInputs(state, nil, nil)
}

// catalog returns this project's one private selected-catalog snapshot.
func projectCatalog(p renderInputs) *catalog.Catalog { return p.catalog() }

// completeCatalog returns the private complete-catalog dependency supplied at composition.
func completeProjectCatalog(p renderInputs) *catalog.Catalog { return p.completeCatalog() }

func fullProfile(p renderInputs) bool { return p.cfg == nil || p.cfg.Profile != catalog.ProfileCore }

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
func syncReportOperation(p renderInputs) ([]Backup, []Change, []string, error) {
	// Refuse an unresolvable hook-command wiring before rendering anything
	// (ADR-0156 Decision 5); first-adoption InitializeReport stays exempt.
	if err := validateCommandWiring(p.cfg); err != nil {
		return nil, nil, nil, err
	}
	return syncReport(p, nil)
}

// InitAuthority is the explicit provenance supplied only by first adoption.
type InitAuthority struct {
	InitializedWithVersion string
}

// InitializeReport renders a first adoption while sealing its existing ADR
// identities. It has the same reporting contract as SyncReport.
func initializeReport(p renderInputs, seed InitAuthority) ([]Backup, []Change, []string, error) {
	return syncReport(p, &seed)
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

func openSyncFilesystems(p renderInputs) (syncFilesystems, func(), error) {
	tracked, err := filesystem.Open(p.residentRoots().Tracked)
	if err != nil {
		return syncFilesystems{}, nil, err
	}
	closeAll := func() { _ = tracked.Close() }
	if p.residentRoots().Resident == p.residentRoots().Tracked {
		return syncFilesystems{tracked: tracked, resident: tracked}, closeAll, nil
	}
	residentHandle, err := filesystem.Open(p.residentRoots().Resident)
	if err != nil {
		closeAll()
		return syncFilesystems{}, nil, err
	}
	return syncFilesystems{tracked: tracked, resident: residentHandle}, func() {
		_ = residentHandle.Close()
		_ = tracked.Close()
	}, nil
}

func syncReport(p renderInputs, seed *InitAuthority) (backups []Backup, changes []Change, pruned []string, err error) {
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(p)
	if err != nil {
		return nil, nil, nil, err
	}
	filesystems, closeAll, err := openSyncFilesystems(p)
	if err != nil {
		return nil, nil, nil, err
	}
	defer closeAll()
	return syncReportWithPitfalls(p, seed, filesystems, corpus, pitfalls, topics, eff)
}

func syncReportWithPitfalls(p renderInputs, seed *InitAuthority, filesystems syncFilesystems, corpus adr.Corpus, pitfalls pitfall.Corpus, topics topic.Corpus, eff map[string]bool) (backups []Backup, changes []Change, pruned []string, err error) {
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
	preservedResidents, err := resident.InspectRoots(p.residentRoots().Resident)
	if err != nil {
		return nil, nil, nil, err
	}
	op, err := outputPlanWithPitfalls(p, corpus, pitfalls, topics, eff)
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
			bak, err := backupFileConfined(outputPath, filesystem)
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
			// the root and send the ancestor walk below it, never reaching p.root().
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
					bak, bakErr := backupFileConfined(outputPath, filesystem)
					if bakErr != nil {
						return backups, changes, pruned, fmt.Errorf("back up pruned local document %s: %w", path, bakErr)
					}
					backups = append(backups, Backup{Path: path, Bak: bak})
				}
			case coOwnedRunnerTID:
				if info, existsErr := filesystem.LinkInfo(outputPath); existsErr == nil && info.Mode()&fs.ModeSymlink == 0 {
					bak, bakErr := backupFileConfined(outputPath, filesystem)
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

func lockPath(root string) string {
	return config.LockPath(root)
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
func deriveOperationStateWithPitfalls(p renderInputs) (adr.Corpus, pitfall.Corpus, topic.Corpus, map[string]bool, error) {
	corpus := adr.Corpus{}
	topics := topic.Corpus{}
	var err error
	if fullProfile(p) {
		corpus, err = adr.LoadCorpus(decisionsDir(p.root()))
		if err != nil {
			return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
		}
		topics, err = topic.LoadCorpus(p.root(), p.cfg, corpus)
		if err != nil {
			return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
		}
	}
	pitfalls, err := loadPitfallCorpus(p)
	if err != nil {
		return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	eff, err := effectiveSkills(p)
	if err != nil {
		return adr.Corpus{}, pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	return corpus, pitfalls, topics, eff, nil
}

// Audit runs the process-conformance audit (ADR-0017) over the caller-supplied
// commit range. No config key supplies a base: the range is always explicit
// (ADR-0127 Decision 3).
func auditOperation(root string, cfg *config.Config, ctx context.Context, base, head string) ([]audit.Finding, int, error) {
	s := audit.Resolve(config.AuditScopes(cfg.Audit))
	generated := map[string]bool{}
	lock, _, err := manifest.LoadOptional(lockPath(root))
	if err != nil {
		return nil, 0, err
	}
	if lock != nil {
		for path := range lock.Files {
			generated[path] = true
		}
	}
	docsDir := config.DocsDir
	adrDir := docsDir + "/decisions"
	return audit.Run(ctx, root, base, head, audit.Inputs{
		Settings:       s,
		GeneratedPaths: generated,
		ADRDir:         adrDir,
		DocsDir:        docsDir,
		IndexMd:        adrDir + "/INDEX.md",
		PlansDir:       docsDir + "/plans",
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
func onIntegrationBranch(root string, cfg *config.Config, repo *awfgit.Repo, ctx context.Context) bool {
	repo, err := gitRepo(root, repo)
	if err != nil {
		return false
	}
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return false
	}
	return branch == cfg.IntegrationBranch
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
func newADR(root string, cfg *config.Config, repo *awfgit.Repo, ctx context.Context, title string) (string, error) {
	if !onIntegrationBranch(root, cfg, repo, ctx) {
		return adr.NewPendingFile(decisionsDir(root), title)
	}
	return adr.NewFile(decisionsDir(root), title)
}

// NewPlan scaffolds a new plan under docsDir/plans from the rendered plans
// template. Mirrors NewADR minus sequential numbering (ADR-0098).
func newPlan(root, title string) (string, error) {
	return plan.NewFile(filepath.Join(root, config.DocsDir, "plans"), title)
}
