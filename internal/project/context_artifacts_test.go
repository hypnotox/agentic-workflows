package project

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// invariant: rendering/sync-and-drift:managed-output-attribution
func TestArtifactRecordsFollowDeclarations(t *testing.T) {
	decls := []OutputDeclaration{{Path: "docs/out.md", TemplateID: "docs/out.md.tmpl", Declarers: []string{"out"}, Inputs: []OutputInput{{Path: ".awf/docs/parts/out/content.md", Role: ArtifactConventionPart}}}}
	generated := artifactRecords("docs/out.md", decls, testArtifactAuthorities("docs", adr.NewCorpus(nil)))
	if len(generated) != 1 || generated[0].Role != ArtifactManagedOutput || len(generated[0].Sources) != 1 {
		t.Fatalf("generated=%#v", generated)
	}
	source := artifactRecords(".awf/docs/parts/out/content.md", decls, testArtifactAuthorities("docs", adr.NewCorpus(nil)))
	if len(source) != 1 || source[0].Role != ArtifactConventionPart || len(source[0].Outputs) != 1 || source[0].Outputs[0].Path != "docs/out.md" {
		t.Fatalf("source=%#v", source)
	}
	unmanaged := artifactRecords("docs/lookalike.md", decls, testArtifactAuthorities("docs", adr.NewCorpus(nil)))
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

// invariant: tooling/context-and-topic:context-known-artifact-navigation
func TestArtifactNavigationCoversClosedRolesOrderingAndLookalikes(t *testing.T) {
	parsed := adr.NewCorpus([]adr.ADR{{Number: "0007", Filename: "0007-real.md"}})
	decls := []OutputDeclaration{
		{Path: "documentation/config-reference.md", TemplateID: "docs/config-reference.md.tmpl", Declarers: []string{"config-reference"}},
		{Path: "documentation/domains/d.md", TemplateID: "domains/domain.md.tmpl", Declarers: []string{"generated-domain"}},
		{Path: "documentation/topics/d/t.md", TemplateID: "topics/topic.md.tmpl", Declarers: []string{"topic:d/t"}},
		{Path: "documentation/decisions/INDEX.md", Declarers: []string{"generated-index"}},
		{Path: "generated.md", TemplateID: "docs/out.tmpl", Declarers: []string{"owner"}, Inputs: []OutputInput{
			{Path: ".awf/config.yaml", Role: ArtifactConfig},
			{Path: "templates/docs/out.tmpl", Role: ArtifactTemplate},
			{Path: ".awf/docs/parts/out/content.md", Role: ArtifactConventionPart},
			{Path: ".awf/docs/out.yaml", Role: ArtifactAuthoredData},
			{Path: ".awf/topics/metadata/d/t.yaml", Role: ArtifactTopicMetadata},
			{Path: ".awf/topics/parts/d/t/current-state.md", Role: ArtifactClaimPart},
			{Path: "documentation/decisions/0007-real.md", Role: ArtifactDecisionRecord},
		}},
		{Path: "second.md", TemplateID: "docs/second.tmpl", Declarers: []string{"second"}, Inputs: []OutputInput{{Path: "templates/docs/out.tmpl", Role: ArtifactTemplate}}},
		{Path: "local.md", TemplateID: "local", Declarers: []string{"local"}, Inputs: []OutputInput{{Path: ".awf/docs/local.yaml", Role: ArtifactAuthoredData}}, Reservation: true},
	}
	authorities := testArtifactAuthorities("documentation", parsed)
	cases := []struct {
		path       string
		role       ArtifactRole
		identity   string
		navigation []ArtifactLink
	}{
		{".awf/config.yaml", ArtifactConfig, "project-config", []ArtifactLink{{Path: "documentation/config-reference.md", Label: "configuration reference"}}},
		{".awf/awf.lock", ArtifactLock, "project-lock", []ArtifactLink{{Path: ".awf/config.yaml", Label: "project config"}, {Path: "documentation/config-reference.md", Label: "configuration reference"}}},
		{".awf/awf.lock", ArtifactManifest, "output-manifest", []ArtifactLink{{Path: "documentation/config-reference.md", Label: "configuration reference"}}},
		{"templates/docs/out.tmpl", ArtifactTemplate, "docs/out.tmpl", []ArtifactLink{{Path: "generated.md", Label: "managed output"}, {Path: "second.md", Label: "managed output"}}},
		{".awf/docs/parts/out/content.md", ArtifactConventionPart, ".awf/docs/parts/out/content.md", []ArtifactLink{{Path: "generated.md", Label: "managed output"}}},
		{".awf/docs/out.yaml", ArtifactAuthoredData, ".awf/docs/out.yaml", []ArtifactLink{{Path: "generated.md", Label: "managed output"}}},
		{".awf/topics/metadata/d/t.yaml", ArtifactTopicMetadata, "d/t", []ArtifactLink{{Path: "documentation/domains/d.md", Label: "domain document"}, {Path: "documentation/topics/d/t.md", Label: "topic document"}}},
		{".awf/topics/parts/d/t/current-state.md", ArtifactClaimPart, "d/t", []ArtifactLink{{Path: "documentation/domains/d.md", Label: "domain document"}, {Path: "documentation/topics/d/t.md", Label: "topic document"}}},
		{"documentation/decisions/0007-real.md", ArtifactDecisionRecord, "0007", []ArtifactLink{{Path: "documentation/decisions/INDEX.md", Label: "decision index"}}},
		{"generated.md", ArtifactManagedOutput, "docs/out.tmpl", []ArtifactLink{{Path: ".awf/config.yaml", Label: "project config"}, {Path: ".awf/docs/out.yaml", Label: "authored data"}, {Path: ".awf/docs/parts/out/content.md", Label: "convention part"}, {Path: ".awf/topics/metadata/d/t.yaml", Label: "topic metadata"}, {Path: ".awf/topics/parts/d/t/current-state.md", Label: "claim part"}, {Path: "documentation/decisions/0007-real.md", Label: "decision record"}}},
	}
	for _, tc := range cases {
		records := artifactRecords(tc.path, decls, authorities)
		idx := slices.IndexFunc(records, func(record ArtifactRecord) bool { return record.Role == tc.role })
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
	if got := []ArtifactRole{lockRoles[0].Role, lockRoles[1].Role}; !reflect.DeepEqual(got, []ArtifactRole{ArtifactLock, ArtifactManifest}) {
		t.Fatalf("lock role ordering = %v", got)
	}
	managed := artifactRecords("generated.md", decls, authorities)[0]
	if len(managed.Sources) != 7 || len(managed.Outputs) != 0 {
		t.Fatalf("managed causal edges = sources %#v outputs %#v", managed.Sources, managed.Outputs)
	}
	template := artifactRecords("templates/docs/out.tmpl", decls, authorities)[0]
	if !reflect.DeepEqual(template.Outputs, []ArtifactLink{{Path: "generated.md", Label: "managed output"}, {Path: "second.md", Label: "managed output"}}) {
		t.Fatalf("template outputs = %#v", template.Outputs)
	}
	inPlaceDecl := []OutputDeclaration{{Path: "awf", TemplateID: "runner/awf.tmpl", Declarers: []string{"runner"}, Inputs: []OutputInput{{Path: "awf", Role: ArtifactManagedOutput}}}}
	inPlace := artifactRecords("awf", inPlaceDecl, authorities)
	if len(inPlace) != 1 || inPlace[0].Role != ArtifactManagedOutput || !reflect.DeepEqual(inPlace[0].Sources, []ArtifactLink{{Path: "awf", Label: "in-place managed output"}}) || !reflect.DeepEqual(inPlace[0].Outputs, []ArtifactLink{{Path: "awf", Label: "managed output"}}) || inPlace[0].Navigation == nil {
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
	duplicateRoles := artifactRecords("duplicate.md", []OutputDeclaration{{Path: "duplicate.md", TemplateID: "z"}, {Path: "duplicate.md", TemplateID: "a"}}, authorities)
	if len(duplicateRoles) != 2 || duplicateRoles[0].Identity != "a" || duplicateRoles[1].Identity != "z" {
		t.Fatalf("same-role identity ordering = %#v", duplicateRoles)
	}
	if got := artifactSourceLabel(ArtifactProtocolDescriptor); got != "protocol descriptor" {
		t.Fatalf("protocol source label = %q", got)
	}
	if got := artifactSourceLabel(ArtifactRole("future")); got != "future" {
		t.Fatalf("unknown source label = %q", got)
	}
	if got := mergeArtifactLinks([]ArtifactLink{{Path: "same", Label: "z"}}, []ArtifactLink{{Path: "same", Label: "a"}}); !reflect.DeepEqual(got, []ArtifactLink{{Path: "same", Label: "a"}, {Path: "same", Label: "z"}}) {
		t.Fatalf("same-path link ordering = %#v", got)
	}
	for _, path := range []string{"documentation/decisions/README.md", "documentation/decisions/0007-lookalike.md", "documentation/decisions/0008-malformed.md", "elsewhere/0007-real.md", "disabled.md", "local.md", ".awf/docs/local.yaml"} {
		if records := artifactRecords(path, decls, authorities); len(records) != 0 {
			t.Errorf("%s received disabled, reservation, or lookalike attribution: %#v", path, records)
		}
	}
}

// invariant: tooling/context-and-topic:context-known-artifact-navigation
func TestContextArtifactFacetAloneRefinesGroupKey(t *testing.T) {
	base := ContextPathImpact{
		Classification: PathCovered,
		Provenance:     []ContextProvenance{{Role: "template", Identity: "shared", Sources: []ArtifactLink{{Path: "a", Label: "source"}}, Outputs: []ArtifactLink{{Path: "out-a", Label: "output"}}, Navigation: []ArtifactLink{{Path: "nav-a", Label: "navigation"}}}},
		Domains:        []DomainRef{{Name: "d", CurrentState: "docs/domains/d.md"}},
		Topics:         []ContextPathTopic{{ID: "d/t"}},
		Relationships:  ContextRelationships{State: []string{"d/t:a"}, Touches: []string{}, Proofs: []string{}},
		Warnings:       []ContextWarning{},
	}
	other := base
	other.Provenance = []ContextProvenance{{Role: "template", Identity: "shared", Sources: []ArtifactLink{{Path: "b", Label: "source"}}, Outputs: []ArtifactLink{{Path: "out-b", Label: "output"}}, Navigation: []ArtifactLink{{Path: "nav-b", Label: "navigation"}}}}
	other.Relationships = ContextRelationships{State: []string{"d/t:b"}, Touches: []string{}, Proofs: []string{}}
	for _, facets := range [][]ContextFacet{nil, {FacetRelationships}, {FacetInvariants}, {FacetAllRules}, {FacetEvidence}, {FacetReferences}} {
		if contextGroupKey(base, facets) != contextGroupKey(other, facets) {
			t.Fatalf("authority facets refined artifact grouping: %v", facets)
		}
	}
	if contextGroupKey(base, []ContextFacet{FacetArtifacts}) == contextGroupKey(other, []ContextFacet{FacetArtifacts}) {
		t.Fatal("artifacts facet did not refine detailed provenance")
	}
}

func TestProjectTreeReaders(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: "a.txt", Mode: snapshot.Regular, Bytes: []byte("a")}, {Path: "link", Mode: snapshot.Symlink, Bytes: []byte("a.txt")}})
	if err != nil {
		t.Fatal(err)
	}
	r := snapshotTreeReader{tree: tree}
	b, ok := r.ReadFile("a.txt")
	if !ok || string(b) != "a" {
		t.Fatal("snapshot read")
	}
	b[0] = 'X'
	again, _ := r.ReadFile("a.txt")
	if string(again) != "a" {
		t.Fatal("snapshot alias")
	}
	if _, ok := r.ReadFile("link"); ok {
		t.Fatal("scanned symlink")
	}
	if got, err := r.Paths(""); err != nil || !reflect.DeepEqual(got, []string{"a.txt"}) {
		t.Fatalf("paths=%v err=%v", got, err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	fr := filesystemProjectReader{root: root}
	if b, ok := fr.ReadFile("a.txt"); !ok || string(b) != "a" {
		t.Fatal("filesystem read")
	}
	if _, ok := fr.ReadFile("missing"); ok {
		t.Fatal("missing read")
	}
	if got, err := fr.Paths(""); err != nil || !reflect.DeepEqual(got, []string{"a.txt"}) {
		t.Fatalf("filesystem paths=%v err=%v", got, err)
	}
	// An absent prefix stays an empty enumeration rather than a fault.
	if got, err := fr.Paths("missing"); err != nil || len(got) != 0 {
		t.Fatalf("missing paths=%v err=%v", got, err)
	}
	// A directory that cannot be read is a fault, not a short list: erasing it
	// would compute the drift oracle over a silently truncated tree.
	denied := filepath.Join(root, "denied")
	if err := os.Mkdir(denied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(denied, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o755) })
	if got, err := fr.Paths(""); err == nil {
		t.Fatalf("unreadable directory erased: paths=%v", got)
	} else if !strings.Contains(err.Error(), "enumerate project tree") {
		t.Fatalf("whole-tree fault names an empty subject: %v", err)
	}
	// A named prefix keeps its own subject rather than the whole-tree wording.
	if got, err := fr.Paths("denied"); err == nil {
		t.Fatalf("unreadable prefix erased: paths=%v", got)
	} else if !strings.Contains(err.Error(), "enumerate denied") {
		t.Fatalf("prefixed fault lost its subject: %v", err)
	}
}

// TestBuildOutputDeclarationsPropagatesEnumerationFaults pins that a faulting
// tree read reaches the caller. Erasing it truncated the declaration set the
// drift oracle is computed over, so a partial enumeration reported a clean tree
// and exited 0.
func TestBuildOutputDeclarationsPropagatesEnumerationFaults(t *testing.T) {
	read := memoryProjectReader{".awf/topics/metadata/d/t.yaml": []byte("x")}
	cfg, err := config.ParseTree(".awf", []byte("prefix: p\ndocsDir: docs\nskills: []\nagents: []\ndomains: [d]\n"), configReaderAdapter{read})
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}
	// Three sites enumerate the tree, in call order: per-domain metadata for the
	// domain docs, the flat metadata list for topic docs, and per-domain metadata
	// for the topic indexes. Each must surface its own fault.
	for site := 1; site <= 3; site++ {
		t.Run("site"+strconv.Itoa(site), func(t *testing.T) {
			calls := 0
			faulting := failingPathsReader{memoryProjectReader: read, failAt: site, calls: &calls}
			if _, err := BuildOutputDeclarations(cfg, cat, nil, faulting, adr.NewCorpus(nil)); err == nil || !strings.Contains(err.Error(), "enumeration fault") {
				t.Fatalf("site %d: error = %v, want the enumeration fault", site, err)
			}
		})
	}
}

