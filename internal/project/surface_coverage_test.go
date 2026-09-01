package project

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

func TestAdvisoryNotesRejectMalformedRetainedData(t *testing.T) {
	t.Run("pitfalls", func(t *testing.T) {
		root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
			"docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: value\n---\nbody\n",
		})
		p, err := loadTestSession(testContext(t), root)
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
			root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{tc.path: tc.sidecar})
			p, err := loadTestSession(testContext(t), root)
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
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	reader := &flippingGlossaryReader{validReads: 2}
	cfg, err := config.ParseTree(".awf", []byte("prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n"), reader)
	if err != nil {
		t.Fatal(err)
	}
	operation := testPublisher(operationInputs(p, testConfig(p)))
	plan, planErr := operation.Plan()
	if planErr != nil {
		t.Fatal(planErr)
	}
	pitfalls, _ := operation.Pitfalls()
	skills, _ := operation.EffectiveSkills()
	generated, _ := operation.GeneratedOutput()
	glossary, _ := operation.Glossary()
	if _, err := BuildCheckReport(p, cfg, testRepo(p), testContext(t), plan, pitfalls, skills, generated, glossary); err != nil {
		t.Fatalf("project report changed after operation construction: %v after %d glossary reads", err, reader.reads)
	}
}

func TestListDocumentRetainedInventory(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\ndomains: [rendering]\n", map[string]string{
		"skills/debugging.yaml": "data:\n  testSurfaces: []\n",
	})
	p, err := loadTestSession(testContext(t), root)
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
