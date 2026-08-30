package migrate

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func writeLock(t *testing.T, root string, schema int) {
	t.Helper()
	testsupport.WriteFile(t, config.ConfigPath(root), "prefix: test\n")
	if err := (&manifest.Lock{AWFVersion: "0.45.0", SchemaVersion: schema, Files: map[string]manifest.Entry{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	got := map[string][]byte{}
	if err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			b, readErr := os.ReadFile(p)
			if readErr != nil {
				return readErr
			}
			rel, relErr := filepath.Rel(root, p)
			if relErr != nil {
				return relErr
			}
			got[rel] = b
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

func assertSnapshot(t *testing.T, root string, want map[string][]byte) {
	t.Helper()
	got := snapshot(t, root)
	if len(got) != len(want) {
		t.Fatalf("file count changed: %#v", got)
	}
	for p, b := range want {
		if !slices.Equal(got[p], b) {
			t.Fatalf("%s changed", p)
		}
	}
}

// invariant: config/migrations-and-locks:upgrade-gate (TestBelowSchema50RefusesBeforePlanningOrMutation)
func TestBelowSchema50RefusesBeforePlanningOrMutation(t *testing.T) {
	original := registry
	called := false
	registry = append(registry, Migration{To: 51, Name: "synthetic future", Build: func(context.Context, *ProposedTree, *Changes) ([]FileMutation, error) {
		called = true
		return []FileMutation{{Path: ".awf/future.yaml", Content: []byte("future\n"), Mode: 0o600}}, nil
	}})
	t.Cleanup(func() { registry = original })

	for _, schema := range []int{0, 49} {
		t.Run("schema-"+strconv.Itoa(schema), func(t *testing.T) {
			root := t.TempDir()
			writeLock(t, root, schema)
			before := snapshot(t, root)
			applied, changes, mutations, err := Build(context.Background(), root)
			if !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
				t.Fatalf("Build() error = %v, want typed unsupported-source refusal", err)
			}
			if called || len(applied) != 0 || len(changes) != 0 || len(mutations) != 0 {
				t.Fatalf("refusal planned migration: called=%t applied=%v changes=%v mutations=%v", called, applied, changes, mutations)
			}
			assertSnapshot(t, root, before)
		})
	}
}

func TestSchema50IsCurrentWithoutAppliedMigrationOrMutation(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 50)
	before := snapshot(t, root)
	applied, changes, mutations, err := Build(context.Background(), root)
	if err != nil || len(applied) != 0 || len(changes) != 0 || len(mutations) != 0 {
		t.Fatalf("Build() = applied=%v changes=%v mutations=%v err=%v", applied, changes, mutations, err)
	}
	assertSnapshot(t, root, before)
}

func TestAheadSchemasRefuse(t *testing.T) {
	for _, schema := range []int{51, 99} {
		t.Run("schema-"+strconv.Itoa(schema), func(t *testing.T) {
			root := t.TempDir()
			writeLock(t, root, schema)
			before := snapshot(t, root)
			if _, _, _, err := Build(context.Background(), root); !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
				t.Fatalf("Build() error = %v, want typed unsupported-source refusal", err)
			}
			assertSnapshot(t, root, before)
		})
	}
}

func TestRegistryIsOnlyTheSchema50NoOpAndHasNoReachablePre50Migrator(t *testing.T) {
	if LiveSchemaFloor != 50 || Current() != 50 {
		t.Fatalf("live schema range = %d..%d, want 50..50", LiveSchemaFloor, Current())
	}
	if len(registry) != 1 || registry[0].To != 50 || registry[0].Name != "supported-schema-50" || registry[0].Build != nil {
		t.Fatalf("registry = %#v, want only schema-50 no-op", registry)
	}
	for _, migration := range registry {
		if migration.To < 50 || migration.Build != nil {
			t.Fatalf("concrete pre-50 migrator remains reachable: %#v", migration)
		}
	}
}

// invariant: config/migrations-and-locks:migration-ordering (TestFutureMigrationsPlanInOrderWithoutWriting)
func TestFutureMigrationsPlanInOrderWithoutWriting(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, Current())
	original := registry
	var calls []string
	registry = append(registry,
		Migration{To: Current() + 1, Name: "first future", Build: func(context.Context, *ProposedTree, *Changes) ([]FileMutation, error) {
			calls = append(calls, "first")
			return []FileMutation{{Path: ".awf/future.yaml", Content: []byte("future\n"), Mode: 0o600}}, nil
		}},
		Migration{To: Current() + 2, Name: "second future", Build: func(context.Context, *ProposedTree, *Changes) ([]FileMutation, error) {
			calls = append(calls, "second")
			return []FileMutation{{Path: ".awf/retired.yaml", Remove: true}}, nil
		}},
	)
	t.Cleanup(func() { registry = original })

	applied, _, mutations, err := Build(context.Background(), root)
	if err != nil || !slices.Equal(applied, []string{"first future", "second future"}) || !slices.Equal(calls, []string{"first", "second"}) {
		t.Fatalf("Build() applied=%v calls=%v err=%v", applied, calls, err)
	}
	if len(mutations) != 2 || mutations[0].Path != ".awf/future.yaml" || mutations[1].Path != ".awf/retired.yaml" || !mutations[1].Remove {
		t.Fatalf("mutations = %#v", mutations)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "future.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Build wrote planned file: %v", err)
	}
}

