// Package git centralises awf's go-git repository access: opening a repository
// tolerantly (linked worktrees, submodules, a stray worktreeConfig extension)
// so every awf command that reads git shares one open path. It reads only; it
// never mutates a repository.
package git

import (
	"errors"
	"fmt"
	"io"
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
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
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
		from, to, perr := ParseRange(rangeSpec, false)
		if perr != nil {
			return nil, perr
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

// HeadExists reports whether the repository has a born HEAD (at least one
// commit). A fresh repository with no commit yet reports false without error, so
// the staged transition check can treat the first commit's before side as the
// empty universe rather than failing to resolve HEAD. It reads the repository
// only.
func HeadExists(repoRoot string) (bool, error) {
	repo, err := OpenRepo(repoRoot)
	if err != nil {
		return false, fmt.Errorf("open repo: %w", err)
	}
	if _, err := repo.Head(); err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("resolve HEAD: %w", err) // coverage-ignore: Head fails only with ErrReferenceNotFound on a healthy repo just opened; other faults require a corrupt ref store
	}
	return true, nil
}

// HeadHash resolves the current HEAD commit hash without requiring a clean
// working tree. The final current-state upgrade runs in an integration worktree
// that carries the applied but uncommitted attestation patches, so it compares
// HEAD identity against the sealed PreparedHead without a cleanliness check.
func HeadHash(repoRoot string) (string, error) {
	repo, err := OpenRepo(repoRoot)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}
	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("resolve HEAD: %w", err)
	}
	return ref.Hash().String(), nil
}

// WorkingPaths returns tracked HEAD paths that still exist plus nonignored
// untracked paths, rerooted to repoRoot. repoRoot may be an adopted project
// nested inside a containing monorepo; paths outside that project are excluded.
// Deleted, ignored, and nested-repository files are excluded by go-git's
// worktree status semantics.
func WorkingPaths(repoRoot string) ([]string, error) {
	repo, prefix, err := openContainingRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil { // coverage-ignore: resolved HEAD points at a commit
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil { // coverage-ignore: a resolved commit always yields its tree
		return nil, err
	}
	set := map[string]bool{}
	if err := tree.Files().ForEach(func(f *object.File) error {
		if path, ok := rerootPath(f.Name, prefix); ok {
			set[path] = true
		}
		return nil
	}); err != nil { // coverage-ignore: collector callback never errors
		return nil, err
	}
	wt, err := repo.Worktree()
	if err != nil { // coverage-ignore: awf operates on non-bare adopted worktrees
		return nil, err
	}
	status, err := wt.Status()
	if err != nil { // coverage-ignore: status on the healthy worktree just opened does not fail
		return nil, err
	}
	for path, state := range status {
		path, ok := rerootPath(path, prefix)
		if !ok {
			continue
		}
		if state.Worktree == gogit.Deleted || state.Staging == gogit.Deleted {
			delete(set, path)
		} else {
			set[path] = true
		}
	}
	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func openContainingRepo(projectRoot string) (*gogit.Repository, string, error) {
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil { // coverage-ignore: Abs fails only when the process working directory is unavailable
		return nil, "", err
	}
	for candidate := projectRoot; ; candidate = filepath.Dir(candidate) {
		repo, openErr := OpenRepo(candidate)
		if openErr == nil {
			prefix, relErr := filepath.Rel(candidate, projectRoot)
			if relErr != nil { // coverage-ignore: both paths are absolute and share the same volume
				return nil, "", relErr
			}
			if prefix == "." {
				prefix = ""
			}
			return repo, filepath.ToSlash(prefix), nil
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return nil, "", openErr
		}
	}
}

func rerootPath(path, prefix string) (string, bool) {
	if prefix == "" {
		return path, true
	}
	return strings.CutPrefix(path, prefix+"/")
}

// ErrIndexUnmerged reports an index that has multiple merge stages and cannot
// represent one deterministic pre-commit snapshot.
var ErrIndexUnmerged = errors.New("index contains unmerged entries")

// ErrIndexBlob reports a stage-0 regular-file entry whose content cannot be
// read from the object store.
var ErrIndexBlob = errors.New("read index blob")

// IndexBlob is one regular file's exact bytes from a stage-0 index or a
// resolved commit tree. Executable reports whether the entry carries the
// executable file mode, so a caller can preserve mode without re-reading the
// source.
type IndexBlob struct {
	Path       string
	Bytes      []byte
	Executable bool
}

// IndexBlobs returns sorted stage-0 ordinary and executable blobs from the
// index. Symlinks and gitlinks have no regular-file content to scan and are
// ignored. An unmerged or unreadable regular entry makes the snapshot unsafe.
func IndexBlobs(repoRoot string) ([]IndexBlob, error) {
	repo, err := OpenRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}
	entries := append([]*index.Entry(nil), idx.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	out := make([]IndexBlob, 0, len(entries))
	for _, e := range entries {
		if e.Stage != 0 {
			return nil, fmt.Errorf("%w: %s", ErrIndexUnmerged, e.Name)
		}
		if e.Mode != filemode.Regular && e.Mode != filemode.Executable {
			continue
		}
		blob, err := repo.BlobObject(e.Hash)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrIndexBlob, e.Name, err)
		}
		r, err := blob.Reader()
		if err != nil { // coverage-ignore: a blob object successfully loaded from go-git's object store always supplies a reader
			return nil, fmt.Errorf("%w: %s: %w", ErrIndexBlob, e.Name, err)
		}
		b, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if readErr != nil { // coverage-ignore: reads from an in-memory git blob reader do not fail
			return nil, fmt.Errorf("%w: %s: %w", ErrIndexBlob, e.Name, readErr)
		}
		if closeErr != nil { // coverage-ignore: go-git's read-only blob reader has no close failure
			return nil, fmt.Errorf("%w: %s: %w", ErrIndexBlob, e.Name, closeErr)
		}
		out = append(out, IndexBlob{Path: e.Name, Bytes: b, Executable: e.Mode == filemode.Executable})
	}
	return out, nil
}

