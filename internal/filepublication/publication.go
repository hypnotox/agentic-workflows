// Package filepublication owns complete same-directory file preparation and atomic no-replace publication.
package filepublication

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Publish writes contents with mode to a same-directory temporary file and atomically creates path only when it is absent.
func Publish(path string, contents []byte, mode fs.FileMode) (returnErr error) {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".filepublication-*.tmp")
	if err != nil {
		return fmt.Errorf("create publication temporary for %s: %w", path, err) // coverage-ignore: a same-directory temporary creation failure requires a storage or permission fault
	}
	temporaryPath := temporary.Name()
	closed, published := false, false
	defer func() {
		if !closed { // coverage-ignore: a deferred close is only needed after a preparation failure
			returnErr = errors.Join(returnErr, temporary.Close()) // coverage-ignore: deferred close failure requires a storage fault
		}
		if !published {
			if err := os.Remove(temporaryPath); err != nil && !os.IsNotExist(err) { // coverage-ignore: a locally-created temporary cleanup failure requires a storage fault
				returnErr = errors.Join(returnErr, fmt.Errorf("remove publication temporary %s: %w", temporaryPath, err)) // coverage-ignore: locally-created temporary cleanup failure requires a storage fault
			}
		}
	}()
	if err := temporary.Chmod(mode); err != nil { // coverage-ignore: chmod of a locally-created file requires a storage fault
		return fmt.Errorf("set publication mode for %s: %w", path, err) // coverage-ignore: chmod of a locally-created file requires a storage fault
	}
	if n, err := temporary.Write(contents); err != nil { // coverage-ignore: a local temporary write failure requires a storage fault
		return fmt.Errorf("write publication temporary for %s: %w", path, err) // coverage-ignore: a local temporary write failure requires a storage fault
	} else if n != len(contents) { // coverage-ignore: os.File.Write returns a non-nil error when it writes fewer bytes than requested
		return fmt.Errorf("write publication temporary for %s: %w", path, io.ErrShortWrite) // coverage-ignore: os.File.Write returns a non-nil error when it writes fewer bytes than requested
	}
	if err := temporary.Sync(); err != nil { // coverage-ignore: a local temporary sync failure requires a storage fault
		return fmt.Errorf("sync publication temporary for %s: %w", path, err) // coverage-ignore: a local temporary sync failure requires a storage fault
	}
	if err := temporary.Close(); err != nil { // coverage-ignore: closing a local temporary after successful preparation requires a storage fault
		return fmt.Errorf("close publication temporary for %s: %w", path, err) // coverage-ignore: closing a local temporary after successful preparation requires a storage fault
	}
	closed = true
	if err := publishNoReplace(temporaryPath, path); err != nil {
		return fmt.Errorf("publish complete file without replacement to %s: %w", path, err)
	}
	published = true
	return nil
}
