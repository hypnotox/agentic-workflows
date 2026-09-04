package migrate

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

func writeTestFile(t *testing.T, root, name, contents string, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func testRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	writeTestFile(t, root, config.DirName+"/config.yaml", "prefix: test\n", 0o644)
	lock := &manifest.Lock{AWFVersion: "0.45.0", SchemaVersion: Current(), Files: map[string]manifest.Entry{"AGENTS.md": {}}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	for name, contents := range files {
		writeTestFile(t, root, name, contents, 0o644)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "fixture")
	return root
}

func withMigration(t *testing.T, build func(context.Context, *proposedTree, *Changes) ([]fileMutation, error)) {
	t.Helper()
	original := registry
	registry = append(registry, migration{To: Current() + 1, Name: "synthetic", Build: build})
	t.Cleanup(func() { registry = original })
}

func convergentBuild(ctx context.Context, tree *proposedTree, _ *Changes) ([]fileMutation, error) {
	var out []fileMutation
	moves, err := planAuthoredMove(ctx, tree, ".awf/old.txt", ".awf/new.txt")
	if err != nil {
		return nil, err
	}
	out = append(out, moves...)
	contents, mode, err := tree.Read(".awf/rewrite.txt")
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(contents, []byte("new rewrite\n")) {
		out = append(out, fileMutation{Path: ".awf/rewrite.txt", Content: []byte("new rewrite\n"), Mode: mode})
	}
	if _, _, err := tree.Read(".awf/remove.txt"); err == nil {
		out = append(out, fileMutation{Path: ".awf/remove.txt", Remove: true})
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	prune, ok, err := tree.PlanEmptyDirectory(".awf/empty-after", out)
	if err != nil {
		return nil, err
	}
	if ok {
		out = append(out, prune)
	}
	return out, nil
}

// invariant: config/migrations-and-locks:migration-ordering (TestApplyOrdersOperationsAndDoesNotWriteLock)
func TestApplyOrdersOperationsAndDoesNotWriteLock(t *testing.T) {
	root := testRepo(t, map[string]string{
		".awf/old.txt": "move me\n", ".awf/rewrite.txt": "old rewrite\n",
		".awf/remove.txt": "remove\n", ".awf/empty-after/child": "child\n",
	})
	withMigration(t, func(ctx context.Context, tree *proposedTree, changes *Changes) ([]fileMutation, error) {
		ops, err := convergentBuild(ctx, tree, changes)
		if err != nil {
			return nil, err
		}
		ops = append(ops, fileMutation{Path: ".awf/empty-after/child", Remove: true})
		prune, ok, err := tree.PlanEmptyDirectory(".awf/empty-after", ops)
		if err != nil {
			return nil, err
		}
		if ok {
			ops = append(ops, prune)
		}
		return ops, nil
	})
	before, err := os.ReadFile(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	result, err := applyWithHook(context.Background(), root, func(_ int, name string) error { order = append(order, name); return nil })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".awf/new.txt", ".awf/rewrite.txt", ".awf/empty-after/child", ".awf/old.txt", ".awf/remove.txt", ".awf/empty-after"}
	if !slices.Equal(order, want) || !slices.Equal(result.Touched, want) || len(result.Pending) != 0 || !slices.Equal(result.Applied, []string{"synthetic"}) {
		t.Fatalf("order=%v result=%#v", order, result)
	}
	after, err := os.ReadFile(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("migration wrote schema lock")
	}
}

// invariant: config/migrations-and-locks:context-skill-source-migration (TestSchema50To53ConvergesWithoutBackupsOrLockWrite)
// invariant: config/migrations-and-locks:skill-extraction-source-migration (TestSchema50To53ConvergesWithoutBackupsOrLockWrite)
// invariant: config/migrations-and-locks:workflow-surface-source-migration (TestSchema50To53ConvergesWithoutBackupsOrLockWrite)
func TestSchema50To53ConvergesWithoutBackupsOrLockWrite(t *testing.T) {
	root := testRepo(t, map[string]string{
		".awf/skills/effort-workflow.yaml":               "data:\n  local: retained\n",
		".awf/skills/repository-context.yaml":            "data:\n  local: retired\n",
		".awf/skills/brainstorming.yaml":                 "data:\n  local: retired\n",
		".awf/maintainable-code-design.yaml":             "data:\n  local: retired\n",
		".awf/parts/working-with-awf/model-selection.md": "retired model guidance\n",
		".awf/working-with-awf.yaml":                     "sections:\n  model-selection:\n    local: true\n  commands:\n    local: true\n",
	})
	lock, found, err := manifest.LoadOptional(config.LockPath(root))
	if err != nil || !found {
		t.Fatalf("load lock: found=%t err=%v", found, err)
	}
	lock.SchemaVersion = 50
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", config.DirName+"/awf.lock")
	gitRun(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--amend", "--no-edit", "-q")
	before, err := os.ReadFile(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}

	result, err := Apply(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Planned, []string{contextSkillMigration, skillExtractionMigration, workflowSurfaceMigration}) || !slices.Equal(result.Applied, result.Planned) || len(result.Pending) != 0 {
		t.Fatalf("result=%#v", result)
	}
	got, err := os.ReadFile(filepath.Join(root, ".awf/skills/awf-effort.yaml"))
	if err != nil || string(got) != "data:\n  local: retained\n" {
		t.Fatalf("retained destination=%q err=%v", got, err)
	}
	for _, name := range []string{
		".awf/skills/effort-workflow.yaml", ".awf/skills/repository-context.yaml",
		".awf/skills/brainstorming.yaml", ".awf/maintainable-code-design.yaml",
		".awf/parts/working-with-awf/model-selection.md",
	} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("retired source %s remains: %v", name, err)
		}
	}
	after, err := os.ReadFile(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("migration changed schema lock")
	}
	if err := filepath.Walk(filepath.Join(root, ".awf"), func(name string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(filepath.Base(name), ".awf-bak") {
			return errors.New("automatic backup exists at " + name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	second, err := Apply(context.Background(), root)
	if err != nil || len(second.Touched) != 0 || len(second.Pending) != 0 {
		t.Fatalf("rerun=%#v err=%v", second, err)
	}
}

func TestPreflightRefusesChangedAuthorityBeforeFirstMutation(t *testing.T) {
	root := testRepo(t, nil)
	withMigration(t, func(context.Context, *proposedTree, *Changes) ([]fileMutation, error) {
		lockPath := config.LockPath(root)
		contents, err := os.ReadFile(lockPath)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(lockPath, append(contents, ' '), 0o644); err != nil {
			return nil, err
		}
		return []fileMutation{{Path: ".awf/not-created", Content: []byte("new\n"), Mode: 0o644}}, nil
	})
	result, err := Apply(context.Background(), root)
	if err == nil || len(result.Touched) != 0 || !slices.Equal(result.Pending, []string{".awf/not-created"}) {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, ".awf/not-created")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("preflight created path: %v", statErr)
	}
}

func TestPreflightRefusesDestructiveSourcesWithoutChangingAnything(t *testing.T) {
	cases := []string{"modified", "staged", "untracked", "ignored"}
	for _, state := range cases {
		t.Run(state, func(t *testing.T) {
			files := map[string]string{}
			if state == "modified" || state == "staged" {
				files[".awf/source.txt"] = "original\n"
			}
			if state == "ignored" {
				files[".gitignore"] = ".awf/source.txt\n"
			}
			root := testRepo(t, files)
			if state == "modified" || state == "staged" {
				writeTestFile(t, root, ".awf/source.txt", "changed\n", 0o644)
			} else {
				writeTestFile(t, root, ".awf/source.txt", "resident\n", 0o644)
			}
			if state == "staged" {
				gitRun(t, root, "add", ".awf/source.txt")
			}
			original := registry
			registry = append(registry, migration{To: Current() + 1, Name: "proof", Build: func(_ context.Context, tree *proposedTree, _ *Changes) ([]fileMutation, error) {
				content, mode, err := tree.Read(".awf/source.txt")
				if err != nil {
					return nil, err
				}
				return []fileMutation{{Path: ".awf/created-first", Content: content, Mode: mode}, {Path: ".awf/source.txt", Remove: true}}, nil
			}})
			result, err := Apply(context.Background(), root)
			registry = original
			if err == nil || len(result.Touched) != 0 {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			if _, statErr := os.Lstat(filepath.Join(root, ".awf/created-first")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("create occurred: %v", statErr)
			}
			if _, statErr := os.Lstat(filepath.Join(root, ".awf/source.txt")); statErr != nil {
				t.Fatalf("source changed: %v", statErr)
			}
		})
	}
}

func TestExecutableModeProofAndPreservation(t *testing.T) {
	root := testRepo(t, map[string]string{".awf/tool": "#!/bin/sh\n"})
	if err := os.Chmod(filepath.Join(root, ".awf/tool"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".awf/tool")
	gitRun(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--amend", "--no-edit", "-q")
	withMigration(t, func(ctx context.Context, tree *proposedTree, _ *Changes) ([]fileMutation, error) {
		return planAuthoredMove(ctx, tree, ".awf/tool", ".awf/tool-new")
	})
	if _, err := Apply(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, ".awf/tool-new"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode=%v err=%v", info, err)
	}
}

func TestExecutableModeMismatchRefusesBeforeMutation(t *testing.T) {
	for _, staged := range []bool{false, true} {
		name := "worktree"
		if staged {
			name = "index"
		}
		t.Run(name, func(t *testing.T) {
			root := testRepo(t, map[string]string{".awf/source": "source\n"})
			if err := os.Chmod(filepath.Join(root, ".awf/source"), 0o755); err != nil {
				t.Fatal(err)
			}
			if staged {
				gitRun(t, root, "add", ".awf/source")
			}
			original := registry
			registry = append(registry, migration{To: Current() + 1, Name: "mode proof", Build: func(context.Context, *proposedTree, *Changes) ([]fileMutation, error) {
				return []fileMutation{{Path: ".awf/source", Remove: true}}, nil
			}})
			t.Cleanup(func() { registry = original })
			result, err := Apply(context.Background(), root)
			if err == nil || len(result.Touched) != 0 {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			if _, err := os.Stat(filepath.Join(root, ".awf/source")); err != nil {
				t.Fatalf("source removed: %v", err)
			}
		})
	}
}

func TestUnrelatedDirtyPathDoesNotBlock(t *testing.T) {
	for _, staged := range []bool{false, true} {
		name := "unstaged"
		if staged {
			name = "staged"
		}
		t.Run(name, func(t *testing.T) {
			root := testRepo(t, map[string]string{".awf/source.txt": "tracked\n", "unrelated.txt": "old\n"})
			writeTestFile(t, root, "unrelated.txt", "dirty\n", 0o644)
			if staged {
				gitRun(t, root, "add", "unrelated.txt")
			}
			original := registry
			registry = append(registry, migration{To: Current() + 1, Name: "unrelated", Build: func(_ context.Context, _ *proposedTree, _ *Changes) ([]fileMutation, error) {
				return []fileMutation{{Path: ".awf/source.txt", Remove: true}}, nil
			}})
			t.Cleanup(func() { registry = original })
			if result, err := Apply(context.Background(), root); err != nil || !slices.Equal(result.Touched, []string{".awf/source.txt"}) {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			got, err := os.ReadFile(filepath.Join(root, "unrelated.txt"))
			if err != nil || string(got) != "dirty\n" {
				t.Fatalf("unrelated=%q err=%v", got, err)
			}
		})
	}
}

// invariant: config/migrations-and-locks:migration-mutation-safe (TestFailureLeavesTouchedAndPendingAndEveryRerunConverges)
func TestFailureLeavesTouchedAndPendingAndEveryRerunConverges(t *testing.T) {
	for failAt := 0; failAt < 4; failAt++ {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			root := testRepo(t, map[string]string{".awf/old.txt": "move\n", ".awf/rewrite.txt": "old\n", ".awf/remove.txt": "remove\n"})
			original := registry
			registry = append(registry, migration{To: Current() + 1, Name: "convergent", Build: convergentBuild})
			failure := errors.New("injected")
			partial, err := applyWithHook(context.Background(), root, func(i int, _ string) error {
				if i == failAt {
					return failure
				}
				return nil
			})
			if !errors.Is(err, failure) || len(partial.Touched) != failAt || len(partial.Pending) != 4-failAt {
				t.Fatalf("partial=%#v err=%v", partial, err)
			}
			final, err := Apply(context.Background(), root)
			registry = original
			if err != nil || len(final.Pending) != 0 {
				t.Fatalf("rerun=%#v err=%v", final, err)
			}
			if got, err := os.ReadFile(filepath.Join(root, ".awf/new.txt")); err != nil || string(got) != "move\n" {
				t.Fatalf("new=%q err=%v", got, err)
			}
			for _, name := range []string{".awf/old.txt", ".awf/remove.txt"} {
				if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("%s remains: %v", name, err)
				}
			}
			matches, _ := filepath.Glob(filepath.Join(root, ".awf", "*.awf-bak*"))
			if len(matches) != 0 {
				t.Fatalf("backups=%v", matches)
			}
		})
	}
}

func TestFinalSymlinkConfinementRefusesBeforeMutation(t *testing.T) {
	root := testRepo(t, nil)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".awf", "escape")); err != nil {
		t.Skip(err)
	}
	withMigration(t, func(context.Context, *proposedTree, *Changes) ([]fileMutation, error) {
		return []fileMutation{{Path: ".awf/a-created", Content: []byte("x"), Mode: 0o644}, {Path: ".awf/escape/victim", Content: []byte("bad"), Mode: 0o644}}, nil
	})
	result, err := Apply(context.Background(), root)
	if err == nil || len(result.Touched) != 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf/a-created")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preflight mutated before symlink refusal")
	}
	if _, err := os.Lstat(filepath.Join(outside, "victim")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("escaped root")
	}
}

func TestNoProductionBackupLanguage(t *testing.T) {
	for _, name := range []string{"skill_extraction.go", "workflow_surface.go"} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), ".awf-bak") {
			t.Fatalf("%s retains backup behavior", name)
		}
	}
}
