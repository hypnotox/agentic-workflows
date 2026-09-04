// Package artifactregistry owns awf's canonical managed-artifact declarations.
package artifactregistry

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
)

// Cardinality describes how a managed artifact family is selected.
type Cardinality string

const (
	// CardinalityCatalog selects artifacts from a named catalog.
	CardinalityCatalog Cardinality = "catalog"
	// CardinalityFreeform selects artifacts by caller-provided name.
	CardinalityFreeform Cardinality = "freeform"
	// CardinalitySingleton selects exactly one artifact.
	CardinalitySingleton Cardinality = "singleton"
)

// Targeting describes whether one artifact is neutral or emitted per target.
type Targeting string

const (
	// TargetNeutral identifies an artifact emitted independently of targets.
	TargetNeutral Targeting = "neutral"
	// TargetAdapter identifies an artifact emitted for each selected target.
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
	// OwnerCatalog identifies catalog-owned artifact declarations.
	OwnerCatalog Owner = "catalog"
	// OwnerCore identifies core artifact declarations.
	OwnerCore Owner = "core"
	// OwnerHooks identifies hook artifact declarations.
	OwnerHooks Owner = "hooks"
	// OwnerResident identifies resident-root artifact declarations.
	OwnerResident Owner = "resident"
)

// Target places fixed AWF skills and an optional guide bridge for one harness.
type Target struct {
	Name           string
	SkillDir       string
	BridgeFile     string
	BridgeTemplate string
}

// SkillPath returns the target-native path for a fixed-name managed skill.
func (t Target) SkillPath(name string) string {
	return fmt.Sprintf("%s/%s/SKILL.md", t.SkillDir, name)
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
	{Plural: "docs", Singular: "doc", Cardinality: CardinalityCatalog, Targeting: TargetNeutral, Owner: OwnerCatalog, OwnsParts: true},
	{Plural: "domains", Singular: "domain", Cardinality: CardinalityFreeform, Targeting: TargetNeutral, Owner: OwnerCatalog, TemplatePattern: "domains/domain.md.tmpl", OwnsParts: true},
}

// Kinds returns the canonical artifact families in stable CLI display order.
func Kinds() []Kind { return slices.Clone(kinds) }

// KindByPlural returns the artifact kind with the given plural name.
func KindByPlural(plural string) (Kind, bool) {
	for _, kind := range kinds {
		if kind.Plural == plural {
			return kind, true
		}
	}
	return Kind{}, false
}

// KindBySingular returns the artifact kind with the given singular name.
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
func OutputPath(cat *catalog.Catalog, target Target, _ string, plural, name string) string {
	switch plural {
	case "skills":
		return target.SkillPath(name)
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

// LocalDocOutputPath returns the managed output path for a local document.
func LocalDocOutputPath(name string) string { return config.DocsDir + "/" + name + ".md" }

// PitfallOutputPath returns the managed output path for a pitfall entry.
func PitfallOutputPath(slug string) string { return config.DocsDir + "/pitfalls/" + slug + ".md" }

// TopicOutputPath returns the managed output path for a current-state topic.
func TopicOutputPath(id string) string { return config.DocsDir + "/topics/" + id + ".md" }

// TopicIndexOutputPath returns the managed output path for a domain topic index.
func TopicIndexOutputPath(domain string) string {
	return config.DocsDir + "/topics/" + domain + "/index.md"
}

// ResidentOutputPath returns the managed marker path for a resident root.
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
	case "skills":
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
	Policy        outputplan.Policy
	Participation Participation
	Owner         Owner
}

var runnerSections = []string{"runner-body"}
var hookNames = []string{"pre-commit", "commit-msg", "pre-push", "pre-merge-commit", "reference-transaction"}

// HookTemplateID returns the canonical template identity for a hook.
func HookTemplateID(name string) string { return "hooks/" + name + ".sh.tmpl" }

// ResidentTemplateID returns the canonical template identity for a resident marker.
func ResidentTemplateID(name string) string { return name + "/gitignore.tmpl" }

// Hooks returns the canonical hook names in stable order.
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

// HookArtifacts returns the registry-owned hook declarations in stable order.
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
		{ID: "bootstrap", Kind: "bootstrap", Path: config.DirName + "/bootstrap.sh", TemplateID: "bootstrap/awf-bootstrap.sh.tmpl", Enabled: enabledBootstrap, Participation: Participation{Check: true}, Owner: OwnerCore},
		{ID: "upgrade", Kind: "bootstrap", Path: config.DirName + "/upgrade.sh", TemplateID: "bootstrap/awf-upgrade.sh.tmpl", Enabled: enabledBootstrap, Participation: Participation{Check: true}, Owner: OwnerCore},
		{ID: "runner", Kind: "runner", Path: "awf", TemplateID: "runner/awf.tmpl", Sections: slices.Clone(runnerSections), Enabled: always, Participation: Participation{Check: true}, Owner: OwnerCore},
	}
	for _, name := range hookNames {
		units = append(units, Unit{ID: "hook:" + name, Kind: "hooks", Path: config.DirName + "/hooks/" + name + ".sh", TemplateID: HookTemplateID(name), Enabled: always, Participation: Participation{Hook: true, Check: true}, Owner: OwnerHooks})
	}
	return units
}

var targets = map[string]Target{
	"claude": {Name: "claude", SkillDir: ".claude/skills", BridgeFile: "CLAUDE.md", BridgeTemplate: "claude/CLAUDE.md.tmpl"},
	"pi":     {Name: "pi", SkillDir: ".pi/skills"},
}

// KnownTargets returns the built-in target names in stable order.
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
		out = append(out, targets[name])
	}
	return slices.Clone(out)
}

// BuiltinTarget returns an isolated copy of the named built-in target.
func BuiltinTarget(name string) Target { return targets[name] }

// ResolveTargets validates and returns isolated copies of the named targets.
func ResolveTargets(names []string) ([]Target, error) {
	out := make([]Target, 0, len(names))
	for _, name := range names {
		declaration, ok := targets[name]
		if !ok {
			return nil, fmt.Errorf("unknown target %q (known: %s)", name, strings.Join(KnownTargets(), ", "))
		}
		if err := ValidateTarget(declaration); err != nil {
			return nil, err
		}
		out = append(out, declaration)
	}
	return out, nil
}

// TargetDescriptorProjection returns the stable output-affecting target identity.
func TargetDescriptorProjection(target Target) string {
	projection := fmt.Sprintf("%#v", struct {
		Name, SkillDir, BridgeFile, BridgeTemplate string
	}{target.Name, target.SkillDir, target.BridgeFile, target.BridgeTemplate})
	return strings.ReplaceAll(projection, "artifactregistry.", "projectstate.")
}

// ValidateTarget verifies the fixed target declaration.
func ValidateTarget(target Target) error {
	if target.Name == "" || target.SkillDir == "" {
		return fmt.Errorf("target name and skill directory must be present")
	}
	if (target.BridgeFile == "") != (target.BridgeTemplate == "") {
		return fmt.Errorf("target %q bridge path and template must be both present or absent", target.Name)
	}
	return nil
}
