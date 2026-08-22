package currentstatecoord

import (
	"context"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// TestCurrentStateContextSubstrateFailuresAndEmptyHead retains staged helper contracts.
func TestCurrentStateContextSubstrateFailuresAndEmptyHead(t *testing.T) {
	tree, err := snapshot.NewTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockFromTree(tree); err == nil {
		t.Fatal("missing staged lock accepted")
	}
}

func TestLoadTreeCurrentStatePropagatesAuthorityParseFailure(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: test\nprofile: full\nintegrationBranch: main\ndomains: [tooling]\n")},
		{Path: "docs/decisions/bad.md", Mode: snapshot.Regular, Bytes: []byte("not an ADR\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadTreeCurrentState(t.TempDir(), tree, nil); err == nil {
		t.Fatal("malformed selected authority was accepted")
	}
}

func TestPrepareStagedContextPropagatesSnapshotFailure(t *testing.T) {
	if _, err := PrepareStagedContext(context.Background(), t.TempDir()); err == nil {
		t.Fatal("staged context accepted a non-repository")
	}
}
