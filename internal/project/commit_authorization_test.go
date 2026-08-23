package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/currentstatecoord"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func TestCheckCommitAuthorizationPropagatesEvidenceErrors(t *testing.T) {
	msg := commitmsg.Message{}
	openRoot := func(t *testing.T, root string) *ProjectState {
		t.Helper()
		_, prefix, err := awfgit.OpenContaining(root)
		if err != nil {
			t.Fatal(err)
		}
		_ = prefix
		return testStateAt(root)
	}
	t.Run("no checkout", func(t *testing.T) {
		if _, err := checkCommitAuthorizationProject(testStateAt(t.TempDir()), testContext(t), msg); err == nil {
			t.Fatal("missing checkout succeeded")
		}
	})
	t.Run("missing composed repository", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		if _, err := currentstatecoord.CheckCommitAuthorization(repo.Root(), nil, testContext(t), msg); err == nil || !strings.Contains(err.Error(), "open authorization repository") {
			t.Fatalf("missing composed repository error = %v", err)
		}
	})
	t.Run("malformed repository", func(t *testing.T) {
		root := t.TempDir()
		testsupport.WriteFile(t, filepath.Join(root, ".git", "HEAD"), "broken\n")
		if _, err := checkCommitAuthorizationProject(testStateAt(root), testContext(t), msg); err == nil {
			t.Fatal("malformed repository succeeded")
		}
	})
	t.Run("unborn adopted HEAD", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Stage(t, repo, map[string]string{
			".awf/config.yaml": "prefix: example\nprofile: full\nintegrationBranch: main\n",
			".awf/awf.lock":    `{"awfVersion":"0.18.0","schemaVersion":31,"files":{}}`,
			"docs/decisions/0001-first.md": `---
format: current-state-v4
slug: first
status: Proposed
date: 2026-01-01
---
# ADR-0001: First

## Context

First commit.

## Decision

1. ` + "`decision: adopt-current-format`" + ` Adopt current format.

## State changes

None.

## Consequences

Current format is admitted.

## Alternatives Considered

None.

## Status history

- 2026-01-01: Proposed
`,
		})
		result, err := checkCommitAuthorizationProject(openRoot(t, repo.Root()), testContext(t), msg)
		if err != nil || len(result.NextActions) != 0 {
			t.Fatalf("unborn current-format admission = %#v, %v", result, err)
		}
	})
	t.Run("HEAD fails after repository open", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		p := openRoot(t, repo.Root())
		testsupport.WriteFile(t, filepath.Join(repo.Root(), ".git", "HEAD"), "broken\n")
		if _, err := checkCommitAuthorizationProject(p, testContext(t), msg); err == nil || !strings.Contains(err.Error(), "resolve first-parent HEAD") {
			t.Fatalf("broken HEAD error = %v", err)
		}
	})
	t.Run("unmerged index", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		gitfixture.StageUnmerged(t, repo, "conflict.md")
		if _, err := checkCommitAuthorizationProject(openRoot(t, repo.Root()), testContext(t), msg); err == nil {
			t.Fatal("unmerged index succeeded")
		}
	})
	t.Run("missing incoming object", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		testsupport.WriteFile(t, filepath.Join(repo.Root(), ".git", "MERGE_HEAD"), "0123456789012345678901234567890123456789\n")
		if _, err := checkCommitAuthorizationProject(openRoot(t, repo.Root()), testContext(t), msg); err == nil {
			t.Fatal("missing incoming object succeeded")
		}
	})
	t.Run("malformed first-parent lock", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		gitfixture.Commit(t, repo, "bad lock", map[string]string{".awf/awf.lock": "{"})
		if _, err := checkCommitAuthorizationProject(openRoot(t, repo.Root()), testContext(t), msg); err == nil {
			t.Fatal("malformed first-parent lock succeeded")
		}
	})
	t.Run("malformed result lock", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		gitfixture.Stage(t, repo, map[string]string{".awf/awf.lock": "{"})
		if _, err := checkCommitAuthorizationProject(openRoot(t, repo.Root()), testContext(t), msg); err == nil {
			t.Fatal("malformed result lock succeeded")
		}
	})
	t.Run("malformed result config", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": "["})
		if _, err := checkCommitAuthorizationProject(openRoot(t, repo.Root()), testContext(t), msg); err == nil || !strings.Contains(err.Error(), "load result index current state") {
			t.Fatalf("malformed result config error = %v", err)
		}
	})
	t.Run("malformed incoming lock", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		base := gitfixture.Commit(t, repo, "base", stagedHeadFiles())
		gitfixture.CheckoutNewBranch(t, repo, "bad-parent", base)
		bad := gitfixture.Commit(t, repo, "bad lock", map[string]string{".awf/awf.lock": "{"})
		gitfixture.CheckoutNewBranch(t, repo, "integration", base)
		testsupport.WriteFile(t, filepath.Join(repo.Root(), ".git", "MERGE_HEAD"), bad+"\n")
		if _, err := checkCommitAuthorizationProject(openRoot(t, repo.Root()), testContext(t), msg); err == nil {
			t.Fatal("malformed incoming lock succeeded")
		}
	})
}
