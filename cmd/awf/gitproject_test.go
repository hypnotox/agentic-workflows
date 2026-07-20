package main

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// gitProjectFiles creates a git-backed project root: a fresh repo with a base
// commit (so the working Tree resolves HEAD), the given config under .awf, and
// the given repo-relative files. Everything but the base README stays
// untracked-nonignored, so the working Tree includes it. The commands that read
// a working Tree (check, context, invariants) require this over a plain temp dir.
func gitProjectFiles(t *testing.T, configYAML string, files map[string]string) string {
	t.Helper()
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, configYAML)
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(dir, filepath.FromSlash(rel)), body)
	}
	return dir
}

// syncedGitProject creates a git-backed project and runs sync so the tree is
// drift-clean, the common fixture for runCheck's clean path.
func syncedGitProject(t *testing.T, configYAML string) string {
	return syncedGitProjectFiles(t, configYAML, nil)
}

// syncedGitProjectFiles writes the given files, then syncs, so the tree is
// drift-clean with the authored files (domains, topics, ADRs) in place.
func syncedGitProjectFiles(t *testing.T, configYAML string, files map[string]string) string {
	t.Helper()
	dir := gitProjectFiles(t, configYAML, files)
	if err := initializeProject(dir, io.Discard); err != nil {
		t.Fatalf("runSync: %v", err)
	}
	return dir
}
