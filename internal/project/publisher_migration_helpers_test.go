package project

import (
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

func pitfallSource(title, extra string) string {
	return "---\ntitle: " + title + "\n" + extra + "---\nok\n"
}

func (l Layout) templateMap() map[string]any {
	docs := map[string]any{}
	for key, value := range l.Docs {
		docs[key] = value
	}
	out := map[string]any{"docsDir": l.DocsDir, "docs": docs, "domainsDir": l.DomainsDir}
	for key, value := range l.Singletons {
		out[key] = value
	}
	return out
}

func targetTemplateData(target artifactregistry.Target) map[string]any {
	return map[string]any{
		"targetSubagentTools":  slices.Contains(target.Capabilities, artifactregistry.CapabilitySubagentTools),
		"targetSessionHandoff": slices.Contains(target.Capabilities, artifactregistry.CapabilitySessionHandoff),
	}
}

func gatedCommandsDisplay() string {
	names := clispec.GatedCommandNames()
	for i := range names {
		names[i] = "`" + names[i] + "`"
	}
	return strings.Join(names, ", ")
}
