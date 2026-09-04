package publisher

import (
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
)

var claudeTarget = artifactregistry.BuiltinTarget("claude")
var piTarget = artifactregistry.BuiltinTarget("pi")

func targetDescriptorProjection(target artifactregistry.Target) string {
	return artifactregistry.TargetDescriptorProjection(target)
}

// targetRecipeProjection excludes declarer name while retaining the fixed
// path and bridge fields that affect target-backed output recipes.
func targetRecipeProjection(target artifactregistry.Target) string {
	return fmt.Sprintf("%#v", struct {
		SkillDir, BridgeFile, BridgeTemplate string
	}{target.SkillDir, target.BridgeFile, target.BridgeTemplate})
}
