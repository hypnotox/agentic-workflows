package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestRangePairUniversesUsesEachFirstParentSnapshotBoundary changes both the
// lock boundary and ADR-0002 bytes across one commit. Each ADR is deliberately
// invalid under the other side's lock, so a parsed format-transition finding
// proves audit loading derived boundaries independently from both snapshots.
func TestRangePairUniversesUsesEachFirstParentSnapshotBoundary(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files["docs/decisions/0002-boundary.md"] = boundaryADR(adr.V1FormatMarker, "V1 side")
	gitfixture.Stage(t, repo, files)
	base := gitfixture.Commit(t, repo, "v1 boundary", nil)
	gitfixture.Stage(t, repo, map[string]string{
		".awf/awf.lock":                   lockJSON(t, &manifest.Lock{AWFVersion: "0.18.0", SchemaVersion: 14}),
		"docs/decisions/0002-boundary.md": boundaryADR(adr.V2FormatMarker, "V2 side"),
	})
	head := gitfixture.Commit(t, repo, "v2 boundary", nil)
	p := openStaged(t, dir)
	findings, commits, err := auditProject(p, testContext(t), base, head)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if commits != 1 {
		t.Fatalf("audit commits = %d, want 1", commits)
	}
	var transitions []audit.Finding
	for _, finding := range findings {
		if finding.Rule == "current-state-transition" {
			transitions = append(transitions, finding)
		}
	}
	if len(transitions) != 1 || transitions[0].Severity != severity.Error || !strings.Contains(transitions[0].Detail, "changed governed format") {
		t.Fatalf("audit transitions findings=%#v; want the parsed format-transition error, not a snapshot-load warning", findings)
	}
}

// TestAuditTransitionsClean retains the live V1-only-lock case: its only
// mutation is a bootstrap claim first appearing below the cutoff, so no add
// operation is owed.
func TestAuditTransitionsClean(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "cutover", nil)
	p := openStaged(t, dir)

	findings, _, err := auditProject(p, testContext(t), base, "HEAD")
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	for _, finding := range findings {
		if finding.Rule == "current-state-transition" {
			t.Fatalf("transition findings = %#v; want none", findings)
		}
	}
}

// TestAuditTransitionsFinding reports the unmatched claim removal at the commit
// that removed it while leaving the bootstrap-add commit clean.
func TestAuditTransitionsFinding(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "cutover", nil)
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	gitfixture.Commit(t, repo, "drop claim", nil)
	p := openStaged(t, dir)

	findings, _, err := auditProject(p, testContext(t), base, "HEAD")
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	var errs []audit.Finding
	for _, f := range findings {
		if f.Rule != "current-state-transition" {
			continue
		}
		if f.Severity != severity.Error {
			t.Fatalf("unexpected transition finding %#v", f)
		}
		errs = append(errs, f)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Detail, "was removed with no ADR remove operation") {
		t.Fatalf("findings = %#v; want one unmatched-removal error", findings)
	}
}

// TestAuditTransitionsWarning warns, rather than aborting, when a commit's
// current-state universes cannot load (a malformed staged ADR).
func TestAuditTransitionsWarning(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, map[string]string{".awf/config.yaml": csYAML, ".awf/domains/alpha.yaml": "paths:\n  - internal/**\n"})
	gitfixture.Commit(t, repo, "config", nil)
	gitfixture.Stage(t, repo, map[string]string{"docs/decisions/0001-bad.md": "---\nstatus: [unterminated\n---\n# X\n"})
	gitfixture.Commit(t, repo, "bad adr", nil)
	p := openStaged(t, dir)

	findings, _, err := auditProject(p, testContext(t), base, "HEAD")
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	var warned bool
	for _, f := range findings {
		if f.Severity == severity.Warn && strings.Contains(f.Detail, "could not load the current-state universes") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("findings = %#v; want a load warning", findings)
	}
}

// TestAuditTransitionsCollectError propagates an unresolvable range.
func TestAuditTransitionsCollectError(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	testsupport.WriteAwfConfig(t, dir, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p := openStaged(t, dir)
	if _, _, err := auditProject(p, testContext(t), "does-not-exist", "HEAD"); err == nil {
		t.Fatal("expected an unresolvable-range error")
	}
}

// TestAuditTransitionsMerge proves first-parent merge integration: a claim
// removed on a branch is validated at the merge commit against its first parent,
// so the merge's transition pair reports the unmatched removal.
func TestAuditTransitionsMerge(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	b0 := gitfixture.Commit(t, repo, "mainline", nil)
	// Branch work: remove the claim.
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	f1 := gitfixture.Commit(t, repo, "remove claim", nil)
	// Merge f1 into mainline: first parent b0 (claim present), tree = f1 (claim removed).
	merge := gitfixture.Merge(t, repo, "merge", b0, f1)
	p := openStaged(t, dir)

	findings, _, err := auditProject(p, testContext(t), b0, merge)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	var mergeReported bool
	for _, f := range findings {
		if f.Subject == "merge" && f.Severity == severity.Error && strings.Contains(f.Detail, "was removed with no ADR remove operation") {
			mergeReported = true
		}
	}
	if !mergeReported {
		t.Fatalf("findings = %#v; want the merge commit's first-parent removal reported", findings)
	}
}

// TestAuditIncludesTransitions proves p.Audit appends the transition findings to
// the range-rule findings.
func TestAuditIncludesTransitions(t *testing.T) {
	t.Parallel()
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	base := gitfixture.Commit(t, repo, "base", map[string]string{"README.md": "base\n"})
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "cutover", nil)
	gitfixture.Stage(t, repo, map[string]string{".awf/topics/parts/alpha/one/current-state.md": "Intro only.\n\n## Claims\n"})
	gitfixture.Commit(t, repo, "drop claim", nil)
	p := openStaged(t, dir)
	writeLock(t, p, attestedLock())

	findings, _, err := auditProject(p, testContext(t), base, "HEAD")
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	var found bool
	for _, f := range findings {
		if f.Rule == "current-state-transition" && strings.Contains(f.Detail, "was removed with no ADR remove operation") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Audit findings = %#v; want the transition finding appended", findings)
	}
}
