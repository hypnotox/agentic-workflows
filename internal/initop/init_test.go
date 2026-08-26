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

func TestRollbackScaffoldRestoresOwnedTreeOrReportsChangedIdentity(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "changed"}[changed], func(t *testing.T) {
			root := t.TempDir()
			cfgPath := config.ConfigPath(root)
			if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cfgPath, []byte("owned"), 0o644); err != nil {
				t.Fatal(err)
			}
			configInfo, err := os.Lstat(cfgPath)
			if err != nil {
				t.Fatal(err)
			}
			dirInfo, err := os.Lstat(filepath.Dir(cfgPath))
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				if err := os.Remove(cfgPath); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(cfgPath, []byte("winner"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want := errors.New("later failure")
			outcome, gotErr := rollbackScaffold(root, cfgPath, scaffoldCommit{committed: true, createdDir: true, configInfo: configInfo, dirInfo: dirInfo}, want)
			if !errors.Is(gotErr, want) {
				t.Fatalf("rollback error = %v, want %v", gotErr, want)
			}
			if changed {
				var partial *PartialError
				if !errors.As(gotErr, &partial) || outcome.ConfigPath != cfgPath {
					t.Fatalf("changed rollback = %#v, %v; want typed partial", outcome, gotErr)
				}
				if got, err := os.ReadFile(cfgPath); err != nil || string(got) != "winner" {
					t.Fatalf("winner = %q, %v", got, err)
				}
				return
			}
			if outcome.ConfigPath != "" {
				t.Fatalf("successful rollback outcome = %#v", outcome)
			}
			if _, err := os.Stat(filepath.Join(root, config.DirName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("owned scaffold survived rollback: %v", err)
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
	if _, statErr := os.Stat(filepath.Join(root, config.DirName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("loader failure left scaffold residue: %v", statErr)
	}
}

func TestRunGateFailureRollsBackOwnedScaffold(t *testing.T) {
	root := t.TempDir()
	want := errors.New("gate failed")
	loader := func(string) (*project.Loader, error) {
		return project.NewLoaderWithoutRepository(config.Load, catalog.Standard, func(_ context.Context, selected string) string { return selected }), nil
	}
	_, err := Run(context.Background(), Input{Root: root, Force: true}, loader, func(context.Context, string) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("Run error = %v, want %v", err, want)
	}
	if _, statErr := os.Stat(filepath.Join(root, config.DirName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("gate failure left scaffold residue: %v", statErr)
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
