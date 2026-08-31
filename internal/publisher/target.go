package publisher

import (
	"fmt"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

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

var claudeTarget = artifactregistry.BuiltinTarget("claude")
var piTarget = artifactregistry.BuiltinTarget("pi")

func targetTemplateData(target Target) map[string]any {
	return artifactregistry.TargetTemplateData(target)
}

func anyTargetHasCapability(targets []Target, capability Capability) bool {
	return artifactregistry.AnyTargetHasCapability(targets, capability)
}

func targetDescriptorProjection(target Target) string {
	return artifactregistry.TargetDescriptorProjection(target)
}

// targetRecipeProjection excludes declarer name but retains every target field
// that artifactConfigHash can fold into target-backed output recipes.
func targetRecipeProjection(target Target) string {
	capabilities := slices.Clone(target.Capabilities)
	slices.Sort(capabilities)
	return fmt.Sprintf("%#v", struct {
		SkillDir, AgentDir, AgentSuffix string
		AgentDialect                    AgentDialect
		BridgeFile, BridgeTemplate      string
		Capabilities                    []Capability
		Outputs                         []TargetOutput
	}{target.SkillDir, target.AgentDir, target.AgentSuffix, target.AgentDialect, target.BridgeFile, target.BridgeTemplate, capabilities, target.Outputs})
}

func agentCommentStyle(Target) render.CommentStyle { return render.HTMLComment }
