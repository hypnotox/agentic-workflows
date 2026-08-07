package contextq

import (
	"reflect"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

func testArtifactAuthorities(docsDir string, corpus adr.Corpus) artifactAuthorities {
	return artifactAuthorities{Layout: project.Layout{DocsDir: docsDir, ADRDir: docsDir + "/decisions", IndexMd: docsDir + "/decisions/INDEX.md", DomainsDir: docsDir + "/domains"}, ADRs: corpus}
}

// invariant: rendering/sync-and-drift:managed-output-attribution (TestArtifactRecordsFollowDeclarations)
func TestArtifactRecordsFollowDeclarations(t *testing.T) {
	decls := []project.OutputDeclaration{{Path: "docs/out.md", TemplateID: "docs/out.md.tmpl", Declarers: []string{"out"}, Inputs: []project.OutputInput{{Path: ".awf/docs/parts/out/content.md", Role: project.ArtifactConventionPart}}}}
	generated := artifactRecords("docs/out.md", decls, testArtifactAuthorities("docs", mustCorpus(nil)))
	if len(generated) != 1 || generated[0].Role != project.ArtifactManagedOutput || len(generated[0].Sources) != 1 {
		t.Fatalf("generated=%#v", generated)
	}
	source := artifactRecords(".awf/docs/parts/out/content.md", decls, testArtifactAuthorities("docs", mustCorpus(nil)))
	if len(source) != 1 || source[0].Role != project.ArtifactConventionPart || len(source[0].Outputs) != 1 || source[0].Outputs[0].Path != "docs/out.md" {
		t.Fatalf("source=%#v", source)
	}
	unmanaged := artifactRecords("docs/lookalike.md", decls, testArtifactAuthorities("docs", mustCorpus(nil)))
	if unmanaged == nil || len(unmanaged) != 0 {
		t.Fatalf("unmanaged=%#v", unmanaged)
	}
	tree, _ := snapshot.NewTree([]snapshot.File{{Path: "docs/out.md", Mode: snapshot.Regular, Bytes: []byte("current")}})
	lock := &manifest.Lock{Files: map[string]manifest.Entry{"docs/out.md": {OutputHash: manifest.Hash([]byte("old"))}}}
	applyArtifactSnapshots(generated, "docs/out.md", tree, lock)
	if generated[0].Snapshot == nil || !generated[0].Snapshot.InManifest || !generated[0].Snapshot.Drifted {
		t.Fatalf("snapshot=%#v", generated[0].Snapshot)
	}
}

// invariant: tooling/context-and-topic:context-known-artifact-navigation (TestArtifactNavigationCoversClosedRolesOrderingAndLookalikes)
func TestArtifactNavigationCoversClosedRolesOrderingAndLookalikes(t *testing.T) {
	// The corpus is deliberately mixed: a pending record is reachable only
	// through its slug identity, so a number-keyed lookup or a number-valued
	// identity would drop it from artifact navigation entirely.
	parsed := mustCorpus([]adr.ADR{
		{Number: "0007", Filename: "0007-real.md"},
		{Slug: "a-pending-record", Filename: "a-pending-record.md", Format: adr.CurrentStateV3},
	})
	decls := []project.OutputDeclaration{
		{Path: "documentation/config-reference.md", TemplateID: "docs/config-reference.md.tmpl", Declarers: []string{"config-reference"}},
		{Path: "documentation/domains/d.md", TemplateID: "domains/domain.md.tmpl", Declarers: []string{"generated-domain"}},
		{Path: "documentation/topics/d/t.md", TemplateID: "topics/topic.md.tmpl", Declarers: []string{"topic:d/t"}},
		{Path: "documentation/decisions/INDEX.md", Declarers: []string{"generated-index"}},
		{Path: "generated.md", TemplateID: "docs/out.tmpl", Declarers: []string{"owner"}, Inputs: []project.OutputInput{
			{Path: ".awf/config.yaml", Role: project.ArtifactConfig},
			{Path: "templates/docs/out.tmpl", Role: project.ArtifactTemplate},
			{Path: ".awf/docs/parts/out/content.md", Role: project.ArtifactConventionPart},
			{Path: ".awf/docs/out.yaml", Role: project.ArtifactAuthoredData},
			{Path: ".awf/topics/metadata/d/t.yaml", Role: project.ArtifactTopicMetadata},
			{Path: ".awf/topics/parts/d/t/current-state.md", Role: project.ArtifactClaimPart},
			{Path: "documentation/decisions/0007-real.md", Role: project.ArtifactDecisionRecord},
		}},
		{Path: "second.md", TemplateID: "docs/second.tmpl", Declarers: []string{"second"}, Inputs: []project.OutputInput{{Path: "templates/docs/out.tmpl", Role: project.ArtifactTemplate}}},
	}
	authorities := testArtifactAuthorities("documentation", parsed)
	cases := []struct {
		path       string
		role       project.ArtifactRole
		identity   string
		navigation []artifactLink
	}{
		{".awf/config.yaml", project.ArtifactConfig, "project-config", []artifactLink{{Path: "documentation/config-reference.md", Label: "configuration reference"}}},
		{".awf/awf.lock", project.ArtifactLock, "project-lock", []artifactLink{{Path: ".awf/config.yaml", Label: "project config"}, {Path: "documentation/config-reference.md", Label: "configuration reference"}}},
		{".awf/awf.lock", project.ArtifactManifest, "output-manifest", []artifactLink{{Path: "documentation/config-reference.md", Label: "configuration reference"}}},
		{"templates/docs/out.tmpl", project.ArtifactTemplate, "docs/out.tmpl", []artifactLink{{Path: "generated.md", Label: "managed output"}, {Path: "second.md", Label: "managed output"}}},
		{".awf/docs/parts/out/content.md", project.ArtifactConventionPart, ".awf/docs/parts/out/content.md", []artifactLink{{Path: "generated.md", Label: "managed output"}}},
		{".awf/docs/out.yaml", project.ArtifactAuthoredData, ".awf/docs/out.yaml", []artifactLink{{Path: "generated.md", Label: "managed output"}}},
		{".awf/topics/metadata/d/t.yaml", project.ArtifactTopicMetadata, "d/t", []artifactLink{{Path: "documentation/domains/d.md", Label: "domain document"}, {Path: "documentation/topics/d/t.md", Label: "topic document"}}},
		{".awf/topics/parts/d/t/current-state.md", project.ArtifactClaimPart, "d/t", []artifactLink{{Path: "documentation/domains/d.md", Label: "domain document"}, {Path: "documentation/topics/d/t.md", Label: "topic document"}}},
		{"documentation/decisions/0007-real.md", project.ArtifactDecisionRecord, "0007", []artifactLink{{Path: "documentation/decisions/INDEX.md", Label: "decision index"}}},
		{"documentation/decisions/a-pending-record.md", project.ArtifactDecisionRecord, "a-pending-record", []artifactLink{{Path: "documentation/decisions/INDEX.md", Label: "decision index"}}},
		{"generated.md", project.ArtifactManagedOutput, "docs/out.tmpl", []artifactLink{{Path: ".awf/config.yaml", Label: "project config"}, {Path: ".awf/docs/out.yaml", Label: "authored data"}, {Path: ".awf/docs/parts/out/content.md", Label: "convention part"}, {Path: ".awf/topics/metadata/d/t.yaml", Label: "topic metadata"}, {Path: ".awf/topics/parts/d/t/current-state.md", Label: "claim part"}, {Path: "documentation/decisions/0007-real.md", Label: "decision record"}}},
	}
	for _, tc := range cases {
		records := artifactRecords(tc.path, decls, authorities)
		idx := slices.IndexFunc(records, func(record artifactRecord) bool { return record.Role == tc.role })
		if idx < 0 {
			t.Fatalf("%s missing role %s: %#v", tc.path, tc.role, records)
		}
		record := records[idx]
		if record.Identity != tc.identity || !reflect.DeepEqual(record.Navigation, tc.navigation) {
			t.Errorf("%s %s = identity %q navigation %#v", tc.path, tc.role, record.Identity, record.Navigation)
		}
		if record.Sources == nil || record.Outputs == nil || record.Navigation == nil {
			t.Errorf("%s %s has null collection: %#v", tc.path, tc.role, record)
		}
	}
	lockRoles := artifactRecords(".awf/awf.lock", decls, authorities)
	if got := []project.ArtifactRole{lockRoles[0].Role, lockRoles[1].Role}; !reflect.DeepEqual(got, []project.ArtifactRole{project.ArtifactLock, project.ArtifactManifest}) {
		t.Fatalf("lock role ordering = %v", got)
	}
	managed := artifactRecords("generated.md", decls, authorities)[0]
	if len(managed.Sources) != 7 || len(managed.Outputs) != 0 {
		t.Fatalf("managed causal edges = sources %#v outputs %#v", managed.Sources, managed.Outputs)
	}
	template := artifactRecords("templates/docs/out.tmpl", decls, authorities)[0]
	if !reflect.DeepEqual(template.Outputs, []artifactLink{{Path: "generated.md", Label: "managed output"}, {Path: "second.md", Label: "managed output"}}) {
		t.Fatalf("template outputs = %#v", template.Outputs)
	}
	inPlaceDecl := []project.OutputDeclaration{{Path: "awf", TemplateID: "runner/awf.tmpl", Declarers: []string{"runner"}, Inputs: []project.OutputInput{{Path: "awf", Role: project.ArtifactManagedOutput}}}}
	inPlace := artifactRecords("awf", inPlaceDecl, authorities)
	if len(inPlace) != 1 || inPlace[0].Role != project.ArtifactManagedOutput || !reflect.DeepEqual(inPlace[0].Sources, []artifactLink{{Path: "awf", Label: "in-place managed output"}}) || !reflect.DeepEqual(inPlace[0].Outputs, []artifactLink{{Path: "awf", Label: "managed output"}}) || inPlace[0].Navigation == nil {
		t.Fatalf("in-place source/output multiplicity = %#v", inPlace)
	}
	generatedIndex := artifactRecords("documentation/decisions/INDEX.md", decls, authorities)
	if len(generatedIndex) != 1 || generatedIndex[0].Identity != "generated-index" {
		t.Fatalf("template-free generated identity = %#v", generatedIndex)
	}
	withoutReference := artifactRecords(".awf/config.yaml", nil, authorities)
	if withoutReference[0].Navigation == nil || len(withoutReference[0].Navigation) != 0 {
		t.Fatalf("undeclared config-reference navigation = %#v", withoutReference)
	}
	duplicateRoles := artifactRecords("duplicate.md", []project.OutputDeclaration{{Path: "duplicate.md", TemplateID: "z"}, {Path: "duplicate.md", TemplateID: "a"}}, authorities)
	if len(duplicateRoles) != 2 || duplicateRoles[0].Identity != "a" || duplicateRoles[1].Identity != "z" {
		t.Fatalf("same-role identity ordering = %#v", duplicateRoles)
	}
	if got := artifactSourceLabel(project.ArtifactProtocolDescriptor); got != "protocol descriptor" {
		t.Fatalf("protocol source label = %q", got)
	}
	if got := artifactSourceLabel(project.ArtifactRole("future")); got != "future" {
		t.Fatalf("unknown source label = %q", got)
	}
	if got := mergeArtifactLinks([]artifactLink{{Path: "same", Label: "z"}}, []artifactLink{{Path: "same", Label: "a"}}); !reflect.DeepEqual(got, []artifactLink{{Path: "same", Label: "a"}, {Path: "same", Label: "z"}}) {
		t.Fatalf("same-path link ordering = %#v", got)
	}
	for _, path := range []string{"documentation/decisions/README.md", "documentation/decisions/0007-lookalike.md", "documentation/decisions/0008-malformed.md", "elsewhere/0007-real.md", "disabled.md", "local.md", ".awf/docs/local.yaml"} {
		if records := artifactRecords(path, decls, authorities); len(records) != 0 {
			t.Errorf("%s received disabled, reservation, or lookalike attribution: %#v", path, records)
		}
	}
}

// invariant: tooling/context-and-topic:context-known-artifact-navigation (TestContextArtifactFacetAloneRefinesGroupKey)
func TestContextArtifactFacetAloneRefinesGroupKey(t *testing.T) {
	base := contextPathImpact{
		Classification: pathCovered,
		Provenance:     []contextProvenance{{Role: "template", Identity: "shared", Sources: []artifactLink{{Path: "a", Label: "source"}}, Outputs: []artifactLink{{Path: "out-a", Label: "output"}}, Navigation: []artifactLink{{Path: "nav-a", Label: "navigation"}}}},
		Domains:        []domainRef{{Name: "d", CurrentState: "docs/domains/d.md"}},
		Topics:         []contextPathTopic{{ID: "d/t"}},
		Relationships:  contextRelationships{State: []string{"d/t:a"}, Touches: []string{}, Proofs: []string{}},
		Warnings:       []contextWarning{},
	}
	other := base
	other.Provenance = []contextProvenance{{Role: "template", Identity: "shared", Sources: []artifactLink{{Path: "b", Label: "source"}}, Outputs: []artifactLink{{Path: "out-b", Label: "output"}}, Navigation: []artifactLink{{Path: "nav-b", Label: "navigation"}}}}
	other.Relationships = contextRelationships{State: []string{"d/t:b"}, Touches: []string{}, Proofs: []string{}}
	for _, facets := range [][]ContextFacet{nil, {FacetRelationships}, {FacetInvariants}, {FacetAllRules}, {FacetEvidence}, {FacetReferences}} {
		if contextGroupKey(base, facets) != contextGroupKey(other, facets) {
			t.Fatalf("authority facets refined artifact grouping: %v", facets)
		}
	}
	if contextGroupKey(base, []ContextFacet{FacetArtifacts}) == contextGroupKey(other, []ContextFacet{FacetArtifacts}) {
		t.Fatal("artifacts facet did not refine detailed provenance")
	}
}
