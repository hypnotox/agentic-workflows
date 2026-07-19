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
const localPartsDetail = "convention parts for a local-managed artifact (local: true renders nothing)"

func TestSweepClaimsBridgeTransactionInputs(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\ntargets: [claude]\n", map[string]string{"current-state-migration.yaml": "version: 1\ninvariantApprovals: []\n", "current-state-upgrade.journal": "{}\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.sweepConfigTree(files)
	if err != nil {
		t.Fatal(err)
	}
	orphans := orphanedByPath(drift)
	if orphans[".awf/current-state-migration.yaml"] != "" || orphans[".awf/current-state-upgrade.journal"] != "" {
		t.Fatalf("bridge inputs orphaned: %#v", orphans)
	}
}

// invariant: closed-config-tree
func TestSweepFlagsUnclaimedEntries(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - tdd\nagents: []\n", map[string]string{
		"notes.md":                        "stray\n",
		"scratch/a.txt":                   "stray\n",
		"scratch/b/c.txt":                 "stray\n",
		"skills/readme.txt":               "stray\n",
		"skills/parts/tdd/stray.txt":      "stray\n",
		"skills/parts/tdd/bogus.md":       "undeclared section\n",
		"memory/anything.md":              "session scratch - exempt\n",
		"memory/deep/file.awf-bak":        "exempt too\n",
		"config.yaml.awf-bak.2":           "numbered backup\n",
		"hooks/pre-commit.sh.awf-bak":     "backup beside a claimed unit\n",
		"skills/debugging.yaml":           "data: {}\n", // debugging not enabled
		"skills/parts/orphan-target/x.md": "stray\n",    // orphan-target not enabled
		"parts/bogus-kind/x.md":           "unknown singleton\n",
		"parts/workflow/bogus.md":         "undeclared singleton section\n",
	})
	// hooks enabled so .awf/hooks/*.sh are claimed render units.
	testsupport.WriteFile(t, configPath(root), "prefix: example\nskills:\n  - tdd\nagents: []\nhooks:\n  enabled: true\n")
	drift := checkDrift(t, root)
	got := orphanedByPath(drift)

	want := map[string]string{
		".awf/notes.md":                   unclaimedDetail,
		".awf/scratch":                    unclaimedDetail,
		".awf/skills/readme.txt":          unclaimedDetail,
		".awf/skills/parts/tdd/stray.txt": unclaimedDetail,
		".awf/skills/parts/tdd/bogus.md":  "convention part for a section not in the target's declared set",
		// invariant: awf-bak-flagged
		".awf/config.yaml.awf-bak.2":       bakDetail,
		".awf/hooks/pre-commit.sh.awf-bak": bakDetail,
		".awf/skills/debugging.yaml":       "sidecar for an artifact not in the enable list",
		".awf/skills/parts/orphan-target":  "convention parts for an artifact not in the enable list",
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

func TestSweepExemptsMemory(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - tdd\nagents: []\n", map[string]string{
		"memory/anything.md":       "scratch\n",
		"memory/deep/file.awf-bak": "scratch\n",
	})
	if got := orphanedByPath(checkDrift(t, root)); len(got) != 0 {
		t.Fatalf(".awf/memory/** is exempt session scratch, got %#v", got)
	}
}

func TestSweepFlagsLocalArtifactParts(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - tdd\nagents: []\n", map[string]string{
		"skills/tdd.yaml":            "local: true\n",
		"skills/parts/tdd/notes.md":  "dead weight\n",
		"workflow.yaml":              "local: true\n",
		"parts/workflow/overview.md": "dead weight too\n",
	})
	testsupport.WriteFile(t, filepath.Join(root, ".claude/skills/example-tdd/SKILL.md"),
		"---\nname: example-tdd\ndescription: hand-maintained\n---\nbody\n")
	got := orphanedByPath(checkDrift(t, root))
	for _, path := range []string{".awf/skills/parts/tdd", ".awf/parts/workflow"} {
		if got[path] != localPartsDetail {
			t.Errorf("%s: got %q, want the local-managed detail", path, got[path])
		}
	}
}

// The ADR-0068 effective-catalog pin: a synthesized local artifact's declared
// content section resolves against the effective catalog, so its part is
// claimed - a future declaredSections change to catalog.Standard would
// otherwise silently flag every local artifact's parts.
func TestSweepClaimsSynthesizedLocalParts(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - my-local\nagents: []\n", map[string]string{
		"skills/my-local.yaml":             "data:\n  description: A local skill.\n",
		"skills/parts/my-local/content.md": "Body here.\n",
		"skills/parts/my-local/bogus.md":   "undeclared\n",
	})
	got := orphanedByPath(checkDrift(t, root))
	if detail, ok := got[".awf/skills/parts/my-local/content.md"]; ok {
		t.Errorf("the synthesized content section must be claimed, got %q", detail)
	}
	if got[".awf/skills/parts/my-local/bogus.md"] != "convention part for a section not in the target's declared set" {
		t.Errorf("undeclared local section: got %q", got[".awf/skills/parts/my-local/bogus.md"])
	}
}

func TestSweepBaselineClean(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills:\n  - tdd\nagents: []\nbootstrap:\n  enabled: true\nhooks:\n  enabled: true\n", nil)
	if got := orphanedByPath(checkDrift(t, root)); len(got) != 0 {
		t.Fatalf("a hygienic tree with all render units enabled must sweep clean, got %#v", got)
	}
}

func TestSweepClaimsOnlyTheTopicCurrentStatePart(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "contracts", "Contracts", "paths: [\"internal/**\"]\n")
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
