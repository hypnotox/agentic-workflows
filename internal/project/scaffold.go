package project

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// ScaffoldConfig generates a .awf/config.yaml with every catalog template's
// referenced vars, the self-pinning bootstrap, and the resolved commit scopes.
func ScaffoldConfig(prefix string, vars map[string]string, scopes []string) ([]byte, error) {
	cat := catalog.CompleteView().Catalog()

	// Collect referenced var names from every selected catalog template family so an
	// opt-in target added later renders without <no value>.
	varSet := map[string]bool{}
	for _, kind := range artifactregistry.Kinds() {
		d, _ := descriptorByPlural(kind.Plural)
		if d.poolNames == nil {
			continue
		}
		for _, name := range d.poolNames(cat) {
			if err := collectVars(templates.FS, d.templateID(cat, name), varSet); err != nil {
				return nil, err
			}
		}
	}
	// Plain singletons (workflow, doc-standard, agents-md-standard included) always
	// render, so their vars must be seeded.
	for _, sg := range plainSingletons(cat) {
		if err := collectVars(templates.FS, sg.tid, varSet); err != nil {
			return nil, err
		}
	}
	// Hook payloads render by default, so their vars must be seeded.
	for _, name := range artifactregistry.Hooks() {
		if err := collectVars(templates.FS, hookTID(name), varSet); err != nil {
			return nil, err
		}
	}
	varNames := slices.Sorted(maps.Keys(varSet))

	seeded := make(map[string]string, len(varNames))
	for _, v := range varNames {
		seeded[v] = vars[v] // resolved value, or "" for an absent/unresolved var
	}
	// A non-empty resolved commitScopes answer becomes the audit block; an empty
	// answer writes nothing, so nil audit.allowedScopes accepts any.
	var auditBlk *config.SkeletonAudit
	if len(scopes) > 0 {
		auditBlk = &config.SkeletonAudit{AllowedScopes: scopes}
	}
	out, err := config.MarshalSkeleton(config.Skeleton{
		Prefix: prefix,
		// The default integration branch a fresh project starts on. It is
		// written, never defaulted in code, so an adopter sees and can change
		// the branch name the scaffold and pending-record block key use.
		IntegrationBranch: "main",
		Vars:              seeded,
		Audit:             auditBlk,
		Bootstrap:         &config.BootstrapConfig{Enabled: true},
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NeededVars returns the var names referenced by the full rendered catalog.
func NeededVars() (map[string]bool, error) {
	return neededVarsFromFS(templates.FS)
}

func neededVarsFromFS(fsys fs.FS) (map[string]bool, error) {
	cat := catalog.CompleteView().Catalog()
	varSet := map[string]bool{}
	for _, kind := range artifactregistry.Kinds() {
		d, _ := descriptorByPlural(kind.Plural)
		if d.poolNames == nil {
			continue
		}
		for _, n := range d.poolNames(cat) {
			if err := collectVars(fsys, d.templateID(cat, n), varSet); err != nil {
				return nil, err
			}
		}
	}
	if err := collectVars(fsys, cat.Docs["agents-doc"].TID, varSet); err != nil {
		return nil, err
	}
	for _, sg := range plainSingletons(cat) {
		if err := collectVars(fsys, sg.tid, varSet); err != nil {
			return nil, err
		}
	}
	for _, name := range artifactregistry.Hooks() {
		if err := collectVars(fsys, hookTID(name), varSet); err != nil {
			return nil, err
		}
	}
	return varSet, nil
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
