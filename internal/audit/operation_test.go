package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

func configuredAuditRepository(t *testing.T) (gitfixture.Fixture, string) {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	base := gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{"main.go": "package x\n"})
	return repo, base
}

// invariant: tooling/audit-commands:audit-reports-evaluated-scope (TestRunConfiguredBuildsReportAndClassifiesOutcome)
func TestRunConfiguredBuildsReportAndClassifiesOutcome(t *testing.T) {
	repo, base := configuredAuditRepository(t)
	gitfixture.Commit(t, repo, "feat(awf): bump a dependency", map[string]string{"go.mod": "module x\n"})

	outcome, err := RunConfigured(testContext(t), repo.Root(), &config.Config{}, base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Failed || outcome.Commits != 1 {
		t.Fatalf("outcome = %#v, want one warning-only commit", outcome)
	}
	document, err := outcome.Report.Document()
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "status: warnings\n") || !strings.Contains(rendered.String(), "scope: 1 commit(s) in "+base+"..HEAD") {
		t.Fatalf("report = %q", rendered.String())
	}
}

// invariant: tooling/audit-commands:audit-empty-range-announced (TestRunConfiguredClassifiesErrorAndEmptyRanges)
func TestRunConfiguredClassifiesErrorAndEmptyRanges(t *testing.T) {
	repo, base := configuredAuditRepository(t)
	gitfixture.Commit(t, repo, "not a conventional commit subject", map[string]string{"main.go": "package x\nvar y int\n"})

	failed, err := RunConfigured(testContext(t), repo.Root(), &config.Config{}, base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !failed.Failed || failed.Commits != 1 || failed.Report.Status != "failed" {
		t.Fatalf("failed outcome = %#v", failed)
	}

	empty, err := RunConfigured(testContext(t), repo.Root(), &config.Config{}, "HEAD", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if empty.Failed || empty.Commits != 0 || empty.Report.Status != "empty" {
		t.Fatalf("empty outcome = %#v", empty)
	}
}

func TestRunConfiguredUsesLockedGeneratedPaths(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	lock := `{"awfVersion":"0.1.0","schemaVersion":1,"files":{"generated.go":{}}}`
	base := gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{
		".awf/awf.lock": lock,
		"generated.go":  "package x\n",
	})
	gitfixture.Commit(t, repo, "feat(awf): generated update", map[string]string{"generated.go": "package x\n" + strings.Repeat("var n int\n", 500)})

	outcome, err := RunConfigured(testContext(t), repo.Root(), &config.Config{}, base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Failed || outcome.Report.Status != "clean" {
		t.Fatalf("locked generated change = %#v", outcome)
	}
}

func TestRunConfiguredPropagatesInputAndAnalysisFailures(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".awf", "awf.lock"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunConfigured(testContext(t), dir, &config.Config{}, "HEAD", "HEAD"); err == nil {
		t.Fatal("invalid lock accepted")
	}

	repo, _ := configuredAuditRepository(t)
	if _, err := RunConfigured(testContext(t), repo.Root(), &config.Config{}, "does-not-exist", "HEAD"); err == nil {
		t.Fatal("unresolvable range accepted")
	}
}
