package checkop

import (
	"context"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestRunExecutesDirectMemoryAndStagedDriftLeaves(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	for _, leaf := range []Leaf{RepositoryProse, RepositoryMemory, StagedDrift} {
		t.Run(leafName(leaf), func(t *testing.T) {
			out, err := Run(context.Background(), root, leaf)
			if err != nil {
				t.Fatalf("Run(%v) error = %v", leaf, err)
			}
			if (leaf == RepositoryProse || leaf == RepositoryMemory) && out.Failure != nil {
				t.Fatalf("Run(%v) produced failure = %v", leaf, out.Failure)
			}
			if leaf == StagedDrift && out.Failure == nil {
				t.Fatal("staged drift did not report fixture drift")
			}
		})
	}
}

func TestRunCheckRetainsStagedCollectionFailure(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	gitfixture.Stage(t, gitfixture.At(root), map[string]string{".awf/awf.lock": "not: [a lock"})
	_, err := Run(context.Background(), root, Check)
	if err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("Run(Check) malformed lock error = %v", err)
	}
}

func TestRunRejectsUnknownLeaf(t *testing.T) {
	_, err := Run(context.Background(), t.TempDir(), Leaf(255))
	if err == nil || !strings.Contains(err.Error(), "unknown repository-check operation") {
		t.Fatalf("unknown leaf error = %v", err)
	}
}

func leafName(leaf Leaf) string {
	switch leaf {
	case RepositoryProse:
		return "repository-prose"
	case RepositoryMemory:
		return "repository-memory"
	case StagedDrift:
		return "staged-drift"
	default:
		return "unknown"
	}
}
