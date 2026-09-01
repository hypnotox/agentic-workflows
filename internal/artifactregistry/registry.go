// Package artifactregistry owns awf's canonical managed-artifact declarations.
package artifactregistry

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

// Cardinality describes how a managed artifact family is selected.
type Cardinality string

const (
	CardinalityCatalog   Cardinality = "catalog"
	CardinalityFreeform  Cardinality = "freeform"
	CardinalitySingleton Cardinality = "singleton"
)

// Targeting describes whether one artifact is neutral or emitted per target.
type Targeting string

const (
	TargetNeutral Targeting = "neutral"
	TargetAdapter Targeting = "target"
)

// Participation declares whether an artifact participates in hook publication
// and in managed-output checks. Output policy supplies the detailed check lanes.
type Participation struct {
	Hook  bool
	Check bool
}

// Owner identifies the subsystem responsible for an artifact declaration.
type Owner string

const (
	OwnerCatalog  Owner = "catalog"
	OwnerTarget   Owner = "target"
	OwnerCore     Owner = "core"
	OwnerHooks    Owner = "hooks"
	OwnerResident Owner = "resident"
)

// AgentDialect names the target-native output representation.
type AgentDialect string

const (
	MarkdownAgentDialect AgentDialect = "markdown"
	PlainAgentDialect    AgentDialect = "plain"
)

// Capability is one closed target capability.
type Capability string

const (
	CapabilitySubagentTools  Capability = "subagent-tools"
	CapabilitySessionHandoff Capability = "session-handoff"
)

// TargetOutputProducer identifies how a target-owned output is produced.
type TargetOutputProducer string

const TargetOutputTemplate TargetOutputProducer = "template"

// TargetOutputInput declares one semantic input to a target-owned output.
type TargetOutputInput struct {
	Path string
	Role outputplan.ArtifactRole
}

// TargetOutput is one target-owned non-catalog managed artifact.
type TargetOutput struct {
	Path           string
	SkillName      string
	RequiresSkill  string
	TemplateID     string
	Producer       TargetOutputProducer
	Inputs         []TargetOutputInput
	Encoder        AgentDialect
	Provenance     render.CommentStyle
	Policy         outputplan.Policy
	PolicyDeclared bool
}

// Target places adapter artifacts for one runtime.
type Target struct {
	Name           string
	SkillDir       string
	AgentDir       string
	AgentSuffix    string
	AgentDialect   AgentDialect
	BridgeFile     string
	BridgeTemplate string
	Capabilities   []Capability
	Outputs        []TargetOutput
}

func (t Target) SkillPath(prefix, name string) string {
	return fmt.Sprintf("%s/%s-%s/SKILL.md", t.SkillDir, prefix, name)
}

func (t Target) AgentPath(name string) string {
	suffix := t.AgentSuffix
	if suffix == "" {
		suffix = ".md"
	}
	return fmt.Sprintf("%s/%s%s", t.AgentDir, name, suffix)
}

// Kind declares one CLI-addressable managed-artifact family.
type Kind struct {
	Plural          string
	Singular        string
	Cardinality     Cardinality
	Targeting       Targeting
	Owner           Owner
	TemplatePattern string
	OwnsParts       bool
}

var kinds = []Kind{
	{Plural: "skills", Singular: "skill", Cardinality: CardinalityCatalog, Targeting: TargetAdapter, Owner: OwnerCatalog, TemplatePattern: "skills/%s/SKILL.md.tmpl", OwnsParts: true},
	{Plural: "agents", Singular: "agent", Cardinality: CardinalityCatalog, Targeting: TargetAdapter, Owner: OwnerCatalog, TemplatePattern: "agents/%s.md.tmpl", OwnsParts: true},
	{Plural: "docs", Singular: "doc", Cardinality: CardinalityCatalog, Targeting: TargetNeutral, Owner: OwnerCatalog, OwnsParts: true},
	{Plural: "domains", Singular: "domain", Cardinality: CardinalityFreeform, Targeting: TargetNeutral, Owner: OwnerCatalog, TemplatePattern: "domains/domain.md.tmpl", OwnsParts: true},
}

