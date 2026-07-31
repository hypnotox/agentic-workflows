package project

import (
	"context"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// ContextState is the loaded universe the context query reads and the single
// seam between the sync core and internal/contextq (ADR-0194 item 2). The core
// keeps the loading machinery - working and staged current-state loads, lock
// reads, declaration building, and eligible-path selection - and hands the
// result over whole; the query package derives everything else from these
// fields and reaches no further into the core.
//
// Every field is a construction input written by one of the two constructors
// below and never afterwards, so a ContextState value is immutable in practice
// and safe to share (code-design/state-ownership).
type ContextState struct {
	// Layout is the docs layout derived from the snapshot's own config.
	Layout Layout
	// Cfg is the configuration parsed from the snapshot, not from disk: the
	// query classifies the universe it was handed.
	Cfg *config.Config
	// Loaded is the parsed ADR and topic view of that same snapshot.
	Loaded currentstate.Loaded
	// Tree is the snapshot the universe was loaded from.
	Tree *snapshot.Tree
	// Lock is the parsed output manifest, nil when the tree carries none.
	Lock *manifest.Lock
	// Declarations is the output plan the artifact projection attributes paths to.
	Declarations []OutputDeclaration
	// Eligible is the coverage universe: every scannable snapshot file that is
	// not a generated output (a lock entry), not matched by a contextIgnore
	// glob, not under a resident root, and not inside a nested adopter tree.
	Eligible []string
}

// ContextState assembles the working-tree universe. The universe project it
// derives the catalog, targets, and layout from is built from the snapshot's
// own configuration rather than the caller's, so the query answers about the
// tree it was given.
func (p *Project) ContextState(ctx context.Context) (ContextState, error) {
	ws, err := p.workingCurrentState(ctx)
	if err != nil {
		return ContextState{}, err
	}
	universe := &Project{Root: p.Root, Cfg: ws.Cfg, standard: p.standard, repo: p.repo}
	universe.Targets, err = resolveTargets(ws.Cfg.Targets)
	if err != nil {
		return ContextState{}, err
	}
	universe.Cat, err = universe.effectiveCatalog()
	if err != nil {
		return ContextState{}, err
	}
	declarations, err := BuildOutputDeclarations(ws.Cfg, universe.Cat, universe.Targets, snapshotTreeReader{tree: ws.Tree}, adr.NewCorpus(ws.Loaded.ADRs))
	if err != nil { // coverage-ignore: the snapshot-local catalog and every declaration input were already parsed from this immutable tree
		return ContextState{}, err
	}
	return ContextState{Layout: universe.layout(), Cfg: ws.Cfg, Loaded: ws.Loaded, Tree: ws.Tree, Lock: ws.Lock, Declarations: declarations, Eligible: eligiblePaths(ws.Tree, ws.Lock, ws.Cfg.ContextIgnore)}, nil
}

// StagedContextState assembles the index universe at root. It deliberately
// never loads working-tree configuration: the staged answer is computed
// entirely from what is staged.
func StagedContextState(ctx context.Context, root string) (ContextState, error) {
	p, err := openRootProject(root)
	if err != nil {
		return ContextState{}, err
	}
	state, err := p.indexCurrentState(ctx)
	if err != nil {
		return ContextState{}, err
	}
	targets, err := resolveTargets(state.Cfg.Targets)
	if err != nil {
		return ContextState{}, err
	}
	universe := &Project{Root: root, Cfg: state.Cfg, Targets: targets, standard: catalog.Standard, repo: p.repo}
	universe.Cat, err = universe.effectiveCatalog()
	if err != nil {
		return ContextState{}, err
	}
	declarations, err := BuildOutputDeclarations(state.Cfg, universe.Cat, universe.Targets, snapshotTreeReader{tree: state.Tree}, adr.NewCorpus(state.Loaded.ADRs))
	if err != nil { // coverage-ignore: the staged snapshot-local catalog and every declaration input were already parsed from this immutable tree
		return ContextState{}, err
	}
	return ContextState{Layout: universe.layout(), Cfg: state.Cfg, Loaded: state.Loaded, Tree: state.Tree, Lock: state.Lock, Declarations: declarations, Eligible: eligiblePaths(state.Tree, state.Lock, state.Cfg.ContextIgnore)}, nil
}

// indexState is one loaded index universe: the parsed ADR/topic view, the Tree
// it came from, the lock, and the staged configuration.
type indexState struct {
	Loaded currentstate.Loaded
	Tree   *snapshot.Tree
	Lock   *manifest.Lock
	Cfg    *config.Config
}

// indexCurrentState loads the staged ADR/topic view from the index tree.
func (p *Project) indexCurrentState(ctx context.Context) (indexState, error) {
	tree, err := p.indexTree(ctx)
	if err != nil {
		return indexState{}, err
	}
	lock, err := lockFromTree(tree)
	if err != nil {
		return indexState{}, err
	}
	boundaries, gaps := attestationBoundaries(lock)
	loaded, cfg, err := loadTreeCurrentState(p.Root, tree, lock, boundaries, gaps)
	if err != nil {
		return indexState{}, err
	}
	if cfg == nil {
		return indexState{}, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	return indexState{Loaded: loaded, Tree: tree, Lock: lock, Cfg: cfg}, nil
}
