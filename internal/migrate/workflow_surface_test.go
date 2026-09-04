package migrate

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: config/migrations-and-locks:workflow-surface-source-migration (TestWorkflowSurfaceMigrationPreservesRetiredSourcesAndDoesNotReuseModelPolicy)
func TestWorkflowSurfaceMigrationPreservesRetiredSourcesAndDoesNotReuseModelPolicy(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 52)
	guideSidecar := filepath.Join(root, ".awf/maintainable-code-design.yaml")
	guidePart := filepath.Join(root, ".awf/parts/maintainable-code-design/readability.md")
	oldSection := filepath.Join(root, ".awf/parts/working-with-awf/model-selection.md")
	workingSidecar := filepath.Join(root, ".awf/working-with-awf.yaml")
	testsupport.WriteFile(t, guideSidecar, "data:\n  note: custom doctrine\n")
	testsupport.WriteFile(t, guidePart, "Custom readability doctrine.\n")
	testsupport.WriteFile(t, oldSection, "Obsolete model policy.\n")
	testsupport.WriteFile(t, workingSidecar, "data:\n  retained: yes\nsections:\n  model-selection:\n    drop: true\n  commands:\n    drop: true\n")
	if err := os.Chmod(oldSection, 0o640); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, oldSection+".awf-bak", "prior backup\n")
	oldEmptyDir := filepath.Join(root, ".awf/skills/parts/using-awf")
	if err := os.MkdirAll(oldEmptyDir, 0o750); err != nil {
		t.Fatal(err)
	}

	before := snapshot(t, root)
	applied, changes, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{workflowSurfaceMigration}) {
		t.Fatalf("applied = %v", applied)
	}
	byPath := map[string]FileMutation{}
	for _, mutation := range mutations {
		byPath[mutation.Path] = mutation
	}
	for path, body := range map[string]string{
		".awf/maintainable-code-design.yaml.awf-bak":                 "data:\n  note: custom doctrine\n",
		".awf/parts/maintainable-code-design/readability.md.awf-bak": "Custom readability doctrine.\n",
		".awf/parts/working-with-awf/model-selection.md.awf-bak.1":   "Obsolete model policy.\n",
		".awf/working-with-awf.yaml.awf-bak":                         "data:\n  retained: yes\nsections:\n  model-selection:\n    drop: true\n  commands:\n    drop: true\n",
	} {
		mutation, ok := byPath[path]
		if !ok || mutation.Remove || string(mutation.Content) != body {
			t.Errorf("backup mutation %s = %#v", path, mutation)
		}
	}
	for _, path := range []string{
		".awf/maintainable-code-design.yaml",
		".awf/parts/maintainable-code-design/readability.md",
		".awf/parts/working-with-awf/model-selection.md",
	} {
		if mutation := byPath[path]; !mutation.Remove {
			t.Errorf("retired source %s mutation = %#v", path, mutation)
		}
	}
	cleaned := byPath[".awf/working-with-awf.yaml"]
	if cleaned.Remove || !strings.Contains(string(cleaned.Content), "retained: yes") || !strings.Contains(string(cleaned.Content), "commands:") || strings.Contains(string(cleaned.Content), "model-selection") || strings.Contains(string(cleaned.Content), "advanced-workflow") {
		t.Fatalf("cleaned working sidecar = %#v", cleaned)
	}
	prune := byPath[".awf/skills/parts/using-awf"]
	if !prune.EmptyDirectory || !prune.Remove || prune.Expected.Mode != 0o750 || len(prune.Expected.Children) != 0 {
		t.Fatalf("obsolete directory prune = %#v", prune)
	}
	joinedChanges := ""
	for _, change := range changes {
		joinedChanges += change.Text + "\n"
	}
	for _, want := range []string{
		".awf/parts/working-with-awf/model-selection.md.awf-bak.1",
		"review its content, then delete the backup when no longer needed and remove its retired parent directory if empty",
		"model-selection to advanced-workflow",
		"was not applied to the new section",
	} {
		if !strings.Contains(joinedChanges, want) {
			t.Errorf("changes omit %q:\n%s", want, joinedChanges)
		}
	}
	assertSnapshot(t, root, before)
}

func TestWorkflowSurfaceMigrationRetainsDirectoriesWithBackupsOrUnrelatedChildren(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 52)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/parts/maintainable-code-design/readability.md"), "custom\n")
	testsupport.WriteFile(t, filepath.Join(root, ".awf/skills/parts/using-awf/unrelated.txt"), "keep\n")
	_, _, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range mutations {
		if mutation.EmptyDirectory && (mutation.Path == ".awf/parts/maintainable-code-design" || mutation.Path == ".awf/skills/parts/using-awf") {
			t.Fatalf("unsafe directory prune = %#v", mutation)
		}
	}
}

func TestWorkflowSurfaceMigrationRejectsMalformedWorkingSidecarWithoutPlanning(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 52)
	testsupport.WriteFile(t, filepath.Join(root, ".awf/working-with-awf.yaml"), "sections: scalar\n")
	before := snapshot(t, root)
	_, changes, mutations, err := Build(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "intermediate mapping conflict") {
		t.Fatalf("Build error = %v", err)
	}
	if len(changes) != 0 || len(mutations) != 0 {
		t.Fatalf("unsafe partial plan changes=%#v mutations=%#v", changes, mutations)
	}
	assertSnapshot(t, root, before)
}
