// Package project owns project selection and validation and exposes immutable project Sessions.
package project

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"golang.org/x/mod/semver"
)

// versionFile is the single embedded version authority.
//
//go:embed VERSION
var versionFile string

// Version is the awf release version. Gate comparisons, the lock stamp, the
// bootstrap pin, and CLI output all read this value (ADR-0049).
var Version = strings.TrimSuffix(versionFile, "\n")

// minVersionBySchema maps each config-schema generation to the minimum
// project.Version allowed to render it; adding a migration without an entry
// here (and a matching const bump) fails the gate (ADR-0049 Decision 4).
var minVersionBySchema = map[int]string{50: "0.44.0", 51: "0.47.0", 52: "0.48.0"}

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
	reader              outputplan.TreeReader
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

// WithSelection returns the same loading authority with one bounded config and
// project-tree selection. Candidate and staged universes use this boundary so
// every Session is constructed through Load from matching inputs.
func (l *Loader) WithSelection(load LoadConfigTree, reader outputplan.TreeReader) *Loader {
	if l == nil || load == nil || reader == nil {
		panic("project Loader: missing selected project input")
	}
	copy := *l
	copy.loadConfigTree = load
	copy.reader = reader
	return &copy
}

// RegistryView is the immutable project-selected projection of the canonical
// artifact registry. The declaration owner remains artifactregistry.
type RegistryView struct {
	catalog catalog.View
	targets []artifactregistry.Target
}

// Catalog returns a defensive copy of the selected catalog.
func (v RegistryView) Catalog() *catalog.Catalog { return v.catalog.Catalog() }

// Targets returns defensive copies of the selected target declarations.
func (v RegistryView) Targets() []artifactregistry.Target { return cloneTargets(v.targets) }

// Session is one authoritative in-memory project selection. It owns the bound
// configuration tree, immutable current facts, repository and root handles,
// selected project reader, and canonical registry projection.
type Session struct {
	root     string
	roots    resident.Roots
	nested   bool
	facts    config.Facts
	selected *config.Config
	repo     *awfgit.Repo
	reader   outputplan.TreeReader
	registry RegistryView
}

// Root returns the invoking checkout root.
func (s *Session) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// Roots returns the selected tracked and resident roots.
func (s *Session) Roots() resident.Roots {
	if s == nil {
		return resident.Roots{}
	}
	return s.roots
}

// Nested reports whether the invoking checkout is nested below its repository root.
func (s *Session) Nested() bool { return s != nil && s.nested }

// Config returns a defensive copy retaining this Session's selected tree binding.
func (s *Session) Config() *config.Config {
	if s == nil || s.selected == nil {
		return config.Facts{}.Config()
	}
	return s.selected.OperationTree().Bind(s.facts)
}

// Repository returns the one Git handle selected during loading, if any.
func (s *Session) Repository() *awfgit.Repo {
	if s == nil {
		return nil
	}
	return s.repo
}

// Reader returns the project-tree reader selected with the configuration.
func (s *Session) Reader() outputplan.TreeReader {
	if s == nil {
		return nil
	}
	return s.reader
}

// Registry returns the Session's immutable artifact-registry projection.
func (s *Session) Registry() RegistryView {
	if s == nil {
		return RegistryView{}
	}
	return RegistryView{catalog: catalog.NewView(s.registry.Catalog()), targets: s.registry.Targets()}
}

// Catalog returns the Session's defensive selected registry catalog.
func (s *Session) Catalog() *catalog.Catalog { return s.Registry().Catalog() }

// Targets returns defensive copies of the resolved target declarations.
func (s *Session) Targets() []artifactregistry.Target { return s.Registry().Targets() }

func (s *Session) catalog() *catalog.Catalog { return s.Catalog() }

func cloneTargets(source []artifactregistry.Target) []artifactregistry.Target {
	return append([]artifactregistry.Target(nil), source...)
}

