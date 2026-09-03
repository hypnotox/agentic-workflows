package publisher

import (
	"fmt"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

var claudeTarget = artifactregistry.BuiltinTarget("claude")
var piTarget = artifactregistry.BuiltinTarget("pi")

func targetTemplateData(target artifactregistry.Target) map[string]any {
	return artifactregistry.TargetTemplateData(target)
}

func anyTargetHasCapability(targets []artifactregistry.Target, capability artifactregistry.Capability) bool {
	return artifactregistry.AnyTargetHasCapability(targets, capability)
}

func targetDescriptorProjection(target artifactregistry.Target) string {
	return artifactregistry.TargetDescriptorProjection(target)
}

// targetRecipeProjection excludes declarer name but retains every target field
// that artifactConfigHash can fold into target-backed output recipes.
func targetRecipeProjection(target artifactregistry.Target) string {
	capabilities := slices.Clone(target.Capabilities)
	slices.Sort(capabilities)
	return fmt.Sprintf("%#v", struct {
		SkillDir                  string
		AgentDialect              artifactregistry.AgentDialect
		BridgeFile, BridgeTemplate string
		Capabilities              []artifactregistry.Capability
		Outputs                   []artifactregistry.TargetOutput
	}{target.SkillDir, target.AgentDialect, target.BridgeFile, target.BridgeTemplate, capabilities, target.Outputs})
}

func agentCommentStyle(artifactregistry.Target) render.CommentStyle { return render.HTMLComment }
