package topicop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

type faultWriter struct {
	file *os.File
	err  error
}

func (w faultWriter) Write(p []byte) (int, error) {
	n, err := w.file.Write(p[:min(3, len(p))])
	if err != nil {
		return n, err
	}
	return n, w.err
}
func (w faultWriter) Close() error { return w.file.Close() }

func TestCreateReportsUnavailableProject(t *testing.T) {
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot)
	if _, err := Create(context.Background(), t.TempDir(), "rendering", "Current State", loader); err == nil {
		t.Fatal("unavailable project accepted")
	}
}

func TestScaffoldCreatesPairedAuthoredInputs(t *testing.T) {
	root := t.TempDir()
	_, err := Scaffold(root, &config.Config{Domains: []string{"rendering"}}, "rendering", "Current State")
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".awf/topics/metadata/rendering/current-state.yaml", ".awf/topics/parts/rendering/current-state/current-state.md"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
	}
}

func TestScaffoldRollsBackInReverseFileOrderAndKeepsCollision(t *testing.T) {
	root := t.TempDir()
	failure := errors.New("write failed")
	dependencies := productionScaffoldDependencies()
	opens := 0
	dependencies.openFile = func(path string, flag int, mode os.FileMode) (scaffoldWriteCloser, error) {
		opens++
		file, err := os.OpenFile(path, flag, mode)
		if err != nil || opens == 1 {
			return file, err
		}
		return faultWriter{file, failure}, nil
	}
	var removed []string
	dependencies.remove = func(path string) error { removed = append(removed, filepath.ToSlash(path)); return os.Remove(path) }
	_, err := scaffoldWith(root, &config.Config{Domains: []string{"rendering"}}, "rendering", "Failure", dependencies)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	want := []string{filepath.ToSlash(filepath.Join(root, ".awf/topics/parts/rendering/failure/current-state.md")), filepath.ToSlash(filepath.Join(root, ".awf/topics/metadata/rendering/failure.yaml"))}
	if len(removed) < len(want) || !slices.Equal(removed[:len(want)], want) {
		t.Fatalf("file removal order = %v, want prefix %v", removed, want)
	}
	for _, path := range want {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("rollback retained %s: %v", path, err)
		}
	}
}

func TestScaffoldReportsInjectedFilesystemFailures(t *testing.T) {
	cfg := &config.Config{Domains: []string{"rendering"}}
	t.Run("invalid topic", func(t *testing.T) {
		if _, err := scaffoldWith(t.TempDir(), cfg, "Rendering", "Title", productionScaffoldDependencies()); err == nil {
			t.Fatal("invalid topic accepted")
		}
	})
	t.Run("parent inspection", func(t *testing.T) {
		dependencies := productionScaffoldDependencies()
		failure := errors.New("inspect failed")
		dependencies.stat = func(string) (os.FileInfo, error) { return nil, failure }
		if _, err := scaffoldWith(t.TempDir(), cfg, "rendering", "Title", dependencies); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("exclusive open", func(t *testing.T) {
		dependencies := productionScaffoldDependencies()
		failure := errors.New("open failed")
		dependencies.openFile = func(string, int, os.FileMode) (scaffoldWriteCloser, error) { return nil, failure }
		if _, err := scaffoldWith(t.TempDir(), cfg, "rendering", "Title", dependencies); !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	})
}

type closeFaultWriter struct{ err error }

func (w closeFaultWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w closeFaultWriter) Close() error                { return w.err }

func TestScaffoldHelpersPreserveFailureIdentity(t *testing.T) {
	failure := errors.New("injected failure")
	if err := writeAndClose("x", closeFaultWriter{failure}, []byte("body")); !errors.Is(err, failure) {
		t.Fatalf("close error = %v", err)
	}
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	dependencies := productionScaffoldDependencies()
	if _, err := createParents(file, dependencies); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("non-directory parent = %v", err)
	}
	dependencies.mkdirAll = func(string, os.FileMode) error { return failure }
	if _, err := createParents(filepath.Join(root, "missing", "child"), dependencies); !errors.Is(err, failure) {
		t.Fatalf("mkdir error = %v", err)
	}
	dependencies.remove = func(string) error { return failure }
	dependencies.readDir = func(string) ([]os.DirEntry, error) { return nil, failure }
	if err := rollback(failure, []string{"file"}, []string{"dir"}, dependencies); !errors.Is(err, failure) {
		t.Fatalf("rollback error = %v", err)
	}
	dependencies.readDir = func(string) ([]os.DirEntry, error) { return nil, nil }
	if err := rollback(failure, nil, []string{"dir"}, dependencies); !errors.Is(err, failure) {
		t.Fatalf("empty directory rollback error = %v", err)
	}
	dependencies.readDir = func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist }
	if err := rollback(failure, nil, []string{"dir"}, dependencies); !errors.Is(err, failure) {
		t.Fatalf("absent directory rollback error = %v", err)
	}
}

func TestCreateParentsStopsAtFilesystemRootWhenEveryAncestorIsMissing(t *testing.T) {
	dependencies := productionScaffoldDependencies()
	dependencies.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	dependencies.mkdirAll = func(string, os.FileMode) error { return nil }
	created, err := createParents(filepath.Join(t.TempDir(), "missing", "child"), dependencies)
	if err != nil || len(created) != 0 {
		t.Fatalf("create parents = %v, %v", created, err)
	}
}

func TestScaffoldDoesNotReplaceLateCollision(t *testing.T) {
	root := t.TempDir()
	dependencies := productionScaffoldDependencies()
	const existing = "existing\n"
	opens := 0
	dependencies.openFile = func(path string, flag int, mode os.FileMode) (scaffoldWriteCloser, error) {
		opens++
		if opens == 2 {
			if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return os.OpenFile(path, flag, mode)
	}
	_, err := scaffoldWith(root, &config.Config{Domains: []string{"rendering"}}, "rendering", "Collision", dependencies)
	part := filepath.Join(root, ".awf/topics/parts/rendering/collision/current-state.md")
	if !errors.Is(err, os.ErrExist) || !strings.Contains(err.Error(), filepath.ToSlash(part)) {
		t.Fatalf("collision = %v", err)
	}
	got, readErr := os.ReadFile(part)
	if readErr != nil || string(got) != existing {
		t.Fatalf("bytes = %q, %v", got, readErr)
	}
}
