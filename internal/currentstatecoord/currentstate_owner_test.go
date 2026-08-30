package currentstatecoord

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func ownerTree(t *testing.T, files ...snapshot.File) *snapshot.Tree {
	t.Helper()
	tree, err := snapshot.NewTree(files)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func writeContextFile(t *testing.T, root, path, contents string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWorkingCurrentStateRequiresConfigInSelectedTree(t *testing.T) {
	fixture := gitfixture.InitRepo(t)
	repo, err := awfgit.Open(fixture.Root())
	if err != nil {
		t.Fatal(err)
	}
	_, err = workingCurrentState(fixture.Root(), repo, context.Background())
	if err == nil || !strings.Contains(err.Error(), "working snapshot has no .awf/config.yaml") {
		t.Fatalf("working state without config error = %v", err)
	}
}

func TestSnapshotReaderProjectsScannableIndexFilesDefensively(t *testing.T) {
	tree := ownerTree(t,
		snapshot.File{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("config")},
		snapshot.File{Path: ".awf/unread.yaml", Mode: snapshot.Regular, Bytes: []byte("value")},
		snapshot.File{Path: "link", Mode: snapshot.Symlink, Bytes: []byte("elsewhere")},
	)
	reader := snapshotReader{tree: tree}
	data, found, err := reader.ReadFile(".awf/config.yaml")
	if err != nil || !found || string(data) != "config" {
		t.Fatalf("selected read = %q, %t, %v", data, found, err)
	}
	data[0] = 'X'
	again, _, _ := reader.ReadFile(".awf/config.yaml")
	if again[0] == 'X' {
		t.Fatal("snapshot reader aliases selected tree bytes")
	}
	if !reader.PathExists(".awf/unread.yaml") {
		t.Fatal("regular index path reported absent")
	}
	if reader.PathExists("link") {
		t.Fatal("symlink reported as declaration input")
	}
	if paths, err := reader.Paths(".awf/"); err != nil || !reflect.DeepEqual(paths, []string{".awf/config.yaml", ".awf/unread.yaml"}) {
		t.Fatalf("snapshot paths = %v, %v", paths, err)
	}
}

func TestPrepareStagedOutputSelectedUniverseErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := PrepareStagedOutput(ctx, t.TempDir()); !errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("outside repository error = %v", err)
	}

	for _, tc := range []struct {
		name, config, lock, want string
		unmerged                 bool
	}{
		{name: "missing config", want: "no staged .awf/config.yaml"},
		{name: "unmerged index", want: "index contains unmerged entries", unmerged: true},
		{name: "malformed lock", config: "prefix: x\nintegrationBranch: main\n", lock: "not: [valid", want: "parse snapshot lock"},
		{name: "malformed config", config: "not: [valid", want: "yaml:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := gitfixture.InitRepo(t)
			if tc.config != "" {
				gitfixture.Stage(t, fixture, map[string]string{".awf/config.yaml": tc.config})
			}
			if tc.lock != "" {
				gitfixture.Stage(t, fixture, map[string]string{".awf/awf.lock": tc.lock})
			} else if tc.config != "" {
				gitfixture.Stage(t, fixture, map[string]string{".awf/awf.lock": `{"awfVersion":"0.44.0","schemaVersion":50,"files":{"prior":{}}}`})
			}
			if tc.unmerged {
				gitfixture.StageUnmerged(t, fixture, "conflict")
			}
			if _, err := PrepareStagedOutput(ctx, fixture.Root()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("PrepareStagedOutput() error = %v, want %q", err, tc.want)
			}
		})
	}

	fixture := gitfixture.InitRepo(t)
	if err := os.Mkdir(filepath.Join(fixture.Root(), ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fixture.Root(), ".awf", "config.yaml")
	if err := os.Symlink("elsewhere", path); err != nil {
		t.Fatal(err)
	}
	writeContextFile(t, fixture.Root(), ".awf/awf.lock", `{"awfVersion":"0.44.0","schemaVersion":50,"files":{"prior":{}}}`)
	gitfixture.Add(t, fixture, ".awf/config.yaml", ".awf/awf.lock")
	if _, err := PrepareStagedOutput(ctx, fixture.Root()); err == nil || !strings.Contains(err.Error(), "not a scannable file") {
		t.Fatalf("unscannable config error = %v", err)
	}
}

