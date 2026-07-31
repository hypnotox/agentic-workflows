package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func findWalkChange(changes []FileChange, path string) (FileChange, bool) {
	for _, change := range changes {
		if change.Path == path {
			return change, true
		}
	}
	return FileChange{}, false
}

func walkRepo(t *testing.T, root string) *Repo {
	t.Helper()
	repo, _, err := OpenContaining(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestRangeCommitsLinearRangeCarriesChangesAndText(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{
		"a.md":       "old\n",
		"delete.txt": "gone\n",
		"rename.txt": "rename\n",
	})
	gitfixture.Commit(t, repo, "feat(awf): one\r\n\r\nbody text\r\nmore\r\n", map[string]string{
		"a.md":        "new\n",
		"create.txt":  "made\n",
		"renamed.txt": "rename\n",
	}, "delete.txt", "rename.txt")
	gitfixture.Commit(t, repo, "fix(awf): two", map[string]string{"c.md": "new\n"})

	commits, err := walkRepo(t, dir).RangeCommits(testContext(t), base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 || commits[0].Subject != "fix(awf): two" || commits[1].Subject != "feat(awf): one" {
		t.Fatalf("commits = %#v", commits)
	}
	one := commits[1]
	if one.Body != "body text\nmore" {
		t.Fatalf("message body = %q", one.Body)
	}
	if c, ok := findWalkChange(one.Changes, "a.md"); !ok || c.Action != Modified || c.OldText != "old\n" || c.NewText != "new\n" || c.Added != 1 || c.Deleted != 1 {
		t.Fatalf("markdown modification = %#v", c)
	}
	if c, ok := findWalkChange(one.Changes, "create.txt"); !ok || c.Action != Added || c.Added != 1 || c.OldText != "" || c.NewText != "" {
		t.Fatalf("create = %#v", c)
	}
	if c, ok := findWalkChange(one.Changes, "delete.txt"); !ok || c.Action != Deleted || c.Deleted != 1 {
		t.Fatalf("delete = %#v", c)
	}
	// Tree-diff intentionally reports a rename as its delete and add sides;
	// both sides retain their line statistics for policy consumers.
	if c, ok := findWalkChange(one.Changes, "rename.txt"); !ok || c.Action != Deleted || c.Deleted != 1 {
		t.Fatalf("rename delete = %#v", c)
	}
	if c, ok := findWalkChange(one.Changes, "renamed.txt"); !ok || c.Action != Added || c.Added != 1 {
		t.Fatalf("rename add = %#v", c)
	}
	if c, ok := findWalkChange(commits[0].Changes, "c.md"); !ok || c.Action != Added || c.NewText != "new\n" {
		t.Fatalf("second commit = %#v", c)
	}
	if text, found, err := walkRepo(t, dir).FileText(testContext(t), base, "a.md"); err != nil || !found || text != "old\n" {
		t.Fatalf("base text = %q, %t, %v", text, found, err)
	}
	if text, found, err := walkRepo(t, dir).FileText(testContext(t), "HEAD", "a.md"); err != nil || !found || text != "new\n" {
		t.Fatalf("head text = %q, %t, %v", text, found, err)
	}
}

func TestRangeCommitsMergedRangeKeepsMergeAndNoChanges(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "chore: base", map[string]string{"README.md": "base\n"})
	main := gitfixture.Commit(t, repo, "feat: main-side work", map[string]string{"mainside.txt": "main\n"})
	gitfixture.CheckoutNewBranch(t, repo, "feature", base)
	feature := gitfixture.Commit(t, repo, "feat(awf): branch work", map[string]string{"branch.txt": "branch\n"})
	gitfixture.StageFile(t, repo, "mainside.txt", "main\n", 0o644)
	gitfixture.Merge(t, repo, "Merge branch 'master' into feature", feature, main)

	commits, err := walkRepo(t, dir).RangeCommits(testContext(t), "master", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 || !commits[0].IsMerge || commits[0].Subject != "Merge branch 'master' into feature" || commits[1].Subject != "feat(awf): branch work" {
		t.Fatalf("merged range = %#v", commits)
	}
	if len(commits[0].Changes) != 0 {
		t.Fatalf("merge changes = %#v", commits[0].Changes)
	}
}

func TestRangeCommitsNestedScopeFiltersAndReroots(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	if err := os.MkdirAll(filepath.Join(dir, "nested", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := gitfixture.Commit(t, repo, "base", map[string]string{"nested/docs/old.md": "old\n", "outside.txt": "old\n"})
	gitfixture.Commit(t, repo, "outside", map[string]string{"outside.txt": "new\n"})
	gitfixture.Commit(t, repo, "inside", map[string]string{"nested/docs/old.md": "new\n", "nested/new.txt": "new\n"})

	commits, err := walkRepo(t, filepath.Join(dir, "nested")).RangeCommits(testContext(t), base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].Subject != "inside" {
		t.Fatalf("nested commits = %#v", commits)
	}
	for _, change := range commits[0].Changes {
		if strings.HasPrefix(change.Path, "nested/") || strings.HasPrefix(change.Path, "outside") {
			t.Fatalf("unrerooted change = %#v", change)
		}
	}
	if _, ok := findWalkChange(commits[0].Changes, "docs/old.md"); !ok {
		t.Fatalf("missing rerooted modification: %#v", commits[0].Changes)
	}
	if _, ok := findWalkChange(commits[0].Changes, "new.txt"); !ok {
		t.Fatalf("missing rerooted addition: %#v", commits[0].Changes)
	}
	if text, found, err := walkRepo(t, filepath.Join(dir, "nested")).FileText(testContext(t), "HEAD", "docs/old.md"); err != nil || !found || text != "new\n" {
		t.Fatalf("nested file text = %q, %t, %v", text, found, err)
	}
}

func TestRangeCommitsBoundaryErrorsAndRoot(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	root := gitfixture.Commit(t, repo, "root", map[string]string{"a.md": "root\n"})
	handle := walkRepo(t, dir)
	rootCommit := walkCommitObject(t, dir, root)
	commit, err := toCommit(rootCommit, "")
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := findWalkChange(commit.Changes, "a.md"); !ok || c.Action != Added || c.NewText != "root\n" {
		t.Fatalf("root change = %#v", c)
	}
	if _, found, err := handle.FileText(testContext(t), "HEAD", "missing.md"); err != nil || found {
		t.Fatalf("missing text = %t, %v", found, err)
	}
	if commits, err := handle.RangeCommits(testContext(t), root, "HEAD"); err != nil || commits != nil {
		t.Fatalf("empty range = %#v, %v", commits, err)
	}
	if _, err := handle.RangeCommits(testContext(t), "missing", "HEAD"); err == nil {
		t.Fatal("missing base revision accepted")
	} else {
		var command *CommandError
		if errors.As(err, &command) {
			t.Fatalf("revision error leaked CommandError: %v", err)
		}
	}
	if _, err := handle.RangeCommits(testContext(t), "HEAD", "missing"); err == nil {
		t.Fatal("missing head revision accepted")
	}
	if _, _, err := handle.FileText(testContext(t), "missing", "a.md"); err == nil {
		t.Fatal("missing FileText revision accepted")
	}
	tree, err := rootCommit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	file, err := tree.File("a.md")
	if err != nil {
		t.Fatal(err)
	}
	hash := file.Hash.String()
	objectPath := filepath.Join(dir, ".git", "objects", hash[:2], hash[2:])
	if err := os.Remove(objectPath); err != nil {
		t.Fatal(err)
	}
	if text := fileText(tree, "a.md"); text != "" {
		t.Fatalf("unreadable text = %q", text)
	}
	if err := os.WriteFile(objectPath, []byte("not a git object"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := handle.FileText(testContext(t), "HEAD", "a.md"); err == nil {
		t.Fatal("corrupt file object accepted")
	}
}

func TestRangeCommitsUnrelatedHistoryAndWorktreeConfig(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	gitfixture.Commit(t, repo, "head", map[string]string{"a.txt": "b"})
	orphan := storeOrphan(t, dir)
	if _, err := walkRepo(t, dir).RangeCommits(testContext(t), orphan, "HEAD"); err == nil || !strings.Contains(err.Error(), "unrelated histories") {
		t.Fatalf("unrelated histories = %v", err)
	}
	enableWalkWorktreeConfig(t, dir)
	if commits, err := walkRepo(t, dir).RangeCommits(testContext(t), base, "HEAD"); err != nil || len(commits) != 1 {
		t.Fatalf("worktreeConfig range = %#v, %v", commits, err)
	}
}

func TestWalkMethodsRespectCanceledContextAndNativeErrors(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.go": "package a\n"})
	gitfixture.Commit(t, repo, "head", map[string]string{"a.go": "package a\n\nvar x int\n"})
	handle := walkRepo(t, dir)
	ctx, cancel := context.WithCancel(testContext(t))
	cancel()
	if _, err := handle.RangeCommits(ctx, "HEAD", "HEAD"); !errors.Is(err, context.Canceled) {
		t.Fatalf("RangeCommits cancellation = %v", err)
	}
	if _, _, err := handle.FileText(ctx, "HEAD", "a.go"); !errors.Is(err, context.Canceled) {
		t.Fatalf("FileText cancellation = %v", err)
	}
	t.Run("base walk", func(t *testing.T) {
		midWalk := &cancelAfterContext{Context: testContext(t), remaining: 2}
		if _, err := handle.RangeCommits(midWalk, "HEAD", "HEAD"); !errors.Is(err, context.Canceled) {
			t.Fatalf("base-walk cancellation = %v", err)
		}
	})
	t.Run("head walk", func(t *testing.T) {
		midWalk := &cancelAfterContext{Context: testContext(t), remaining: 3}
		if _, err := handle.RangeCommits(midWalk, "HEAD~1", "HEAD"); !errors.Is(err, context.Canceled) {
			t.Fatalf("head-walk cancellation = %v", err)
		}
	})
	t.Setenv("PATH", t.TempDir())
	for name, call := range map[string]func() error{
		"merge-base": func() error { _, err := handle.MergeBase(testContext(t), "HEAD", "HEAD"); return err },
		"paths":      func() error { _, err := handle.RangeChangedPaths(testContext(t), "HEAD", "HEAD"); return err },
		"diff":       func() error { _, err := handle.RangeDiffText(testContext(t), "HEAD", "HEAD"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("missing git binary accepted")
			}
		})
	}
}

func TestRangeNativeReadOperations(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"a.go": "package a\n"})
	head := gitfixture.Commit(t, repo, "head", map[string]string{"a.go": "package a\n\nvar x = 1\n"})
	handle := walkRepo(t, dir)
	ctx := testContext(t)
	if got, err := handle.MergeBase(ctx, base, head); err != nil || got != base {
		t.Fatalf("merge base = %q, %v", got, err)
	}
	if got, err := handle.RangeChangedPaths(ctx, base, head); err != nil || len(got) != 1 || got[0] != "a.go" {
		t.Fatalf("paths = %#v, %v", got, err)
	}
	if got, err := handle.RangeDiffText(ctx, base, head); err != nil || !strings.Contains(got, "+++ b/a.go") {
		t.Fatalf("diff = %q, %v", got, err)
	}
}

func TestSplitWalkMessage(t *testing.T) {
	if subject, body := splitMessage("subject  \n\nbody\n"); subject != "subject" || body != "body" {
		t.Fatalf("split = %q / %q", subject, body)
	}
	if subject, body := splitMessage("subject"); subject != "subject" || body != "" {
		t.Fatalf("single line = %q / %q", subject, body)
	}
}

type cancelAfterContext struct {
	context.Context
	remaining int
}

func (c *cancelAfterContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return c.Context.Err()
}

// walkCommitObject reads a commit object directly, for the package-internal
// helpers whose subject is a go-git commit rather than a seam entrypoint.
func walkCommitObject(t *testing.T, dir, rev string) *object.Commit {
	t.Helper()
	commit, err := openWalkRepo(t, dir).CommitObject(plumbing.NewHash(rev))
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

func openWalkRepo(t *testing.T, dir string) *gogit.Repository {
	t.Helper()
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func storeOrphan(t *testing.T, dir string) string {
	t.Helper()
	repo := openWalkRepo(t, dir)
	tree := &object.Tree{}
	encodedTree := repo.Storer.NewEncodedObject()
	if err := tree.Encode(encodedTree); err != nil {
		t.Fatal(err)
	}
	treeHash, err := repo.Storer.SetEncodedObject(encodedTree)
	if err != nil {
		t.Fatal(err)
	}
	commit := &object.Commit{Author: *gitfixture.Sig, Committer: *gitfixture.Sig, Message: "orphan\n", TreeHash: treeHash}
	encodedCommit := repo.Storer.NewEncodedObject()
	if err := commit.Encode(encodedCommit); err != nil {
		t.Fatal(err)
	}
	hash, err := repo.Storer.SetEncodedObject(encodedCommit)
	if err != nil {
		t.Fatal(err)
	}
	return hash.String()
}

func enableWalkWorktreeConfig(t *testing.T, dir string) {
	t.Helper()
	repo := openWalkRepo(t, dir)
	cfg, err := repo.Storer.Config()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Raw.Section("extensions").SetOption("worktreeConfig", "true")
	if err := repo.Storer.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
}
