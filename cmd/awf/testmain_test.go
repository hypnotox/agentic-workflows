package main

import (
	"os"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestMain isolates this package's tests from the host by giving them a
// throwaway HOME, so the global-gitignore read behind the uncommitted-changes
// audit rule finds nothing. awf reaches git only through internal/git, whose
// native invocations run under an isolated environment carrying no ambient host
// git config, and this package's fixtures come from internal/testsupport/
// gitfixture, which isolates its own lane the same way. The tests therefore
// build their state in temp repos and never read or write the developer's
// machine.
func TestMain(m *testing.M) {
	os.Exit(testsupport.RunIsolated(m))
}
