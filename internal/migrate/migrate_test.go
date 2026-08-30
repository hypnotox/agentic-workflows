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
	"strings"
	"syscall"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func writeLock(t *testing.T, root string, schema int) {
	t.Helper()
	testsupport.WriteFile(t, config.ConfigPath(root), "prefix: test\n")
	if err := (&manifest.Lock{AWFVersion: "0.39.2", SchemaVersion: schema, Files: map[string]manifest.Entry{}}).Save(config.LockPath(root)); err != nil {
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

// invariant: config/migrations-and-locks:upgrade-gate (TestUnsupportedSourcesRefuseWithoutMutation)
func TestUnsupportedSourcesRefuseWithoutMutation(t *testing.T) {
	cases := []struct {
		name, path string
		schema     int
	}{
		{"legacy single file", ".claude/awf.yaml", 0}, {"retired tree", ".claude/awf/config.yaml", 0},
		{"schema below floor", ".awf/config.yaml", LiveSchemaFloor - 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.schema == 0 {
				testsupport.WriteFile(t, filepath.Join(root, tc.path), "malformed: [")
			} else {
				writeLock(t, root, tc.schema)
			}
			before := snapshot(t, root)
			_, _, _, err := Build(context.Background(), root)
			if !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
				t.Fatalf("Upgrade error=%v", err)
			}
			assertSnapshot(t, root, before)
		})
	}
}

// invariant: config/migrations-and-locks:migration-ordering (TestSchema47MigrationAndFutureOrdering)
func TestSchema47MigrationAndFutureOrdering(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, LiveSchemaFloor)
	applied, _, _, err := Build(context.Background(), root)
	if err != nil || !slices.Equal(applied, []string{retireRelevanceMetadataName, retireClaimProvenanceMetadataName, retireWorkflowConfigName, retirePitfallRelationsName}) {
		t.Fatalf("schema 46: applied=%v err=%v", applied, err)
	}
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
	if err != nil || !slices.Equal(applied, []string{retireRelevanceMetadataName, retireClaimProvenanceMetadataName, retireWorkflowConfigName, retirePitfallRelationsName, "first future", "second future"}) || !slices.Equal(calls, []string{"first", "second"}) {
		t.Fatalf("future seam: applied=%v calls=%v err=%v", applied, calls, err)
	}
	if len(mutations) != 2 || mutations[0].Path != ".awf/future.yaml" || mutations[1].Path != ".awf/retired.yaml" || !mutations[1].Remove {
		t.Fatalf("future mutations = %#v", mutations)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".awf", "future.yaml")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Build mutated future file: %v", statErr)
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
	if _, statErr := os.Stat(filepath.Join(root, ".awf", "future.yaml")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("planning mutated project: %v", statErr)
	}
}

func TestBuildValidatesPlannedPathsAgainstTheConfinedTree(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, LiveSchemaFloor)
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim")
	if err := os.WriteFile(victim, []byte("outside\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".awf", "escape")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	original := registry
	registry = append(registry, Migration{To: LiveSchemaFloor + 1, Name: "escaping plan", Build: func(context.Context, *ProposedTree, *Changes) ([]FileMutation, error) {
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

func TestAheadSchemaRefuses(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, Current()+1)
	_, _, _, err := Build(context.Background(), root)
	if !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("error=%v", err)
	}
}

func TestSupportedMigrationClassifierAndFailure(t *testing.T) {
	if got := gateStateFor(LiveSchemaFloor+1, LiveSchemaFloor, []int{LiveSchemaFloor}); got != "ahead" {
		t.Fatalf("ahead state = %q", got)
	}
	if got := gateStateFor(LiveSchemaFloor, LiveSchemaFloor, []int{LiveSchemaFloor}); got != "ok" {
		t.Fatalf("current state = %q", got)
	}
	if got := gateStateFor(LiveSchemaFloor, LiveSchemaFloor+1, []int{LiveSchemaFloor, LiveSchemaFloor + 1}); got != "gate" {
		t.Fatalf("migration state = %q", got)
	}
	if got := gateStateFor(LiveSchemaFloor, LiveSchemaFloor+1, []int{LiveSchemaFloor}); got != "autobump" {
		t.Fatalf("autobump state = %q", got)
	}

	original := registry
	failure := errors.New("future migration failed")
	registry = append(registry, Migration{To: Current() + 1, Name: "future", Build: func(context.Context, *ProposedTree, *Changes) ([]FileMutation, error) { return nil, failure }})
	t.Cleanup(func() { registry = original })

	err := CheckLiveGeneration(LiveSchemaFloor)
	var required *UpgradeRequiredError
	if !errors.As(err, &required) || !strings.Contains(required.Error(), "requires migration") {
		t.Fatalf("CheckLiveGeneration() error = %v, want upgrade requirement", err)
	}
	root := t.TempDir()
	writeLock(t, root, Current()-1)
	_, _, _, err = Build(context.Background(), root)
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), `migration "future"`) {
		t.Fatalf("Upgrade() error = %v, want named migration failure", err)
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
		{"wrong floor", []Migration{{To: LiveSchemaFloor + 1}}, "begin at supported floor"},
		{"unordered", append(append([]Migration{}, original...), Migration{To: LiveSchemaFloor + 2}, Migration{To: LiveSchemaFloor + 1}), "strictly ascending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry = tc.set
			if _, _, _, err := Build(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid registry error = %v, want %q", err, tc.want)
			}
		})
	}
}

