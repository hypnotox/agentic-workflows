package publisher

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestPitfallDogfoodSourceOutputParity(t *testing.T) {
	root := filepath.Clean("../..")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := loadPitfallCorpus(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	var source []string
	for _, e := range corpus.All() {
		source = append(source, e.Slug)
	}
	slices.Sort(source)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "pitfalls", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	output := make([]string, 0, len(matches))
	for _, match := range matches {
		output = append(output, strings.TrimSuffix(filepath.Base(match), ".md"))
	}
	slices.Sort(output)
	indexBytes, err := os.ReadFile(filepath.Join(root, "docs", "pitfalls.md"))
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`\(pitfalls/([a-z0-9-]+)\.md\)`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(indexBytes), -1) {
		seen[m[1]] = true
	}
	tableRows := regexp.MustCompile(`(?m)^\| .*\(pitfalls/([a-z0-9-]+)\.md\)`).FindAllStringSubmatch(string(indexBytes), -1)
	rowCounts := map[string]int{}
	for _, row := range tableRows {
		rowCounts[row[1]]++
	}
	for _, slug := range source {
		if rowCounts[slug] != 1 {
			t.Fatalf("pitfall %s has %d metadata rows, want exactly one", slug, rowCounts[slug])
		}
	}
	index := make([]string, 0, len(seen))
	for slug := range seen {
		index = append(index, slug)
	}
	slices.Sort(index)
	if !slices.Equal(source, output) || !slices.Equal(source, index) {
		t.Fatalf("pitfall parity mismatch\nsource-only=%v\noutput-only=%v\nindex-only=%v", difference(source, output), difference(output, source), difference(source, index))
	}
}

// This named integration stack exercises the complete pitfall output family:
// corpus-to-index/leaf parity and navigation, source guidance, definitions and
// dependencies, hash isolation, lock/drift/backup/prune lifecycle, staged plan
// parity, and malformed-source refusal.
// invariant: rendering/doc-outputs:pitfall-output-complete (TestPitfallOutputCompleteIntegration)
func TestPitfallOutputCompleteIntegration(t *testing.T) {
	t.Run("dogfood-unique-row-leaf-parity", TestPitfallDogfoodSourceOutputParity)
	t.Run("index-domain-navigation-and-leaf-metadata", TestPitfallCorpusRendersIndexAndLeaves)
	t.Run("markdown-safe-metadata-projection", TestPitfallMetadataProjectionKeepsMarkdownStructure)
	t.Run("working-definition-dependencies", TestPitfallDefinitionsPreserveDependencies)
	t.Run("source-guidance", TestSourceMarkerFamilyMatrix)
	t.Run("hash-lock-drift-backup-prune", testPitfallHashAndOutputLifecycle)
	t.Run("staged-plan", testPitfallStagedPlanParity)
	t.Run("malformed-source", TestPitfallCorpusMalformedSourceFailsRender)
}

func testPitfallHashAndOutputLifecycle(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCfg, map[string]string{
		"docs/pitfalls/alpha.md": pitfallSource("Alpha", "domains: [rendering]\n", "alpha body\n"),
		"docs/pitfalls/beta.md":  pitfallSource("Beta", "", "beta body\n"),
		"domains/rendering.yaml": "paths: ['internal/**']\n",
	})
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	indexBefore := outputNodeAt(t, before, "docs/pitfalls.md")
	leafBefore := outputNodeAt(t, before, "docs/pitfalls/alpha.md")
	sourcePath := filepath.Join(root, ".awf/docs/pitfalls/alpha.md")
	testsupport.WriteFile(t, sourcePath, pitfallSource("Alpha", "domains: [rendering]\n", "changed alpha body\n"))
	p, err = loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	afterBody, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	indexAfterBody := outputNodeAt(t, afterBody, "docs/pitfalls.md")
	leafAfterBody := outputNodeAt(t, afterBody, "docs/pitfalls/alpha.md")
	if indexBefore.Recipe.ConfigHash != indexAfterBody.Recipe.ConfigHash || indexBefore.file.Content != indexAfterBody.file.Content {
		t.Fatal("body-only source edit changed metadata-only pitfall index")
	}
	if leafBefore.Recipe.ConfigHash == leafAfterBody.Recipe.ConfigHash || leafBefore.file.Content == leafAfterBody.file.Content {
		t.Fatal("body-only source edit did not change full-source pitfall leaf")
	}
	testsupport.WriteFile(t, sourcePath, pitfallSource("Alpha renamed", "domains: [rendering]\n", "changed alpha body\n"))
	p, err = loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	afterMetadata, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if outputNodeAt(t, afterMetadata, "docs/pitfalls.md").Recipe.ConfigHash == indexAfterBody.Recipe.ConfigHash {
		t.Fatal("pitfall metadata edit did not change index hash")
	}

	foreign := filepath.Join(root, "docs/pitfalls/alpha.md")
	testsupport.WriteFile(t, foreign, "foreign output\n")
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(foreign + ".awf-bak")
	if err != nil || string(backup) != "foreign output\n" {
		t.Fatalf("foreign pitfall backup = %q, %v", backup, err)
	}
	lock, err := manifest.Load(lockFile(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"docs/pitfalls.md", "docs/pitfalls/alpha.md", "docs/pitfalls/beta.md"} {
		if _, ok := lock.Files[path]; !ok {
			t.Fatalf("pitfall output %s absent from lock", path)
		}
	}
	indexPath := filepath.Join(root, "docs/pitfalls.md")
	if err := os.Remove(foreign); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(indexPath); err != nil {
		t.Fatal(err)
	}
	assertPitfallDrift(t, p, "docs/pitfalls/alpha.md", "missing")
	assertPitfallDrift(t, p, "docs/pitfalls.md", "missing")
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, foreign, "hand edit\n")
	testsupport.WriteFile(t, indexPath, "hand-edited index\n")
	assertPitfallDrift(t, p, "docs/pitfalls/alpha.md", "hand-edited")
	assertPitfallDrift(t, p, "docs/pitfalls.md", "hand-edited")
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".awf/docs/pitfalls/alpha.md")); err != nil {
		t.Fatal(err)
	}
	p, err = loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	_, pruned, err := syncReportProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(pruned, "docs/pitfalls/alpha.md") {
		t.Fatalf("deleted source did not prune leaf: %v", pruned)
	}
	if _, err := os.Stat(foreign); !os.IsNotExist(err) {
		t.Fatalf("pruned pitfall leaf survived: %v", err)
	}
	indexBytes, err := os.ReadFile(filepath.Join(root, "docs/pitfalls.md"))
	if err != nil || strings.Contains(string(indexBytes), "pitfalls/alpha.md") {
		t.Fatalf("deleted source index row survived: %v\n%s", err, indexBytes)
	}
}

