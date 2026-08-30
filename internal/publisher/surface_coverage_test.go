package publisher

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
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

func TestBuildOutputDeclarationsCoversTargetAndMetadataEdges(t *testing.T) {
	cfg, err := config.ParseTree(".awf", []byte("prefix: example\ndomains: [d]\n"), configReaderAdapter{memoryProjectReader{}})
	if err != nil {
		t.Fatal(err)
	}
	targets := []Target{{Name: "one", Outputs: []TargetOutput{{Path: "shared", TemplateID: "template", Inputs: []TargetOutputInput{{Path: "same", Role: ArtifactConfig}, {Path: "same", Role: ArtifactTemplate}}}}}, {Name: "two", Outputs: []TargetOutput{{Path: "shared", TemplateID: "template"}}}}
	read := memoryProjectReader{".awf/topics/metadata/note.txt": []byte("ignored")}
	decls, err := buildOutputDeclarations(cfg, &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}, targets, read, mustCorpus())
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
	if shared < 0 || len(decls[shared].Declarers) != 2 || len(decls[shared].Inputs) != 4 {
		t.Fatalf("target declarations = %#v", decls)
	}

	unknown := []Target{{Name: "bad", Outputs: []TargetOutput{{Path: "missing", RequiresSkill: "absent"}}}}
	if _, err := buildOutputDeclarations(cfg, &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}, unknown, read, mustCorpus()); err == nil || !strings.Contains(err.Error(), "unknown catalog skill") {
		t.Fatalf("unknown target requirement error = %v", err)
	}

	badSidecar := memoryProjectReader{".awf/domains/d.yaml": []byte("data: [\n")}
	cfg, err = config.ParseTree(".awf", []byte("prefix: example\ndomains: [d]\n"), configReaderAdapter{badSidecar})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildOutputDeclarations(cfg, &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}, nil, badSidecar, mustCorpus()); err == nil || !strings.Contains(err.Error(), "domains/d.yaml") {
		t.Fatalf("domain sidecar error = %v", err)
	}
}

func TestBuildOutputDeclarationsPropagatesCatalogSidecarReadFault(t *testing.T) {
	read := failingReadReader{memoryProjectReader: memoryProjectReader{}}
	cfg, err := config.ParseTree(".awf", []byte("prefix: example\n"), configReaderAdapter(read))
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{"implementing": {}}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}
	if _, err := buildOutputDeclarations(cfg, cat, []Target{{Name: "test"}}, read, mustCorpus()); err == nil || !strings.Contains(err.Error(), "read fault") {
		t.Fatalf("declaration read error = %v", err)
	}
}
