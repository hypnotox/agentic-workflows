package project

import (
	"context"
	"maps"
	"slices"

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

// CheckStagedDrift renders from the index configuration and compares only the
// stale and hand-edited rendered-output properties against that same index.
func (p *Project) CheckStagedDrift(ctx context.Context) ([]manifest.Drift, error) {
	state, err := p.indexCurrentState(ctx)
	if err != nil {
		return nil, err
	}
	targets, err := resolveTargets(state.Cfg.Targets)
	if err != nil {
		return nil, err
	}
	read := snapshotTreeReader{tree: state.Tree}
	universe := &Project{
		Root: p.Root, roots: p.roots, Cfg: state.Cfg, Targets: targets,
		standard: p.standard, read: read, nested: p.nested, repo: p.repo,
	}
	universe.Cat, err = universe.effectiveCatalog()
	if err != nil { // coverage-ignore: indexCurrentState already validated every catalog-synthesis sidecar and artifact name from this immutable snapshot
		return nil, err
	}
	if err := universe.validateAgainstCatalog(); err != nil {
		return nil, err
	}
	effective := map[string]bool{}
	for _, name := range state.Cfg.Skills {
		effective[name] = true
	}
	op, err := universe.outputPlan(ctx, state.Loaded.Corpus, state.Loaded.Topics, effective)
	if err != nil {
		return nil, err
	}
	rendered := map[string]RenderedFile{}
	for _, file := range op.writeFiles() {
		rendered[file.Path] = file
	}
	return checkStagedRenderedFiles(state.Lock, rendered, read, !p.nested), nil
}

// checkStagedRenderedFiles intentionally has no structural drift branches.
// Missing, orphaned, unsynced, invalid-frontmatter, and other repo-only kinds
// are outside the staged rendered-output comparison.
func checkStagedRenderedFiles(lock *manifest.Lock, rendered map[string]RenderedFile, read ProjectTreeReader, includeResident bool) []manifest.Drift {
	var drift []manifest.Drift
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		if !includeResident && resident.IsResidentPath(path) {
			continue
		}
		entry := lock.Files[path]
		file, produced := rendered[path]
		if !produced {
			continue
		}
		staged, present := read.ReadFile(path)
		if file.Policy.Regenerate {
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
		if present && manifest.Hash(staged) != entry.OutputHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "hand-edited", Detail: "staged output differs from lock; run awf render to discard the edit, or move it into a .awf convention part to keep it"})
		}
	}
	return drift
}
