package testselection

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testPolicy() Policy {
	return Policy{
		Version: 1,
		MetaSuites: []SuitePolicy{{
			ID:      "composition",
			Package: "./cmd/meta",
			Tests:   []string{"TestRegistry", "TestArchitecture"},
		}},
		SharedPathPatterns:  []string{"templates/**", "*.json", "x", "tools/**"},
		GeneratedGoPatterns: []string{"*_generated.go"},
	}
}

func testGraph() graph {
	return graph{packages: map[string]node{
		"./internal/leaf":     {},
		"./internal/user":     {imports: []string{"./internal/leaf"}},
		"./internal/testuser": {testImports: []string{"./internal/leaf"}},
		"./internal/consumer": {imports: []string{"./internal/testuser"}},
		"./cmd/meta":          {imports: []string{"./internal/user"}},
	}}
}

// invariant: tooling/quality-gates:affected-package-feedback (TestSelectPathsConservativelyClosesAffectedPackages)
func TestSelectPathsConservativelyClosesAffectedPackages(t *testing.T) {
	result := selectPaths(testPolicy(), testGraph(), []string{"internal/leaf/leaf_external_test.go", "internal/leaf/leaf.go"})
	if result.Outcome != "selected" {
		t.Fatalf("outcome = %q", result.Outcome)
	}
	got := make([]string, len(result.Packages))
	for i, pkg := range result.Packages {
		got[i] = pkg.Path
	}
	want := []string{"./cmd/meta", "./internal/leaf", "./internal/testuser", "./internal/user"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %#v, want %#v", got, want)
	}
	if len(result.Suites) != 0 {
		t.Fatalf("suite duplicated full package: %#v", result.Suites)
	}
	if wantReasons := []string{"contains-suite:composition:declared-meta-suite", "reverse-dependent:./internal/leaf"}; !reflect.DeepEqual(result.Packages[0].Reasons, wantReasons) {
		t.Fatalf("meta package reasons = %#v, want %#v", result.Packages[0].Reasons, wantReasons)
	}
	for _, pkg := range result.Packages {
		if pkg.Path == "./internal/consumer" {
			t.Fatal("test-only dependency incorrectly propagated to production consumer")
		}
	}
}

func TestChangedTestPackageDoesNotPropagateAsProduction(t *testing.T) {
	result := selectPaths(testPolicy(), testGraph(), []string{"internal/leaf/leaf_test.go"})
	if len(result.Packages) != 1 || result.Packages[0].Path != "./internal/leaf" || len(result.Suites) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if !reflect.DeepEqual(result.Suites[0].Reasons, []string{"declared-meta-suite"}) {
		t.Fatalf("suite reasons = %#v", result.Suites[0].Reasons)
	}
}

func TestDirectSuitePackageRunsFullyWithoutDuplicateSuite(t *testing.T) {
	result := selectPaths(testPolicy(), testGraph(), []string{"cmd/meta/main.go"})
	if len(result.Packages) != 1 || result.Packages[0].Path != "./cmd/meta" || len(result.Suites) != 0 {
		t.Fatalf("result = %#v", result)
	}
	if !reflect.DeepEqual(result.Packages[0].Reasons, []string{"changed-package:cmd/meta/main.go", "contains-suite:composition:declared-meta-suite"}) {
		t.Fatalf("reasons = %#v", result.Packages[0].Reasons)
	}
}

func TestSelectPathsOwnsNonGoPackageInputsAndWidensUnknownPaths(t *testing.T) {
	result := selectPaths(testPolicy(), testGraph(), []string{"internal/leaf/testdata/input.txt"})
	got := []string{}
	for _, pkg := range result.Packages {
		got = append(got, pkg.Path)
	}
	if want := []string{"./cmd/meta", "./internal/leaf", "./internal/testuser", "./internal/user"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("package input selection = %#v, want %#v", got, want)
	}
	result = selectPaths(testPolicy(), testGraph(), []string{"docs/unknown.txt"})
	if result.Outcome != "widened" || !reflect.DeepEqual(result.Reasons, []string{"unclassified-change:docs/unknown.txt"}) {
		t.Fatalf("unknown path result = %#v", result)
	}
}

func TestSelectPathsWidensDeletedOrUnownedGoPackage(t *testing.T) {
	result := selectPaths(testPolicy(), testGraph(), []string{"removed/package.go"})
	if result.Outcome != "widened" || !reflect.DeepEqual(result.Reasons, []string{"unowned-go-change:removed/package.go"}) {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Packages) != len(testGraph().packages) || len(result.Suites) != 0 {
		t.Fatalf("widened targets = %#v", result)
	}
}

func TestSharedChangeRecognizesTemplatesConfigurationToolingGeneratedAndBuildTags(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "tagged"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "tagged", "tagged.go"), []byte("//go:build linux\npackage tagged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"templates/a.tmpl", "test-selection.json", "x", "tools/run.go", "internal/leaf_generated.go", "internal/tagged/tagged.go"} {
		shared, reason := sharedChange(root, testPolicy(), []string{path})
		if !shared || reason == "" {
			t.Errorf("%s was not shared: %q", path, reason)
		}
	}
	ordinary := filepath.Join(root, "internal", "tagged", "ordinary.go")
	if err := os.WriteFile(ordinary, []byte("package tagged\nconst buildMarker = `//go:build linux`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if shared, reason := sharedChange(root, testPolicy(), []string{"internal/tagged/ordinary.go"}); shared {
		t.Fatalf("ordinary source with marker text widened: %q", reason)
	}
}

