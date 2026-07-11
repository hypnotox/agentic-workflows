// Package git centralises awf's go-git repository access: opening a repository
// tolerantly (linked worktrees, submodules, a stray worktreeConfig extension)
// so every awf command that reads git shares one open path. It reads only; it
// never mutates a repository.
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
)

// OpenRepo opens the repo at repoRoot like git.PlainOpen, but hides its
// [extensions] config section from go-git's own extension-support check
// (repository_extensions.go verifyExtensions). That check has an upstream bug:
// it lowercases the incoming extension name ("worktreeconfig") before comparing
// it against its allow-list, whose key is mixed-case ("worktreeConfig") — the
// lookup never matches, so PlainOpen rejects any repo with
// `extensions.worktreeConfig` set (a flag `git worktree add` can leave behind
// even after the worktree is removed) regardless of repositoryformatversion.
// awf's git-reading commands never read repo extensions, so hiding the section
// is safe.
//
// Unlike git.PlainOpen with default options, this also resolves a `.git`
// *file* — the `gitdir:` pointer `git worktree add` leaves at a linked
// worktree's root (and the submodule layout) — mirroring what
// PlainOpenWithOptions' EnableDotGitCommonDir does, so awf's git-reading
// commands work from a linked worktree. The manual storage construction (over
// PlainOpenWithOptions) exists solely so the storer wrapper above can be
// injected.
func OpenRepo(repoRoot string) (*gogit.Repository, error) {
	dotFs, err := dotGitFs(repoRoot)
	if err != nil {
		return nil, err
	}
	st := filesystem.NewStorage(dotFs, cache.NewObjectLRUDefault())
	return gogit.Open(noExtensionsStorer{st}, osfs.New(repoRoot))
}

// dotGitFs resolves repoRoot's .git — a directory in a primary checkout, a
// `gitdir:` pointer file in a linked worktree or submodule — to the filesystem
// go-git should treat as the repository's dotgit. For a gitdir carrying a
// `commondir` file (the linked-worktree layout), the returned filesystem
// routes worktree-private files (HEAD, index) to the worktree's own gitdir and
// everything shared (objects, refs, config) to the common dir. A missing or
// unreadable .git falls through to the plain path so go-git reports its
// canonical not-a-repository error.
func dotGitFs(repoRoot string) (billy.Filesystem, error) {
	dotPath := filepath.Join(repoRoot, ".git")
	if fi, err := os.Stat(dotPath); err == nil && !fi.IsDir() {
		return gitfileFs(repoRoot, dotPath)
	}
	// A directory (primary checkout) or missing entirely — go-git reports its
	// canonical not-a-repository error downstream.
	return osfs.New(dotPath), nil
}

// gitfileFs resolves a `.git` pointer file to its gitdir's filesystem.
func gitfileFs(repoRoot, dotPath string) (billy.Filesystem, error) {
	raw, err := os.ReadFile(dotPath)
	if err != nil {
		return nil, err
	}
	gitdirPath, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir: ")
	if !ok {
		return nil, fmt.Errorf("parse %s: expected a `gitdir:` pointer (the linked-worktree/submodule layout)", dotPath)
	}
	if !filepath.IsAbs(gitdirPath) {
		gitdirPath = filepath.Join(repoRoot, gitdirPath)
	}
	dot := osfs.New(gitdirPath)
	if commonRaw, cerr := os.ReadFile(filepath.Join(gitdirPath, "commondir")); cerr == nil {
		common := strings.TrimSpace(string(commonRaw))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitdirPath, common)
		}
		return dotgit.NewRepositoryFilesystem(dot, osfs.New(common)), nil
	}
	// No commondir: the gitdir is self-contained (submodule layout). An
	// unreadable one degrades the same way — go-git then errors canonically on
	// the refs it cannot find.
	return dot, nil
}

type noExtensionsStorer struct {
	storage.Storer
}

func (s noExtensionsStorer) Config() (*gitconfig.Config, error) {
	cfg, err := s.Storer.Config()
	if err != nil {
		return nil, err
	}
	cfg.Raw.RemoveSection("extensions")
	return cfg, nil
}
