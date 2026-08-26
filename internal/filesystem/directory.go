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
func (h *Handle) CreateDirectory(destination string, mode fs.FileMode) (created fs.FileInfo, returnErr error) {
	if err := validPath(destination); err != nil {
		return nil, fmt.Errorf("filesystem: create directory %q: %w", destination, err)
	}
	temporary := ""
	createdTemporary := false
	for range 100 {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil { // coverage-ignore: the operating-system random source is required by the Go runtime and cannot be faulted through this seam
			return nil, fmt.Errorf("filesystem: name directory temporary for %q: %w", destination, err)
		}
		temporary = path.Join(path.Dir(destination), ".awf-directory-"+hex.EncodeToString(token[:]))
		if err := h.root.Mkdir(temporary, mode); errors.Is(err, fs.ErrExist) { // coverage-ignore: a collision requires predicting the cryptographically random temporary name before this operation
			continue // coverage-ignore: a collision requires predicting the cryptographically random temporary name before this operation
		} else if err != nil {
			return nil, fmt.Errorf("filesystem: create directory temporary for %q: %w", destination, err)
		}
		createdTemporary = true
		break
	}
	if !createdTemporary { // coverage-ignore: exhausting 100 cryptographically random temporary names cannot be induced through the concrete production handle
		return nil, fmt.Errorf("filesystem: create directory temporary for %q: temporary name collisions exhausted", destination) // coverage-ignore: exhausting 100 cryptographically random temporary names cannot be induced through the concrete production handle
	}
	defer func() {
		if temporary != "" {
			if err := h.root.Remove(temporary); err != nil && !errors.Is(err, fs.ErrNotExist) { // coverage-ignore: cleanup fails only after a concurrent namespace change or storage fault
				returnErr = errors.Join(returnErr, fmt.Errorf("filesystem: remove directory temporary %q: %w", temporary, err)) // coverage-ignore: cleanup fails only after a concurrent namespace change or storage fault
			}
		}
	}()
	directory, err := h.root.Open(temporary)
	if err != nil { // coverage-ignore: opening the just-created unpredictable temporary fails only after a concurrent namespace change or storage fault
		return nil, fmt.Errorf("filesystem: open created directory for %q: %w", destination, err) // coverage-ignore: opening the just-created unpredictable temporary fails only after a concurrent namespace change or storage fault
	}
	created, statErr := directory.Stat()
	closeErr := directory.Close()
	if statErr != nil || closeErr != nil { // coverage-ignore: stat or close of the just-opened directory requires a storage fault
		return nil, fmt.Errorf("filesystem: identify created directory for %q: %w", destination, errors.Join(statErr, closeErr)) // coverage-ignore: stat or close of the just-opened directory requires a storage fault
	}
	if err := publishDirectoryNoReplace(h.root, temporary, destination); err != nil {
		return nil, fmt.Errorf("filesystem: publish directory without replacement to %q: %w", destination, err)
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
	if err != nil { // coverage-ignore: the newly opened parent remains live until its deferred close
		return fmt.Errorf("filesystem: open directory publication anchor %q: %w", parent, err) // coverage-ignore: the newly opened parent remains live until its deferred close
	}
	defer anchor.Close()
	return filepublication.MoveNoReplaceAt(anchor, path.Base(temporary), path.Base(destination))
}
