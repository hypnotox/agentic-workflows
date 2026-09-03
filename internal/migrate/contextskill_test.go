package migrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func buildContextSkillMigrationForTest(t *testing.T, ctx context.Context, root string) ([]Change, []FileMutation, error) {
	t.Helper()
	files, err := filesystem.Open(root)
	if err != nil {
		return nil, nil, err
	}
	defer files.Close() //nolint:errcheck // test helper reports migration behavior
	tree := &ProposedTree{files: files, mutations: map[string]FileMutation{}}
	changes := &Changes{}
	planned, err := renameRepositoryContextSkill(ctx, tree, changes)
	if err != nil {
		return changes.Items(), nil, err
	}
	if err := tree.overlay(planned); err != nil {
		return changes.Items(), nil, err
	}
	return changes.Items(), tree.coalesced(), nil
}

// invariant: config/migrations-and-locks:context-skill-source-migration (TestContextSkillMigrationPreservesAuthoredSources)
func TestContextSkillMigrationPreservesAuthoredSources(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 50)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/repository-context.yaml"), "data:\n  note: custom\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/parts/repository-context/explore.md"), "Custom exploration.\n")

	before := snapshot(t, root)
	changes, mutations, err := buildContextSkillMigrationForTest(t, context.Background(), root)
	if err != nil {
		t.Fatal(err)
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
		changes, mutations, err := buildContextSkillMigrationForTest(t, context.Background(), root)
		if err != nil || len(changes) != 1 || len(mutations) != 0 {
			t.Fatalf("migration = changes=%v mutations=%v err=%v", changes, mutations, err)
		}
	})

	t.Run("equivalent target", func(t *testing.T) {
		root := t.TempDir()
		writeLock(t, root, 50)
		for _, path := range []string{".awf/skills/repository-context.yaml", ".awf/skills/context.yaml"} {
			testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(path)), "data: {}\n")
		}
		_, mutations, err := buildContextSkillMigrationForTest(t, context.Background(), root)
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

	changes, mutations, err := buildContextSkillMigrationForTest(t, context.Background(), root)
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
	if _, _, err := buildContextSkillMigrationForTest(t, ctx, root); !errors.Is(err, context.Canceled) {
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
	_, mutations, err := buildContextSkillMigrationForTest(t, context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 2 || mutations[0].Mode != 0o640 {
		t.Fatalf("mutations = %#v", mutations)
	}
}
