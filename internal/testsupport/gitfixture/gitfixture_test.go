package gitfixture_test

import (
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestInitRepoAndCommit(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, dir, "feat(awf): base", map[string]string{"a.txt": "1\n"})
	if base.IsZero() {
		t.Fatal("expected a non-zero base commit hash")
	}
	head := gitfixture.Commit(t, repo, dir, "feat(awf): head", map[string]string{"b.txt": "2\n"}, "a.txt")
	if head == base {
		t.Fatal("expected head to differ from base")
	}
	c, err := repo.CommitObject(head)
	if err != nil {
		t.Fatal(err)
	}
	if c.Author.Name != gitfixture.Sig.Name || c.Author.Email != gitfixture.Sig.Email {
		t.Errorf("commit author = %+v, want Sig %+v", c.Author, gitfixture.Sig)
	}
}
