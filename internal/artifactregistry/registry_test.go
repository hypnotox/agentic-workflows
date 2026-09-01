package artifactregistry

import (
	"reflect"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
)

func TestRegistryStableUniqueMetadata(t *testing.T) {
	seenKinds, seenTargets, seenHooks := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, kind := range Kinds() {
		if kind.Plural == "" || kind.Singular == "" || kind.TemplatePattern == "" && kind.Plural != "docs" {
			t.Fatalf("incomplete kind %#v", kind)
		}
		if seenKinds[kind.Plural] || seenKinds[kind.Singular] {
			t.Fatalf("duplicate kind %#v", kind)
		}
		seenKinds[kind.Plural], seenKinds[kind.Singular] = true, true
	}
	for _, target := range Targets() {
		if target.Name == "" || seenTargets[target.Name] {
			t.Fatalf("duplicate or incomplete target %#v", target)
		}
		seenTargets[target.Name] = true
		outputs := map[string]bool{}
		for _, output := range target.Outputs {
			if output.TemplateID == "" || (output.Path == "" && output.SkillName == "") || (output.Path != "" && output.SkillName != "") {
				t.Fatalf("incomplete output %#v", output)
			}
			if outputs[output.Path] {
				t.Fatalf("target %s declares output path %q twice", target.Name, output.Path)
			}
			outputs[output.Path] = true
		}
	}
	for _, hook := range HookArtifacts() {
		if hook.Name == "" || hook.TemplateID == "" || hook.OutputPath == "" || hook.Owner == "" || !hook.Checked || seenHooks[hook.Name] {
			t.Fatalf("duplicate or incomplete hook %#v", hook)
		}
		seenHooks[hook.Name] = true
	}
	if got, want := Hooks(), []string{"pre-commit", "commit-msg", "pre-push", "pre-merge-commit", "reference-transaction"}; !slices.Equal(got, want) {
		t.Fatalf("hook order = %v, want %v", got, want)
	}
}

func TestCatalogProjectionPreservesCatalogAuthority(t *testing.T) {
	cat := catalog.CompleteView().Catalog()
	for _, kind := range []string{"skills", "agents", "docs"} {
		names, ok := CatalogNames(cat, kind)
		if !ok || !slices.IsSorted(names) || len(names) == 0 {
			t.Fatalf("catalog names for %s = %v, %v", kind, names, ok)
		}
		for _, name := range names {
			_, present := Sections(cat, kind, name)
			if !present {
				t.Fatalf("sections for %s/%s are absent", kind, name)
			}
			if TemplateID(cat, kind, name) == "" {
				t.Fatalf("template id for %s/%s is empty", kind, name)
			}
		}
	}
	sections, present := Sections(cat, "domains", "anything")
	if present || !slices.Equal(sections, cat.DomainDoc.Sections) || TemplateID(cat, "domains", "anything") == "" {
		t.Fatal("domain projection drifted from catalog/registry")
	}
}

func TestRegistryTargetOutputNormalizationPreservesCollisionInputs(t *testing.T) {
	one := Target{SkillDir: ".one/skills", Outputs: []TargetOutput{{SkillName: "workflow", RequiresSkill: "workflow"}}}
	two := Target{SkillDir: ".two/skills", Outputs: []TargetOutput{{SkillName: "workflow", RequiresSkill: "workflow"}}}
	oneOutput := ResolveTargetArtifacts(one, "acme", []string{"workflow"})
	twoOutput := ResolveTargetArtifacts(two, "acme", []string{"workflow"})
	if got, want := oneOutput[0].Output.Path, ".one/skills/acme-workflow/SKILL.md"; got != want {
		t.Fatalf("first resolved path = %q, want %q", got, want)
	}
	if oneOutput[0].Output.Path == twoOutput[0].Output.Path {
		t.Fatal("different target directories collapsed to one collision key")
	}
	if got := ResolveTargetArtifacts(one, "acme", nil); len(got) != 0 {
		t.Fatalf("required-skill filtering returned %#v", got)
	}
}

