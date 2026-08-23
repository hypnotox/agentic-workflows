package main

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"golang.org/x/tools/go/packages"
)

func TestRunSyncEntryPointsRejectMalformedRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := testContext(t)
	if err := runSync(ctx, root, io.Discard); err == nil || errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("malformed repository error = %v", err)
	}
}

func TestRunSyncPrintingUsesInjectedLoader(t *testing.T) {
	ctx := testContext(t)
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, minimalYAML)
	if _, err := os.Stat(config.LockPath(root)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock stat error = %v, want not exist", err)
	}
	if err := initializeProject(ctx, root, io.Discard); err != nil {
		t.Fatal(err)
	}
	var loadPaths []string
	loader := project.NewLoader(func(path string) (*config.Config, error) {
		loadPaths = append(loadPaths, path)
		return config.Load(path)
	}, catalog.Standard, func(_ context.Context, got string) string { return got }, mustOpenGit(t, root))
	if err := runSyncPrinting(ctx, loader, root, io.Discard); err != nil {
		t.Fatal(err)
	}
	want := config.RootDir(root)
	if len(loadPaths) != 1 || loadPaths[0] != want {
		t.Fatalf("config load paths = %v, want [%q]", loadPaths, want)
	}
}

type syncCompositionCall struct {
	file  string
	owner string
	name  string
}

// invariant: code-design/dependency-composition:sync-project-loader-wiring (TestSyncCompositionAndCallers)
func TestSyncCompositionAndCallers(t *testing.T) {
	want := map[syncCompositionCall]int{
		{file: "projectstate.go", owner: "openProjectOperation", name: "NewLoader"}:                   1,
		{file: "projectstate.go", owner: "openProjectOperation", name: "NewLoaderWithoutRepository"}:  1,
		{file: "projectstate.go", owner: "openProjectOperation", name: "OpenForOperation"}:            2,
		{file: "sync.go", owner: "newProjectLoader", name: "NewLoader"}:                               1,
		{file: "sync.go", owner: "newProjectLoader", name: "NewLoaderWithoutRepository"}:              1,
		{file: "sync.go", owner: "runSyncPrinting", name: "OpenForOperation"}:                         1,
		{file: "sync.go", owner: "runSyncPrinting", name: "Sync"}:                                     1,
		{file: "upgrade_presentation.go", owner: "productionUpgradeSyncDependencies", name: "Sync"}:   1,
		{file: "upgrade_presentation.go", owner: "upgradeSyncMutationWith", name: "OpenForOperation"}: 1,
		{file: "adr.go", owner: "runADR", name: "Sync"}:                                               1,
	}
	assertSyncCompositionCalls(t, syncCompositionCalls(loadSyncCompositionPackage(t, nil)), want)

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "cmd", "awf", "sync_wiring_mutation_fixture.go")
	mutation := loadSyncCompositionPackage(t, map[string][]byte{
		fixture: []byte(`package main

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

func mutationAddsPostRender(state *project.ProjectState, cfg *config.Config, ctx context.Context) {
	_ = ctx
	_, _ = publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version).Sync()
}

var mutationAddsPackagePostRender = func(state *project.ProjectState, cfg *config.Config, ctx context.Context) {
	_ = ctx
	_, _ = publisher.New(state.OutputState(), cfg, publisher.NewFilesystemReader(state.Root()), project.Version).Sync()
}
`),
	})
	got := syncCompositionCalls(mutation)
	if got[syncCompositionCall{file: "sync_wiring_mutation_fixture.go", owner: "mutationAddsPostRender", name: "Sync"}] != 1 {
		t.Fatal("typed caller census did not detect an added post-mutation render")
	}
	if got[syncCompositionCall{file: "sync_wiring_mutation_fixture.go", owner: "<package>", name: "Sync"}] != 1 {
		t.Fatal("typed caller census did not detect a package-level post-mutation render")
	}
}

func loadSyncCompositionPackage(t *testing.T, overlay map[string][]byte) *packages.Package {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	mode := packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
		packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo
	pkgs, err := packages.Load(&packages.Config{Dir: root, Mode: mode, Overlay: overlay}, "./cmd/awf")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("loaded command packages = %d, want 1", len(pkgs))
	}
	if len(pkgs[0].Errors) != 0 {
		t.Fatal(pkgs[0].Errors[0])
	}
	return pkgs[0]
}

