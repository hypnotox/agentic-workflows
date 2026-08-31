package publisher

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
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
	enabled    func(*config.Config) bool
	path       string
	tid        string
	kind       string
	sections   []string
	encoder    AgentDialect
	provenance render.CommentStyle
	policy     OutputPolicy
}

func conditionalUnits() []conditionalUnit {
	declarations := artifactregistry.ConditionalUnits()
	out := make([]conditionalUnit, len(declarations))
	for i, declaration := range declarations {
		out[i] = conditionalUnit{
			enabled: declaration.Enabled, path: declaration.Path, tid: declaration.TemplateID,
			kind: declaration.Kind, sections: append([]string(nil), declaration.Sections...),
			encoder: AgentDialect(declaration.Encoder), provenance: declaration.Provenance,
			policy: declaration.Policy,
		}
	}
	return out
}

// Compatibility projections for package-local callers and behavioral tests.
var hookNames = artifactregistry.HookNames()
var runnerSections = artifactregistry.RunnerSections()

// liveTemplateEncoders derives every embedded identity that can participate in render authority.
func liveTemplateEncoders(p renderInputs) map[string]AgentDialect {
	encoders := map[string]AgentDialect{topicTID: MarkdownAgentDialect, topicIndexTID: MarkdownAgentDialect, pitfallEntryTID: MarkdownAgentDialect}
	if len(p.cfg.LocalDocs) != 0 {
		encoders[localDocTID] = MarkdownAgentDialect
	}
	for _, descriptor := range kindDescriptors {
		if descriptor.freeformDomain {
			encoders[descriptor.templateID(projectCatalog(p), "")] = MarkdownAgentDialect
		}
	}
	for name := range projectCatalog(p).Skills {
		encoders[skillTID(p, name)] = MarkdownAgentDialect
	}
	for name := range projectCatalog(p).Agents {
		encoders[agentTID(p, name)] = MarkdownAgentDialect
	}
	for _, entry := range projectCatalog(p).Docs {
		encoders[entry.TID] = MarkdownAgentDialect
	}
	for _, target := range p.targets() {
		if target.BridgeTemplate != "" {
			encoders[target.BridgeTemplate] = MarkdownAgentDialect
		}
		for _, output := range target.Outputs {
			encoders[output.TemplateID] = output.Encoder
		}
	}
	for _, unit := range artifactregistry.ConditionalUnits() {
		encoders[unit.TemplateID] = AgentDialect(unit.Encoder)
	}
	for _, root := range resident.RootNames() {
		encoders[residentGitignoreTID(root)] = PlainAgentDialect
	}
	return encoders
}

func liveTemplateIDs(p renderInputs) map[string]bool {
	ids := map[string]bool{}
	for tid := range liveTemplateEncoders(p) {
		ids[tid] = true
	}
	return ids
}
