// Package projectstate owns immutable loaded project facts and resolved target declarations.
package projectstate

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

// AgentDialect names the target-native encoding for rendered agents.
type AgentDialect string

const (
	// MarkdownAgentDialect selects Markdown agent encoding.
	MarkdownAgentDialect AgentDialect = "markdown"
	// PlainAgentDialect selects plain-text agent encoding.
	PlainAgentDialect AgentDialect = "plain"
)

// Capability is an awf-owned template capability. It is deliberately closed:
// targets cannot inject arbitrary template data.
type Capability string

const (
	// CapabilitySubagentTools enables target-native subagent tools.
	CapabilitySubagentTools Capability = "subagent-tools"
	// CapabilitySessionHandoff enables target-native session handoff.
	CapabilitySessionHandoff Capability = "session-handoff"
	// CapabilityEffortSessions enables target-native effort sessions.
	CapabilityEffortSessions Capability = "effort-sessions"
)

// TargetOutputProducer identifies how a target-owned output is produced.
type TargetOutputProducer string

const (
	// TargetOutputTemplate declares a template-produced target output.
	TargetOutputTemplate TargetOutputProducer = "template"
)

// TargetOutputInput declares one semantic input to a target-owned output.
type TargetOutputInput struct {
	Path string
	Role outputplan.ArtifactRole
}

