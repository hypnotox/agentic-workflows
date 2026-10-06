package effortfs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Root finds the main Git checkout for shared effort memory. Commands run from
// a checkout root; without Git or a .git entry, memory stays local to that root.
func Root(root string) (string, error) {
	if exists, err := pathExists(filepath.Join(root, ".git")); err != nil {
		return "", err
	} else if !exists {
		return root, nil
	}

	output, err := git(root, "worktree", "list", "--porcelain", "-z")
	if errors.Is(err, exec.ErrNotFound) {
		return root, nil
	}
	if err != nil {
		return "", err
	}
	first, _, _ := bytes.Cut(output, []byte{0})
	primary, ok := bytes.CutPrefix(first, []byte("worktree "))
	if !ok || len(primary) == 0 {
		return "", fmt.Errorf("git worktree list did not report a main checkout")
	}
	return string(primary), nil
}

// Worktree describes a newly created checkout and its coordinating memory.
type Worktree struct {
	Path       string
	Branch     string
	MemoryPath string
}

// AddWorktree creates a sibling checkout from the primary checkout's HEAD.
// An empty suffix selects the default checkout; explicit suffixes cannot be default.
func AddWorktree(primary, slug, suffix string) (Worktree, error) {
	if err := validateSlug(slug); err != nil {
		return Worktree{}, err
	}
	component := "default"
	if suffix != "" {
		if suffix == component {
			return Worktree{}, fmt.Errorf("worktree suffix %q is reserved; omit --suffix for the default checkout", suffix)
		}
		if err := validateSlug(suffix); err != nil {
			return Worktree{}, fmt.Errorf("invalid worktree suffix: %w", err)
		}
		component = suffix
	}

	memoryPath := filepath.Join(primary, activePath(slug), memoryName)
	info, err := os.Lstat(memoryPath)
	if err != nil {
		return Worktree{}, fmt.Errorf("inspect active effort %q memory: %w", slug, err)
	}
	if !info.Mode().IsRegular() {
		return Worktree{}, fmt.Errorf("active effort %q requires a regular memory.md file", slug)
	}

	path := filepath.Join(primary, ".awf", "worktrees", slug, component)
	if exists, err := pathExists(path); err != nil {
		return Worktree{}, fmt.Errorf("inspect worktree destination: %w", err)
	} else if exists {
		return Worktree{}, fmt.Errorf("worktree destination %q already exists", path)
	}
	branch := "awf/" + slug + "/" + component
	if _, err := git(primary, "worktree", "add", "-b", branch, path, "HEAD"); err != nil {
		return Worktree{}, err
	}
	return Worktree{Path: path, Branch: branch, MemoryPath: memoryPath}, nil
}

func git(root string, args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s in %q: %w: %s", strings.Join(args, " "), root, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
