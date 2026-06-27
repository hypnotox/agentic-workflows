package main

import "github.com/go-git/go-git/v5"

// awf interacts with git through go-git, never by shelling the host git binary,
// so neither the tool nor its tests require a git executable on PATH.

// openWorktree opens the git repository containing dir (searching upward for a
// .git, the job of `git rev-parse --show-toplevel`) and returns the repository,
// its work-tree root, and whether dir is inside a repository at all.
func openWorktree(dir string) (*git.Repository, string, bool) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, "", false
	}
	wt, err := repo.Worktree()
	if err != nil { // coverage-ignore: a bare repo has no worktree; awf operates on normal work trees
		return nil, "", false
	}
	return repo, wt.Filesystem.Root(), true
}

// localHooksPath returns the repository-local core.hooksPath, or "" when unset.
// Only the repo's own .git/config is read, never the user's global/system config.
func localHooksPath(repo *git.Repository) string {
	cfg, err := repo.Config()
	if err != nil { // coverage-ignore: Config reads the .git/config of an already-opened repo
		return ""
	}
	return cfg.Raw.Section("core").Option("hooksPath")
}

// writeLocalHooksPath sets the repository-local core.hooksPath to value, or
// removes it when value is empty. Unrelated config options are preserved
// (go-git round-trips the full config via its raw representation).
func writeLocalHooksPath(repo *git.Repository, value string) error {
	cfg, err := repo.Config()
	if err != nil { // coverage-ignore: Config reads the .git/config of an already-opened repo
		return err
	}
	if value == "" {
		cfg.Raw.Section("core").RemoveOption("hooksPath")
	} else {
		cfg.Raw.Section("core").SetOption("hooksPath", value)
	}
	return repo.SetConfig(cfg)
}
