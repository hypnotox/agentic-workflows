package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ErrNotARepository reports a path that carries no Git repository. It is the
// seam's own not-a-repository identity: a consumer matches it with errors.Is
// and never sees the backend sentinel it translates, so which backend answered
// the open stays an implementation detail.
var ErrNotARepository = errors.New("not a git repository")

// Repo is the seam's handle on one opened repository. It is constructed once,
// at a composition point that owns a validated root, and every read entrypoint
// hangs off it as a method taking the operation's context: no entrypoint
// re-opens the repository, and no backend type crosses the handle's surface.
//
// prefix is the repository-relative slash-separated path of root, empty when
// root is the repository root itself. It is what lets an adopted project nested
// inside a containing monorepo read only its own paths.
type Repo struct {
	root   string
	prefix string
	repo   *gogit.Repository
	runner runner
}

// Open opens the repository at root exactly, tolerating the layouts awf must
// read (a linked worktree's `gitdir:` pointer, a submodule, a stray
// `extensions.worktreeConfig`), and validates root once so every later
// operation reuses that validation. A path carrying no repository at all is
// reported as ErrNotARepository; a present but malformed checkout keeps its own
// error, because the two must never be confused by a caller deciding whether a
// checkout exists.
func Open(root string) (*Repo, error) {
	absolute, err := filepath.Abs(root)
	if err != nil { // coverage-ignore: Abs fails only when the process working directory is unavailable
		return nil, err
	}
	repo, err := openTolerant(absolute)
	if err != nil {
		return nil, translateOpenError(err)
	}
	return &Repo{root: absolute, repo: repo, runner: newRunner(absolute)}, nil
}

// OpenContaining opens the repository containing start and reports start's
// repository-relative slash-separated prefix, empty when start is itself the
// repository root. The returned handle stays anchored at start, so its reads
// are scoped to that subtree exactly as a command invoked there expects.
func OpenContaining(start string) (*Repo, string, error) {
	absolute, err := filepath.Abs(start)
	if err != nil { // coverage-ignore: Abs fails only when the process working directory is unavailable
		return nil, "", err
	}
	for candidate := absolute; ; candidate = filepath.Dir(candidate) {
		repo, openErr := openTolerant(candidate)
		if openErr == nil {
			prefix, relErr := filepath.Rel(candidate, absolute)
			if relErr != nil { // coverage-ignore: both paths are absolute and share the same volume
				return nil, "", relErr
			}
			if prefix == "." {
				prefix = ""
			}
			prefix = filepath.ToSlash(prefix)
			return &Repo{root: absolute, prefix: prefix, repo: repo, runner: newRunner(absolute)}, prefix, nil
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return nil, "", translateOpenError(openErr)
		}
	}
}

// translateOpenError replaces the backend's canonical not-a-repository sentinel
// with the seam's own identity and leaves every other failure intact.
func translateOpenError(err error) error {
	if errors.Is(err, gogit.ErrRepositoryNotExists) {
		return ErrNotARepository
	}
	return err
}

// Root is the path the handle is anchored at: the directory whose subtree the
// handle's reads are scoped to, which a consumer joining repository-relative
// paths back onto disk needs.
func (r *Repo) Root() string { return r.root }

// ChangedPaths returns the sorted, unique repo-relative paths changed either in
// the staged index (staged) or between the two revisions of rangeSpec ("a..b").
// staged takes precedence; with neither selector the caller should not call
// this. A malformed range or an unresolvable revision is a clear error. It
// reads the repository only.
func (r *Repo) ChangedPaths(_ context.Context, staged bool, rangeSpec string) ([]string, error) {
	set := map[string]bool{}
	if staged {
		wt, err := r.repo.Worktree()
		if err != nil { // coverage-ignore: a bare / worktree-less repo is outside awf's intended use
			return nil, err
		}
		status, err := wt.Status()
		if err != nil { // coverage-ignore: Status on a healthy worktree we just opened does not fail
			return nil, err
		}
		for path, st := range status {
			if path, ok := rerootPath(path, r.prefix); ok && st.Staging != gogit.Unmodified && st.Staging != gogit.Untracked {
				set[path] = true
			}
		}
	} else {
		from, to, perr := ParseRange(rangeSpec, false)
		if perr != nil {
			return nil, perr
		}
		fromTree, err := treeAt(r.repo, from)
		if err != nil {
			return nil, err
		}
		toTree, err := treeAt(r.repo, to)
		if err != nil {
			return nil, err
		}
		changes, err := object.DiffTree(fromTree, toTree)
		if err != nil { // coverage-ignore: diffing two resolved trees does not fail
			return nil, err
		}
		for _, ch := range changes {
			if path, ok := rerootPath(ch.From.Name, r.prefix); ok && ch.From.Name != "" {
				set[path] = true
			}
			if path, ok := rerootPath(ch.To.Name, r.prefix); ok && ch.To.Name != "" {
				set[path] = true
			}
		}
	}
	return sortedPaths(set), nil
}