// invariant: config/configuration:awf-config-root (TestRetiredConfigLayoutsHavePresenceOnlyProductionConsumers)
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
	// This mutation-style specimen proves the census rejects a retired-layout
	// representation read rather than merely counting its path literal.
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
				if _, ok := allowedCalls[function.Name.Name]; !ok {
					t.Fatalf("retired config layout interpreted by %s", function.Name.Name)
				}
				if !presenceOnly(function) {
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

func TestProjectPresentPreservesControlPathStatFailures(t *testing.T) {
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
}

func TestGenerationPreservesControlPathStatFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		path func(string) string
	}{
		{"current config", config.ConfigPath},
		{"retired layout", func(root string) string { return filepath.Join(root, ".claude", "awf.yaml") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := tc.path(root)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(path), path); err != nil {
				t.Fatal(err)
			}
			if _, err := Generation(root); !errors.Is(err, syscall.ELOOP) {
				t.Fatalf("Generation error = %v, want stat loop", err)
			}
		})
	}
}

func TestRetireRelevanceMetadataMigration(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, config.ConfigPath(root), "# keep\nprefix: test\nprofile: full\nintegrationBranch: main\ntags: {one: meaning}\ncontextIgnore:\n  - docs/**\nvars: {kept: value}\n")
	testsupport.WriteFile(t, filepath.Join(root, pitfall.SourceDir, "with-domain.md"), "---\ntitle: With domain\ndomains: [rendering]\ntags: [one]\nrelated: [1]\n---\nbody\n")
	testsupport.WriteFile(t, filepath.Join(root, decisionDir, "0001-decision.md"), "# Historical decision\n")
	testsupport.WriteFile(t, filepath.Join(root, pitfall.SourceDir, "without-domain.md"), "---\ntitle: Without domain\ntags:\n  - one\n---\nbody\n")
	if err := (&manifest.Lock{AWFVersion: "0.39.2", SchemaVersion: LiveSchemaFloor, Files: map[string]manifest.Entry{}}).Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	applied, changes, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{retireRelevanceMetadataName, retireClaimProvenanceMetadataName, retireWorkflowConfigName, retirePitfallRelationsName}) || len(changes) != 5 || len(mutations) != 3 {
		t.Fatalf("applied=%v changes=%v mutations=%v", applied, changes, mutations)
	}
	for _, mutation := range mutations {
		if strings.Contains(string(mutation.Content), "tags:") || strings.Contains(string(mutation.Content), "contextIgnore:") {
			t.Fatalf("retired metadata remains in %s: %s", mutation.Path, mutation.Content)
		}
		if mutation.Path == config.DirName+"/config.yaml" && (!strings.Contains(string(mutation.Content), "# keep") || !strings.Contains(string(mutation.Content), "vars: {kept: value}")) {
			t.Fatalf("unrelated config formatting changed: %s", mutation.Content)
		}
	}
}
