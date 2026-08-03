package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	fixture := gitfixture.InitNativeAt(t, t.TempDir())
	base := gitfixture.NativeCommitAllowEmpty(t, fixture, "base")
	head := gitfixture.NativeCommitAllowEmpty(t, fixture, "head")
	gitfixture.NativeLightweightTag(t, fixture, "light", head)
	gitfixture.NativeAnnotatedTag(t, fixture, "inner", head)
	gitfixture.NativeAnnotatedTag(t, fixture, "outer", "inner")
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
	commits, err := repo.CommitsAfter(ctx, base, []string{"HEAD", "light", "outer", base + "..HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].ID != head {
		t.Fatalf("CommitsAfter = %#v", commits)
	}
	peeled, terminal, err := repo.PeelCommit(ctx, "outer")
	if err != nil || peeled != head || terminal != "commit" {
		t.Fatalf("nested tag peel = %q, %q, %v", peeled, terminal, err)
	}
	_, terminal, err = repo.PeelCommit(ctx, "HEAD^{tree}")
	if err != nil || terminal != "tree" {
		t.Fatalf("non-commit peel = %q, %v", terminal, err)
	}
	_, err = repo.CommitsAfter(ctx, head[:8], []string{"HEAD"})
	assertPolicyError(t, err, CommitPolicyBaselineError)
	_, err = repo.CommitsAfter(ctx, base, []string{"does-not-exist"})
	assertPolicyError(t, err, CommitPolicyRevisionError)
	_, err = repo.CommitsAfter(ctx, base, []string{"HEAD^{tree}"})
	assertPolicyError(t, err, CommitPolicyTagPeelError)
	_, err = repo.CommitsAfter(ctx, base, nil)
	assertPolicyError(t, err, CommitPolicyRevisionError)
	_, err = repo.CommitsAfter(ctx, strings.Repeat("f", len(base)), []string{"HEAD"})
	assertPolicyError(t, err, CommitPolicyBaselineError)
	tree := gitfixture.NativeRevParse(t, fixture, "HEAD^{tree}")
	_, err = repo.CommitsAfter(ctx, tree, []string{"HEAD"})
	assertPolicyError(t, err, CommitPolicyBaselineError)
	_, err = repo.CommitsAfter(ctx, base, []string{"does-not-exist..HEAD"})
	assertPolicyError(t, err, CommitPolicyRevisionError)
	if _, err := repo.CommitFacts(ctx, strings.Repeat("f", len(base))); err == nil {
		t.Fatal("missing commit facts succeeded")
	}
}

func TestCommitPolicySupportsRepositoryObjectFormatWidth(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			fixture := gitfixture.InitNativeObjectFormat(t, t.TempDir(), format)
			base := gitfixture.NativeCommitAllowEmpty(t, fixture, "base")
			head := gitfixture.NativeCommitAllowEmpty(t, fixture, "head")
			repo, err := Open(fixture.Root())
			if err != nil {
				t.Fatal(err)
			}
			commits, err := repo.CommitsAfter(commitPolicyContext(t), base, []string{"HEAD"})
			if err != nil || len(commits) != 1 || commits[0].ID != head {
				t.Fatalf("%s commits = %#v, %v", format, commits, err)
			}
			wantWidth := 40
			if format == "sha256" {
				wantWidth = 64
			}
			if len(head) != wantWidth {
				t.Fatalf("%s object width = %d", format, len(head))
			}
		})
	}
}

func TestVerifySSHUsesRealGitAndCleansTrustFile(t *testing.T) {
	ctx := commitPolicyContext(t)
	fixture := gitfixture.InitNativeAt(t, t.TempDir())
	unsigned := gitfixture.NativeCommitAllowEmpty(t, fixture, "unsigned")
	privateKey, publicKey := gitfixture.NativeSSHKey(t)
	signed := gitfixture.NativeSignedCommit(t, fixture, "signed", privateKey)
	raw := gitfixture.NativeCatFile(t, fixture, "commit", signed)
	malformedRaw := strings.Replace(string(raw), "U1NIU0lH", "!1NIU0lH", 1)
	if malformedRaw == string(raw) {
		t.Fatal("signed fixture did not contain OpenSSH signature payload")
	}
	malformed := gitfixture.NativeHashObject(t, fixture, "commit", []byte(malformedRaw))
	_, otherKey := gitfixture.NativeSSHKey(t)
	repo, err := Open(fixture.Root())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		id      string
		signers []commitpolicy.Signer
		want    commitpolicy.SignatureVerdict
	}{
		{"unsigned", unsigned, []commitpolicy.Signer{{Principal: "t@example.com", Key: publicKey}}, commitpolicy.SignatureMissing},
		{"valid", signed, []commitpolicy.Signer{{Principal: "t@example.com", Key: publicKey}}, commitpolicy.SignatureValid},
		{"malformed", malformed, []commitpolicy.Signer{{Principal: "t@example.com", Key: publicKey}}, commitpolicy.SignatureMalformed},
		{"wrong-key", signed, []commitpolicy.Signer{{Principal: "t@example.com", Key: otherKey}}, commitpolicy.SignatureWrongKey},
		{"unknown-signer", signed, nil, commitpolicy.SignatureWrongKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.VerifySSH(ctx, tc.id, tc.signers)
			if err != nil || got != tc.want {
				t.Fatalf("VerifySSH = %v, %v, want %v", got, err, tc.want)
			}
		})
	}
	matches, err := filepath.Glob(filepath.Join(fixture.Root(), ".awf-allowed-signers-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary allowed-signers files = %v, %v", matches, err)
	}
}