func TestSelectEmptyAndRejectsInvalidPath(t *testing.T) {
	result, err := Select(context.Background(), t.TempDir(), testPolicy(), nil)
	if err != nil || result.Outcome != "empty" || !reflect.DeepEqual(result.Reasons, []string{"no-relevant-changes"}) || result.Suites == nil {
		t.Fatalf("empty = %#v, %v", result, err)
	}
	result, err = Select(context.Background(), t.TempDir(), testPolicy(), []string{"../outside.go"})
	if err == nil || result.Outcome != "refused" || result.Suites == nil {
		t.Fatalf("invalid = %#v, %v", result, err)
	}
	result, err = Select(context.Background(), t.TempDir(), testPolicy(), []string{"missing.go"})
	if err == nil || result.Outcome != "refused" {
		t.Fatalf("unavailable graph = %#v, %v", result, err)
	}
}

func TestDiscoverAndSelectSeparatesProductionAndTestDependencyEdges(t *testing.T) {
	root := t.TempDir()
	for path, contents := range map[string]string{
		"go.mod":                             "module example.test/selection\ngo 1.26\n",
		"internal/leaf/leaf.go":              "package leaf\n",
		"internal/user/user.go":              "package user\nimport _ \"example.test/selection/internal/leaf\"\n",
		"internal/testuser/testuser.go":      "package testuser\n",
		"internal/testuser/testuser_test.go": "package testuser\nimport _ \"example.test/selection/internal/leaf\"\n",
		"internal/consumer/consumer.go":      "package consumer\nimport _ \"example.test/selection/internal/testuser\"\n",
		"cmd/meta/main.go":                   "package main\nimport _ \"example.test/selection/internal/user\"\nfunc main() {}\n",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Select(context.Background(), root, testPolicy(), []string{"internal/leaf/leaf.go"})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, pkg := range result.Packages {
		got = append(got, pkg.Path)
	}
	if want := []string{"./cmd/meta", "./internal/leaf", "./internal/testuser", "./internal/user"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %#v, want %#v; result=%#v", got, want, result)
	}
}

func TestDiscoverUnionsSupportedPlatformDependencies(t *testing.T) {
	root := t.TempDir()
	for path, contents := range map[string]string{
		"go.mod":                         "module example.test/platforms\ngo 1.26\n",
		"internal/leaf/leaf.go":          "package leaf\n",
		"internal/darwin/user_darwin.go": "//go:build darwin\n\npackage darwin\nimport _ \"example.test/platforms/internal/leaf\"\n",
		"internal/darwin/user_linux.go":  "//go:build !darwin\n\npackage darwin\n",
		"cmd/meta/main.go":               "package main\nfunc main() {}\n",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Select(t.Context(), root, testPolicy(), []string{"internal/leaf/leaf.go"})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, pkg := range result.Packages {
		got = append(got, pkg.Path)
	}
	if want := []string{"./internal/darwin", "./internal/leaf"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("platform union packages = %#v, want %#v; result=%#v", got, want, result)
	}
	if len(result.Suites) != 1 || result.Suites[0].ID != "composition" {
		t.Fatalf("platform union suites = %#v", result.Suites)
	}
}

func TestRepositoryPolicySelectsRepresentativeCommonChange(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := Load(filepath.Join(root, "test-selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Select(t.Context(), root, policy, []string{"internal/testselection/selection.go"})
	if err != nil {
		t.Fatal(err)
	}
	gotPackages := []string{}
	for _, pkg := range result.Packages {
		gotPackages = append(gotPackages, pkg.Path)
	}
	if want := []string{"./cmd/testselection", "./internal/testselection"}; !reflect.DeepEqual(gotPackages, want) {
		t.Fatalf("packages = %#v, want %#v; result=%#v", gotPackages, want, result)
	}
	if len(result.Suites) != 1 || result.Suites[0].ID != "awf-composition" || !reflect.DeepEqual(result.Suites[0].Reasons, []string{"declared-meta-suite"}) {
		t.Fatalf("suites = %#v", result.Suites)
	}
}

func TestLoadRejectsUnsupportedOrIncompletePolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	bodies := []string{
		`{"version":2,"meta_suites":[{"id":"meta","package":"./cmd/meta","tests":["TestMeta"]}],"shared_path_patterns":["x"],"generated_go_patterns":["*.gen.go"]}`,
		`{"version":1,"meta_suites":[],"shared_path_patterns":["x"],"generated_go_patterns":["*.gen.go"]}`,
		`{"version":1,"meta_suites":[{"id":"meta","package":"./...","tests":["TestMeta"]}],"shared_path_patterns":["x"],"generated_go_patterns":["*.gen.go"]}`,
		`{"version":1,"meta_suites":[{"id":"meta","package":"./cmd/meta","tests":["not-a-test"]}],"shared_path_patterns":["x"],"generated_go_patterns":["*.gen.go"]}`,
		`{"version":1,"meta_suites":[{"id":"meta","package":"./cmd/meta","tests":["TestMeta"]}],"shared_path_patterns":["x"],"generated_go_patterns":["*.gen.go"],"unknown":true}`,
		`{"version":1,"meta_suites":[{"id":"meta","package":"./cmd/meta","tests":["TestMeta"]}],"shared_path_patterns":[],"generated_go_patterns":["*.gen.go"]}`,
		`{"version":1,"meta_suites":[{"id":"meta","package":"./cmd/meta","tests":["TestMeta"]}],"shared_path_patterns":[""],"generated_go_patterns":["*.gen.go"]}`,
		`{"version":1,"meta_suites":[{"id":"meta","package":"./cmd/meta","tests":["TestMeta"]}],"shared_path_patterns":["["] ,"generated_go_patterns":["*.gen.go"]}`,
	}
	for _, body := range bodies {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("Load(%s) unexpectedly succeeded", body)
		}
	}
}
