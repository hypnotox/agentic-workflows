package effort

import (
	"bytes"
	"os"
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

	root := initEffortRepo(t)
	service := openTestService(t, root, func(d *Dependencies) {
		d.Clock = func() time.Time { return time.Date(2026, 8, 2, 13, 0, 0, 0, time.UTC) }
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "coherent-effort", Title: "Coherent effort"}); err != nil {
		t.Fatal(err)
	}
	path := service.paths.memoryFile("coherent-effort")
	body := []byte("## Brief\r\n\r\nbody bytes stay exact\r\n")
	canonical := []byte("---\neffort: coherent-effort\nphase: old phase\nnext: old next\nupdated: 2026-08-01T12:00:00Z\n---\n")
	if err := os.WriteFile(path, append(canonical, body...), 0o600); err != nil {
		t.Fatal(err)
	}
	phase := "new phase"
	if err := service.UpdateMemory("coherent-effort", MemoryUpdate{Phase: &phase}); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(updated, []byte("---\neffort: coherent-effort\nphase: new phase\nnext: old next\nupdated: \"2026-08-02T13:00:00Z\"\n---\n")) || !bytes.HasSuffix(updated, body) {
		t.Fatalf("selective canonical update err=%v memory=%q", err, updated)
	}
	legacyBody := []byte("## Brief\r\n\r\nlegacy body stays exact\r\n")
	legacy := append([]byte("Effort: coherent-effort\nPhase: legacy phase\nNext: legacy next\nUpdated: Not yet updated.\n\n"), legacyBody...)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	next := "new next"
	if err := service.UpdateMemory("coherent-effort", MemoryUpdate{Next: &next}); err != nil {
		t.Fatal(err)
	}
	updated, err = os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(updated, []byte("---\neffort: coherent-effort\nphase: legacy phase\nnext: new next\nupdated: \"2026-08-02T13:00:00Z\"\n---\n")) || !bytes.HasSuffix(updated, legacyBody) {
		t.Fatalf("exact legacy migration err=%v memory=%q", err, updated)
	}
	invalid := append([]byte("---\neffort: coherent-effort\nphase: \"\"\nnext: \"\"\nupdated: not-a-time\n---\n"), body...)
	if err := os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateMemory("coherent-effort", MemoryUpdate{Phase: &phase}); err == nil || !strings.Contains(err.Error(), "./awf effort memory update coherent-effort --phase <replacement-phase> --next <replacement-next>") {
		t.Fatalf("partial invalid-metadata repair = %v", err)
	}
	if err := service.UpdateMemory("coherent-effort", MemoryUpdate{Phase: &phase, Next: &next}); err != nil {
		t.Fatal(err)
	}
	updated, err = os.ReadFile(path)
	if err != nil || !bytes.HasSuffix(updated, body) {
		t.Fatalf("safe repair err=%v memory=%q", err, updated)
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
