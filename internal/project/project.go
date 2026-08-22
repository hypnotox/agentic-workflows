// Package project ties config, catalog, render, and manifest together to sync rendered files into a project and check them for drift.
package project

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
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
func (s *ProjectState) Root() string {
	if s == nil || s.state == nil {
		return ""
	}
	return s.state.Root()
}

// Config returns a defensive copy of the immutable loaded configuration facts.
func (s *ProjectState) Config() *config.Config {
	if s == nil || s.state == nil {
		return config.Facts{}.Config()
	}
	return s.state.Config()
}

// Targets returns a defensive copy of the resolved targets.
func (s *ProjectState) Targets() []Target {
	if s == nil || s.state == nil {
		return nil
	}
	return s.state.Targets()
}

// OutputState returns the immutable loaded facts consumed by Publisher. The
// lower value is projected defensively at this package boundary so Publisher
// cannot retain the compatibility facade's target slice.
func (s *ProjectState) OutputState() *projectstate.ProjectState {
	if s == nil || s.state == nil {
		return nil
	}
	return projectstate.NewDerivedWithFacts(s.Root(), s.roots(), s.nested(), s.facts(), s.catalog(), s.completeCatalog(), s.Targets())
}

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

func lockPath(root string) string {
	return config.LockPath(root)
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
