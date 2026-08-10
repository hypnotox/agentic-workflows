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
	publication, err := prepare(path, contents, mode)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, publication.cleanup())
	}()
	return publication.publish()
}

type prepared struct {
	temporary string
	path      string
}

func prepare(path string, contents []byte, mode fs.FileMode) (_ *prepared, returnErr error) {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".filepublication-*.tmp")
	if err != nil { // coverage-ignore: a same-directory temporary creation failure requires a storage or permission fault
		return nil, fmt.Errorf("create publication temporary for %s: %w", path, err) // coverage-ignore: a same-directory temporary creation failure requires a storage or permission fault
	}
	publication := &prepared{temporary: temporary.Name(), path: path}
	closed := false
	defer func() {
		if !closed { // coverage-ignore: a deferred close is only needed after a preparation failure
			returnErr = errors.Join(returnErr, temporary.Close()) // coverage-ignore: deferred close failure requires a storage fault
		}
		if returnErr != nil { // coverage-ignore: preparation cleanup follows only a local file preparation fault
			returnErr = errors.Join(returnErr, publication.cleanup()) // coverage-ignore: preparation cleanup follows only a local file preparation fault
		}
	}()
	if err := temporary.Chmod(mode); err != nil { // coverage-ignore: chmod of a locally-created file requires a storage fault
		return nil, fmt.Errorf("set publication mode for %s: %w", path, err) // coverage-ignore: chmod of a locally-created file requires a storage fault
	}
	if n, err := temporary.Write(contents); err != nil { // coverage-ignore: a local temporary write failure requires a storage fault
		return nil, fmt.Errorf("write publication temporary for %s: %w", path, err) // coverage-ignore: a local temporary write failure requires a storage fault
	} else if n != len(contents) { // coverage-ignore: os.File.Write returns a non-nil error when it writes fewer bytes than requested
		return nil, fmt.Errorf("write publication temporary for %s: %w", path, io.ErrShortWrite) // coverage-ignore: os.File.Write returns a non-nil error when it writes fewer bytes than requested
	}
	if err := temporary.Sync(); err != nil { // coverage-ignore: a local temporary sync failure requires a storage fault
		return nil, fmt.Errorf("sync publication temporary for %s: %w", path, err) // coverage-ignore: a local temporary sync failure requires a storage fault
	}
	if err := temporary.Close(); err != nil { // coverage-ignore: closing a local temporary after successful preparation requires a storage fault
		return nil, fmt.Errorf("close publication temporary for %s: %w", path, err) // coverage-ignore: closing a local temporary after successful preparation requires a storage fault
	}
	closed = true
	return publication, nil
}

func (p *prepared) publish() error {
	if err := publishNoReplace(p.temporary, p.path); err != nil {
		return fmt.Errorf("publish complete file without replacement to %s: %w", p.path, err)
	}
	p.temporary = ""
	return nil
}

func (p *prepared) cleanup() error {
	if p.temporary == "" {
		return nil
	}
	if err := os.Remove(p.temporary); err != nil && !errors.Is(err, os.ErrNotExist) { // coverage-ignore: a locally-created temporary cleanup failure requires a storage fault
		return fmt.Errorf("remove publication temporary %s: %w", p.temporary, err) // coverage-ignore: locally-created temporary cleanup failure requires a storage fault
	}
	p.temporary = ""
	return nil
}
