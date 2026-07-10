// Package catalog is the compile-time Go value declaring the standard's skills, agents, and docs.
package catalog

import "slices"

// TargetSpec declares the render sections of a target that has no further
// per-target configuration (agents and the domain doc). Data carries the
// artifact's default render data; sidecars override it per top-level key
// (ADR-0045).
type TargetSpec struct {
	Sections []string `yaml:"sections"`
	// Base marks a synthesized project-local agent (ADR-0068); see SkillSpec.Base.
	Base bool `yaml:"base"`
	// RequiresSkills names the catalog skills this artifact's template references
	// unconditionally — rendered into its output even when the referenced skill is
	// not enabled (deliberate chain coupling; the agent guide's "disable them as a
	// unit"). Declarations are exact: the template test sweep fails on an
	// undeclared unconditional reference AND on a stale entry (ADR-0080). Data,
	// not gated validation — promoting it to add/remove pairing UX is deferred.
	RequiresSkills []string       `yaml:"requiresSkills"`
	Data           map[string]any `yaml:"data"`
}

// SkillSpec declares a skill's render sections plus its optional gating fields.
// RequiresDoc is *suppression* (ADR-0013): a non-empty value gates the skill on
// that doc being enabled — with the doc off, the skill silently drops out of
// the effective render set. RequiresAgent is *hard validation* (ADR-0050): a
// non-empty value names the reviewer agent the skill dispatches, and enabling
// the skill without that agent fails every gated command at project open — a
// silently-dropped reviewing skill would sever the workflow chain, so the
// pairing must be loud. Core marks a skill as part of the workflow-core set
// awf init scaffolds by default (ADR-0022). Data carries the artifact's
// default render data; sidecars override it per top-level key (ADR-0045).
type SkillSpec struct {
	Sections      []string `yaml:"sections"`
	RequiresDoc   string   `yaml:"requiresDoc"`
	RequiresAgent string   `yaml:"requiresAgent"`
	Core          bool     `yaml:"core"`
	// Chain marks a workflow-chain progression node (ADR-0054). A standard skill
	// that is not Chain is a task skill; Core covers default scaffolding, not
	// chain membership — adr-lifecycle is core yet a task skill.
	Chain bool `yaml:"chain"`
	// Base marks a synthesized project-local entry (ADR-0068): render resolves its
	// template id to the shared base template, not the name-derived catalog path.
	// Standard skills never set it.
	Base bool `yaml:"base"`
	// RequiresSkills: see TargetSpec.RequiresSkills (ADR-0080).
	RequiresSkills []string       `yaml:"requiresSkills"`
	Data           map[string]any `yaml:"data"`
}

// DocEntry is one entry in the unified doc collection (ADR-0061): a toggleable
// doc (Mandatory false) or an always-on singleton (Mandatory true). Path is the
// docsDir-relative output suffix (empty for agents-doc, which renders to root
// AGENTS.md); TemplateKey is its .layout camelCase key (empty when not
// layout-exposed); TID is the embedded template id; DocumentMap marks entries
// the AGENTS.md document map lists via .layout.*; AgentsDoc flags the one
// root-output special case. Title/Desc/Sections/Data are as before.
type DocEntry struct {
	Title       string
	Desc        string
	Sections    []string
	Data        map[string]any
	Mandatory   bool
	Path        string
	TemplateKey string
	TID         string
	DocumentMap bool
	AgentsDoc   bool
	// Generated marks a Mandatory doc rendered outside RenderAll from computed
	// project state (the config reference): excluded from plainSingletons and
	// hash checking, regeneration-checked like ACTIVE.md and the domain docs.
	Generated bool
}

// SingletonKinds returns every always-on singleton kind (the Mandatory doc
// entries, agents-doc included), derived from the one doc collection — no
// hand-maintained list (ADR-0061). internal/config.IsSingletonKind reads it for
// sidecar/part path classification.
func SingletonKinds() []string {
	var out []string
	for k, e := range Standard.Docs {
		if e.Mandatory {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// NonMandatoryDocNames returns c's sorted toggleable-doc names (Mandatory
// false) — the pool an adopter selects from and that `awf add`/`remove doc`
// operate on. Mandatory singletons are excluded (ADR-0061). It takes the catalog
// so the kind-descriptor pool honours the catalog handed to it.
func NonMandatoryDocNames(c *Catalog) []string {
	var out []string
	for k, e := range c.Docs {
		if !e.Mandatory {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// VarDescriptor describes one fillable init value: a config var, or (via Target)
// a non-var routing target (catalog trim, audit scopes). Kind ∈ {string, enum,
// multiselect}. Target ∈ {"" or "var", "catalog-skills", "catalog-docs",
// "audit-scopes"}; "" means a plain config var. Default pre-fills interactive
// prompts and appears in `awf init --describe`; it is never applied on the silent
// non-interactive path (ADR-0029).
type VarDescriptor struct {
	Key         string   `yaml:"key" json:"key"`
	Kind        string   `yaml:"kind" json:"kind"`
	Description string   `yaml:"description" json:"description"`
	Default     string   `yaml:"default" json:"default"`
	Options     []string `yaml:"options" json:"options"`
	Target      string   `yaml:"target" json:"target"`
}

type Catalog struct {
	Skills    map[string]SkillSpec  `yaml:"skills"`
	Agents    map[string]TargetSpec `yaml:"agents"`
	DomainDoc TargetSpec            `yaml:"domainDoc"`
	Docs      map[string]DocEntry   `yaml:"docs"`
	Vars      []VarDescriptor       `yaml:"vars"`
}
