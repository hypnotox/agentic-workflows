package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// orphanedByPath returns the "orphaned" drift entries keyed by path.
func orphanedByPath(drift []manifest.Drift) map[string]string {
	out := map[string]string{}
	for _, d := range drift {
		if d.Kind == "orphaned" {
			out[d.Path] = d.Detail
		}
	}
	return out
}

const unclaimedDetail = "unclaimed file or directory: not part of the .awf config tree; delete it or move it out"
const bakDetail = "stale awf-bak backup: review and delete"

func checkDrift(t *testing.T, root string) []manifest.Drift {
	t.Helper()
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncProject(p); err != nil {
		t.Fatal(err)
	}
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	return drift
}

func TestSweepClaimsOnlyUpgradeJournalAfterCutover(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{"current-state-migration.yaml": "version: 1\ninvariantApprovals: []\n", "current-state-upgrade.journal": "{}\n"})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := sweepConfigTree(renderInputsForTest(p), files, mustDeriveTopics(t, p))
	if err != nil {
		t.Fatal(err)
	}
	orphans := orphanedByPath(drift)
	if orphans[".awf/current-state-migration.yaml"] != unclaimedDetail {
		t.Fatalf("reintroduced migration approval was not unclaimed: %#v", orphans)
	}
	if orphans[".awf/current-state-upgrade.journal"] != "" {
		t.Fatalf("upgrade journal must remain transaction-owned: %#v", orphans)
	}
}

// invariant: rendering/sync-and-drift:closed-config-tree (TestSweepFlagsUnclaimedEntries)
func TestSweepFlagsUnclaimedEntries(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"notes.md":                         "stray\n",
		"local/context-spills.log":         "repository-private state stays outside configuration\n",
		"scratch/a.txt":                    "stray\n",
		"scratch/b/c.txt":                  "stray\n",
		"skills/readme.txt":                "stray\n",
		"skills/parts/debugging/stray.txt": "stray\n",
		"skills/parts/debugging/bogus.md":  "undeclared section\n",
		"efforts/anything.md":              "session scratch - exempt\n",
		"efforts/deep/file.awf-bak":        "exempt too\n",
		"config.yaml.awf-bak.2":            "numbered backup\n",
		"hooks/pre-commit.sh.awf-bak":      "backup beside a claimed unit\n",
		"skills/unknown.yaml":              "data: {}\n", // unknown catalog artifact
		"skills/parts/orphan-target/x.md":  "stray\n",    // unknown catalog artifact
		"parts/bogus-kind/x.md":            "unknown singleton\n",
		"parts/workflow/bogus.md":          "undeclared singleton section\n",
	})
	// hooks enabled so .awf/hooks/*.sh are claimed render units; gateCmd and
	// the runner keep the enabled hooks command-wiring valid (ADR-0156).
	testsupport.WriteFile(t, configPath(root), "prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: make gate\n")
	drift := checkDrift(t, root)
	got := orphanedByPath(drift)

	want := map[string]string{
		".awf/notes.md":                         unclaimedDetail,
		".awf/local":                            unclaimedDetail,
		".awf/scratch":                          unclaimedDetail,
		".awf/skills/readme.txt":                unclaimedDetail,
		".awf/skills/parts/debugging/stray.txt": unclaimedDetail,
		".awf/skills/parts/debugging/bogus.md":  "convention part for a section not in the target's declared set",
		".awf/skills/unknown.yaml":              "sidecar for an artifact not in the catalog",
		// invariant: rendering/sync-and-drift:awf-bak-flagged (.awf/config.yaml.awf-bak.2)
		".awf/config.yaml.awf-bak.2":       bakDetail,
		".awf/hooks/pre-commit.sh.awf-bak": bakDetail,
		".awf/skills/parts/orphan-target":  "convention parts for an artifact not in the catalog",
		".awf/parts/bogus-kind":            "convention parts for an unknown singleton kind",
		".awf/parts/workflow/bogus.md":     "convention part for a section not in the singleton's declared set",
	}
	for path, detail := range want {
		if got[path] != detail {
			t.Errorf("%s: got detail %q, want %q", path, got[path], detail)
		}
	}
	for path := range got {
		if _, ok := want[path]; !ok {
			t.Errorf("unexpected orphaned entry %s (%q)", path, got[path])
		}
	}
}

// Sweep never recurses into an owned resident root: every descendant is dynamic
// local authority, including one shaped like a nested adopter or a stale backup.
func TestSweepExemptsResidentRoots(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"efforts/e/memory.md":                                "scratch\n",
		"efforts/e/deep/file.awf-bak":                        "scratch\n",
		"efforts/e/sessions/s":                               "resident\n",
		"efforts/e/.awf/config.yaml":                         "adversarial\n",
		"worktrees/w/anything.md":                            "scratch\n",
		"effort-archive/id-e/deep/file.awf-bak":              "archive\n",
		"effort-archive/id-e/nested/.awf/config.yaml":        "adversarial\n",
		"effort-archive/id-e/nested/internal/adversarial.go": "not go\n",
	})
	if got := orphanedByPath(checkDrift(t, root)); len(got) != 0 {
		t.Fatalf("dynamic resident trees must be exempt, got %#v", got)
	}
}

// The ADR-0068 effective-catalog pin: a synthesized local artifact's declared
// content section resolves against the effective catalog, so its part is
// claimed - a future declaredSections change to catalog.Standard would
// otherwise silently flag every local artifact's parts.
func TestSweepBaselineClean(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars:\n  gateCmd: make gate\nbootstrap:\n  enabled: true\n", nil)
	if got := orphanedByPath(checkDrift(t, root)); len(got) != 0 {
		t.Fatalf("a hygienic tree with all render units enabled must sweep clean, got %#v", got)
	}
}

func TestSweepClaimsOnlyTheTopicCurrentStatePart(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/topics/parts/rendering/contracts/notes.md"), "stray\n")
	got := orphanedByPath(checkDrift(t, root))
	if got[".awf/topics/parts/rendering/contracts/notes.md"] != unclaimedDetail {
		t.Fatalf("topic sweep = %#v", got)
	}
}

func TestSweepClaimsConfiguredEmptyTopicDomainDirectories(t *testing.T) {
	root := topicProject(t)
	for _, rel := range []string{".awf/topics/metadata/rendering", ".awf/topics/parts/rendering"} {
		if err := os.MkdirAll(filepath.Join(root, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{".awf/topics/metadata/unconfigured", ".awf/topics/parts/unconfigured"} {
		if err := os.MkdirAll(filepath.Join(root, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := orphanedByPath(checkDrift(t, root))
	for _, rel := range []string{".awf/topics/metadata/rendering", ".awf/topics/parts/rendering"} {
		if detail, exists := got[rel]; exists {
			t.Errorf("configured domain directory %s was rejected: %q", rel, detail)
		}
	}
	for _, rel := range []string{".awf/topics/metadata/unconfigured", ".awf/topics/parts/unconfigured"} {
		if got[rel] != unclaimedDetail {
			t.Errorf("unconfigured domain directory %s = %q", rel, got[rel])
		}
	}
}
