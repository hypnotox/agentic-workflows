package artifactregistry

import (
	"reflect"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
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
		if target.SkillDir == "" {
			t.Fatalf("target %s has no skill directory", target.Name)
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
	for _, kind := range []string{"skills", "docs"} {
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

func TestRegistryCanonicalMetadata(t *testing.T) {
	if got, want := Kinds(), []Kind{
		{Plural: "skills", Singular: "skill", Cardinality: CardinalityCatalog, Targeting: TargetAdapter, Owner: OwnerCatalog, TemplatePattern: "skills/%s/SKILL.md.tmpl", OwnsParts: true},
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
		{Name: "claude", SkillDir: ".claude/skills", BridgeFile: "CLAUDE.md", BridgeTemplate: "claude/CLAUDE.md.tmpl"},
		{Name: "pi", SkillDir: ".pi/skills"},
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
	if target.Name != "pi" || target.SkillDir != ".pi/skills" || target.BridgeFile != "" {
		t.Fatalf("Pi target declaration = %#v", target)
	}
}

func TestRegistryReturnsDefensiveProjections(t *testing.T) {
	targets := Targets()
	targets[0].Name = "changed"
	if Targets()[0].Name == "changed" {
		t.Fatal("target projection aliases registry")
	}
	if got, ok := KindBySingular("skill"); !ok || got.Plural != "skills" {
		t.Fatalf("kind projection = %#v, %v", got, ok)
	}
}