func TestOrderedMigrationStepsReadAndCoalesceTheProposedTree(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, Current())
	original := registry
	registry = append(registry,
		Migration{To: Current() + 1, Name: "write first image", Build: func(_ context.Context, tree *ProposedTree, _ *Changes) ([]FileMutation, error) {
			if _, _, err := tree.Read(".awf/future.yaml"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("initial proposed read error = %v, want not-exist", err)
			}
			return []FileMutation{{Path: ".awf/future.yaml", Content: []byte("first\n"), Mode: 0o640}}, nil
		}},
		Migration{To: Current() + 2, Name: "remove first image", Build: func(_ context.Context, tree *ProposedTree, _ *Changes) ([]FileMutation, error) {
			contents, mode, err := tree.Read(".awf/future.yaml")
			if err != nil || string(contents) != "first\n" || mode != 0o640 {
				t.Fatalf("proposed read = %q mode=%#o err=%v", contents, mode, err)
			}
			return []FileMutation{{Path: ".awf/future.yaml", Remove: true}}, nil
		}},
		Migration{To: Current() + 3, Name: "replace removed image", Build: func(_ context.Context, tree *ProposedTree, _ *Changes) ([]FileMutation, error) {
			if _, _, err := tree.Read(".awf/future.yaml"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("removed proposed read error = %v, want not-exist", err)
			}
			return []FileMutation{{Path: ".awf/future.yaml", Content: []byte("final\n"), Mode: 0o600}}, nil
		}},
	)
	t.Cleanup(func() { registry = original })

	_, _, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 1 || mutations[0].Path != ".awf/future.yaml" || string(mutations[0].Content) != "final\n" || mutations[0].Mode != 0o600 {
		t.Fatalf("coalesced mutations = %#v", mutations)
	}
}

