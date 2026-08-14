package project

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// CheckStagedDriftRoot compares rendered output entirely within the staged
// universe without loading working-tree project configuration.
func CheckStagedDriftRoot(ctx context.Context, root string) ([]manifest.Drift, error) {
	p, err := openRootProject(root)
	if err != nil {
		return nil, err
	}
	return p.CheckStagedDrift(ctx)
}

// CheckStagedDrift renders from the index configuration and compares generated
// output membership plus stale and hand-edited properties against that same index.
func (p *Project) CheckStagedDrift(ctx context.Context) ([]manifest.Drift, error) {
	tree, err := p.indexTree(ctx)
	if err != nil {
		return nil, err
	}
	lock, _, err := optionalLockFromTree(tree)
	if err != nil {
		return nil, err
	}
	loaded, cfg, err := loadTreeCurrentState(p.Root, tree, lock)
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
	universe := &Project{
		Root: p.Root, roots: p.roots, Cfg: state.Cfg, Targets: targets,
		standard: p.standard, read: read, nested: p.nested, repo: p.repo,
	}
	universe.Cat = universe.standard
	if err := universe.validateAgainstCatalog(); err != nil {
		return nil, err
	}
	effective := map[string]bool{}
	for name := range universe.Cat.Skills {
		effective[name] = true
	}
	pitfalls, err := universe.loadPitfallCorpus()
	if err != nil {
		return nil, err
	}
	op, err := universe.outputPlanWithPitfalls(ctx, state.Loaded.Corpus, pitfalls, state.Loaded.Topics, effective)
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
	return checkStagedRenderedFiles(state.Lock, rendered, read, indexed, !p.nested)
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
