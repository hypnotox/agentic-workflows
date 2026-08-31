package project

import "github.com/hypnotox/agentic-workflows/internal/artifactregistry"

type AgentDialect = artifactregistry.AgentDialect

const (
	MarkdownAgentDialect = artifactregistry.MarkdownAgentDialect
	PlainAgentDialect    = artifactregistry.PlainAgentDialect
)

type Capability = artifactregistry.Capability

const (
	CapabilitySubagentTools  = artifactregistry.CapabilitySubagentTools
	CapabilitySessionHandoff = artifactregistry.CapabilitySessionHandoff
)

type TargetOutputProducer = artifactregistry.TargetOutputProducer

const TargetOutputTemplate = artifactregistry.TargetOutputTemplate

type TargetOutputInput = artifactregistry.TargetOutputInput
type TargetOutput = artifactregistry.TargetOutput
type Target = artifactregistry.Target

func KnownTargets() []string { return artifactregistry.KnownTargets() }

func resolveTargets(names []string) ([]Target, error) { return artifactregistry.ResolveTargets(names) }

var claudeTarget = artifactregistry.BuiltinTarget("claude")
var piTarget = artifactregistry.BuiltinTarget("pi")
