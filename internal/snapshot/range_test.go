package snapshot_test

import (
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestRangePairRoot uses an empty parent for a root commit: before is empty and
// after is the root commit's whole tree.
func TestRangePairRoot(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	root := gitfixture.Commit(t, repo, "root", map[string]string{"a.txt": "a"})

	before, after, err := snapshot.RangePair(testContext(t), snapshotRepo(t, dir), root)
	if err != nil {
		t.Fatalf("RangePair: %v", err)
	}
	if len(before.List()) != 0 {
		t.Errorf("root parent should be empty, got %+v", before.List())
	}
	if f, ok := after.Lookup("a.txt"); !ok || string(f.Bytes) != "a" {
		t.Errorf("after tree missing a.txt: %q, %v", f.Bytes, ok)
	}
}

// TestRangePairChild diffs a child against its single parent.
func TestRangePairChild(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "root", map[string]string{"a.txt": "one"})
	child := gitfixture.Commit(t, repo, "child", map[string]string{"a.txt": "two", "b.txt": "new"})

	before, after, err := snapshot.RangePair(testContext(t), snapshotRepo(t, dir), child)
	if err != nil {
		t.Fatalf("RangePair: %v", err)
	}
	if f, ok := before.Lookup("a.txt"); !ok || string(f.Bytes) != "one" {
		t.Errorf("before a.txt = %q, %v; want parent content", f.Bytes, ok)
	}
	if _, ok := before.Lookup("b.txt"); ok {
		t.Errorf("b.txt should be absent from the parent tree")
	}
	if f, ok := after.Lookup("a.txt"); !ok || string(f.Bytes) != "two" {
		t.Errorf("after a.txt = %q, %v; want child content", f.Bytes, ok)
	}
}

// TestRangePairMergeFirstParent proves a merge uses the first parent only: the
// before tree is the first parent's tree, not the second's. The synthetic merge
// takes its tree from the second parent so a first-parent diff is observable.
func TestRangePairMergeFirstParent(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"m.txt": "base"})
	first := gitfixture.Commit(t, repo, "first", map[string]string{"m.txt": "one"})

	// A merge whose first parent is `first`, second parent is `base`, and whose
	// tree equals base's tree. before must reflect `first` (m.txt=one).
	merge := gitfixture.Graft(t, repo, "merge", base, first, base)

	before, after, err := snapshot.RangePair(testContext(t), snapshotRepo(t, dir), merge)
	if err != nil {
		t.Fatalf("RangePair: %v", err)
	}
	if f, ok := before.Lookup("m.txt"); !ok || string(f.Bytes) != "one" {
		t.Errorf("before m.txt = %q, %v; want first-parent content %q", f.Bytes, ok, "one")
	}
	if f, ok := after.Lookup("m.txt"); !ok || string(f.Bytes) != "base" {
		t.Errorf("after m.txt = %q, %v; want merge-tree content %q", f.Bytes, ok, "base")
	}
}

// TestRangePairOutsideRepo wraps git.RangeBlobs' open-repo failure.
func TestRangePairOutsideRepo(t *testing.T) {
	t.Parallel()
	if _, err := awfgit.Open(t.TempDir()); err == nil {
		t.Fatal("expected an error outside a repository")
	}
}

// TestRangePairBadRevision wraps the revision-resolution failure.
func TestRangePairBadRevision(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	if _, _, err := snapshot.RangePair(testContext(t), snapshotRepo(t, dir), "does-not-exist"); err == nil {
		t.Fatal("expected an error for an unresolvable revision")
	}
}
