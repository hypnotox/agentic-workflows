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
// to end: one staged pair, refused as an authored commit and accepted as a merge.
// Without this the aggregate contract could be correct in isolation and never
// reach the command that needs it.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestCheckStagedMergeUsesTheAggregateContract(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = lockJSON(t, &manifest.Lock{AWFVersion: "0.20.0", SchemaVersion: 15, Files: map[string]manifest.Entry{}, ADRFormatV1From: 2, ADRFormatV2From: 2, LegacyADRGaps: []int{}})

	// A fourth declared operation stays unapplied so the record is legally
	// Implementing at both ends: that status requires applied AND remaining work.
	ops := "- add `alpha/one:a`\n- add `alpha/one:b`\n- add `alpha/one:c`\n- add `alpha/one:d`"
	oneBatch := "- 2026-07-21: Implementing; content-sha256: %s\n" +
		"- 2026-07-21: Applied; state-sequence: 1; operations: add `alpha/one:a`"
	threeBatches := oneBatch +
		"\n- 2026-07-22: Applied; state-sequence: 2; operations: add `alpha/one:b`" +
		"\n- 2026-07-23: Applied; state-sequence: 3; operations: add `alpha/one:c`"

	// ADR numbering must stay contiguous, so 0002 exists purely as a filler.
	files["docs/decisions/0002-filler.md"] = publicV2ADR(t, "0002", "Filler", "Proposed", "None.", "")
	files["docs/decisions/0003-incremental.md"] = publicV2ADR(t, "0003", "Incremental", "Implementing", ops, oneBatch)
	files[".awf/topics/parts/alpha/one/current-state.md"] = publicTopicClaims("a")
	gitfixture.Stage(t, repo, dir, files)
	gitfixture.Commit(t, repo, dir, "feat(invariants): apply the first batch", nil)

	// The index carries the two further batches the branch applied.
	gitfixture.Stage(t, repo, dir, map[string]string{
		"docs/decisions/0003-incremental.md":           publicV2ADR(t, "0003", "Incremental", "Implementing", ops, threeBatches),
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("a", "b", "c"),
	})

	p := openStaged(t, dir)
	authored, err := p.CheckStaged()
	if err != nil {
		t.Fatalf("CheckStaged: %v", err)
	}
	if got := strings.Join(authored.Findings(), "\n"); !strings.Contains(got, "at most one new batch") {
		t.Fatalf("an authored commit must refuse the extra batches, got:\n%s", got)
	}

	// The same index, now carrying merge provenance.
	if err := os.WriteFile(filepath.Join(dir, ".git", "MERGE_HEAD"), []byte("deadbeef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	merged, err := p.CheckStaged()
	if err != nil {
		t.Fatalf("CheckStaged during a merge: %v", err)
	}
	if got := merged.Findings(); len(got) != 0 {
		t.Fatalf("a merge must accept the aggregate, got: %v", got)
	}
}

// TestAuditTransitionsMergeUsesTheAggregateContract proves the audit caller maps
// its IsMerge onto the aggregate contract. Without it the mapping is executed by
// the existing merge test but never asserted, so awf audit could silently regress
// to refusing every legitimate merge.
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestAuditTransitionsMergeUsesTheAggregateContract(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	files := stagedHeadFiles()
	files[".awf/awf.lock"] = lockJSON(t, &manifest.Lock{AWFVersion: "0.20.0", SchemaVersion: 15, Files: map[string]manifest.Entry{}, ADRFormatV1From: 2, ADRFormatV2From: 2, LegacyADRGaps: []int{}})
	ops := "- add `alpha/one:a`\n- add `alpha/one:b`\n- add `alpha/one:c`\n- add `alpha/one:d`"
	oneBatch := "- 2026-07-21: Implementing; content-sha256: %s\n" +
		"- 2026-07-21: Applied; state-sequence: 1; operations: add `alpha/one:a`"
	threeBatches := oneBatch +
		"\n- 2026-07-22: Applied; state-sequence: 2; operations: add `alpha/one:b`" +
		"\n- 2026-07-23: Applied; state-sequence: 3; operations: add `alpha/one:c`"

	files["docs/decisions/0002-filler.md"] = publicV2ADR(t, "0002", "Filler", "Proposed", "None.", "")
	files["docs/decisions/0003-incremental.md"] = publicV2ADR(t, "0003", "Incremental", "Implementing", ops, oneBatch)
	files[".awf/topics/parts/alpha/one/current-state.md"] = publicTopicClaims("a")
	gitfixture.Stage(t, repo, dir, files)
	b0 := gitfixture.Commit(t, repo, dir, "mainline", nil)

	// Branch work: two further batches, which an authored commit would refuse.
	gitfixture.Stage(t, repo, dir, map[string]string{
		"docs/decisions/0003-incremental.md":           publicV2ADR(t, "0003", "Incremental", "Implementing", ops, threeBatches),
		".awf/topics/parts/alpha/one/current-state.md": publicTopicClaims("a", "b", "c"),
	})
	f1 := gitfixture.Commit(t, repo, dir, "apply two more batches", nil)
	merge := gitfixture.Merge(t, repo, "merge", b0, f1)

	p := openStaged(t, dir)
	findings, err := p.auditTransitions(b0.String(), merge.String())
	if err != nil {
		t.Fatalf("auditTransitions: %v", err)
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
// invariant: invariants/current-state-authority:merge-transition-ordered-aggregate
func TestCheckStagedToleratesUnresolvableControlRoot(t *testing.T) {
	repo, dir := gitfixture.InitRepo(t)
	gitfixture.Stage(t, repo, dir, stagedHeadFiles())
	gitfixture.Commit(t, repo, dir, "head", nil)
	p := openStaged(t, dir)
	if _, err := p.CheckStaged(); err != nil {
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
	if _, err := p.CheckStaged(); err != nil {
		t.Fatalf("a symlinked control root must not fail the staged check: %v", err)
	}
}
