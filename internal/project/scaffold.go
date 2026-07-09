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
// The second return value lists the closure additions beyond a trim's
// explicit selection (kind-prefixed, e.g. "skill reviewing-plan-resync"),
// empty for the untrimmed default.
func ScaffoldConfig(prefix string, vars map[string]string, trim *config.CatalogTrim, scopes []string) ([]byte, []string, error) {
	cat := catalog.Standard

	// Collect referenced var names from every catalog template family — not only
	// the core ones — so an opt-in target added later renders without <no value>.
	// invariant: scaffold-seeds-all-vars
	varSet := map[string]bool{}
	for _, kind := range []string{"skills", "agents", "docs"} {
		d, _ := descriptorByPlural(kind)
		for _, name := range d.poolNames(cat) {
			if err := collectVars(templates.FS, d.tid(name), varSet); err != nil { // coverage-ignore: every catalog name has a backing template in the embedded FS, so collectVars cannot fail
				return nil, nil, err
			}
		}
	}
	// Plain singletons (workflow, doc-standard, agents-md-standard included) always
	// render — their vars must be seeded even though they left cat.Docs (ADR-0043).
	for _, sg := range plainSingletons {
		if err := collectVars(templates.FS, sg.tid, varSet); err != nil { // coverage-ignore: every plainSingletons entry has a backing template in the embedded FS, so collectVars cannot fail
			return nil, nil, err
		}
	}
	// Hook payloads render by default (ADR-0048) — seed their vars (commitGateCmd)
	// so an init prompt answer is not silently dropped.
	for _, name := range hookNames {
		if err := collectVars(templates.FS, "hooks/"+name+".sh.tmpl", varSet); err != nil { // coverage-ignore: every hookNames entry has a backing template in the embedded FS, so collectVars cannot fail
			return nil, nil, err
		}
	}
	varNames := slices.Sorted(maps.Keys(varSet))

	// Enable the core skills; under the untrimmed default agents are all
	// enabled (every one is workflow-essential). No core docs remain —
	// workflow/doc-standard/agents-md-standard are mandatory singletons
	// (ADR-0043), not toggleable.
	// invariant: scaffold-core-only
	var skillNames, docNames []string
	for name, spec := range cat.Skills {
		if spec.Core {
			skillNames = append(skillNames, name)
		}
	}
	agentNames := slices.Sorted(maps.Keys(cat.Agents))
	// A non-nil trim dimension (ADR-0029 full-deselectable catalog trim) replaces
	// the curated-core default, then is closure-completed and its agents derived
	// from the selection's requirements (ADR-0081 Decision 9) — without the
	// derivation, the always-enabled plan-reviewer's edge would silently
	// re-complete any planning-core trim. Additions beyond the selection are
	// returned so init can note each one.
	// invariant: catalog-trim-applied
	// invariant: init-set-closed
	var added []string
	if trim != nil && trim.Docs != nil {
		docNames = slices.Clone(*trim.Docs)
	}
	if trim != nil && trim.Skills != nil {
		selected := map[catalog.Node]bool{}
		seeds := make([]catalog.Node, 0, len(*trim.Skills))
		for _, s := range *trim.Skills {
			n := catalog.Node{Kind: "skill", Name: s}
			selected[n] = true
			seeds = append(seeds, n)
		}
		for _, d := range docNames {
			selected[catalog.Node{Kind: "doc", Name: d}] = true
		}
		skillNames, agentNames = nil, nil
		for _, n := range catalog.Closure(cat, seeds) {
			switch n.Kind {
			case "skill":
				skillNames = append(skillNames, n.Name)
			case "agent":
				agentNames = append(agentNames, n.Name)
			case "doc":
				if !selected[n] {
					docNames = append(docNames, n.Name)
				}
			}
			if !selected[n] {
				added = append(added, n.Kind+" "+n.Name)
			}
		}
	}
	slices.Sort(skillNames)
	slices.Sort(docNames)
	slices.Sort(agentNames)
	slices.Sort(added)

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
	out, err := config.MarshalSkeleton(config.Skeleton{
		Prefix:    prefix,
		Vars:      seeded,
		Skills:    skillNames,
		Agents:    agentNames,
		Docs:      docNames,
		Audit:     auditBlk,
		Bootstrap: &config.BootstrapConfig{Enabled: true},
		Hooks:     &config.HooksConfig{Enabled: true},
	})
	if err != nil { // coverage-ignore: MarshalSkeleton serializes an in-memory struct; it cannot fail on this input
		return nil, nil, err
	}
	return out, added, nil
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
