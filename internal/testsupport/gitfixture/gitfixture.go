// Package gitfixture builds Git repository state for tests through an opaque
// Fixture value. Its in-process lane creates ordinary state, while its native
// lane creates registered worktrees, orphan branches, in-progress merges, and
// non-default object formats. Both lanes expose backend-neutral repository and
// commit values; Sig supports tests that exercise go-git behavior directly.
package gitfixture

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	indexformat "github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// authorName and authorEmail are the fixed commit identity both lanes use, so a
// go-git-built and a natively-built fixture carry the same authorship.
const (
	authorName           = "T"
	authorEmail          = "t@example.com"
	maintenanceAutoKey   = "maintenance.auto"
	maintenanceAutoValue = "false"
)

// Sig is the fixed commit signature the go-git lane writes. It stays exported
// for the internal/git suites, which are allowed to drive go-git directly and
// need the same identity on the commits they build by hand.
var Sig = &object.Signature{Name: authorName, Email: authorEmail, When: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

// Fixture identifies a fixture repository by its root. Both lanes operate on
// it, and it is the only repository value the package hands out.
type Fixture struct {
	root string
}

// Root reports the fixture repository's working-tree root.
func (f Fixture) Root() string { return f.root }

// At names an existing repository checkout, including a linked worktree the
// code under test registered, so fixture operations can be aimed at it.
func At(root string) Fixture { return Fixture{root: root} }

// InitRepo creates a fresh git repository in a new t.TempDir().
func InitRepo(t testing.TB) Fixture {
	t.Helper()
	return InitRepoAt(t, t.TempDir())
}

// InitRepoAt creates a git repository in root, which may already hold files a
// test wrote before the repository existed.
func InitRepoAt(t testing.TB, root string) Fixture {
	t.Helper()
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	cfg, err := repo.Storer.Config()
	if err != nil {
		t.Fatalf("read fixture config: %v", err)
	}
	cfg.Raw.Section("maintenance").SetOption("auto", maintenanceAutoValue)
	if err := repo.Storer.SetConfig(cfg); err != nil {
		t.Fatalf("disable fixture auto-maintenance: %v", err)
	}
	return Fixture{root: root}
}

// Stage writes the given paths into the fixture's worktree (creating any parent
// directories) and adds them to the index without committing, so a test can
// exercise a staged-but-uncommitted index universe distinct from the working
// tree.
func Stage(t testing.TB, f Fixture, write map[string]string) {
	t.Helper()
	stageInto(t, worktree(t, f), f.root, write)
}

// StageFile writes one path with an explicit file mode and stages it, so a
// fixture can pin an executable bit the index must preserve.
func StageFile(t testing.TB, f Fixture, name, content string, mode os.FileMode) {
	t.Helper()
	writeUnder(t, f.root, name, content, mode)
	Add(t, f, name)
}

// Add stages paths that already exist in the worktree, including a symlink the
// test created itself.
func Add(t testing.TB, f Fixture, paths ...string) {
	t.Helper()
	addAll(t, worktree(t, f), paths)
}

// AddAll stages every change in the worktree, matching `git add -A` at the
// repository root.
func AddAll(t testing.TB, f Fixture) {
	t.Helper()
	if err := worktree(t, f).AddWithOptions(&git.AddOptions{All: true}); err != nil {
		t.Fatalf("add all: %v", err)
	}
}

// StageRemoval stages the deletion of tracked paths without committing, so a
// test can distinguish a staged deletion from a working-tree one.
func StageRemoval(t testing.TB, f Fixture, names ...string) {
	t.Helper()
	removeFrom(t, worktree(t, f), names)
}

// StageGitlink appends a gitlink (submodule) index entry, which carries no
// regular file content and so exercises the readers' skip path.
func StageGitlink(t testing.TB, f Fixture, name string) {
	t.Helper()
	appendIndexEntry(t, f, &indexformat.Entry{
		Name: name,
		Mode: filemode.Submodule,
		Hash: plumbing.NewHash("0123456789012345678901234567890123456789"),
	})
}

// StageUnmerged appends a conflicted (stage-2) index entry, so a test can drive
// the unmerged-index refusal.
func StageUnmerged(t testing.TB, f Fixture, name string) {
	t.Helper()
	appendIndexEntry(t, f, &indexformat.Entry{Name: name, Mode: filemode.Regular, Stage: indexformat.OurMode})
}

// Commit writes/removes the given paths in the fixture's worktree, stages them,
// and commits with Sig, returning the commit's hex hash.
func Commit(t testing.TB, f Fixture, msg string, write map[string]string, remove ...string) string {
	t.Helper()
	wt := worktree(t, f)
	stageInto(t, wt, f.root, write)
	removeFrom(t, wt, remove)
	h, err := wt.Commit(msg, &git.CommitOptions{Author: Sig, Committer: Sig})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return h.String()
}

// Merge creates a commit whose tree is the current index, with the given
// parents in order (the first is the first parent), so a fixture can exercise
// first-parent range semantics. It allows an empty commit, so a merge can
// integrate a branch whose tree already matches HEAD, and with no parents named
// it simply commits the index on top of HEAD.
func Merge(t testing.TB, f Fixture, msg string, parents ...string) string {
	t.Helper()
	h, err := worktree(t, f).Commit(msg, &git.CommitOptions{
		Parents:           hashes(parents),
		AllowEmptyCommits: true,
		Author:            Sig,
		Committer:         Sig,
	})
	if err != nil {
		t.Fatalf("merge commit: %v", err)
	}
	return h.String()
}

// Graft writes a commit object directly, taking its tree from treeFrom and its
// parents as given, without moving any reference. It builds the shapes an
// ordinary commit cannot reach, such as a merge whose tree deliberately differs
// from its first parent's.
func Graft(t testing.TB, f Fixture, msg, treeFrom string, parents ...string) string {
	t.Helper()
	commit := &object.Commit{
		Author:       *Sig,
		Committer:    *Sig,
		Message:      msg,
		TreeHash:     plumbing.NewHash(TreeHash(t, f, treeFrom)),
		ParentHashes: hashes(parents),
	}
	repo := open(t, f)
	encoded := repo.Storer.NewEncodedObject()
	if err := commit.Encode(encoded); err != nil {
		t.Fatalf("encode graft: %v", err)
	}
	h, err := repo.Storer.SetEncodedObject(encoded)
	if err != nil {
		t.Fatalf("store graft: %v", err)
	}
	return h.String()
}

// TreeHash reports the hex tree hash of the commit named by rev.
func TreeHash(t testing.TB, f Fixture, rev string) string {
	t.Helper()
	commit, err := open(t, f).CommitObject(plumbing.NewHash(rev))
	if err != nil {
		t.Fatalf("commit object %s: %v", rev, err)
	}
	return commit.TreeHash.String()
}

// CheckoutNewBranch creates a branch at the given commit and checks it out, so
// a fixture can put history on a branch other than the initial one.
func CheckoutNewBranch(t testing.TB, f Fixture, name, at string) {
	t.Helper()
	options := &git.CheckoutOptions{
		Hash:   plumbing.NewHash(at),
		Branch: plumbing.NewBranchReferenceName(name),
		Create: true,
	}
	if err := worktree(t, f).Checkout(options); err != nil {
		t.Fatalf("checkout -b %s: %v", name, err)
	}
}

// appendIndexEntry adds a raw index entry, the escape hatch for index states
// (gitlink, conflicted) that no worktree operation produces.
func appendIndexEntry(t testing.TB, f Fixture, entry *indexformat.Entry) {
	t.Helper()
	repo := open(t, f)
	idx, err := repo.Storer.Index()
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	idx.Entries = append(idx.Entries, entry)
	if err := repo.Storer.SetIndex(idx); err != nil {
		t.Fatalf("write index: %v", err)
	}
}

// stageInto writes and stages a whole path set through one open worktree.
func stageInto(t testing.TB, wt *git.Worktree, root string, write map[string]string) {
	t.Helper()
	for name, content := range write {
		writeUnder(t, root, name, content, 0o644)
		addAll(t, wt, []string{name})
	}
}

// addAll stages existing paths through an already-open worktree.
func addAll(t testing.TB, wt *git.Worktree, paths []string) {
	t.Helper()
	for _, name := range paths {
		if _, err := wt.Add(name); err != nil {
			t.Fatalf("add %s: %v", name, err)
		}
	}
}

// removeFrom stages deletions through an already-open worktree.
func removeFrom(t testing.TB, wt *git.Worktree, names []string) {
	t.Helper()
	for _, name := range names {
		if _, err := wt.Remove(name); err != nil {
			t.Fatalf("remove %s: %v", name, err)
		}
	}
}

// writeUnder writes content at name relative to root, creating parents.
func writeUnder(t testing.TB, root, name, content string, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// open returns the go-git repository behind the fixture.
func open(t testing.TB, f Fixture) *git.Repository {
	t.Helper()
	repo, err := git.PlainOpen(f.root)
	if err != nil {
		t.Fatalf("open %s: %v", f.root, err)
	}
	return repo
}

// worktree returns the fixture repository's worktree.
func worktree(t testing.TB, f Fixture) *git.Worktree {
	t.Helper()
	wt, err := open(t, f).Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	return wt
}

// hashes converts hex commit ids to plumbing hashes, keeping the exported
// surface free of go-git types.
func hashes(revs []string) []plumbing.Hash {
	if len(revs) == 0 {
		return nil
	}
	converted := make([]plumbing.Hash, 0, len(revs))
	for _, rev := range revs {
		converted = append(converted, plumbing.NewHash(rev))
	}
	return converted
}