func TestBuildValidatesPlannedPathsAgainstTheConfinedTree(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, Current())
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim")
	if err := os.WriteFile(victim, []byte("outside\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".awf", "escape")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	original := registry
	registry = append(registry, Migration{To: Current() + 1, Name: "escaping plan", Build: func(context.Context, *ProposedTree, *Changes) ([]FileMutation, error) {
		return []FileMutation{{Path: ".awf/escape/victim", Content: []byte("changed\n"), Mode: 0o600}}, nil
	}})
	t.Cleanup(func() { registry = original })

	if _, _, _, err := Build(context.Background(), root); err == nil {
		t.Fatal("planning accepted a symlink-ancestor escape")
	}
	contents, readErr := os.ReadFile(victim)
	info, statErr := os.Stat(victim)
	if readErr != nil || statErr != nil || string(contents) != "outside\n" || info.Mode().Perm() != 0o640 {
		t.Fatalf("outside victim changed = %q mode=%v errors=%v", contents, info, errors.Join(readErr, statErr))
	}
}

func TestSupportedMigrationClassifierAndFailure(t *testing.T) {
	if got := gateStateFor(51, 50, []int{50}); got != "ahead" {
		t.Fatalf("ahead state = %q", got)
	}
	if got := gateStateFor(50, 50, []int{50}); got != "ok" {
		t.Fatalf("current state = %q", got)
	}
	if got := gateStateFor(50, 51, []int{50, 51}); got != "gate" {
		t.Fatalf("migration state = %q", got)
	}
	if got := gateStateFor(50, 51, []int{50}); got != "autobump" {
		t.Fatalf("autobump state = %q", got)
	}

	original := registry
	failure := errors.New("future migration failed")
	registry = append(registry, Migration{To: Current() + 1, Name: "future", Build: func(context.Context, *ProposedTree, *Changes) ([]FileMutation, error) { return nil, failure }})
	t.Cleanup(func() { registry = original })
	if err := CheckLiveGeneration(50); err == nil || !strings.Contains(err.Error(), "requires migration") {
		t.Fatalf("CheckLiveGeneration() error = %v, want upgrade requirement", err)
	}
	root := t.TempDir()
	writeLock(t, root, 50)
	if _, _, _, err := Build(context.Background(), root); !errors.Is(err, failure) || !strings.Contains(err.Error(), `migration "future"`) {
		t.Fatalf("Build() error = %v, want named migration failure", err)
	}
}

func TestBuildRejectsInvalidMigrationRegistry(t *testing.T) {
	original := registry
	t.Cleanup(func() { registry = original })
	for _, tc := range []struct {
		name string
		set  []Migration
		want string
	}{
		{"empty", nil, "begin at supported floor"},
		{"wrong floor", []Migration{{To: 51}}, "begin at supported floor"},
		{"unordered", []Migration{{To: 50}, {To: 52}, {To: 51}}, "strictly ascending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry = tc.set
			if _, _, _, err := Build(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid registry error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRetiredConfigLayoutsHavePresenceOnlyProductionConsumers(t *testing.T) {
	allowedCalls := map[string]map[string]bool{
		"retiredLayout":           {"filepath.Join": true, "os.Stat": true, "errors.Is": true, "fmt.Errorf": true},
		"ProjectPresent":          {"config.ConfigPath": true, "config.LockPath": true, "filepath.Join": true, "os.Stat": true, "errors.Is": true, "fmt.Errorf": true},
		"ProjectPresentFromFiles": {"has": true},
	}
	presenceOnly := func(function *ast.FuncDecl) bool {
		valid := true
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch target := call.Fun.(type) {
			case *ast.Ident:
				name = target.Name
			case *ast.SelectorExpr:
				if owner, ok := target.X.(*ast.Ident); ok {
					name = owner.Name + "." + target.Sel.Name
				}
			}
			if !allowedCalls[function.Name.Name][name] {
				valid = false
			}
			return valid
		})
		return valid
	}
	synthetic, err := parser.ParseFile(token.NewFileSet(), "synthetic.go", `package migrate; func retiredLayout(string) { os.ReadFile(".claude/awf.yaml") }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if presenceOnly(synthetic.Decls[0].(*ast.FuncDecl)) {
		t.Fatal("retired-layout census accepted synthetic representation read")
	}
	var found int
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(rel string, body []byte) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || (!strings.Contains(string(body), `".claude"`) && !strings.Contains(string(body), `".claude/awf`)) {
			return
		}
		if rel != "internal/migrate/migrate.go" {
			t.Fatalf("retired config layout has production consumer %s", rel)
		}
		file, err := parser.ParseFile(token.NewFileSet(), rel, body, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING || (!strings.Contains(literal.Value, `.claude`) && !strings.Contains(literal.Value, `awf.yaml`)) {
					return true
				}
				found++
				if _, ok := allowedCalls[function.Name.Name]; !ok || !presenceOnly(function) {
					t.Fatalf("retired config layout representation interpreted by %s", function.Name.Name)
				}
				return true
			})
		}
	})
	if found == 0 {
		t.Fatal("retired layout presence census found no production recognizer")
	}
}

func TestProjectPresentAndGenerationPreserveControlPathStatFailures(t *testing.T) {
	root := t.TempDir()
	path := config.ConfigPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(path), path); err != nil {
		t.Fatal(err)
	}
	if present, err := ProjectPresent(root); present || !errors.Is(err, syscall.ELOOP) {
		t.Fatalf("ProjectPresent = %t, %v; want false, stat loop", present, err)
	}
	if _, err := Generation(root); !errors.Is(err, syscall.ELOOP) {
		t.Fatalf("Generation error = %v, want stat loop", err)
	}
}
