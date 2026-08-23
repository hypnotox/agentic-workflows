package checkop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"golang.org/x/mod/semver"
)

var errNoStagedLock = errors.New("no staged lock")

func stagedTree(ctx context.Context, root string) (*snapshot.Tree, error) {
	repo, _, err := awfgit.OpenContaining(root)
	if err != nil {
		return nil, err
	}
	return snapshot.IndexTree(ctx, repo)
}

func stagedLock(ctx context.Context, root string) (*manifest.Lock, error) {
	tree, err := stagedTree(ctx, root)
	if err != nil {
		return nil, err
	}
	file, ok := tree.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, fmt.Errorf("%w: no staged %s/awf.lock", errNoStagedLock, config.DirName)
	}
	lock, err := manifest.Parse(file.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse staged lock: %w", err)
	}
	return lock, nil
}

func lockVsBinary(root string) (lockV, binV string, ok bool, err error) {
	l, found, err := manifest.LoadOptional(config.LockPath(root))
	if err != nil || !found {
		return "", "", false, err
	}
	lockV, binV, ok = lockVsBinaryLock(l)
	return lockV, binV, ok, nil
}
func lockVsBinaryLock(l *manifest.Lock) (lockV, binV string, ok bool) {
	if l == nil || l.AWFVersion == "" {
		return "", "", false
	}
	lockV, lok := normalizeSemver(l.AWFVersion)
	binV, bok := normalizeSemver(project.Version)
	return lockV, binV, lok && bok
}
func normalizeSemver(v string) (string, bool) {
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return "", false
	}
	return v, true
}