// Kinds returns the canonical artifact families in stable CLI display order.
func Kinds() []Kind { return slices.Clone(kinds) }

func KindByPlural(plural string) (Kind, bool) {
	for _, kind := range kinds {
		if kind.Plural == plural {
			return kind, true
		}
	}
	return Kind{}, false
}

func KindBySingular(singular string) (Kind, bool) {
	for _, kind := range kinds {
		if kind.Singular == singular {
			return kind, true
		}
	}
	return Kind{}, false
}

// Names returns a kind's sorted catalog inventory. Freeform kinds have no pool.
func Names(cat *catalog.Catalog, plural string) ([]string, bool) {
	switch plural {
	case "skills":
		return slices.Sorted(maps.Keys(cat.Skills)), true
	case "agents":
		return slices.Sorted(maps.Keys(cat.Agents)), true
	case "docs":
		return slices.Sorted(maps.Keys(cat.Docs)), true
	default:
		return nil, false
	}
}

// CatalogNames is the named-catalog inventory projection.
func CatalogNames(cat *catalog.Catalog, plural string) ([]string, bool) { return Names(cat, plural) }

// Sections returns one artifact's declared sections and catalog presence.
func Sections(cat *catalog.Catalog, plural, name string) ([]string, bool) {
	switch plural {
	case "skills":
		spec, ok := cat.Skills[name]
		return slices.Clone(spec.Sections), ok
	case "agents":
		spec, ok := cat.Agents[name]
		return slices.Clone(spec.Sections), ok
	case "docs":
		spec, ok := cat.Docs[name]
		return slices.Clone(spec.Sections), ok
	case "domains":
		return slices.Clone(cat.DomainDoc.Sections), false
	default:
		return nil, false
	}
}

// TemplateID returns the canonical template source for one artifact.
func TemplateID(cat *catalog.Catalog, plural, name string) string {
	kind, ok := KindByPlural(plural)
	if !ok {
		return ""
	}
	if plural == "docs" {
		return cat.Docs[name].TID
	}
	if strings.Contains(kind.TemplatePattern, "%s") {
		return fmt.Sprintf(kind.TemplatePattern, name)
	}
	return kind.TemplatePattern
}

// OutputPath returns one catalog artifact's canonical managed output path.
func OutputPath(cat *catalog.Catalog, target Target, prefix, plural, name string) string {
	switch plural {
	case "skills":
		return target.SkillPath(prefix, name)
	case "agents":
		return target.AgentPath(name)
	case "docs":
		entry := cat.Docs[name]
		if entry.AgentsDoc {
			return "AGENTS.md"
		}
		out := entry.Path
		if out == "" {
			out = name + ".md"
		}
		return config.DocsDir + "/" + out
	case "domains":
		return config.DocsDir + "/domains/" + name + ".md"
	default:
		return ""
	}
}

func LocalDocOutputPath(name string) string { return config.DocsDir + "/" + name + ".md" }
func PitfallOutputPath(slug string) string  { return config.DocsDir + "/pitfalls/" + slug + ".md" }
func TopicOutputPath(id string) string      { return config.DocsDir + "/topics/" + id + ".md" }
func TopicIndexOutputPath(domain string) string {
	return config.DocsDir + "/topics/" + domain + "/index.md"
}
func ResidentOutputPath(name string) string { return config.DirName + "/" + name + "/.gitignore" }

// ResidentArtifact is one registry-owned resident-root marker declaration.
type ResidentArtifact struct {
	Name          string
	TemplateID    string
	OutputPath    string
	Participation Participation
	Owner         Owner
}

// Resident returns the canonical marker declaration for one resident root.
func Resident(name string) ResidentArtifact {
	return ResidentArtifact{
		Name: name, TemplateID: ResidentTemplateID(name), OutputPath: ResidentOutputPath(name),
		Participation: Participation{Check: true}, Owner: OwnerResident,
	}
}

