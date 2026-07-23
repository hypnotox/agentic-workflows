package project

import (
	"os"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestMain isolates HOME so working-tree snapshots taken by these tests never
// pick up the developer's real global gitignore (git.GlobalExcludePatterns).
func TestMain(m *testing.M) { os.Exit(testsupport.RunIsolated(m, "awf-project-test-home")) }
