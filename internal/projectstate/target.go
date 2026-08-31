// Package projectstate owns immutable loaded project facts and resolved target declarations.
package projectstate

import "github.com/hypnotox/agentic-workflows/internal/artifactregistry"

// AgentDialect names the target-native encoding for rendered agents.
type AgentDialect = artifactregistry.AgentDialect

const (
	MarkdownAgentDialect = artifactregistry.MarkdownAgentDialect
	PlainAgentDialect    = artifactregistry.PlainAgentDialect
)

// Capability is an awf-owned closed template capability.
type Capability = artifactregistry.Capability

const (
	CapabilitySubagentTools  = artifactregistry.CapabilitySubagentTools
	CapabilitySessionHandoff = artifactregistry.CapabilitySessionHandoff
)

// TargetOutputProducer identifies how a target-owned output is produced.
type TargetOutputProducer = artifactregistry.TargetOutputProducer

const TargetOutputTemplate = artifactregistry.TargetOutputTemplate

// TargetOutputInput declares one semantic input to a target-owned output.
type TargetOutputInput = artifactregistry.TargetOutputInput

// TargetOutput declares a target-owned non-catalog output.
type TargetOutput = artifactregistry.TargetOutput

// Target places adapter artifacts for one runtime.
type Target = artifactregistry.Target

// KnownTargets returns the known adapter names in sorted order.
func KnownTargets() []string { return artifactregistry.KnownTargets() }

// ResolveTargets resolves defensive copies of the closed built-in declarations.
func ResolveTargets(names []string) ([]Target, error) { return artifactregistry.ResolveTargets(names) }

// ValidateTarget validates one resolved target declaration.
func ValidateTarget(target Target) error { return artifactregistry.ValidateTarget(target) }

// BuiltinTarget returns a defensive copy of one named built-in target.
func BuiltinTarget(name string) Target { return artifactregistry.BuiltinTarget(name) }