// HeadExists reports whether the repository has a born HEAD (at least one
// commit). A fresh repository whose immediate symbolic HEAD target is absent
// reports false without error. Missing refs deeper in a symbolic chain and
// corrupt or cyclic chains are errors. It reads the repository only.
func (r *Repo) HeadExists(_ context.Context) (bool, error) {
	head, err := resolveHead(r.repo)
	if err != nil {
		return false, err
	}
	if head.unborn {
		return false, nil
	}
	if _, err := r.repo.CommitObject(head.ref.Hash()); err != nil {
		return false, fmt.Errorf("resolve HEAD commit: %w", err)
	}
	return true, nil
}

// HeadHash resolves the current HEAD commit hash without requiring a clean
// working tree. The final current-state upgrade runs in an integration worktree
// that carries the applied but uncommitted attestation patches, so it compares
// HEAD identity against the sealed PreparedHead without a cleanliness check.
func (r *Repo) HeadHash(_ context.Context) (string, error) {
	ref, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("resolve HEAD: %w", err)
	}
	return ref.Hash().String(), nil
}

// Branches returns the repository's local branch short names.
func (r *Repo) Branches(_ context.Context) (map[string]bool, error) {
	iter, err := r.repo.Branches()
	if err != nil { // coverage-ignore: go-git returns an iterator over the validated reference storer without a reachable failure
		return nil, err
	}
	defer iter.Close()
	branches := map[string]bool{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		branches[ref.Name().Short()] = true
		return nil
	})
	return branches, err
}

// ChangeCounts returns native Git's tracked-change and nonignored untracked-file
// counts for the handle's worktree. Native porcelain is the cleanliness oracle
// because go-git's status traversal can re-include a nested .gitignore below an
// ignored parent directory. It runs through the package runner, so it inherits
// the isolated environment and reports a failure with Git's own stderr; the
// isolation's stripped user and system config is restored for ignore purposes
// alone by replaying the effective core.excludesFile (see excludesFileArgs), so
// the oracle keeps real Git's ignore universe.
func (r *Repo) ChangeCounts(ctx context.Context) (tracked, untracked int, err error) {
	argv := append(r.runner.excludesFileArgs(ctx), "--no-optional-locks", "status", "--porcelain=v2", "-z", "--untracked-files=all")
	out, err := r.runner.run(ctx, argv...)
	if err != nil {
		return 0, 0, fmt.Errorf("read native Git worktree status: %w", err)
	}
	return parseWorktreeStatus(out)
}

