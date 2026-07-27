package effort

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func memorySkeleton(id string) []byte {
	return []byte("Effort: " + id + "\nRoute: unselected\nPhase: unselected\nWorkflow: unselected\nNext: Record the next concrete action.\n\n## Brief\n\n## Decisions\n\n## Handoff log\n")
}

func (p paths) createMemory(id string) (string, error) {
	if err := p.ensure(p.memory); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return "", err
	}
	path := p.memoryFile(id)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		raw, readErr := os.ReadFile(path)
		if readErr != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
			return "", fmt.Errorf("inspect existing memory: %w", readErr)
		}
		if strings.HasPrefix(string(raw), "Effort: "+id+"\n") {
			return path, nil
		}
		return "", fmt.Errorf("refuse existing non-owned memory file %s", path)
	}
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return "", fmt.Errorf("create memory: %w", err)
	}
	if _, err := file.Write(memorySkeleton(id)); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Sync(); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return "", err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return "", err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return "", err
	}
	return path, nil
}

func (p paths) memoryTruth(id string) (bool, error) {
	path := p.memoryFile(id)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("unsafe memory path %s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return false, err
	}
	if !strings.HasPrefix(string(raw), "Effort: "+id+"\n") {
		return false, fmt.Errorf("memory file %s is not owned by effort %s", path, id)
	}
	return true, nil
}
