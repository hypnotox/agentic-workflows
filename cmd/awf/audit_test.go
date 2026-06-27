package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var auditSig = &object.Signature{Name: "T", Email: "t@example.com", When: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

// auditProject creates a temp project (minimal .awf config) with a git repo and
// a base commit, returning the root and the base commit hash.
func auditProject(t *testing.T) (string, plumbing.Hash) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "prefix: example\nskills: []\nagents: []\nhooks: []\n"
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	// Sync writes the lock so Project.Audit's generated-path set is populated.
	if err := runSync(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatal(err)
	}
	// Stage everything (synced scaffold + source) so the baseline working tree is
	// clean — otherwise the uncommitted-changes rule (ADR-0025) fires on the
	// untracked synced files.
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	base, err := wt.Commit("feat(awf): base", &git.CommitOptions{Author: auditSig, Committer: auditSig})
	if err != nil {
		t.Fatal(err)
	}
	return root, base
}

func auditCommit(t *testing.T, repo *git.Repository, root, msg string, write map[string]string) plumbing.Hash {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range write {
		if werr := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); werr != nil {
			t.Fatal(werr)
		}
		if _, aerr := wt.Add(name); aerr != nil {
			t.Fatal(aerr)
		}
	}
	h, err := wt.Commit(msg, &git.CommitOptions{Author: auditSig, Committer: auditSig})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// invariant: audit-warn-exit-zero
func TestRunAuditWarningsExitZero(t *testing.T) {
	root, base := auditProject(t)
	repo, err := git.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	// Valid CC subject, but touches go.mod with no ADR -> dependency-adr warning only.
	auditCommit(t, repo, root, "feat(awf): bump a dependency", map[string]string{"go.mod": "module x\n// dep\n"})
	var out bytes.Buffer
	if err := runAudit(root, base.String(), &out); err != nil {
		t.Fatalf("warnings-only run should exit zero, got: %v", err)
	}
	if !strings.Contains(out.String(), "warning") {
		t.Errorf("expected a warning in output, got: %q", out.String())
	}
}

func TestRunAuditErrorExitsNonZero(t *testing.T) {
	root, base := auditProject(t)
	repo, err := git.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	auditCommit(t, repo, root, "not a conventional commit subject", map[string]string{"main.go": "package x\nvar y int\n"})
	if err := runAudit(root, base.String(), out(t)); err == nil {
		t.Fatal("an Error finding must make runAudit return non-nil")
	}
}

// A branch-level finding (plan-for-large-change has no commit hash) exercises the
// loc == "branch" label path in runAudit.
func TestRunAuditBranchLevelFinding(t *testing.T) {
	root, base := auditProject(t)
	repo, err := git.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("var n int\n", 500) // > default diffThreshold 400
	auditCommit(t, repo, root, "feat(awf): big change", map[string]string{"big.go": "package x\n" + big})
	var buf bytes.Buffer
	if err := runAudit(root, base.String(), &buf); err != nil {
		t.Fatalf("branch-level warning should exit zero, got: %v", err)
	}
	if !strings.Contains(buf.String(), "branch") {
		t.Errorf("expected a branch-level finding, got: %q", buf.String())
	}
}

func TestRunAuditCleanRange(t *testing.T) {
	root, base := auditProject(t)
	repo, err := git.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	auditCommit(t, repo, root, "feat(awf): small clean change", map[string]string{"main.go": "package x\nvar z int\n"})
	var buf bytes.Buffer
	if err := runAudit(root, base.String(), &buf); err != nil {
		t.Fatalf("clean range should exit zero, got: %v", err)
	}
	if !strings.Contains(buf.String(), "awf audit: clean") {
		t.Errorf("expected clean message, got: %q", buf.String())
	}
}

func TestRunAuditOpenError(t *testing.T) {
	// A dir with no .awf/config.yaml -> project.Open fails.
	if err := runAudit(t.TempDir(), "", out(t)); err == nil {
		t.Fatal("expected a project.Open error")
	}
}

func TestRunAuditAuditError(t *testing.T) {
	root, _ := auditProject(t)
	// Unresolvable base ref -> p.Audit (Collect) errors after Open succeeds.
	if err := runAudit(root, "no-such-ref", out(t)); err == nil {
		t.Fatal("expected a p.Audit error for an unresolvable base")
	}
}

// out returns a throwaway writer for cases that only assert on the error.
func out(t *testing.T) *bytes.Buffer {
	t.Helper()
	return &bytes.Buffer{}
}

func TestBaseFlag(t *testing.T) {
	if v := baseFlag([]string{"awf", "audit", "--base", "main"}); v != "main" {
		t.Errorf("present: got %q, want main", v)
	}
	if v := baseFlag([]string{"awf", "audit"}); v != "" {
		t.Errorf("absent: got %q, want empty", v)
	}
	if v := baseFlag([]string{"awf", "audit", "--base"}); v != "" {
		t.Errorf("flag without value: got %q, want empty", v)
	}
}

// TestRunAuditDispatch drives the `audit` switch arm through run(), covering the
// dispatch statement and the --base flag plumbing.
func TestRunAuditDispatch(t *testing.T) {
	root, base := auditProject(t)
	repo, err := git.PlainOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	auditCommit(t, repo, root, "feat(awf): clean change", map[string]string{"main.go": "package x\nvar z int\n"})
	swapGetwd(t, func() (string, error) { return root, nil })
	var outb, errb bytes.Buffer
	if code := run([]string{"awf", "audit", "--base", base.String()}, &outb, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
	}
}
