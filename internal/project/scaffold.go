package project

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// ScaffoldConfig generates the bytes of a .awf/config.yaml that enables the
// workflow-core skills and docs (ADR-0022) and every agent in the embedded
// catalog (as flat name arrays), and pre-populates the vars block with
// the union of all {{ .vars.X }} names referenced by every catalog template. Each
// var is seeded with an empty string so that strict render (missingkey=zero +
// <no value> check) does not fail on sync, and so a later `awf add` of an opt-in
// skill renders cleanly. It also seeds the self-pinning bootstrap enabled by
// default (ADR-0040).
func ScaffoldConfig(prefix string, vars map[string]string, inv *config.InvariantConfig, trim *config.CatalogTrim) ([]byte, error) {
	cat, err := catalog.Load(templates.FS)
	if err != nil { // coverage-ignore: catalog.Load over the embedded templates.FS cannot fail at runtime
		return nil, fmt.Errorf("scaffold: load catalog: %w", err)
	}

	// Collect referenced var names from every catalog template family — not only
	// the core ones — so an opt-in target added later renders without <no value>.
	// invariant: scaffold-seeds-all-vars
	varSet := map[string]bool{}
	for name := range cat.Skills {
		path := fmt.Sprintf("skills/%s/SKILL.md.tmpl", name)
		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog skill name has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}
	for name := range cat.Agents {
		path := fmt.Sprintf("agents/%s.md.tmpl", name)
		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog agent name has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}
	for name := range cat.Docs {
		path := fmt.Sprintf("docs/%s.md.tmpl", name)
		if err := collectVars(templates.FS, path, varSet); err != nil { // coverage-ignore: every catalog doc name has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}
	varNames := slices.Sorted(maps.Keys(varSet))

	// Enable the core skills and core docs; agents are all enabled (every
	// one is workflow-essential).
	// invariant: scaffold-core-only
	var skillNames, docNames []string
	for name, spec := range cat.Skills {
		if spec.Core {
			skillNames = append(skillNames, name)
		}
	}
	for name, spec := range cat.Docs {
		if spec.Core {
			docNames = append(docNames, name)
		}
	}
	// A non-nil trim dimension (ADR-0029 full-deselectable catalog trim) replaces the
	// curated-core default verbatim; nil keeps exactly the core (scaffold-core-only).
	// invariant: catalog-trim-applied
	if trim != nil && trim.Skills != nil {
		skillNames = slices.Clone(*trim.Skills)
	}
	if trim != nil && trim.Docs != nil {
		docNames = slices.Clone(*trim.Docs)
	}
	slices.Sort(skillNames)
	slices.Sort(docNames)
	agentNames := slices.Sorted(maps.Keys(cat.Agents))

	seeded := make(map[string]string, len(varNames))
	for _, v := range varNames {
		seeded[v] = vars[v] // resolved value, or "" for an absent/unresolved var
	}
	return config.MarshalSkeleton(config.Skeleton{
		Prefix:     prefix,
		Vars:       seeded,
		Skills:     skillNames,
		Agents:     agentNames,
		Docs:       docNames,
		Invariants: inv,
		Bootstrap:  &config.BootstrapConfig{Enabled: true},
	})
}

// collectVars reads the template at path and adds all .vars.X names to varSet.
func collectVars(fsys fs.FS, path string, varSet map[string]bool) error {
	src, err := fs.ReadFile(fsys, path)
	if err != nil {
		return fmt.Errorf("scaffold: read template %s: %w", path, err)
	}
	for _, v := range render.ReferencedVars(string(src)) {
		varSet[v] = true
	}
	return nil
}
