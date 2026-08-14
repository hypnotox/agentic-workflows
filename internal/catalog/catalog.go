// Package catalog is the compile-time Go value declaring the standard's skills, agents, and docs.
package catalog

import (
	"fmt"
	"maps"
	"reflect"
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
	FullOnly bool     `yaml:"fullOnly"`
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
	FullOnly       bool
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
	FullOnly      bool     `yaml:"fullOnly"`
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
	FullOnly    bool
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
func SingletonKinds() []string { return SingletonKindsFor(Standard) }

// SingletonKindsFor returns c's sorted structural singleton kinds.
func SingletonKindsFor(c *Catalog) []string {
	var out []string
	for k, e := range c.Docs {
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

// Profile selects one closed workflow footprint from the complete catalog.
type Profile string

const (
	ProfileCore Profile = "core"
	ProfileFull Profile = "full"
)

func ParseProfile(value string) (Profile, error) {
	profile := Profile(value)
	if profile != ProfileCore && profile != ProfileFull {
		return "", fmt.Errorf("profile must be core or full, got %q", value)
	}
	return profile, nil
}

// View is the immutable selected catalog one composition root gives to a project.
type View struct {
	catalog *Catalog
	profile Profile
}

// CompleteView returns one complete Full catalog snapshot.
func CompleteView() View { return NewProfileView(Standard, ProfileFull) }

// StandardProfileView projects a profile from the compile-time standard catalog.
func StandardProfileView(profile Profile) View { return NewProfileView(Standard, profile) }

// NewView preserves the injected-catalog seam as a Full view.
func NewView(c *Catalog) View { return NewProfileView(c, ProfileFull) }

// NewProfileView projects one closed profile from the complete catalog.
func NewProfileView(c *Catalog, profile Profile) View {
	if c == nil {
		panic("catalog view: missing catalog")
	}
	if _, err := ParseProfile(string(profile)); err != nil {
		panic(err)
	}
	selected := cloneCatalog(c)
	if profile == ProfileCore {
		projectCore(selected)
	}
	return View{catalog: selected, profile: profile}
}

func projectCore(c *Catalog) {
	for name, spec := range c.Skills {
		if spec.FullOnly {
			delete(c.Skills, name)
		}
	}
	for name, spec := range c.Agents {
		if spec.FullOnly {
			delete(c.Agents, name)
		}
	}
	for name, spec := range c.Docs {
		if spec.FullOnly {
			delete(c.Docs, name)
		}
	}
	if c.DomainDoc.FullOnly {
		c.DomainDoc = TargetSpec{}
	}
	if workflow, ok := c.Docs["workflow"]; ok {
		workflow.Desc = "principles, the brainstorm -> implement/test -> review chain, continuity, and commit discipline"
		c.Docs["workflow"] = workflow
	}
	if reviewer, ok := c.Agents["code-reviewer"]; ok {
		reviewer.Data["focusItems"] = []any{
			map[string]any{"name": "approved-boundary-adherence", "description": "the diff matches the approved scope and content; unexplained drift is a finding"},
			map[string]any{"name": "test-coverage", "description": "behaviour changes carry tests in the same commit; no assertion is weakened to pass"},
			map[string]any{"name": "verification-instrument-can-fail", "description": "every added or changed mechanical check has a negative case or temporary falsification proving the mutation landed before its passing verdict counts"},
			map[string]any{"name": "check-purpose", "description": "every material check names the behavior or repository property it proves; flag choreography-only enforcement with no such obligation"},
		}
		reviewer.Data["readStep"] = "Read the diff in full (`git diff baseSha..headSha`) and every requirement or project document referenced by name in the brief."
		c.Agents["code-reviewer"] = reviewer
	}
	for name, spec := range c.Skills {
		spec.Profile.UsuallyFollows = selectedNames(spec.Profile.UsuallyFollows, c.Skills)
		spec.Profile.CommonFollowUps = selectedNames(spec.Profile.CommonFollowUps, c.Skills)
		spec.RequiresSkills = selectedNames(spec.RequiresSkills, c.Skills)
		c.Skills[name] = spec
	}
	for name, spec := range c.Agents {
		spec.RequiresSkills = selectedNames(spec.RequiresSkills, c.Skills)
		c.Agents[name] = spec
	}
	// Governance record-model vocabulary belongs to Full. Operational terms stay.
	if glossary, ok := c.Docs["glossary"]; ok {
		if terms, ok := glossary.Data["standardTerms"].([]any); ok {
			filtered := terms[:0]
			for _, raw := range terms {
				entry, _ := raw.(map[string]any)
				term, _ := entry["term"].(string)
				if slices.Contains([]string{"current-state topic", "claim", "invariant backing"}, term) {
					continue
				}
				filtered = append(filtered, raw)
			}
			glossary.Data["standardTerms"] = filtered
			c.Docs["glossary"] = glossary
		}
	}
}

func selectedNames(names []string, selected map[string]SkillSpec) []string {
	return slices.DeleteFunc(slices.Clone(names), func(name string) bool { _, ok := selected[name]; return !ok })
}

// Catalog returns a defensive snapshot of the view. Callers may retain or
// mutate that snapshot without changing the View or another caller's project.
func (v View) Catalog() *Catalog { return cloneCatalog(v.catalog) }

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
	if value == nil {
		return nil
	}
	return cloneReferenceValue(reflect.ValueOf(value)).Interface()
}

func cloneReferenceValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.New(value.Type()).Elem()
		out.Set(cloneReferenceValue(value.Elem()))
		return out
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			out.SetMapIndex(cloneReferenceValue(iter.Key()), cloneReferenceValue(iter.Value()))
		}
		return out
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			out.Index(i).Set(cloneReferenceValue(value.Index(i)))
		}
		return out
	case reflect.Array:
		out := reflect.New(value.Type()).Elem()
		for i := range value.Len() {
			out.Index(i).Set(cloneReferenceValue(value.Index(i)))
		}
		return out
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(cloneReferenceValue(value.Elem()))
		return out
	default:
		return value
	}
}
