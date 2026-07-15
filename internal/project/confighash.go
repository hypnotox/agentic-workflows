package project

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"

	"gopkg.in/yaml.v3"
)

// consumedParts returns the absolute paths of the convention parts an artifact
// consumed; editing any reflags the artifact's drift.
func (p *Project) consumedParts(kind, artifact string, plan map[string]render.SectionPlan) []string {
	var paths []string
	for sec, sp := range plan {
		if sp.HasPart {
			paths = append(paths, p.Cfg.PartPath(kind, artifact, sec))
		}
	}
	return paths
}

// artifactConfigHash projects the drift signal onto one rendered file: the prefix, the
// subset of vars the assembled template references, the artifact's sidecar (marshalled),
// and the bytes of every convention part it consumed - in deterministic order.
func (p *Project) artifactConfigHash(assembled string, sc config.Sidecar, partPaths []string) (string, error) {
	refs := render.ReferencedVars(assembled)
	proj := map[string]any{"prefix": p.Cfg.Prefix, "layout": p.layout().templateMap()}
	vs := map[string]any{}
	for _, r := range refs {
		vs[r] = p.Cfg.Vars[r]
	}
	proj["vars"] = vs
	if render.ReferencesSkills(assembled) {
		// A template that reads .skills re-renders when the enable array
		// changes; folding the effective set in flags it stale (ADR-0046).
		// touches-invariant: skills-set-in-confighash - folds the effective skills set into ConfigHash; proof in drift_test.go
		proj["skills"] = slices.Sorted(maps.Keys(p.effSkills))
	}
	// A template that reads .commitScopes re-renders when audit.allowedScopes
	// changes; folding the resolved list in flags it stale (ADR-0051).
	// invariant: scopes-in-confighash
	foldScopes := render.ReferencesScopes(assembled)
	foldInvariants := render.ReferencesInvariantMarkers(assembled)
	proj["sidecar"] = sc
	sort.Strings(partPaths)
	parts := map[string]string{}
	for _, pp := range partPaths {
		b, err := os.ReadFile(pp)
		if err != nil {
			return "", err
		}
		if render.ReferencesScopePlaceholder(string(b)) {
			// A convention part using {{=awf:commitScope*}} re-renders when
			// audit.allowedScopes changes (ADR-0057).
			// invariant: part-scopes-in-confighash
			foldScopes = true
		}
		if render.ReferencesInvariantMarkerPlaceholder(string(b)) {
			// A convention part using {{=awf:invariantMarker*}} re-renders when
			// invariants.sources changes (ADR-0064).
			// invariant: invariant-markers-in-confighash
			foldInvariants = true
		}
		parts[filepath.Base(filepath.Dir(pp))+"/"+filepath.Base(pp)] = manifest.Hash(b)
	}
	proj["parts"] = parts
	if foldScopes {
		proj["commitScopes"] = audit.Resolve(p.Cfg.Audit).AllowedScopes
	}
	if foldInvariants {
		if p.Cfg.Invariants != nil {
			proj["invariantMarkers"] = p.Cfg.Invariants.Sources
		} else {
			proj["invariantMarkers"] = nil
		}
	}
	enc, err := yaml.Marshal(proj)
	if err != nil { // coverage-ignore: proj holds only YAML-sourced, marshalable values; yaml.Marshal cannot fail here
		return "", err
	}
	return manifest.Hash(enc), nil
}
