//go:build linux

package effort

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// publishAtomic consumes tempPath with an atomic no-replace creation or an
// exchange whose displaced inode proves the expected replacement identity.
func publishAtomic(tempPath, path string, expected *fileIdentity) error {
	if expected == nil {
		if err := unix.Renameat2(unix.AT_FDCWD, tempPath, unix.AT_FDCWD, path, unix.RENAME_NOREPLACE); err != nil {
			return err
		}
		return nil
	}
	if err := unix.Renameat2(unix.AT_FDCWD, tempPath, unix.AT_FDCWD, path, unix.RENAME_EXCHANGE); err != nil {
		return err
	}
	displaced, err := lstatRegular(tempPath)
	if err == nil && os.SameFile(expected.info, displaced.info) {
		return nil
	}
	mismatch := err
	if mismatch == nil {
		mismatch = safety("identity", path, errors.New("destination changed before atomic publication"))
	}
	if rollbackErr := unix.Renameat2(unix.AT_FDCWD, tempPath, unix.AT_FDCWD, path, unix.RENAME_EXCHANGE); rollbackErr != nil { // coverage-ignore: requires a second namespace race or kernel fault during immediate rollback
		return errors.Join(mismatch, fmt.Errorf("restore unexpected destination at %s after refused publication: %w", path, rollbackErr))
	}
	return mismatch
}
