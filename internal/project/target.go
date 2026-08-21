package project

import "github.com/hypnotox/agentic-workflows/internal/projectstate"

type AgentDialect = projectstate.AgentDialect

const (
	MarkdownAgentDialect = projectstate.MarkdownAgentDialect
	PlainAgentDialect    = projectstate.PlainAgentDialect
)

type Capability = projectstate.Capability

const (
	CapabilitySubagentTools  = projectstate.CapabilitySubagentTools
	CapabilitySessionHandoff = projectstate.CapabilitySessionHandoff
	CapabilityEffortSessions = projectstate.CapabilityEffortSessions
)

type TargetOutputProducer = projectstate.TargetOutputProducer

const TargetOutputTemplate = projectstate.TargetOutputTemplate

type TargetOutputInput = projectstate.TargetOutputInput
type TargetOutput = projectstate.TargetOutput
type Target = projectstate.Target

func KnownTargets() []string                          { return projectstate.KnownTargets() }
func resolveTargets(names []string) ([]Target, error) { return projectstate.ResolveTargets(names) }

var claudeTarget = projectstate.BuiltinTarget("claude")
var piTarget = projectstate.BuiltinTarget("pi")
