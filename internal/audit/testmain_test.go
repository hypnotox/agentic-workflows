package audit

import (
	"os"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestMain isolates this package's tests from the host by giving them a throwaway
// HOME, so go-git's global-gitignore read finds nothing. The uncommitted-changes
// rule reads live global ignore patterns by design, so the tests must not inherit
// the developer's.
func TestMain(m *testing.M) {
	os.Exit(testsupport.RunIsolated(m, "awf-audit-test-home"))
}