// WorkingPaths returns tracked HEAD paths that still exist plus nonignored
// untracked paths, rerooted to the handle's root. A specifically unborn HEAD
// supplies an empty committed baseline; every other repository, reference, or
// object error still fails. The root may be an adopted project nested inside a
// containing monorepo; paths outside that project are excluded. Deleted and
// nested-repository files are excluded by go-git's worktree status semantics;
// ignored files are excluded by those semantics plus the injected global and
// system excludes (globalExcludePatterns).
func (r *Repo) WorkingPaths(_ context.Context) ([]string, error) {
	head, err := resolveHead(r.repo)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	if !head.unborn {
		commit, err := r.repo.CommitObject(head.ref.Hash())
		if err != nil {
			return nil, fmt.Errorf("resolve working paths HEAD commit %s: %w", head.ref.Hash(), err)
		}
		tree, err := commit.Tree()
		if err != nil {
			return nil, fmt.Errorf("resolve working paths HEAD tree %s: %w", commit.TreeHash, err)
		}
		if err := tree.Files().ForEach(func(f *object.File) error {
			if path, ok := rerootPath(f.Name, r.prefix); ok {
				set[path] = true
			}
			return nil
		}); err != nil { // coverage-ignore: collector callback never errors
			return nil, err
		}
	}
	wt, err := r.repo.Worktree()
	if err != nil { // coverage-ignore: awf operates on non-bare adopted worktrees
		return nil, err
	}
	wt.Excludes = globalExcludePatterns()
	status, err := wt.Status()
	if err != nil { // coverage-ignore: status on the healthy worktree just opened does not fail
		return nil, err
	}
	for path, state := range status {
		path, ok := rerootPath(path, r.prefix)
		if !ok {
			continue
		}
		_, diskErr := os.Lstat(filepath.Join(r.root, filepath.FromSlash(path)))
		switch {
		case state.Worktree == gogit.Deleted || os.IsNotExist(diskErr):
			delete(set, path)
		case diskErr != nil: // coverage-ignore: status returned the path; only a concurrent filesystem fault can make Lstat fail otherwise
			return nil, diskErr
		default:
			set[path] = true
		}
	}
	return sortedPaths(set), nil
}

// IndexBlobs returns sorted stage-0 ordinary and executable blobs from the
// index. Symlinks and gitlinks have no regular-file content to scan and are
// ignored. An unmerged or unreadable regular entry makes the snapshot unsafe.
func (r *Repo) IndexBlobs(_ context.Context) ([]IndexBlob, error) {
	idx, err := r.repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}
	entries := append([]*index.Entry(nil), idx.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	out := make([]IndexBlob, 0, len(entries))
	for _, e := range entries {
		path, ok := rerootPath(e.Name, r.prefix)
		if !ok {
			continue
		}
		if e.Stage != 0 {
			return nil, fmt.Errorf("%w: %s", ErrIndexUnmerged, e.Name)
		}
		if e.Mode != filemode.Regular && e.Mode != filemode.Executable && e.Mode != filemode.Symlink {
			continue
		}
		blob, err := readBlob(r.repo, e.Hash)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrIndexBlob, e.Name, err)
		}
		out = append(out, IndexBlob{Path: path, Bytes: blob, Mode: blobModeOf(e.Mode)})
	}
	return out, nil
}

// CommitBlobs returns the sorted regular and executable blobs of the tree that
// rev resolves to. Symlinks and gitlinks carry no regular-file content to scan
// and are skipped. It reads the repository only.
func (r *Repo) CommitBlobs(_ context.Context, rev string) ([]IndexBlob, error) {
	tree, err := treeAt(r.repo, rev)
	if err != nil {
		return nil, err
	}
	return blobsOfTree(tree, r.prefix)
}

// RangeBlobs returns the before/after regular-blob sets for the transition into
// the commit rev resolves to: after is that commit's tree, before is its
// first-parent tree, or nil for a root commit. Merges follow the first parent
// only, so an ADR status change committed on a branch and merged is still
// observed at the merge. It reads the repository only.
func (r *Repo) RangeBlobs(_ context.Context, rev string) (before, after []IndexBlob, err error) {
	h, err := r.repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, nil, fmt.Errorf("resolve %q: %w", rev, err)
	}
	c, err := r.repo.CommitObject(*h)
	if err != nil { // coverage-ignore: a hash ResolveRevision just returned points at a real object
		return nil, nil, fmt.Errorf("commit %q: %w", rev, err)
	}
	curTree, err := c.Tree()
	if err != nil { // coverage-ignore: a resolved commit always yields its tree
		return nil, nil, err
	}
	if after, err = blobsOfTree(curTree, r.prefix); err != nil { // coverage-ignore: reading in-memory blobs from a resolved tree does not fail
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
		if before, perr = blobsOfTree(parentTree, r.prefix); perr != nil { // coverage-ignore: reading in-memory blobs from a resolved tree does not fail
			return nil, nil, perr
		}
	}
	return before, after, nil
}

// sortedPaths flattens a path set into the sorted slice every path-returning
// entrypoint promises.
func sortedPaths(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}
