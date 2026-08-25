package initop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/project"
)

func TestCleanupScaffoldRemovesOnlyOperationOwnedConfig(t *testing.T) {
	for _, scaffolded := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing", true: "scaffolded"}[scaffolded], func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), ".awf", "config.yaml")
			if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cfgPath, []byte("config"), 0o644); err != nil {
				t.Fatal(err)
			}
			cleanupScaffold(cfgPath, scaffolded)
			_, err := os.Stat(cfgPath)
			if scaffolded && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("scaffolded config survived cleanup: %v", err)
			}
			if !scaffolded && err != nil {
				t.Fatalf("existing config was removed: %v", err)
			}
		})
	}
}

func TestRunPropagatesLoaderOpenFailure(t *testing.T) {
	root := t.TempDir()
	want := errors.New("open project")
	_, err := Run(context.Background(), Input{Root: root, Force: true}, func(string) (*project.Loader, error) {
		return project.NewLoaderWithoutRepository(
			func(string) (*config.Config, error) { return nil, want },
			catalog.Standard,
			func(_ context.Context, selected string) string { return selected },
		), nil
	}, func(context.Context, string) error {
		t.Fatal("gate called after loader open failure")
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("Run error = %v, want %v", err, want)
	}
}

func TestProbeCollisionsPropagatesLoaderConstructionFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(config.ConfigPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.ConfigPath(root), []byte("prefix: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := errors.New("loader failed")
	_, err := probeCollisions(context.Background(), root, func(string) (*project.Loader, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("probe error = %v, want %v", err, want)
	}
}
