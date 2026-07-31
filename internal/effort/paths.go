package effort

import (
	"fmt"
	"os"
	"path/filepath"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type paths struct {
	roots     awfgit.ControlRoots
	efforts   string
	worktrees string
}

func resolvePaths(roots awfgit.ControlRoots) (paths, error) {
	efforts, err := roots.ResidentRoot(awfgit.ResidentEfforts)
	if err != nil {
		return paths{}, fmt.Errorf("resolve efforts resident root: %w", err)
	}
	worktrees, err := roots.ResidentRoot(awfgit.ResidentWorktrees)
	if err != nil {
		return paths{}, fmt.Errorf("resolve worktrees resident root: %w", err)
	}
	return paths{roots: roots, efforts: efforts, worktrees: worktrees}, nil
}

func (p paths) ensure(root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create resident root %s: %w", root, err)
	}
	info, err := os.Lstat(root)
	if err != nil { // coverage-ignore: MkdirAll just proved this exact path exists; only a concurrent namespace race can remove it
		return fmt.Errorf("inspect resident root %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 { // coverage-ignore: MkdirAll rejects a symlink leaf and ResidentRoot rejects ancestor symlinks
		return &awfgit.HardSafetyError{Category: "symlink", Path: root}
	}
	if !info.IsDir() { // coverage-ignore: MkdirAll rejects a non-directory leaf
		return &awfgit.HardSafetyError{Category: "file-type", Path: root, Err: fmt.Errorf("mode %s is not a directory", info.Mode())}
	}
	if info.Mode().Perm()&0o022 != 0 {
		return &awfgit.HardSafetyError{Category: "resident-permissions", Path: root, Err: fmt.Errorf("mode is %o, group/world write bits must be clear", info.Mode().Perm())}
	}
	return p.validate(root)
}

func (p paths) validate(root string) error {
	var name awfgit.ResidentName
	switch root {
	case p.efforts:
		name = awfgit.ResidentEfforts
	case p.worktrees:
		name = awfgit.ResidentWorktrees
	default:
		return fmt.Errorf("unknown resident root %s", root)
	}
	resolved, err := p.roots.ResidentRoot(name)
	if err != nil {
		return fmt.Errorf("revalidate resident root %s: %w", root, err)
	}
	if filepath.Clean(resolved) != filepath.Clean(root) { // coverage-ignore: the closed mapping returns the same resolved root
		return fmt.Errorf("resident root changed from %s to %s", root, resolved)
	}
	return nil
}

func (p paths) effort(slug string) string          { return filepath.Join(p.efforts, slug) }
func (p paths) stateFile(slug string) string       { return filepath.Join(p.effort(slug), "state.json") }
func (p paths) memoryFile(slug string) string      { return filepath.Join(p.effort(slug), "memory.md") }
func (p paths) managedWorktree(slug string) string { return filepath.Join(p.worktrees, slug) }
func memoryPublicPath(slug string) string {
	return filepath.ToSlash(filepath.Join(".awf", "efforts", slug, "memory.md"))
}

// publicMemoryPath reports the owned memory path resolvably: the historical
// repository-relative form from the primary root, an absolute
// primary-root-qualified path from anywhere else (ADR-0189).
func (p paths) publicMemoryPath(slug string) string {
	if filepath.Clean(p.roots.InvokingRoot) == filepath.Clean(p.roots.PrimaryRoot) {
		return memoryPublicPath(slug)
	}
	return p.memoryFile(slug)
}
