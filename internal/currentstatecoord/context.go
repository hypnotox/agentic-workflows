package currentstatecoord

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/contextinput"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// ContextPreparation selects one immutable tree, config, lock, reader, and publisher state.
// Publisher receives these exact values and completion never reparses their ADR, topic, or plan corpora.
type ContextPreparation struct {
	State     *projectstate.ProjectState
	Config    *config.Config
	Reader    outputplan.TreeReader
	layout    contextinput.Layout
	tree      *snapshot.Tree
	inventory *snapshot.Inventory
	lock      *manifest.Lock
	eligible  []string
}

// PrepareWorkingContext selects the working universe once for context output.
func PrepareWorkingContext(state *projectstate.ProjectState, repo *awfgit.Repo, ctx context.Context) (*ContextPreparation, error) {
	if state == nil {
		return nil, errors.New("context preparation: missing project state")
	}
	tree, err := workingTree(state.Root(), repo, ctx)
	if errors.Is(err, awfgit.ErrNotARepository) {
		tree, err = snapshot.FilesystemTree(ctx, state.Root())
	}
	if err != nil {
		return nil, err
	}
	lock, _, err := optionalLockFromTree(tree)
	if err != nil {
		return nil, err
	}
	cfg, found, err := configFromTree(state.Root(), tree, lock)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("working snapshot has no %s/config.yaml", config.DirName)
	}
	return newContextPreparation(state, cfg, tree, lock)
}

// PrepareStagedContext selects only the index universe for staged context
// output. It never consults working-tree configuration or locks.
func PrepareStagedContext(ctx context.Context, root string) (*ContextPreparation, error) {
	repo, prefix, err := awfgit.OpenContaining(root)
	if err != nil {
		return nil, err
	}
	tree, err := indexTree(root, repo, ctx)
	if err != nil {
		return nil, err
	}
	lock, _, err := optionalLockFromTree(tree)
	if err != nil {
		return nil, err
	}
	cfg, found, err := configFromTree(root, tree, lock)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	complete := catalog.CompleteView().Catalog()
	selected := catalog.NewProfileView(complete, cfg.Profile).Catalog()
	facts, err := config.NewFacts(cfg)
	if err != nil { // coverage-ignore: configFromTree already parsed a finite YAML semantic representation that NewFacts can clone
		return nil, err
	}
	targets, err := projectstate.ResolveTargets(projectstate.KnownTargets())
	if err != nil { // coverage-ignore: KnownTargets is the same closed built-in registry ResolveTargets projects
		return nil, err
	}
	state := projectstate.NewDerivedWithFacts(root, resident.NewRoots(root, ""), prefix != "", facts, selected, complete, targets)
	return newContextPreparation(state, cfg, tree, lock)
}

// Tree returns the immutable selected snapshot for a project-owned comparison.
func (p *ContextPreparation) Tree() *snapshot.Tree { return p.tree }

// Lock returns a defensive projection for a project-owned comparison.
func (p *ContextPreparation) Lock() *manifest.Lock { return p.lock.Clone() }

func newContextPreparation(state *projectstate.ProjectState, cfg *config.Config, tree *snapshot.Tree, lock *manifest.Lock, inventory ...*snapshot.Inventory) (*ContextPreparation, error) {
	complete := state.CompleteCatalog()
	selected := catalog.NewProfileView(complete, cfg.Profile).Catalog()
	operationState, err := projectstate.New(state.Root(), state.Roots(), state.Nested(), cfg, selected, complete, state.Targets())
	if err != nil {
		return nil, err
	}
	cat := operationState.Catalog()
	docs := map[string]string{}
	singletons := map[string]string{}
	for name, entry := range cat.Docs {
		out := config.DocsDir + "/" + name + ".md"
		if entry.AgentsDoc {
			out = "AGENTS.md"
		} else if entry.Path != "" {
			out = config.DocsDir + "/" + entry.Path
		}
		docs[name] = out
		if !entry.AgentsDoc && entry.TemplateKey != "" {
			singletons[entry.TemplateKey] = out
		}
	}
	var liveInventory *snapshot.Inventory
	if len(inventory) > 0 {
		liveInventory = inventory[0]
	}
	prep := &ContextPreparation{State: operationState, Config: cfg, Reader: snapshotReader{tree: tree, inventory: liveInventory}, layout: contextinput.Layout{DocsDir: config.DocsDir, ADRDir: config.DocsDir + "/decisions", IndexMd: config.DocsDir + "/decisions/INDEX.md", PlansDir: config.DocsDir + "/plans", DomainsDir: config.DocsDir + "/domains", Docs: docs, Singletons: singletons}, tree: tree, inventory: liveInventory, lock: lock, eligible: eligiblePaths(tree, lock, cfg.ContextIgnore)}
	return prep, nil
}

// CompleteContext consumes Publisher's defensive semantic projections. The
// coordinator deliberately depends only on neutral values, never Publisher.
func CompleteContext(prep *ContextPreparation, corpus adr.Corpus, topics topic.Corpus, plans []plan.Plan, declarations []outputplan.Declaration) contextinput.Input {
	loaded := currentstate.Loaded{ADRs: corpus.All(), Corpus: corpus.Clone(), Topics: topics.Clone()}
	return contextinput.NewWithInventory(prep.layout, loaded, contextinput.NewPlanContext(plans, corpus), prep.tree, prep.inventory, prep.lock, declarations, prep.eligible, prep.Config.ContextIgnore)
}

type snapshotReader struct {
	tree      *snapshot.Tree
	inventory *snapshot.Inventory
}