func syncCompositionCalls(pkg *packages.Package) map[syncCompositionCall]int {
	got := map[syncCompositionCall]int{}
	for _, file := range pkg.Syntax {
		owners := map[token.Pos]string{}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if node != nil {
					owners[node.Pos()] = fn.Name.Name
				}
				return true
			})
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			object, ok := pkg.TypesInfo.Uses[selector.Sel].(*types.Func)
			if !ok || object.Pkg() == nil {
				return true
			}
			packagePath := object.Pkg().Path()
			if packagePath != "github.com/hypnotox/agentic-workflows/internal/project" && packagePath != "github.com/hypnotox/agentic-workflows/internal/publisher" {
				return true
			}
			switch object.Name() {
			case "NewLoader", "NewLoaderWithoutRepository", "Open", "OpenForOperation", "Sync":
				owner := owners[call.Pos()]
				if owner == "" {
					owner = "<package>"
				}
				position := pkg.Fset.Position(call.Pos())
				got[syncCompositionCall{file: filepath.Base(position.Filename), owner: owner, name: object.Name()}]++
			}
			return true
		})
	}
	return got
}

func assertSyncCompositionCalls(t *testing.T, got, want map[syncCompositionCall]int) {
	t.Helper()
	for site, count := range want {
		if got[site] != count {
			t.Errorf("call %s:%s %s count = %d, want %d", site.file, site.owner, site.name, got[site], count)
		}
		delete(got, site)
	}
	for site, count := range got {
		t.Errorf("unexpected call %s:%s %s count %d", site.file, site.owner, site.name, count)
	}
}

func TestRunSyncIgnoresSkillSelection(t *testing.T) {
	ctx := testContext(t)
	root := scaffoldProject(t)
	// Phase 2 retains parsing of the selection field but renders the full catalog.
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "skills: [tdd]", "skills: []", 1))
	var out bytes.Buffer
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	const expected = "status: completed\n\nmutation:\n  next actions:\n    step 1: continue with the rendered project state\n"
	if out.String() != expected {
		t.Errorf("selection-free sync bytes = %q, want %q", out.String(), expected)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")); err != nil {
		t.Fatalf("full-catalog skill was pruned after selection edit: %v", err)
	}
	// A drift-clean re-sync emits the complete empty-success document.
	out.Reset()
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "status: completed\n\nmutation:\n  next actions:\n    step 1: continue with the rendered project state\n" {
		t.Errorf("empty sync bytes = %q", got)
	}
}

func TestRunSyncPrintsChangedFiles(t *testing.T) {
	ctx := testContext(t)
	root := scaffoldProject(t)
	// A var edit moves the config hash of every artifact referencing it; the
	// re-sync attributes the changed output to the project's own inputs.
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "gateCmd: make gate", "gateCmd: ./x gate", 1))
	var out bytes.Buffer
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"changed .claude/agents/implementer.md (config)",
		"changed .claude/skills/example-tdd/SKILL.md (config)",
		"changed .pi/agents/implementer.md (config)",
		"changed .pi/skills/example-tdd/SKILL.md (config)",
		"changed AGENTS.md (config)",
		"changed docs/workflow.md (config)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("config change did not update full-catalog output %q:\n%s", want, out.String())
		}
	}
	// A drift-clean re-sync emits the complete empty-success document.
	out.Reset()
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "status: completed\n\nmutation:\n  next actions:\n    step 1: continue with the rendered project state\n" {
		t.Errorf("empty sync bytes = %q", got)
	}
	// Enabling an artifact reports its files as added.
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "gateCmd: make gate", "gateCmd: ./x gate", 1)+"")
	out.Reset()
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	const selectionIgnored = "status: completed\n\nmutation:\n  next actions:\n    step 1: continue with the rendered project state\n"
	if out.String() != selectionIgnored {
		t.Errorf("docs selection-free sync bytes = %q, want %q", out.String(), selectionIgnored)
	}
}
