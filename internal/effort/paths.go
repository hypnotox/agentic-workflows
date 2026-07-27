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
	memory    string
	worktrees string
	assign    string
	fs        fileSystem
}

func resolvePaths(roots awfgit.ControlRoots) (paths, error) {
	efforts, err := roots.ResidentRoot(awfgit.ResidentEfforts)
	if err != nil {
		return paths{}, fmt.Errorf("resolve efforts resident root: %w", err)
	}
	memory, err := roots.ResidentRoot(awfgit.ResidentMemory)
	if err != nil {
		return paths{}, fmt.Errorf("resolve memory resident root: %w", err)
	}
	worktrees, err := roots.ResidentRoot(awfgit.ResidentWorktrees)
	if err != nil {
		return paths{}, fmt.Errorf("resolve worktrees resident root: %w", err)
	}
	assign, err := roots.ResidentRoot(awfgit.ResidentAssignments)
	if err != nil {
		return paths{}, fmt.Errorf("resolve assignments resident root: %w", err)
	}
	return paths{roots: roots, efforts: efforts, memory: memory, worktrees: worktrees, assign: assign}, nil
}

func (p paths) ensure(root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create resident root %s: %w", root, err)
	}
	info, err := os.Lstat(root)
	if err != nil { // coverage-ignore: MkdirAll just proved this exact path exists; only a concurrent namespace race can make the adjacent lstat fail
		return fmt.Errorf("inspect resident root %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 { // coverage-ignore: MkdirAll rejects a symlink leaf before this check; ancestor symlinks are rejected by ResidentRoot revalidation
		return &awfgit.HardSafetyError{Category: "symlink", Path: root}
	}
	if !info.IsDir() { // coverage-ignore: MkdirAll rejects a non-directory leaf before this check
		return &awfgit.HardSafetyError{Category: "file-type", Path: root, Err: fmt.Errorf("mode %s is not a directory", info.Mode())}
	}
	if info.Mode().Perm()&0o022 != 0 {
		return &awfgit.HardSafetyError{Category: "resident-permissions", Path: root, Err: fmt.Errorf("mode is %o, group/world write bits must be clear", info.Mode().Perm())}
	}
	// Re-run the control-root proof after creation so a symlink race is never
	// accepted merely because MkdirAll returned success.
	return p.validate(root)
}

func (p paths) validate(root string) error {
	var name awfgit.ResidentName
	switch root {
	case p.efforts:
		name = awfgit.ResidentEfforts
	case p.memory:
		name = awfgit.ResidentMemory
	case p.worktrees:
		name = awfgit.ResidentWorktrees
	case p.assign:
		name = awfgit.ResidentAssignments
	default:
		return fmt.Errorf("unknown resident root %s", root)
	}
	resolved, err := p.roots.ResidentRoot(name)
	if err != nil {
		return fmt.Errorf("revalidate resident root %s: %w", root, err)
	}
	if filepath.Clean(resolved) != filepath.Clean(root) { // coverage-ignore: the closed root-to-ResidentName switch maps each input back to the identical cleaned path
		return fmt.Errorf("resident root changed from %s to %s", root, resolved)
	}
	return nil
}

func (p paths) filesystem() fileSystem {
	if p.fs == nil {
		return osFileSystem{}
	}
	return p.fs
}

func (p paths) record(id string) string          { return filepath.Join(p.efforts, id+".json") }
func (p paths) memoryFile(id string) string      { return filepath.Join(p.memory, id+".md") }
func (p paths) managedWorktree(id string) string { return filepath.Join(p.worktrees, id) }
func (p paths) assignments() string              { return filepath.Join(p.assign, "sessions.json") }