// Policy returns the canonical detailed check participation for a producer family.
func Policy(kind string, regenerate bool) outputplan.Policy {
	policy := outputplan.Policy{Regenerate: regenerate}
	switch kind {
	case "skills", "agents":
		policy.ValidateFrontmatter, policy.ScanReferences, policy.ScanSkillReferences = true, true, true
	case "docs", "local-doc", "agents-doc", "doc-standard", "agents-md-standard", "working-with-awf", "pi-runtime-reference", "workflow", "architecture", "development", "glossary", "pitfalls", "roadmap", "testing", "releasing", "domains", "topics":
		policy.ScanReferences, policy.ScanSkillReferences = true, true
	}
	return policy
}

// Singleton is a catalog-derived structural singleton artifact.
type Singleton struct {
	Kind          string
	TemplateID    string
	OutputPath    string
	Sections      []string
	Generated     bool
	Participation Participation
	Owner         Owner
}

// Singletons returns structural catalog artifacts in stable kind order.
func Singletons(cat *catalog.Catalog) []Singleton {
	var out []Singleton
	for _, name := range slices.Sorted(maps.Keys(cat.Docs)) {
		entry := cat.Docs[name]
		if !entry.AgentsDoc && entry.Path == "" {
			continue
		}
		path := config.DocsDir + "/" + entry.Path
		if entry.AgentsDoc {
			path = "AGENTS.md"
		}
		out = append(out, Singleton{Kind: name, TemplateID: entry.TID, OutputPath: path, Sections: slices.Clone(entry.Sections), Generated: entry.Generated, Participation: Participation{Check: true}, Owner: OwnerCatalog})
	}
	return out
}

// PlainSingletons excludes the root guide and computed generated outputs.
func PlainSingletons(cat *catalog.Catalog) []Singleton {
	return slices.Collect(func(yield func(Singleton) bool) {
		for _, singleton := range Singletons(cat) {
			if singleton.OutputPath != "AGENTS.md" && !singleton.Generated && !yield(singleton) {
				return
			}
		}
	})
}

// Canonical template identities for registry-owned dynamic artifact families.
const (
	TargetBridgeKind       = "target-bridge"
	ClaudeBridgeTemplateID = "claude/CLAUDE.md.tmpl"
	TopicTemplateID        = "topics/topic.md.tmpl"
	TopicIndexTemplateID   = "topics/index.md.tmpl"
	PitfallEntryTemplateID = "pitfalls/entry.md.tmpl"
	LocalDocTemplateID     = "docs/local.md.tmpl"
)

// Unit declares one core singleton or hook artifact.
type Unit struct {
	ID            string
	Kind          string
	Path          string
	TemplateID    string
	Sections      []string
	Enabled       func(*config.Config) bool
	Encoder       AgentDialect
	Provenance    render.CommentStyle
	Policy        outputplan.Policy
	Participation Participation
	Owner         Owner
}

var runnerSections = []string{"runner-body"}
var hookNames = []string{"pre-commit", "commit-msg", "pre-push", "pre-merge-commit", "reference-transaction"}

func HookTemplateID(name string) string     { return "hooks/" + name + ".sh.tmpl" }
func ResidentTemplateID(name string) string { return name + "/gitignore.tmpl" }
func Hooks() []string {
	artifacts := HookArtifacts()
	out := make([]string, len(artifacts))
	for i, artifact := range artifacts {
		out[i] = artifact.Name
	}
	return out
}

// Hook is the stable hook-specific projection of a managed unit.
type Hook struct {
	Name, TemplateID, OutputPath, Owner string
	Checked                             bool
}

