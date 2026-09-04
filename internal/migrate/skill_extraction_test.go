package migrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestSkillExtractionMigrationPreservesRetainedAuthoredSources(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 51)
	oldSidecar := filepath.Join(root, ".awf/skills/effort-workflow.yaml")
	oldPart := filepath.Join(root, ".awf/skills/parts/current-state/claims.md")
	testsupport.WriteFile(t, oldSidecar, "data:\n  note: custom\n")
	testsupport.WriteFile(t, oldPart, "Custom claims.\n")
	if err := os.Chmod(oldSidecar, 0o640); err != nil {
		t.Fatal(err)
	}

	before := snapshot(t, root)
	applied, changes, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{skillExtractionMigration, workflowSurfaceMigration}) || len(changes) < 2 {
		t.Fatalf("Build() = applied=%v changes=%#v", applied, changes)
	}
	want := map[string]struct {
		content string
		mode    os.FileMode
		remove  bool
	}{
		".awf/skills/awf-effort.yaml":               {content: "data:\n  note: custom\n", mode: 0o640},
		".awf/skills/effort-workflow.yaml":          {remove: true},
		".awf/skills/parts/awf-topics/claims.md":    {content: "Custom claims.\n", mode: 0o644},
		".awf/skills/parts/current-state/claims.md": {remove: true},
		".awf/skills/parts/current-state":           {remove: true},
	}
	if len(mutations) != len(want) {
		t.Fatalf("mutations = %#v", mutations)
	}
	for _, mutation := range mutations {
		expected, ok := want[mutation.Path]
		if !ok || string(mutation.Content) != expected.content || mutation.Mode != expected.mode || mutation.Remove != expected.remove {
			t.Fatalf("mutation %s = %#v, want %#v", mutation.Path, mutation, expected)
		}
	}
	assertSnapshot(t, root, before)
}

func TestSkillExtractionMigrationCollapsesEquivalentAndRefusesConflictingRetainedTargets(t *testing.T) {
	t.Run("equivalent", func(t *testing.T) {
		root := t.TempDir()
		writeLock(t, root, 51)
		for _, name := range []string{"effort-workflow", "awf-effort"} {
			testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/"+name+".yaml"), "data: {}\n")
		}
		_, _, mutations, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if len(mutations) != 1 || mutations[0].Path != ".awf/skills/effort-workflow.yaml" || !mutations[0].Remove {
			t.Fatalf("mutations = %#v", mutations)
		}
	})

	for _, tc := range []struct {
		name    string
		oldMode os.FileMode
		newMode os.FileMode
		newBody string
	}{
		{name: "content", oldMode: 0o644, newMode: 0o644, newBody: "data: {new: true}\n"},
		{name: "mode", oldMode: 0o640, newMode: 0o644, newBody: "data: {}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeLock(t, root, 51)
			oldPath := filepath.Join(root, ".awf/skills/decision-records.yaml")
			newPath := filepath.Join(root, ".awf/skills/awf-decisions.yaml")
			testsupport.WriteFile(t, oldPath, "data: {}\n")
			testsupport.WriteFile(t, newPath, tc.newBody)
			if err := os.Chmod(oldPath, tc.oldMode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(newPath, tc.newMode); err != nil {
				t.Fatal(err)
			}
			before := snapshot(t, root)
			_, changes, mutations, err := Build(context.Background(), root)
			if err == nil || !strings.Contains(err.Error(), "already exists with different content or mode") {
				t.Fatalf("Build() error = %v", err)
			}
			if len(changes) != 0 || len(mutations) != 0 {
				t.Fatalf("unsafe partial plan: changes=%#v mutations=%#v", changes, mutations)
			}
			assertSnapshot(t, root, before)
		})
	}
}

