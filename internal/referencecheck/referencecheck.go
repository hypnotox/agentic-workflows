// Package referencecheck owns managed rendered-reference validity policy.
package referencecheck

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/refs"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// PropertyCorrectness is the protected property for managed link validity.
const PropertyCorrectness checkresult.Property = "correctness"

// PropertyAuthority is the protected property for ADR related-link validity.
const PropertyAuthority checkresult.Property = "authority"

// Exists is the working-universe path-existence capability. It intentionally
// does not expose a filesystem, repository, or project carrier.
type Exists func(string) bool

func finding(property checkresult.Property, kind, path, detail string) checkresult.Finding {
	return checkresult.Finding{Rank: severity.Error, Property: property, Evidence: checkresult.Evidence{Kind: kind, Path: path, Detail: detail}}
}

// Check validates managed Markdown and skill references using the supplied
// semantic output plan and the working-universe existence capability.
func Check(plan outputplan.Plan, prefix string, effectiveSkills map[string]bool, knownSkills map[string]bool, exists Exists) (checkresult.Result, error) {
	var findings []checkresult.Finding
	for _, output := range plan.Outputs() {
		if output.Policy().ScanReferences {
			for _, target := range refs.Links(output.Content()) {
				resolved := filepath.Clean(filepath.Join(filepath.Dir(output.Path()), target))
				if strings.HasPrefix(target, "/") {
					resolved = filepath.Clean(strings.TrimPrefix(target, "/"))
				}
				if strings.HasPrefix(resolved, "../") || filepath.IsAbs(resolved) || exists == nil || !exists(resolved) {
					findings = append(findings, finding(PropertyCorrectness, "dead-reference", output.Path(), target))
				}
			}
		}
		if output.Policy().ScanSkillReferences {
			re := regexp.MustCompile(`(?:^|[^a-zA-Z0-9_-])` + regexp.QuoteMeta(prefix) + `-([a-z0-9]+(?:-[a-z0-9]+)*)`)
			seen := map[string]bool{}
			for _, m := range re.FindAllStringSubmatch(refs.WithoutFences(output.Content()), -1) {
				name := m[1]
				if !knownSkills[name] || effectiveSkills[name] || seen[name] {
					continue
				}
				seen[name] = true
				findings = append(findings, finding(PropertyCorrectness, "dead-skill-reference", output.Path(), prefix+"-"+name))
			}
		}
	}
	return checkresult.New(findings, nil)
}