func TestBuildOutputDeclarationsFamiliesAndReservations(t *testing.T) {
	read := memoryProjectReader{".awf/topics/metadata/d/t.yaml": []byte("x"), ".awf/topics/metadata/d/readme.txt": []byte("x"), ".awf/skills/local.yaml": []byte("local: true\n"), ".awf/skills/parts/local/content.md": []byte("part"), ".awf/agents/agent.yaml": []byte("local: true\n"), ".awf/agents/parts/agent/content.md": []byte("part"), "docs/decisions/0001-real.md": []byte("parsed"), "docs/decisions/0002-malformed.md": []byte("not parsed"), "docs/decisions/INDEX.md": []byte("generated"), "docs/decisions/README.md": []byte("navigation")}
	cfg, err := config.ParseTree(".awf", []byte("prefix: p\ndocsDir: docs\nskills: [local]\nagents: [agent]\ndocs: [enabled]\ndomains: [d]\nrunner: {enabled: true}\nbootstrap: {enabled: true}\nhooks: {enabled: true}\n"), configReaderAdapter{read})
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{"local": {Base: true, Sections: []string{"content"}}}, Agents: map[string]catalog.AgentSpec{"agent": {Base: true, Sections: []string{"content"}}}, Docs: map[string]catalog.DocEntry{"agents-doc": {Mandatory: true, AgentsDoc: true, TID: "agents-doc/AGENTS.md.tmpl"}, "architecture": {Mandatory: true, Path: "architecture.md", TID: "docs/architecture.md.tmpl"}, "disabled": {Path: "disabled.md", TID: "docs/disabled.md.tmpl"}, "enabled": {Path: "enabled.md", TID: "docs/enabled.md.tmpl"}}}
	target := Target{Name: "one", SkillDir: ".one/skills", Outputs: []TargetOutput{{Path: "shared", TemplateID: "target.tmpl", Producer: TargetOutputTemplate}}}
	other := target
	other.Name = "two"
	parsedADRs := adr.NewCorpus([]adr.ADR{{Number: "0001", Filename: "0001-real.md"}})
	decls, err := BuildOutputDeclarations(cfg, cat, []Target{target, other}, read, parsedADRs)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]OutputDeclaration{}
	for _, d := range decls {
		byPath[d.Path] = d
	}
	for _, p := range []string{".one/skills/p-local/SKILL.md", "shared", "AGENTS.md", "docs/domains/d.md", "docs/topics/d/t.md", "docs/decisions/INDEX.md", "awf", ".awf/bootstrap.sh", ".awf/upgrade.sh", ".awf/hooks/pre-commit.sh", ".awf/efforts/.gitignore", ".awf/worktrees/.gitignore"} {
		if _, ok := byPath[p]; !ok {
			t.Errorf("missing %s", p)
		}
	}
	for _, tc := range []struct {
		read memoryProjectReader
		body string
	}{{memoryProjectReader{".awf/agents-doc.yaml": []byte("local: [bad")}, "prefix: p\n"}, {memoryProjectReader{".awf/docs/enabled.yaml": []byte("local: [bad")}, "prefix: p\ndocs: [enabled]\n"}} {
		badCfg, _ := config.ParseTree(".awf", []byte(tc.body), configReaderAdapter{tc.read})
		if _, err := BuildOutputDeclarations(badCfg, cat, []Target{target}, tc.read, adr.NewCorpus(nil)); err == nil {
			t.Fatal("malformed document declaration accepted")
		}
	}
	badSkillRead := memoryProjectReader{".awf/skills/bad.yaml": []byte("local: [bad")}
	badSkillCfg, _ := config.ParseTree(".awf", []byte("prefix: p\nskills: [bad]\n"), configReaderAdapter{badSkillRead})
	if _, err := BuildOutputDeclarations(badSkillCfg, cat, []Target{target}, badSkillRead, adr.NewCorpus(nil)); err == nil {
		t.Fatal("malformed skill declaration accepted")
	}
	badRead := memoryProjectReader{".awf/agents/bad.yaml": []byte("local: [bad")}
	badCfg, _ := config.ParseTree(".awf", []byte("prefix: p\nagents: [bad]\n"), configReaderAdapter{badRead})
	if _, err := BuildOutputDeclarations(badCfg, cat, []Target{target}, badRead, adr.NewCorpus(nil)); err == nil {
		t.Fatal("malformed agent declaration accepted")
	}
	badDomainRead := memoryProjectReader{".awf/domains/d.yaml": []byte("paths: [bad")}
	badDomainCfg, _ := config.ParseTree(".awf", []byte("prefix: p\ndomains: [d]\n"), configReaderAdapter{badDomainRead})
	if _, err := BuildOutputDeclarations(badDomainCfg, cat, []Target{target}, badDomainRead, adr.NewCorpus(nil)); err == nil {
		t.Fatal("malformed domain declaration accepted")
	}
	if !byPath[".one/skills/p-local/SKILL.md"].Reservation || !reflect.DeepEqual(byPath["shared"].Declarers, []string{"one", "two"}) {
		t.Fatalf("declarations=%#v", decls)
	}
	index := byPath["docs/decisions/INDEX.md"]
	decisionInputs := []string{}
	for _, input := range index.Inputs {
		if input.Role == ArtifactDecisionRecord {
			decisionInputs = append(decisionInputs, input.Path)
		}
	}
	if !reflect.DeepEqual(decisionInputs, []string{"docs/decisions/0001-real.md"}) {
		t.Fatalf("decision inputs include unparsed lookalikes: %v", decisionInputs)
	}
}

