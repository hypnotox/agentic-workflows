package publisher

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/resident"
)

const (
	targetBridgeKind = artifactregistry.TargetBridgeKind
	bridgeTID        = artifactregistry.ClaudeBridgeTemplateID
	topicTID         = artifactregistry.TopicTemplateID
	topicIndexTID    = artifactregistry.TopicIndexTemplateID
	pitfallEntryTID  = artifactregistry.PitfallEntryTemplateID
	localDocTID      = artifactregistry.LocalDocTemplateID
)

func hookTID(name string) string              { return artifactregistry.HookTemplateID(name) }
func residentGitignoreTID(name string) string { return artifactregistry.ResidentTemplateID(name) }
func isResidentGitignoreTID(tid string) bool  { return strings.HasSuffix(tid, "/gitignore.tmpl") }

type singletonSpec struct {
	kind     string
	tid      string
	outPath  func(Layout) string
	sections func(*catalog.Catalog) []string
}

func plainSingletons(cat *catalog.Catalog) []singletonSpec {
	declarations := artifactregistry.PlainSingletons(cat)
	out := make([]singletonSpec, len(declarations))
	for i, declaration := range declarations {
		declaration := declaration
		out[i] = singletonSpec{
			kind:     declaration.Kind,
			tid:      declaration.TemplateID,
			outPath:  func(Layout) string { return declaration.OutputPath },
			sections: func(*catalog.Catalog) []string { return append([]string(nil), declaration.Sections...) },
		}
	}
	return out
}

type conditionalUnit struct {
	enabled  func(*config.Config) bool
	path     string
	tid      string
	kind     string
	sections []string
	policy   outputplan.Policy
}

func conditionalUnits() []conditionalUnit {
	declarations := artifactregistry.ConditionalUnits()
	out := make([]conditionalUnit, len(declarations))
	for i, declaration := range declarations {
		out[i] = conditionalUnit{
			enabled: declaration.Enabled, path: declaration.Path, tid: declaration.TemplateID,
			kind: declaration.Kind, sections: append([]string(nil), declaration.Sections...),
			policy: declaration.Policy,
		}
	}
	return out
}

// liveMarkdownTemplateIDs derives every Markdown identity that can participate
// in render authority. Core scripts and resident markers are deliberately plain.
func liveMarkdownTemplateIDs(p renderInputs) map[string]bool {
	return liveMarkdownTemplateIDsWithKinds(p, allKindDescriptors())
}

func liveMarkdownTemplateIDsWithKinds(p renderInputs, kinds []kindDescriptor) map[string]bool {
	ids := map[string]bool{topicTID: true, topicIndexTID: true, pitfallEntryTID: true}
	if len(p.cfg.LocalDocs) != 0 {
		ids[localDocTID] = true
	}
	for _, descriptor := range kinds {
		if descriptor.freeformDomain {
			ids[descriptor.templateID(projectCatalog(p), "")] = true
		}
	}
	for name := range projectCatalog(p).Skills {
		ids[skillTID(p, name)] = true
	}
	for _, entry := range projectCatalog(p).Docs {
		ids[entry.TID] = true
	}
	for _, target := range p.targets() {
		if target.BridgeTemplate != "" {
			ids[target.BridgeTemplate] = true
		}
	}
	return ids
}

func liveTemplateIDs(p renderInputs) map[string]bool {
	ids := liveMarkdownTemplateIDs(p)
	for _, unit := range artifactregistry.ConditionalUnits() {
		ids[unit.TemplateID] = true
	}
	for _, root := range resident.RootNames() {
		ids[residentGitignoreTID(root)] = true
	}
	return ids
}
