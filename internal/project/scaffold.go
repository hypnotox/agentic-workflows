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
// skill renders cleanly. It also seeds the self-pinning bootstrap (ADR-0040)
// and the git-hook payloads (ADR-0048) enabled by default, and writes a resolved
// commit-scope list to audit.allowedScopes (ADR-0051).
func ScaffoldConfig(prefix string, vars map[string]string, inv *config.InvariantConfig, trim *config.CatalogTrim, scopes []string) ([]byte, error) {
	cat := catalog.Standard

	// Collect referenced var names from every catalog template family — not only
	// the core ones — so an opt-in target added later renders without <no value>.
	// invariant: scaffold-seeds-all-vars
	varSet := map[string]bool{}
	for _, kind := range []string{"skills", "agents", "docs"} {
		d, _ := descriptorByPlural(kind)
		for _, name := range d.poolNames(cat) {
			if err := collectVars(templates.FS, d.tid(name), varSet); err != nil { // coverage-ignore: every catalog name has a backing template in the embedded FS, so collectVars cannot fail
				return nil, err
			}
		}
	}
	// Plain singletons (workflow, doc-standard, agents-md-standard included) always
	// render — their vars must be seeded even though they left cat.Docs (ADR-0043).
	for _, sg := range plainSingletons {
		if err := collectVars(templates.FS, sg.tid, varSet); err != nil { // coverage-ignore: every plainSingletons entry has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}
	// Hook payloads render by default (ADR-0048) — seed their vars (commitGateCmd)
	// so an init prompt answer is not silently dropped.
	for _, name := range hookNames {
		if err := collectVars(templates.FS, "hooks/"+name+".sh.tmpl", varSet); err != nil { // coverage-ignore: every hookNames entry has a backing template in the embedded FS, so collectVars cannot fail
			return nil, err
		}
	}
	varNames := slices.Sorted(maps.Keys(varSet))

	// Enable the core skills; agents are all enabled (every one is
	// workflow-essential). No core docs remain — workflow/doc-standard/
	// agents-md-standard are mandatory singletons (ADR-0043), not toggleable.
	// invariant: scaffold-core-only
	var skillNames, docNames []string
	for name, spec := range cat.Skills {
		if spec.Core {
			skillNames = append(skillNames, name)
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
	// A non-empty resolved commitScopes answer becomes the audit block; an empty
	// answer writes nothing — nil audit.allowedScopes = accept any (ADR-0017,
	// ADR-0051 Decision 2).
	// invariant: audit-scopes-descriptor-routed
	var auditBlk *config.SkeletonAudit
	if len(scopes) > 0 {
		auditBlk = &config.SkeletonAudit{AllowedScopes: scopes}
	}
	return config.MarshalSkeleton(config.Skeleton{
		Prefix:     prefix,
		Vars:       seeded,
		Skills:     skillNames,
		Agents:     agentNames,
		Docs:       docNames,
		Audit:      auditBlk,
		Invariants: inv,
		Bootstrap:  &config.BootstrapConfig{Enabled: true},
		Hooks:      &config.HooksConfig{Enabled: true},
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
