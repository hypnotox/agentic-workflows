package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestAdvisoryCompatibilityAndReportErrorPaths(t *testing.T) {
	if got := advisoryCompatibilityFiles(&OutputPlan{Nodes: []OutputNode{{Path: "declaration-only"}}}); len(got) != 0 {
		t.Fatalf("compatibility files = %#v", got)
	}
	failure := errors.New("advisory failure")
	if _, err := finishCheckReport(nil, nil, nil, nil, &OutputPlan{}, failure); !errors.Is(err, failure) {
		t.Fatalf("finish error = %v", err)
	}
}

func TestAdvisoryNotesRejectMalformedRetainedData(t *testing.T) {
	t.Run("pitfalls", func(t *testing.T) {
		root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\ntags:\n  narrow: Narrow.\n", map[string]string{
			"docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: value\n---\nbody\n",
		})
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.AdvisoryNotes(testContext(t)); err == nil || !strings.Contains(err.Error(), "unknown") {
			t.Fatalf("advisory pitfall error = %v", err)
		}
	})
	t.Run("glossary", func(t *testing.T) {
		root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n", map[string]string{
			"docs/glossary.yaml": "data:\n  terms: not-a-list\n",
		})
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		corpus, err := adr.NewCorpus(nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.advisoryNotesWithState(corpus, pitfall.Corpus{}, nil, &OutputPlan{}); err == nil || !strings.Contains(err.Error(), "must be a list") {
			t.Fatalf("advisory glossary error = %v", err)
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
			if _, err := p.OutputPlan(testContext(t)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("output-plan error = %v", err)
			}
		})
	}
}

func TestCheckWithStatePropagatesMalformedRetainedData(t *testing.T) {
	for _, tc := range []struct {
		name, path, sidecar string
	}{
		{"glossary", "docs/glossary.yaml", "data:\n  terms: not-a-list\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Sync(); err != nil {
				t.Fatal(err)
			}
			corpus, pitfalls, topics, effective, err := p.deriveOperationStateWithPitfalls()
			if err != nil {
				t.Fatal(err)
			}
			op, err := p.outputPlanWithPitfalls(testContext(t), corpus, pitfalls, topics, effective)
			if err != nil {
				t.Fatal(err)
			}
			sidecarPath := filepath.Join(root, ".awf", tc.path)
			if err := os.MkdirAll(filepath.Dir(sidecarPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sidecarPath, []byte(tc.sidecar), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := p.checkWithTrackingState(testContext(t), corpus, pitfall.Corpus{}, topics, effective, mustParsePlans(t, p), op); err == nil || !strings.Contains(err.Error(), "must be a list") {
				t.Fatalf("check-with-state error = %v", err)
			}
		})
	}
}

func TestCheckReportPropagatesAdvisorySidecarError(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	reader := &flippingGlossaryReader{validReads: 5}
	cfg, err := config.ParseTree(".awf", []byte("prefix: example\nprofile: full\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n"), reader)
	if err != nil {
		t.Fatal(err)
	}
	p.Cfg = cfg
	if _, err := p.CheckReport(testContext(t)); err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Fatalf("CheckReport advisory error = %v after %d glossary reads", err, reader.reads)
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
	for _, kind := range []string{"", "skill", "target"} {
		doc, err := p.ListDocument(kind)
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
	p := &Project{Cfg: &config.Config{}, cat: catalog.NewView(&catalog.Catalog{Skills: map[string]catalog.SkillSpec{"bad\nentry": {}}}).Catalog()}
	if _, err := p.ListDocument("skill"); err == nil {
		t.Fatal("invalid catalog entry reached the list presentation")
	}
}

func TestBuildOutputDeclarationsCoversTargetAndMetadataEdges(t *testing.T) {
	cfg, err := config.ParseTree(".awf", []byte("prefix: example\ndomains: [d]\n"), configReaderAdapter{memoryProjectReader{}})
	if err != nil {
		t.Fatal(err)
	}
	targets := []Target{{Name: "one", Outputs: []TargetOutput{{Path: "shared", TemplateID: "template", Inputs: []TargetOutputInput{{Path: "same", Role: ArtifactConfig}, {Path: "same", Role: ArtifactTemplate}}}}}, {Name: "two", Outputs: []TargetOutput{{Path: "shared", TemplateID: "template"}}}}
	read := memoryProjectReader{".awf/topics/metadata/note.txt": []byte("ignored")}
	decls, err := BuildOutputDeclarations(cfg, &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}, targets, read, mustCorpus())
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
	if _, err := BuildOutputDeclarations(cfg, &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}, unknown, read, mustCorpus()); err == nil || !strings.Contains(err.Error(), "unknown catalog skill") {
		t.Fatalf("unknown target requirement error = %v", err)
	}

	badSidecar := memoryProjectReader{".awf/domains/d.yaml": []byte("data: [\n")}
	cfg, err = config.ParseTree(".awf", []byte("prefix: example\ndomains: [d]\n"), configReaderAdapter{badSidecar})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildOutputDeclarations(cfg, &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}, nil, badSidecar, mustCorpus()); err == nil || !strings.Contains(err.Error(), "domains/d.yaml") {
		t.Fatalf("domain sidecar error = %v", err)
	}
}

func TestBuildOutputDeclarationsPropagatesCatalogSidecarReadFault(t *testing.T) {
	read := failingReadReader{memoryProjectReader: memoryProjectReader{}}
	cfg, err := config.ParseTree(".awf", []byte("prefix: example\n"), configReaderAdapter(read))
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{"tdd": {}}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}
	if _, err := BuildOutputDeclarations(cfg, cat, []Target{{Name: "test"}}, read, mustCorpus()); err == nil || !strings.Contains(err.Error(), "read fault") {
		t.Fatalf("declaration read error = %v", err)
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
			if err := p.Sync(); err != nil {
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