// TargetOutput declares a target-owned non-catalog output such as a project extension.
type TargetOutput struct {
	Path string
	// SkillName derives a target-native skill path; exactly one of it and Path is set.
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

func (t Target) validate() error {
	known := map[Capability]bool{CapabilitySubagentTools: true, CapabilitySessionHandoff: true, CapabilityEffortSessions: true}
	for _, c := range t.Capabilities {
		if !known[c] {
			return fmt.Errorf("target %q has unknown capability %q", t.Name, c)
		}
	}
	if (t.BridgeFile == "") != (t.BridgeTemplate == "") {
		return fmt.Errorf("target %q bridge path and template must be both present or absent", t.Name)
	}
	if t.AgentDialect != MarkdownAgentDialect {
		return fmt.Errorf("target %q has unknown agent encoder %q", t.Name, t.AgentDialect)
	}
	for _, out := range t.Outputs {
		if (out.Path == "") == (out.SkillName == "") || out.TemplateID == "" || (out.Path != "" && !filepath.IsLocal(filepath.FromSlash(out.Path))) {
			return fmt.Errorf("target %q has unsafe output %q", t.Name, out.Path)
		}
		if out.SkillName != "" {
			if err := config.ValidateArtifactName("skill", out.SkillName); err != nil {
				return fmt.Errorf("target %q has unsafe skill output %q: %w", t.Name, out.SkillName, err)
			}
		}
		if out.Producer != TargetOutputTemplate {
			return fmt.Errorf("target %q output %q has unknown producer %q", t.Name, out.Path, out.Producer)
		}
		if len(out.Inputs) != 0 {
			return fmt.Errorf("target %q template output %q declares producer inputs", t.Name, out.Path)
		}
		if out.Encoder != MarkdownAgentDialect && out.Encoder != PlainAgentDialect {
			return fmt.Errorf("target %q output %q has unknown encoder %q", t.Name, out.Path, out.Encoder)
		}
		if !out.PolicyDeclared {
			return fmt.Errorf("target %q output %q has no declared policy", t.Name, out.Path)
		}
		if err := validateOutputCompatibility(out); err != nil {
			return fmt.Errorf("target %q output %q: %w", t.Name, out.Path, err)
		}
	}
	return nil
}

// validateOutputCompatibility rejects descriptor combinations that cannot
// describe one coherent encoded output. Policy is deliberately independent of
// the path and template spelling, but not of an encoder that cannot support it.
func validateOutputCompatibility(out TargetOutput) error {
	validProvenance := (out.Encoder == MarkdownAgentDialect && out.Provenance == render.HTMLComment) ||
		(out.Encoder == PlainAgentDialect && out.Provenance == render.SlashComment)
	if !validProvenance {
		return fmt.Errorf("encoder %q is incompatible with provenance", out.Encoder)
	}
	if out.Encoder == PlainAgentDialect && (out.Policy.ValidateFrontmatter || out.Policy.ScanReferences || out.Policy.ScanSkillReferences) {
		return errors.New("plain encoder is incompatible with frontmatter or Markdown reference policy")
	}
	return nil
}

// Target places adapter (tool-specific) artifacts for one runtime. Neutral
// artifacts (AGENTS.md, docs, domains) are not target-scoped (ADR-0016).
type Target struct {
	Name           string
	SkillDir       string // dir holding rendered skills, e.g. ".claude/skills"
	AgentDir       string // dir holding rendered agents, e.g. ".claude/agents"
	AgentSuffix    string // agent filename suffix, including its extension
	AgentDialect   AgentDialect
	BridgeFile     string // adapter bridge file at repo root, "" if none
	BridgeTemplate string
	// Capabilities is the closed capability declaration exposed through the
	// fixed targetTemplateData projection.
	Capabilities []Capability
	Outputs      []TargetOutput
}

// SkillPath is the output path for a rendered skill under this target.
func (t Target) SkillPath(prefix, name string) string {
	return fmt.Sprintf("%s/%s-%s/SKILL.md", t.SkillDir, prefix, name)
}

// AgentPath is the output path for a rendered agent under this target.
func (t Target) AgentPath(name string) string {
	suffix := t.AgentSuffix
	if suffix == "" {
		suffix = ".md"
	}
	return fmt.Sprintf("%s/%s%s", t.AgentDir, name, suffix)
}

// The built-in adapters are declared below and wired into targetRegistry. Adding
// a runtime is a new Target value plus a registry entry, not a render-loop change
// (ADR-0037, ADR-0122).
var claudeTarget = Target{
	Name:           "claude",
	SkillDir:       ".claude/skills",
	AgentDir:       ".claude/agents",
	AgentSuffix:    ".md",
	AgentDialect:   MarkdownAgentDialect,
	BridgeFile:     "CLAUDE.md",
	BridgeTemplate: "claude/CLAUDE.md.tmpl",
}

var piTarget = Target{
	Name:         "pi",
	SkillDir:     ".pi/skills",
	AgentDir:     ".pi/agents",
	AgentSuffix:  ".md",
	AgentDialect: MarkdownAgentDialect,
	Capabilities: []Capability{CapabilitySubagentTools, CapabilitySessionHandoff, CapabilityEffortSessions},
	Outputs: []TargetOutput{
		{Path: ".pi/extensions/awf-subagents/index.ts", TemplateID: "pi/awf-subagents/index.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: outputplan.Policy{}, PolicyDeclared: true},
		{Path: ".pi/extensions/awf-subagents/model-routing.ts", TemplateID: "pi/awf-subagents/model-routing.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: outputplan.Policy{}, PolicyDeclared: true},
		{Path: ".pi/extensions/awf-effort/index.ts", RequiresSkill: "effort-workflow", TemplateID: "pi/awf-effort/index.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: outputplan.Policy{}, PolicyDeclared: true},
		{Path: ".pi/extensions/awf-effort/client.ts", RequiresSkill: "effort-workflow", TemplateID: "pi/awf-effort/client.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: outputplan.Policy{}, PolicyDeclared: true},
		{SkillName: "using-effort", RequiresSkill: "effort-workflow", TemplateID: "skills/using-effort/SKILL.md.tmpl", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, Policy: outputplan.Policy{ValidateFrontmatter: true, ScanReferences: true, ScanSkillReferences: true}, PolicyDeclared: true},
	},
}

// targetRegistry maps an adapter name to its Target. It is the sole enumeration
// of known adapters; resolveTargets rejects any name absent from it.
var targetRegistry = map[string]Target{
	"claude": claudeTarget,
	"pi":     piTarget,
}

// KnownTargets returns the known adapter names in sorted order. The bespoke
// `awf {enable,disable,list} target` path validates against this set (inv: target-cli).
func KnownTargets() []string {
	return slices.Sorted(maps.Keys(targetRegistry))
}

// resolveTargets maps configured adapter names to their Target values in config
// order, rejecting any unknown name (inv: targets-default-claude).
func resolveTargets(names []string) ([]Target, error) {
	out := make([]Target, 0, len(names))
	for _, n := range names {
		t, ok := targetRegistry[n]
		if !ok {
			return nil, fmt.Errorf("unknown target %q (known: %s)", n, strings.Join(KnownTargets(), ", "))
		}
		if err := t.validate(); err != nil {
			return nil, err
		}
		out = append(out, cloneTargets([]Target{t})[0])
	}
	return out, nil
}

// ResolveTargets resolves defensive copies of the closed built-in target declarations.
func ResolveTargets(names []string) ([]Target, error) { return resolveTargets(names) }

// ValidateTarget validates one resolved target declaration.
func ValidateTarget(target Target) error { return target.validate() }

// BuiltinTarget returns a defensive copy of one named built-in target.
func BuiltinTarget(name string) Target {
	return cloneTargets([]Target{targetRegistry[name]})[0]
}
