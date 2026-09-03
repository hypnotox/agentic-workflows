package publisher

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

func TestDomainDescriptorProjectsCatalogSections(t *testing.T) {
	descriptor, ok := descriptorByPlural("domains")
	if !ok {
		t.Fatal("domains descriptor is absent")
	}
	sections, present := descriptor.sections(catalog.Standard, "example")
	if present || len(sections) == 0 {
		t.Fatalf("domain sections = %v, present=%t", sections, present)
	}
}

func TestBuildOutputDefinitionsCoversTargetAndMetadataEdges(t *testing.T) {
	cfg, err := config.ParseTree(".awf", []byte("prefix: example\ndomains: [d]\n"), configReaderAdapter{memoryProjectReader{}})
	if err != nil {
		t.Fatal(err)
	}
	sharedOutput := artifactregistry.TargetOutput{Path: "shared", TemplateID: "template", Producer: artifactregistry.TargetOutputTemplate, Encoder: artifactregistry.MarkdownAgentDialect, Provenance: render.HTMLComment, PolicyDeclared: true}
	targets := []artifactregistry.Target{{Name: "one", AgentDialect: artifactregistry.MarkdownAgentDialect, Outputs: []artifactregistry.TargetOutput{sharedOutput}}, {Name: "two", AgentDialect: artifactregistry.MarkdownAgentDialect, Outputs: []artifactregistry.TargetOutput{sharedOutput}}}
	read := memoryProjectReader{".awf/topics/metadata/note.txt": []byte("ignored")}
	decls, err := buildOutputDefinitions(cfg, &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Docs: map[string]catalog.DocEntry{}}, targets, read)
	if err != nil {
		t.Fatal(err)
	}
	shared := -1
	for i, decl := range decls {
		if decl.Path == "shared" {
			shared = i
			break
		}
	}
	if shared < 0 || len(decls[shared].Declarers) != 2 || len(decls[shared].Projections) != 2 {
		t.Fatalf("target definitions = %#v", decls)
	}

	unknownOutput := sharedOutput
	unknownOutput.Path, unknownOutput.RequiresSkill = "missing", "absent"
	unknown := []artifactregistry.Target{{Name: "bad", AgentDialect: artifactregistry.MarkdownAgentDialect, Outputs: []artifactregistry.TargetOutput{unknownOutput}}}
	if _, err := buildOutputDefinitions(cfg, &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Docs: map[string]catalog.DocEntry{}}, unknown, read); err == nil || !strings.Contains(err.Error(), "unknown catalog skill") {
		t.Fatalf("unknown target requirement error = %v", err)
	}
}
