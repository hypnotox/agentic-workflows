package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

// TestCheckStagedMergeUsesTheAggregateContract proves the MERGE_HEAD wiring end
// to end with the remaining semantic distinction: same-claim chains are refused
// as authored transactions and folded only for a merge.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestCheckStagedMergeUsesTheAggregateContract)
func TestCheckStagedMergeUsesTheAggregateContract(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = lockJSON(t, &manifest.Lock{AWFVersion: "0.20.0", SchemaVersion: 15, Files: map[string]manifest.Entry{}})

	ops := "- add `alpha/one:c`\n- add `alpha/one:d`"
	corrected := "- 2026-07-21: Implementing; content-sha256: %s\n" +
		"- 2026-07-21: Applied; operations: add `alpha/one:c`\n" +
		"- 2026-07-22: Reapplied; operations: add `alpha/one:c`"

	// ADR numbering must stay contiguous, so 0002 exists purely as a filler.
	files["docs/decisions/0002-filler.md"] = publicV2ADR(t, "0002", "Filler", "Proposed", "None.", "")
	files["docs/decisions/0003-corrected.md"] = publicV2ADR(t, "0003", "Corrected", "Proposed", ops, "")
	gitfixture.Stage(t, repo, files)
	gitfixture.Commit(t, repo, "feat(invariants): propose a corrected add", nil)

	gitfixture.Stage(t, repo, map[string]string{
		"docs/decisions/0003-corrected.md":             publicV2ADR(t, "0003", "Corrected", "Implementing", ops, corrected),
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("c"),
	})

	p := openStaged(t, dir)
	authored, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if got := strings.Join(currentStateFindings(authored), "\n"); !strings.Contains(got, "target of more than one operation") {
		t.Fatalf("an authored commit must refuse the same-claim chain, got:\n%s", got)
	}

	// The same index, now carrying merge provenance.
	if err := os.WriteFile(filepath.Join(dir, ".git", "MERGE_HEAD"), []byte("deadbeef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	merged, err := checkStagedProject(p, testContext(t))
	if err != nil {
		t.Fatalf("CheckStaged during a merge: %v", err)
	}
	if got := currentStateFindings(merged); len(got) != 0 {
		t.Fatalf("a merge must accept the same-claim aggregate, got: %v", got)
	}
}

// TestAuditTransitionsMergeUsesTheAggregateContract proves the audit caller maps
// its IsMerge onto the aggregate contract. Without it the mapping is executed by
// the existing merge test but never asserted, so awf audit could silently regress
// to refusing every legitimate merge.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate (TestAuditTransitionsMergeUsesTheAggregateContract)
func TestAuditTransitionsMergeUsesTheAggregateContract(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = lockJSON(t, &manifest.Lock{AWFVersion: "0.20.0", SchemaVersion: 15, Files: map[string]manifest.Entry{}})
	ops := "- add `alpha/one:a`\n- add `alpha/one:b`\n- add `alpha/one:c`\n- add `alpha/one:d`"
	oneBatch := "- 2026-07-21: Implementing; content-sha256: %s\n" +
		"- 2026-07-21: Applied; operations: add `alpha/one:a`"
	threeBatches := oneBatch +
		"\n- 2026-07-22: Applied; operations: add `alpha/one:b`" +
		"\n- 2026-07-23: Applied; operations: add `alpha/one:c`"

	files["docs/decisions/0002-filler.md"] = publicV2ADR(t, "0002", "Filler", "Proposed", "None.", "")
	files["docs/decisions/0003-incremental.md"] = publicV2ADR(t, "0003", "Incremental", "Implementing", ops, oneBatch)
	files[".awf/topics/parts/alpha/one/current-state.md"] = publicTopicClaims("a")
	gitfixture.Stage(t, repo, files)
	b0 := gitfixture.Commit(t, repo, "mainline", nil)

	// Branch work: two further batches, which an authored commit would refuse.
	gitfixture.Stage(t, repo, map[string]string{
		"docs/decisions/0003-incremental.md":           publicV2ADR(t, "0003", "Incremental", "Implementing", ops, threeBatches),
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("a", "b", "c"),
	})
	f1 := gitfixture.Commit(t, repo, "apply two more batches", nil)
	merge := gitfixture.Merge(t, repo, "merge", b0, f1)

	p := openStaged(t, dir)
	findings, _, err := auditProject(p, testContext(t), b0, merge)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	for _, f := range findings {
		if f.Subject == "merge" {
			t.Fatalf("the merge commit must carry no transition finding, got: %#v", f)
		}
	}
}

// TestCheckStagedToleratesUnresolvableControlRoot pins the degrade: go-git's
// index read follows a symlinked .git, so the staged check must keep working
// there. It falls back to the stricter authored-commit contract rather than
// failing, which is exactly what it did before merge detection existed.
// It carries no claim marker: detection selects which contract applies and sits
// outside the claim's scope (ADR-0182 item 11), and a touches-state marker is
// only legal inside the topic's own selectors, which internal/project is not in.
func TestCheckStagedToleratesUnresolvableControlRoot(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Stage(t, repo, stagedHeadFiles())
	gitfixture.Commit(t, repo, "head", nil)
	p := openStaged(t, dir)
	if _, err := checkStagedProject(p, testContext(t)); err != nil {
		t.Fatalf("baseline CheckStaged: %v", err)
	}

	// Replace .git with a symlink to its own contents, which the control-root
	// rules refuse while go-git still reads the index through it.
	gitdir := filepath.Join(filepath.Dir(dir), "moved-git")
	if err := os.Rename(filepath.Join(dir, ".git"), gitdir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(gitdir, filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := checkStagedProject(p, testContext(t)); err != nil {
		t.Fatalf("a symlinked control root must not fail the staged check: %v", err)
	}
}
