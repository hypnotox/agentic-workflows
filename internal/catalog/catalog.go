// Package catalog is the compile-time Go value declaring the standard's skills, agents, and docs.
package catalog

// TargetSpec declares the render sections of a target that has no further
// per-target configuration (agents and the always-on singletons). Data carries
// the artifact's default render data; sidecars override it per top-level key
// (ADR-0045).
type TargetSpec struct {
	Sections []string       `yaml:"sections"`
	Data     map[string]any `yaml:"data"`
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
	Sections      []string       `yaml:"sections"`
	RequiresDoc   string         `yaml:"requiresDoc"`
	RequiresAgent string         `yaml:"requiresAgent"`
	Core          bool           `yaml:"core"`
	Data          map[string]any `yaml:"data"`
}

// DocSpec declares a doc's catalog metadata. Docs no longer carry a Core marker
// (ADR-0043): the three docs that used to set it are always-on singletons now,
// outside this map entirely.
type DocSpec struct {
	Title    string   `yaml:"title"`
	Desc     string   `yaml:"desc"`
	Sections []string `yaml:"sections"`
}

// SingletonKinds lists every kind name that is an always-on singleton — never
// toggled via an enable array (ADR-0004, ADR-0021, ADR-0043). It is a plain
// compile-time list, not derived from a loaded Catalog, because
// internal/config.IsSingletonKind needs this classification without holding a
// *Catalog instance. internal/project tests its members against both this list
// and Catalog.Singletons' loaded keys (an exact match, agents-doc included), so
// the compile-time list and the YAML-driven map never drift apart silently.
var SingletonKinds = []string{
	"agents-doc", "adr-readme", "adr-template", "plans-readme",
	"workflow", "doc-standard", "agents-md-standard", "working-with-awf",
}

// VarDescriptor describes one fillable init value: a config var, or (via Target)
// the invariants backing config. Kind ∈ {string, enum, multiselect}; multiselect
// is reserved for the deferred catalog-trim work (ADR-0029). Target ∈ {"" or
// "var", "invariants-marker", "invariants-globs"}; "" means a plain config var.
// Default pre-fills interactive prompts and appears in `awf init --describe`; it
// is never applied on the silent non-interactive path (ADR-0029).
type VarDescriptor struct {
	Key         string   `yaml:"key" json:"key"`
	Kind        string   `yaml:"kind" json:"kind"`
	Description string   `yaml:"description" json:"description"`
	Default     string   `yaml:"default" json:"default"`
	Options     []string `yaml:"options" json:"options"`
	Target      string   `yaml:"target" json:"target"`
}

type Catalog struct {
	Skills     map[string]SkillSpec  `yaml:"skills"`
	Agents     map[string]TargetSpec `yaml:"agents"`
	DomainDoc  TargetSpec            `yaml:"domainDoc"`
	Singletons map[string]TargetSpec `yaml:"singletons"`
	Docs       map[string]DocSpec    `yaml:"docs"`
	Vars       []VarDescriptor       `yaml:"vars"`
}
