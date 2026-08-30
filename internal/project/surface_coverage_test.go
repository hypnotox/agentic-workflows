package project

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestAdvisoryCompatibilityAndReportErrorPaths(t *testing.T) {
	if got := advisoryCompatibilityFiles(func() *OutputPlan {
		plan := outputplan.New([]outputplan.Node{outputplan.NewNode(outputplan.NodeSpec{Path: "declaration-only"})})
		return &plan
	}()); len(got) != 0 {
		t.Fatalf("compatibility files = %#v", got)
	}
	if _, err := checkresult.New([]checkresult.Finding{{Rank: severity.Error, Property: propertyCorrectness, Evidence: checkresult.Evidence{Kind: "broken", Path: "path"}}}, nil); err == nil {
		t.Fatal("report finalizer accepted invalid producer evidence")
	}
}

func TestAdvisoryNotesRejectMalformedRetainedData(t *testing.T) {
	t.Run("pitfalls", func(t *testing.T) {
		root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", map[string]string{
			"docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: value\n---\nbody\n",
		})
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := advisoryNotesProject(p); err == nil || !strings.Contains(err.Error(), "unknown") {
			t.Fatalf("advisory pitfall error = %v", err)
		}
	})
}

func TestOutputPlanRejectsMalformedRetainedData(t *testing.T) {
	for _, tc := range []struct {
		name, path, sidecar, want string
	}{
		{"pitfalls", "docs/pitfalls/bad.md", "---\ntitle: Bad\nunknown: value\n---\nbody\n", "unknown"},
		{"glossary", "docs/glossary.yaml", "data:\n  terms: not-a-list\n", "must be a list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", map[string]string{tc.path: tc.sidecar})
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := outputPlanProject(p); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("output-plan error = %v", err)
			}
		})
	}
}

func TestCheckReportUsesPreparedAdvisorySources(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	reader := &flippingGlossaryReader{validReads: 2}
	cfg, err := config.ParseTree(".awf", []byte("prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n"), reader)
	if err != nil {
		t.Fatal(err)
	}
	prepared, planErr := testPublisher(operationInputs(p, testConfig(p))).Prepare()
	if planErr != nil {
		t.Fatal(planErr)
	}
	semantics := OperationSemantics{ADRs: prepared.ADRs(), Pitfalls: prepared.Pitfalls(), Topics: prepared.Topics(), EffectiveSkills: prepared.EffectiveSkills(), GeneratedOutput: prepared.GeneratedOutput(), Glossary: prepared.Glossary()}
	if _, err := BuildCheckReport(p, cfg, testRepo(p), testContext(t), prepared.Plan(), semantics); err != nil {
		t.Fatalf("CheckReport changed after preparation: %v after %d glossary reads", err, reader.reads)
	}
}

func TestListDocumentRetainedInventory(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\ndomains: [rendering]\n", map[string]string{
		"skills/tdd.yaml": "data:\n  testSurfaces: []\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"", "skill", "target"} {
		doc, err := BuildListDocument(p, cfg, kind)
		if err != nil {
			t.Fatalf("ListDocument(%q): %v", kind, err)
		}
		var rendered bytes.Buffer
		if err := presentation.Render(&rendered, doc); err != nil {
			t.Fatalf("render ListDocument(%q): %v", kind, err)
		}
		if rendered.Len() == 0 {
			t.Fatalf("ListDocument(%q) has no inventory output", kind)
		}
	}
}

func TestListDocumentPropagatesInvalidCatalogEntry(t *testing.T) {
	cat := catalog.NewView(&catalog.Catalog{Skills: map[string]catalog.SkillSpec{"bad\nentry": {}}}).Catalog()
	if _, err := listDocument(&config.Config{}, cat, "skill"); err == nil {
		t.Fatal("invalid catalog entry reached the list presentation")
	}
}

func TestCheckStagedDriftRejectsInvalidStagedSidecars(t *testing.T) {
	for _, tc := range []struct {
		name, path, contents string
	}{
		{"catalog validation", ".awf/skills/tdd.yaml", "data: [\n"},
		{"output planning", ".awf/docs/glossary.yaml", "data:\n  terms: not-a-list\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
			repo := gitfixture.InitRepoAt(t, root)
			gitfixture.AddAll(t, repo)
			gitfixture.Commit(t, repo, "config", nil)
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := syncProject(p); err != nil {
				t.Fatal(err)
			}
			gitfixture.AddAll(t, repo)
			gitfixture.Commit(t, repo, "render", nil)
			gitfixture.Stage(t, repo, map[string]string{tc.path: tc.contents})
			if _, err := CheckStagedDriftRoot(testContext(t), root); err == nil {
				t.Fatal("staged malformed sidecar was accepted")
			}
		})
	}
}

type flippingGlossaryReader struct {
	validReads int
	reads      int
}

func (r *flippingGlossaryReader) ReadFile(path string) ([]byte, bool) {
	if path != "docs/glossary.yaml" {
		return nil, false
	}
	r.reads++
	if r.reads > r.validReads {
		return []byte("data:\n  terms: not-a-list\n"), true
	}
	return []byte("data:\n  terms: []\n"), true
}

func (r *flippingGlossaryReader) Paths(string) []string { return nil }

func TestCheckStagedDriftRootRejectsNonRepository(t *testing.T) {
	if _, err := CheckStagedDriftRoot(testContext(t), t.TempDir()); err == nil {
		t.Fatal("staged drift accepted a non-repository root")
	}
}
