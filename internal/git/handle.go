package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
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
	root       string
	prefix     string
	repo       *gogit.Repository
	runner     runner
	createTemp func(string, string) (trustFile, error)
	removeFile func(string) error
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
		if !errors.Is(openErr, gogit.ErrRepositoryNotExists) {
			return nil, "", translateOpenError(openErr)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return nil, "", ErrNotARepository
		}
	}
}

// translateOpenError replaces backend identities with seam-owned errors. Backend
// text remains useful for diagnosis, but callers cannot couple to go-git.
func translateOpenError(err error) error {
	if errors.Is(err, gogit.ErrRepositoryNotExists) {
		return ErrNotARepository
	}
	return opaqueError(err)
}

func opaqueError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return errors.New(err.Error())
}

func opaqueWrap(message string, err error) error {
	if contextErr := opaqueError(err); errors.Is(contextErr, context.Canceled) || errors.Is(contextErr, context.DeadlineExceeded) {
		return contextErr
	}
	return errors.New(message + ": " + err.Error())
}

func checkContext(ctx context.Context) error {
	return ctx.Err()
}

// Root is the path the handle is anchored at: the directory whose subtree the
// handle's reads are scoped to, which a consumer joining repository-relative
// paths back onto disk needs.
func (r *Repo) Root() string { return r.root }

// IsNested reports whether the handle is anchored below its containing
// repository root. Consumers use this composition fact to preserve output
// ownership boundaries without reopening the repository.
func (r *Repo) IsNested() bool { return r.prefix != "" }

// IndexPaths returns sorted, unique stage paths from index metadata, rerooted
// to the handle's project subtree. It deliberately neither consults ignore
// rules nor reads blob objects: index membership answers tracking even when a
// path is now ignored or its blob cannot be read. Repeated conflict stages
// collapse to their one path because tracking is path membership, not content.
func (r *Repo) IndexPaths(ctx context.Context) ([]string, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	idx, err := r.repo.Storer.Index()
	if err != nil {
		return nil, opaqueWrap("read index", err)
	}
	set := map[string]bool{}
	for _, entry := range idx.Entries {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		if path, ok := rerootPath(entry.Name, r.prefix); ok {
			set[path] = true
		}
	}
	return sortedPaths(set), nil
}

