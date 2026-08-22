package currentstatecoord

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func ownerTree(t *testing.T, files ...snapshot.File) *snapshot.Tree {
	t.Helper()
	tree, err := snapshot.NewTree(files)
	if err != nil {
		t.Fatal(err)
	}
	return tree
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

func TestCoordinatorPlansFromSelectedTree(t *testing.T) {
	tree := ownerTree(t,
		snapshot.File{Path: "guide/plans/link.md", Mode: snapshot.Symlink, Bytes: []byte("outside")},
		snapshot.File{Path: "guide/plans/nested/ignored.md", Mode: snapshot.Regular, Bytes: []byte("---\nformat: plan-v2\n---\n")},
		snapshot.File{Path: "guide/plans/2026-08-03-broken.md", Mode: snapshot.Regular, Bytes: []byte("---\nformat: plan-v2\ndate: 2026-08-03\nadrs: []\nstatus: Proposed\n---\n# Bad\n")},
	)
	plans, drift, err := plansFromTree(tree, "guide")
	if err != nil || plans != nil || len(drift) != 1 || drift[0].Path != "guide/plans/2026-08-03-broken.md" || drift[0].Kind != "plan-structure" {
		t.Fatalf("plans from selected tree = %#v, %#v, %v", plans, drift, err)
	}
}

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
	if got, want := eligiblePaths(tree, lock, []string{"ignored.go"}), []string{".awf/config.yaml", ".awf/dir/value.yaml", "kept.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("eligible paths = %v, want %v", got, want)
	}
}

func TestCoordinatorLockTransitionAndCoreConfig(t *testing.T) {
	empty := ownerTree(t)
	if err := validateLockTransition(empty, nil, &manifest.Lock{}); err != nil {
		t.Fatal(err)
	}
	withConfig := ownerTree(t, snapshot.File{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: x\nprofile: core\nintegrationBranch: main\n")})
	if err := validateLockTransition(withConfig, nil, &manifest.Lock{}); err == nil {
		t.Fatal("pre-adoption config accepted")
	}
	if err := validateLockTransition(empty, &manifest.Lock{InitializedWithVersion: "one"}, &manifest.Lock{InitializedWithVersion: "two"}); err == nil {
		t.Fatal("initialized version mutation accepted")
	}
	loaded, cfg, err := loadTreeCurrentState(".", withConfig, nil)
	if err != nil || cfg == nil || len(loaded.ADRs) != 0 || cfg.Profile != catalog.ProfileCore {
		t.Fatalf("core config load = %#v, %#v, %v", loaded, cfg, err)
	}
}
