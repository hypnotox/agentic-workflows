package project

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

// AgentDialect names the target-native encoding for rendered agents.
type AgentDialect string

const (
	MarkdownAgentDialect AgentDialect = "markdown"
	PlainAgentDialect    AgentDialect = "plain"
)

// Capability is an awf-owned template capability. It is deliberately closed:
// targets cannot inject arbitrary template data.
type Capability string

const (
	CapabilitySubagentTools  Capability = "subagent-tools"
	CapabilitySessionHandoff Capability = "session-handoff"
	CapabilityEffortSessions Capability = "effort-sessions"
)

type TargetOutputProducer string

const (
	TargetOutputTemplate TargetOutputProducer = "template"
)

type TargetOutputInput struct {
	Path string
	Role ArtifactRole
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
	Policy         OutputPolicy
	PolicyDeclared bool
}

// targetTemplateData is the complete target projection exposed to templates.
func (t Target) targetTemplateData() map[string]any {
	return map[string]any{
		"targetSubagentTools":  t.hasCapability(CapabilitySubagentTools),
		"targetSessionHandoff": t.hasCapability(CapabilitySessionHandoff),
		"targetEffortSessions": t.hasCapability(CapabilityEffortSessions),
	}
}

func (t Target) hasCapability(c Capability) bool {
	return slices.Contains(t.Capabilities, c)
}

// anyTargetHasCapability reports whether any target in the set declares the
// capability. The neutral singleton render (the guide and the singleton docs)
// reads it to set a project-level targetSessionHandoff signal, so
// target-conditional prose in singleton templates renders iff some enabled
// target supports it (ADR-0157 Decision 6); per-target renders still override
// the key with their own targetTemplateData projection.
func anyTargetHasCapability(targets []Target, c Capability) bool {
	return slices.ContainsFunc(targets, func(t Target) bool { return t.hasCapability(c) })
}

// targetDescriptorProjection is stable across declaration ordering and includes
// identity plus every descriptor field. It is hash input for a coalesced node.
func targetDescriptorProjection(t Target) string {
	caps := slices.Clone(t.Capabilities)
	slices.Sort(caps)
	outputs := slices.Clone(t.Outputs)
	slices.SortFunc(outputs, func(a, b TargetOutput) int {
		return strings.Compare(fmt.Sprintf("%#v", a), fmt.Sprintf("%#v", b))
	})
	return fmt.Sprintf("%#v", struct {
		Name, SkillDir, AgentDir, AgentSuffix, BridgeFile, BridgeTemplate string
		AgentDialect                                                      AgentDialect
		Capabilities                                                      []Capability
		Outputs                                                           []TargetOutput
	}{t.Name, t.SkillDir, t.AgentDir, t.AgentSuffix, t.BridgeFile, t.BridgeTemplate, t.AgentDialect, caps, outputs})
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

func (t Target) agentCommentStyle() render.CommentStyle {
	return render.HTMLComment
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
	BridgeTemplate: bridgeTID,
}

var piTarget = Target{
	Name:         "pi",
	SkillDir:     ".pi/skills",
	AgentDir:     ".pi/agents",
	AgentSuffix:  ".md",
	AgentDialect: MarkdownAgentDialect,
	Capabilities: []Capability{CapabilitySubagentTools, CapabilitySessionHandoff, CapabilityEffortSessions},
	Outputs: []TargetOutput{
		{Path: ".pi/extensions/awf-subagents/index.ts", TemplateID: "pi/awf-subagents/index.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: OutputPolicy{}, PolicyDeclared: true},
		{Path: ".pi/extensions/awf-subagents/model-routing.ts", TemplateID: "pi/awf-subagents/model-routing.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: OutputPolicy{}, PolicyDeclared: true},
		{Path: ".pi/extensions/awf-effort/index.ts", RequiresSkill: "effort-workflow", TemplateID: "pi/awf-effort/index.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: OutputPolicy{}, PolicyDeclared: true},
		{Path: ".pi/extensions/awf-effort/client.ts", RequiresSkill: "effort-workflow", TemplateID: "pi/awf-effort/client.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: OutputPolicy{}, PolicyDeclared: true},
		{SkillName: "using-effort", RequiresSkill: "effort-workflow", TemplateID: "skills/using-effort/SKILL.md.tmpl", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, Policy: declaredPolicy("skills", false), PolicyDeclared: true},
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
		if err := t.validate(); err != nil { // coverage-ignore: built-in registry descriptors are validated by descriptor tests
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