func TestOutputPlanObservesConsumedInputsIndependently(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\n"+debuggingVars+"skills: [debugging, exploring]\nagents: [explorer]\n", map[string]string{
		"skills/debugging.yaml":                        "data: {}\n",
		"skills/parts/debugging/debugging-surfaces.md": "Observed part.\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := p.OutputPlan(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	path := p.Targets[0].SkillPath(p.Cfg.Prefix, "debugging")
	idx := slices.IndexFunc(plan.Nodes, func(node OutputNode) bool { return node.Path == path })
	if idx < 0 {
		t.Fatalf("missing node %s", path)
	}
	want := normalizeOutputInputs([]OutputInput{
		{Path: ".awf/config.yaml", Role: ArtifactConfig},
		{Path: ".awf/skills/debugging.yaml", Role: ArtifactAuthoredData},
		{Path: ".awf/skills/parts/debugging/debugging-surfaces.md", Role: ArtifactConventionPart},
		{Path: "templates/skills/debugging/SKILL.md.tmpl", Role: ArtifactTemplate},
	})
	if !reflect.DeepEqual(plan.Nodes[idx].ConsumedInputs, want) {
		t.Fatalf("consumed inputs = %#v, want %#v", plan.Nodes[idx].ConsumedInputs, want)
	}
}

