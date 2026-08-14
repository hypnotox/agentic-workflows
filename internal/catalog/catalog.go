// Package catalog is the compile-time Go value declaring the standard's skills, agents, and docs.
package catalog

import (
	"maps"
	"slices"
)

// WorkflowKind classifies a governed workflow body.
type WorkflowKind string

const (
	WorkflowChain   WorkflowKind = "chain"
	WorkflowTask    WorkflowKind = "task"
	WorkflowSupport WorkflowKind = "support"
)

// WorkflowProfile describes how an enabled skill can be selected. Its
// relationships are advisory metadata and never enablement edges.
type WorkflowProfile struct {
	Kind            WorkflowKind
	Purpose         string
	Trigger         string
	UsuallyFollows  []string
	CommonFollowUps []string
}

// TargetSpec declares the render sections of a target that has no further
// per-target configuration (the domain doc). Data carries the artifact's
// default render data; sidecars override it per top-level key (ADR-0045).
type TargetSpec struct {
	Sections []string `yaml:"sections"`
	// RequiresSkills names the catalog skills this artifact's template references
	// unconditionally - rendered into its output even when the referenced skill is
	// not enabled (deliberate chain coupling; the agent guide's "disable them as a
	// unit"). Declarations are exact: the template test sweep fails on an
	// undeclared unconditional reference AND on a stale entry (ADR-0080). Data,
	// not gated validation - promoting it to enable/disable pairing UX is deferred.
	RequiresSkills []string       `yaml:"requiresSkills"`
	Data           map[string]any `yaml:"data"`
}

// AgentSpec declares an output-format-neutral agent. Name is literal while
// Description is a normally rendered template fragment; the instruction body
// comes from the section-rendered agent template.
type AgentSpec struct {
	Name           string
	Description    string
	Sections       []string       `yaml:"sections"`
	RequiresSkills []string       `yaml:"requiresSkills"`
	Data           map[string]any `yaml:"data"`
}

// SkillSpec declares a skill's render sections and relationship metadata.
// RequiresDoc and RequiresAgent preserve catalog declarations used by frozen
// migrations and workflow-reference checks; neither selects the render set.
// Data carries the artifact's default render data; sidecars override it per
// top-level key (ADR-0045).
type SkillSpec struct {
	Sections      []string `yaml:"sections"`
	RequiresDoc   string   `yaml:"requiresDoc"`
	RequiresAgent string   `yaml:"requiresAgent"`
	// RequiresSkills: see TargetSpec.RequiresSkills (ADR-0080).
	RequiresSkills []string        `yaml:"requiresSkills"`
	Data           map[string]any  `yaml:"data"`
	Profile        WorkflowProfile `yaml:"profile"`
}

// DocEntry is one entry in the unified doc collection. Every entry renders;
// Path distinguishes structural singleton outputs from name-derived docs
// (empty for agents-doc, which renders to root AGENTS.md). Mandatory remains
// the sidecar-location discriminator. TemplateKey is its .layout camelCase key (empty when not
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
	// Generated marks a Mandatory doc rendered outside the ordinary render pass from computed
	// project state (the config reference): excluded from plainSingletons and
	// hash checking, regeneration-checked like INDEX.md and topic navigation.
	Generated bool
}

// SingletonKinds returns every structural singleton kind: the root agent guide
// and entries that declare their own output Path. It is derived from the one doc
// collection; internal/config.IsSingletonKind reads it for sidecar and part
// path classification.
func SingletonKinds() []string {
	var out []string
	for k, e := range Standard.Docs {
		if e.AgentsDoc || e.Path != "" {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// NameDerivedDocNames returns c's sorted non-singleton document names.
func NameDerivedDocNames(c *Catalog) []string {
	var out []string
	for k, e := range c.Docs {
		if !e.AgentsDoc && e.Path == "" {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// VarDescriptor describes one fillable init value: a config var, or (via Target)
// a non-var routing target for audit scopes. Kind is string or enum. Target is
// "", "var", or "audit-scopes"; "" means a plain config var. Default pre-fills interactive
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
	Skills    map[string]SkillSpec `yaml:"skills"`
	Agents    map[string]AgentSpec `yaml:"agents"`
	DomainDoc TargetSpec           `yaml:"domainDoc"`
	Docs      map[string]DocEntry  `yaml:"docs"`
	Vars      []VarDescriptor      `yaml:"vars"`
}

// View is the immutable-in-practice catalog selection a composition root gives
// to one project. It owns a defensive snapshot, so neither the global Standard
// value nor an injected fixture can change through a project's catalog seam.
type View struct{ catalog *Catalog }

// CompleteView returns one complete, Full-equivalent catalog snapshot.
func CompleteView() View { return NewView(Standard) }

// NewView snapshots an explicitly composed complete catalog for one project.
func NewView(c *Catalog) View {
	if c == nil {
		panic("catalog view: missing catalog")
	}
	return View{catalog: cloneCatalog(c)}
}

// Catalog returns the view-owned catalog. Callers treat the snapshot as
// read-only; its only mutable alias is confined inside the owning project.
func (v View) Catalog() *Catalog { return v.catalog }

func cloneCatalog(src *Catalog) *Catalog {
	out := &Catalog{
		Skills:    maps.Clone(src.Skills),
		Agents:    maps.Clone(src.Agents),
		DomainDoc: src.DomainDoc,
		Docs:      maps.Clone(src.Docs),
		Vars:      slices.Clone(src.Vars),
	}
	out.DomainDoc.Sections = slices.Clone(src.DomainDoc.Sections)
	out.DomainDoc.RequiresSkills = slices.Clone(src.DomainDoc.RequiresSkills)
	out.DomainDoc.Data = cloneData(src.DomainDoc.Data)
	for name, spec := range out.Skills {
		spec.Sections = slices.Clone(spec.Sections)
		spec.RequiresSkills = slices.Clone(spec.RequiresSkills)
		spec.Data = cloneData(spec.Data)
		spec.Profile.UsuallyFollows = slices.Clone(spec.Profile.UsuallyFollows)
		spec.Profile.CommonFollowUps = slices.Clone(spec.Profile.CommonFollowUps)
		out.Skills[name] = spec
	}
	for name, spec := range out.Agents {
		spec.Sections = slices.Clone(spec.Sections)
		spec.RequiresSkills = slices.Clone(spec.RequiresSkills)
		spec.Data = cloneData(spec.Data)
		out.Agents[name] = spec
	}
	for name, entry := range out.Docs {
		entry.Sections = slices.Clone(entry.Sections)
		entry.Data = cloneData(entry.Data)
		out.Docs[name] = entry
	}
	for i := range out.Vars {
		out.Vars[i].Options = slices.Clone(out.Vars[i].Options)
	}
	return out
}

func cloneData(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = cloneDataValue(value)
	}
	return out
}

func cloneDataValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneData(value)
	case []any:
		out := make([]any, len(value))
		for i := range value {
			out[i] = cloneDataValue(value[i])
		}
		return out
	case []string:
		return slices.Clone(value)
	default:
		return value
	}
}