// CommitBlobs returns the sorted regular and executable blobs of the tree that
// rev resolves to. Symlinks and gitlinks carry no regular-file content to scan
// and are skipped. It reads the repository only.
func CommitBlobs(repoRoot, rev string) ([]IndexBlob, error) {
	repo, err := OpenRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	tree, err := treeAt(repo, rev)
	if err != nil {
		return nil, err
	}
	return blobsOfTree(tree)
}

// RangeBlobs returns the before/after regular-blob sets for the transition into
// the commit rev resolves to: after is that commit's tree, before is its
// first-parent tree, or nil for a root commit. Merges follow the first parent
// only, so an ADR status change committed on a branch and merged is still
// observed at the merge. It reads the repository only.
func RangeBlobs(repoRoot, rev string) (before, after []IndexBlob, err error) {
	repo, err := OpenRepo(repoRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("open repo: %w", err)
	}
	h, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, nil, fmt.Errorf("resolve %q: %w", rev, err)
	}
	c, err := repo.CommitObject(*h)
	if err != nil { // coverage-ignore: a hash ResolveRevision just returned points at a real object
		return nil, nil, fmt.Errorf("commit %q: %w", rev, err)
	}
	curTree, err := c.Tree()
	if err != nil { // coverage-ignore: a resolved commit always yields its tree
		return nil, nil, err
	}
	if after, err = blobsOfTree(curTree); err != nil { // coverage-ignore: reading in-memory blobs from a resolved tree does not fail
		return nil, nil, err
	}
	if c.NumParents() > 0 {
		parent, perr := c.Parent(0)
		if perr != nil { // coverage-ignore: parent count was just checked > 0; the parent object exists
			return nil, nil, perr
		}
		parentTree, perr := parent.Tree()
		if perr != nil { // coverage-ignore: a valid parent commit's tree resolves
			return nil, nil, perr
		}
		if before, perr = blobsOfTree(parentTree); perr != nil { // coverage-ignore: reading in-memory blobs from a resolved tree does not fail
			return nil, nil, perr
		}
	}
	return before, after, nil
}

// blobsOfTree collects the sorted regular and executable blobs of a resolved
// tree. Symlinks and gitlinks are skipped; the executable mode is preserved.
func blobsOfTree(tree *object.Tree) ([]IndexBlob, error) {
	var out []IndexBlob
	err := tree.Files().ForEach(func(f *object.File) error {
		if f.Mode != filemode.Regular && f.Mode != filemode.Executable {
			return nil
		}
		reader, err := f.Reader()
		if err != nil { // coverage-ignore: a tree file always supplies its blob reader
			return err
		}
		data, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil { // coverage-ignore: in-memory object readers do not fail
			return err
		}
		if closeErr != nil { // coverage-ignore: in-memory object readers do not fail
			return closeErr
		}
		out = append(out, IndexBlob{Path: f.Name, Bytes: data, Executable: f.Mode == filemode.Executable})
		return nil
	})
	if err != nil { // coverage-ignore: the callback only returns the impossible blob-reader faults excluded above
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
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
