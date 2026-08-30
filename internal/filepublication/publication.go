// Package filepublication owns complete same-directory file preparation and released-platform atomic no-replace namespace publication.
package filepublication

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// MoveNoReplace atomically moves one filesystem entry to an absent destination.
// It never copies, traverses, or replaces either entry.
func MoveNoReplace(fromPath, toPath string) error {
	return publishNoReplace(fromPath, toPath)
}

// CommittedCleanupError reports that destination was committed but its
// same-directory publication temporary could not be removed.
type CommittedCleanupError struct {
	DestinationPath string
	ResiduePath     string
	Cause           error
}

func (e *CommittedCleanupError) Error() string {
	return fmt.Sprintf("publication committed to %s but cleanup residue %s remains: %v", e.DestinationPath, e.ResiduePath, e.Cause)
}

// Unwrap preserves the cleanup failure identity.
func (e *CommittedCleanupError) Unwrap() error { return e.Cause }

type publicationBackend interface {
	openTemporary(string) (string, *os.File, error)
	publishNoReplace(string, string) error
	remove(string) error
}

type hostBackend struct{}

func (hostBackend) openTemporary(destination string) (string, *os.File, error) {
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".filepublication-*.tmp")
	if err != nil {
		return "", nil, err
	}
	return temporary.Name(), temporary, nil
}

func (hostBackend) publishNoReplace(temporary, destination string) error {
	return publishNoReplace(temporary, destination)
}

func (hostBackend) remove(temporary string) error { return os.Remove(temporary) }

// Publish writes contents with mode to a same-directory temporary file and atomically creates path only when it is absent.
func Publish(path string, contents []byte, mode fs.FileMode) error {
	return publish(hostBackend{}, path, contents, mode)
}

type confinedRoot interface {
	OpenFile(string, int, os.FileMode) (*os.File, error)
	Link(string, string) error
	Remove(string) error
}

type confinedBackend struct {
	root confinedRoot
}

func (b confinedBackend) openTemporary(destination string) (string, *os.File, error) {
	for range 100 {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return "", nil, fmt.Errorf("name publication temporary for %s: %w", destination, err)
		}
		temporary := path.Join(path.Dir(destination), ".filepublication-"+hex.EncodeToString(token[:])+".tmp")
		file, err := b.root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		return temporary, file, nil
	}
	return "", nil, errors.New("temporary name collisions exhausted")
}

func (b confinedBackend) publishNoReplace(temporary, destination string) error {
	return b.root.Link(temporary, destination)
}

func (b confinedBackend) remove(temporary string) error { return b.root.Remove(temporary) }

// PublishConfined prepares and publishes a complete file through root-confined
// operations. Link supplies the atomic no-replace namespace operation: the
// complete temporary inode becomes visible at path only when path is absent.
func PublishConfined(root confinedRoot, destination string, contents []byte, mode fs.FileMode) error {
	return publish(confinedBackend{root: root}, destination, contents, mode)
}

// publish is the single complete-file publication state machine. Backends own
// only the namespace operations whose path representation differs.
func publish(backend publicationBackend, destination string, contents []byte, mode fs.FileMode) (returnErr error) {
	committed := false
	temporary, file, err := backend.openTemporary(destination)
	if err != nil {
		return fmt.Errorf("create publication temporary for %s: %w", destination, err)
	}
	defer func() {
		if file != nil {
			returnErr = errors.Join(returnErr, file.Close())
		}
		if temporary != "" {
			if err := backend.remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
				cleanupErr := fmt.Errorf("remove publication temporary %s: %w", temporary, err)
				if committed && returnErr == nil {
					returnErr = &CommittedCleanupError{DestinationPath: destination, ResiduePath: temporary, Cause: cleanupErr}
				} else {
					returnErr = errors.Join(returnErr, cleanupErr)
				}
			}
		}
	}()
	if err := file.Chmod(mode); err != nil {
		return fmt.Errorf("set publication mode for %s: %w", destination, err)
	}
	if n, err := file.Write(contents); err != nil {
		return fmt.Errorf("write publication temporary for %s: %w", destination, err)
	} else if n != len(contents) {
		return fmt.Errorf("write publication temporary for %s: %w", destination, io.ErrShortWrite)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync publication temporary for %s: %w", destination, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close publication temporary for %s: %w", destination, err)
	}
	file = nil
	if err := backend.publishNoReplace(temporary, destination); err != nil {
		return fmt.Errorf("publish complete file without replacement to %s: %w", destination, err)
	}
	committed = true
	return nil
}
