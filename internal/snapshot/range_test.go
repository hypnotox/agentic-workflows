package snapshot_test

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestRangePairRoot uses an empty parent for a root commit: before is empty and
// after is the root commit's whole tree.
func TestRangePairRoot(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	root := gitfixture.Commit(t, repo, dir, "root", map[string]string{"a.txt": "a"})

	before, after, err := snapshot.RangePair(dir, root.String())
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
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "root", map[string]string{"a.txt": "one"})
	child := gitfixture.Commit(t, repo, dir, "child", map[string]string{"a.txt": "two", "b.txt": "new"})

	before, after, err := snapshot.RangePair(dir, child.String())
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
	repo, dir := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, dir, "base", map[string]string{"m.txt": "base"})
	first := gitfixture.Commit(t, repo, dir, "first", map[string]string{"m.txt": "one"})

	baseCommit, err := repo.CommitObject(base)
	if err != nil {
		t.Fatal(err)
	}
	// A merge whose first parent is `first`, second parent is `base`, and whose
	// tree equals base's tree. before must reflect `first` (m.txt=one).
	merge := &object.Commit{
		Author:       *gitfixture.Sig,
		Committer:    *gitfixture.Sig,
		Message:      "merge",
		TreeHash:     baseCommit.TreeHash,
		ParentHashes: []plumbing.Hash{first, base},
	}
	enc := repo.Storer.NewEncodedObject()
	if err := merge.Encode(enc); err != nil {
		t.Fatal(err)
	}
	mh, err := repo.Storer.SetEncodedObject(enc)
	if err != nil {
		t.Fatal(err)
	}

	before, after, err := snapshot.RangePair(dir, mh.String())
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
	if _, _, err := snapshot.RangePair(t.TempDir(), "HEAD"); err == nil {
		t.Fatal("expected an error outside a repository")
	}
}

// TestRangePairBadRevision wraps the revision-resolution failure.
func TestRangePairBadRevision(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "base", map[string]string{"a.txt": "a"})
	if _, _, err := snapshot.RangePair(dir, "does-not-exist"); err == nil {
		t.Fatal("expected an error for an unresolvable revision")
	}
}
