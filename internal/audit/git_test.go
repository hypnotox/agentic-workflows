package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// orphan stores an unrelated root commit (empty tree, no parents).
func orphan(t *testing.T, repo *git.Repository) plumbing.Hash {
	t.Helper()
	tree := &object.Tree{}
	to := repo.Storer.NewEncodedObject()
	if err := tree.Encode(to); err != nil {
		t.Fatalf("encode tree: %v", err)
	}
	treeHash, err := repo.Storer.SetEncodedObject(to)
	if err != nil {
		t.Fatalf("store tree: %v", err)
	}
	c := &object.Commit{Author: *gitfixture.Sig, Committer: *gitfixture.Sig, Message: "orphan\n", TreeHash: treeHash}
	co := repo.Storer.NewEncodedObject()
	if err := c.Encode(co); err != nil {
		t.Fatalf("encode commit: %v", err)
	}
	h, err := repo.Storer.SetEncodedObject(co)
	if err != nil {
		t.Fatalf("store commit: %v", err)
	}
	return h
}

// enableWorktreeConfigExtension mirrors go-git's own TestVerifyExtensions setup
// (repository_extensions_test.go): it sets extensions.worktreeConfig on the repo's
// config the way `git config extensions.worktreeConfig true` would, reproducing a
// repo state real projects can carry (a leftover from a since-removed `git worktree
// add`), which open real git via git.PlainOpen must tolerate.
func enableWorktreeConfigExtension(t *testing.T, repo *git.Repository) {
	t.Helper()
	cfg, err := repo.Storer.Config()
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	cfg.Raw.Section("extensions").SetOption("worktreeConfig", "true")
	if err := repo.Storer.SetConfig(cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func findChange(changes []FileChange, path string) (FileChange, bool) {
	for _, ch := range changes {
		if ch.Path == path {
			return ch, true
		}
	}
	return FileChange{}, false
}

func TestCollectNormalRange(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, dir, "feat(awf): base", map[string]string{
		"go.mod": "module x\n",
		"a.md":   "---\nstatus: Proposed\n---\nold body\n",
	})
	gitfixture.Commit(t, repo, dir, "feat(awf): one", map[string]string{
		"a.md":  "---\nstatus: Accepted\n---\nnew longer body\nwith more lines\n",
		"b.txt": "data\n",
	})
	gitfixture.Commit(t, repo, dir, "fix(awf): two", map[string]string{"c.md": "new\n"}, "b.txt")

	commits, err := Collect(dir, base.String())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	if commits[0].Subject != "fix(awf): two" || commits[1].Subject != "feat(awf): one" {
		t.Fatalf("unexpected order: %q, %q", commits[0].Subject, commits[1].Subject)
	}
	// commit "one": a.md modified (md content captured + stats), b.txt added (non-md, no text).
	one := commits[1]
	amd, ok := findChange(one.Changes, "a.md")
	if !ok || amd.Action != Modified {
		t.Fatalf("a.md change missing/not modified: %+v", amd)
	}
	if amd.OldText == "" || amd.NewText == "" {
		t.Errorf("a.md md text not captured: old=%q new=%q", amd.OldText, amd.NewText)
	}
	if amd.Added == 0 && amd.Deleted == 0 {
		t.Errorf("a.md stats empty: +%d -%d", amd.Added, amd.Deleted)
	}
	btxt, ok := findChange(one.Changes, "b.txt")
	if !ok || btxt.Action != Added {
		t.Fatalf("b.txt change missing/not added: %+v", btxt)
	}
	if btxt.OldText != "" || btxt.NewText != "" {
		t.Errorf("non-md b.txt should carry no text: old=%q new=%q", btxt.OldText, btxt.NewText)
	}
	// commit "two": b.txt deleted, c.md added.
	two := commits[0]
	if del, ok := findChange(two.Changes, "b.txt"); !ok || del.Action != Deleted {
		t.Fatalf("b.txt not deleted in commit two: %+v", del)
	}
	if cmd, ok := findChange(two.Changes, "c.md"); !ok || cmd.Action != Added || cmd.NewText != "new\n" {
		t.Fatalf("c.md add/content wrong: %+v", cmd)
	}
}

// invariant: audit-empty-range-clean
// A merge commit's first-parent diff is the *other* side's work (typically
// main merged into the branch); attributing it to the branch makes every
// file-based rule fire on foreign changes. Merges must carry no Changes.
func TestCollectMergeCommitCarriesNoChanges(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	c0 := gitfixture.Commit(t, repo, dir, "chore: base\n", map[string]string{"README.md": "base\n"})
	// Advance the base branch (master) by one commit.
	m1 := gitfixture.Commit(t, repo, dir, "feat: main-side work\n", map[string]string{"mainside.txt": "big change on main\n"})
	// Branch from c0 and do the branch's own work.
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Hash: c0, Branch: plumbing.NewBranchReferenceName("feature"), Create: true}); err != nil {
		t.Fatalf("checkout feature: %v", err)
	}
	f1 := gitfixture.Commit(t, repo, dir, "feat(awf): branch work\n", map[string]string{"branch.txt": "branch change\n"})
	// Merge master into the branch: tree carries main's file, parents [f1, m1].
	if err := os.WriteFile(filepath.Join(dir, "mainside.txt"), []byte("big change on main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("mainside.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("Merge branch 'master' into feature\n", &git.CommitOptions{
		Author: gitfixture.Sig, Committer: gitfixture.Sig,
		Parents: []plumbing.Hash{f1, m1},
	}); err != nil {
		t.Fatal(err)
	}
	commits, err := Collect(dir, "master")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	var sawMerge bool
	for _, c := range commits {
		if !c.IsMerge {
			continue
		}
		sawMerge = true
		if len(c.Changes) != 0 {
			t.Errorf("merge commit carries %d changes (first-parent diff of the merged-in side); want 0: %+v", len(c.Changes), c.Changes)
		}
	}
	if !sawMerge {
		t.Fatalf("no merge commit collected; commits = %+v", commits)
	}
}

func TestCollectEmptyRangeIsClean(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "feat(awf): base", map[string]string{"go.mod": "module x\n"})
	head := gitfixture.Commit(t, repo, dir, "feat(awf): head", map[string]string{"a.txt": "x\n"})

	commits, err := Collect(dir, head.String())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if commits != nil {
		t.Fatalf("empty range should be nil, got %d commits", len(commits))
	}
	// Drive Run for the empty range: clean, no error.
	findings, err := Run(dir, Inputs{Settings: Settings{BaseBranch: head.String()}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("empty range yielded findings: %v", findings)
	}
}

func TestRunPropagatesCollectError(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "feat(awf): base", map[string]string{"go.mod": "module x\n"})
	if _, err := Run(dir, Inputs{Settings: Settings{BaseBranch: "no-such-ref"}}); err == nil {
		t.Fatal("expected Run to propagate an unresolvable-base error")
	}
}

