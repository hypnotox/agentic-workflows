package filesystem

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

// CreateDirectory atomically publishes one new directory and returns the
// identity of the directory it created. It refuses an existing destination.
func (h *Handle) CreateDirectory(destination string, mode fs.FileMode) (created *ExpectedIdentity, returnErr error) {
	if err := validPath(destination); err != nil {
		return nil, fmt.Errorf("filesystem: create directory %q: %w", destination, err)
	}
	temporary := ""
	createdTemporary := false
	for range 100 {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return nil, fmt.Errorf("filesystem: name directory temporary for %q: %w", destination, err)
		}
		temporary = path.Join(path.Dir(destination), ".awf-directory-"+hex.EncodeToString(token[:]))
		if err := h.root.Mkdir(temporary, mode); errors.Is(err, fs.ErrExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("filesystem: create directory temporary for %q: %w", destination, err)
		}
		createdTemporary = true
		break
	}
	if !createdTemporary {
		return nil, fmt.Errorf("filesystem: create directory temporary for %q: temporary name collisions exhausted", destination)
	}
	defer func() {
		if temporary != "" {
			if err := h.root.Remove(temporary); err != nil && !errors.Is(err, fs.ErrNotExist) {
				returnErr = errors.Join(returnErr, fmt.Errorf("filesystem: remove directory temporary %q: %w", temporary, err))
			}
		}
	}()
	created, err := h.ExpectedIdentity(temporary)
	if err != nil {
		return nil, fmt.Errorf("filesystem: identify created directory for %q: %w", destination, err)
	}
	if err := publishDirectoryNoReplace(h.root, temporary, destination); err != nil {
		return nil, errors.Join(fmt.Errorf("filesystem: publish directory without replacement to %q: %w", destination, err), created.Release())
	}
	temporary = ""
	return created, nil
}

func publishDirectoryNoReplace(root *os.Root, temporary, destination string) error {
	parent := path.Dir(destination)
	if path.Dir(temporary) != parent {
		return fmt.Errorf("filesystem: directory publication paths have different parents")
	}
	parentRoot, err := root.OpenRoot(parent)
	if err != nil {
		return fmt.Errorf("filesystem: open directory publication parent %q: %w", parent, err)
	}
	defer parentRoot.Close()
	anchor, err := parentRoot.Open(".")
	if err != nil {
		return fmt.Errorf("filesystem: open directory publication anchor %q: %w", parent, err)
	}
	defer anchor.Close()
	return filepublication.MoveNoReplaceAt(anchor, path.Base(temporary), path.Base(destination))
}
