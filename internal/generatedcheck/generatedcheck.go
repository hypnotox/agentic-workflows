// Package generatedcheck owns generated-output conformance policy.
package generatedcheck

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// PropertyReproducibility is the protected property for generated-output findings.
const PropertyReproducibility checkresult.Property = "reproducibility"

// ReadFile reads one prepared working-universe output.
type ReadFile func(string) ([]byte, error)

// IndexPaths lists prepared Git-index paths without exposing a repository.
type IndexPaths func(context.Context) ([]string, error)

func result(findings []checkresult.Finding, information []checkresult.Information) (checkresult.Result, error) {
	return checkresult.New(findings, information)
}
func errorFinding(kind, path, detail string) checkresult.Finding {
	return checkresult.Finding{Rank: severity.Error, Property: PropertyReproducibility, Evidence: checkresult.Evidence{Kind: kind, Path: path, Detail: detail}}
}
func information(kind, path, detail string) checkresult.Information {
	return checkresult.Information{Evidence: checkresult.Evidence{Kind: kind, Path: path, Detail: detail}}
}

// Tracking checks index membership without retaining a repository object.
func Tracking(ctx context.Context, nested bool, paths IndexPaths, plan outputplan.Plan) (checkresult.Result, error) {
	if paths == nil {
		return result(nil, []checkresult.Information{information("tracking", "", "generated-artifact tracking is unavailable outside a Git repository")})
	}
	indexed, err := paths(ctx)
	if err != nil {
		return checkresult.Result{}, err
	}
	present := map[string]bool{}
	for _, path := range indexed {
		present[path] = true
	}
	required := requiredPaths(nested, plan)
	var findings []checkresult.Finding
	for _, path := range slices.Sorted(maps.Keys(required)) {
		if !present[path] {
			findings = append(findings, errorFinding("untracked", path, "generated artifact is absent from the Git index; run awf render, then git add -f "+path))
		}
	}
	return result(findings, nil)
}

func requiredPaths(nested bool, plan outputplan.Plan) map[string]bool {
	required := map[string]bool{config.DirName + "/awf.lock": true}
	for _, output := range plan.Outputs() {
		if !nested || !resident.IsResidentPath(output.Path()) {
			required[output.Path()] = true
		}
	}
	return required
}

// Locked compares the prepared output plan with a lock and observed output bytes.
func Locked(nested bool, lock *manifest.Lock, plan outputplan.Plan, read ReadFile, tracking checkresult.Result) (checkresult.Result, error) {
	untracked := map[string]bool{}
	for _, f := range tracking.Findings() {
		if f.Evidence.Kind == "untracked" {
			untracked[f.Evidence.Path] = true
		}
	}
	rendered := map[string]outputplan.Output{}
	for _, output := range plan.Outputs() {
		rendered[output.Path()] = output
	}
	var findings []checkresult.Finding
	for _, path := range slices.Sorted(maps.Keys(rendered)) {
		if _, ok := lock.Files[path]; !ok {
			findings = append(findings, errorFinding("unsynced", path, "enabled but not in lock; run awf render"))
		}
	}
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		output, ok := rendered[path]
		entry := lock.Files[path]
		if !ok {
			findings = append(findings, errorFinding("orphaned", path, "in lock but no longer produced"))
			continue
		}
		if output.Policy().Regenerate {
			observed, err := read(path)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) && untracked[path] {
					continue
				}
				findings = append(findings, errorFinding("missing", path, "file absent; run awf render"))
				continue
			}
			if manifest.Hash(observed) != manifest.Hash([]byte(output.Content())) {
				kind, detail := "stale", "generated output out of date; run awf render"
				if output.TemplateID() != "" {
					kind, detail = "hand-edited", "on-disk output differs from the regenerated file; run awf render to restore awf-owned regions"
				}
				findings = append(findings, errorFinding(kind, path, detail))
			}
			continue
		}
		if output.TemplateHash() != entry.TemplateHash || output.ConfigHash() != entry.ConfigHash {
			findings = append(findings, errorFinding("stale", path, "template or config changed; run awf render"))
			continue
		}
		if manifest.Hash([]byte(output.Content())) != entry.OutputHash {
			findings = append(findings, errorFinding("stale", path, "rendered output out of date; run awf render"))
			continue
		}
		observed, err := read(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && untracked[path] {
				continue
			}
			findings = append(findings, errorFinding("missing", path, "file absent; run awf render"))
			continue
		}
		if manifest.Hash(observed) != entry.OutputHash {
			findings = append(findings, errorFinding("hand-edited", path, "on-disk output differs from lock; run awf render to discard the edit, or move it into a .awf convention part to keep it"))
			continue
		}
		if output.Policy().ValidateFrontmatter {
			if err := ValidateFrontmatter(observed); err != nil {
				findings = append(findings, errorFinding("invalid-frontmatter", path, err.Error()))
			}
		}
	}
	return result(findings, nil)
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// ValidateFrontmatter validates generated skill frontmatter.
func ValidateFrontmatter(content []byte) error {
	var fm skillFrontmatter
	_, found, err := frontmatter.Parse(content, &fm)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("missing frontmatter")
	}
	if strings.TrimSpace(fm.Name) == "" {
		return errors.New("frontmatter name is empty")
	}
	if strings.TrimSpace(fm.Description) == "" {
		return errors.New("frontmatter description is empty")
	}
	return nil
}