func TestCollectUnrelatedHistories(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "feat(awf): head", map[string]string{"go.mod": "module x\n"})
	base := orphan(t, repo)
	_, err := Collect(dir, base.String())
	if err == nil {
		t.Fatal("expected an unrelated-histories error")
	}
}

func TestCollectNotARepo(t *testing.T) {
	if _, err := Collect(t.TempDir(), "main"); err == nil {
		t.Fatal("expected a not-a-repo error")
	}
}

func TestCollectEmptyRepoHeadError(t *testing.T) {
	_, dir := gitfixture.InitRepo(t)
	if _, err := Collect(dir, "main"); err == nil {
		t.Fatal("expected a HEAD-resolution error on a commit-less repo")
	}
}

func TestCollectBadBase(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "feat(awf): base", map[string]string{"go.mod": "module x\n"})
	if _, err := Collect(dir, "no-such-ref"); err == nil {
		t.Fatal("expected an unresolvable-base error")
	}
}

func TestSplitMessage(t *testing.T) {
	if s, b := splitMessage("just a subject"); s != "just a subject" || b != "" {
		t.Errorf("no-newline: %q / %q", s, b)
	}
	if s, b := splitMessage("subject line\n\nbody text\nmore\n"); s != "subject line" || b != "body text\nmore" {
		t.Errorf("multiline: %q / %q", s, b)
	}
}

