package project

import (
	"io/fs"
	"maps"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// Base template ids shared by every synthesized project-local artifact (ADR-0068).
const (
	baseSkillTID = "skills/_base/SKILL.md.tmpl"
	baseAgentTID = "agents/_base.md.tmpl"
)

// ScaffoldVarRefs returns the vars referenced by the base template a new local
// artifact of kind ("skill"/"agent") renders from — `awf new`'s seeding surface
// (ADR-0087 Decision 4). Parts are raw (ADR-0034), so the base template is a
// local artifact's only var channel; today both bases are varless and this
// returns empty, but a future base gaining a var reference is seeded correct
// by construction.
func ScaffoldVarRefs(kind string) ([]string, error) {
	tid := baseSkillTID
	if kind == "agent" {
		tid = baseAgentTID
	}
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil { // coverage-ignore: constant path into the embedded FS
		return nil, err
	}
	expanded, err := render.ExpandIncludes(string(src), templates.FS)
	if err != nil { // coverage-ignore: shipped base templates always expand
		return nil, err
	}
	return render.ReferencedVars(expanded), nil
}

// effectiveCatalog returns a per-project clone of catalog.Standard augmented with
// a synthesized entry for every enabled local (non-Standard) skill/agent — a name
// outside the standard pool that carries a declaring sidecar. The package global
// is never mutated: the maps are cloned before any insert, and existing values
// are only read (ADR-0068).
// invariant: local-catalog-clone
func (p *Project) effectiveCatalog() (*catalog.Catalog, error) {
	// Start from a full struct copy so any Catalog field (present or future) is
	// carried unchanged, then replace only the two maps synthesis mutates with
	// fresh clones. The remaining fields stay shared with the global by value —
	// synthesis never touches them (ADR-0068).
	clone := *catalog.Standard
	clone.Skills = maps.Clone(catalog.Standard.Skills)
	clone.Agents = maps.Clone(catalog.Standard.Agents)
	cat := &clone
	if err := synthesizeLocals(p, cat.Skills, p.Cfg.Skills, "skills", func(n string) catalog.SkillSpec {
		return catalog.SkillSpec{Base: true, Sections: []string{"content"}, Data: localData(n)}
	}); err != nil {
		return nil, err
	}
	if err := synthesizeLocals(p, cat.Agents, p.Cfg.Agents, "agents", func(n string) catalog.TargetSpec {
		return catalog.TargetSpec{Base: true, Sections: []string{"content"}, Data: localData(n)}
	}); err != nil {
		return nil, err
	}
	return cat, nil
}

// synthesizeLocals inserts a base-rendered entry into pool for each enabled name
// that is absent from the standard pool and carries a non-local declaring sidecar.
func synthesizeLocals[T any](p *Project, pool map[string]T, enabled []string, kind string, mk func(string) T) error {
	for _, name := range enabled {
		if _, ok := pool[name]; ok {
			// A standard entry is never overwritten by a local synthesis.
			// invariant: local-no-shadow
			continue
		}
		has, err := p.Cfg.HasSidecar(kind, name)
		if err != nil { // coverage-ignore: HasSidecar only errors on a permission fault a test cannot trigger
			return err
		}
		if !has {
			// Undeclared non-standard name: leave it absent so validateAgainstCatalog
			// rejects it as a typo.
			// invariant: local-requires-declaration
			continue
		}
		sc, err := p.Cfg.Sidecar(kind, name)
		if err != nil {
			return err
		}
		if sc.Local {
			continue // hand-authored opt-out — render and validate already skip it.
		}
		if err := config.ValidateArtifactName(kind, name); err != nil {
			return err
		}
		pool[name] = mk(name)
	}
	return nil
}

// localData is a synthesized local artifact's default render data: its slug (the
// frontmatter name stem). The description falls through from the sidecar, guarded
// by the base template.
func localData(name string) map[string]any {
	return map[string]any{"slug": name}
}
