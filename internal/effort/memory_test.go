package effort

import (
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestOwnedMemorySkeletonIsCoherentAndSlugged)
func TestOwnedMemorySkeletonIsCoherentAndSlugged(t *testing.T) {
	const want = "---\n" +
		"effort: coherent-effort\n" +
		"phase: Not started.\n" +
		"next: Record the next concrete action.\n" +
		"updated: \"2026-07-29T12:00:00Z\"\n" +
		"---\n" +
		"## Brief\n\n" +
		"Describe the intended outcome and link the effort's durable artifacts (ADR, plan, worktree, branch) as they come to exist; a resuming session reads this first.\n\n" +
		"## Decision log\n\n" +
		"Append one entry per settled decision: \"- D<n> <date> <phase> (user|autonomous) <decision>. Why: <one line>.\" A user entry whose decision changes scope, design, authority, or previously-approved output adds an indented \"Record:\" block quoting the user's load-bearing wording verbatim; other user entries need none. Reviewers check artifacts against user entries. Full rules: the workflow doc's working-memory section.\n\n" +
		"## Observations\n\n" +
		"Append friction, surprises, near-misses, and recurrences when they happen: \"- <date> <phase> <observation>.\" The retrospective reads this log as its primary input.\n\n" +
		"## Handoff log\n\n" +
		"No handoffs recorded. Append one line per session boundary; a resuming session reads this audit after the Brief.\n"
	if got := string(memorySkeleton("coherent-effort", time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC))); got != want {
		t.Fatalf("memory skeleton mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
	}

	headings := []string{"## Brief", "## Decision log", "## Observations", "## Handoff log"}
	testsupport.WalkRepoSources(t, testsupport.RepoRoot(t), func(rel string, body []byte) {
		if rel == "internal/effort/memory.go" {
			return
		}
		for _, heading := range headings {
			if strings.Contains(string(body), heading) {
				t.Errorf("production source %s depends on memory section heading %q; headings belong only to the skeleton producer", rel, heading)
			}
		}
	})
}
