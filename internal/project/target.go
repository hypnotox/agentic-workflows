package project

import "github.com/hypnotox/agentic-workflows/internal/artifactregistry"

func KnownTargets() []string { return artifactregistry.KnownTargets() }

func resolveTargets(names []string) ([]artifactregistry.Target, error) {
	return artifactregistry.ResolveTargets(names)
}

var claudeTarget = artifactregistry.BuiltinTarget("claude")
var piTarget = artifactregistry.BuiltinTarget("pi")
