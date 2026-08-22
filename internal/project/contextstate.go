package project

import (
	"context"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// ContextPreparation holds the selected index universe required to compare a
// staged Publisher plan against the index. It is deliberately not a contextq
// input: currentstatecoord owns context composition.
type ContextPreparation struct {
	State  *projectstate.ProjectState
	Config *config.Config
	Reader outputplan.TreeReader
	tree   *snapshot.Tree
	lock   *manifest.Lock
}

// PrepareStagedContextState loads one index-only snapshot for Publisher drift
// checking. It remains separate from currentstatecoord because generated drift
// comparison is owned by project.
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
	return &ContextPreparation{State: lower, Config: state.Cfg, Reader: snapshotTreeReader{tree: state.Tree}, tree: state.Tree, lock: state.Lock}, nil
}

func catalogForProfile(complete *catalog.Catalog, cfg *config.Config) *catalog.Catalog {
	return catalog.NewProfileView(complete, cfg.Profile).Catalog()
}

// indexState is one loaded index universe.
type indexState struct {
	Tree *snapshot.Tree
	Lock *manifest.Lock
	Cfg  *config.Config
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
	_, cfg, err := loadTreeCurrentState(root, tree, lock)
	if err != nil {
		return indexState{}, err
	}
	if cfg == nil {
		return indexState{}, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	return indexState{Tree: tree, Lock: lock, Cfg: cfg}, nil
}
