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
}

func resolvePaths(roots awfgit.ControlRoots) (paths, error) {
	efforts, err := roots.ResidentRoot(awfgit.ResidentEfforts)
	if err != nil {
		return paths{}, err
	}
	memory, err := roots.ResidentRoot(awfgit.ResidentMemory)
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return paths{}, err
	}
	worktrees, err := roots.ResidentRoot(awfgit.ResidentWorktrees)
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return paths{}, err
	}
	assign, err := roots.ResidentRoot(awfgit.ResidentAssignments)
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return paths{}, err
	}
	return paths{roots: roots, efforts: efforts, memory: memory, worktrees: worktrees, assign: assign}, nil
}

func (p paths) ensure(root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("create resident root %s: %w", root, err)
	}
	if err := os.Chmod(root, 0o700); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("secure resident root %s: %w", root, err)
	}
	// Re-run the control-root proof after creation so a symlink race is never
	// accepted merely because MkdirAll returned success.
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
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return err
	}
	if filepath.Clean(resolved) != filepath.Clean(root) { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("resident root changed from %s to %s", root, resolved)
	}
	return nil
}

func (p paths) record(id string) string          { return filepath.Join(p.efforts, id+".json") }
func (p paths) memoryFile(id string) string      { return filepath.Join(p.memory, id+".md") }
func (p paths) managedWorktree(id string) string { return filepath.Join(p.worktrees, id) }
func (p paths) assignments() string              { return filepath.Join(p.assign, "sessions.json") }
