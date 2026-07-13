package project

import (
	"io/fs"
	"maps"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// Base template ids shared by every synthesized project-local artifact (ADR-0068).
const (
	baseSkillTID = "skills/_base/SKILL.md.tmpl"
	baseAgentTID = "agents/_base.md.tmpl"
	baseDocTID   = "docs/_base.md.tmpl"
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
// touches-invariant: local-catalog-clone — effectiveCatalog clones the skill/agent maps before any insert; proof in local_test.go
func (p *Project) effectiveCatalog() (*catalog.Catalog, error) {
	// Start from a full struct copy so any Catalog field (present or future) is
	// carried unchanged, then replace only the two maps synthesis mutates with
	// fresh clones. The remaining fields stay shared with the global by value —
	// synthesis never touches them (ADR-0068).
	clone := *catalog.Standard
	clone.Skills = maps.Clone(catalog.Standard.Skills)
	clone.Agents = maps.Clone(catalog.Standard.Agents)
	// touches-invariant: local-doc-catalog-clone — the Docs map is cloned before doc synthesis; proof in local_test.go
	clone.Docs = maps.Clone(catalog.Standard.Docs)
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
	if err := synthesizeLocalDocs(p, cat.Docs, p.Cfg.Docs); err != nil {
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
			// touches-invariant: local-no-shadow — a standard skill/agent is never overwritten by local synthesis; proof in local_test.go
			continue
		}
		has, err := p.Cfg.HasSidecar(kind, name)
		if err != nil { // coverage-ignore: HasSidecar only errors on a permission fault a test cannot trigger
			return err
		}
		if !has {
			// Undeclared non-standard name: leave it absent so validateAgainstCatalog
			// rejects it as a typo.
			// touches-invariant: local-requires-declaration — an undeclared name is left absent to fail open; proof in local_test.go
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

// defaultLocalDocDesc is the document-map summary for a local doc whose sidecar
// omits data.description (ADR-0091).
const defaultLocalDocDesc = "Project-local documentation."

// DeriveDocTitle turns a local doc name into a display title: the last path
// segment, hyphens to spaces, each word capitalized, empty words (from a
// trailing or double hyphen) dropped. "guides/release-steps" → "Release Steps".
// awf new doc seeds the sidecar with it, and synthesis falls back to it when the
// sidecar omits data.title.
func DeriveDocTitle(name string) string {
	seg := name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		seg = name[i+1:]
	}
	var words []string
	for _, w := range strings.Split(seg, "-") {
		if w == "" {
			continue
		}
		words = append(words, strings.ToUpper(w[:1])+w[1:])
	}
	return strings.Join(words, " ")
}

// localDocData is a synthesized local doc's render fallback data: the derived
// title and generic description the base template falls back to when the sidecar
// omits the key (a sidecar value overrides via withDefaultData).
func localDocData(name string) map[string]any {
	return map[string]any{"title": DeriveDocTitle(name), "description": defaultLocalDocDesc}
}

// docStringData returns sc.Data[key] as a trimmed string, or "" when absent or
// not a string.
func docStringData(sc config.Sidecar, key string) string {
	if v, ok := sc.Data[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// synthesizeLocalDocs inserts a base-rendered DocEntry into pool for each enabled
// doc name absent from the standard pool that carries a non-local declaring
// sidecar, lifting the sidecar's title/description (with defaults) into the entry
// so the document map lists it (ADR-0091).
func synthesizeLocalDocs(p *Project, pool map[string]catalog.DocEntry, enabled []string) error {
	for _, name := range enabled {
		if _, ok := pool[name]; ok {
			// A standard doc is never overwritten by a local synthesis.
			// touches-invariant: local-doc-no-shadow — a standard doc is never overwritten by local synthesis; proof in local_test.go
			continue
		}
		has, err := p.Cfg.HasSidecar("docs", name)
		if err != nil { // coverage-ignore: HasSidecar only errors on a permission fault a test cannot trigger
			return err
		}
		if !has {
			// Undeclared non-standard name: leave it absent so validateAgainstCatalog rejects it.
			// touches-invariant: local-doc-requires-declaration — an undeclared doc name is left absent to fail open; proof in local_test.go
			continue
		}
		sc, err := p.Cfg.Sidecar("docs", name)
		if err != nil {
			return err
		}
		if sc.Local {
			continue // hand-authored opt-out — render and validate already skip it.
		}
		if err := config.ValidateDocName(name); err != nil {
			return err
		}
		title := docStringData(sc, "title")
		if title == "" {
			title = DeriveDocTitle(name)
		}
		desc := docStringData(sc, "description")
		if desc == "" {
			desc = defaultLocalDocDesc
		}
		// touches-invariant: local-doc-map-fields — synthesized DocEntry carries title/desc for the document map; proof in local_test.go
		pool[name] = catalog.DocEntry{
			Title:    title,
			Desc:     desc,
			Sections: []string{"content"},
			TID:      baseDocTID,
			Data:     localDocData(name),
		}
	}
	return nil
}
