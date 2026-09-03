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

// invariant: config/migrations-and-locks:context-skill-source-migration (TestContextSkillMigrationPreservesAuthoredSources)
func TestContextSkillMigrationPreservesAuthoredSources(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 50)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/repository-context.yaml"), "data:\n  note: custom\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/parts/repository-context/explore.md"), "Custom exploration.\n")

	before := snapshot(t, root)
	applied, changes, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{contextSkillMigration}) {
		t.Fatalf("applied = %v", applied)
	}
	if len(changes) != 1 || changes[0].Text != "renamed the repository-context skill to context" {
		t.Fatalf("changes = %#v", changes)
	}
	wantPaths := []string{
		".awf/skills/context.yaml",
		".awf/skills/parts/context/explore.md",
		".awf/skills/parts/repository-context/explore.md",
		".awf/skills/repository-context.yaml",
	}
	gotPaths := make([]string, len(mutations))
	for i, mutation := range mutations {
		gotPaths[i] = mutation.Path
	}
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("mutation paths = %v, want %v", gotPaths, wantPaths)
	}
	if string(mutations[0].Content) != "data:\n  note: custom\n" || mutations[0].Mode != 0o644 || mutations[2].Remove != true || mutations[3].Remove != true {
		t.Fatalf("mutations = %#v", mutations)
	}
	assertSnapshot(t, root, before)
}

func TestContextSkillMigrationHandlesAbsentAndEquivalentTargets(t *testing.T) {
	t.Run("no authored sources", func(t *testing.T) {
		root := t.TempDir()
		writeLock(t, root, 50)
		applied, changes, mutations, err := Build(context.Background(), root)
		if err != nil || !slices.Equal(applied, []string{contextSkillMigration}) || len(changes) != 1 || len(mutations) != 0 {
			t.Fatalf("Build() = applied=%v changes=%v mutations=%v err=%v", applied, changes, mutations, err)
		}
	})

	t.Run("equivalent target", func(t *testing.T) {
		root := t.TempDir()
		writeLock(t, root, 50)
		for _, path := range []string{".awf/skills/repository-context.yaml", ".awf/skills/context.yaml"} {
			testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(path)), "data: {}\n")
		}
		_, _, mutations, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if len(mutations) != 1 || mutations[0].Path != ".awf/skills/repository-context.yaml" || !mutations[0].Remove {
			t.Fatalf("mutations = %#v", mutations)
		}
	})
}

func TestContextSkillMigrationRefusesConflictingTarget(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 50)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/repository-context.yaml"), "data:\n  note: old\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/context.yaml"), "data:\n  note: new\n")
	before := snapshot(t, root)

	_, changes, mutations, err := Build(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "already exists with different content or mode") {
		t.Fatalf("Build() error = %v", err)
	}
	if len(changes) != 0 || len(mutations) != 0 {
		t.Fatalf("reported effects after collision: changes=%#v mutations=%#v", changes, mutations)
	}
	assertSnapshot(t, root, before)
}

func TestContextSkillMigrationHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 50)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := Build(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("Build() error = %v, want cancellation", err)
	}
}

func TestContextSkillMigrationPreservesMode(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 50)
	path := filepath.Join(root, ".awf/skills/repository-context.yaml")
	testsupport.WriteFile(t, path, "data: {}\n")
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	_, _, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 2 || mutations[0].Mode != 0o640 {
		t.Fatalf("mutations = %#v", mutations)
	}
}
