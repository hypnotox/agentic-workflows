package pitfallop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func fixture(t *testing.T) (string, *project.Loader) {
	t.Helper()
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\nintegrationBranch: main\nvars: {testCmd: go test ./..., gateCmd: make gate}\n")
	return root, project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(context.Context, string) string { return root })
}

func TestCreateExclusiveRerunConvergesAndDifferingCollisionRefuses(t *testing.T) {
	root, loader := fixture(t)
	first, err := Create(context.Background(), root, "Durable Hazard", loader, nil)
	if err != nil || first.SourcePath == "" {
		t.Fatalf("first=%#v %v", first, err)
	}
	second, err := Create(context.Background(), root, "Durable Hazard", loader, nil)
	if err != nil || second.SourcePath != first.SourcePath {
		t.Fatalf("rerun=%#v %v", second, err)
	}
	path := filepath.Join(root, filepath.FromSlash(first.SourcePath))
	if err := os.WriteFile(path, []byte("foreign"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(context.Background(), root, "Durable Hazard", loader, nil); err == nil {
		t.Fatal("differing collision accepted")
	}
}

func TestCreateAcquiresTrackedLeaseBeforeAuthorityAndJoinsRelease(t *testing.T) {
	root, _ := fixture(t)
	acquired := false
	observing := false
	loader := project.NewLoaderWithoutRepository(func(path string) (*config.Config, error) {
		if observing && !acquired {
			t.Fatal("read before lease")
		}
		return config.Load(path)
	}, catalog.Standard, func(context.Context, string) string { return root })
	fault := errors.New("release sentinel")
	acquire := func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
		acquired = true
		lease, err := filesystem.AcquireTrackedLease(ctx, root)
		return lease, func() error { return errors.Join(lease.Release(), fault) }, err
	}
	observing = true
	outcome, err := Create(context.Background(), root, "Lease Hazard", loader, acquire)
	if outcome.SourcePath == "" || !errors.Is(err, fault) {
		t.Fatalf("outcome=%#v error=%v", outcome, err)
	}
}
