package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
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
	if len(one.Revision) != 40 || one.Message != "feat(awf): one\r\n\r\nbody text\r\nmore\r\n" || len(one.Parents) != 1 || len(one.Parents[0]) != 40 {
		t.Fatalf("committed evidence = %#v", one)
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
	if len(commits[0].Parents) != 2 || len(commits[0].Revision) != 40 || commits[0].Message != "Merge branch 'master' into feature" {
		t.Fatalf("merge evidence = %#v", commits[0])
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

func TestRangeCommitsNestedScopeKeepsRelevantMerges(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"nested/base.txt": "base\n", "outside.txt": "base\n"})
	main := gitfixture.Commit(t, repo, "main", map[string]string{"outside.txt": "main\n"})
	gitfixture.CheckoutNewBranch(t, repo, "feature", base)
	feature := gitfixture.Commit(t, repo, "feature", map[string]string{"nested/feature.txt": "feature\n"})
	gitfixture.StageFile(t, repo, "outside.txt", "main\n", 0o644)
	gitfixture.Merge(t, repo, "Merge feature", main, feature)

	commits, err := walkRepo(t, filepath.Join(dir, "nested")).RangeCommits(testContext(t), "master", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 || !commits[0].IsMerge || commits[0].Subject != "Merge feature" || commits[1].Subject != "feature" {
		t.Fatalf("nested merge range = %#v", commits)
	}
	if len(commits[0].Changes) != 0 {
		t.Fatalf("nested merge changes = %#v", commits[0].Changes)
	}

	outsideRepo := gitfixture.InitRepo(t)
	outsideDir := outsideRepo.Root()
	outsideBase := gitfixture.Commit(t, outsideRepo, "base", map[string]string{"nested/base.txt": "base\n", "outside.txt": "base\n"})
	outsideMain := gitfixture.Commit(t, outsideRepo, "main", map[string]string{"outside.txt": "main\n"})
	gitfixture.CheckoutNewBranch(t, outsideRepo, "feature", outsideBase)
	outsideFeature := gitfixture.Commit(t, outsideRepo, "feature", map[string]string{"feature.txt": "feature\n"})
	gitfixture.StageFile(t, outsideRepo, "outside.txt", "main\n", 0o644)
	gitfixture.Merge(t, outsideRepo, "Merge outside", outsideMain, outsideFeature)
	commits, err = walkRepo(t, filepath.Join(outsideDir, "nested")).RangeCommits(testContext(t), "master", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 0 {
		t.Fatalf("outside-only nested range = %#v", commits)
	}
}

func TestRangeCommitsNestedMergeReportsTreeErrors(t *testing.T) {
	for _, name := range []string{"merge result tree", "first parent tree"} {
		t.Run(name, func(t *testing.T) {
			repo := gitfixture.InitRepo(t)
			dir := repo.Root()
			base := gitfixture.Commit(t, repo, "base", map[string]string{"nested/base.txt": "base\n"})
			main := gitfixture.Commit(t, repo, "main", map[string]string{"nested/main.txt": "main\n"})
			gitfixture.CheckoutNewBranch(t, repo, "feature", base)
			feature := gitfixture.Commit(t, repo, "feature", map[string]string{"nested/feature.txt": "feature\n"})
			gitfixture.StageFile(t, repo, "nested/main.txt", "main\n", 0o644)
			merge := gitfixture.Merge(t, repo, "Merge feature", main, feature)

			var hash string
			switch name {
			case "merge result tree":
				hash = gitfixture.TreeHash(t, repo, merge)
			case "first parent tree":
				hash = gitfixture.TreeHash(t, repo, main)
			}
			if err := os.Remove(filepath.Join(dir, ".git", "objects", hash[:2], hash[2:])); err != nil {
				t.Fatal(err)
			}
			if _, err := walkRepo(t, filepath.Join(dir, "nested")).RangeCommits(testContext(t), "master", "HEAD"); err == nil {
				t.Fatal("nested merge with missing tree object accepted")
			}
		})
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
	// The fixture is built to falsify each flag these entrypoints depend on,
	// because the obvious two-linear-commit shape falsifies none of them: with
	// base as head's direct parent, merge-base cannot be told from rev-parse; in
	// a clean checkout a diff against base alone cannot be told from a diff of
	// base..head; a range that changes no existing file carries no context lines
	// to reveal a missing -U0; and under the seam's isolated environment the
	// default prefix is already b/, so only REPOSITORY-local diff config can
	// show the -c pins doing work.
	root := gitfixture.Commit(t, repo, "root", map[string]string{
		"a.go":         "package a\n\nconst One = 1\nconst Two = 2\nconst Three = 3\n",
		"untouched.go": "package a\n",
	})
	trunk := lifecycleGit(t, dir, "symbolic-ref", "--short", "HEAD")
	gitfixture.CheckoutNewBranch(t, repo, "side", root)
	side := gitfixture.Commit(t, repo, "side", map[string]string{"side.go": "package a\n"})
	gitfixture.NativeCheckout(t, repo, trunk)
	base := gitfixture.Commit(t, repo, "base", map[string]string{"b.go": "package a\n"})
	head := gitfixture.Commit(t, repo, "head", map[string]string{
		"a.go": "package a\n\nconst One = 1\nconst Two = 22\nconst Three = 3\n",
	})
	// A repository-local diff configuration the isolated environment does not
	// strip. Without the entrypoint's -c pins this renders the diff without the
	// a/ and b/ prefixes its consumer parses.
	lifecycleGit(t, dir, "config", "diff.noprefix", "true")
	lifecycleGit(t, dir, "config", "diff.dstPrefix", "destination/")
	lifecycleGit(t, dir, "config", "diff.mnemonicprefix", "true")
	// An uncommitted change, so a diff that forgot its head argument would
	// compare base against the working tree and report this file too.
	if err := os.WriteFile(filepath.Join(dir, "untouched.go"), []byte("package a\n\nvar Dirty = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	handle := walkRepo(t, dir)
	ctx := testContext(t)

	// The fork point, not either tip: a rev-parse of the first argument would
	// answer base here, and the side branch's own tip would answer side.
	if got, err := handle.MergeBase(ctx, base, side); err != nil || got != root {
		t.Fatalf("merge base = %q, %v; want the fork point %q", got, err, root)
	}
	if got, err := handle.RangeChangedPaths(ctx, base, head); err != nil || len(got) != 1 || got[0] != "a.go" {
		t.Fatalf("paths = %#v, %v; want exactly the file the range changed, with the working tree ignored", got, err)
	}
	diff, err := handle.RangeDiffText(ctx, base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "+++ b/a.go") {
		t.Fatalf("diff = %q, want the b/ destination prefix the consumer parses", diff)
	}
	if strings.Contains(diff, "untouched.go") {
		t.Fatalf("diff reports a working-tree change outside the range: %q", diff)
	}
	// Zero context: in unified format a context line is one starting with a
	// single space, and -U0 emits none. Asserting on the neighbouring source
	// text instead would misfire, because git repeats the enclosing line as the
	// hunk heading regardless of the context setting.
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, " ") {
			t.Fatalf("diff carries the context line %q, so -U0 is not in force", line)
		}
	}
}

func TestFirstParentChangedPathsContracts(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{
		"changed.txt": "before\n", "deleted.txt": "gone\n", "renamed.txt": "rename\n", "nested/inside.txt": "old\n", "outside.txt": "old\n",
	})
	head := gitfixture.Commit(t, repo, "change", map[string]string{
		"changed.txt": "after\n", "added.txt": "added\n", "renamed-new.txt": "rename\n", "nested/inside.txt": "new\n", "outside.txt": "new\n",
	}, "deleted.txt", "renamed.txt")
	handle := walkRepo(t, dir)
	paths, err := handle.FirstParentChangedPaths(testContext(t), head)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(paths, ","), "added.txt,changed.txt,deleted.txt,nested/inside.txt,outside.txt,renamed-new.txt,renamed.txt"; got != want {
		t.Fatalf("first-parent paths = %q, want %q", got, want)
	}
	rootPaths, err := handle.FirstParentChangedPaths(testContext(t), base)
	if err != nil || strings.Join(rootPaths, ",") != "changed.txt,deleted.txt,nested/inside.txt,outside.txt,renamed.txt" {
		t.Fatalf("root first-parent paths = %v, %v", rootPaths, err)
	}
	nested, err := walkRepo(t, filepath.Join(dir, "nested")).FirstParentChangedPaths(testContext(t), head)
	if err != nil || strings.Join(nested, ",") != "inside.txt" {
		t.Fatalf("nested first-parent paths = %v, %v", nested, err)
	}

	gitfixture.CheckoutNewBranch(t, repo, "feature", base)
	feature := gitfixture.Commit(t, repo, "feature", map[string]string{"feature.txt": "feature\n"})
	lifecycleGit(t, dir, "checkout", "master")
	gitfixture.Stage(t, repo, map[string]string{"feature.txt": "feature\n"})
	merge := gitfixture.Merge(t, repo, "Merge feature", head, feature)
	mergePaths, err := handle.FirstParentChangedPaths(testContext(t), merge)
	if err != nil || strings.Join(mergePaths, ",") != "feature.txt" {
		t.Fatalf("merge first-parent paths = %v, %v", mergePaths, err)
	}
	commits, err := handle.RangeCommits(testContext(t), head, merge)
	if err != nil {
		t.Fatal(err)
	}
	var mergeCommit *Commit
	for i := range commits {
		if commits[i].Revision == merge {
			mergeCommit = &commits[i]
		}
	}
	if mergeCommit == nil || !mergeCommit.IsMerge || len(mergeCommit.Changes) != 0 {
		t.Fatalf("range commits exposed merge changes: %#v", commits)
	}

	cancelled, cancel := context.WithCancel(testContext(t))
	cancel()
	if _, err := handle.FirstParentChangedPaths(cancelled, head); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled first-parent paths = %v", err)
	}
	if _, err := handle.FirstParentChangedPaths(testContext(t), "missing-revision"); err == nil {
		t.Fatal("first-parent paths accepted a missing revision")
	}
}

func TestFirstParentChangedPathsPropagatesCancellationDuringDiff(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "base\n"})
	head := gitfixture.Commit(t, repo, "head", map[string]string{"a.txt": "head\n"})
	parent, cancel := context.WithCancel(testContext(t))
	ctx := &cancelAtDiffContext{Context: parent, remaining: 3, cancel: cancel}
	if _, err := walkRepo(t, repo.Root()).FirstParentChangedPaths(ctx, head); !errors.Is(err, context.Canceled) {
		t.Fatalf("first-parent diff cancellation = %v, want context.Canceled; base=%s", err, base)
	}
	if !ctx.diffObserved {
		t.Fatal("first-parent diff did not observe the caller context")
	}
}

