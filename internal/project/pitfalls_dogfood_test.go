package project

import (
	"bytes"
	"go/ast"
	"go/format"
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
	corpus, err := loadPitfallCorpus(p)
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
	t.Run("markdown-safe-metadata-projection", TestPitfallMetadataProjectionKeepsMarkdownStructure)
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
	configYAML := withTestProfile(withTestGateCmd(pitfallsCfg))
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

// invariant: code-design/single-home:pitfall-model-single-home (TestPitfallModelSingleHome)
func TestPitfallModelSingleHome(t *testing.T) {
	sources := pitfallProductionSources(t)
	if findings := pitfallSemanticHomeFindings(sources); len(findings) != 0 {
		t.Fatalf("pitfall semantic ownership findings:\n%s", strings.Join(findings, "\n"))
	}
}

func TestPitfallModelSingleHomeProofRejectsMutations(t *testing.T) {
	production := pitfallProductionSources(t)
	for _, tc := range []struct {
		name   string
		mutate func(map[string][]byte)
		want   string
	}{
		{
			name: "outside-entry-duplicate",
			mutate: func(sources map[string][]byte) {
				sources["internal/migrate/pitfallcorpus.go"] = append(sources["internal/migrate/pitfallcorpus.go"], []byte("\ntype Entry struct {\nSlug, SourcePath, Title string\nDomains, Tags []string\nRelated []int\nBody string\nSource []byte\n}\n")...)
			},
			want: "type Entry",
		},
		{
			name: "consumer-local-allocation-rule",
			mutate: func(sources map[string][]byte) {
				path := "internal/migrate/pitfallcorpus.go"
				sources[path] = bytes.ReplaceAll(sources[path], []byte("pitfall.AllocateSlug("), []byte("consumerAllocateSlug("))
			},
			want: "internal/migrate does not call pitfall.AllocateSlug",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sources := make(map[string][]byte, len(production))
			for path, body := range production {
				sources[path] = bytes.Clone(body)
			}
			tc.mutate(sources)
			if findings := pitfallSemanticHomeFindings(sources); !slices.ContainsFunc(findings, func(finding string) bool { return strings.Contains(finding, tc.want) }) {
				t.Fatalf("mutation escaped proof: findings=%v, want %q", findings, tc.want)
			}
		})
	}
}

func pitfallProductionSources(t *testing.T) map[string][]byte {
	t.Helper()
	sources := map[string][]byte{}
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(rel string, body []byte) {
		sources[rel] = bytes.Clone(body)
	})
	return sources
}

func pitfallSemanticHomeFindings(sources map[string][]byte) []string {
	semanticTypes := map[string]bool{"Entry": true, "Corpus": true, "SourceFile": true, "RelativeLink": true}
	semanticFuncs := map[string]bool{
		"Load": true, "Parse": true, "EqualTitle": true, "Serialize": true,
		"EscapeTitle": true, "EscapeHeading": true, "EscapeLinkLabel": true,
		"EscapeTableCell": true, "RelativeLinks": true, "AllocateSlug": true,
	}
	type parsedSource struct {
		path string
		file *ast.File
	}
	var parsed []parsedSource
	for path, body := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), path, body, 0)
		if err != nil {
			return []string{"parse " + path + ": " + err.Error()}
		}
		parsed = append(parsed, parsedSource{path: path, file: file})
	}
	slices.SortFunc(parsed, func(a, b parsedSource) int { return strings.Compare(a.path, b.path) })

	ownerShapes := map[string]string{}
	for _, source := range parsed {
		if !strings.HasPrefix(source.path, "internal/pitfall/") {
			continue
		}
		for _, decl := range source.file.Decls {
			switch node := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range node.Specs {
					if typ, ok := spec.(*ast.TypeSpec); ok && semanticTypes[typ.Name.Name] {
						ownerShapes["type "+typ.Name.Name] = formattedNode(typ.Type)
					}
				}
			case *ast.FuncDecl:
				if node.Recv == nil && semanticFuncs[node.Name.Name] {
					ownerShapes["func "+node.Name.Name] = functionShape(node.Type)
				}
			}
		}
	}

	homes := map[string][]string{}
	consumers := map[string]bool{}
	consumerCalls := map[string]map[string]bool{}
	for _, source := range parsed {
		pkg := filepath.ToSlash(filepath.Dir(source.path))
		pitfallImportNames := map[string]bool{}
		for _, imp := range source.file.Imports {
			if imp.Path.Value != `"github.com/hypnotox/agentic-workflows/internal/pitfall"` {
				continue
			}
			name := "pitfall"
			if imp.Name != nil {
				name = imp.Name.Name
			}
			pitfallImportNames[name] = true
			consumers[pkg] = true
		}
		for _, decl := range source.file.Decls {
			switch node := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range node.Specs {
					typ, ok := spec.(*ast.TypeSpec)
					if !ok || !semanticTypes[typ.Name.Name] {
						continue
					}
					key := "type " + typ.Name.Name
					if formattedNode(typ.Type) == ownerShapes[key] {
						homes[key] = append(homes[key], source.path)
					}
				}
			case *ast.FuncDecl:
				key := "func " + node.Name.Name
				if node.Recv == nil && semanticFuncs[node.Name.Name] && functionShape(node.Type) == ownerShapes[key] {
					homes[key] = append(homes[key], source.path)
				}
			}
		}
		ast.Inspect(source.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok || !pitfallImportNames[ident.Name] {
				return true
			}
			if consumerCalls[pkg] == nil {
				consumerCalls[pkg] = map[string]bool{}
			}
			consumerCalls[pkg][selector.Sel.Name] = true
			return true
		})
	}

	var findings []string
	for declaration := range ownerShapes {
		paths := homes[declaration]
		if len(paths) != 1 || paths[0] != "internal/pitfall/pitfall.go" {
			findings = append(findings, declaration+" homes = "+strings.Join(paths, ", "))
		}
	}
	wantConsumers := map[string]bool{"internal/project": true, "internal/migrate": true}
	for consumer := range consumers {
		if !wantConsumers[consumer] {
			findings = append(findings, "unexpected pitfall consumer "+consumer)
		}
	}
	for consumer := range wantConsumers {
		if !consumers[consumer] {
			findings = append(findings, "missing pitfall consumer "+consumer)
		}
	}
	for consumer, expected := range map[string][]string{
		"internal/project": {"Load", "EscapeHeading", "EscapeLinkLabel", "EscapeTableCell"},
		"internal/migrate": {"AllocateSlug", "Serialize", "RelativeLinks", "Load"},
	} {
		for _, name := range expected {
			if !consumerCalls[consumer][name] {
				findings = append(findings, consumer+" does not call pitfall."+name)
			}
		}
	}
	slices.Sort(findings)
	return findings
}

func functionShape(function *ast.FuncType) string {
	fieldShapes := func(fields *ast.FieldList) []string {
		if fields == nil {
			return nil
		}
		var shapes []string
		for _, field := range fields.List {
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for range count {
				shapes = append(shapes, formattedNode(field.Type))
			}
		}
		return shapes
	}
	return "(" + strings.Join(fieldShapes(function.Params), ",") + ")->(" + strings.Join(fieldShapes(function.Results), ",") + ")"
}

func formattedNode(node any) string {
	var out bytes.Buffer
	if err := format.Node(&out, token.NewFileSet(), node); err != nil {
		return "<format-error>"
	}
	return out.String()
}
