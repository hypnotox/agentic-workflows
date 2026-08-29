package currentstatecoord

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// OutputPreparation selects the index universe used exclusively by staged
// generated-output drift. It carries output-plan inputs, not authority context.
type OutputPreparation struct {
	State  *projectstate.ProjectState
	Config *config.Config
	Reader outputplan.TreeReader
	tree   *snapshot.Tree
	lock   *manifest.Lock
}

// PrepareStagedOutput selects only the index universe for staged output drift.
// It never consults working-tree configuration or locks.
func PrepareStagedOutput(ctx context.Context, root string) (*OutputPreparation, error) {
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
	if err != nil {
		return nil, err
	}
	targets, err := projectstate.ResolveTargets(projectstate.KnownTargets())
	if err != nil {
		return nil, err
	}
	state := projectstate.NewDerivedWithFacts(root, resident.NewRoots(root, ""), prefix != "", facts, selected, complete, targets)
	return &OutputPreparation{State: state, Config: cfg, Reader: snapshotReader{tree: tree}, tree: tree, lock: lock}, nil
}

func (p *OutputPreparation) Tree() *snapshot.Tree { return p.tree }
func (p *OutputPreparation) Lock() *manifest.Lock { return p.lock.Clone() }

type snapshotReader struct{ tree *snapshot.Tree }

func (r snapshotReader) ReadFile(path string) ([]byte, bool, error) {
	f, ok := r.tree.Lookup(filepath.ToSlash(path))
	if !ok || !f.Scannable() {
		return nil, false, nil
	}
	return slices.Clone(f.Bytes), true, nil
}
func (r snapshotReader) PathExists(path string) bool {
	f, ok := r.tree.Lookup(filepath.ToSlash(path))
	return ok && f.Scannable()
}
func (r snapshotReader) Paths(prefix string) ([]string, error) {
	var out []string
	prefix = filepath.ToSlash(prefix)
	for _, f := range r.tree.List() {
		if f.Scannable() && strings.HasPrefix(f.Path, prefix) {
			out = append(out, f.Path)
		}
	}
	return out, nil
}
