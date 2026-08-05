package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// minimalYAML is a valid tree-config for a scaffolded fixture project.
const minimalYAML = `prefix: example
integrationBranch: master
vars: {testCmd: go test ./..., gateCmd: make gate}
skills: [tdd]
agents: []
`

// scaffoldProject writes a minimal tree config under a git-backed root and syncs
// it, leaving a drift-clean project. The base commit gives the working Tree a
// HEAD, which the commands that read one (check, invariants) require.
func initializeProject(ctx context.Context, root string, out io.Writer) error {
	loader, err := newProjectLoader(root)
	if err != nil {
		return err
	}
	seed := &project.InitAuthority{InitializedWithVersion: project.Version}
	return runSyncPrinting(ctx, loader, root, seed, out)
}

func scaffoldProject(t *testing.T) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, minimalYAML)
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatalf("scaffold sync: %v", err)
	}
	return root
}

func mustOpenGit(t *testing.T, root string) *awfgit.Repo {
	t.Helper()
	repo, err := awfgit.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestResolveProjectResidentRoot(t *testing.T) {
	ctx := testContext(t)
	repo := gitfixture.InitRepo(t)
	primary := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	linked := filepath.Join(t.TempDir(), "linked")
	gitfixture.NativeWorktreeAdd(t, repo, linked, "linked")
	if got := awfgit.ProjectResidentRoot(ctx, linked); got != primary {
		t.Fatalf("resident root = %q, want primary %q", got, primary)
	}
}

func TestResolveProjectResidentRootFallsBackOutsideGit(t *testing.T) {
	ctx := testContext(t)
	root := t.TempDir()
	if got := awfgit.ProjectResidentRoot(ctx, root); got != root {
		t.Fatalf("resident root = %q, want invoking root", got)
	}
}

func TestResolveProjectResidentRootFallsBackOnUnsafeResident(t *testing.T) {
	ctx := testContext(t)
	root := gitfixture.InitRepo(t).Root()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, ".awf")); err != nil {
		t.Fatal(err)
	}
	if got := awfgit.ProjectResidentRoot(ctx, root); got != root {
		t.Fatalf("resident root = %q, want invoking root", got)
	}
}

func TestCommandStagesUseIndependentDeadlines(t *testing.T) {
	first, cancelFirst := newGitCommandContext()
	second, cancelSecond := newGitCommandContext()
	defer cancelSecond()
	firstDeadline, firstOK := first.Deadline()
	secondDeadline, secondOK := second.Deadline()
	if !firstOK || !secondOK {
		t.Fatalf("stage deadlines present = %v, %v", firstOK, secondOK)
	}
	for name, deadline := range map[string]time.Time{"first": firstDeadline, "second": secondDeadline} {
		remaining := time.Until(deadline)
		if remaining < gitCommandTimeout-time.Second || remaining > gitCommandTimeout {
			t.Fatalf("%s stage deadline remaining = %v, want approximately %v", name, remaining, gitCommandTimeout)
		}
	}
	cancelFirst()
	if !errors.Is(first.Err(), context.Canceled) || second.Err() != nil {
		t.Fatalf("stage cancellation leaked: first=%v second=%v", first.Err(), second.Err())
	}

	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, decl := range src.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "run" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "newGitCommandContext" {
				calls++
			}
			return true
		})
	}
	if calls != 3 {
		t.Fatalf("run creates %d stage contexts, want guard, gate, and handler", calls)
	}
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for stage, wiring := range map[string]string{
		"guard":   "guardProjectState(guardCtx,",
		"gate":    "gateFn(gateCtx,",
		"handler": "ctx: handlerCtx,",
	} {
		if !bytes.Contains(raw, []byte(wiring)) {
			t.Errorf("%s stage is not wired to its own context variable %q", stage, wiring)
		}
	}
}

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
	var loadPaths []string
	loader := project.NewLoader(func(path string) (*config.Config, error) {
		loadPaths = append(loadPaths, path)
		return config.Load(path)
	}, catalog.Standard, func(_ context.Context, got string) string { return got }, mustOpenGit(t, root))
	if err := runSyncPrinting(ctx, loader, root, &project.InitAuthority{InitializedWithVersion: project.Version}, io.Discard); err != nil {
		t.Fatal(err)
	}
	want := config.RootDir(root)
	if len(loadPaths) != 1 || loadPaths[0] != want {
		t.Fatalf("config load paths = %v, want [%q]", loadPaths, want)
	}
}

