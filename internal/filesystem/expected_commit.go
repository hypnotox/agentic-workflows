package filesystem

import (
	"fmt"
	"io/fs"
	"os"
	"path"
)

func exchangeExpected(root *os.Root, temporary, destination string, expected fs.FileInfo, remove bool) (bool, error) {
	parent := path.Dir(destination)
	if path.Dir(temporary) != parent {
		return false, fmt.Errorf("filesystem: expected mutation paths have different parents")
	}
	parentRoot, err := root.OpenRoot(parent)
	if err != nil {
		return false, fmt.Errorf("filesystem: open atomic parent %q: %w", parent, err)
	}
	defer parentRoot.Close()
	anchor, err := parentRoot.Open(".")
	if err != nil { // coverage-ignore: the newly opened parent remains live until its deferred close
		return false, fmt.Errorf("filesystem: open atomic parent anchor %q: %w", parent, err) // coverage-ignore: the newly opened parent remains live until its deferred close
	}
	defer anchor.Close()
	return exchangeExpectedAnchored(parentRoot, anchor, path.Base(temporary), path.Base(destination), expected, remove)
}
