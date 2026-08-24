package publisher

import (
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
func consumedParts(p renderInputs, kind, artifact string, plan map[string]render.SectionPlan) []string {
	var paths []string
	for sec, sp := range plan {
		if sp.HasPart {
			paths = append(paths, p.cfg.PartPath(kind, artifact, sec))
		}
	}
	return paths
}

// artifactConfigHash projects the drift signal onto one rendered file: the prefix, the
// subset of vars the assembled template references, the artifact's sidecar (marshalled),
// and the bytes of every convention part it consumed - in deterministic order.
func artifactConfigHash(p renderInputs, assembled string, sc config.Sidecar, partPaths []string, _ map[string]bool, targets ...Target) (string, error) {
	refs := render.ReferencedVars(assembled)
	proj := map[string]any{
		"prefix":  p.cfg.Prefix,
		"profile": p.cfg.Profile,
		"layout":  layout(p).templateMap(),
	}
	if len(targets) != 0 {
		// Identity is a declarer property of an output-plan node, not part of
		// its recipe. Hash only output-affecting target descriptor fields.
		t := targets[0]
		caps := slices.Clone(t.Capabilities)
		slices.Sort(caps)
		proj["target"] = struct {
			SkillDir, AgentDir, AgentSuffix string
			AgentDialect                    AgentDialect
			BridgeFile, BridgeTemplate      string
			Capabilities                    []Capability
			Outputs                         []TargetOutput
		}{t.SkillDir, t.AgentDir, t.AgentSuffix, t.AgentDialect, t.BridgeFile, t.BridgeTemplate, caps, t.Outputs}
	}
	vs := map[string]any{}
	for _, r := range refs {
		vs[r] = p.cfg.Vars[r]
	}
	proj["vars"] = vs
	if render.ReferencesCommitPolicy(assembled) {
		// Only consumers fold the normalized typed policy. This keeps unrelated
		// outputs and adopters with no policy byte-stable.
		proj["commitPolicy"] = p.cfg.CommitPolicy
	}
	// A template that reads .commitScopes re-renders when audit.allowedScopes
	// changes; folding the resolved list in flags it stale (ADR-0051).
	foldScopes := render.ReferencesScopes(assembled)
	proj["sidecar"] = sc
	sort.Strings(partPaths)
	parts := map[string]string{}
	for _, pp := range partPaths {
		b, err := p.cfg.ReadPartPath(pp)
		if err != nil {
			return "", err
		}
		// The detectors read the stripped body (ADR-0121 Decision 2): a
		// placeholder mentioned only inside an authoring comment never renders,
		// so it must not fold config into the hash. The hash itself stays over
		// the raw on-disk bytes below.
		stripped, serr := render.StripAuthoringComments(string(b))
		if serr != nil {
			return "", serr
		}
		if render.ReferencesScopePlaceholder(stripped) {
			// A convention part using {{=awf:commitScope*}} re-renders when
			// audit.allowedScopes changes (ADR-0057).
			foldScopes = true
		}
		parts[filepath.Base(filepath.Dir(pp))+"/"+filepath.Base(pp)] = manifest.Hash(b)
	}
	proj["parts"] = parts
	if foldScopes {
		proj["commitScopes"] = audit.Resolve(config.AuditScopes(p.cfg.Audit)).AllowedScopes
	}
	enc, err := yaml.Marshal(proj)
	if err != nil { // coverage-ignore: proj holds only YAML-sourced, marshalable values; yaml.Marshal cannot fail here
		return "", err
	}
	return manifest.Hash(enc), nil
}
