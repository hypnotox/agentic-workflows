package currentstatecoord

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// OutputPreparation selects the index universe used exclusively by staged
// generated-output drift. Session owns the exact selected configuration,
// repository, reader, current facts, and registry view.
type OutputPreparation struct {
	Session *project.Session
	tree    *snapshot.Tree
	lock    *manifest.Lock
}

// PrepareStagedOutput selects only the index universe for staged output drift.
// It never consults working-tree configuration or locks.
func PrepareStagedOutput(ctx context.Context, root string) (*OutputPreparation, error) {
	repo, _, err := awfgit.OpenContaining(root)
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
	reader := snapshotReader{tree: tree}
	load := func(string) (*config.Config, error) { return cfg, nil }
	loader := project.NewLoader(load, catalog.Standard, func(context.Context, string) string { return "" }, repo).WithSelection(load, reader)
	session, err := loader.Load(ctx, root)
	if err != nil {
		return nil, err
	}
	return &OutputPreparation{Session: session, tree: tree, lock: lock}, nil
}

func (p *OutputPreparation) Lock() *manifest.Lock { return p.lock.Clone() }

// Check compares one Publisher plan entirely within this prepared index universe.
func (p *OutputPreparation) Check(plan outputplan.Plan) (checkresult.Result, error) {
	indexed := map[string]bool{}
	for _, file := range p.tree.List() {
		indexed[file.Path] = true
	}
	return generatedcheck.Staged(p.Session.Nested(), p.Lock(), plan, p.Session.Reader(), indexed)
}

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
