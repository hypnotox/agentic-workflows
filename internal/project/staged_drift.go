package project

import (
	"maps"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

// CheckStagedDrift compares one Publisher plan entirely within its prepared index universe.
func CheckStagedDrift(prep *ContextPreparation, plan outputplan.Plan) ([]manifest.Drift, error) {
	rendered := map[string]RenderedFile{}
	for _, output := range plan.Outputs() {
		file := checkFile(output)
		rendered[file.Path] = file
	}
	indexed := map[string]bool{}
	for _, file := range prep.tree.List() {
		indexed[file.Path] = true
	}
	return checkStagedRenderedFiles(prep.lock, rendered, prep.Reader, indexed, !prep.State.Nested())
}

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
		if !produced || (!includeResident && resident.IsResidentPath(path)) || !indexed[path] {
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
