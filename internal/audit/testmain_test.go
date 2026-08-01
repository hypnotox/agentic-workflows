package audit

import (
	"os"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestMain isolates this package's tests from the host by giving them a throwaway
// HOME, so the global-gitignore read finds nothing. The uncommitted-changes rule
// reads live global ignore patterns by design - the seam replays the effective
// core.excludesFile into its cleanliness read precisely so the answer matches
// real git - so the tests must not inherit the developer's.
func TestMain(m *testing.M) {
	os.Exit(testsupport.RunIsolated(m))
}
