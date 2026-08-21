package project

import (
	"context"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// ContextState is the immutable loaded universe consumed by contextq.
type ContextState struct {
	Layout       Layout
	Cfg          *config.Config
	Loaded       currentstate.Loaded
	PlanState    PlanContext
	Tree         *snapshot.Tree
	Lock         *manifest.Lock
	Declarations []outputplan.Declaration
	Eligible     []string
}

// ContextPreparation holds one snapshot universe while outer composition asks
// Publisher to construct its plan from the same tree.
type ContextPreparation struct {
	State    *projectstate.ProjectState
	Config   *config.Config
	Reader   outputplan.TreeReader
	layout   Layout
	loaded   currentstate.Loaded
	tree     *snapshot.Tree
	lock     *manifest.Lock
	plans    PlanContext
	eligible []string
}

func completeContext(prep *ContextPreparation, plan outputplan.Plan) ContextState {
	return ContextState{Layout: prep.layout, Cfg: prep.Config, Loaded: prep.loaded, PlanState: prep.plans, Tree: prep.tree, Lock: prep.lock, Declarations: plan.Declarations(), Eligible: prep.eligible}
}

// PrepareContextState loads one working snapshot for Publisher and contextq.
func PrepareContextState(state *ProjectState, repo *awfgit.Repo, ctx context.Context) (*ContextPreparation, error) {
	ws, err := workingCurrentState(state.Root(), repo, ctx)
	if err != nil {
		return nil, err
	}
	universe := newRenderInputs(state, ws.Cfg, snapshotTreeReader{tree: ws.Tree})
	plans, err := planContextFromTree(ws.Tree, config.DocsDir, ws.Loaded.Corpus)
	if err != nil { // coverage-ignore: workingCurrentState already parsed this immutable tree and converts parser failures into loaded diagnostics
		return nil, err
	}
	return &ContextPreparation{State: state.state, Config: ws.Cfg, Reader: snapshotTreeReader{tree: ws.Tree}, layout: layout(universe), loaded: ws.Loaded, tree: ws.Tree, lock: ws.Lock, plans: plans, eligible: eligiblePaths(ws.Tree, ws.Lock, ws.Cfg.ContextIgnore)}, nil
}

// CompleteContextState threads the Publisher-produced plan into the prepared universe.
func CompleteContextState(prep *ContextPreparation, plan outputplan.Plan) ContextState {
	return completeContext(prep, plan)
}

// PrepareStagedContextState loads one index-only snapshot for Publisher.
func PrepareStagedContextState(ctx context.Context, root string) (*ContextPreparation, error) {
	repo, prefix, err := awfgit.OpenContaining(root)
	if err != nil {
		return nil, err
	}
	p := stagedProject(root, prefix)
	state, err := indexCurrentState(p.root(), repo, ctx)
	if err != nil {
		return nil, err
	}
	targets, err := resolveTargets(KnownTargets())
	if err != nil { // coverage-ignore: KnownTargets is the same closed built-in registry ResolveTargets projects
		return nil, err
	}
	complete := completeProjectCatalog(p)
	selected := catalogForProfile(complete, state.Cfg)
	facts, err := config.NewFacts(state.Cfg)
	if err != nil { // coverage-ignore: indexCurrentState already parsed this validated semantic config
		return nil, err
	}
	lower := projectstate.NewDerivedWithFacts(p.root(), p.residentRoots(), p.isNested(), facts, selected, complete, targets)
	universeState := &ProjectState{state: lower}
	universe := newRenderInputs(universeState, state.Cfg, snapshotTreeReader{tree: state.Tree})
	plans, err := planContextFromTree(state.Tree, config.DocsDir, state.Loaded.Corpus)
	if err != nil { // coverage-ignore: indexCurrentState already parsed this immutable tree and converts parser failures into loaded diagnostics
		return nil, err
	}
	return &ContextPreparation{State: lower, Config: state.Cfg, Reader: snapshotTreeReader{tree: state.Tree}, layout: layout(universe), loaded: state.Loaded, tree: state.Tree, lock: state.Lock, plans: plans, eligible: eligiblePaths(state.Tree, state.Lock, state.Cfg.ContextIgnore)}, nil
}

func catalogForProfile(complete *catalog.Catalog, cfg *config.Config) *catalog.Catalog {
	return catalog.NewProfileView(complete, cfg.Profile).Catalog()
}

// CompleteStagedContextState threads the staged Publisher plan into contextq.
func CompleteStagedContextState(prep *ContextPreparation, plan outputplan.Plan) ContextState {
	return completeContext(prep, plan)
}

// StagedContextState is intentionally retired: outer composition must build a Publisher plan.

// indexState is one loaded index universe.
type indexState struct {
	Loaded currentstate.Loaded
	Tree   *snapshot.Tree
	Lock   *manifest.Lock
	Cfg    *config.Config
}

func indexCurrentState(root string, repo *awfgit.Repo, ctx context.Context) (indexState, error) {
	tree, err := indexTree(root, repo, ctx)
	if err != nil {
		return indexState{}, err
	}
	var lock *manifest.Lock
	if _, found := tree.Lookup(config.DirName + "/awf.lock"); found {
		lock, err = lockFromTree(tree)
		if err != nil {
			return indexState{}, err
		}
	}
	loaded, cfg, err := loadTreeCurrentState(root, tree, lock)
	if err != nil {
		return indexState{}, err
	}
	if cfg == nil {
		return indexState{}, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	return indexState{loaded, tree, lock, cfg}, nil
}