func HookArtifacts() []Hook {
	out := make([]Hook, 0, len(hookNames))
	for _, unit := range ConditionalUnits() {
		if !unit.Participation.Hook {
			continue
		}
		name := strings.TrimPrefix(unit.ID, "hook:")
		out = append(out, Hook{Name: name, TemplateID: unit.TemplateID, OutputPath: unit.Path, Owner: string(unit.Owner), Checked: unit.Participation.Check})
	}
	return out
}

// ConditionalUnits returns stable core singleton and hook declarations.
func ConditionalUnits() []Unit {
	enabledBootstrap := func(c *config.Config) bool { return c.Bootstrap != nil && c.Bootstrap.Enabled }
	always := func(*config.Config) bool { return true }
	units := []Unit{
		{ID: "bootstrap", Kind: "bootstrap", Path: config.DirName + "/bootstrap.sh", TemplateID: "bootstrap/awf-bootstrap.sh.tmpl", Enabled: enabledBootstrap, Encoder: PlainAgentDialect, Provenance: render.HTMLComment, Participation: Participation{Check: true}, Owner: OwnerCore},
		{ID: "upgrade", Kind: "bootstrap", Path: config.DirName + "/upgrade.sh", TemplateID: "bootstrap/awf-upgrade.sh.tmpl", Enabled: enabledBootstrap, Encoder: PlainAgentDialect, Provenance: render.HTMLComment, Participation: Participation{Check: true}, Owner: OwnerCore},
		{ID: "runner", Kind: "runner", Path: "awf", TemplateID: "runner/awf.tmpl", Sections: slices.Clone(runnerSections), Enabled: always, Encoder: PlainAgentDialect, Provenance: render.HTMLComment, Participation: Participation{Check: true}, Owner: OwnerCore},
	}
	for _, name := range hookNames {
		units = append(units, Unit{ID: "hook:" + name, Kind: "hooks", Path: config.DirName + "/hooks/" + name + ".sh", TemplateID: HookTemplateID(name), Enabled: always, Encoder: PlainAgentDialect, Provenance: render.HTMLComment, Participation: Participation{Hook: true, Check: true}, Owner: OwnerHooks})
	}
	return units
}

// TargetArtifact attaches registry governance metadata to one target output.
type TargetArtifact struct {
	Output        TargetOutput
	Participation Participation
	Owner         Owner
}

type targetDeclaration struct {
	Target  Target
	Outputs []TargetArtifact
}

