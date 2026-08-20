package project

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// CheckStagedDriftRoot compares rendered output entirely within the staged
// universe without loading working-tree project configuration.
func CheckStagedDriftRoot(ctx context.Context, root string) ([]manifest.Drift, error) {
	repo, prefix, err := awfgit.OpenContaining(root)
	if err != nil {
		return nil, err
	}
	p := stagedProject(root, prefix)
	return checkStagedDrift(p, repo, ctx)
}

// CheckStagedDrift renders from the index configuration and compares generated
// output membership plus stale and hand-edited properties against that same index.
func checkStagedDrift(p renderInputs, repo *awfgit.Repo, ctx context.Context) ([]manifest.Drift, error) {
	tree, err := indexTree(p.root(), repo, ctx)
	if err != nil {
		return nil, err
	}
	lock, _, err := optionalLockFromTree(tree)
	if err != nil {
		return nil, err
	}
	loaded, cfg, err := loadTreeCurrentState(p.root(), tree, lock)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	state := indexState{Loaded: loaded, Tree: tree, Lock: lock, Cfg: cfg}
	targets, err := resolveTargets(KnownTargets())
	if err != nil { // coverage-ignore: KnownTargets is the closed built-in descriptor set and its resolution is exhaustively validated by target tests
		return nil, err
	}
	read := snapshotTreeReader{tree: state.Tree}
	// The staged universe always selects from the complete immutable catalog
	// injected into p. It may describe a differently profiled working tree, so
	// reusing projectCatalog(p) would make Core -> Full staged validation lose the Full
	// layer, while consulting the global catalog would discard injected entries.
	completeCat := completeProjectCatalog(p)
	selected := catalog.NewProfileView(completeCat, state.Cfg.Profile).Catalog()
	facts, err := config.NewFacts(state.Cfg)
	if err != nil { // coverage-ignore: loadTreeCurrentState already parsed semantic config data
		return nil, err
	}
	universeState := &projectState{
		invokingRoot: p.root(),
		roots:        p.residentRoots(),
		nested:       p.isNested(),
		facts:        facts,
		selectedCat:  catalog.NewProfileView(selected, catalog.ProfileFull),
		completeCat:  catalog.NewProfileView(completeCat, catalog.ProfileFull),
		targets:      cloneTargets(targets),
	}
	universe := newRenderInputs(universeState, state.Cfg, read)
	if err := validateAgainstCatalog(universe); err != nil {
		return nil, err
	}
	effective := map[string]bool{}
	for name := range projectCatalog(universe).Skills {
		effective[name] = true
	}
	pitfalls, err := loadPitfallCorpus(universe)
	if err != nil {
		return nil, err
	}
	op, err := outputPlanWithPitfalls(universe, ctx, state.Loaded.Corpus, pitfalls, state.Loaded.Topics, effective)
	if err != nil {
		return nil, err
	}
	rendered := map[string]RenderedFile{}
	for _, file := range op.writeFiles() {
		rendered[file.Path] = file
	}
	indexed := map[string]bool{}
	for _, file := range state.Tree.List() {
		indexed[file.Path] = true
	}
	return checkStagedRenderedFiles(state.Lock, rendered, read, indexed, !p.isNested())
}

// checkStagedRenderedFiles intentionally has no structural drift branches.
// Missing, orphaned, unsynced, invalid-frontmatter, and other repo-only kinds
// are outside the staged rendered-output comparison. Membership is the one
// exception: every planned write and the separately written lock must be indexed.
func checkStagedRenderedFiles(lock *manifest.Lock, rendered map[string]RenderedFile, read ProjectTreeReader, indexed map[string]bool, includeResident bool) ([]manifest.Drift, error) {
	required := map[string]bool{config.DirName + "/awf.lock": true}
	for _, file := range rendered {
		if !includeResident && resident.IsResidentPath(file.Path) {
			continue
		}
		required[file.Path] = true
	}
	stagedBytes := map[string][]byte{}
	for path := range required {
		if !indexed[path] {
			continue
		}
		contents, _, err := read.ReadFile(path)
		if err != nil {
			return nil, err
		}
		stagedBytes[path] = contents
	}
	var drift []manifest.Drift
	for _, path := range slices.Sorted(maps.Keys(required)) {
		if !indexed[path] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "untracked", Detail: "generated artifact is absent from the Git index; run awf render, then git add -f " + path})
		}
	}
	if lock == nil {
		return drift, nil
	}
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		file, produced := rendered[path]
		if !produced {
			continue
		}
		if !includeResident && resident.IsResidentPath(path) {
			continue
		}
		if !indexed[path] {
			continue
		}
		entry := lock.Files[path]
		if file.Policy.Regenerate {
			if manifest.Hash(stagedBytes[path]) != manifest.Hash([]byte(file.Content)) {
				kind, detail := "stale", "generated output out of date; run awf render"
				if file.TemplateID != "" {
					kind, detail = "hand-edited", "staged output differs from the regenerated file; run awf render to restore awf-owned regions"
				}
				drift = append(drift, manifest.Drift{Path: path, Kind: kind, Detail: detail})
			}
			continue
		}
		if file.TemplateHash != entry.TemplateHash || file.ConfigHash != entry.ConfigHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "stale", Detail: "template or config changed; run awf render"})
			continue
		}
		if finding, found := classifyFrozenOutputFreshness(file, entry); found {
			drift = append(drift, finding)
			continue
		}
		if finding, found := classifyFrozenObservedDrift(file, entry, stagedBytes[path], "staged output differs from lock; run awf render to discard the edit, or move it into a .awf convention part to keep it"); found {
			drift = append(drift, finding)
		}
	}
	slices.SortFunc(drift, func(a, b manifest.Drift) int { return strings.Compare(a.Path, b.Path) })
	return drift, nil
}
