package effort

import "github.com/hypnotox/agentic-workflows/internal/filepublication"

func moveDirectoryNoReplace(fromPath, toPath string) error {
	return filepublication.MoveNoReplace(fromPath, toPath)
}
