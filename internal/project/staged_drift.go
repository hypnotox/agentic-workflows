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
	if err != nil { // coverage-ignore: root construction and index readers are independently covered by staged current-state tests
		return nil, err
	}
	lock, _, err := optionalLockFromTree(tree)
	if err != nil { // coverage-ignore: invalid lock handling is covered by the command staged-lock tests
		return nil, err
	}
	loaded, cfg, err := loadTreeCurrentState(p.Root, tree, lock)
	if err != nil { // coverage-ignore: staged config parsing is covered through indexCurrentState
		return nil, err
	}
	if cfg == nil { // coverage-ignore: staged drift is only selected for adopted projects
		return nil, fmt.Errorf("no staged %s/config.yaml", config.DirName)
	}
	state := indexState{Loaded: loaded, Tree: tree, Lock: lock, Cfg: cfg}
	targets, err := resolveTargets(KnownTargets())
	if err != nil { // coverage-ignore: configured-target validation succeeded and KnownTargets is exhaustively backed by built-in descriptor tests
		return nil, err
	}
	read := snapshotTreeReader{tree: state.Tree}
	universe := &Project{
		Root: p.Root, roots: p.roots, Cfg: state.Cfg, Targets: targets,
		standard: p.standard, read: read, nested: p.nested, repo: p.repo,
	}
	universe.Cat = universe.standard
	if err := universe.validateAgainstCatalog(); err != nil { // coverage-ignore: staged catalog validation is covered by index-current-state callers
		return nil, err
	}
	effective := map[string]bool{}
	for name := range universe.Cat.Skills {
		effective[name] = true
	}
	pitfalls, err := universe.loadPitfallCorpus()
	if err != nil { // coverage-ignore: indexCurrentState and catalog validation already read this immutable staged tree
		return nil, err
	}
	op, err := universe.outputPlanWithPitfalls(ctx, state.Loaded.Corpus, pitfalls, state.Loaded.Topics, effective)
	if err != nil { // coverage-ignore: immutable staged plan construction failures are covered by ordinary plan tests
		return nil, err
	}
	rendered := map[string]RenderedFile{}
	for _, file := range op.writeFiles() {
		rendered[file.Path] = file
	}
	return checkStagedRenderedFiles(state.Lock, rendered, read, !p.nested)
}

// checkStagedRenderedFiles intentionally has no structural drift branches.
// Missing, orphaned, unsynced, invalid-frontmatter, and other repo-only kinds
// are outside the staged rendered-output comparison. Membership is the one
// exception: every planned write and the separately written lock must be indexed.
func checkStagedRenderedFiles(lock *manifest.Lock, rendered map[string]RenderedFile, read ProjectTreeReader, includeResident bool) ([]manifest.Drift, error) {
	required := map[string]bool{config.DirName + "/awf.lock": true}
	for _, file := range rendered {
		if !includeResident && resident.IsResidentPath(file.Path) { // coverage-ignore: nested composition supplies this exclusion before staged rendering
			continue
		}
		required[file.Path] = true
	}
	indexed := map[string]bool{}
	for path := range required {
		_, present, err := read.ReadFile(path)
		if err != nil { // coverage-ignore: snapshot readers return only staged bytes or absence
			return nil, err
		}
		indexed[path] = present
	}
	var drift []manifest.Drift
	for _, path := range slices.Sorted(maps.Keys(required)) {
		if !indexed[path] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "untracked", Detail: "generated artifact is absent from the Git index; run awf render, then git add -f " + path})
		}
	}
	if lock == nil { // coverage-ignore: absent staged lock is an integration-level staged-drift condition
		return drift, nil
	}
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		if !indexed[path] { // coverage-ignore: membership precedence is covered by the required-set loop above
			continue
		}
		if !includeResident && resident.IsResidentPath(path) { // coverage-ignore: nested residents are excluded before staged comparison
			continue
		}
		entry := lock.Files[path]
		file, produced := rendered[path]
		if !produced { // coverage-ignore: non-produced lock entries are outside staged rendered-output comparison
			continue
		}
		if file.Policy.Regenerate {
			staged, present, err := read.ReadFile(path)
			if err != nil { // coverage-ignore: snapshot staged paths cannot produce read faults
				return nil, err
			}
			if present && manifest.Hash(staged) != manifest.Hash([]byte(file.Content)) {
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
		staged, present, err := read.ReadFile(path)
		if err != nil { // coverage-ignore: snapshot staged paths cannot produce read faults
			return nil, err
		}
		if present {
			if finding, found := classifyFrozenObservedDrift(file, entry, staged, "staged output differs from lock; run awf render to discard the edit, or move it into a .awf convention part to keep it"); found {
				drift = append(drift, finding)
			}
		}
	}
	slices.SortFunc(drift, func(a, b manifest.Drift) int {
		if order := strings.Compare(a.Path, b.Path); order != 0 {
			return order
		}
		return strings.Compare(a.Kind, b.Kind) // coverage-ignore: one path cannot carry two staged drift kinds
	})
	return drift, nil
}
