package main

import (
	"os"
	"testing"
)

// TestMain isolates this package's tests from the host's git environment: a
// throwaway HOME (so git's global config and go-git's global-gitignore read find
// nothing) and neutered global/system git config files. Tests build the state
// they need in temp repos and never read or write the developer's machine.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "awf-test-home")
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