// Staged compares one Publisher plan entirely in its prepared index universe.
func Staged(nested bool, lock *manifest.Lock, plan outputplan.Plan, read outputplan.TreeReader, indexed map[string]bool) (checkresult.Result, error) {
	required := requiredPaths(nested, plan)
	staged := map[string][]byte{}
	for path := range required {
		if !indexed[path] {
			continue
		}
		bytes, _, err := read.ReadFile(path)
		if err != nil {
			return checkresult.Result{}, err
		}
		staged[path] = bytes
	}
	var drift []manifest.Drift
	for _, path := range slices.Sorted(maps.Keys(required)) {
		if !indexed[path] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "untracked", Detail: "generated artifact is absent from the Git index; run awf render, then git add -f " + path})
		}
	}
	if lock == nil {
		return stagedResult(drift)
	}
	outputs := map[string]outputplan.Output{}
	for _, out := range plan.Outputs() {
		outputs[out.Path()] = out
	}
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		output, ok := outputs[path]
		if !ok || (nested && resident.IsResidentPath(path)) || !indexed[path] {
			continue
		}
		entry := lock.Files[path]
		if output.Policy().Regenerate {
			if manifest.Hash(staged[path]) != manifest.Hash([]byte(output.Content())) {
				kind, detail := "stale", "generated output out of date; run awf render"
				if output.TemplateID() != "" {
					kind, detail = "hand-edited", "staged output differs from the regenerated file; run awf render to restore awf-owned regions"
				}
				drift = append(drift, manifest.Drift{Path: path, Kind: kind, Detail: detail})
			}
			continue
		}
		if output.TemplateHash() != entry.TemplateHash || output.ConfigHash() != entry.ConfigHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "stale", Detail: "template or config changed; run awf render"})
			continue
		}
		if manifest.Hash([]byte(output.Content())) != entry.OutputHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "stale", Detail: "rendered output out of date; run awf render"})
			continue
		}
		if manifest.Hash(staged[path]) != entry.OutputHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "hand-edited", Detail: "staged output differs from lock; run awf render to discard the edit, or move it into a .awf convention part to keep it"})
		}
	}
	sort.SliceStable(drift, func(i, j int) bool { return drift[i].Path < drift[j].Path })
	return stagedResult(drift)
}

func stagedResult(drift []manifest.Drift) (checkresult.Result, error) {
	findings := make([]checkresult.Finding, len(drift))
	for i, item := range drift {
		findings[i] = errorFinding(item.Kind, item.Path, item.Detail)
	}
	return result(findings, nil)
}

// GuideSizeAdvisory reports the fixed guide-size heuristic finding.
func GuideSizeAdvisory(plan outputplan.Plan) (checkresult.Result, error) {
	for _, output := range plan.Outputs() {
		if output.Path() == "AGENTS.md" && len(output.Content()) > 12*1024 {
			return result([]checkresult.Finding{{Rank: severity.Warn, Property: "heuristic-quality", Evidence: checkresult.Evidence{Kind: "advisory", Detail: fmt.Sprintf("AGENTS.md is %d bytes, allowed %d bytes; see docs/agents-md-standard.md", len(output.Content()), 12*1024)}}}, nil)
		}
	}
	return result(nil, nil)
}
