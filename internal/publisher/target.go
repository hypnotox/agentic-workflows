package publisher

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

// AgentDialect preserves the project package's target-dialect compatibility name.
type AgentDialect = projectstate.AgentDialect

const (
	// MarkdownAgentDialect preserves the Markdown target-dialect compatibility value.
	MarkdownAgentDialect = projectstate.MarkdownAgentDialect
	// PlainAgentDialect preserves the plain-text target-dialect compatibility value.
	PlainAgentDialect = projectstate.PlainAgentDialect
)

// Capability preserves the project package's target-capability compatibility name.
type Capability = projectstate.Capability

const (
	// CapabilitySubagentTools preserves the subagent-tools capability value.
	CapabilitySubagentTools = projectstate.CapabilitySubagentTools
	// CapabilitySessionHandoff preserves the session-handoff capability value.
	CapabilitySessionHandoff = projectstate.CapabilitySessionHandoff
	// CapabilityEffortSessions preserves the effort-sessions capability value.
	CapabilityEffortSessions = projectstate.CapabilityEffortSessions
)

// TargetOutputProducer preserves the project package's producer compatibility name.
type TargetOutputProducer = projectstate.TargetOutputProducer

// TargetOutputTemplate preserves the template-producer compatibility value.
const TargetOutputTemplate = projectstate.TargetOutputTemplate

// TargetOutputInput preserves the project package's target-input compatibility name.
type TargetOutputInput = projectstate.TargetOutputInput

// TargetOutput preserves the project package's target-output compatibility name.
type TargetOutput = projectstate.TargetOutput

// Target preserves the project package's resolved-target compatibility name.
type Target = projectstate.Target

var claudeTarget = projectstate.BuiltinTarget("claude")
var piTarget = projectstate.BuiltinTarget("pi")

func targetTemplateData(t Target) map[string]any {
	return map[string]any{
		"targetSubagentTools":  hasCapability(t, CapabilitySubagentTools),
		"targetSessionHandoff": hasCapability(t, CapabilitySessionHandoff),
		"targetEffortSessions": hasCapability(t, CapabilityEffortSessions),
	}
}

func hasCapability(t Target, capability Capability) bool {
	return slices.Contains(t.Capabilities, capability)
}

func anyTargetHasCapability(targets []Target, capability Capability) bool {
	return slices.ContainsFunc(targets, func(target Target) bool {
		return hasCapability(target, capability)
	})
}

func targetDescriptorProjection(target Target) string {
	capabilities := slices.Clone(target.Capabilities)
	slices.Sort(capabilities)
	outputs := slices.Clone(target.Outputs)
	slices.SortFunc(outputs, func(left, right TargetOutput) int {
		return strings.Compare(fmt.Sprintf("%#v", left), fmt.Sprintf("%#v", right))
	})
	return fmt.Sprintf("%#v", struct {
		Name, SkillDir, AgentDir, AgentSuffix, BridgeFile, BridgeTemplate string
		AgentDialect                                                      AgentDialect
		Capabilities                                                      []Capability
		Outputs                                                           []TargetOutput
	}{target.Name, target.SkillDir, target.AgentDir, target.AgentSuffix, target.BridgeFile, target.BridgeTemplate, target.AgentDialect, capabilities, outputs})
}

func agentCommentStyle(Target) render.CommentStyle { return render.HTMLComment }