// invariant: code-design/dependency-composition:sync-project-loader-wiring (TestSyncCompositionAndCallers)
func TestSyncCompositionAndCallers(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	type call struct{ file, owner, name string }
	var calls []call
	fset := token.NewFileSet()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		owners := map[token.Pos]string{}
		for _, decl := range src.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if n != nil {
					owners[n.Pos()] = fn.Name.Name
				}
				return true
			})
		}
		ast.Inspect(src, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch fun := ce.Fun.(type) {
			case *ast.Ident:
				if fun.Name == "runSync" || fun.Name == "runSyncPrinting" || fun.Name == "syncMutation" || fun.Name == "newProjectLoader" || fun.Name == "initProjectLoader" {
					name = fun.Name
				}
			case *ast.SelectorExpr:
				recv, ok := fun.X.(*ast.Ident)
				if ok && recv.Name == "project" && (fun.Sel.Name == "NewLoader" || fun.Sel.Name == "NewLoaderWithoutRepository" || fun.Sel.Name == "Open" || fun.Sel.Name == "VerifyCommitPolicyAt") {
					name = "project." + fun.Sel.Name
				} else if ok && recv.Name == "loader" && fun.Sel.Name == "Open" {
					name = "loader.Open"
				} else if call, ok := fun.X.(*ast.CallExpr); ok && fun.Sel.Name == "Open" {
					if constructor, ok := call.Fun.(*ast.SelectorExpr); ok {
						constructorRecv, recvOK := constructor.X.(*ast.Ident)
						if recvOK && constructorRecv.Name == "project" && (constructor.Sel.Name == "NewLoader" || constructor.Sel.Name == "NewLoaderWithoutRepository") {
							name = "project." + constructor.Sel.Name + ".Open"
						}
					}
				}
			}
			if name != "" {
				calls = append(calls, call{file: path, owner: owners[ce.Pos()], name: name})
			}
			return true
		})
	}

	want := map[call]int{
		{file: "sync.go", owner: "runSync", name: "newProjectLoader"}:                                                     1,
		{file: "sync.go", owner: "runSync", name: "runSyncPrinting"}:                                                      1,
		{file: "sync.go", owner: "runSyncPrinting", name: "syncMutation"}:                                                 1,
		{file: "sync.go", owner: "syncMutation", name: "loader.Open"}:                                                     1,
		{file: "sync.go", owner: "newProjectLoader", name: "project.NewLoader"}:                                           1,
		{file: "sync.go", owner: "newProjectLoader", name: "project.NewLoaderWithoutRepository"}:                          1,
		{file: "dispatch.go", owner: "", name: "runSync"}:                                                                 1,
		{file: "init.go", owner: "runInitWithProjectLoader", name: "initProjectLoader"}:                                   1,
		{file: "init.go", owner: "runInitWithProjectLoader", name: "syncMutation"}:                                        1,
		{file: "list_add.go", owner: "enableDisableSingleton", name: "runSync"}:                                           1,
		{file: "list_add.go", owner: "enableDisableTarget", name: "runSync"}:                                              1,
		{file: "list_add.go", owner: "toggleWithProjectLoader", name: "syncMutation"}:                                     1,
		{file: "new.go", owner: "newLocal", name: "runSync"}:                                                              1,
		{file: "upgrade_presentation.go", owner: "upgradeSyncMutationWith", name: "newProjectLoader"}:                     1,
		{file: "upgrade_presentation.go", owner: "upgradeSyncMutationWith", name: "loader.Open"}:                          1,
		{file: "adr.go", owner: "runADR", name: "project.Open"}:                                                           1,
		{file: "audit.go", owner: "runAudit", name: "project.Open"}:                                                       1,
		{file: "checkrepo.go", owner: "productionRepoCheckDependencies", name: "project.NewLoader"}:                       1,
		{file: "checkrepo.go", owner: "productionRepoCheckDependencies", name: "project.NewLoader.Open"}:                  1,
		{file: "checkrepo.go", owner: "productionRepoCheckDependencies", name: "project.NewLoaderWithoutRepository"}:      1,
		{file: "checkrepo.go", owner: "productionRepoCheckDependencies", name: "project.NewLoaderWithoutRepository.Open"}: 1,
		{file: "commitgate.go", owner: "openCommitGateProjectFromDisk", name: "project.Open"}:                             1,
		{file: "commitpolicy.go", owner: "runCommitPolicy", name: "project.VerifyCommitPolicyAt"}:                         1,
		{file: "config.go", owner: "runConfig", name: "project.Open"}:                                                     1,
		{file: "context.go", owner: "runContext", name: "project.Open"}:                                                   1,
		{file: "context.go", owner: "runUncovered", name: "project.Open"}:                                                 1,
		{file: "init.go", owner: "probeCollisions", name: "project.Open"}:                                                 2,
		{file: "init.go", owner: "runInitWithProjectLoader", name: "project.Open"}:                                        1,
		{file: "list_add.go", owner: "enableDisableSingleton", name: "project.Open"}:                                      1,
		{file: "list_add.go", owner: "enableDisableTarget", name: "project.Open"}:                                         1,
		{file: "list_add.go", owner: "runList", name: "project.Open"}:                                                     1,
		{file: "list_add.go", owner: "toggleWithProjectLoader", name: "project.Open"}:                                     1,
		{file: "new.go", owner: "newADR", name: "project.Open"}:                                                           1,
		{file: "new.go", owner: "newLocal", name: "project.Open"}:                                                         1,
		{file: "new.go", owner: "newPlan", name: "project.Open"}:                                                          1,
		{file: "new.go", owner: "newTopic", name: "project.Open"}:                                                         1,
		{file: "read.go", owner: "runReadPlan", name: "project.Open"}:                                                     1,
		{file: "topic.go", owner: "runTopic", name: "project.Open"}:                                                       1,
	}
	got := map[call]int{}
	for _, site := range calls {
		got[site]++
	}
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

func TestInitRejectsAmbiguousBrownfieldAuthority(t *testing.T) {
	ctx := testContext(t)
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"malformed", map[string]string{"docs/decisions/0001-bad.md": "---\nstatus: [bad\n---\n"}},
		{"duplicate", map[string]string{
			"docs/decisions/0001-one.md": testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle("0001: One")),
			"docs/decisions/0001-two.md": testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle("0001: Two")),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.SwapVar(t, &isInteractive, func() bool { return false })
			for path, body := range tc.files {
				testsupport.WriteFile(t, filepath.Join(root, path), body)
			}
			before := snapshotTree(t, root)
			var out bytes.Buffer
			if err := runInit(ctx, root, false, false, nil, "", &out); err == nil {
				t.Fatal("expected refusal")
			}
			if after := snapshotTree(t, root); after != before {
				t.Fatal("ambiguous first adoption mutated the repository tree")
			}
			if out.Len() != 0 {
				t.Fatalf("ambiguous first adoption wrote output: %q", out.String())
			}
		})
	}
}

