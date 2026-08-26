//go:build linux

package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"golang.org/x/sys/unix"
)

func exchangeExpected(root *os.Root, temporary, destination string, expected fs.FileInfo, remove bool) (bool, error) {
	parentPath := path.Dir(destination)
	parent, err := root.Open(parentPath)
	if err != nil {
		return false, fmt.Errorf("filesystem: open atomic parent %q: %w", parentPath, err)
	}
	defer parent.Close()
	temporaryLeaf, destinationLeaf := path.Base(temporary), path.Base(destination)
	exchange := func() error {
		return unix.Renameat2(int(parent.Fd()), temporaryLeaf, int(parent.Fd()), destinationLeaf, unix.RENAME_EXCHANGE)
	}
	if err := exchange(); err != nil {
		return false, fmt.Errorf("filesystem: exchange expected entry %q: %w", destination, err)
	}
	displaced, inspectErr := root.Lstat(temporary)
	if inspectErr != nil || !os.SameFile(expected, displaced) {
		mismatch := ErrIdentityChanged
		if inspectErr != nil {
			mismatch = errors.Join(mismatch, inspectErr)
		}
		if rollbackErr := exchange(); rollbackErr != nil {
			cause := errors.Join(mismatch, fmt.Errorf("restore unexpected entry at %q: %w", destination, rollbackErr))
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
		}
		return false, mismatch
	}
	if err := root.Remove(temporary); err != nil {
		if rollbackErr := exchange(); rollbackErr != nil {
			cause := errors.Join(err, fmt.Errorf("restore expected entry at %q: %w", destination, rollbackErr))
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
		}
		return false, err
	}
	if remove {
		if err := root.Remove(destination); err != nil {
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: destination, Cause: err}
		}
	}
	return true, nil
}
