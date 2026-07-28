package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/effort"
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
	path, head, branch       string
	detached, bare, prunable bool
}

func registrations(ctx context.Context, run Runner, invoking string) ([]registration, error) {
	out, err := run(ctx, invoking, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	if len(out) < 2 || !strings.HasSuffix(string(out), "\x00\x00") {
		return nil, errors.New("unterminated worktree registration porcelain")
	}
	var result []registration
	for _, chunk := range strings.Split(string(out[:len(out)-2]), "\x00\x00") {
		if chunk == "" {
			return nil, errors.New("invalid worktree registration")
		}
		fields := strings.Split(chunk, "\x00")
		current := registration{}
		head, branch, detached := false, false, false
		for _, field := range fields {
			key, val, has := strings.Cut(field, " ")
			switch key {
			case "worktree":
				if !has || val == "" || current.path != "" || current.bare {
					return nil, errors.New("invalid worktree registration")
				}
				current.path = filepath.Clean(val)
			case "HEAD":
				if !has || val == "" || head || current.bare {
					return nil, errors.New("invalid worktree registration")
				}
				head = true
				current.head = val
			case "branch":
				if !has || val == "" || branch || current.bare {
					return nil, errors.New("invalid worktree registration")
				}
				branch = true
				current.branch = val
			case "detached":
				if has || detached || current.bare {
					return nil, errors.New("invalid worktree registration")
				}
				detached = true
				current.detached = true
			case "bare":
				if has || current.bare || current.path != "" || head || branch || detached || current.prunable {
					return nil, errors.New("invalid worktree registration")
				}
				current.bare = true
			case "prunable":
				if !has || val == "" || current.prunable || current.bare {
					return nil, errors.New("invalid worktree registration")
				}
				current.prunable = true
			default:
				return nil, fmt.Errorf("unknown worktree field %q", key)
			}
		}
		if !current.bare && (current.path == "" || !head || branch == detached) {
			return nil, errors.New("invalid worktree registration")
		}
		result = append(result, current)
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

var managedLstat = os.Lstat
var managedOwner = effort.ValidateCurrentOwner

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
		componentInfo, componentErr := managedLstat(current)
		if componentErr != nil {
			return componentErr
		}
		info = componentInfo
		if componentInfo.Mode()&os.ModeSymlink != 0 {
			return &awfgit.HardSafetyError{Category: "symlink", Path: current}
		}
		// Shared sticky parents such as /tmp are not manager-owned resident
		// components; validate ownership once the confined path enters its own
		// tree.
		if componentInfo.Mode()&os.ModeSticky == 0 || componentInfo.Mode().Perm()&0o002 == 0 {
			if err := managedOwner(current, componentInfo); err != nil {
				return err
			}
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
