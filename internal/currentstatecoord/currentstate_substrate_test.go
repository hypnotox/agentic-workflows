package currentstatecoord

import (
	"context"
	"errors"
	"reflect"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestCurrentStateSubstrateFailuresAndEmptyHead retains staged helper contracts.
func TestCurrentStateSubstrateFailuresAndEmptyHead(t *testing.T) {
	tree, err := snapshot.NewTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockFromTree(tree); err == nil {
		t.Fatal("missing staged lock accepted")
	}
}

func TestCurrentStateLiveAuthorityRefusals(t *testing.T) {
	belowFloor, err := snapshot.NewTree([]snapshot.File{{Path: ".awf/awf.lock", Mode: snapshot.Regular, Bytes: []byte(`{"awfVersion":"0.44.0","schemaVersion":49,"files":{}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := optionalLockFromTree(belowFloor); !found || !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("below-floor live lock = found %v, err %v; want typed live-source refusal", found, err)
	}
	partial, err := snapshot.NewTree([]snapshot.File{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: test\nintegrationBranch: main\n")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadTreeCurrentState(t.TempDir(), partial, nil); err == nil {
		t.Fatal("config-only authority was accepted")
	}
	empty, err := snapshot.NewTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadTreeCurrentState(t.TempDir(), empty, &manifest.Lock{}); err == nil {
		t.Fatal("lock-only authority was accepted")
	}
}

func TestPrepareStagedOutputRefusesBelowFloorLockWithoutMutatingIndex(t *testing.T) {
	fixture := gitfixture.InitRepo(t)
	gitfixture.Stage(t, fixture, map[string]string{
		".awf/config.yaml": "prefix: test\nintegrationBranch: main\n",
		".awf/awf.lock":    `{"awfVersion":"0.44.0","schemaVersion":49,"files":{"prior":{}}}`,
	})
	repo, err := awfgit.Open(fixture.Root())
	if err != nil {
		t.Fatal(err)
	}
	before, err := snapshot.IndexTree(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := PrepareStagedOutput(context.Background(), fixture.Root()); !errors.Is(err, manifest.ErrUnsupportedLiveSource) {
		t.Fatalf("PrepareStagedOutput() error = %v, want typed live-source refusal", err)
	}

	after, err := snapshot.IndexTree(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.List(), before.List()) {
		t.Fatalf("below-floor refusal mutated the staged tree: got %#v, want %#v", after.List(), before.List())
	}
}

func TestLoadTreeCurrentStateIgnoresMalformedHistoricalDecision(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: test\nintegrationBranch: main\ndomains: [tooling]\n")},
		{Path: "docs/decisions/bad.md", Mode: snapshot.Regular, Bytes: []byte("not an ADR\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	loaded, cfg, err := loadTreeCurrentState(t.TempDir(), tree, &manifest.Lock{SchemaVersion: 50, Files: map[string]manifest.Entry{"prior": {}}})
	if err != nil || cfg == nil || len(loaded.Topics.All()) != 0 {
		t.Fatalf("historical decision changed current-state load: loaded=%#v cfg=%#v err=%v", loaded, cfg, err)
	}
}

func TestPrepareStagedOutputPropagatesSnapshotFailure(t *testing.T) {
	if _, err := PrepareStagedOutput(context.Background(), t.TempDir()); err == nil {
		t.Fatal("staged output preparation accepted a non-repository")
	}
}
