package project

import "github.com/hypnotox/agentic-workflows/internal/projectstate"

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

// KnownTargets returns the closed built-in target names.
func KnownTargets() []string { return projectstate.KnownTargets() }

func resolveTargets(names []string) ([]Target, error) { return projectstate.ResolveTargets(names) }

var claudeTarget = projectstate.BuiltinTarget("claude")
var piTarget = projectstate.BuiltinTarget("pi")
