//go:build darwin

package effort

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// publishAtomic uses Darwin's exclusive rename for creation and swap rename
// for identity-checked replacement.
func publishAtomic(tempPath, path string, expected *fileIdentity) error {
	if expected == nil {
		return unix.RenamexNp(tempPath, path, unix.RENAME_EXCL)
	}
	if err := unix.RenamexNp(tempPath, path, unix.RENAME_SWAP); err != nil {
		return err
	}
	displaced, err := lstatRegular(tempPath)
	mismatch := unexpectedPublicationIdentity(path, expected, displaced, err)
	if mismatch == nil {
		return nil
	}
	if rollbackErr := unix.RenamexNp(tempPath, path, unix.RENAME_SWAP); rollbackErr != nil { // coverage-ignore: requires a second namespace race or kernel fault during immediate rollback
		return errors.Join(mismatch, fmt.Errorf("restore unexpected destination at %s after refused publication: %w", path, rollbackErr))
	}
	return mismatch
}