// invariant: tooling/upgrade-runtime:initial-adoption-version-immutable (TestInitialAdoptionVersionImmutableAcrossCommands)
func TestInitialAdoptionVersionImmutableAcrossCommands(t *testing.T) {
	ctx := testContext(t)
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.SwapVar(t, &isInteractive, func() bool { return false })
	if err := runInit(ctx, root, false, false, []string{"gateCmd=make gate"}, "", io.Discard); err != nil {
		t.Fatal(err)
	}
	initial, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	assertVersion := func(step string) {
		t.Helper()
		got, err := manifest.Load(config.LockPath(root))
		if err != nil {
			t.Fatal(err)
		}
		if got.InitializedWithVersion != initial.InitializedWithVersion {
			t.Fatalf("%s changed initializedWithVersion: %q -> %q", step, initial.InitializedWithVersion, got.InitializedWithVersion)
		}
	}
	if err := runSync(ctx, root, io.Discard); err != nil {
		t.Fatal(err)
	}
	assertVersion("ordinary sync")
	if err := runUpgrade(ctx, root, io.Discard); err != nil {
		t.Fatal(err)
	}
	assertVersion("zero-migration upgrade")
	if err := runInit(ctx, root, true, false, []string{"gateCmd=make gate"}, "", io.Discard); err != nil {
		t.Fatal(err)
	}
	assertVersion("forced initialization")

	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "initialize", nil)
	mutated := *initial
	mutated.InitializedWithVersion = "0.1.0"
	if mutated.InitializedWithVersion == initial.InitializedWithVersion {
		mutated.InitializedWithVersion = "0.2.0"
	}
	if err := mutated.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	gitfixture.Add(t, repo, ".awf/awf.lock")
	if err := runCheckStaged(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), "immutable initializedWithVersion") {
		t.Fatalf("staged initializedWithVersion mutation error = %v", err)
	}
}

func TestInitFirstADRChecksClean(t *testing.T) {
	testInitFirstADRChecksClean(t)
}

func testInitFirstADRChecksClean(t *testing.T) {
	ctx := testContext(t)
	for _, tc := range []struct {
		name       string
		legacy     []string
		nextNumber int
	}{
		{name: "fresh", nextNumber: 1},
		{name: "brownfield", legacy: []string{"0001-old.md", "0003-old.md"}, nextNumber: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gitfixture.InitRepo(t)
			root := repo.Root()
			gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
			for _, name := range tc.legacy {
				testsupport.WriteFile(t, filepath.Join(root, "docs/decisions", name), testsupport.ADR("Accepted", testsupport.WithDate("2026-07-21"), testsupport.WithTitle(name[:4]+": Old")))
			}
			testsupport.SwapVar(t, &isInteractive, func() bool { return false })
			// The gateCmd answer keeps the scaffold's enabled hooks singleton
			// valid for the post-init syncs (ADR-0156 Decision 5).
			if err := runInit(ctx, root, false, false, []string{"gateCmd=make gate"}, "", io.Discard); err != nil {
				t.Fatal(err)
			}

			gitfixture.AddAll(t, repo)
			gitfixture.Commit(t, repo, "initialize", nil)
			// The scaffold writes integrationBranch: main while a go-git
			// fixture starts on master; put the checkout on the branch the
			// scaffolded config names, so `new adr` takes the numbered path
			// this test is about (ADR-0202 item 5).
			gitfixture.NativeBranch(t, repo, "main")
			gitfixture.NativeCheckout(t, repo, "main")
			if err := runNew(ctx, root, "adr", []string{"First", "Current"}, io.Discard); err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf("%04d-", tc.nextNumber)
			entries, err := os.ReadDir(filepath.Join(root, "docs/decisions"))
			if err != nil {
				t.Fatal(err)
			}
			var created string
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), want) {
					created = filepath.Join(root, "docs/decisions", entry.Name())
				}
			}
			if created == "" {
				t.Fatalf("new ADR not created at next number %d", tc.nextNumber)
			}
			body, err := os.ReadFile(created)
			if err != nil {
				t.Fatal(err)
			}
			text := string(body)
			// Scaffolding uses the activation registry's current format, so a new
			// record is V3 with its slug key regardless of existing ADR numbers.
			if !strings.Contains(text, "format: current-state-v4\n") {
				t.Fatalf("new ADR at next number %d is not current-state-v4", tc.nextNumber)
			}
			start, end := strings.Index(text, "## State changes\n"), strings.Index(text, "## Consequences\n")
			if start < 0 || end < 0 || end <= start {
				t.Fatal("scaffold lacks state-change section")
			}
			text = text[:start] + "## State changes\n\nNone.\n\n" + text[end:]
			history := strings.Index(text, "## Status history\n")
			if history < 0 {
				t.Fatal("scaffold lacks status history")
			}
			text = text[:history] + "## Status history\n\n- 2026-07-21: Proposed\n"
			if err := os.WriteFile(created, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := runSync(ctx, root, io.Discard); err != nil {
				t.Fatal(err)
			}
			if err := runCheckRepo(ctx, root, io.Discard); err != nil {
				t.Fatalf("repo check: %v", err)
			}
		})
	}
}

