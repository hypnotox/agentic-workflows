package git

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/commitpolicy"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func commitPolicyContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestCommitPolicyGitReads(t *testing.T) {
	ctx := commitPolicyContext(t)
	fixture := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, fixture, "base", map[string]string{"base": "x"})
	head := gitfixture.Commit(t, fixture, "head", map[string]string{"head": "y"})
	repo, err := Open(fixture.Root())
	if err != nil {
		t.Fatal(err)
	}
	id, typ, err := repo.FullOID(ctx, "HEAD")
	if err != nil || id != head || typ != "commit" {
		t.Fatalf("FullOID = %q, %q, %v", id, typ, err)
	}
	fact, err := repo.CommitFacts(ctx, head)
	if err != nil {
		t.Fatal(err)
	}
	if fact.ID != head || fact.Author.Name != "T" || fact.Committer.Email != "t@example.com" {
		t.Fatalf("facts = %#v", fact)
	}
	commits, err := repo.CommitsAfter(ctx, base, []string{"HEAD", "HEAD", base + "..HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].ID != head {
		t.Fatalf("CommitsAfter = %#v", commits)
	}
	if _, err := repo.CommitsAfter(ctx, head[:8], []string{"HEAD"}); err == nil {
		t.Fatal("accepted abbreviated baseline")
	}
	if _, _, err := repo.PeelCommit(ctx, "does-not-exist"); err == nil {
		t.Fatal("accepted missing target")
	}
	_, typ, err = repo.PeelCommit(ctx, "HEAD^{tree}")
	if err != nil || typ != "tree" {
		t.Fatalf("non-commit peel = %q, %v", typ, err)
	}
	verdict, err := repo.VerifySSH(ctx, head, []commitpolicy.Signer{{Principal: "x", Key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA=="}})
	if err != nil || verdict != commitpolicy.SignatureMissing {
		t.Fatalf("unsigned verification = %v, %v", verdict, err)
	}
}

func TestVerifySSHRefusesCancelledProcess(t *testing.T) {
	fixture := gitfixture.InitRepo(t)
	head := gitfixture.Commit(t, fixture, "head", map[string]string{"x": "x"})
	repo, err := Open(fixture.Root())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(commitPolicyContext(t))
	cancel()
	_, err = repo.VerifySSH(ctx, head, nil)
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("cancelled verification = %v", err)
	}
}
