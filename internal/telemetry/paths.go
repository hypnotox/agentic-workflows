package telemetry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type paths struct{ root, sessions, efforts string }

func resolvePaths(ctx context.Context, invoking string) (paths, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, invoking)
	if err != nil {
		return paths{}, err
	}
	root, err := roots.ResidentRoot(awfgit.ResidentMetrics)
	if err != nil {
		return paths{}, err
	}
	return paths{root: root, sessions: filepath.Join(root, "sessions"), efforts: filepath.Join(root, "efforts")}, nil
}
func inspectRegular(path string) (os.FileInfo, error) {
	i, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if i.Mode()&os.ModeSymlink != 0 || !i.Mode().IsRegular() {
		return nil, fmt.Errorf("unsafe telemetry file %s", path)
	}
	return i, nil
}
func inspectDirectory(path string) (os.FileInfo, error) {
	i, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if i.Mode()&os.ModeSymlink != 0 || !i.IsDir() {
		return nil, fmt.Errorf("unsafe telemetry directory %s", path)
	}
	return i, nil
}
