package testselection

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testPolicy() Policy {
	return Policy{
		Version: PolicyVersion,
		Lanes: []LanePolicy{
			{Name: "go", Patterns: []string{"**/*.go", "**/testdata/**", "go.mod"}},
			{Name: "render-template", Patterns: []string{".awf/**", "templates/**", "internal/publisher/**"}},
			{Name: "platform-sensitive", Patterns: []string{"**/*_linux.go", "cmd/releasecheck/**", "internal/filesystem/**", "internal/resident/**", "internal/upgrade/**", "tools/native-release-test/**"}},
			{Name: "release-archive", Patterns: []string{".goreleaser.yaml", "cmd/releasecheck/**", "changelog/**"}},
		},
		SharedPathPatterns:  []string{"go.mod", "test-selection.json", "x"},
		GeneratedGoPatterns: []string{"**/*_generated.go"},
	}
}

func testGraph() graph {
	return graph{packages: map[string]node{
		"./internal/leaf":     {},
		"./internal/user":     {imports: []string{"./internal/leaf"}},
		"./internal/testuser": {testImports: []string{"./internal/user"}},
		"./internal/consumer": {imports: []string{"./internal/testuser"}},
		"./cmd/meta":          {imports: []string{"./internal/user"}},
	}}
}

func laneNames(lanes []Lane) []string {
	names := make([]string, 0, len(lanes))
	for _, lane := range lanes {
		names = append(names, lane.Name)
	}
	return names
}

func packagePaths(packages []Package) []string {
	paths := make([]string, 0, len(packages))
	for _, selected := range packages {
		paths = append(paths, selected.Path)
	}
	return paths
}

// invariant: tooling/quality-gates:affected-package-feedback (TestTypedLaneChangeMatrix)
func TestTypedLaneChangeMatrix(t *testing.T) {
	policy := testPolicy()
	cases := []struct {
		name string
		path string
		want []string
	}{
		{name: "Go", path: "internal/leaf/leaf.go", want: []string{"go"}},
		{name: "skill template renders", path: "templates/skills/awf-maintenance/SKILL.md.tmpl", want: []string{"render-template"}},
		{name: "partial", path: "templates/partials/gate.md", want: []string{"render-template"}},
		{name: "platform Go", path: "internal/filesystem/replace_linux.go", want: []string{"go", "platform-sensitive"}},
		{name: "release Go is native-sensitive", path: "cmd/releasecheck/main.go", want: []string{"go", "platform-sensitive", "release-archive"}},
		{name: "resident lifecycle", path: "internal/resident/resident.go", want: []string{"go", "platform-sensitive"}},
		{name: "upgrade mutation", path: "internal/upgrade/operation.go", want: []string{"go", "platform-sensitive"}},
		{name: "render authority", path: ".awf/docs/parts/testing.md", want: []string{"render-template"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, classified := selectedLanes(policy, []string{tc.path})
			if !classified[tc.path] || !reflect.DeepEqual(laneNames(got), tc.want) {
				t.Fatalf("lanes for %s = %v, classified=%v; want %v", tc.path, laneNames(got), classified[tc.path], tc.want)
			}
		})
	}
}

func TestDynamicReaderInputsHaveExplicitOwners(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := Load(filepath.Join(root, "test-selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		"templates/embed.go":                       {"go", "render-template"},
		"templates/partials/gate-cadence.md":       {"render-template"},
		"internal/catalog/standard.go":             {"go", "render-template"},
		"internal/publisher/testdata/input.golden": {"go", "render-template"},
		".goreleaser.yaml":                         {"release-archive"},
		"changelog/CHANGELOG.md":                   {"release-archive"},
		"internal/project/VERSION":                 {"release-archive"},
		"cmd/releasecheck/testdata/archive.tar.gz": {"go", "platform-sensitive", "release-archive"},
	}
	for changed, want := range cases {
		lanes, classified := selectedLanes(policy, []string{changed})
		if !classified[changed] || !reflect.DeepEqual(laneNames(lanes), want) {
			t.Errorf("dynamic input %s lanes = %v; want %v", changed, laneNames(lanes), want)
		}
	}
}

func TestUnrelatedLaneExclusion(t *testing.T) {
	lanes, _ := selectedLanes(testPolicy(), []string{"templates/partials/a.md"})
	if got, want := laneNames(lanes), []string{"render-template"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unrelated lanes selected: got %v want %v", got, want)
	}
}

func TestSelectGoPackagesClosesProductionAndTestDependenciesWithoutSmoke(t *testing.T) {
	packages, reason := selectGoPackages(testPolicy(), testGraph(), []string{"internal/leaf/leaf.go"})
	if reason != "" {
		t.Fatal(reason)
	}
	want := []string{"./cmd/meta", "./internal/leaf", "./internal/testuser", "./internal/user"}
	if got := packagePaths(packages); !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %v, want %v", got, want)
	}
	for _, selected := range packages {
		if selected.Path == "./cmd/meta" && !reflect.DeepEqual(selected.Reasons, []string{"reverse-dependent:./internal/leaf"}) {
			t.Fatalf("cmd/meta reasons = %v", selected.Reasons)
		}
	}
}

func TestChangedTestDoesNotPropagateAndPackageFixtureDoes(t *testing.T) {
	packages, reason := selectGoPackages(testPolicy(), testGraph(), []string{"internal/leaf/leaf_test.go"})
	if reason != "" || !reflect.DeepEqual(packagePaths(packages), []string{"./internal/leaf"}) {
		t.Fatalf("test-only selection = %v, %q", packagePaths(packages), reason)
	}
	packages, reason = selectGoPackages(testPolicy(), testGraph(), []string{"internal/leaf/testdata/input.txt"})
	want := []string{"./cmd/meta", "./internal/leaf", "./internal/testuser", "./internal/user"}
	if reason != "" || !reflect.DeepEqual(packagePaths(packages), want) {
		t.Fatalf("fixture selection = %v, %q; want %v", packagePaths(packages), reason, want)
	}
}

