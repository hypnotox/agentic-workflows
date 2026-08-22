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
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// ContextPreparation selects one immutable tree, config, lock, reader, and
// publisher state. Publisher receives these exact values and completion never
// reparses their ADR, topic, or plan corpora.
type ContextPreparation struct {
	State    *projectstate.ProjectState
	Config   *config.Config
	Reader   outputplan.TreeReader
	layout   contextinput.Layout
	tree     *snapshot.Tree
	lock     *manifest.Lock
	eligible []string
}

// PrepareWorkingContext selects the working universe once for context output.
func PrepareWorkingContext(state *projectstate.ProjectState, repo *awfgit.Repo, ctx context.Context) (*ContextPreparation, error) {
	if state == nil {
		return nil, errors.New("context preparation: missing project state")
	}
	tree, err := workingTree(state.Root(), repo, ctx)
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
	return newContextPreparation(state, cfg, tree, lock), nil
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
	return newContextPreparation(state, cfg, tree, lock), nil
}

// Tree returns the immutable selected snapshot for a project-owned comparison.
func (p *ContextPreparation) Tree() *snapshot.Tree { return p.tree }

// Lock returns a defensive projection for a project-owned comparison.
func (p *ContextPreparation) Lock() *manifest.Lock { return p.lock.Clone() }

func newContextPreparation(state *projectstate.ProjectState, cfg *config.Config, tree *snapshot.Tree, lock *manifest.Lock) *ContextPreparation {
	cat := state.Catalog()
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
	return &ContextPreparation{State: state, Config: cfg, Reader: snapshotReader{tree}, layout: contextinput.Layout{DocsDir: config.DocsDir, ADRDir: config.DocsDir + "/decisions", IndexMd: config.DocsDir + "/decisions/INDEX.md", PlansDir: config.DocsDir + "/plans", DomainsDir: config.DocsDir + "/domains", Docs: docs, Singletons: singletons}, tree: tree, lock: lock, eligible: eligiblePaths(tree, lock, cfg.ContextIgnore)}
}

// CompleteContext consumes Publisher's defensive semantic projections. The
// coordinator deliberately depends only on neutral values, never Publisher.
func CompleteContext(prep *ContextPreparation, corpus adr.Corpus, topics topic.Corpus, plans []plan.Plan, declarations []outputplan.Declaration) contextinput.Input {
	loaded := currentstate.Loaded{ADRs: corpus.All(), Corpus: corpus.Clone(), Topics: topics.Clone()}
	return contextinput.New(prep.layout, loaded, contextinput.NewPlanContext(plans, corpus), prep.tree, prep.lock, declarations, prep.eligible, prep.Config.ContextIgnore)
}

type snapshotReader struct{ tree *snapshot.Tree }

func (r snapshotReader) ReadFile(path string) ([]byte, bool, error) {
	f, ok := r.tree.Lookup(filepath.ToSlash(path))
	if !ok || !f.Scannable() {
		return nil, false, nil
	}
	return slices.Clone(f.Bytes), true, nil
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
