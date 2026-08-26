package checkop

import (
	"context"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestLockVsBinaryLockRejectsMissingVersion(t *testing.T) {
	if lockV, binV, ok := lockVsBinaryLock(&manifest.Lock{Files: map[string]manifest.Entry{"prior": {}}}); ok || lockV != "" || binV != "" {
		t.Fatalf("lockVsBinaryLock() = %q, %q, %t, want empty invalid result", lockV, binV, ok)
	}
}

func TestStagedLockRejectsMalformedLockAndNormalizeSemverRejectsInvalidVersion(t *testing.T) {
	root := stagedCheckProject(t, map[string]string{".awf/config.yaml": checkYAML})
	gitfixture.Stage(t, gitfixture.At(root), map[string]string{".awf/awf.lock": "not: [a lock"})
	if _, err := stagedLock(context.Background(), root); err == nil || !strings.Contains(err.Error(), "parse staged lock") {
		t.Fatalf("malformed staged lock error = %v", err)
	}
	if got, ok := normalizeSemver("not-a-version"); ok || got != "" {
		t.Fatalf("normalizeSemver invalid = %q, %t", got, ok)
	}
}
