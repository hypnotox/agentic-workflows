package audit

import (
	"os"
	"testing"
)

// TestMain isolates this package's tests from the host's git environment: a
// throwaway HOME (so go-git's global-gitignore read finds nothing) and neutered
// global/system git config files. The uncommitted-changes rule reads live global
// ignore patterns by design, so the tests must not inherit the developer's.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "awf-audit-test-home")
	if err != nil {
		panic(err)
	}
	for k, v := range map[string]string{
		"HOME":              home,
		"GIT_CONFIG_GLOBAL": os.DevNull,
		"GIT_CONFIG_SYSTEM": os.DevNull,
	} {
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