func testPitfallStagedPlanParity(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	configYAML := withTestGateCmd(pitfallsCfg)
	gitfixture.Stage(t, repo, map[string]string{
		".awf/config.yaml":            configYAML,
		".awf/docs/pitfalls/alpha.md": pitfallSource("Alpha", "", "alpha body\n"),
		".awf/docs/pitfalls/beta.md":  pitfallSource("Beta", "", "beta body\n"),
	})
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	lockBytes, err := os.ReadFile(filepath.Join(root, ".awf/awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	gitfixture.Stage(t, repo, map[string]string{".awf/awf.lock": string(lockBytes)})
	working, err := testPublisher(renderInputsForTest(p)).Plan()
	if err != nil {
		t.Fatal(err)
	}
	stagedOutput, err := currentstatecoord.PrepareStagedOutput(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := New(stagedOutput.Session, Version).Plan()
	if err != nil {
		t.Fatal(err)
	}
	pitfallNodes := func(all []outputplan.Node) []outputplan.Node {
		var out []outputplan.Node
		for _, node := range all {
			output, ok := node.Output()
			if ok && (output.Path() == "docs/pitfalls.md" || strings.HasPrefix(output.Path(), "docs/pitfalls/")) {
				out = append(out, node)
			}
		}
		return out
	}
	workingNodes := pitfallNodes(working.Nodes())
	stagedNodes := pitfallNodes(staged.Nodes())
	if !slices.EqualFunc(workingNodes, stagedNodes, func(a, b outputplan.Node) bool {
		aOutput, aRendered := a.Output()
		bOutput, bRendered := b.Output()
		return aOutput.Path() == bOutput.Path() &&
			aRendered == bRendered &&
			(!aRendered || (aOutput.Path() == bOutput.Path() &&
				aOutput.Content() == bOutput.Content() &&
				aOutput.TemplateID() == bOutput.TemplateID() &&
				aOutput.TemplateHash() == bOutput.TemplateHash() &&
				aOutput.ConfigHash() == bOutput.ConfigHash() &&
				aOutput.RegenChecked() == bOutput.RegenChecked() &&
				aOutput.Policy() == bOutput.Policy() &&
				aOutput.Declarer() == bOutput.Declarer() &&
				aOutput.DeclarerProjection() == bOutput.DeclarerProjection() &&
				aOutput.Assembled() == bOutput.Assembled() &&
				slices.Equal(aOutput.StubDefaults(), bOutput.StubDefaults()) &&
				slices.Equal(aOutput.StubParts(), bOutput.StubParts()) &&
				slices.Equal(aOutput.MarkerParts(), bOutput.MarkerParts()) &&
				aOutput.Kind() == bOutput.Kind() &&
				aOutput.Artifact() == bOutput.Artifact() &&
				slices.Equal(aOutput.PartVarRefs(), bOutput.PartVarRefs()))) &&
			slices.Equal(a.Declarers(), b.Declarers())
	}) {
		t.Fatalf("working/staged pitfall nodes differ:\nworking=%#v\nstaged=%#v", workingNodes, stagedNodes)
	}
}

func outputNodeAt(t *testing.T, plan *OutputPlan, path string) OutputNode {
	t.Helper()
	idx := slices.IndexFunc(plan.Nodes, func(node OutputNode) bool { return node.Path == path })
	if idx < 0 {
		t.Fatalf("missing output node %s", path)
	}
	return plan.Nodes[idx]
}

func assertPitfallDrift(t *testing.T, p *project.Session, path, kind string) {
	t.Helper()
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range drift {
		if item.Path == path && item.Kind == kind {
			return
		}
	}
	t.Fatalf("missing %s drift for %s: %#v", kind, path, drift)
}

func difference(a, b []string) []string {
	set := map[string]bool{}
	for _, value := range b {
		set[value] = true
	}
	var out []string
	for _, value := range a {
		if !set[value] {
			out = append(out, value)
		}
	}
	return out
}