// toCommit on a root commit exercises the no-parent (nil parentTree) path that
// Collect never reaches (a valid range's base is always a common ancestor).
func TestToCommitRootAndFileText(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	root := gitfixture.Commit(t, repo, dir, "feat(awf): root", map[string]string{"a.md": "---\nstatus: Proposed\n---\nx\n"})
	rc, err := repo.CommitObject(root)
	if err != nil {
		t.Fatal(err)
	}
	nc, err := toCommit(rc)
	if err != nil {
		t.Fatalf("toCommit root: %v", err)
	}
	if amd, ok := findChange(nc.Changes, "a.md"); !ok || amd.Action != Added || amd.NewText == "" {
		t.Fatalf("root a.md: %+v", amd)
	}
	tree, err := rc.Tree()
	if err != nil {
		t.Fatal(err)
	}
	if fileText(tree, "missing.md") != "" {
		t.Error("fileText(missing) should be empty")
	}
}

func TestRuleUncommittedChanges(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "init", map[string]string{"a.txt": "a"})

	// Clean tree: no finding.
	if f := ruleUncommittedChanges(dir, Inputs{Settings: Settings{UncommittedChanges: true}}); len(f) != 0 {
		t.Fatalf("clean tree should yield no finding, got %#v", f)
	}

	// Dirty tree — a modified tracked file (tracked count) plus an untracked file
	// (untracked count): one Error finding.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "uncommitted.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := ruleUncommittedChanges(dir, Inputs{Settings: Settings{UncommittedChanges: true}})
	if len(f) != 1 || f[0].Rule != "uncommitted-changes" || f[0].Severity != Error || f[0].Commit != "" {
		t.Fatalf("dirty tree finding = %#v", f)
	}
	// The tracked/untracked tally is load-bearing in the Detail message; assert
	// the exact Detail so both a miscount in the classification loop and an
	// off-by-one on either counter are caught (a substring match would let a
	// "-1"/"11" count slip through since it contains the "1" token).
	wantDetail := "working tree not clean: 1 tracked change(s), 1 untracked file(s); commit or discard before concluding the implementation"
	if f[0].Detail != wantDetail {
		t.Errorf("Detail mismatch:\n got %q\nwant %q", f[0].Detail, wantDetail)
	}

	// Disabled: no finding even when dirty.
	if f := ruleUncommittedChanges(dir, Inputs{Settings: Settings{UncommittedChanges: false}}); len(f) != 0 {
		t.Fatalf("disabled rule should yield no finding, got %#v", f)
	}
}

func TestRunIncludesUncommittedChanges(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "init", map[string]string{"a.txt": "a"})
	if err := os.WriteFile(filepath.Join(dir, "uncommitted.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Empty range (base == HEAD) so only the live-state rule can fire.
	findings, err := Run(dir, Inputs{Settings: Settings{BaseBranch: "HEAD", UncommittedChanges: true}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got bool
	for _, f := range findings {
		if f.Rule == "uncommitted-changes" {
			got = true
		}
	}
	if !got {
		t.Errorf("Run did not surface uncommitted-changes: %#v", findings)
	}
}

func TestCollectWorktreeConfigExtension(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, dir, "feat(awf): base", map[string]string{"go.mod": "module x\n"})
	gitfixture.Commit(t, repo, dir, "feat(awf): one", map[string]string{"a.txt": "x\n"})
	enableWorktreeConfigExtension(t, repo)

	commits, err := Collect(dir, base.String())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(commits))
	}
}

func TestRuleUncommittedChangesWorktreeConfigExtension(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Commit(t, repo, dir, "init", map[string]string{"a.txt": "a"})
	enableWorktreeConfigExtension(t, repo)

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := ruleUncommittedChanges(dir, Inputs{Settings: Settings{UncommittedChanges: true}})
	if len(f) != 1 || f[0].Rule != "uncommitted-changes" {
		t.Fatalf("dirty tree finding = %#v", f)
	}
}
