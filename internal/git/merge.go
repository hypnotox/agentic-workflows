package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// MergeInProgress reports whether the checkout containing projectRoot has a
// merge in progress, detected by the presence of MERGE_HEAD.
//
// MERGE_HEAD is worktree-private, so it lives in a linked worktree's own gitdir
// rather than the shared common dir, and a project root may sit below the
// checkout root. A naive <root>/.git lookup is wrong for both, so resolution
// walks up to the containing checkout and reuses worktreeGitDir, which already
// resolves a `.git` directory or a validated `gitdir:` pointer (ADR-0182 item 4).
//
// Detection is by repository state rather than by the shape of the staged diff.
// It must stay true through a conflict resolution, where the merge is committed
// by a later `git commit`, and false for a hand-staged commit that merely looks
// branch-sized. `git merge --squash` records no MERGE_HEAD and so reports false,
// which is correct: it produces an ordinary commit carrying no merge provenance.
func MergeInProgress(projectRoot string) (bool, error) {
	dir, err := containingGitDir(projectRoot)
	if err != nil {
		return false, err
	}
	switch _, err = os.Lstat(filepath.Join(dir, "MERGE_HEAD")); {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		// Reachable without a fault: a gitdir pointer naming a regular file makes
		// the MERGE_HEAD lstat report ENOTDIR rather than os.ErrNotExist.
		return false, err
	}
}

// containingGitDir walks up from projectRoot to the nearest checkout and returns
// its worktree-private Git directory. Presence of `.git` decides where the walk
// stops, so a malformed pointer at a real checkout surfaces as an error instead
// of being masked by continuing upward.
func containingGitDir(projectRoot string) (string, error) {
	abs, err := filepath.Abs(projectRoot)
	if err != nil { // coverage-ignore: Abs fails only when the process working directory is unavailable
		return "", err
	}
	for candidate := abs; ; candidate = filepath.Dir(candidate) {
		if _, statErr := os.Stat(filepath.Join(candidate, ".git")); statErr == nil {
			return worktreeGitDir(candidate)
		}
		if parent := filepath.Dir(candidate); parent == candidate {
			return "", fmt.Errorf("no git checkout contains %s", abs)
		}
	}
}