func TestRunSyncPrintsPrunedFiles(t *testing.T) {
	ctx := testContext(t)
	root := scaffoldProject(t)
	// Disable the only skill; the re-sync prunes its rendered file and says so.
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "skills: [tdd]", "skills: []", 1))
	var out bytes.Buffer
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	const pruned = "status: completed\n\nmutation:\n  changes:\n    outputs:\n      changed docs/config-reference.md (regenerated)\n    pruned:\n      .claude/skills/example-tdd/SKILL.md\n  next actions:\n    step 1: continue with the rendered project state\n"
	if out.String() != pruned {
		t.Errorf("pruned sync bytes = %q, want %q", out.String(), pruned)
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
	const changed = "status: completed\n\nmutation:\n  changes:\n    outputs:\n      changed .claude/skills/example-tdd/SKILL.md (config)\n      changed AGENTS.md (config)\n      changed docs/config-reference.md (regenerated)\n      changed docs/plans/template.md (config)\n      changed docs/workflow.md (config)\n  next actions:\n    step 1: continue with the rendered project state\n"
	if out.String() != changed {
		t.Errorf("changed sync bytes = %q, want %q", out.String(), changed)
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
	testsupport.WriteAwfConfig(t, root, strings.Replace(minimalYAML, "gateCmd: make gate", "gateCmd: ./x gate", 1)+"docs: [pitfalls]\n")
	out.Reset()
	if err := runSync(ctx, root, &out); err != nil {
		t.Fatal(err)
	}
	const added = "status: completed\n\nmutation:\n  changes:\n    outputs:\n      changed AGENTS.md (config)\n      changed docs/config-reference.md (regenerated)\n      added docs/pitfalls.md\n  next actions:\n    step 1: continue with the rendered project state\n"
	if out.String() != added {
		t.Errorf("added sync bytes = %q, want %q", out.String(), added)
	}
}

func TestRunNoArgs(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"awf"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for no args, got %d", code)
	}
	want := "condition: awf: usage: " + clispec.UsageLine() + " [args]; run `awf help` for command details\n"
	if out.Len() != 0 || errb.String() != want {
		t.Errorf("streams stdout=%q stderr=%q, want stderr=%q", out.String(), errb.String(), want)
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		var out, errb bytes.Buffer
		if code := run([]string{"awf", arg}, &out, &errb); code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", arg, code)
		}
		if !strings.Contains(out.String(), "commands:") || !strings.Contains(out.String(), "uninstall") {
			t.Errorf("%s: help text missing content:\n%s", arg, out.String())
		}
	}
}

// TestTopLevelCommandFamiliesUseStructuredHelpAndUsageFailures is an explicit
// registry-keyed interface contract. A new top-level family must be added here;
// there is no count-based or missing-family allowance. Each entry names the
// separately executed test that pins a real result from that family; this unit
// additionally pins its exact help, usage failure, and operational failure.
func TestTopLevelCommandFamiliesUseStructuredHelpAndUsageFailures(t *testing.T) {
	families := map[string]string{
		"init":      "TestInitDescribeReadOnly",
		"render":    "TestEmptyInitChecksOnUnbornHead",
		"check":     "TestRunCheckCleanThenDirty",
		"read":      "TestReadPlanCommand",
		"audit":     "TestRunAuditDispatch",
		"effort":    "TestEffortPublicTextProtocol",
		"adr":       "TestRunADRNumberThroughTheDriver",
		"list":      "TestRunListBareShowsAllKinds",
		"config":    "TestRunConfigDispatch",
		"context":   "TestRunContextModesShareDeliveryIncludingOversize",
		"topic":     "TestRunTopicHumanTextAndFlags",
		"new":       "TestRunNewDispatch",
		"enable":    "TestDispatchAddRemoveList",
		"disable":   "TestDispatchAddRemoveList",
		"upgrade":   "TestRunUpgradeRendersSuccessfulFinalJournalMutation",
		"uninstall": "TestRunUninstallDispatch",
		"changelog": "TestChangelogPublicPayloadContracts",
		"version":   "TestRunVersion",
	}
	contractTests := map[string]bool{}
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				contractTests[function.Name.Name] = true
			}
		}
	}
	commands := make(map[string]clispec.Command, len(clispec.Commands))
	for _, command := range clispec.Commands {
		commands[command.Name] = command
		if _, ok := families[command.Name]; !ok {
			t.Errorf("uncontracted top-level command family %q", command.Name)
		}
	}
	for name, contractTest := range families {
		command, ok := commands[name]
		if !ok {
			t.Errorf("contract names missing top-level command family %q", name)
			continue
		}
		if !contractTests[contractTest] {
			t.Errorf("%s result contract test %q is missing", name, contractTest)
		}
		t.Run(name, func(t *testing.T) {
			document, err := command.Help.Document("awf "+name, command.Summary)
			if err != nil {
				t.Fatal(err)
			}
			var want bytes.Buffer
			if err := presentation.Render(&want, document); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := run([]string{"awf", name, "--help"}, &stdout, &stderr); code != 0 || stdout.String() != want.String() || stderr.Len() != 0 {
				t.Fatalf("help exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			stdout.Reset()
			stderr.Reset()
			if code := run([]string{"awf", name, "--presentation-contract-invalid"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.String() != "condition: awf: awf "+name+": unknown flag \"--presentation-contract-invalid\"\n" {
				t.Fatalf("usage exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			stdout.Reset()
			stderr.Reset()
			func() {
				priorGetwd := getwd
				defer func() { getwd = priorGetwd }()
				getwd = func() (string, error) { return "", errors.New("working directory unavailable") }
				if code := run([]string{"awf", name}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.String() != "condition: awf: working directory unavailable\n" {
					t.Fatalf("operational exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
				}
			}()
		})
	}
}

func TestRunGetwdError(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return "", errors.New("boom") })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "render"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on getwd error, got %d", code)
	}
	if out.Len() != 0 || errb.String() != "condition: awf: boom\n" {
		t.Errorf("streams stdout=%q stderr=%q", out.String(), errb.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "bogus"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for unknown command, got %d", code)
	}
	if !strings.Contains(errb.String(), "unknown command") {
		t.Errorf("missing unknown-command text: %q", errb.String())
	}
}

func TestRunAddMissingSkillArg(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "enable"}, &out, &errb); code != 2 {
		t.Fatalf("expected exit 2 for add without skill, got %d", code)
	}
}

func TestRunArgValidation(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"awf", "check", "--bogus"}, "unknown flag"},
		{"unexpected positional", []string{"awf", "render", "extra"}, "unexpected arguments"},
		{"value flag without value", []string{"awf", "context", "--range"}, "needs a value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(c.args, &out, &errb); code != 2 {
				t.Fatalf("expected exit 2, got %d", code)
			}
			if !strings.Contains(errb.String(), c.want) {
				t.Errorf("missing %q in stderr: %q", c.want, errb.String())
			}
		})
	}
}

