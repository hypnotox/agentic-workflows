package audit

import (
	"os"
	"testing"
)

// TestMain isolates this package's tests from the host by giving them a throwaway
// HOME, so go-git's global-gitignore read finds nothing. The uncommitted-changes
// rule reads live global ignore patterns by design, so the tests must not inherit
// the developer's.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "awf-audit-test-home")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
