package main

import (
	"os"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestMain isolates this package's tests from the host by giving them a throwaway
// HOME, so go-git's global-gitignore read (the uncommitted-changes audit rule)
// finds nothing. awf drives git purely through go-git — no host git binary, and
// no host git config — so the tests build their state in temp repos and never
// read or write the developer's machine.
func TestMain(m *testing.M) {
	os.Exit(testsupport.RunIsolated(m, "awf-test-home"))
}
