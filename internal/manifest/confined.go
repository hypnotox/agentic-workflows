package manifest

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
)

// LiveFile is one safely captured regular lock file and its parsed live model.
type LiveFile struct {
	Lock    *Lock
	Content []byte
	Mode    fs.FileMode
}

// LoadLiveFileOptional reads a live lock through a root-confined, no-follow,
// nonblocking regular-file boundary and parses exactly those captured bytes.
func LoadLiveFileOptional(root, path string, floor, current int) (LiveFile, bool, error) {
	content, mode, found, err := readConfinedOptional(root, path)
	if err != nil {
		return LiveFile{}, false, unreadableConfinedLock(err)
	}
	if !found {
		return LiveFile{}, false, nil
	}
	lock, err := ParseLive(content, floor, current)
	if err != nil {
		return LiveFile{}, false, unreadableConfinedLock(err)
	}
	return LiveFile{Lock: lock, Content: content, Mode: mode.Perm()}, true, nil
}

// LoadSchemaConfinedOptional reads only the schema stamp through the same safe
// regular-file boundary used for full live authority capture.
func LoadSchemaConfinedOptional(root, path string) (int, bool, error) {
	content, _, found, err := readConfinedOptional(root, path)
	if err != nil {
		return 0, false, unreadableConfinedLock(err)
	}
	if !found {
		return 0, false, nil
	}
	schema, err := parseSchemaVersion(content)
	if err != nil {
		return 0, false, unreadableConfinedLock(fmt.Errorf("parse lock: %w", err))
	}
	return schema, true, nil
}

func readConfinedOptional(root, path string) (content []byte, mode fs.FileMode, found bool, returnErr error) {
	files, err := filesystem.Open(root)
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { returnErr = errors.Join(returnErr, files.Close()) }()
	expected, err := files.ExpectedIdentity(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	defer expected.Release() //nolint:errcheck // read-only authority capture owns no mutation
	content, mode, err = files.ReadExpected(path, expected)
	if err != nil {
		return nil, 0, false, err
	}
	return content, mode.Perm(), true, nil
}

func unreadableConfinedLock(err error) error {
	return fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
}