func TestSkillExtractionMigrationBacksUpRemovedAuthoredSourcesWithoutOverwrite(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rel     string
		content string
	}{
		{name: "skill sidecar", rel: ".awf/skills/debugging.yaml", content: "data:\n  note: custom debugging\n"},
		{name: "skill section part", rel: ".awf/skills/parts/context/explore.md", content: "custom generic exploration\n"},
		{name: "role sidecar", rel: ".awf/agents/reviewer.yaml", content: "data:\n  note: custom reviewer\n"},
		{name: "role section part", rel: ".awf/agents/parts/implementer/work.md", content: "custom implementation role\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeLock(t, root, 51)
			source := filepath.Join(root, filepath.FromSlash(tc.rel))
			testsupport.WriteFile(t, source, tc.content)
			if err := os.Chmod(source, 0o640); err != nil {
				t.Fatal(err)
			}
			testsupport.WriteFile(t, source+".awf-bak", "older recovery\n")
			testsupport.WriteFile(t, source+".awf-bak.1", "another recovery\n")

			_, _, mutations, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if len(mutations) != 2 {
				t.Fatalf("mutations = %#v", mutations)
			}
			var replacement, removal FileMutation
			for _, mutation := range mutations {
				if mutation.Remove {
					removal = mutation
				} else {
					replacement = mutation
				}
			}
			if replacement.Path != tc.rel+".awf-bak.2" || string(replacement.Content) != tc.content || replacement.Mode != 0o640 || replacement.Expected.Present {
				t.Fatalf("replacement = %#v", replacement)
			}
			if removal.Path != tc.rel || !removal.Expected.Present || string(removal.Expected.Content) != tc.content || removal.Expected.Mode != 0o640 {
				t.Fatalf("removal = %#v", removal)
			}
		})
	}
}

// invariant: config/migrations-and-locks:migration-preimage-safe (TestSkillExtractionMigrationRejectsFinalSymlinkSource)
func TestSkillExtractionMigrationRejectsFinalSymlinkSource(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 51)
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "authored.yaml"), "data:\n  note: custom\n")
	link := filepath.Join(root, ".awf", "skills", "debugging.yaml")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../authored.yaml", link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, _, mutations, err := Build(context.Background(), root); err == nil || !strings.Contains(err.Error(), "final symlink") || len(mutations) != 0 {
		t.Fatalf("Build() mutations=%#v error=%v", mutations, err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("source symlink changed: info=%v err=%v", info, err)
	}
}

func TestSkillExtractionMigrationRejectsNonRegularSource(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 51)
	directory := filepath.Join(root, ".awf", "skills", "debugging.yaml")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, mutations, err := Build(context.Background(), root); err == nil || !strings.Contains(err.Error(), "not a regular file") || len(mutations) != 0 {
		t.Fatalf("Build() mutations=%#v error=%v", mutations, err)
	}
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		t.Fatalf("source directory changed: info=%v err=%v", info, err)
	}
}

func TestSkillExtractionMigrationInventoryAndCancellation(t *testing.T) {
	gotRetained := make([]string, 0, len(retainedSkillSources))
	for _, source := range retainedSkillSources {
		gotRetained = append(gotRetained, source.old+">"+source.new+":"+strings.Join(source.sections, ","))
	}
	wantRetained := []string{
		"effort-workflow>awf-effort:continuity-and-resident,execution-and-checkpoints,integration-and-recovery,close",
		"current-state>awf-topics:claims",
		"decision-records>awf-decisions:format",
		"using-awf>awf-maintenance:generated-documents,upgrades",
	}
	if !slices.Equal(gotRetained, wantRetained) {
		t.Fatalf("retained inventory = %v", gotRetained)
	}
	gotRemoved := make([]string, 0, len(removedSkillSources))
	for _, source := range removedSkillSources {
		gotRemoved = append(gotRemoved, source.name+":"+strings.Join(source.sections, ","))
	}
	wantRemoved := []string{
		"context:orient,explore,challenge", "brainstorming:procedure", "debugging:oracle-and-handoff",
		"implementing:ownership,procedure,review-handoff", "planning:shape", "reviewing:brief", "refactor-scope:inventory",
	}
	if !slices.Equal(gotRemoved, wantRemoved) {
		t.Fatalf("removed skill inventory = %v", gotRemoved)
	}
	gotAgents := make([]string, 0, len(removedAgentSources))
	for _, source := range removedAgentSources {
		gotAgents = append(gotAgents, source.name+":"+strings.Join(source.sections, ","))
	}
	wantAgents := []string{
		"explorer:scope,report", "premise-checker:procedure,report",
		"implementer:authority,work,receipt", "reviewer:review,report",
	}
	if !slices.Equal(gotAgents, wantAgents) {
		t.Fatalf("removed role inventory = %v", gotAgents)
	}

	root := t.TempDir()
	writeLock(t, root, 51)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := Build(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("Build() error = %v, want cancellation", err)
	}
}