func TestUnknownDeletedSharedGeneratedAndBuildTagChangesWiden(t *testing.T) {
	root := t.TempDir()
	for filename, contents := range map[string]string{
		"go.mod":                  "module example.test/tagged\ngo 1.27\n",
		"internal/leaf/tagged.go": "//go:build linux\n\npackage leaf\n",
	} {
		full := filepath.Join(root, filename)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if lanes, classified := selectedLanes(testPolicy(), []string{"docs/unowned.txt"}); classified["docs/unowned.txt"] || len(lanes) != 0 {
		t.Fatalf("unknown path was classified: lanes=%v", lanes)
	}
	if _, reason := selectGoPackages(testPolicy(), testGraph(), []string{"internal/missing/missing.go"}); reason != "unowned-go-change:internal/missing/missing.go" {
		t.Fatalf("deleted Go reason = %q", reason)
	}
	if shared, reason := sharedChange(testPolicy(), []string{"x"}); !shared || reason != "shared-change:x" {
		t.Fatalf("shared=%v reason=%q", shared, reason)
	}
	if widened, reason := packageWideningChange(root, testPolicy(), []string{"internal/leaf/code_generated.go"}); !widened || reason != "generated-go-change:internal/leaf/code_generated.go" {
		t.Fatalf("generated widened=%v reason=%q", widened, reason)
	}
	result, err := Select(t.Context(), root, testPolicy(), []string{"internal/leaf/tagged.go"})
	if err != nil || result.Outcome != "widened" || result.Diagnostics[0] != "build-tag-change:internal/leaf/tagged.go" {
		t.Fatalf("build-tag result=%#v err=%v", result, err)
	}
	if got, want := laneNames(result.Lanes), []string{"go", "platform-sensitive"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("build-tag lanes=%v, want %v", got, want)
	}
}

func TestSelectEmptyRefusedAndWidenedResultsAreStableJSON(t *testing.T) {
	policy := testPolicy()
	emptyResult, err := Select(context.Background(), t.TempDir(), policy, nil)
	if err != nil || emptyResult.Outcome != "empty" || emptyResult.Lanes == nil || emptyResult.Packages == nil || emptyResult.Diagnostics == nil {
		t.Fatalf("empty = %#v, %v", emptyResult, err)
	}
	refusedResult, err := Select(context.Background(), t.TempDir(), policy, []string{"a/../../outside.go"})
	if err == nil || refusedResult.Outcome != "refused" {
		t.Fatalf("refused = %#v, %v", refusedResult, err)
	}
	one, err := json.Marshal(refusedResult)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := json.Marshal(refusedResult)
	if !reflect.DeepEqual(one, two) || !strings.Contains(string(one), `"lanes":[]`) || !strings.Contains(string(one), `"diagnostics":[`) {
		t.Fatalf("unstable interface: %s", one)
	}
}

func TestDiscoverUnionsSupportedPlatformDependencies(t *testing.T) {
	root := t.TempDir()
	for filename, contents := range map[string]string{
		"go.mod":                         "module example.test/platforms\ngo 1.27\n",
		"internal/leaf/leaf.go":          "package leaf\n",
		"internal/darwin/user_darwin.go": "//go:build darwin\n\npackage darwin\nimport _ \"example.test/platforms/internal/leaf\"\n",
		"internal/darwin/user_linux.go":  "//go:build !darwin\n\npackage darwin\n",
	} {
		full := filepath.Join(root, filename)
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
	if got, want := packagePaths(result.Packages), []string{"./internal/darwin", "./internal/leaf"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("platform union packages = %v, want %v", got, want)
	}
}

func TestRepositoryPolicySelectsRepresentativeGoChangeWithoutCmdAWFDefault(t *testing.T) {
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
	got := packagePaths(result.Packages)
	want := []string{"./cmd/testselection", "./internal/testselection"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %v, want %v; result=%#v", got, want, result)
	}
	for _, selected := range got {
		if selected == "./cmd/awf" {
			t.Fatal("Go-only change selected cmd/awf unconditionally")
		}
	}
}

func TestLoadRejectsUnsupportedIncompleteDuplicateAndUnknownPolicy(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "policy.json")
	validLanes := `[{"name":"go","patterns":["**/*.go"]},{"name":"render-template","patterns":["templates/**"]},{"name":"platform-sensitive","patterns":["platform/**"]},{"name":"release-archive","patterns":["release/**"]}]`
	bodies := []string{
		`{"version":1}`,
		`{"version":2,"lanes":[],"shared_path_patterns":["x"],"generated_go_patterns":["*.go"]}`,
		`{"version":2,"lanes":` + validLanes + `,"shared_path_patterns":[],"generated_go_patterns":["*.go"]}`,
		`{"version":2,"lanes":` + validLanes + `,"shared_path_patterns":["x","x"],"generated_go_patterns":["*.go"]}`,
		`{"version":2,"lanes":` + validLanes + `,"shared_path_patterns":["x"],"generated_go_patterns":["*.go"],"unknown":true}`,
	}
	for _, body := range bodies {
		if err := os.WriteFile(filename, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(filename); err == nil {
			t.Errorf("Load(%s) unexpectedly succeeded", body)
		}
	}
}