func TestUnknownRequiredSkillPreservesExactRefusal(t *testing.T) {
	target := Target{Name: "pi", Outputs: []TargetOutput{{Path: "out", RequiresSkill: "missing"}}}
	err := ValidateTargetRequirements(target, &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}})
	if got, want := err.Error(), `target "pi" output "out" requires unknown catalog skill "missing"`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestRegistryCanonicalMetadata(t *testing.T) {
	if got, want := Kinds(), []Kind{
		{Plural: "skills", Singular: "skill", Cardinality: CardinalityCatalog, Targeting: TargetAdapter, Owner: OwnerCatalog, TemplatePattern: "skills/%s/SKILL.md.tmpl", OwnsParts: true},
		{Plural: "agents", Singular: "agent", Cardinality: CardinalityCatalog, Targeting: TargetAdapter, Owner: OwnerCatalog, TemplatePattern: "agents/%s.md.tmpl", OwnsParts: true},
		{Plural: "docs", Singular: "doc", Cardinality: CardinalityCatalog, Targeting: TargetNeutral, Owner: OwnerCatalog, OwnsParts: true},
		{Plural: "domains", Singular: "domain", Cardinality: CardinalityFreeform, Targeting: TargetNeutral, Owner: OwnerCatalog, TemplatePattern: "domains/domain.md.tmpl", OwnsParts: true},
	}; !slices.Equal(got, want) {
		t.Fatalf("Kinds() = %#v, want %#v", got, want)
	}
	if got, want := HookArtifacts(), []Hook{
		{Name: "pre-commit", TemplateID: "hooks/pre-commit.sh.tmpl", OutputPath: ".awf/hooks/pre-commit.sh", Owner: "hooks", Checked: true},
		{Name: "commit-msg", TemplateID: "hooks/commit-msg.sh.tmpl", OutputPath: ".awf/hooks/commit-msg.sh", Owner: "hooks", Checked: true},
		{Name: "pre-push", TemplateID: "hooks/pre-push.sh.tmpl", OutputPath: ".awf/hooks/pre-push.sh", Owner: "hooks", Checked: true},
		{Name: "pre-merge-commit", TemplateID: "hooks/pre-merge-commit.sh.tmpl", OutputPath: ".awf/hooks/pre-merge-commit.sh", Owner: "hooks", Checked: true},
		{Name: "reference-transaction", TemplateID: "hooks/reference-transaction.sh.tmpl", OutputPath: ".awf/hooks/reference-transaction.sh", Owner: "hooks", Checked: true},
	}; !slices.Equal(got, want) {
		t.Fatalf("HookArtifacts() = %#v, want %#v", got, want)
	}
	if got, want := Targets(), []Target{
		{Name: "claude", SkillDir: ".claude/skills", AgentDir: ".claude/agents", AgentSuffix: ".md", AgentDialect: MarkdownAgentDialect, BridgeFile: "CLAUDE.md", BridgeTemplate: "claude/CLAUDE.md.tmpl"},
		{Name: "pi", SkillDir: ".pi/skills", AgentDir: ".pi/agents", AgentSuffix: ".md", AgentDialect: MarkdownAgentDialect, Capabilities: []Capability{CapabilitySubagentTools, CapabilitySessionHandoff}, Outputs: []TargetOutput{
			{Path: ".pi/extensions/awf-subagents/index.ts", TemplateID: "pi/awf-subagents/index.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, PolicyDeclared: true},
			{Path: ".pi/extensions/awf-subagents/model-routing.ts", TemplateID: "pi/awf-subagents/model-routing.ts.tmpl", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, PolicyDeclared: true},
		}},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Targets() = %#v, want %#v", got, want)
	}
}

func TestRegistryOperationalOwnershipAndParticipation(t *testing.T) {
	resident := Resident("efforts")
	if resident.Owner != OwnerResident || !resident.Participation.Check || resident.OutputPath != ".awf/efforts/.gitignore" {
		t.Fatalf("resident declaration = %#v", resident)
	}
	target := BuiltinTarget("pi")
	artifacts := ResolveTargetArtifacts(target, "awf", nil)
	if len(artifacts) == 0 || artifacts[0].Owner != OwnerTarget || !artifacts[0].Participation.Check {
		t.Fatalf("target artifact declarations = %#v", artifacts)
	}
}

func TestRegistryReturnsDefensiveProjections(t *testing.T) {
	targets := Targets()
	targets[0].Name = "changed"
	if len(targets) > 1 && len(targets[1].Capabilities) > 0 {
		targets[1].Capabilities[0] = "changed"
	}
	if Targets()[0].Name == "changed" {
		t.Fatal("target projection aliases registry")
	}
	if got, ok := KindBySingular("skill"); !ok || got.Plural != "skills" {
		t.Fatalf("kind projection = %#v, %v", got, ok)
	}
}
