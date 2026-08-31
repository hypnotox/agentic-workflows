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
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectmutation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
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

func runCreate(ctx context.Context, root, domain, title string, loader *project.Loader) (document presentation.Document, returnErr error) {
	lease, err := filesystem.AcquireTrackedLease(ctx, root)
	if err != nil {
		return presentation.Document{}, err
	}
	defer func() { returnErr = errors.Join(returnErr, lease.Release()) }()
	tx, err := projectmutation.UseTracked(ctx, root, loader, lease)
	if err != nil {
		return presentation.Document{}, err
	}
	outcome, err := Create(ctx, domain, title, tx)
	return outcome.Document, err
}

func TestCreateReportsUnavailableProject(t *testing.T) {
	loader := project.NewLoaderWithoutRepository(config.Load, catalog.Standard, awfgit.ProjectResidentRoot)
	if _, err := runCreate(context.Background(), t.TempDir(), "rendering", "Current State", loader); err == nil {
		t.Fatal("unavailable project accepted")
	}
}

func TestCreateLeasedRefusesRootReplacementBetweenAuthorityLoadAndPublication(t *testing.T) {
	root := t.TempDir()
	relocated := root + ".opened"
	t.Cleanup(func() { _ = os.RemoveAll(relocated) })
	const oldConfig = "prefix: example\nintegrationBranch: master\nvars: {testCmd: go test ./..., gateCmd: make gate}\ndomains: [rendering]\n"
	const replacementConfig = "prefix: example\nintegrationBranch: master\nvars: {testCmd: go test ./..., gateCmd: make gate}\ndomains: [payments]\n"
	testsupport.WriteAwfConfig(t, root, oldConfig)
	loader := project.NewLoaderWithoutRepository(func(awfDir string) (*config.Config, error) {
		cfg, err := config.Load(awfDir)
		if err != nil {
			return nil, err
		}
		if err := os.Rename(root, relocated); err != nil {
			return nil, err
		}
		testsupport.WriteAwfConfig(t, root, replacementConfig)
		return cfg, nil
	}, catalog.Standard, awfgit.ProjectResidentRoot)
	lease, err := filesystem.AcquireTrackedLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Release(); err != nil {
			t.Errorf("release tracked lease: %v", err)
		}
	}()
	tx, err := projectmutation.UseTracked(context.Background(), root, loader, lease)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Create(context.Background(), "rendering", "Root Swap", tx); !errors.Is(err, filesystem.ErrIdentityChanged) {
		t.Fatalf("root replacement = %v, want identity refusal", err)
	}
	for _, tree := range []string{root, relocated} {
		if _, err := os.Lstat(filepath.Join(tree, ".awf", "topics", "metadata", "rendering", "root-swap.yaml")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("topic scaffold mutated %s: %v", tree, err)
		}
	}
}

func TestScaffoldCreatesPairedAuthoredInputs(t *testing.T) {
	root := t.TempDir()
	files, err := filesystem.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	_, _, err = scaffoldConfined(files, &config.Config{Domains: []string{"rendering"}}, "rendering", "Current State")
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

func TestScaffoldCloseFailureReportsEveryCommittedPath(t *testing.T) {
	closeFailure := errors.New("close selected root")
	created := []string{"metadata.yaml", "part.md"}
	err := finishScaffoldClose(created, nil, closeFailure)
	var partial *PartialScaffoldError
	if !errors.As(err, &partial) || !errors.Is(err, closeFailure) {
		t.Fatalf("close outcome = %v, want typed partial scaffold preserving cause", err)
	}
	if !slices.Equal(partial.Created, created) || !slices.Equal(partial.Remaining, created) {
		t.Fatalf("close paths = created %v, remaining %v; want %v", partial.Created, partial.Remaining, created)
	}
	if len(partial.Recovery) != 2 || !strings.Contains(partial.Recovery[0], "inspect") || !strings.Contains(partial.Recovery[1], "do not retry") {
		t.Fatalf("close recovery = %v", partial.Recovery)
	}
	document, documentErr := partial.Document()
	if documentErr != nil {
		t.Fatal(documentErr)
	}
	var rendered strings.Builder
	if renderErr := presentation.Render(&rendered, document); renderErr != nil {
		t.Fatal(renderErr)
	}
	for _, want := range []string{"created paths", "metadata.yaml", "part.md", "remaining paths", "do not retry scaffolding"} {
		if !strings.Contains(rendered.String(), want) {
			t.Errorf("close diagnostic omitted %q: %s", want, rendered.String())
		}
	}
	if strings.Contains(rendered.String(), "removed paths") {
		t.Fatalf("close diagnostic rendered an empty removed group: %s", rendered.String())
	}

	operationFailure := errors.New("operation failed")
	for _, test := range []struct {
		name      string
		created   []string
		operation error
	}{
		{name: "pre-publication close", created: nil},
		{name: "operation and close", created: created, operation: operationFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := finishScaffoldClose(test.created, test.operation, closeFailure)
			if !errors.Is(err, closeFailure) || (test.operation != nil && !errors.Is(err, operationFailure)) {
				t.Fatalf("combined close outcome = %v", err)
			}
			var gotPartial *PartialScaffoldError
			if errors.As(err, &gotPartial) {
				t.Fatalf("uncommitted or already-classified operation became a new partial outcome: %v", err)
			}
		})
	}
}

func TestFinishDoesNotPromoteFullyRolledBackFailure(t *testing.T) {
	operationFault := errors.New("write sentinel")
	releaseFault := errors.New("release sentinel")
	err := Finish(Outcome{}, operationFault, releaseFault)
	var partial *PartialScaffoldError
	if errors.As(err, &partial) {
		t.Fatalf("rolled-back failure became partial: %#v", partial)
	}
	if !errors.Is(err, operationFault) || !errors.Is(err, releaseFault) {
		t.Fatalf("rolled-back causes = %v", err)
	}
}

func TestFinishTypesReleaseFaultWithCreatedPaths(t *testing.T) {
	fault := errors.New("release sentinel")
	outcome := Outcome{Created: []string{"metadata.yaml", "part.md"}}
	err := Finish(outcome, nil, fault)
	var partial *PartialScaffoldError
	if !errors.As(err, &partial) || !errors.Is(err, fault) {
		t.Fatalf("release partial = %#v, %v", partial, err)
	}
	if !slices.Equal(partial.Created, outcome.Created) || !slices.Equal(partial.Remaining, outcome.Created) {
		t.Fatalf("release paths = created %v remaining %v", partial.Created, partial.Remaining)
	}
	if len(partial.Recovery) != 2 || !strings.Contains(partial.Recovery[1], "lease-release fault") {
		t.Fatalf("release recovery = %#v", partial.Recovery)
	}
}

func TestPartialScaffoldErrorDocumentRetainsEveryPathGroupAndDefaultRecovery(t *testing.T) {
	partial := &PartialScaffoldError{Created: []string{"metadata.yaml"}, Removed: []string{"part.md"}, Remaining: []string{"part.md"}}
	document, err := partial.Document()
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	if err := presentation.Render(&rendered, document); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"topic scaffold partially committed", "created paths", "metadata.yaml", "removed paths", "remaining paths", "remove only the listed remaining topic paths, then retry"} {
		if !strings.Contains(rendered.String(), want) {
			t.Errorf("document omitted %q: %s", want, rendered.String())
		}
	}
}
