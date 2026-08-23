package checkop

import (
	"context"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestStagedDriftResultRejectsPreparationAndPublisherFailures(t *testing.T) {
	if _, err := stagedDriftResult(context.Background(), t.TempDir()); err == nil {
		t.Fatal("staged drift accepted a directory outside a repository")
	}

	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	gitfixture.Stage(t, gitfixture.At(root), map[string]string{".awf/docs/pitfalls/bad.md": "malformed source\n"})
	if _, err := stagedDriftResult(context.Background(), root); err == nil || !strings.Contains(err.Error(), "missing frontmatter") {
		t.Fatalf("staged drift publisher preparation error = %v", err)
	}
}