func TestRunDispatchError(t *testing.T) {
	// render in a bare dir: project.Open fails -> handler error -> exit 1.
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "render"}, &out, &errb); code != 1 {
		t.Fatalf("expected exit 1 on dispatch error, got %d", code)
	}
	if !strings.HasPrefix(errb.String(), "condition: awf:") {
		t.Errorf("expected typed diagnostic, got %q", errb.String())
	}
}

// TestRunDispatchArms drives every switch arm through run() against a scaffolded
// project, covering the dispatch statements. The check children are spelled as
// full argv because they are subcommands, not top-level names: a single-token
// loop could no longer reach them.
func TestRunDispatchArms(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"render", []string{"awf", "render"}},
		{"check repo drift", []string{"awf", "check", "repo", "drift"}},
		{"check repo state", []string{"awf", "check", "repo", "state"}},
		{"list", []string{"awf", "list"}},
		{"upgrade", []string{"awf", "upgrade"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffoldProject(t)
			testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
			var out, errb bytes.Buffer
			if code := run(tc.args, &out, &errb); code != 0 {
				t.Fatalf("%s: expected exit 0, got %d (%s)", tc.name, code, errb.String())
			}
		})
	}
	t.Run("enable", func(t *testing.T) {
		root := t.TempDir()
		awf := filepath.Join(root, ".awf")
		if err := os.MkdirAll(awf, 0o755); err != nil {
			t.Fatal(err)
		}
		// skills: [] so a fresh skill can be added.
		cfg := strings.Replace(minimalYAML, "skills: [tdd]", "skills: []", 1)
		if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := initializeProject(testContext(t), root, io.Discard); err != nil {
			t.Fatal(err)
		}
		testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		if code := run([]string{"awf", "enable", "skill", "tdd"}, &out, &errb); code != 0 {
			t.Fatalf("add: expected exit 0, got %d (%s)", code, errb.String())
		}
	})
	t.Run("init", func(t *testing.T) {
		root := t.TempDir()
		testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
		var out, errb bytes.Buffer
		if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
			t.Fatalf("init: expected exit 0, got %d (%s)", code, errb.String())
		}
	})
}