func (r snapshotReader) ReadFile(path string) ([]byte, bool, error) {
	f, ok := r.tree.Lookup(filepath.ToSlash(path))
	if !ok || !f.Scannable() {
		return nil, false, nil
	}
	return slices.Clone(f.Bytes), true, nil
}

// PathExists lets declaration preparation use complete inventory presence
// without presenting unread files as empty semantic sources.
func (r snapshotReader) PathExists(path string) bool {
	path = filepath.ToSlash(path)
	if r.inventory != nil {
		entry, ok := r.inventory.Lookup(path)
		return ok && entry.Mode != snapshot.Symlink
	}
	f, ok := r.tree.Lookup(path)
	return ok && f.Scannable()
}

func (r snapshotReader) Paths(prefix string) ([]string, error) {
	out := []string{}
	prefix = filepath.ToSlash(prefix)
	for _, f := range r.tree.List() {
		if f.Scannable() && strings.HasPrefix(f.Path, prefix) {
			out = append(out, f.Path)
		}
	}
	return out, nil
}

func selectedPaths(selected map[string]bool) []string {
	paths := make([]string, 0, len(selected))
	for path := range selected {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

// PrepareFocusedWorkingContext captures complete live metadata while reading
// only the bytes needed by the ordinary answer. Config and lock form the
// operation's initial frozen facts; the second read is a delta, never a new
// configuration universe.
func PrepareFocusedWorkingContext(state *projectstate.ProjectState, repo *awfgit.Repo, ctx context.Context, requests []string) (*ContextPreparation, error) {
	if state == nil {
		return nil, errors.New("context preparation: missing project state")
	}
	if repo == nil {
		return nil, fmt.Errorf("%s: %w", state.Root(), awfgit.ErrNotARepository)
	}
	entries, err := repo.WorkingEntries(ctx)
	if err != nil {
		return nil, err
	}
	initial, err := snapshot.WorkingContextFromEntries(ctx, repo, entries, []string{".awf/config.yaml", ".awf/awf.lock"})
	if err != nil {
		return nil, err
	}
	tree, _ := snapshot.NewTree(initial.Selection().List())
	lock, _, err := optionalLockFromTree(tree)
	if err != nil {
		return nil, err
	}
	cfg, found, err := configFromTree(state.Root(), tree, lock)
	if err != nil || !found {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("working snapshot has no %s/config.yaml", config.DirName)
	}

	nested := nestedAdopters(entries)
	selected := focusedContextPaths(entries, state, cfg, lock, requests, nested)
	// Config and lock were captured above and are deliberately not reread.
	delete(selected, ".awf/config.yaml")
	delete(selected, ".awf/awf.lock")
	delta, err := snapshot.WorkingContextFromEntries(ctx, repo, entries, selectedPaths(selected))
	if err != nil {
		return nil, err
	}
	files := append(initial.Selection().List(), delta.Selection().List()...)
	// Initial config/lock and delta paths are disjoint validated selections from
	// one inventory, so their union necessarily remains a valid Tree.
	tree, _ = snapshot.NewTree(files)
	// Preserve the parsed config and lock bytes from the initial capture while
	// binding sidecar reads to the completed selected tree.
	cfg = cfg.WithTree(configSnapshotReader{tree: tree})
	return newContextPreparation(state, cfg, tree, lock, initial.Inventory())
}

func nestedAdopters(entries []awfgit.TreeEntry) []string {
	var roots []string
	for _, entry := range entries {
		if entry.Mode != awfgit.BlobSymlink && entry.Path != ".awf/config.yaml" && strings.HasSuffix(entry.Path, "/.awf/config.yaml") {
			roots = append(roots, strings.TrimSuffix(entry.Path, "/.awf/config.yaml"))
		}
	}
	slices.Sort(roots)
	return roots
}

func insideNested(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func focusedContextPaths(entries []awfgit.TreeEntry, state *projectstate.ProjectState, cfg *config.Config, lock *manifest.Lock, requests, nested []string) map[string]bool {
	selected := map[string]bool{}
	add := func(path string) {
		if !insideNested(path, nested) {
			selected[path] = true
		}
	}
	for _, entry := range entries {
		path := entry.Path
		if entry.Mode == awfgit.BlobSymlink || strings.HasPrefix(path, "docs/decisions/") || strings.HasPrefix(path, "docs/plans/") || strings.HasPrefix(path, ".awf/topics/metadata/") || strings.HasPrefix(path, ".awf/topics/parts/") || strings.HasPrefix(path, ".awf/docs/pitfalls/") {
			add(path)
		}
		for _, request := range requests {
			request = filepath.ToSlash(filepath.Clean(request))
			manifested := false
			if lock != nil {
				_, manifested = lock.Files[path]
			}
			if manifested && (request == "." || path == request || strings.HasPrefix(path, request+"/")) {
				add(path)
			}
		}
		if cfg.CurrentState != nil {
			for _, source := range cfg.CurrentState.Sources {
				if pathglob.MatchAny(source.Globs, path) {
					add(path)
				}
			}
		}
	}
	// Publisher parses sidecars, while convention parts and local-doc outputs
	// need only inventory-backed presence during declaration preparation.
	cat := catalog.NewProfileView(state.CompleteCatalog(), cfg.Profile).Catalog()
	for name := range cat.Skills {
		add(".awf/skills/" + name + ".yaml")
	}
	for name := range cat.Agents {
		add(".awf/agents/" + name + ".yaml")
	}
	for name, doc := range cat.Docs {
		if doc.Mandatory {
			add(".awf/" + name + ".yaml")
		} else {
			add(".awf/docs/" + name + ".yaml")
		}
	}
	for _, domain := range cfg.Domains {
		add(".awf/domains/" + domain + ".yaml")
	}
	return selected
}