func newSession(root string, roots resident.Roots, nested bool, cfg *config.Config, selected *catalog.Catalog, targets []artifactregistry.Target, repo *awfgit.Repo, reader outputplan.TreeReader) (*Session, error) {
	facts, err := config.NewFacts(cfg)
	if err != nil {
		return nil, err
	}
	session := &Session{
		root: root, roots: roots, nested: nested, facts: facts,
		selected: cfg.OperationTree().Bind(facts), repo: repo, reader: reader,
		registry: RegistryView{catalog: catalog.NewView(selected), targets: cloneTargets(targets)},
	}
	return session, nil
}

// renderInputs is the small rendering concern boundary over one Session.
type renderInputs struct {
	session *Session
	cfg     *config.Config
	read    outputplan.TreeReader
}

func newRenderInputs(session *Session, cfg *config.Config, read outputplan.TreeReader) renderInputs {
	return renderInputs{session: session, cfg: cfg, read: read}
}

func (p renderInputs) root() string                  { return p.session.Root() }
func (p renderInputs) residentRoots() resident.Roots { return p.session.Roots() }
func (p renderInputs) catalog() *catalog.Catalog     { return p.session.catalog() }
func (p renderInputs) isNested() bool                { return p.session.Nested() }

// gitRepo returns the handle this project reads Git through, or the reason it
// has none. Opening is never retried per operation: the handle is chosen once,
// at construction, and this is the single place that reports its absence.
func gitRepo(root string, repo *awfgit.Repo) (*awfgit.Repo, error) {
	if repo == nil {
		return nil, fmt.Errorf("%s: %w", root, awfgit.ErrNotARepository)
	}
	return repo, nil
}

// AcquireProjectLease obtains both the selected-checkout and shared-resident
// transaction capabilities in filesystem's deterministic canonical order.
func (l *Loader) AcquireProjectLease(ctx context.Context, root string) (*filesystem.Lease, error) {
	return filesystem.AcquireProjectLease(ctx, root, l.resolveResidentRoot(ctx, root))
}

// CoversProjectLease verifies that a supplied transaction covers this loader's
// selected checkout and its resolved shared-resident root.
func (l *Loader) CoversProjectLease(ctx context.Context, root string, lease *filesystem.Lease) bool {
	return lease.CoversProject(root, l.resolveResidentRoot(ctx, root))
}

// LoadForMutation selects authority through the supplied confined handle and
// returns the identity of precisely the config bytes used by the Session.
func (l *Loader) LoadForMutation(ctx context.Context, root string, files *filesystem.Handle) (*Session, *filesystem.ExpectedIdentity, error) {
	if files == nil {
		return nil, nil, errors.New("project Loader: missing confined filesystem handle")
	}
	identity, err := files.ExpectedIdentity(".awf/config.yaml")
	if err != nil {
		return nil, nil, err
	}
	bytesBefore, err := files.Read(".awf/config.yaml")
	if err != nil {
		_ = identity.Release()
		return nil, nil, err
	}
	session, err := l.Load(ctx, root)
	if err != nil {
		_ = identity.Release()
		return nil, nil, err
	}
	if !bytes.Equal(bytesBefore, session.Config().Source()) {
		_ = identity.Release()
		return nil, nil, filesystem.ErrIdentityChanged
	}
	return session, identity, nil
}

// Load constructs one Session from the Loader's exact selected inputs.
func (l *Loader) Load(ctx context.Context, root string) (*Session, error) {
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
	selectedCatalog := l.view.Catalog()
	targets, err := resolveTargets(KnownTargets())
	if err != nil {
		return nil, err
	}
	roots := resident.NewRoots(root, l.resolveResidentRoot(ctx, root))
	nested := l.repo != nil && l.repo.IsNested()
	reader := l.reader
	if reader == nil {
		reader = filesystemProjectReader{root: root}
	}
	session, err := newSession(root, roots, nested, cfg, selectedCatalog, targets, l.repo, reader)
	if err != nil {
		return nil, err
	}
	if err := validateAgainstCatalog(newRenderInputs(session, session.Config(), session.Reader())); err != nil {
		return nil, err
	}
	return session, nil
}

// catalog returns this project's one private selected-catalog snapshot.
func projectCatalog(p renderInputs) *catalog.Catalog { return p.catalog() }

func lockPath(root string) string {
	return config.LockPath(root)
}