// TestHandlersOnBareDirError covers each handler's project.Open error return.
func TestHandlersOnBareDirError(t *testing.T) {
	ctx := testContext(t)
	bare := func(t *testing.T) string { return t.TempDir() }
	t.Run("check", func(t *testing.T) {
		if err := runCheck(ctx, bare(t), io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("list", func(t *testing.T) {
		if err := runList(ctx, bare(t), "", io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("new", func(t *testing.T) {
		if err := runNew(ctx, bare(t), "adr", []string{"x"}, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("enable", func(t *testing.T) {
		if err := runEnable(ctx, bare(t), "skill", "tdd", false, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
	t.Run("disable", func(t *testing.T) {
		if err := runDisable(ctx, bare(t), "skill", "tdd", false, false, io.Discard); err == nil {
			t.Error("expected Open error")
		}
	})
}

func TestRunCheckErrorPaths(t *testing.T) {
	ctx := testContext(t)
	t.Run("stale-schema", func(t *testing.T) {
		root := t.TempDir()
		claude := filepath.Join(root, ".claude")
		if err := os.MkdirAll(claude, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// check is Gated: the driver refuses a stale schema before the handler.
		var out, errb bytes.Buffer
		if code := runAt(t, root, []string{"awf", "check"}, &out, &errb); code != 1 {
			t.Errorf("expected the driver to refuse check on stale schema, got %d", code)
		}
	})
	t.Run("check-error-malformed-adr", func(t *testing.T) {
		// A malformed ADR makes p.Check() (INDEX.md generation) error.
		root := scaffoldProject(t)
		adrDir := filepath.Join(root, "docs", "decisions")
		if err := os.MkdirAll(adrDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(adrDir, "0001-x.md"), []byte("---\n: : bad yaml : :\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := runCheck(ctx, root, io.Discard); err == nil {
			t.Error("expected check error on a malformed ADR")
		}
	})
}

func TestRunListSidecarError(t *testing.T) {
	ctx := testContext(t)
	// A malformed sidecar for the enabled skill makes Sidecar() error.
	root := scaffoldProject(t)
	skillsDir := filepath.Join(root, ".awf", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "tdd.yaml"), []byte("data: [not, a, map]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runList(ctx, root, "", io.Discard); err == nil {
		t.Error("expected Sidecar parse error")
	}
}

func TestRunSyncSyncError(t *testing.T) {
	ctx := testContext(t)
	// A directory squatting on a rendered output path makes p.SyncReport() fail.
	root := scaffoldProject(t)
	out := filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil { // SKILL.md is now a directory
		t.Fatal(err)
	}
	if err := runSync(ctx, root, io.Discard); err == nil {
		t.Error("expected Sync error when an output path is a directory")
	}
}

func TestRunInitSyncError(t *testing.T) {
	ctx := testContext(t)
	// Config exists (skip scaffold); a squatting output dir makes the inner
	// runSync fail, covering runInit's runSync error return.
	root := scaffoldProject(t)
	out := filepath.Join(root, ".claude", "skills", "example-tdd", "SKILL.md")
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runInit(ctx, root, false, false, nil, "", io.Discard); err == nil {
		t.Error("expected runInit to surface the sync error")
	}
}

func TestRunUpgradeLegacyAdopterRendersAndChecksClean(t *testing.T) {
	ctx := testContext(t)
	// A legacy single-file project migrates to the tree layout, covering the
	// applied-migrations loop and the terminal sync.
	root := gitfixture.InitRepo(t).Root()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "prefix: example\nvars:\n  testCmd: go test ./...\n  gateCmd: make gate\nskills: {}\nagents: {}\n"
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: "0.1.0", Files: map[string]manifest.Entry{}}).Save(filepath.Join(claude, "awf.lock")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runUpgrade(ctx, root, &out); err != nil {
		t.Fatalf("runUpgrade legacy: %v", err)
	}
	if !strings.Contains(out.String(), "status: completed") || !strings.Contains(out.String(), "awf-dir-relocation: moved .claude/awf to .awf") || strings.Contains(out.String(), "note:") {
		t.Errorf("expected structured migration mutation with production relocation evidence, got %q", out.String())
	}
	for _, path := range []string{".awf/config.yaml", ".awf/awf.lock"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Errorf("upgraded project missing %s: %v", path, err)
		}
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read rendered AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agents), "# example Agent Guide") {
		t.Errorf("rendered AGENTS.md missing stable heading: %q", agents)
	}
	if err := runCheckRepo(ctx, root, io.Discard); err != nil {
		t.Fatalf("repository check after upgrade: %v", err)
	}
}

// A schema-7 config the ADR-0081 closure validation refuses is repaired by
// awf upgrade: close-enabled-set closes the enabled set, then the terminal
// sync opens it cleanly.
func TestRunUpgradeRepairsUnclosedConfig(t *testing.T) {
	ctx := testContext(t)
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, root, "prefix: example\nvars: {}\nskills: [brainstorming]\nagents: []\n")
	lock := &manifest.Lock{SchemaVersion: 7, Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(ctx, root, io.Discard); err == nil {
		t.Fatal("pre-upgrade check should refuse (schema gate)")
	}
	var out bytes.Buffer
	if err := runUpgrade(ctx, root, &out); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if strings.Contains(out.String(), `close-enabled-set: enabled skill`) {
		t.Errorf("advisory workflow catalog unexpectedly closed requirements: %q", out.String())
	}
	// The migration's resident-root rewrite is validated by its own focused tests;
	// this fixture has no generated resident files to compare after the upgrade.
}

func TestRunUpgradeMigrationError(t *testing.T) {
	ctx := testContext(t)
	// A legacy config that fails to parse makes the migration error.
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte(": : not valid : :\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: "0.1.0", Files: map[string]manifest.Entry{}}
	if err := lock.Save(filepath.Join(claude, "awf.lock")); err != nil {
		t.Fatal(err)
	}
	if err := runUpgrade(ctx, root, io.Discard); err == nil {
		t.Error("expected migration error for a malformed legacy config")
	}
}

// invariant: tooling/cli:single-os-exit (TestNoOsExitOutsideMain)
func TestNoOsExitOutsideMain(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "os.Exit") {
				continue
			}
			// The sole permitted os.Exit is main's one-line wrapper.
			if f == "main.go" && strings.Contains(line, "func main()") {
				continue
			}
			t.Errorf("%s:%d: os.Exit outside main's wrapper: %s", f, i+1, strings.TrimSpace(line))
		}
	}
}

func TestGateRejectsStaleSchema(t *testing.T) {
	ctx := testContext(t)
	// A legacy single-file layout (.claude/awf.yaml, no tree config) reports
	// generation 0 -> GateState "gate".
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, "awf.yaml"), []byte("prefix: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gate(ctx, root); err == nil {
		t.Fatal("expected gate to reject stale schema")
	}
	// render is Gated: the driver surfaces the same gate error before the handler.
	var out, errb bytes.Buffer
	if code := runAt(t, root, []string{"awf", "render"}, &out, &errb); code != 1 {
		t.Errorf("expected the driver to fail render on stale schema, got %d", code)
	}
}

func TestProbeCollisionsOpenError(t *testing.T) {
	ctx := testContext(t)
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: [bad\n")
	if _, err := probeCollisions(testContext(t), root); err == nil {
		t.Fatal("expected config open error")
	}
	if err := runInit(ctx, root, false, false, nil, "", io.Discard); err == nil {
		t.Fatal("expected init probe error")
	}
}

func TestRunInitOnExistingConfigSkipsScaffold(t *testing.T) {
	ctx := testContext(t)
	// Pre-existing config -> scaffold branch is skipped; init still syncs.
	root := scaffoldProject(t)
	if err := runInit(ctx, root, false, false, nil, "", io.Discard); err != nil {
		t.Fatalf("runInit on existing config: %v", err)
	}
}

func TestRunListPrintsSkills(t *testing.T) {
	ctx := testContext(t)
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runList(ctx, root, "", &out); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(out.String(), "tdd") {
		t.Errorf("expected tdd in listing, got %q", out.String())
	}
}

// invariant: tooling/cli:upgrade-always-syncs (TestRunUpgradeAlreadyCurrentStillSyncs)
func TestRunUpgradeAlreadyCurrentStillSyncs(t *testing.T) {
	ctx := testContext(t)
	root := scaffoldProject(t)
	var out bytes.Buffer
	if err := runUpgrade(ctx, root, &out); err != nil {
		t.Fatalf("runUpgrade: %v", err)
	}
	if !strings.Contains(out.String(), "migration changes:\n      config schema already current") {
		t.Errorf("expected the schema-current fact, got %q", out.String())
	}
	// The zero-migrations path must still sync: a same-schema binary bump
	// re-renders every managed file and re-pins the bootstrap (ADR-0085).
	if !strings.Contains(out.String(), "status: completed") || !strings.Contains(out.String(), "continue with the rendered project state") {
		t.Errorf("expected the schema-current upgrade to run a sync, got %q", out.String())
	}
}

// invariant: tooling/init-and-enablement:init-collision-guard (TestInitGuardBlocksAndForceOverrides)
func TestInitGuardBlocksAndForceOverrides(t *testing.T) {
	forceNonInteractive(t)
	root := t.TempDir()
	// A pre-existing, non-awf CLAUDE.md is a collision.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail on collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	// Nothing written: the scaffolded config tree was rolled back.
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("expected .awf to be rolled back")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md")); string(b) != "mine\n" {
		t.Fatalf("CLAUDE.md clobbered: %q", b)
	}
	// --force backs up the colliding file, then overwrites and completes.
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code != 0 {
		t.Fatalf("init --force failed: %s", errb.String())
	}
	// The original is preserved at <path>.awf-bak.
	// invariant: tooling/init-and-enablement:init-force-backs-up (TestInitGuardBlocksAndForceOverrides)
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak")); string(b) != "mine\n" {
		t.Fatalf("CLAUDE.md.awf-bak = %q, want original %q", b, "mine\n")
	}
	// And the live file was overwritten with managed content.
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md")); string(b) == "mine\n" {
		t.Fatalf("CLAUDE.md should have been overwritten, still %q", b)
	}
	initForceMutation := fmt.Sprintf("status: initialization completed\n\nmutation:\n  identity:\n    config: %s/.awf/config.yaml\n    config action: scaffolded\n  changes:\n    backups:\n      CLAUDE.md to CLAUDE.md.awf-bak\n  notes:\n    agent adr-reviewer references unset vars: invariantTestPath; set a value, or delete the key to accept the generic prose\n    agent implementer references unset vars: gateCmd; set a value, or delete the key to accept the generic prose\n    agents-doc references unset vars: checkCmd, gateCmd, testCmd; set a value, or delete the key to accept the generic prose\n    doc workflow references unset vars: checkCmd, gateCmd, gateCmdFull, testCmd; set a value, or delete the key to accept the generic prose\n    hooks commit-msg references unset vars: commitGateCmd; set a value, or delete the key to accept the generic prose\n    hooks pre-commit references unset vars: checkCmd, gateCmd; set a value, or delete the key to accept the generic prose\n    hooks pre-merge-commit references unset vars: checkCmd; set a value, or delete the key to accept the generic prose\n    hooks pre-push references unset vars: checkCmd, gateCmd, gateCmdFull; set a value, or delete the key to accept the generic prose\n    plans-template references unset vars: gateCmd; set a value, or delete the key to accept the generic prose\n    skill adr-lifecycle references unset vars: activeMdRegenCmd, gateCmd; set a value, or delete the key to accept the generic prose\n    skill executing-plans references unset vars: gateCmd; set a value, or delete the key to accept the generic prose\n    skill proposing-adr references unset vars: activeMdRegenCmd; set a value, or delete the key to accept the generic prose\n    skill retrospective references unset vars: gateCmd, invariantTestPath; set a value, or delete the key to accept the generic prose\n    skill reviewing-impl references unset vars: gateCmd; set a value, or delete the key to accept the generic prose\n    skill subagent-driven-development references unset vars: gateCmd; set a value, or delete the key to accept the generic prose\n    skill writing-plans references unset vars: gateCmd; set a value, or delete the key to accept the generic prose\n    AGENTS.md has unauthored stub content: sections at stub default: identity\n  next actions:\n    step 1: continue with the rendered project state\n    step 2: fill the Identity section at .awf/parts/agents-doc/identity.md, then run awf render\n    step 3: set still-empty vars in .awf/config.yaml (the notes above list what each artifact misses), then run awf render\n    step 4: wire rendered hook payloads under .awf/hooks/ into git hooks you own (see the workflow doc's local-hooks section); awf never activates hooks itself\n    step 5: commit .awf/ and the rendered files together\n", root)
	if out.String() != initForceMutation {
		t.Errorf("init --force output = %q, want exact %q", out.String(), initForceMutation)
	}
	// Regression: init delegates its backup to the chained sync (one BackupFile path,
	// ADR-0035), so the colliding file is backed up exactly once - no double-backup.
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md.awf-bak.1")); !os.IsNotExist(err) {
		t.Error("expected exactly one backup; CLAUDE.md.awf-bak.1 should not exist")
	}
}

func TestInitRollbackPreservesExistingAwf(t *testing.T) {
	ctx := testContext(t)
	root := t.TempDir()
	// Pre-existing authored .awf/ content but no config.yaml -> init scaffolds config,
	// then a collision (non-managed CLAUDE.md) forces a refusal + rollback.
	part := filepath.Join(root, ".awf", "skills", "parts", "foo", "extra.md")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("hand-authored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(ctx, root, false, false, nil, "", io.Discard); err == nil {
		t.Fatal("expected init to refuse on collision")
	}
	// The scaffolded config.yaml is rolled back...
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Error("config.yaml should have been removed on rollback")
	}
	// ...but the pre-existing authored content survives.
	if _, err := os.Stat(part); err != nil {
		t.Errorf("pre-existing .awf content must be preserved, got: %v", err)
	}
}

func TestInitForceBackupDoesNotClobberPriorBak(t *testing.T) {
	root := t.TempDir()
	// A colliding CLAUDE.md plus a pre-existing backup from an earlier --force.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md.awf-bak"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code != 0 {
		t.Fatalf("init --force: %s", errb.String())
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak")); string(b) != "v1\n" {
		t.Errorf("prior .awf-bak clobbered: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md.awf-bak.1")); string(b) != "v2\n" {
		t.Errorf("CLAUDE.md.awf-bak.1 = %q, want v2", b)
	}
}

func TestInitIdempotentReinitNoCollision(t *testing.T) {
	root := scaffoldProject(t)
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("first init failed: %s", errb.String())
	}
	// Re-init over the now-managed tree: every planned path is in the prior lock,
	// so p.InitCollisions skips them all and init proceeds without --force.
	out.Reset()
	errb.Reset()
	if code := run([]string{"awf", "init"}, &out, &errb); code != 0 {
		t.Fatalf("re-init failed: %s", errb.String())
	}
}

func TestInitCollisionsOpenError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".awf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unknown field → strict config.Load fails → project.Open errors inside runInit.
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("bogusField: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: project.Version, SchemaVersion: 14, Files: map[string]manifest.Entry{}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail when project.Open errors")
	}
	// --force skips the probe, so the same malformed config now fails at
	// runInit's own post-scaffold project.Open - keeping that branch covered.
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code == 0 {
		t.Fatal("expected init --force to fail when project.Open errors")
	}
}