func TestVerifySSHReportsTrustAndProcessFailures(t *testing.T) {
	ctx := commitPolicyContext(t)
	fixture := gitfixture.InitNativeAt(t, t.TempDir())
	privateKey, publicKey := gitfixture.NativeSSHKey(t)
	signed := gitfixture.NativeSignedCommit(t, fixture, "signed", privateKey)
	repo, err := Open(fixture.Root())
	if err != nil {
		t.Fatal(err)
	}
	repo.createTemp = func(string, string) (*os.File, error) { return nil, errors.New("no temporary file") }
	_, err = repo.VerifySSH(ctx, signed, []commitpolicy.Signer{{Principal: "t@example.com", Key: publicKey}})
	assertPolicyError(t, err, CommitPolicyTrustError)

	repo.createTemp = nil
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = repo.VerifySSH(cancelled, signed, []commitpolicy.Signer{{Principal: "t@example.com", Key: publicKey}})
	assertPolicyError(t, err, CommitPolicyVerifyError)
}

func TestCommitPolicyRejectsMalformedNativeResponses(t *testing.T) {
	if (*CommitPolicyError)(nil).Error() == "" || (*CommitPolicyError)(nil).Unwrap() != nil {
		t.Fatal("nil policy error contract changed")
	}
	if validObjectID(strings.Repeat("z", 40), 40) || validObjectID("abc", 40) {
		t.Fatal("invalid object ID accepted")
	}
	for _, raw := range []string{
		"tree a\nauthor malformed\ncommitter T <t@example.com> 1 +0000\n\n",
		"tree a\nauthor T <t@example.com> 1 +0000\ncommitter malformed\n\n",
		"tree a\nauthor T <t@example.com> 1 +0000\n\n",
	} {
		if _, _, err := parseCommitPeople([]byte(raw)); err == nil {
			t.Fatalf("malformed commit people accepted: %q", raw)
		}
	}

	repo := fakeCommitPolicyRepo(t)
	ctx := commitPolicyContext(t)
	for _, mode := range []string{"format-fail", "bad-format", "bad-oid"} {
		t.Setenv("AWF_FAKE_GIT_MODE", mode)
		if _, _, err := repo.FullOID(ctx, "HEAD"); err == nil {
			t.Fatalf("FullOID accepted fake mode %s", mode)
		}
	}
	t.Setenv("AWF_FAKE_GIT_MODE", "format-fail")
	_, err := repo.CommitsAfter(ctx, strings.Repeat("a", 40), []string{"HEAD"})
	assertPolicyError(t, err, CommitPolicyBaselineError)
	for _, raw := range []string{"author malformed\ncommitter T <t@example.com> 1 +0000\n\n", "author T <t@example.com> 1 +0000\ncommitter malformed\n\n", "author T <t@example.com> 1 +0000\n\n"} {
		t.Setenv("AWF_FAKE_GIT_MODE", "commit-body")
		t.Setenv("AWF_FAKE_COMMIT_BODY", raw)
		if _, err := repo.CommitFacts(ctx, strings.Repeat("a", 40)); err == nil {
			t.Fatalf("CommitFacts accepted %q", raw)
		}
	}
	t.Setenv("AWF_FAKE_GIT_MODE", "verify-process")
	_, err = repo.VerifySSH(ctx, strings.Repeat("a", 40), nil)
	assertPolicyError(t, err, CommitPolicyVerifyError)
}

func fakeCommitPolicyRepo(t *testing.T) *Repo {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
case "$AWF_FAKE_GIT_MODE:$*" in
  format-fail:*--show-object-format*) exit 2 ;;
  bad-format:*--show-object-format*) printf 'sha512\n' ;;
  bad-oid:*--show-object-format*) printf 'sha1\n' ;;
  bad-oid:*rev-parse*) printf 'bad\n' ;;
  commit-body:*cat-file\ commit*) printf '%s' "$AWF_FAKE_COMMIT_BODY" ;;
  verify-process:*cat-file\ commit*) printf 'tree a\ngpgsig -----BEGIN SSH SIGNATURE-----\n YWJj\n -----END SSH SIGNATURE-----\n\nmessage\n' ;;
  verify-process:*verify-commit*) exit 2 ;;
  *--show-object-format*) printf 'sha1\n' ;;
  *rev-parse*) printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n' ;;
  *cat-file\ -t*) printf 'commit\n' ;;
  *) exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &Repo{root: dir, runner: newRunner(dir)}
}

func assertPolicyError(t *testing.T, err error, want CommitPolicyErrorKind) {
	t.Helper()
	var policyErr *CommitPolicyError
	if !errors.As(err, &policyErr) || policyErr.Kind != want {
		t.Fatalf("error = %v, want commit-policy kind %s", err, want)
	}
	if policyErr.Error() == "" || policyErr.Unwrap() == nil {
		t.Fatalf("incomplete policy error = %#v", policyErr)
	}
	if !strings.Contains(policyErr.Error(), string(want)) {
		t.Fatalf("error %q does not name kind %s", policyErr, want)
	}
}