func TestDeclarationPlanParityDiagnostics(t *testing.T) {
	input := OutputInput{Path: ".awf/config.yaml", Role: ArtifactConfig}
	node := OutputNode{Path: "a", Recipe: OutputRecipe{TemplateID: "a.tmpl"}, ObservedTemplateID: "a.tmpl", Declarers: []string{"owner"}, ConsumedInputs: []OutputInput{input}}
	declaration := OutputDeclaration{Path: "a", TemplateID: "a.tmpl", Declarers: []string{"owner"}, Inputs: []OutputInput{input}}
	if err := validateDeclarationPlanParity([]OutputNode{node}, []OutputDeclaration{declaration}); err != nil {
		t.Fatalf("matching parity rejected: %v", err)
	}
	if err := validateDeclarationPlanParity([]OutputNode{{Path: "a"}}, nil); err == nil {
		t.Fatal("length mismatch accepted")
	}
	if err := validateDeclarationPlanParity(nil, []OutputDeclaration{{Path: "a"}}); err == nil {
		t.Fatal("reverse length mismatch accepted")
	}
	mutations := []struct {
		name string
		edit func(*OutputNode)
	}{
		{"path", func(n *OutputNode) { n.Path = "b" }},
		{"reservation", func(n *OutputNode) { n.Reservation = true }},
		{"template", func(n *OutputNode) { n.ObservedTemplateID = "other.tmpl" }},
		{"declarers", func(n *OutputNode) { n.Declarers = []string{"other"} }},
		{"inputs", func(n *OutputNode) { n.ConsumedInputs = []OutputInput{{Path: "other", Role: ArtifactAuthoredData}} }},
		{"dependencies", func(n *OutputNode) { n.DependsOn = []string{"other"} }},
	}
	for _, mutation := range mutations {
		t.Run("node-"+mutation.name, func(t *testing.T) {
			changed := node
			mutation.edit(&changed)
			if err := validateDeclarationPlanParity([]OutputNode{changed}, []OutputDeclaration{declaration}); err == nil {
				t.Fatalf("%s mismatch accepted", mutation.name)
			}
		})
	}
	for _, mutation := range []struct {
		name   string
		inputs []OutputInput
	}{
		{"missing-declaration-input", []OutputInput{}},
		{"extra-declaration-input", []OutputInput{input, {Path: "extra", Role: ArtifactAuthoredData}}},
		{"role-misclassified-declaration-input", []OutputInput{{Path: input.Path, Role: ArtifactAuthoredData}}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changed := declaration
			changed.Inputs = mutation.inputs
			if err := validateDeclarationPlanParity([]OutputNode{node}, []OutputDeclaration{changed}); err == nil {
				t.Fatal("declaration input mutation accepted")
			}
		})
	}
	rolesAtOnePath := normalizeOutputInputs([]OutputInput{{Path: "same", Role: ArtifactTemplate}, {Path: "same", Role: ArtifactConfig}})
	if !reflect.DeepEqual(rolesAtOnePath, []OutputInput{{Path: "same", Role: ArtifactConfig}, {Path: "same", Role: ArtifactTemplate}}) {
		t.Fatalf("same-path role ordering = %#v", rolesAtOnePath)
	}
	if got := difference([]string{"a", "b"}, []string{"b"}); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("difference=%v", got)
	}
}

