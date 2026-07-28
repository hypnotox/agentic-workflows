package effort

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// invariant: tooling/effort-management:effort-record-authority
func TestEffortFocusedFailureBranches(t *testing.T) {
	t.Run("default allocator", func(t *testing.T) {
		id, err := randomUUIDv4()
		if err != nil || !uuidV4Pattern.MatchString(id) {
			t.Fatalf("random UUID = %q, %v", id, err)
		}
	})

	t.Run("later resident roots", func(t *testing.T) {
		for _, leaf := range []string{"memory", "worktrees"} {
			t.Run(leaf, func(t *testing.T) {
				primary := filepath.Join(t.TempDir(), "primary")
				if err := os.MkdirAll(filepath.Join(primary, ".awf", "efforts"), 0o700); err != nil {
					t.Fatal(err)
				}
				outside := t.TempDir()
				if err := os.Symlink(outside, filepath.Join(primary, ".awf", leaf)); err != nil {
					t.Skip(err)
				}
				if _, err := resolvePaths(awfgit.ControlRoots{PrimaryRoot: primary}); err == nil || !strings.Contains(err.Error(), leaf) {
					t.Fatalf("%s root accepted: %v", leaf, err)
				}
			})
		}
	})

	t.Run("ensure root types and modes", func(t *testing.T) {
		primary := filepath.Join(t.TempDir(), "primary")
		p, err := resolvePaths(awfgit.ControlRoots{PrimaryRoot: primary})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(p.efforts), 0o700); err != nil {
			t.Fatal(err)
		}
		writeEffortFile(t, p.efforts, "not a directory")
		if err := p.ensure(p.efforts); err == nil {
			t.Fatal("file resident root accepted")
		}
		if err := os.Remove(p.efforts); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(p.efforts, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := p.ensure(p.efforts); err != nil {
			t.Fatalf("safe Git-created resident mode refused: %v", err)
		}
		if err := os.Chmod(p.efforts, 0o775); err != nil {
			t.Fatal(err)
		}
		if err := p.ensure(p.efforts); err == nil || !strings.Contains(err.Error(), "resident-permissions") {
			t.Fatalf("group-writable resident mode error = %v", err)
		}
	})

	t.Run("open and memory resident failures", func(t *testing.T) {
		root := initEffortRepo(t)
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, ".awf")); err != nil {
			t.Skip(err)
		}
		if _, err := Open(t.Context(), root, Options{}); err == nil {
			t.Fatal("Open accepted symlinked resident ancestor")
		}
	})

	t.Run("new default memory durability failure", func(t *testing.T) {
		root := initEffortRepo(t)
		service, err := Open(t.Context(), root, Options{UUID: func() (string, error) { return idA, nil }, Filesystem: &faultFileSystem{fail: "create-temp"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.New("Memory fault", true); !errors.Is(err, errInjectedDurability) {
			t.Fatalf("default memory fault = %v", err)
		}
	})

	t.Run("memory ensure failure", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, time.Now().UTC())
		if _, err := service.New("No memory", false); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(service.paths.memory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(service.paths.memory, 0o777); err != nil {
			t.Fatal(err)
		}
		if _, _, err := service.Memory(idA); err == nil || !strings.Contains(err.Error(), "resident-permissions") {
			t.Fatalf("memory ensure error = %v", err)
		}
	})

	t.Run("new refuses unsafe allocated leaf", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, time.Now().UTC())
		if err := os.MkdirAll(service.paths.efforts, 0o700); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "outside")
		writeEffortFile(t, outside, "outside")
		if err := os.Symlink(outside, service.paths.record(idA)); err != nil {
			t.Skip(err)
		}
		if _, err := service.New("Unsafe ID", false); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("unsafe allocated leaf error = %v", err)
		}
	})

	t.Run("corrupt mutation list and atomic destination", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, time.Now().UTC())
		if _, err := service.New("Corrupt", false); err != nil {
			t.Fatal(err)
		}
		writeEffortFile(t, service.paths.record(idA), "{")
		if _, err := service.List(); err == nil {
			t.Fatal("list accepted corrupt valid-name record")
		}
		if _, err := service.Rename(idA, "No"); err == nil {
			t.Fatal("rename accepted corrupt record")
		}
		valid := Record{SchemaVersion: SchemaVersion, ID: idA, Title: "Valid", State: StateActive, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Integration: IntegrationNone}
		if err := service.store.replace(valid, true); err == nil {
			t.Fatal("replacement accepted corrupt existing record")
		}
		invalidTime := valid
		invalidTime.ID = idB
		invalidTime.CreatedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
		invalidTime.UpdatedAt = invalidTime.CreatedAt
		if err := service.store.replace(invalidTime, false); err == nil || !strings.Contains(err.Error(), "encode") {
			t.Fatalf("unencodable timestamp error = %v", err)
		}
		directory := filepath.Join(service.paths.efforts, "destination")
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := atomicReplaceForTest(directory, []byte("new")); err == nil || !strings.Contains(err.Error(), "file-type") {
			t.Fatalf("atomic directory error = %v", err)
		}
	})

	t.Run("service mutation durability propagation", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, time.Now().UTC())
		if _, err := service.New("Durability", false); err != nil {
			t.Fatal(err)
		}
		service.store.fs = &faultFileSystem{fail: "create-temp"}
		if _, err := service.Rename(idA, "Cannot publish"); !errors.Is(err, errInjectedDurability) {
			t.Fatalf("mutation fault = %v", err)
		}
		service.store.fs = nil
		if _, _, err := service.Memory(idA); err != nil {
			t.Fatal(err)
		}
		writeEffortFile(t, service.paths.record(idA), schemaRecordJSON(time.Now().UTC(), "null", "none"))
		service.store.fs = &faultFileSystem{fail: "create-temp"}
		if _, _, err := service.Memory(idA); !errors.Is(err, errInjectedDurability) {
			t.Fatalf("memory record fault = %v", err)
		}
		service.store.fs = nil
		writeEffortFile(t, service.paths.record(idA), schemaRecordJSON(time.Now().UTC(), "null", "none"))
		writeEffortFile(t, service.paths.memoryFile(idA), "Effort: "+idA+"\n")
		service.store.fs = &faultFileSystem{fail: "create-temp"}
		if _, err := service.Repair(idA); !errors.Is(err, errInjectedDurability) {
			t.Fatalf("repair publication fault = %v", err)
		}
		service.store.fs = nil
		if err := os.Remove(service.paths.memoryFile(idA)); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "outside-memory")
		writeEffortFile(t, outside, "Effort: "+idA+"\n")
		if err := os.Symlink(outside, service.paths.memoryFile(idA)); err != nil {
			t.Skip(err)
		}
		if _, err := service.Repair(idA); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("repair memory safety error = %v", err)
		}
	})

	t.Run("repair and join propagated errors", func(t *testing.T) {
		root := initEffortRepo(t)
		service, err := Open(t.Context(), root, Options{Clock: func() time.Time { return time.Now().UTC() }, UUID: func() (string, error) { return idA, nil }, Worktrees: func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
			return nil, os.ErrPermission
		}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.New("Failures", false); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Repair(idA); !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), service.paths.managedWorktree(idA)) {
			t.Fatalf("repair registration error = %v", err)
		}
		writeEffortFile(t, service.paths.record(idA), schemaRecordJSON(time.Now().UTC(), "null", "none")+" x")
		if _, err := service.Show(idA); err == nil {
			t.Fatal("trailing malformed JSON accepted")
		}
		writeEffortFile(t, service.paths.record(idA), schemaRecordJSON(time.Now().UTC(), "null", "none"))
		service.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
		if _, err := service.Repair(idA); err != nil {
			t.Fatalf("repair worktree inspection error = %v", err)
		}
	})
}
