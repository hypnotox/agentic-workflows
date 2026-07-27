package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type RefusalError struct {
	Category, Risk string
	Forceable      bool
}

func (e *RefusalError) Error() string {
	if e.Risk == "" {
		return "worktree refusal (" + e.Category + ")"
	}
	return "worktree refusal (" + e.Category + "): " + e.Risk
}

type PartialMutationError struct {
	EffortID, Repair string
	Err              error
}

func (e *PartialMutationError) Error() string {
	message := fmt.Sprintf("effort %s has partial Git mutation; repair with awf effort repair %s (%s)", e.EffortID, e.EffortID, e.Repair)
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e *PartialMutationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type registration struct {
	path, head, branch string
	detached, bare     bool
}

func registrations(ctx context.Context, run Runner, invoking string) ([]registration, error) {
	out, err := run(ctx, invoking, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	if len(out) == 0 || out[len(out)-1] != 0 {
		return nil, errors.New("invalid worktree registration porcelain")
	}
	var result []registration
	var current *registration
	for _, field := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if field == "" {
			if current == nil || current.path == "" {
				return nil, errors.New("invalid worktree registration")
			}
			result = append(result, *current)
			current = nil
			continue
		}
		key, val, has := strings.Cut(field, " ")
		if key == "worktree" {
			if current != nil || !has || val == "" {
				return nil, errors.New("invalid worktree registration")
			}
			current = &registration{path: filepath.Clean(val)}
			continue
		}
		if current == nil {
			return nil, errors.New("invalid worktree registration")
		}
		switch key {
		case "HEAD":
			current.head = val
		case "branch":
			current.branch = val
		case "detached":
			current.detached = true
		case "bare":
			current.bare = true
		case "prunable":
		default:
			return nil, fmt.Errorf("unknown worktree field %q", key)
		}
	}
	if current != nil {
		return nil, errors.New("unterminated worktree registration")
	}
	return result, nil
}
func exactRegistration(ctx context.Context, run Runner, invoking, path, branch string) error {
	regs, err := registrations(ctx, run, invoking)
	if err != nil {
		return err
	}
	var found []registration
	for _, r := range regs {
		if filepath.Clean(r.path) == filepath.Clean(path) {
			found = append(found, r)
		}
		if r.branch == branch && filepath.Clean(r.path) != filepath.Clean(path) {
			return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("managed branch is registered elsewhere")}
		}
	}
	if len(found) != 1 {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("managed path is not uniquely registered")}
	}
	r := found[0]
	if r.branch != branch || r.detached || r.bare {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("managed registration branch mismatch")}
	}
	return nil
}
func safeManagedPath(path string) error {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	remainder := strings.TrimPrefix(strings.TrimPrefix(clean, volume), string(filepath.Separator))
	current := volume + string(filepath.Separator)
	var info os.FileInfo
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		componentInfo, componentErr := os.Lstat(current)
		if componentErr != nil {
			return componentErr
		}
		info = componentInfo
		if componentInfo.Mode()&os.ModeSymlink != 0 {
			return &awfgit.HardSafetyError{Category: "symlink", Path: current}
		}
	}
	if info == nil {
		return errors.New("managed path has no components")
	}
	if !info.IsDir() {
		return &awfgit.HardSafetyError{Category: "file-type", Path: path}
	}
	return nil
}
