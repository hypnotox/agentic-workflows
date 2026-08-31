package publisher

import (
	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

// AgentDialect preserves the project package's target-dialect compatibility name.
type AgentDialect = projectstate.AgentDialect

const (
	MarkdownAgentDialect = projectstate.MarkdownAgentDialect
	PlainAgentDialect    = projectstate.PlainAgentDialect
)

// Capability preserves the project package's target-capability compatibility name.
type Capability = projectstate.Capability

const (
	CapabilitySubagentTools  = projectstate.CapabilitySubagentTools
	CapabilitySessionHandoff = projectstate.CapabilitySessionHandoff
)

type TargetOutputProducer = projectstate.TargetOutputProducer

const TargetOutputTemplate = projectstate.TargetOutputTemplate

type TargetOutputInput = projectstate.TargetOutputInput
type TargetOutput = projectstate.TargetOutput
type Target = projectstate.Target

var claudeTarget = projectstate.BuiltinTarget("claude")
var piTarget = projectstate.BuiltinTarget("pi")

func targetTemplateData(target Target) map[string]any {
	return artifactregistry.TargetTemplateData(target)
}

func anyTargetHasCapability(targets []Target, capability Capability) bool {
	return artifactregistry.AnyTargetHasCapability(targets, capability)
}

func targetDescriptorProjection(target Target) string {
	return artifactregistry.TargetDescriptorProjection(target)
}

func agentCommentStyle(Target) render.CommentStyle { return render.HTMLComment }
