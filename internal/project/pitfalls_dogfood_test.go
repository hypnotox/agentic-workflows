package project

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestPitfallDogfoodSourceOutputParity(t *testing.T) {
	root := filepath.Clean("../..")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := p.loadPitfallCorpus()
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
// corpus-to-index/leaf parity and navigation, source guidance, declarations and
// dependencies, hash isolation, lock/drift/backup/prune lifecycle, staged
// projection, and malformed-source refusal.
// invariant: rendering/doc-outputs:pitfall-output-complete (TestPitfallOutputCompleteIntegration)
func TestPitfallOutputCompleteIntegration(t *testing.T) {
	t.Run("dogfood-unique-row-leaf-parity", TestPitfallDogfoodSourceOutputParity)
	t.Run("index-domain-navigation-and-leaf-metadata", TestPitfallCorpusRendersIndexAndLeaves)
	t.Run("working-declaration-plan-parity", TestPitfallDeclarationPlanDependencyParity)
	t.Run("source-guidance", TestSourceMarkerFamilyMatrix)
	t.Run("hash-lock-drift-backup-prune", testPitfallHashAndOutputLifecycle)
	t.Run("staged-projection", testPitfallStagedDeclarationParity)
	t.Run("malformed-source", TestPitfallCorpusMalformedSourceFailsRender)
}

func testPitfallHashAndOutputLifecycle(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCfg, map[string]string{
		"docs/pitfalls/alpha.md": pitfallSource("Alpha", "domains: [rendering]\n", "alpha body\n"),
		"docs/pitfalls/beta.md":  pitfallSource("Beta", "", "beta body\n"),
		"domains/rendering.yaml": "paths: ['internal/**']\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := p.OutputPlan(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	indexBefore := outputNodeAt(t, before, "docs/pitfalls.md")
	leafBefore := outputNodeAt(t, before, "docs/pitfalls/alpha.md")
	sourcePath := filepath.Join(root, ".awf/docs/pitfalls/alpha.md")
	testsupport.WriteFile(t, sourcePath, pitfallSource("Alpha", "domains: [rendering]\n", "changed alpha body\n"))
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	afterBody, err := p.OutputPlan(testContext(t))
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
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	afterMetadata, err := p.OutputPlan(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if outputNodeAt(t, afterMetadata, "docs/pitfalls.md").Recipe.ConfigHash == indexAfterBody.Recipe.ConfigHash {
		t.Fatal("pitfall metadata edit did not change index hash")
	}

	foreign := filepath.Join(root, "docs/pitfalls/alpha.md")
	testsupport.WriteFile(t, foreign, "foreign output\n")
	if err := p.Sync(); err != nil {
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
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, foreign, "hand edit\n")
	testsupport.WriteFile(t, indexPath, "hand-edited index\n")
	assertPitfallDrift(t, p, "docs/pitfalls/alpha.md", "hand-edited")
	assertPitfallDrift(t, p, "docs/pitfalls.md", "hand-edited")
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".awf/docs/pitfalls/alpha.md")); err != nil {
		t.Fatal(err)
	}
	p, err = Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, pruned, err := p.SyncReport(testContext(t))
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

func testPitfallStagedDeclarationParity(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	configYAML := withTestGateCmd(pitfallsCfg)
	gitfixture.Stage(t, repo, map[string]string{
		".awf/config.yaml":            configYAML,
		".awf/docs/pitfalls/alpha.md": pitfallSource("Alpha", "", "alpha body\n"),
		".awf/docs/pitfalls/beta.md":  pitfallSource("Beta", "", "beta body\n"),
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	lockBytes, err := os.ReadFile(filepath.Join(root, ".awf/awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	gitfixture.Stage(t, repo, map[string]string{".awf/awf.lock": string(lockBytes)})
	working, err := p.ContextState(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	staged, err := StagedContextState(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	pitfallDeclarations := func(all []OutputDeclaration) []OutputDeclaration {
		var out []OutputDeclaration
		for _, declaration := range all {
			if declaration.Path == "docs/pitfalls.md" || strings.HasPrefix(declaration.Path, "docs/pitfalls/") {
				out = append(out, declaration)
			}
		}
		return out
	}
	if !slices.EqualFunc(pitfallDeclarations(working.Declarations), pitfallDeclarations(staged.Declarations), func(a, b OutputDeclaration) bool {
		return a.Path == b.Path && a.TemplateID == b.TemplateID && slices.Equal(a.Declarers, b.Declarers) && slices.Equal(a.Inputs, b.Inputs) && slices.Equal(a.Dependencies, b.Dependencies)
	}) {
		t.Fatalf("working/staged pitfall declarations differ:\nworking=%#v\nstaged=%#v", pitfallDeclarations(working.Declarations), pitfallDeclarations(staged.Declarations))
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

func assertPitfallDrift(t *testing.T, p *Project, path, kind string) {
	t.Helper()
	drift, err := p.Check(testContext(t))
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

// invariant: code-design/single-home:pitfall-model-single-home (TestPitfallModelSingleHome)
func TestPitfallModelSingleHome(t *testing.T) {
	root := testsupport.RepoRoot(t)
	semanticTypes := map[string]bool{"Entry": true, "Corpus": true, "SourceFile": true, "RelativeLink": true}
	semanticFuncs := map[string]bool{
		"Load": true, "Parse": true, "EqualTitle": true, "Serialize": true,
		"EscapeTitle": true, "EscapeHeading": true, "EscapeLinkLabel": true,
		"EscapeTableCell": true, "RelativeLinks": true, "AllocateSlug": true,
	}
	declarations := map[string][]string{}
	consumers := map[string]bool{}
	consumerCalls := map[string]map[string]bool{}
	testsupport.WalkRepoSources(t, root, func(rel string, body []byte) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || (!strings.HasPrefix(rel, "internal/") && !strings.HasPrefix(rel, "cmd/")) {
			return
		}
		file, err := parser.ParseFile(token.NewFileSet(), rel, body, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, imp := range file.Imports {
			if imp.Path.Value == `"github.com/hypnotox/agentic-workflows/internal/pitfall"` {
				consumers[filepath.ToSlash(filepath.Dir(rel))] = true
			}
		}
		for _, decl := range file.Decls {
			switch node := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range node.Specs {
					if typ, ok := spec.(*ast.TypeSpec); ok && semanticTypes[typ.Name.Name] && strings.HasPrefix(rel, "internal/pitfall/") {
						declarations["type "+typ.Name.Name] = append(declarations["type "+typ.Name.Name], rel)
					}
				}
			case *ast.FuncDecl:
				if node.Recv == nil && semanticFuncs[node.Name.Name] && strings.HasPrefix(rel, "internal/pitfall/") {
					declarations["func "+node.Name.Name] = append(declarations["func "+node.Name.Name], rel)
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok || ident.Name != "pitfall" {
				return true
			}
			pkg := filepath.ToSlash(filepath.Dir(rel))
			if consumerCalls[pkg] == nil {
				consumerCalls[pkg] = map[string]bool{}
			}
			consumerCalls[pkg][selector.Sel.Name] = true
			return true
		})
	})
	for name := range semanticTypes {
		assertPitfallSemanticHome(t, "type "+name, declarations["type "+name])
	}
	for name := range semanticFuncs {
		assertPitfallSemanticHome(t, "func "+name, declarations["func "+name])
	}
	wantConsumers := map[string]bool{"internal/project": true, "internal/migrate": true}
	if len(consumers) != len(wantConsumers) {
		t.Fatalf("pitfall production consumer packages = %v, want %v", consumers, wantConsumers)
	}
	for consumer := range wantConsumers {
		if !consumers[consumer] {
			t.Fatalf("pitfall model consumer %s does not import the semantic home; consumers=%v", consumer, consumers)
		}
	}
	for consumer, expected := range map[string][]string{
		"internal/project": {"Load", "EscapeHeading", "EscapeLinkLabel", "EscapeTableCell"},
		"internal/migrate": {"AllocateSlug", "Serialize", "RelativeLinks", "Load"},
	} {
		for _, name := range expected {
			if !consumerCalls[consumer][name] {
				t.Fatalf("%s does not consume pitfall.%s; calls=%v", consumer, name, consumerCalls[consumer])
			}
		}
	}
}

func assertPitfallSemanticHome(t *testing.T, declaration string, paths []string) {
	t.Helper()
	if len(paths) != 1 || paths[0] != "internal/pitfall/pitfall.go" {
		t.Fatalf("%s homes = %v, want exactly internal/pitfall/pitfall.go", declaration, paths)
	}
}