func TestFirstParentChangedPathsReportsCorruptTreeEvidence(t *testing.T) {
	for _, target := range []string{"current tree", "parent tree", "parent subtree", "changed subtree", "unsafe path"} {
		t.Run(target, func(t *testing.T) {
			repo := gitfixture.InitRepo(t)
			dir := repo.Root()
			base := gitfixture.Commit(t, repo, "base", map[string]string{"nested/base.txt": "base\n"})
			head := gitfixture.Commit(t, repo, "head", map[string]string{"nested/head.txt": "head\n"})
			var removeHash string
			switch target {
			case "current tree":
				removeHash = gitfixture.TreeHash(t, repo, head)
			case "parent tree":
				removeHash = gitfixture.TreeHash(t, repo, base)
			case "parent subtree":
				commit := walkCommitObject(t, dir, base)
				tree, err := commit.Tree()
				if err != nil {
					t.Fatal(err)
				}
				subtree, err := tree.Tree("nested")
				if err != nil {
					t.Fatal(err)
				}
				removeHash = subtree.Hash.String()
			case "changed subtree":
				backend := openWalkRepo(t, dir)
				top := &object.Tree{Entries: []object.TreeEntry{{Name: "nested", Mode: filemode.Dir, Hash: plumbing.NewHash(strings.Repeat("1", 40))}}}
				encodedTree := backend.Storer.NewEncodedObject()
				if err := top.Encode(encodedTree); err != nil {
					t.Fatal(err)
				}
				treeHash, err := backend.Storer.SetEncodedObject(encodedTree)
				if err != nil {
					t.Fatal(err)
				}
				commit := &object.Commit{Author: *gitfixture.Sig, Committer: *gitfixture.Sig, Message: "corrupt subtree\n", TreeHash: treeHash, ParentHashes: []plumbing.Hash{plumbing.NewHash(base)}}
				encodedCommit := backend.Storer.NewEncodedObject()
				if err := commit.Encode(encodedCommit); err != nil {
					t.Fatal(err)
				}
				commitHash, err := backend.Storer.SetEncodedObject(encodedCommit)
				if err != nil {
					t.Fatal(err)
				}
				head = commitHash.String()
			case "unsafe path":
				backend := openWalkRepo(t, dir)
				baseCommit, err := backend.CommitObject(plumbing.NewHash(base))
				if err != nil {
					t.Fatal(err)
				}
				baseTree, err := baseCommit.Tree()
				if err != nil {
					t.Fatal(err)
				}
				file, err := baseTree.File("nested/base.txt")
				if err != nil {
					t.Fatal(err)
				}
				top := &object.Tree{Entries: []object.TreeEntry{{Name: "..", Mode: filemode.Regular, Hash: file.Hash}}}
				encodedTree := backend.Storer.NewEncodedObject()
				if err := top.Encode(encodedTree); err != nil {
					t.Fatal(err)
				}
				treeHash, err := backend.Storer.SetEncodedObject(encodedTree)
				if err != nil {
					t.Fatal(err)
				}
				commit := &object.Commit{Author: *gitfixture.Sig, Committer: *gitfixture.Sig, Message: "unsafe path\n", TreeHash: treeHash, ParentHashes: []plumbing.Hash{plumbing.NewHash(base)}}
				encodedCommit := backend.Storer.NewEncodedObject()
				if err := commit.Encode(encodedCommit); err != nil {
					t.Fatal(err)
				}
				commitHash, err := backend.Storer.SetEncodedObject(encodedCommit)
				if err != nil {
					t.Fatal(err)
				}
				head = commitHash.String()
			}
			if removeHash != "" {
				if err := os.Remove(filepath.Join(dir, ".git", "objects", removeHash[:2], removeHash[2:])); err != nil {
					t.Fatal(err)
				}
			}
			if paths, err := walkRepo(t, dir).FirstParentChangedPaths(testContext(t), head); err == nil {
				t.Fatalf("first-parent paths accepted missing tree evidence: %#v", paths)
			} else if errors.Is(err, plumbing.ErrObjectNotFound) {
				t.Fatalf("first-parent paths leaked backend error identity: %v", err)
			}
		})
	}
}