// ChangedPaths returns the sorted, unique repo-relative paths changed either in
// the staged index (staged) or between the two revisions of rangeSpec ("a..b").
// staged takes precedence; with neither selector the caller should not call
// this. A malformed range or an unresolvable revision is a clear error. It
// reads the repository only.
func (r *Repo) ChangedPaths(ctx context.Context, staged bool, rangeSpec string) ([]string, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	set := map[string]bool{}
	if staged {
		wt, err := r.repo.Worktree()
		if err != nil { // coverage-ignore: a bare / worktree-less repo is outside awf's intended use
			return nil, opaqueError(err)
		}
		status, err := wt.Status()
		if err != nil { // coverage-ignore: Status on a healthy worktree we just opened does not fail
			return nil, opaqueError(err)
		}
		for path, st := range status {
			if err := checkContext(ctx); err != nil {
				return nil, err
			}
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
			return nil, opaqueError(err)
		}
		toTree, err := treeAt(r.repo, to)
		if err != nil {
			return nil, opaqueError(err)
		}
		changes, err := object.DiffTree(fromTree, toTree)
		if err != nil { // coverage-ignore: diffing two resolved trees does not fail
			return nil, opaqueError(err)
		}
		for _, ch := range changes {
			if err := checkContext(ctx); err != nil {
				return nil, err
			}
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
func (r *Repo) HeadExists(ctx context.Context) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	head, err := resolveHead(r.repo)
	if err != nil {
		return false, opaqueError(err)
	}
	if head.unborn {
		return false, nil
	}
	if _, err := r.repo.CommitObject(head.ref.Hash()); err != nil {
		return false, opaqueWrap("resolve HEAD commit", err)
	}
	return true, nil
}

// HeadHash resolves the current HEAD commit hash without requiring a clean
// working tree. The final current-state upgrade runs in an integration worktree
// that carries the applied but uncommitted attestation patches, so it compares
// HEAD identity against the sealed PreparedHead without a cleanliness check.
func (r *Repo) HeadHash(ctx context.Context) (string, error) {
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	ref, err := r.repo.Head()
	if err != nil {
		return "", opaqueWrap("resolve HEAD", err)
	}
	return ref.Hash().String(), nil
}

// Branches returns the repository's local branch short names.
func (r *Repo) Branches(ctx context.Context) (map[string]bool, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	iter, err := r.repo.Branches()
	if err != nil {
		return nil, opaqueError(err)
	}
	defer iter.Close()
	branches := map[string]bool{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if err := checkContext(ctx); err != nil {
			return err
		}
		branches[ref.Name().Short()] = true
		return nil
	})
	return branches, opaqueError(err)
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
func (r *Repo) WorkingPaths(ctx context.Context) ([]string, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	head, err := resolveHead(r.repo)
	if err != nil {
		return nil, opaqueError(err)
	}
	set := map[string]bool{}
	if !head.unborn {
		commit, err := r.repo.CommitObject(head.ref.Hash())
		if err != nil {
			return nil, opaqueWrap(fmt.Sprintf("resolve working paths HEAD commit %s", head.ref.Hash()), err)
		}
		tree, err := commit.Tree()
		if err != nil {
			return nil, opaqueWrap(fmt.Sprintf("resolve working paths HEAD tree %s", commit.TreeHash), err)
		}
		if err := tree.Files().ForEach(func(f *object.File) error {
			if err := checkContext(ctx); err != nil {
				return err
			}
			if path, ok := rerootPath(f.Name, r.prefix); ok {
				set[path] = true
			}
			return nil
		}); err != nil {
			return nil, opaqueError(err)
		}
	}
	wt, err := r.repo.Worktree()
	if err != nil { // coverage-ignore: awf operates on non-bare adopted worktrees
		return nil, opaqueError(err)
	}
	wt.Excludes = globalExcludePatterns()
	status, err := wt.Status()
	if err != nil { // coverage-ignore: status on the healthy worktree just opened does not fail
		return nil, opaqueError(err)
	}
	patterns, err := gitignore.ReadPatterns(wt.Filesystem, nil)
	if err != nil { // coverage-ignore: Status read the same worktree immediately above; failure requires a concurrent filesystem fault
		return nil, opaqueError(err)
	}
	patterns = append(globalExcludePatterns(), patterns...)
	status = visibleWorkingStatus(status, gitignore.NewMatcher(patterns))
	for path, state := range status {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
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

// visibleWorkingStatus closes go-git's ignored-parent gap without suppressing
// tracked modifications or deletions: only untracked entries that the complete
// repository/system/global matcher rejects are removed.
func visibleWorkingStatus(status gogit.Status, ignored gitignore.Matcher) gogit.Status {
	visible := make(gogit.Status, len(status))
	for path, state := range status {
		if state.Worktree == gogit.Untracked && ignored.Match(strings.Split(filepath.ToSlash(path), "/"), false) {
			continue
		}
		visible[path] = state
	}
	return visible
}

// IndexBlobs returns sorted stage-0 ordinary and executable blobs from the
// index. Symlinks and gitlinks have no regular-file content to scan and are
// ignored. An unmerged or unreadable regular entry makes the snapshot unsafe.
func (r *Repo) IndexBlobs(ctx context.Context) ([]IndexBlob, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	idx, err := r.repo.Storer.Index()
	if err != nil {
		return nil, opaqueWrap("read index", err)
	}
	entries := append([]*index.Entry(nil), idx.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	out := make([]IndexBlob, 0, len(entries))
	for _, e := range entries {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
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
			return nil, fmt.Errorf("%w: %s: %s", ErrIndexBlob, e.Name, err.Error())
		}
		out = append(out, IndexBlob{Path: path, Bytes: blob, Mode: blobModeOf(e.Mode)})
	}
	return out, nil
}

// CommitParents returns the full parent hashes of rev in recorded order.
func (r *Repo) CommitParents(ctx context.Context, rev string) ([]string, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	commit, err := r.resolveCommit(rev)
	if err != nil {
		return nil, err
	}
	parents := make([]string, len(commit.ParentHashes))
	for i, hash := range commit.ParentHashes {
		parents[i] = hash.String()
	}
	return parents, nil
}

// CommitMessage returns the full message recorded by rev.
func (r *Repo) CommitMessage(ctx context.Context, rev string) (string, error) {
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	commit, err := r.resolveCommit(rev)
	if err != nil {
		return "", err
	}
	return commit.Message, nil
}

func (r *Repo) resolveCommit(rev string) (*object.Commit, error) {
	hash, err := r.repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, opaqueWrap(fmt.Sprintf("resolve %q", rev), err)
	}
	commit, err := r.repo.CommitObject(*hash)
	if err != nil { // coverage-ignore: ResolveRevision accepts only commitish revisions; the non-commit-object test proves blobs refuse before this lookup
		return nil, opaqueWrap(fmt.Sprintf("commit %q", rev), err)
	}
	return commit, nil
}

// CommitEntries returns the sorted metadata for every regular, executable, or
// symlink entry in rev's tree scoped to the handle's project root. It does not
// read blob objects; gitlinks and unsupported entries are omitted.
func (r *Repo) CommitEntries(ctx context.Context, rev string) ([]TreeEntry, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	tree, err := r.commitTree(rev)
	if err != nil {
		return nil, err
	}
	entries, err := treeEntries(ctx, tree, r.prefix)
	if err != nil {
		return nil, opaqueError(err)
	}
	return entries, nil
}

// CommitBlobsAt returns the sorted exact blobs selected by canonical,
// project-relative paths in rev. Every requested path must name a regular,
// executable, or symlink entry in the handle's project subtree.
func (r *Repo) CommitBlobsAt(ctx context.Context, rev string, paths []string) ([]IndexBlob, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(paths))
	for _, projectPath := range paths {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		if !safeProjectPath(projectPath) {
			return nil, fmt.Errorf("select commit blob: unsafe path %q", projectPath)
		}
		if seen[projectPath] {
			return nil, fmt.Errorf("select commit blob: duplicate path %q", projectPath)
		}
		seen[projectPath] = true
	}
	if len(paths) == 0 {
		return []IndexBlob{}, nil
	}
	tree, err := r.commitTree(rev)
	if err != nil {
		return nil, err
	}
	out := make([]IndexBlob, 0, len(paths))
	for _, projectPath := range paths {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		entry, err := tree.FindEntry(prefixedPath(r.prefix, projectPath))
		if err != nil {
			return nil, opaqueWrap(fmt.Sprintf("select commit blob %q", projectPath), err)
		}
		if entry.Mode != filemode.Regular && entry.Mode != filemode.Executable && entry.Mode != filemode.Symlink {
			return nil, fmt.Errorf("select commit blob: unsupported entry %q", projectPath)
		}
		bytes, err := readBlob(r.repo, entry.Hash)
		if err != nil {
			return nil, opaqueWrap(fmt.Sprintf("read commit blob %q", projectPath), err)
		}
		out = append(out, IndexBlob{Path: projectPath, Bytes: bytes, Mode: blobModeOf(entry.Mode)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// commitTree resolves rev to its tree through the seam's opaque error
// translation.
func (r *Repo) commitTree(rev string) (*object.Tree, error) {
	commit, err := r.resolveCommit(rev)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, opaqueWrap(fmt.Sprintf("tree %q", rev), err)
	}
	return tree, nil
}

// treeEntries walks tree metadata without constructing a file or blob object.
func treeEntries(ctx context.Context, tree *object.Tree, prefix string) ([]TreeEntry, error) {
	if prefix != "" {
		for _, segment := range strings.Split(prefix, "/") {
			if err := checkContext(ctx); err != nil {
				return nil, err
			}
			entryIndex := slices.IndexFunc(tree.Entries, func(entry object.TreeEntry) bool { return entry.Name == segment })
			if entryIndex < 0 || tree.Entries[entryIndex].Mode != filemode.Dir {
				return []TreeEntry{}, nil
			}
			var err error
			tree, err = tree.Tree(segment)
			if err != nil {
				return nil, err
			}
		}
	}
	out := []TreeEntry{}
	var walk func(*object.Tree, string) error
	walk = func(current *object.Tree, base string) error {
		for _, entry := range current.Entries {
			if err := checkContext(ctx); err != nil {
				return err
			}
			fullPath := prefixedPath(base, entry.Name)
			if !safeProjectPath(fullPath) {
				return fmt.Errorf("read commit entries: unsafe tree path %q", fullPath)
			}
			if entry.Mode == filemode.Dir {
				directory, err := current.Tree(entry.Name)
				if err != nil {
					return err
				}
				if err := walk(directory, fullPath); err != nil {
					return err
				}
				continue
			}
			if entry.Mode == filemode.Regular || entry.Mode == filemode.Executable || entry.Mode == filemode.Symlink {
				out = append(out, TreeEntry{Path: fullPath, Mode: blobModeOf(entry.Mode)})
			}
		}
		return nil
	}
	if err := walk(tree, ""); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func safeProjectPath(p string) bool {
	if p == "" || strings.ContainsRune(p, '\\') || pathpkg.IsAbs(p) || p != pathpkg.Clean(p) {
		return false
	}
	return p != "." && p != ".." && !strings.HasPrefix(p, "../")
}

func prefixedPath(prefix, p string) string {
	if prefix == "" {
		return p
	}
	return prefix + "/" + p
}

// CommitBlobs returns the sorted regular and executable blobs of the tree that
// rev resolves to. Symlinks and gitlinks carry no regular-file content to scan
// and are skipped. It reads the repository only.
func (r *Repo) CommitBlobs(ctx context.Context, rev string) ([]IndexBlob, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	tree, err := treeAt(r.repo, rev)
	if err != nil {
		return nil, opaqueError(err)
	}
	blobs, err := blobsOfTree(ctx, tree, r.prefix)
	return blobs, opaqueError(err)
}

// RangeBlobs returns the before/after regular-blob sets for the transition into
// the commit rev resolves to: after is that commit's tree, before is its
// first-parent tree, or nil for a root commit. Merges follow the first parent
// only, so an ADR status change committed on a branch and merged is still
// observed at the merge. It reads the repository only.
func (r *Repo) RangeBlobs(ctx context.Context, rev string) (before, after []IndexBlob, err error) {
	if err := checkContext(ctx); err != nil {
		return nil, nil, err
	}
	h, err := r.repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, nil, opaqueWrap(fmt.Sprintf("resolve %q", rev), err)
	}
	c, err := r.repo.CommitObject(*h)
	if err != nil { // coverage-ignore: a hash ResolveRevision just returned points at a real object
		return nil, nil, opaqueWrap(fmt.Sprintf("commit %q", rev), err)
	}
	curTree, err := c.Tree()
	if err != nil { // coverage-ignore: a resolved commit always yields its tree
		return nil, nil, opaqueError(err)
	}
	if after, err = blobsOfTree(ctx, curTree, r.prefix); err != nil {
		return nil, nil, opaqueError(err)
	}
	if c.NumParents() > 0 {
		parent, perr := c.Parent(0)
		if perr != nil {
			// Reachable, and not only on a corrupt repository: NumParents counts
			// recorded parent hashes, while resolving one is an object lookup, so
			// a shallow clone's boundary commit fails here on a healthy checkout.
			return nil, nil, opaqueError(perr)
		}
		parentTree, perr := parent.Tree()
		if perr != nil { // coverage-ignore: a valid parent commit's tree resolves
			return nil, nil, opaqueError(perr)
		}
		if before, perr = blobsOfTree(ctx, parentTree, r.prefix); perr != nil {
			return nil, nil, opaqueError(perr)
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
