//go:build darwin

package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
	"golang.org/x/sys/unix"
)

// exchangeExpectedAnchored performs the platform-native leaf exchange.
//
// The common expected-mutation owner resolves the immediate parent through
// os.Root, opens its stable anchor, and supplies only final path components
// before this platform helper runs. Keeping intermediate path resolution out
// of the native syscall prevents a replaced parent symlink from redirecting
// the exchange outside the selected root while preserving one native owner
// for each released platform's atomic replacement operation.
func exchangeExpectedAnchored(root *os.Root, anchor *os.File, temporary, destination string, expected fs.FileInfo, remove, retain bool) (bool, error) {
	exchange := func() error {
		return unix.RenameatxNp(int(anchor.Fd()), temporary, int(anchor.Fd()), destination, unix.RENAME_SWAP)
	}
	if err := exchange(); err != nil {
		return false, fmt.Errorf("filesystem: exchange expected entry %q: %w", destination, err)
	}
	displaced, inspectErr := root.Lstat(temporary)
	if inspectErr != nil || !os.SameFile(expected, displaced) {
		mismatch := error(ErrIdentityChanged)
		if inspectErr != nil {
			mismatch = errors.Join(mismatch, inspectErr)
		}
		if rollbackErr := exchange(); rollbackErr != nil {
			cause := errors.Join(mismatch, fmt.Errorf("restore unexpected entry at %q: %w", destination, rollbackErr))
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
		}
		return false, mismatch
	}
	if retain {
		if err := root.Remove(destination); err != nil {
			if rollbackErr := exchange(); rollbackErr != nil {
				cause := errors.Join(err, fmt.Errorf("restore expected entry at %q: %w", destination, rollbackErr))
				return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
			}
			cause := fmt.Errorf("remove retirement reservation before restoring expected entry: %w", err)
			return true, &filepublication.CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cause}
		}
		return true, nil
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
