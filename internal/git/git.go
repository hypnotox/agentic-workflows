// Package git is awf's one semantic git seam: every git capability the
// application needs is an entrypoint here, and which backend answers an
// entrypoint (in-process object reads, or native Git through the package
// runner) is an implementation detail no consumer can observe. A handle opened
// with Open or OpenContaining carries the read entrypoints as methods; the pure
// range parser and the repository-topology entrypoints stay free functions
// because they precede or do without an opened repository. No backend type,
// sentinel, or error value crosses the seam surface in either direction.
//
// This file holds the go-git backend: the tolerant repository open and the
// object, tree, and ignore reads the handle's methods are implemented with.
package git

import (
	"bytes"
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
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
)

type headResolution struct {
	ref    *plumbing.Reference
	unborn bool
}

// resolveHead follows HEAD one symbolic reference at a time so only absence of
// HEAD's immediate target is classified as unborn. go-git's Repository.Head
// reports ErrReferenceNotFound for both that case and a missing ref anywhere
// deeper in the chain, losing the distinction.
func resolveHead(repo *gogit.Repository) (headResolution, error) {
	ref, err := repo.Reference(plumbing.HEAD, false)
	if err != nil { // coverage-ignore: a handle is only returned after go-git has successfully read HEAD from the same storer
		return headResolution{}, fmt.Errorf("resolve HEAD: %w", err)
	}
	seen := map[plumbing.ReferenceName]bool{plumbing.HEAD: true}
	for ref.Type() == plumbing.SymbolicReference {
		target := ref.Target()
		if seen[target] {
			return headResolution{}, fmt.Errorf("resolve HEAD: cyclic symbolic reference at %s", target)
		}
		seen[target] = true
		next, err := repo.Reference(target, false)
		if err != nil {
			if ref.Name() == plumbing.HEAD && errors.Is(err, plumbing.ErrReferenceNotFound) {
				return headResolution{unborn: true}, nil
			}
			return headResolution{}, fmt.Errorf("resolve HEAD via %s: %w", target, err)
		}
		ref = next
	}
	return headResolution{ref: ref}, nil
}

func parseWorktreeStatus(out []byte) (tracked, untracked int, err error) {
	for len(out) > 0 {
		end := bytes.IndexByte(out, 0)
		if end < 0 {
			return 0, 0, errors.New("parse native Git worktree status: unterminated record")
		}
		record := out[:end]
		out = out[end+1:]
		if len(record) == 0 {
			continue
		}
		switch record[0] {
		case '?':
			untracked++
		case '1', 'u':
			tracked++
		case '2':
			tracked++
			end = bytes.IndexByte(out, 0)
			if end < 0 {
				return 0, 0, errors.New("parse native Git worktree status: rename missing original path")
			}
			out = out[end+1:]
		default:
			return 0, 0, fmt.Errorf("parse native Git worktree status: unknown record type %q", record[0])
		}
	}
	return tracked, untracked, nil
}

// globalExcludePatterns returns the ignore patterns git applies from outside
// the repository: core.excludesfile from the system /etc/gitconfig and from
// the user's ~/.gitconfig. go-git's worktree status consults only the repo's
// own .gitignore chain and .git/info/exclude, so a status-based path universe
// that consumes untracked entries must inject these itself to mirror
// `git status`. Global patterns follow system patterns so the user's rules win
// where they conflict, matching git's precedence; the ordering is exercised
// only against the real root filesystem because LoadSystemPatterns hardcodes
// /etc/gitconfig. One narrow divergence from git remains: go-git composes
// Excludes after the repo's .gitignore chain, so a repo-level negation cannot
// re-include a globally-ignored file. Absent or unreadable sources contribute
// no patterns: the callers read repository state, and a missing optional ignore
// source must not fail them.
func globalExcludePatterns() []gitignore.Pattern {
	root := osfs.New("/")
	system, _ := gitignore.LoadSystemPatterns(root)
	global, _ := gitignore.LoadGlobalPatterns(root)
	return append(system, global...)
}

// OpenContainingRepo opens the Git repository containing projectRoot and
// returns the repository-relative slash-separated prefix of projectRoot.
//
// It is the last go-git-typed export, retained only for internal/audit's
// commit-range walk until that walk moves behind the seam; every other consumer
// reads through a handle from Open or OpenContaining.
func OpenContainingRepo(projectRoot string) (*gogit.Repository, string, error) {
	repo, prefix, err := OpenContaining(projectRoot)
	if err != nil {
		return nil, "", err
	}
	return repo.repo, prefix, nil
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

// BlobMode is the closed set of Git blob modes preserved by snapshots.
type BlobMode uint8

const (
	BlobRegular BlobMode = iota
	BlobExecutable
	BlobSymlink
)

// IndexBlob is one file's exact bytes and mode from a stage-0 index or a
// resolved commit tree. Symlink bytes are the inert link target.
type IndexBlob struct {
	Path  string
	Bytes []byte
	Mode  BlobMode
}

// blobModeOf maps the three git filemodes the seam preserves onto BlobMode.
// Callers filter to those three before calling.
func blobModeOf(mode filemode.FileMode) BlobMode {
	switch mode {
	case filemode.Executable:
		return BlobExecutable
	case filemode.Symlink:
		return BlobSymlink
	default:
		return BlobRegular
	}
}

// readBlob reads one blob object's exact bytes from the object store.
func readBlob(repo *gogit.Repository, hash plumbing.Hash) ([]byte, error) {
	blob, err := repo.BlobObject(hash)
	if err != nil {
		return nil, err
	}
	r, err := blob.Reader()
	if err != nil { // coverage-ignore: a blob object successfully loaded from go-git's object store always supplies a reader
		return nil, err
	}
	data, readErr := io.ReadAll(r)
	closeErr := r.Close()
	if readErr != nil { // coverage-ignore: reads from an in-memory git blob reader do not fail
		return nil, readErr
	}
	if closeErr != nil { // coverage-ignore: go-git's read-only blob reader has no close failure
		return nil, closeErr
	}
	return data, nil
}

// blobsOfTree collects the sorted regular and executable blobs of a resolved
// tree. Symlinks and gitlinks are skipped; the executable mode is preserved.
func blobsOfTree(tree *object.Tree, prefix string) ([]IndexBlob, error) {
	var out []IndexBlob
	err := tree.Files().ForEach(func(f *object.File) error {
		path, ok := rerootPath(f.Name, prefix)
		if !ok || f.Mode != filemode.Regular && f.Mode != filemode.Executable && f.Mode != filemode.Symlink {
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
		out = append(out, IndexBlob{Path: path, Bytes: data, Mode: blobModeOf(f.Mode)})
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

// openTolerant opens the repo at repoRoot like git.PlainOpen, but hides its
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
func openTolerant(repoRoot string) (*gogit.Repository, error) {
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