func TestCoordinatorSnapshotHelpersRejectUnsafeInputs(t *testing.T) {
	symlinkLock := ownerTree(t, snapshot.File{Path: ".awf/awf.lock", Mode: snapshot.Symlink, Bytes: []byte("elsewhere")})
	if _, err := lockFromTree(symlinkLock); err == nil {
		t.Fatal("staged symlink lock accepted")
	}
	if _, found, err := optionalLockFromTree(symlinkLock); !found || err == nil {
		t.Fatal("optional symlink lock accepted")
	}

	symlinkConfig := ownerTree(t, snapshot.File{Path: ".awf/config.yaml", Mode: snapshot.Symlink, Bytes: []byte("elsewhere")})
	if _, _, err := loadTreeCurrentState(".", symlinkConfig, nil); err == nil {
		t.Fatal("symlink config accepted")
	}
	if _, _, err := headTreeAndLock(nil, context.Background()); !errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("head tree without repository error = %v", err)
	}
}

// invariant: invariants/current-state-authority:domain-owned-coverage-no-ignore (TestCoordinatorSnapshotReaderAndEligiblePaths)
func TestCoordinatorSnapshotReaderAndEligiblePaths(t *testing.T) {
	tree := ownerTree(t,
		snapshot.File{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: x\nintegrationBranch: main\n")},
		snapshot.File{Path: ".awf/unsafe.yaml", Mode: snapshot.Symlink, Bytes: []byte("outside")},
		snapshot.File{Path: ".awf/dir/value.yaml", Mode: snapshot.Regular, Bytes: []byte("value")},
		snapshot.File{Path: "nested/.awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: nested\nintegrationBranch: main\n")},
		snapshot.File{Path: "nested/file.go", Mode: snapshot.Regular, Bytes: []byte("package nested")},
		snapshot.File{Path: "generated.go", Mode: snapshot.Regular, Bytes: []byte("package generated")},
		snapshot.File{Path: "ignored.go", Mode: snapshot.Regular, Bytes: []byte("package ignored")},
		snapshot.File{Path: "kept.go", Mode: snapshot.Regular, Bytes: []byte("package kept")},
	)
	reader := configSnapshotReader{tree: tree}
	if _, ok := reader.ReadFile("missing.yaml"); ok {
		t.Fatal("missing snapshot config read")
	}
	if _, ok := reader.ReadFile("unsafe.yaml"); ok {
		t.Fatal("unscannable snapshot config read")
	}
	bytes, ok := reader.ReadFile("config.yaml")
	if !ok {
		t.Fatal("snapshot config missing")
	}
	bytes[0] = 'X'
	again, _ := reader.ReadFile("config.yaml")
	if again[0] == 'X' {
		t.Fatal("snapshot config aliases tree bytes")
	}
	if got, want := reader.Paths("dir"), []string{"dir/value.yaml"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot config paths = %v, want %v", got, want)
	}
	lock := &manifest.Lock{Files: map[string]manifest.Entry{"generated.go": {}}}
	if got, want := eligiblePaths(tree, lock), []string{".awf/config.yaml", ".awf/dir/value.yaml", "ignored.go", "kept.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("eligible paths = %v, want %v", got, want)
	}
}