var targets = map[string]targetDeclaration{
	"claude": {Target: Target{Name: "claude", SkillDir: ".claude/skills", AgentDir: ".claude/agents", AgentSuffix: ".md", AgentDialect: MarkdownAgentDialect, BridgeFile: "CLAUDE.md", BridgeTemplate: "claude/CLAUDE.md.tmpl"}},
	"pi": {Target: Target{Name: "pi", SkillDir: ".pi/skills", AgentDir: ".pi/agents", AgentSuffix: ".md", AgentDialect: MarkdownAgentDialect, Capabilities: []Capability{CapabilitySubagentTools, CapabilitySessionHandoff}}, Outputs: []TargetArtifact{
		{Output: TargetOutput{Path: ".pi/extensions/awf-subagents/index.ts", TemplateID: "pi/awf-subagents/index.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: outputplan.Policy{}, PolicyDeclared: true}, Participation: Participation{Check: true}, Owner: OwnerTarget},
		{Output: TargetOutput{Path: ".pi/extensions/awf-subagents/model-routing.ts", TemplateID: "pi/awf-subagents/model-routing.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: outputplan.Policy{}, PolicyDeclared: true}, Participation: Participation{Check: true}, Owner: OwnerTarget},
	}},
}

func declaredTarget(declaration targetDeclaration) Target {
	target := declaration.Target
	if len(declaration.Outputs) != 0 {
		target.Outputs = make([]TargetOutput, len(declaration.Outputs))
		for i, artifact := range declaration.Outputs {
			target.Outputs[i] = artifact.Output
		}
	}
	return target
}

func KnownTargets() []string {
	declarations := Targets()
	out := make([]string, len(declarations))
	for i, declaration := range declarations {
		out[i] = declaration.Name
	}
	return out
}

// Targets returns all built-in targets in stable name order.
func Targets() []Target {
	names := slices.Sorted(maps.Keys(targets))
	out := make([]Target, 0, len(names))
	for _, name := range names {
		out = append(out, declaredTarget(targets[name]))
	}
	return cloneTargets(out)
}

func BuiltinTarget(name string) Target {
	return cloneTargets([]Target{declaredTarget(targets[name])})[0]
}

func ResolveTargets(names []string) ([]Target, error) {
	out := make([]Target, 0, len(names))
	for _, name := range names {
		declaration, ok := targets[name]
		if !ok {
			return nil, fmt.Errorf("unknown target %q (known: %s)", name, strings.Join(KnownTargets(), ", "))
		}
		target := declaredTarget(declaration)
		if err := ValidateTarget(target); err != nil {
			return nil, err
		}
		out = append(out, cloneTargets([]Target{target})[0])
	}
	return out, nil
}

// TargetTemplateData projects the closed capability set into template inputs.
func TargetTemplateData(target Target) map[string]any {
	return map[string]any{
		"targetSubagentTools":  slices.Contains(target.Capabilities, CapabilitySubagentTools),
		"targetSessionHandoff": slices.Contains(target.Capabilities, CapabilitySessionHandoff),
	}
}

// AnyTargetHasCapability reports whether any selected target declares capability.
func AnyTargetHasCapability(targets []Target, capability Capability) bool {
	return slices.ContainsFunc(targets, func(target Target) bool {
		return slices.Contains(target.Capabilities, capability)
	})
}

// TargetDescriptorProjection returns the stable output-affecting target identity.
func TargetDescriptorProjection(target Target) string {
	capabilities := slices.Clone(target.Capabilities)
	slices.Sort(capabilities)
	outputs := slices.Clone(target.Outputs)
	slices.SortFunc(outputs, func(left, right TargetOutput) int {
		return strings.Compare(fmt.Sprintf("%#v", left), fmt.Sprintf("%#v", right))
	})
	projection := fmt.Sprintf("%#v", struct {
		Name, SkillDir, AgentDir, AgentSuffix, BridgeFile, BridgeTemplate string
		AgentDialect                                                      AgentDialect
		Capabilities                                                      []Capability
		Outputs                                                           []TargetOutput
	}{target.Name, target.SkillDir, target.AgentDir, target.AgentSuffix, target.BridgeFile, target.BridgeTemplate, target.AgentDialect, capabilities, outputs})
	// Preserve the historical hash projection after moving declaration ownership.
	// The legacy package qualifier is serialized hash input, not a live type owner.
	return strings.ReplaceAll(projection, "artifactregistry.", "projectstate.")
}

// ValidateTargetRequirements rejects outputs that name an absent catalog skill.
func ValidateTargetRequirements(target Target, cat *catalog.Catalog) error {
	for _, output := range target.Outputs {
		if output.RequiresSkill != "" {
			if _, ok := cat.Skills[output.RequiresSkill]; !ok {
				return fmt.Errorf("target %q output %q requires unknown catalog skill %q", target.Name, output.Path, output.RequiresSkill)
			}
		}
	}
	return nil
}

func targetArtifacts(target Target) []TargetArtifact {
	metadata := targets[target.Name].Outputs
	out := make([]TargetArtifact, len(target.Outputs))
	for i, output := range target.Outputs {
		out[i] = TargetArtifact{Output: output, Participation: Participation{Check: true}, Owner: OwnerTarget}
		if i < len(metadata) && metadata[i].Output.TemplateID == output.TemplateID {
			out[i].Participation = metadata[i].Participation
			out[i].Owner = metadata[i].Owner
		}
	}
	return out
}

// ResolveTargetArtifacts applies required-skill selection and skill-path translation
// while retaining the registry's ownership and check-participation metadata.
func ResolveTargetArtifacts(target Target, prefix string, selected []string) []TargetArtifact {
	enabled := make(map[string]bool, len(selected))
	for _, name := range selected {
		enabled[name] = true
	}
	out := []TargetArtifact{}
	for _, artifact := range targetArtifacts(target) {
		if artifact.Output.RequiresSkill != "" && !enabled[artifact.Output.RequiresSkill] {
			continue
		}
		if !artifact.Participation.Check {
			continue
		}
		if artifact.Output.SkillName != "" {
			artifact.Output.Path = target.SkillPath(prefix, artifact.Output.SkillName)
		}
		out = append(out, artifact)
	}
	return out
}

func ValidateTarget(target Target) error {
	known := map[Capability]bool{CapabilitySubagentTools: true, CapabilitySessionHandoff: true}
	for _, capability := range target.Capabilities {
		if !known[capability] {
			return fmt.Errorf("target %q has unknown capability %q", target.Name, capability)
		}
	}
	if (target.BridgeFile == "") != (target.BridgeTemplate == "") {
		return fmt.Errorf("target %q bridge path and template must be both present or absent", target.Name)
	}
	if target.AgentDialect != MarkdownAgentDialect {
		return fmt.Errorf("target %q has unknown agent encoder %q", target.Name, target.AgentDialect)
	}
	for _, artifact := range targetArtifacts(target) {
		if artifact.Owner != OwnerTarget {
			return fmt.Errorf("target %q output %q has invalid owner %q", target.Name, artifact.Output.Path, artifact.Owner)
		}
	}
	for _, out := range target.Outputs {
		if (out.Path == "") == (out.SkillName == "") || out.TemplateID == "" || (out.Path != "" && !filepath.IsLocal(filepath.FromSlash(out.Path))) {
			return fmt.Errorf("target %q has unsafe output %q", target.Name, out.Path)
		}
		if out.SkillName != "" {
			if err := config.ValidateArtifactName("skill", out.SkillName); err != nil {
				return fmt.Errorf("target %q has unsafe skill output %q: %w", target.Name, out.SkillName, err)
			}
		}
		if out.Producer != TargetOutputTemplate {
			return fmt.Errorf("target %q output %q has unknown producer %q", target.Name, out.Path, out.Producer)
		}
		if len(out.Inputs) != 0 {
			return fmt.Errorf("target %q template output %q declares producer inputs", target.Name, out.Path)
		}
		if out.Encoder != MarkdownAgentDialect && out.Encoder != PlainAgentDialect {
			return fmt.Errorf("target %q output %q has unknown encoder %q", target.Name, out.Path, out.Encoder)
		}
		if !out.PolicyDeclared {
			return fmt.Errorf("target %q output %q has no declared policy", target.Name, out.Path)
		}
		validProvenance := (out.Encoder == MarkdownAgentDialect && out.Provenance == render.HTMLComment) || (out.Encoder == PlainAgentDialect && out.Provenance == render.SlashComment)
		if !validProvenance {
			return fmt.Errorf("target %q output %q: encoder %q is incompatible with provenance", target.Name, out.Path, out.Encoder)
		}
		if out.Encoder == PlainAgentDialect && (out.Policy.ValidateFrontmatter || out.Policy.ScanReferences || out.Policy.ScanSkillReferences) {
			return fmt.Errorf("target %q output %q: %w", target.Name, out.Path, errors.New("plain encoder is incompatible with frontmatter or Markdown reference policy"))
		}
	}
	return nil
}

func cloneTargets(source []Target) []Target {
	out := slices.Clone(source)
	for i := range out {
		out[i].Capabilities = slices.Clone(out[i].Capabilities)
		out[i].Outputs = slices.Clone(out[i].Outputs)
		for j := range out[i].Outputs {
			out[i].Outputs[j].Inputs = slices.Clone(out[i].Outputs[j].Inputs)
		}
	}
	return out
}
