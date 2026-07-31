package gitfixture_test

import (
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestInitRepoAndCommit pins the go-git lane's neutral contract: hex commit
// ids, a Fixture that reports its own root, and the fixed authorship both lanes
// share.
func TestInitRepoAndCommit(t *testing.T) {
	t.Parallel()
	fx := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, fx, "feat(awf): base", map[string]string{"a.txt": "1\n"})
	if base == "" || plumbing.NewHash(base).IsZero() {
		t.Fatalf("expected a non-zero base commit hash, got %q", base)
	}
	head := gitfixture.Commit(t, fx, "feat(awf): head", map[string]string{"b.txt": "2\n"}, "a.txt")
	if head == base {
		t.Fatal("expected head to differ from base")
	}
	repo, err := gogit.PlainOpen(fx.Root())
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(plumbing.NewHash(head))
	if err != nil {
		t.Fatal(err)
	}
	if c.Author.Name != gitfixture.Sig.Name || c.Author.Email != gitfixture.Sig.Email {
		t.Errorf("commit author = %+v, want Sig %+v", c.Author, gitfixture.Sig)
	}
}

// TestNativeLaneIsolation proves the native lane ignores a hostile inherited
// git environment: the fixture repository it builds is the one the test named,
// not whatever GIT_DIR points at.
func TestNativeLaneIsolation(t *testing.T) {
	t.Setenv("GIT_DIR", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", "/nonexistent/gitconfig")
	fx := gitfixture.InitNativeAt(t, t.TempDir())
	if err := os.WriteFile(filepath.Join(fx.Root(), "a.txt"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, fx, "a.txt")
	gitfixture.NativeCommit(t, fx, "base")
	if head := gitfixture.NativeRevParse(t, fx, "HEAD"); head == "" {
		t.Fatal("native commit left no resolvable HEAD")
	}
	if gitfixture.NativeRevisionExists(t, fx, "refs/heads/does-not-exist") {
		t.Fatal("absent branch reported as resolvable")
	}
}