func TestInitAbortsWhenInitCollisionsFails(t *testing.T) {
	root := t.TempDir()
	// An existing permanent project can still have a malformed ADR. --force skips
	// the probe so runInit's own p.InitCollisions call forwards that deterministic
	// planning error.
	testsupport.WriteAwfConfig(t, root, minimalYAML)
	if err := (&manifest.Lock{AWFVersion: project.Version, SchemaVersion: 14, Files: map[string]manifest.Entry{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	dd := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(dd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dd, "0099-bad.md"), []byte("---\nstatus: [unclosed\n---\n# Bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--force"}, &out, &errb); code == 0 {
		t.Fatal("expected init to fail when p.InitCollisions errors")
	}
}

func TestSyncReportsIndexOwnershipTakeover(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	if err := os.MkdirAll(awf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(awf, "config.yaml"), []byte(minimalYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	// Foreign ADR index present before any sync (no lock yet).
	adrDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(adrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adrDir, "INDEX.md"), []byte("hand index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&manifest.Lock{AWFVersion: project.Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "render"}, &out, &errb); code != 0 {
		t.Fatalf("sync: %s", errb.String())
	}
	const indexTakeoverOutput = "status: completed\n\nmutation:\n  changes:\n    backups:\n      docs/decisions/INDEX.md to docs/decisions/INDEX.md.awf-bak\n    outputs:\n      added .awf/efforts/.gitignore\n      added .awf/worktrees/.gitignore\n      added .claude/skills/example-tdd/SKILL.md\n      added AGENTS.md\n      added CLAUDE.md\n      added docs/agents-md-standard.md\n      added docs/config-reference.md\n      added docs/decisions/INDEX.md\n      added docs/decisions/README.md\n      added docs/decisions/template.md\n      added docs/doc-standard.md\n      added docs/maintainable-code-design.md\n      added docs/plans/README.md\n      added docs/plans/template.md\n      added docs/workflow.md\n      added docs/working-with-awf.md\n  notes:\n    awf now generates docs/decisions/INDEX.md; retire any external generator for it\n  next actions:\n    step 1: continue with the rendered project state\n"
	if out.String() != indexTakeoverOutput {
		t.Errorf("index takeover stdout = %q, want %q", out.String(), indexTakeoverOutput)
	}
}

// A collision refuses BEFORE any prompt: with a colliding AGENTS.md and an
// interactive stdin, init exits without emitting a single prompt line and
// without creating .awf/.
func TestInitCollisionProbeRefusesBeforePrompts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	testsupport.SwapVar(t, &isInteractive, func() bool { return true })
	testsupport.SwapVar(t, &stdin, io.Reader(strings.NewReader("SHOULD-NOT-BE-CONSUMED\n")))
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init"}, &out, &errb); code == 0 {
		t.Fatal("expected init to refuse on collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	if out.String() != "" {
		t.Errorf("prompt text emitted before the collision refusal:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf")); !os.IsNotExist(err) {
		t.Errorf(".awf/ should not exist after a probe refusal (err=%v)", err)
	}
}

// A trim answer can enable a non-core artifact the curated-core probe set does
// not cover: the probe passes, and the accurate post-answer check still
// refuses and rolls the scaffolded config back. (The leaves-only trim derives
// zero agents under ADR-0081, so the selection is closure-valid.)
func TestInitPostAnswerCollisionAfterProbePasses(t *testing.T) {
	root := t.TempDir()
	skillPath := filepath.Join(root, ".claude", "skills", filepath.Base(root)+"-tdd", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	forceNonInteractive(t)
	var out, errb bytes.Buffer
	if code := run([]string{"awf", "init", "--set", "skills=tdd"}, &out, &errb); code == 0 {
		t.Fatal("expected init to refuse on the post-answer collision")
	}
	if !strings.Contains(errb.String(), "refusing to overwrite") {
		t.Fatalf("stderr = %q", errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "config.yaml")); !os.IsNotExist(err) {
		t.Error("scaffolded config should have been rolled back")
	}
}

func TestInitAndUpgradeRefusePreTrackingAuthority(t *testing.T) {
	ctx := testContext(t)
	for _, tc := range []struct{ name, lock, want string }{
		{"missing", "", "bridge release"},
		{"invalid", `{`, "restore .awf/awf.lock"},
		{"bridge", `{"awfVersion":"0.1.0","schemaVersion":30,"files":{},"bridgeAttestation":{"version":1,"preparedHead":"h","treeDigest":"sha256:x","adrFormatV1From":1,"legacyADRGaps":[]}}`, "bridge release"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteAwfConfig(t, root, minimalYAML)
			if tc.lock != "" {
				testsupport.WriteFile(t, config.LockPath(root), tc.lock)
			}
			if err := runInit(ctx, root, true, false, nil, "", io.Discard); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("init=%v", err)
			}
			if tc.name == "missing" {
				if err := runUpgrade(ctx, root, io.Discard); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("upgrade=%v", err)
				}
			}
		})
	}
}