type memoryProjectReader map[string][]byte

func (r memoryProjectReader) ReadFile(p string) ([]byte, bool) {
	b, ok := r[p]
	return append([]byte(nil), b...), ok
}
func (r memoryProjectReader) Paths(prefix string) ([]string, error) {
	out := []string{}
	for p := range r {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out, nil
}

// failingPathsReader faults the failAt'th Paths call so each propagation site
// can be exercised on its own.
type failingPathsReader struct {
	memoryProjectReader
	failAt int
	calls  *int
}

func (r failingPathsReader) Paths(prefix string) ([]string, error) {
	*r.calls++
	if *r.calls == r.failAt {
		return nil, errors.New("enumeration fault at " + prefix)
	}
	return r.memoryProjectReader.Paths(prefix)
}

type configReaderAdapter struct{ memoryProjectReader }

func (r configReaderAdapter) ReadFile(p string) ([]byte, bool) {
	return r.memoryProjectReader.ReadFile(".awf/" + p)
}
func (r configReaderAdapter) Paths(prefix string) []string { return nil }

func testArtifactAuthorities(docsDir string, corpus adr.Corpus) artifactAuthorities {
	return artifactAuthorities{Layout: Layout{DocsDir: docsDir, ADRDir: docsDir + "/decisions", IndexMd: docsDir + "/decisions/INDEX.md", DomainsDir: docsDir + "/domains"}, ADRs: corpus}
}