func TestCoordinatorLockTransitionAndCoreConfig(t *testing.T) {
	empty := ownerTree(t)
	afterWithConfig := ownerTree(t, snapshot.File{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: x\nintegrationBranch: main\n")})
	if err := validateLockTransition(empty, empty, nil, &manifest.Lock{}); err == nil || !strings.Contains(err.Error(), "requires .awf/config.yaml") {
		t.Fatalf("missing staged config error = %v", err)
	}
	if err := validateLockTransition(empty, afterWithConfig, nil, &manifest.Lock{}); err != nil {
		t.Fatal(err)
	}
	residualHead := ownerTree(t, snapshot.File{Path: ".awf/orphan", Mode: snapshot.Regular, Bytes: []byte("residue")})
	if err := validateLockTransition(residualHead, afterWithConfig, nil, &manifest.Lock{}); err == nil || !strings.Contains(err.Error(), "complete pre-adoption HEAD") {
		t.Fatalf("residual .awf HEAD error = %v", err)
	}
	withConfig := ownerTree(t,
		snapshot.File{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: x\nintegrationBranch: main\n")},
		snapshot.File{Path: ".awf/awf.lock", Mode: snapshot.Regular, Bytes: []byte(`{"awfVersion":"0.44.0","schemaVersion":50,"files":{"prior":{}}}`)},
	)
	if err := validateLockTransition(withConfig, withConfig, nil, &manifest.Lock{}); err == nil {
		t.Fatal("pre-adoption config accepted")
	}
	if err := validateLockTransition(empty, afterWithConfig, &manifest.Lock{InitializedWithVersion: "one"}, &manifest.Lock{InitializedWithVersion: "two"}); err == nil {
		t.Fatal("initialized version mutation accepted")
	}
	loaded, cfg, err := loadTreeCurrentState(".", withConfig, &manifest.Lock{AWFVersion: "0.44.0", SchemaVersion: 50})
	if err != nil || cfg == nil || len(loaded.Topics.All()) != 0 {
		t.Fatalf("current-state config load = %#v, %#v, %v", loaded, cfg, err)
	}
}

func TestQueryTopicPropagatesWorkingSnapshotFailure(t *testing.T) {
	if _, err := QueryTopic(t.TempDir(), nil, context.Background(), "missing", topic.QueryOptions{}); err == nil {
		t.Fatal("topic query accepted a directory outside a repository")
	}
}

// TestCurrentStateCoordinationIgnoresHistoricalDecisions proves each selected
// current-state route retains topic coverage, backing, and query behavior while
// decisions are absent from the working tree and malformed in the index.
func TestCurrentStateCoordinationIgnoresHistoricalDecisions(t *testing.T) {
	fixture := gitfixture.InitRepo(t)
	const configBody = "prefix: test\nintegrationBranch: main\ndomains: [alpha]\ncurrentState:\n  sources:\n    - globs: [\"internal/**/*_test.go\"]\n      marker: //\n  testGlobs: [\"internal/**/*_test.go\"]\n"
	const topicPart = "Intro.\n\n## Claims\n\n### `invariant: covered`\nCoverage remains active.\nBacking: test\n"
	files := map[string]string{
		".awf/config.yaml":                                  configBody,
		".awf/awf.lock":                                     `{"awfVersion":"0.44.0","schemaVersion":50,"files":{"prior":{}}}`,
		".awf/domains/alpha.yaml":                           "paths: [\"internal/**\"]\n",
		".awf/topics/metadata/alpha/coverage.yaml":          "title: Coverage\nsummary: Active coverage.\npaths: [\"internal/**\"]\n",
		".awf/topics/parts/alpha/coverage/current-state.md": topicPart,
		"internal/covered_test.go":                          "// invariant: alpha/coverage:covered (TestCovered)\nfunc TestCovered() {}\n",
		"docs/decisions/0001-history.md":                    "# Historical Markdown\n",
	}
	gitfixture.Commit(t, fixture, "baseline", files)
	gitfixture.Stage(t, fixture, map[string]string{"docs/decisions/0001-history.md": "---\nformat: unknown\n---\n# Malformed old lifecycle text\n"})
	if err := os.Remove(filepath.Join(fixture.Root(), "docs/decisions/0001-history.md")); err != nil {
		t.Fatal(err)
	}
	repo, err := awfgit.Open(fixture.Root())
	if err != nil {
		t.Fatal(err)
	}

	working, err := CheckWorking(fixture.Root(), repo, context.Background())
	if err != nil || len(working.Coverage) != 0 {
		t.Fatalf("working current-state report = %#v, err=%v", working, err)
	}
	query, err := QueryTopic(fixture.Root(), repo, context.Background(), "alpha/coverage:covered", topic.QueryOptions{Coverage: true})
	if err != nil || query.ID != "alpha/coverage:covered" || len(query.Claims) != 1 || query.Coverage == nil || len(query.Coverage.Applicability.MarkerSites) != 1 {
		t.Fatalf("working current-state query = %#v, err=%v", query, err)
	}

	staged, err := CheckStaged(fixture.Root(), repo, context.Background())
	if err != nil || len(staged.Coverage) != 0 {
		t.Fatalf("staged current-state report = %#v, err=%v", staged, err)
	}
}
