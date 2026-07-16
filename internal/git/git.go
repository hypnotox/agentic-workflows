// Package git centralises awf's go-git repository access: opening a repository
// tolerantly (linked worktrees, submodules, a stray worktreeConfig extension)
// so every awf command that reads git shares one open path. It reads only; it
// never mutates a repository.
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
)

// ChangedPaths returns the sorted, unique repo-relative paths changed either in
// the staged index (staged) or between the two revisions of rangeSpec ("a..b").
// staged takes precedence; with neither selector the caller should not call
// this. A malformed range or an unresolvable revision is a clear error. It
// reads the repository only.
func ChangedPaths(repoRoot string, staged bool, rangeSpec string) ([]string, error) {
	repo, err := OpenRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	set := map[string]bool{}
	if staged {
		wt, err := repo.Worktree()
		if err != nil { // coverage-ignore: a bare / worktree-less repo is outside awf's intended use
			return nil, err
		}
		status, err := wt.Status()
		if err != nil { // coverage-ignore: Status on a healthy worktree we just opened does not fail
			return nil, err
		}
		for path, st := range status {
			if st.Staging != gogit.Unmodified && st.Staging != gogit.Untracked {
				set[path] = true
			}
		}
	} else {
		from, to, ok := strings.Cut(rangeSpec, "..")
		if !ok {
			return nil, fmt.Errorf("range %q must be <a>..<b>", rangeSpec)
		}
		fromTree, err := treeAt(repo, from)
		if err != nil {
			return nil, err
		}
		toTree, err := treeAt(repo, to)
		if err != nil {
			return nil, err
		}
		changes, err := object.DiffTree(fromTree, toTree)
		if err != nil { // coverage-ignore: diffing two resolved trees does not fail
			return nil, err
		}
		for _, ch := range changes {
			if ch.From.Name != "" {
				set[ch.From.Name] = true
			}
			if ch.To.Name != "" {
				set[ch.To.Name] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// TrackedPaths returns the sorted, unique repo-relative slash paths tracked at
// HEAD. It reads the repository only.
func TrackedPaths(repoRoot string) ([]string, error) {
	repo, err := OpenRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}
	c, err := repo.CommitObject(ref.Hash())
	if err != nil { // coverage-ignore: HEAD resolved above points at a real commit
		return nil, err
	}
	tree, err := c.Tree()
	if err != nil { // coverage-ignore: a resolved commit always yields its tree
		return nil, err
	}
	var out []string
	if ferr := tree.Files().ForEach(func(f *object.File) error {
		out = append(out, f.Name)
		return nil
	}); ferr != nil { // coverage-ignore: the collector callback never returns an error
		return nil, ferr
	}
	sort.Strings(out)
	return out, nil
}

// IndexPaths returns the sorted, unique repo-relative slash paths in the index,
// which is exactly what `git ls-files` reports. It reads the repository only.
//
// This differs from TrackedPaths, which reads HEAD: the index carries a staged
// new file that HEAD does not yet, and drops a staged deletion that HEAD still
// holds. A presence-level scan wired into a pre-commit hook (ADR-0119) needs the
// index set, so a file added in the very commit being made is in scope, and a
// file being deleted is not (reading it would fail).
func IndexPaths(repoRoot string) ([]string, error) {
	repo, err := OpenRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		// OpenRepo does not decode the index; Storer.Index() reads .git/index on
		// first call, so a corrupt index file fails here even though the open
		// succeeded.
		return nil, fmt.Errorf("read index: %w", err)
	}
	out := make([]string, 0, len(idx.Entries))
	for _, e := range idx.Entries {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out, nil
}

// treeAt resolves a revision to its commit tree.
func treeAt(repo *gogit.Repository, rev string) (*object.Tree, error) {
	h, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", rev, err)
	}
	c, err := repo.CommitObject(*h)
	if err != nil { // coverage-ignore: a hash ResolveRevision just returned points at a real object
		return nil, fmt.Errorf("commit %q: %w", rev, err)
	}
	return c.Tree()
}

// OpenRepo opens the repo at repoRoot like git.PlainOpen, but hides its
// [extensions] config section from go-git's own extension-support check
// (repository_extensions.go verifyExtensions). That check has an upstream bug:
// it lowercases the incoming extension name ("worktreeconfig") before comparing
// it against its allow-list, whose key is mixed-case ("worktreeConfig") - the
// lookup never matches, so PlainOpen rejects any repo with
// `extensions.worktreeConfig` set (a flag `git worktree add` can leave behind
// even after the worktree is removed) regardless of repositoryformatversion.
// awf's git-reading commands never read repo extensions, so hiding the section
// is safe.
//
// Unlike git.PlainOpen with default options, this also resolves a `.git`
// *file* - the `gitdir:` pointer `git worktree add` leaves at a linked
// worktree's root (and the submodule layout) - mirroring what
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

// dotGitFs resolves repoRoot's .git - a directory in a primary checkout, a
// `gitdir:` pointer file in a linked worktree or submodule - to the filesystem
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
	// A directory (primary checkout) or missing entirely - go-git reports its
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
	// unreadable one degrades the same way - go-git then errors canonically on
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
