package effort

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func memorySkeleton(id string) []byte {
	return []byte("Effort: " + id + "\nRoute: unselected\nPhase: unselected\nWorkflow: unselected\nNext: Record the next concrete action.\n\n## Brief\n\n## Decisions\n\n## Handoff log\n")
}

func (p paths) createMemory(id string) (string, error) {
	if err := p.ensure(p.memory); err != nil {
		return "", fmt.Errorf("prepare memory directory %s: %w", p.memory, err)
	}
	path := p.memoryFile(id)
	raw, _, err := readRegularNoFollow(path)
	if err == nil {
		if strings.HasPrefix(string(raw), "Effort: "+id+"\n") {
			return path, nil
		}
		return "", fmt.Errorf("refuse existing non-owned memory file %s", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect memory destination %s: %w", path, err)
	}
	if err := atomicReplaceFS(p.filesystem(), path, memorySkeleton(id), nil); err != nil {
		return "", fmt.Errorf("publish memory file %s: %w", path, err)
	}
	return path, nil
}

func (p paths) memoryTruth(id string) (bool, error) {
	if err := p.validate(p.memory); err != nil {
		return false, fmt.Errorf("validate memory resident root before truth read: %w", err)
	}
	path := p.memoryFile(id)
	raw, _, err := readRegularNoFollow(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect memory truth at %s: %w", path, err)
	}
	if !strings.HasPrefix(string(raw), "Effort: "+id+"\n") {
		return false, fmt.Errorf("memory file %s is not owned by effort %s", path, id)
	}
	return true, nil
}