func TestFirstParentChangedPathsReportsShallowMissingParent(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	for _, n := range []string{"one", "two", "three"} {
		gitfixture.Commit(t, repo, n, map[string]string{"a.txt": n + "\n"})
	}
	shallow := filepath.Join(t.TempDir(), "shallow")
	if out, err := exec.CommandContext(t.Context(), "git", "clone", "--depth", "1", "file://"+dir, shallow).CombinedOutput(); err != nil {
		t.Skipf("shallow clone unavailable: %v: %s", err, out)
	}
	head := gitfixture.NativeRevParse(t, gitfixture.At(shallow), "HEAD")
	if _, err := walkRepo(t, shallow).FirstParentChangedPaths(testContext(t), head); err == nil {
		t.Fatal("first-parent paths accepted a missing shallow parent")
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

type cancelAtDiffContext struct {
	context.Context
	remaining    int
	cancel       context.CancelFunc
	diffObserved bool
}

func (c *cancelAtDiffContext) Done() <-chan struct{} {
	c.diffObserved = true
	return c.Context.Done()
}

func (c *cancelAtDiffContext) Err() error {
	c.remaining--
	if c.remaining == 0 {
		c.cancel()
		return nil
	}
	return c.Context.Err()
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

// TestRangeBlobsReportsCancellationFromBothTreeReads covers the two escapes in
// RangeBlobs that claimed reading in-memory blobs from a resolved tree cannot
// fail. That justification was written for the pre-seam blobsOfTree(tree,
// prefix); the seam's signature takes a context and checks it per file, so both
// branches are reachable on a healthy repository. The package already disagreed
// with itself about this: CommitBlobs routes the same call's error through
// opaqueError with no escape at all.
func TestRangeBlobsReportsCancellationFromBothTreeReads(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a\n"})
	head := gitfixture.Commit(t, repo, "second", map[string]string{"b.txt": "b\n"})
	handle := walkRepo(t, dir)

	// RangeBlobs checks the context once on entry, so cancelling on the second
	// observation lands inside the after-tree read rather than before it.
	afterCancel := &cancelAfterContext{Context: testContext(t), remaining: 2}
	if _, _, err := handle.RangeBlobs(afterCancel, head); !errors.Is(err, context.Canceled) {
		t.Fatalf("RangeBlobs with the after-tree read cancelled = %v, want context.Canceled", err)
	}
	// Letting the after-tree read finish and cancelling during the parent read
	// reaches the second escape, which only a commit with a parent can hit.
	beforeCancel := &cancelAfterContext{Context: testContext(t), remaining: 4}
	if _, _, err := handle.RangeBlobs(beforeCancel, head); !errors.Is(err, context.Canceled) {
		t.Fatalf("RangeBlobs with the parent-tree read cancelled = %v, want context.Canceled", err)
	}
}

// TestObjectReadsReportAMissingParentInAShallowClone covers the escapes that
// claimed a parent object always exists because the parent COUNT was checked.
// It counts recorded parent hashes; resolving one is an object lookup, and a
// shallow clone's boundary commit records a parent whose object was never
// fetched. This is not a corrupt-repository case: `actions/checkout` defaults to
// fetch-depth 1, so `awf audit` in CI reads exactly this shape.
func TestObjectReadsReportAMissingParentInAShallowClone(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	// Six commits, so a depth-3 clone below still has a boundary above it.
	for _, n := range []string{"one", "two", "three", "four", "five", "six"} {
		gitfixture.Commit(t, repo, n, map[string]string{"a.txt": n + "\n"})
	}

	shallow := filepath.Join(t.TempDir(), "shallow")
	// file:// forces the transport that honours --depth; a plain path clone is
	// a local copy and keeps the whole history.
	if out, err := exec.CommandContext(t.Context(), "git", "clone", "--depth", "1", "file://"+dir, shallow).CombinedOutput(); err != nil {
		t.Skipf("shallow clone unavailable in this environment: %v: %s", err, out)
	}
	handle := walkRepo(t, shallow)
	head := gitfixture.NativeRevParse(t, gitfixture.At(shallow), "HEAD")

	if _, _, err := handle.RangeBlobs(testContext(t), head); err == nil {
		t.Error("RangeBlobs resolved a parent the shallow clone never fetched")
	}
	// The range walk reaches the same absent object: it enumerates ancestors of
	// head, and the boundary commit's recorded parent is not there to resolve.
	if _, err := handle.RangeCommits(testContext(t), head, head); err == nil {
		t.Error("RangeCommits walked past a parent the shallow clone never fetched")
	}

	// Merge-base resolution fails the same way, on an ordinary range wholly
	// inside the fetched window: the graph walk it performs runs off the
	// shallow boundary. The escape here previously claimed only a corrupt
	// object graph could reach it.
	deeper := filepath.Join(t.TempDir(), "deeper")
	if out, err := exec.CommandContext(t.Context(), "git", "clone", "--depth", "3", "file://"+dir, deeper).CombinedOutput(); err != nil {
		t.Skipf("shallow clone unavailable in this environment: %v: %s", err, out)
	}
	deepHandle := walkRepo(t, deeper)
	deepHead := gitfixture.NativeRevParse(t, gitfixture.At(deeper), "HEAD")
	deepBase := gitfixture.NativeRevParse(t, gitfixture.At(deeper), "HEAD~2")
	if _, err := deepHandle.RangeCommits(testContext(t), deepBase, deepHead); err == nil {
		t.Error("RangeCommits resolved a merge base across the shallow boundary")
	}
}
