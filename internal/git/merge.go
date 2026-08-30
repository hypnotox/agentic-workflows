package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	heads, err := MergeHeads(projectRoot)
	return len(heads) > 0, err
}

// MergeHeads returns every worktree-private MERGE_HEAD hash in file order.
// Absence means no merge is in progress and returns a nil slice.
func MergeHeads(projectRoot string) ([]string, error) {
	dir, err := containingGitDir(projectRoot)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "MERGE_HEAD"))
	switch {
	case err == nil:
		var heads []string
		for _, line := range strings.Split(string(data), "\n") {
			if hash := strings.TrimSpace(line); hash != "" {
				heads = append(heads, hash)
			}
		}
		return heads, nil
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	default:
		// Reachable without a fault: a gitdir pointer naming a regular file makes
		// the MERGE_HEAD read report ENOTDIR rather than os.ErrNotExist.
		return nil, err
	}
}

// containingGitDir walks up from projectRoot to the nearest checkout and returns
// its worktree-private Git directory. Presence of `.git` decides where the walk
// stops, so a malformed pointer at a real checkout surfaces as an error instead
// of being masked by continuing upward.
func containingGitDir(projectRoot string) (string, error) {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
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
