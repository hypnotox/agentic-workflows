package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// auditProject creates a temp project (minimal .awf config) with a git repo and
// a base commit, returning the root and the base commit hash.
func auditProject(t *testing.T) (gitfixture.Fixture, string) {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\n")
	// Sync writes the lock so the configured audit operation can identify generated paths.
	if err := initializeProject(testContext(t), root, io.Discard); err != nil {
		t.Fatal(err)
	}
	repo := gitfixture.InitRepoAt(t, root)
	// Stage everything (synced scaffold + source) so the baseline working tree is
	// clean - otherwise the uncommitted-changes rule (ADR-0025) fires on the
	// untracked synced files.
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitfixture.AddAll(t, repo)
	return repo, gitfixture.Commit(t, repo, "feat(awf): base", nil)
}

// invariant: tooling/audit-commands:audit-warn-exit-zero (TestRunAuditWarningsExitZero)
func TestRunAuditWarningsExitZero(t *testing.T) {
	repo, base := auditProject(t)
	root := repo.Root()
	// Valid CC subject, but touches go.mod with no ADR -> dependency-adr warn only.
	gitfixture.Commit(t, repo, "feat(awf): bump a dependency", map[string]string{"go.mod": "module x\n// dep\n"})
	var out bytes.Buffer
	if err := runAudit(testContext(t), root, base, &out); err != nil {
		t.Fatalf("warnings-only run should exit zero, got: %v", err)
	}
	// The readable category is plural while the domain rank remains warn.
	if !strings.Contains(out.String(), "warnings:\n    dependency-adr |") {
		t.Errorf("expected a warn-ranked dependency-adr finding, got: %q", out.String())
	}
}

func TestRunAuditPropagatesAuditFailure(t *testing.T) {
	repo, _ := auditProject(t)
	if err := runAudit(testContext(t), repo.Root(), "does-not-exist", out(t)); err == nil {
		t.Fatal("unresolvable audit range accepted")
	}
}

func TestPresentAuditRefusal(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "below horizon", err: &audit.HistoricalHorizonError{Schema: 2, Floor: 3, Horizon: 46}, want: "supporting schemas 3 through 46"},
		{name: "above horizon", err: &audit.HistoricalHorizonError{Schema: 47, Floor: 3, Horizon: 46}, want: "supporting schemas 3 through 46"},
		{name: "partial authority", err: &audit.PartialHistoricalAuthorityError{Config: true, Lock: false}, want: "restore the complete .awf/config.yaml and .awf/awf.lock pair"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := presentAuditRefusal(tc.err)
			if !errors.Is(got, tc.err) || !strings.Contains(got.Error(), tc.want) {
				t.Fatalf("refusal = %v, want wrapped source and %q", got, tc.want)
			}
		})
	}
	other := errors.New("other audit failure")
	if got := presentAuditRefusal(other); !errors.Is(got, other) {
		t.Fatalf("unclassified refusal = %v, want source identity", got)
	}
}

func TestRunAuditPropagatesWriterFailure(t *testing.T) {
	repo, base := auditProject(t)
	gitfixture.Commit(t, repo, "feat(awf): clean change", map[string]string{"main.go": "package x\nvar z int\n"})
	if err := runAudit(testContext(t), repo.Root(), base, &failOnWrite{failAt: 1, err: io.ErrClosedPipe}); err == nil {
		t.Fatal("writer failure accepted")
	}
}

func TestRunAuditErrorExitsNonZero(t *testing.T) {
	repo, base := auditProject(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "not a conventional commit subject", map[string]string{"main.go": "package x\nvar y int\n"})
	if err := runAudit(testContext(t), root, base, out(t)); err == nil {
		t.Fatal("an error-ranked finding must make runAudit return non-nil")
	}
}

// A missing range is refused before the project is even opened: an audit that
// silently reports over nothing is worse than one that refuses (ADR-0127
// Decision 2).
// invariant: tooling/audit-commands:audit-requires-explicit-range (TestRunAuditRequiresARange)
func TestRunAuditRequiresARange(t *testing.T) {
	err := runAudit(testContext(t), t.TempDir(), "", out(t))
	if err == nil {
		t.Fatal("a missing range must be refused")
	}
	msg := err.Error()
	if !strings.Contains(msg, "<base>") || !strings.Contains(msg, "<a>..<b>") {
		t.Errorf("the refusal must name both accepted forms, got %q", msg)
	}
}

// A malformed range is refused by the shared parser before the project opens.
func TestRunAuditRejectsMalformedRange(t *testing.T) {
	err := runAudit(testContext(t), t.TempDir(), "a...b", out(t))
	if err == nil {
		t.Fatal("a three-dot range must be refused")
	}
	if !strings.Contains(err.Error(), "exactly two dots") {
		t.Errorf("expected the parser's diagnosis, got %q", err)
	}
}

func TestRunAuditOpenError(t *testing.T) {
	// A dir with no .awf/config.yaml -> project.Open fails. The range is valid,
	// so this reaches Open rather than stopping at the refusal above.
	if err := runAudit(testContext(t), t.TempDir(), "HEAD", out(t)); err == nil {
		t.Fatal("expected a project.Open error")
	}
}

// out returns a throwaway writer for cases that only assert on the error.
func out(t *testing.T) *bytes.Buffer {
	t.Helper()
	return &bytes.Buffer{}
}

// TestRunAuditDispatch drives the `audit` switch arm through run(), covering the
// dispatch statement and the positional range argument (ADR-0127 Decision 1).
func TestRunAuditDispatch(t *testing.T) {
	repo, base := auditProject(t)
	root := repo.Root()
	gitfixture.Commit(t, repo, "feat(awf): clean change", map[string]string{"main.go": "package x\nvar z int\n"})
	var outb, errb bytes.Buffer
	if code := runFrom(root, []string{"awf", "audit", base}, &outb, &errb); code != 0 {
		t.Fatalf("expected exit 0, got %d (%s)", code, errb.String())
	}
}

// TestRunAuditRoutesConfiguredWorkToAudit proves the command keeps only its
// CLI boundary role rather than reconstructing audit inputs or reports.
func TestRunAuditRoutesConfiguredWorkToAudit(t *testing.T) {
	source, err := os.ReadFile("audit.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "audit.RunConfigured(") {
		t.Fatal("runAudit must invoke the audit-owned configured operation")
	}
	for _, obsolete := range []string{"project.Audit(", "audit.Report("} {
		if strings.Contains(text, obsolete) {
			t.Fatalf("runAudit retains obsolete audit orchestration route %q", obsolete)
		}
	}
}

func TestRunAuditDispatchFailingReport(t *testing.T) {
	repo, base := auditProject(t)
	root := repo.Root()
	commit := gitfixture.Commit(t, repo, "not a conventional commit subject", map[string]string{"main.go": "package x\nvar y int\n"})
	var stdout, stderr bytes.Buffer
	code := runFrom(root, []string{"awf", "audit", base}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty after produced report", stderr.String())
	}
	want := fmt.Sprintf("status: failed\n\ncontext:\n  scope: 1 commit(s) in %s..HEAD\n\nsummary:\n  findings: 1 errors, 0 warnings\n\nfindings:\n  errors:\n    conventional-commits | %s | subject is not Conventional Commits (type(scope)?: subject)\n", base, commit[:8])
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}
